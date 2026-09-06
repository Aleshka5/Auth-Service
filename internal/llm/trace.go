package llm

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// askSpan records one question and its answer for MLflow.
//
// Two sets of attributes go on every span. `mlflow.span*` are the keys MLflow's
// own SDK writes, and are what its UI reads for a span's inputs and outputs;
// `gen_ai.*` are the OpenTelemetry GenAI semantic conventions, which MLflow and
// every other OTLP backend understand. Writing both costs a few bytes and means
// the trace is legible wherever it lands.
type askSpan struct {
	span    trace.Span
	started time.Time
}

func startAskSpan(ctx context.Context, tracer trace.Tracer, model, question string, meta Meta) (context.Context, *askSpan) {
	if tracer == nil {
		return ctx, nil
	}
	ctx, span := tracer.Start(ctx, "owl.chat", trace.WithSpanKind(trace.SpanKindClient))
	span.SetAttributes(
		attribute.String("mlflow.spanType", "LLM"),
		attribute.String("mlflow.spanInputs", jsonAttr(map[string]any{"question": question})),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.request.model", model),
		attribute.String("gen_ai.system", "mlflow.gateway"),
	)
	// Identity is exactly what already goes to the proxy: the session id for
	// everyone, id and email only for a signed-in reader.
	if meta.SessionID != "" {
		span.SetAttributes(attribute.String("session.id", meta.SessionID))
	}
	if meta.UserID != "" {
		span.SetAttributes(attribute.String("user.id", meta.UserID))
	}
	if meta.Email != "" {
		span.SetAttributes(attribute.String("user.email", meta.Email))
	}
	return ctx, &askSpan{span: span, started: time.Now()}
}

// end closes the span with the assembled answer, or with the failure.
func (a *askSpan) end(answer string, err error) {
	if a == nil {
		return
	}
	a.span.SetAttributes(
		attribute.String("mlflow.spanOutputs", jsonAttr(map[string]any{"answer": answer})),
		attribute.Int("owl.answer_chars", len([]rune(answer))),
		attribute.Int64("owl.duration_ms", time.Since(a.started).Milliseconds()),
	)
	if err != nil {
		a.span.RecordError(err)
		a.span.SetStatus(codes.Error, err.Error())
	} else {
		a.span.SetStatus(codes.Ok, "")
	}
	a.span.End()
}

func jsonAttr(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
