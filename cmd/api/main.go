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
	"github.com/aleshka5/auth-service/internal/llm"
	applog "github.com/aleshka5/auth-service/internal/log"
	"github.com/aleshka5/auth-service/internal/oauth"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/aleshka5/auth-service/internal/rejected"
	"github.com/aleshka5/auth-service/internal/sid"
	"github.com/aleshka5/auth-service/internal/token"
	"github.com/aleshka5/auth-service/internal/tracing"
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

	// Tracing is optional and never load-bearing: with no endpoint the tracer
	// is a no-op, and an unreachable MLflow only produces log lines.
	tracer, shutdownTracing, err := tracing.New(ctx, tracing.Config{
		Endpoint:     cfg.MLflowEndpoint,
		ExperimentID: cfg.MLflowExperimentID,
		ServiceName:  "auth-api",
	})
	if err != nil {
		logger.Warn("tracing_disabled", "err", err)
		tracer, shutdownTracing = nil, func(context.Context) error { return nil }
	} else if cfg.MLflowEndpoint != "" {
		logger.Info("tracing_enabled",
			"endpoint", tracing.TracesURL(cfg.MLflowEndpoint),
			"experiment_id", cfg.MLflowExperimentID)
	}

	// The owl chat is not load-bearing for sign-in: a bad LLM config disables
	// POST /api/chat and leaves the rest of the API running.
	chat, err := llm.New(llm.Config{
		Host:            cfg.LLMHost,
		APIKey:          cfg.LLMAPIKey,
		Model:           cfg.LLMModel,
		ReasoningEffort: cfg.LLMReasoningEffort,
		Tracer:          tracer,
	})
	if err != nil {
		logger.Warn("chat_disabled", "err", err)
	} else {
		logger.Info("chat_enabled", "host", cfg.LLMHost, "model", cfg.LLMModel, "reasoning_effort", cfg.LLMReasoningEffort)
	}

	httpHandler := httpapi.New(
		cfg,
		tokens,
		rejected.New(rdb),
		users,
		oauthSvc,
		limiter,
		func() error { return rdb.Ping(context.Background()).Err() },
		sid.New(rdb),
	).WithChat(chat).Handler()

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
	// Flush queued spans before the process goes, but never fail shutdown on it.
	if err := shutdownTracing(shutdownCtx); err != nil {
		logger.Warn("tracing_shutdown", "err", err)
	}
	return httpSrv.Shutdown(shutdownCtx)
}
