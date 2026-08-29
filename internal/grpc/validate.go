package grpcapi

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/aleshka5/auth-service/internal/session"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/aleshka5/auth-service/internal/whitelist"
	"github.com/aleshka5/auth-service/pkg/authpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	authpb.UnimplementedAuthServer
	sessions *session.Store
	users    users.Repository
	wl       *whitelist.List
}

func New(sessions *session.Store, usersStore users.Repository, wl *whitelist.List) *Server {
	return &Server{sessions: sessions, users: usersStore, wl: wl}
}

func (s *Server) Validate(ctx context.Context, req *authpb.ValidateRequest) (*authpb.ValidateResponse, error) {
	u, _, fields, err := s.authenticate(ctx, req.GetSessionId(), req.GetCallerHost(), "validate_deny")
	if err != nil {
		return nil, err
	}
	_ = s.sessions.Touch(ctx, req.GetSessionId())
	_ = s.users.TouchLastSeen(ctx, u.ID)
	return &authpb.ValidateResponse{Fields: fields}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *authpb.ListUsersRequest) (*authpb.ListUsersResponse, error) {
	u, host, _, err := s.authenticate(ctx, req.GetSessionId(), req.GetCallerHost(), "list_users_deny")
	if err != nil {
		return nil, err
	}
	if !isServiceAdmin(host, u) {
		slog.Warn("list_users_deny", "reason", "not_admin", "host", host, "user_id", u.ID.String())
		return nil, status.Error(codes.PermissionDenied, "permission denied")
	}

	list, err := s.users.List(ctx)
	if err != nil {
		slog.Error("list_users_failed", "host", host, "user_id", u.ID.String())
		return nil, status.Error(codes.Internal, "list failed")
	}

	out := make([]*authpb.ListUser, 0, len(list))
	for i := range list {
		fields, err := s.wl.Project(host, &list[i])
		if err != nil {
			slog.Error("list_users_projection_failed", "host", host, "user_id", list[i].ID.String())
			return nil, status.Error(codes.Internal, "projection failed")
		}
		out = append(out, &authpb.ListUser{Fields: fields})
	}

	_ = s.sessions.Touch(ctx, req.GetSessionId())
	_ = s.users.TouchLastSeen(ctx, u.ID)
	slog.Info("list_users_ok", "host", host, "user_id", u.ID.String(), "count", len(out))
	return &authpb.ListUsersResponse{Users: out}, nil
}

func (s *Server) authenticate(ctx context.Context, sessionID, callerHost, denyEvent string) (*users.User, string, map[string]string, error) {
	host := whitelistHost(callerHost)
	if sessionID == "" || host == "" {
		return nil, "", nil, status.Error(codes.InvalidArgument, "session_id and caller_host are required")
	}

	uid, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		slog.Warn(denyEvent, "reason", "unauthenticated", "host", host)
		return nil, "", nil, status.Error(codes.Unauthenticated, "invalid session")
	}
	u, err := s.users.GetByID(ctx, uid)
	if err != nil {
		slog.Warn(denyEvent, "reason", "unauthenticated", "host", host)
		return nil, "", nil, status.Error(codes.Unauthenticated, "invalid session")
	}
	fields, err := s.wl.Project(host, u)
	if errors.Is(err, whitelist.ErrUnknownHost) {
		slog.Warn(denyEvent, "reason", "unknown_host", "host", host, "user_id", u.ID.String())
		return nil, "", nil, status.Error(codes.PermissionDenied, "unknown caller_host")
	}
	if err != nil {
		return nil, "", nil, status.Error(codes.Internal, "projection failed")
	}
	if users.HasBlockedRole(fields) {
		slog.Warn(denyEvent, "reason", "blocked", "host", host, "user_id", u.ID.String())
		return nil, "", nil, status.Error(codes.PermissionDenied, "blocked")
	}
	return u, host, fields, nil
}

func isServiceAdmin(host string, u *users.User) bool {
	switch host {
	case "storage.filenkov.store":
		return u.StorageRoles == users.ServiceRoleAdmin
	case "resume.filenkov.store":
		return u.ResumeRoles == users.ServiceRoleAdmin
	default:
		return u.Roles == users.RoleAdmin
	}
}

func whitelistHost(raw string) string {
	h := strings.TrimSpace(strings.ToLower(raw))
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i]
	}
	return h
}
