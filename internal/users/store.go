package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("user not found")
	ErrLastAdmin = errors.New("cannot demote or block the last admin")
)

const userColumns = `id, google_sub, google_email, name, roles, storage_roles, resume_roles, last_seen_at, created_at, updated_at`

type Store struct {
	pool       *pgxpool.Pool
	adminEmail string
}

func NewStore(pool *pgxpool.Pool, adminEmail string) *Store {
	return &Store{pool: pool, adminEmail: strings.ToLower(strings.TrimSpace(adminEmail))}
}

var _ Repository = (*Store)(nil)

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) UpsertFromGoogle(ctx context.Context, p GoogleProfile) (*User, bool, error) {
	email := strings.ToLower(strings.TrimSpace(p.Email))
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = email
	}
	role := RoleStranger
	if s.adminEmail != "" && email == s.adminEmail {
		role = RoleAdmin
	}

	var (
		u        User
		roles    string
		storage  string
		resume   string
		inserted bool
	)
	err := s.pool.QueryRow(ctx, `
INSERT INTO users (google_sub, google_email, name, roles)
VALUES ($1, $2, $3, $4)
ON CONFLICT (google_sub) DO UPDATE SET
	google_email = EXCLUDED.google_email,
	name = EXCLUDED.name,
	updated_at = NOW()
RETURNING `+userColumns+`, (xmax = 0) AS inserted
`, p.Sub, email, name, role).Scan(
		&u.ID, &u.GoogleSub, &u.GoogleEmail, &u.Name,
		&roles, &storage, &resume, &u.LastSeenAt, &u.CreatedAt, &u.UpdatedAt,
		&inserted,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upsert user: %w", err)
	}
	u.Roles = Role(roles)
	u.StorageRoles = ServiceRole(storage)
	u.ResumeRoles = ServiceRole(resume)
	slog.Info("user_upsert",
		"user_id", u.ID.String(),
		"google_email", u.GoogleEmail,
		"inserted", inserted,
		"roles", u.Roles,
	)
	return &u, inserted, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRoles(ctx context.Context, id uuid.UUID, patch RolePatch) (*User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	u, err := scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if patch.Roles != nil && u.Roles == RoleAdmin && *patch.Roles != RoleAdmin {
		n, err := countActiveAdminsTx(ctx, tx)
		if err != nil {
			return nil, err
		}
		if RemovesLastAdmin(n, *u, patch.Roles) {
			return nil, ErrLastAdmin
		}
	}

	var roles any
	var storage any
	var resume any
	if patch.Roles != nil {
		roles = string(*patch.Roles)
	}
	if patch.StorageRoles != nil {
		storage = string(*patch.StorageRoles)
	}
	if patch.ResumeRoles != nil {
		resume = string(*patch.ResumeRoles)
	}

	u, err = scanUser(tx.QueryRow(ctx, `
UPDATE users SET
	roles = COALESCE($2::roles, roles),
	storage_roles = COALESCE($3::storage_roles, storage_roles),
	resume_roles = COALESCE($4::resume_roles, resume_roles),
	updated_at = NOW()
WHERE id = $1
RETURNING `+userColumns, id, roles, storage, resume))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	slog.Info("admin_role_change",
		"user_id", u.ID.String(),
		"google_email", u.GoogleEmail,
		"roles", u.Roles,
		"storage_roles", u.StorageRoles,
		"resume_roles", u.ResumeRoles,
	)
	return u, nil
}

func (s *Store) TouchLastSeen(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
UPDATE users
SET last_seen_at = NOW(), updated_at = NOW()
WHERE id = $1 AND (last_seen_at IS NULL OR last_seen_at < NOW() - INTERVAL '1 minute')
`, id)
	return err
}

func countActiveAdminsTx(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE roles = 'ADMIN'`).Scan(&n)
	return n, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var (
		u       User
		roles   string
		storage string
		resume  string
	)
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.GoogleEmail, &u.Name,
		&roles, &storage, &resume, &u.LastSeenAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.Roles = Role(roles)
	u.StorageRoles = ServiceRole(storage)
	u.ResumeRoles = ServiceRole(resume)
	return &u, nil
}
