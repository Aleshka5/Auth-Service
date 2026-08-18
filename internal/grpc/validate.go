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
	host := whitelistHost(req.GetCallerHost())
	if req.GetSessionId() == "" || host == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id and caller_host are required")
	}

	uid, err := s.sessions.Get(ctx, req.GetSessionId())
	if err != nil {
		slog.Warn("validate_deny", "reason", "unauthenticated", "host", host)
		return nil, status.Error(codes.Unauthenticated, "invalid session")
	}
	u, err := s.users.GetByID(ctx, uid)
	if err != nil {
		slog.Warn("validate_deny", "reason", "unauthenticated", "host", host)
		return nil, status.Error(codes.Unauthenticated, "invalid session")
	}
	if u.Blocked {
		slog.Warn("validate_deny", "reason", "blocked", "host", host, "user_id", u.ID.String())
		return nil, status.Error(codes.PermissionDenied, "blocked")
	}
	fields, err := s.wl.Project(host, u)
	if errors.Is(err, whitelist.ErrUnknownHost) {
		slog.Warn("validate_deny", "reason", "unknown_host", "host", host, "user_id", u.ID.String())
		return nil, status.Error(codes.PermissionDenied, "unknown caller_host")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "projection failed")
	}
	_ = s.sessions.Touch(ctx, req.GetSessionId())
	_ = s.users.TouchLastSeen(ctx, u.ID)
	return &authpb.ValidateResponse{Fields: fields}, nil
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
