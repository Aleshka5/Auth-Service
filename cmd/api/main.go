package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	grpcapi "github.com/aleshka5/auth-service/internal/grpc"
	httpapi "github.com/aleshka5/auth-service/internal/http"
	applog "github.com/aleshka5/auth-service/internal/log"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/session"
	"github.com/aleshka5/auth-service/internal/users"
	"github.com/aleshka5/auth-service/internal/whitelist"
	"github.com/aleshka5/auth-service/pkg/authpb"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	logger := applog.New()
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		return err
	}
	logger.Info("migrations_applied", "path", cfg.MigrationsPath)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	userStore := users.NewStore(pool, cfg.AdminEmail)
	sess := session.NewStore(rdb, cfg.SessionTTL)
	wl := whitelist.New(cfg.Whitelist)
	if err := wl.Persist(ctx, rdb); err != nil {
		return err
	}
	logger.Info("whitelist_loaded", "hosts", cfg.Whitelist)
	limiter := ratelimit.New(rdb)
	oauthSvc := oauth.New(cfg, rdb)

	httpHandler := httpapi.New(
		cfg,
		userStore,
		sess,
		oauthSvc,
		limiter,
		func() error { return pool.Ping(context.Background()) },
		func() error { return rdb.Ping(context.Background()).Err() },
	).Handler()

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	grpcSrv := grpc.NewServer()
	authpb.RegisterAuthServer(grpcSrv, grpcapi.New(sess, userStore, wl))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("http_listen", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		logger.Info("grpc_listen", "addr", cfg.GRPCAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	logger.Info("shutting_down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
	return nil
}

func runMigrations(databaseURL, path string) error {
	u := databaseURL
	switch {
	case strings.HasPrefix(u, "postgres://"):
		u = "pgx5://" + strings.TrimPrefix(u, "postgres://")
	case strings.HasPrefix(u, "postgresql://"):
		u = "pgx5://" + strings.TrimPrefix(u, "postgresql://")
	}
	src := path
	if !strings.HasPrefix(src, "file://") {
		src = "file://" + src
	}
	m, err := migrate.New(src, u)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
