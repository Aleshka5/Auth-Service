package users

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the user persistence surface used by HTTP and gRPC handlers.
type Repository interface {
	UpsertFromGoogle(ctx context.Context, p GoogleProfile) (*User, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	List(ctx context.Context) ([]User, error)
	UpdateRoles(ctx context.Context, id uuid.UUID, patch RolePatch) (*User, error)
	TouchLastSeen(ctx context.Context, id uuid.UUID) error
}
