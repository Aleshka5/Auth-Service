package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func recordingTracer(t *testing.T) (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	return rec, sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
}

func attrs(span sdktrace.ReadOnlySpan) map[string]string {
	m := map[string]string{}
	for _, kv := range span.Attributes() {
		m[string(kv.Key)] = kv.Value.Emit()
	}
	return m
}

func TestAskRecordsQuestionAndAnswer(t *testing.T) {
	srv := sseServer(t, []string{delta("He led "), delta("reporting agents."), "data: [DONE]\n\n"}, nil)
	rec, tp := recordingTracer(t)
	c, _ := New(Config{Host: srv.URL, Model: "m", Tracer: tp.Tracer("test")})

	err := c.Ask(context.Background(), "What did he do?", Meta{SessionID: "s-1", UserID: "u-1", Email: "a@b.c"},
		func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("want one span, got %d", len(spans))
	}
	got := attrs(spans[0])
	if spans[0].Name() != "owl.chat" {
		t.Fatalf("span name %q", spans[0].Name())
	}
	if !strings.Contains(got["mlflow.spanInputs"], "What did he do?") {
		t.Fatalf("inputs %q", got["mlflow.spanInputs"])
	}
	if !strings.Contains(got["mlflow.spanOutputs"], "He led reporting agents.") {
		t.Fatalf("outputs %q", got["mlflow.spanOutputs"])
	}
	if got["gen_ai.request.model"] != "m" || got["mlflow.spanType"] != "LLM" {
		t.Fatalf("attrs %v", got)
	}
	if got["session.id"] != "s-1" || got["user.id"] != "u-1" || got["user.email"] != "a@b.c" {
		t.Fatalf("identity %v", got)
	}
}

func TestAskSpanOmitsIdentityForGuests(t *testing.T) {
	srv := sseServer(t, []string{delta("hi"), "data: [DONE]\n\n"}, nil)
	rec, tp := recordingTracer(t)
	c, _ := New(Config{Host: srv.URL, Model: "m", Tracer: tp.Tracer("test")})

	_ = c.Ask(context.Background(), "q", Meta{SessionID: "s-2"}, func(string) error { return nil })

	got := attrs(rec.Ended()[0])
	if _, ok := got["user.email"]; ok {
		t.Fatalf("guest span carries an email: %v", got)
	}
	if _, ok := got["user.id"]; ok {
		t.Fatalf("guest span carries a user id: %v", got)
	}
}

func TestAskSpanRecordsAFailure(t *testing.T) {
	srv := sseServer(t, []string{delta("partial"), `data: {"error":{"message":"upstream boom"}}` + "\n\n"}, nil)
	rec, tp := recordingTracer(t)
	c, _ := New(Config{Host: srv.URL, Model: "m", Tracer: tp.Tracer("test")})

	if err := c.Ask(context.Background(), "q", Meta{}, func(string) error { return nil }); err == nil {
		t.Fatal("expected the upstream error")
	}

	span := rec.Ended()[0]
	if span.Status().Code.String() != "Error" {
		t.Fatalf("status %v", span.Status())
	}
	// The partial answer is still worth having in the trace.
	if !strings.Contains(attrs(span)["mlflow.spanOutputs"], "partial") {
		t.Fatalf("outputs %v", attrs(span))
	}
	if len(span.Events()) == 0 {
		t.Fatal("the error should be recorded on the span")
	}
}

func TestAskDoesNotTraceARejectedQuestion(t *testing.T) {
	rec, tp := recordingTracer(t)
	c, _ := New(Config{Host: "http://unused", Model: "m", Tracer: tp.Tracer("test")})

	err := c.Ask(context.Background(), "   ", Meta{}, func(string) error { return nil })
	if !errors.Is(err, ErrEmptyQuestion) {
		t.Fatalf("err %v", err)
	}
	if len(rec.Ended()) != 0 {
		t.Fatal("a question that never reached the model must not produce a span")
	}
}

func TestAskWithoutATracerStillAnswers(t *testing.T) {
	srv := sseServer(t, []string{delta("fine"), "data: [DONE]\n\n"}, nil)
	c, _ := New(Config{Host: srv.URL, Model: "m"})

	var got strings.Builder
	if err := c.Ask(context.Background(), "q", Meta{}, func(d string) error {
		got.WriteString(d)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got.String() != "fine" {
		t.Fatalf("got %q", got.String())
	}
}
