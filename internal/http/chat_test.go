package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aleshka5/auth-service/internal/config"
	"github.com/aleshka5/auth-service/internal/llm"
	"github.com/aleshka5/auth-service/internal/ratelimit"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// chatHarness wires just enough Server to exercise POST /api/chat, plus a stub
// standing in for the LiteLLM proxy.
type chatHarness struct {
	h        http.Handler
	upstream *httptest.Server
	lastBody map[string]any
}

func newChatHarness(t *testing.T, chunks ...string) *chatHarness {
	t.Helper()
	if len(chunks) == 0 {
		chunks = []string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"He builds \"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\"agents.\"}}]}\n\n",
			"data: [DONE]\n\n",
		}
	}
	hr := &chatHarness{}
	hr.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &hr.lastBody)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			_, _ = io.WriteString(w, c)
		}
	}))
	t.Cleanup(hr.upstream.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	client, err := llm.New(llm.Config{Host: hr.upstream.URL + "/v1", APIKey: "dummy", Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{PublicOrigin: "https://filenkov.store", DevHTTP: true}
	hr.h = New(cfg, nil, nil, nil, nil, ratelimit.New(rdb), nil, nil).WithChat(client).Handler()
	return hr
}

func postChat(t *testing.T, h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChatStreamsSSE(t *testing.T) {
	hr := newChatHarness(t)
	rec := postChat(t, hr.h, `{"question":"What does he do?"}`, map[string]string{
		chatSessionHeader: "abc-123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{`data: {"delta":"He builds "}`, `data: {"delta":"agents."}`, "data: [DONE]"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %q", want, body)
		}
	}
	if hr.lastBody["user"] != "abc-123" {
		t.Fatalf("session id not forwarded: %v", hr.lastBody["user"])
	}
}

func TestChatForwardsIdentityOnlyFromGatewayHeaders(t *testing.T) {
	hr := newChatHarness(t)
	postChat(t, hr.h, `{"question":"hi"}`, map[string]string{
		chatSessionHeader: "sess-1",
		"X-Auth-User-Id":  "uuid-1",
		"X-Auth-Email":    "user@example.com",
	})
	meta, _ := hr.lastBody["metadata"].(map[string]any)
	if meta["user_email"] != "user@example.com" || meta["user_id"] != "uuid-1" {
		t.Fatalf("metadata %v", meta)
	}

	// A guest sends no X-Auth-*; the gateway strips any the client invented.
	postChat(t, hr.h, `{"question":"hi"}`, map[string]string{chatSessionHeader: "sess-2"})
	meta, _ = hr.lastBody["metadata"].(map[string]any)
	if _, ok := meta["user_email"]; ok {
		t.Fatalf("guest metadata %v", meta)
	}
}

func TestChatReplacesUnusableSessionID(t *testing.T) {
	hr := newChatHarness(t)
	postChat(t, hr.h, `{"question":"hi"}`, map[string]string{
		chatSessionHeader: "not a session id; drop table",
	})
	got, _ := hr.lastBody["user"].(string)
	if got == "" || strings.Contains(got, " ") {
		t.Fatalf("expected a generated id, got %q", got)
	}
}

func TestChatRejectsBadQuestions(t *testing.T) {
	hr := newChatHarness(t)
	cases := map[string]string{
		"empty":    `{"question":"   "}`,
		"missing":  `{}`,
		"too long": `{"question":"` + strings.Repeat("a", llm.MaxQuestionRunes+1) + `"}`,
		"not json": `{`,
	}
	for name, body := range cases {
		rec := postChat(t, hr.h, body, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, rec.Code)
		}
	}
}

func TestChatSendsErrorEventWhenUpstreamFails(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer up.Close()
	client, _ := llm.New(llm.Config{Host: up.URL, Model: "m"})
	h := New(&config.Config{}, nil, nil, nil, nil, ratelimit.New(rdb), nil, nil).WithChat(client).Handler()

	rec := postChat(t, h, `{"question":"hi"}`, nil)
	// Headers are already flushed, so the failure has to arrive as an event.
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestChatWithoutClientIsUnavailable(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	h := New(&config.Config{}, nil, nil, nil, nil, ratelimit.New(rdb), nil, nil).Handler()

	if rec := postChat(t, h, `{"question":"hi"}`, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d", rec.Code)
	}
}
