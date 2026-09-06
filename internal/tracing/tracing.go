// Package tracing ships spans to MLflow over OTLP/HTTP.
//
// MLflow 3.6+ accepts OpenTelemetry traces at POST {endpoint}/v1/traces with the
// target experiment in the `x-mlflow-experiment-id` header, which is the only
// ingestion path open to a non-Python client. Tracing is deliberately decoupled
// from generation: spans are batched on a background goroutine, so a slow or
// dead MLflow delays no answer and fails no request.
package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	// ExperimentHeader routes a trace to an MLflow experiment.
	ExperimentHeader = "x-mlflow-experiment-id"
	// TracesPath is MLflow's OTLP/HTTP ingestion path.
	TracesPath = "/v1/traces"
)

type Config struct {
	// Endpoint is the MLflow server base URL, e.g. http://mlflow:5000.
	// A URL that already ends in /v1/traces is accepted as-is. Empty disables
	// tracing entirely.
	Endpoint string
	// ExperimentID is the MLflow experiment ("project") the traces land in.
	ExperimentID string
	ServiceName  string
	// Timeout bounds one export attempt, not the answer.
	Timeout time.Duration
}

// Shutdown flushes whatever is still queued. Always safe to call.
type Shutdown func(context.Context) error

// New returns a tracer and its shutdown. With no endpoint configured it returns
// a no-op tracer and a no-op shutdown, so callers need no branch of their own.
func New(ctx context.Context, cfg Config) (trace.Tracer, Shutdown, error) {
	name := cfg.ServiceName
	if name == "" {
		name = "auth-api"
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return noop.NewTracerProvider().Tracer(name), func(context.Context) error { return nil }, nil
	}
	if strings.TrimSpace(cfg.ExperimentID) == "" {
		return nil, nil, errors.New("MLFLOW_EXPERIMENT_ID is required when MLFLOW_TRACING_ENDPOINT is set")
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(TracesURL(endpoint)),
		otlptracehttp.WithHeaders(map[string]string{ExperimentHeader: strings.TrimSpace(cfg.ExperimentID)}),
		otlptracehttp.WithTimeout(timeout),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("mlflow otlp exporter: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		// Batched, so Ask never waits on MLflow.
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", name),
		)),
	)

	// An unreachable MLflow is a log line, never a failed answer.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Warn("tracing_export", "err", err)
	}))

	return provider.Tracer(name), provider.Shutdown, nil
}

// TracesURL turns an MLflow base URL into its OTLP ingestion URL, and leaves a
// URL that already points at the ingestion path alone.
func TracesURL(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(trimmed, TracesPath) {
		return trimmed
	}
	return trimmed + TracesPath
}
