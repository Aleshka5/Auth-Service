package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/gateway"
	applog "github.com/aleshka5/auth-service/internal/log"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/rejected"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/usersvc"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := applog.New()
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.LoadShared()
	if err != nil {
		return err
	}

	path := cfg.GatewayConfig
	if path == "" {
		path = env("GATEWAY_CONFIG", "deploy/gateway.yaml")
	}
	file, err := gateway.LoadFile(path)
	if err != nil {
		return err
	}
	if origin := strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN")); origin != "" {
		cfg.PublicOrigin = strings.TrimRight(origin, "/")
	} else if file.PublicOrigin != "" {
		cfg.PublicOrigin = strings.TrimRight(file.PublicOrigin, "/")
	}
	cfg.SetGatewayHosts(file.Hostnames())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	tok, err := token.New(token.Config{
		Secret:     []byte(cfg.JWTSecret),
		Issuer:     cfg.PublicOrigin,
		Audiences:  cfg.JWTAudiences(),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	if err != nil {
		return fmt.Errorf("jwt: %w", err)
	}

	users, err := usersvc.New(cfg.UserServiceURL, nil)
	if err != nil {
		return fmt.Errorf("user-service: %w", err)
	}

	gw, err := gateway.New(cfg, file, tok, rejected.New(rdb), ratelimit.New(rdb), users, sid.New(rdb))
	if err != nil {
		return err
	}

	addr := env("GATEWAY_HTTP_ADDR", file.Listen)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           gw.Handler(),
		ReadHeaderTimeout: file.Timeouts.ReadHeader,
		IdleTimeout:       file.Timeouts.Idle,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("gateway_listen", "addr", addr, "config", path, "hosts", file.Hostnames())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	return httpSrv.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
