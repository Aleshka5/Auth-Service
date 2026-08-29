package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/gateway"
	httpapi "github.com/aleshka5/auth-service/internal/http"
	applog "github.com/aleshka5/auth-service/internal/log"
	"github.com/aleshka5/auth-service/internal/oauth"
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.GatewayConfig != "" {
		gwFile, err := gateway.LoadFile(cfg.GatewayConfig)
		if err != nil {
			return err
		}
		cfg.SetGatewayHosts(gwFile.Hostnames())
		logger.Info("gateway_return_to_hosts", "path", cfg.GatewayConfig, "hosts", gwFile.Hostnames())
	}

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

	tokens, err := token.New(token.Config{
		Secret:     []byte(cfg.JWTSecret),
		Issuer:     cfg.PublicOrigin,
		Audiences:  cfg.JWTAudiences(),
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	if err != nil {
		return err
	}
	users, err := usersvc.New(cfg.UserServiceURL, nil)
	if err != nil {
		return err
	}

	limiter := ratelimit.New(rdb)
	oauthSvc := oauth.New(cfg, rdb)
	httpHandler := httpapi.New(
		cfg,
		tokens,
		rejected.New(rdb),
		users,
		oauthSvc,
		limiter,
		func() error { return rdb.Ping(context.Background()).Err() },
		sid.New(rdb),
	).Handler()

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http_listen", "addr", cfg.HTTPAddr)
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
