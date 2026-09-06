package tracing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTracesURL(t *testing.T) {
	cases := map[string]string{
		"http://mlflow:5000":            "http://mlflow:5000/v1/traces",
		"http://mlflow:5000/":           "http://mlflow:5000/v1/traces",
		"  http://mlflow:5000  ":        "http://mlflow:5000/v1/traces",
		"http://mlflow:5000/v1/traces":  "http://mlflow:5000/v1/traces",
		"http://mlflow:5000/v1/traces/": "http://mlflow:5000/v1/traces",
	}
	for in, want := range cases {
		if got := TracesURL(in); got != want {
			t.Errorf("TracesURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewWithoutEndpointIsANoOp(t *testing.T) {
	tracer, shutdown, err := New(context.Background(), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if tracer == nil || shutdown == nil {
		t.Fatal("a disabled tracer must still be usable")
	}
	// The no-op tracer records nothing but must not panic.
	_, span := tracer.Start(context.Background(), "owl.chat")
	span.End()
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewRequiresAnExperimentID(t *testing.T) {
	if _, _, err := New(context.Background(), Config{Endpoint: "http://mlflow:5000"}); err == nil {
		t.Fatal("an endpoint without an experiment id must be refused")
	}
}

// TestExportsToMLflowPath drives a real export against a stand-in MLflow, so the
// path, method and experiment header are checked rather than assumed.
func TestExportsToMLflowPath(t *testing.T) {
	type seen struct {
		method, path, experiment, contentType string
		body                                  int
	}
	got := make(chan seen, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case got <- seen{r.Method, r.URL.Path, r.Header.Get(ExperimentHeader), r.Header.Get("Content-Type"), len(body)}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tracer, shutdown, err := New(context.Background(), Config{
		Endpoint:     srv.URL,
		ExperimentID: "42",
		ServiceName:  "auth-api",
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, span := tracer.Start(context.Background(), "owl.chat")
	span.End()
	// Shutdown flushes the batch processor.
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case s := <-got:
		if s.method != http.MethodPost || s.path != TracesPath {
			t.Fatalf("%s %s, want POST %s", s.method, s.path, TracesPath)
		}
		if s.experiment != "42" {
			t.Fatalf("%s = %q", ExperimentHeader, s.experiment)
		}
		if s.contentType != "application/x-protobuf" {
			t.Fatalf("content-type %q", s.contentType)
		}
		if s.body == 0 {
			t.Fatal("empty OTLP payload")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no export reached the server")
	}
}

// A dead MLflow must cost the caller nothing but a log line.
func TestExportFailureDoesNotBlockTheCaller(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close() // refuse connections outright

	tracer, shutdown, err := New(context.Background(), Config{Endpoint: srv.URL, ExperimentID: "1", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_, span := tracer.Start(context.Background(), "owl.chat")
		span.End()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("span recording blocked on an unreachable MLflow")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = shutdown(ctx)
}
