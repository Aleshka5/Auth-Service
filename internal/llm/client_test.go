package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sseServer(t *testing.T, chunks []string, seen *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]any
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Errorf("request body: %v", err)
			}
			parsed["_path"] = r.URL.Path
			parsed["_auth"] = r.Header.Get("Authorization")
			*seen = parsed
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			_, _ = io.WriteString(w, c)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func delta(text string) string {
	return `data: {"choices":[{"delta":{"content":` + jsonString(text) + `}}]}` + "\n\n"
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestAskStreamsDeltasAndSendsMetadata(t *testing.T) {
	var seen map[string]any
	srv := sseServer(t, []string{
		": keep-alive\n\n",
		delta("Alexey "),
		"data: {\"choices\":[{\"delta\":{}}]}\n\n",
		delta("builds agents."),
		"data: [DONE]\n\n",
		delta("after done"),
	}, &seen)

	c, err := New(Config{Host: srv.URL + "/v1", APIKey: "dummy", Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	err = c.Ask(context.Background(), "  Who is he?  ", Meta{SessionID: "s-1", UserID: "u-1", Email: "a@b.c"}, func(d string) error {
		got.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "Alexey builds agents." {
		t.Fatalf("deltas %q", got.String())
	}

	if seen["_path"] != "/v1/chat/completions" {
		t.Fatalf("path %v", seen["_path"])
	}
	if seen["_auth"] != "Bearer dummy" {
		t.Fatalf("auth %v", seen["_auth"])
	}
	if seen["model"] != "test-model" || seen["stream"] != true {
		t.Fatalf("model/stream %v %v", seen["model"], seen["stream"])
	}
	if seen["user"] != "s-1" {
		t.Fatalf("user %v", seen["user"])
	}
	meta, _ := seen["metadata"].(map[string]any)
	if meta["session_id"] != "s-1" || meta["user_id"] != "u-1" || meta["user_email"] != "a@b.c" {
		t.Fatalf("metadata %v", meta)
	}

	// One question, no history: system prompt plus the trimmed question.
	msgs, _ := seen["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages %v", msgs)
	}
	user, _ := msgs[1].(map[string]any)
	if user["role"] != "user" || user["content"] != "Who is he?" {
		t.Fatalf("user message %v", user)
	}
	system, _ := msgs[0].(map[string]any)
	text, _ := system["content"].(string)
	if !strings.Contains(text, "Alexey Filenkov") || !strings.Contains(text, "/experience/sber") {
		t.Fatal("system prompt is missing the bio or the node map")
	}
}

func TestAskOmitsIdentityForGuests(t *testing.T) {
	var seen map[string]any
	srv := sseServer(t, []string{delta("hi"), "data: [DONE]\n\n"}, &seen)
	c, _ := New(Config{Host: srv.URL, Model: "m"})

	if err := c.Ask(context.Background(), "hi", Meta{SessionID: "s-2"}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if seen["_auth"] != "" {
		t.Fatalf("empty key must not send Authorization, got %v", seen["_auth"])
	}
	meta, _ := seen["metadata"].(map[string]any)
	if _, ok := meta["user_email"]; ok {
		t.Fatal("guest request must not carry an email")
	}
	if _, ok := meta["user_id"]; ok {
		t.Fatal("guest request must not carry a user id")
	}
}

func TestAskReportsUpstreamFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"model not found"}}`)
	}))
	defer srv.Close()

	c, _ := New(Config{Host: srv.URL, Model: "nope"})
	err := c.Ask(context.Background(), "q", Meta{}, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("err %v", err)
	}
}

func TestValidateQuestion(t *testing.T) {
	if _, err := ValidateQuestion("   "); err != ErrEmptyQuestion {
		t.Fatalf("empty: %v", err)
	}
	limit := strings.Repeat("я", MaxQuestionRunes)
	if q, err := ValidateQuestion(limit); err != nil || q != limit {
		t.Fatalf("%d runes must pass: %v", MaxQuestionRunes, err)
	}
	if _, err := ValidateQuestion(limit + "я"); err != ErrQuestionTooLong {
		t.Fatalf("over limit: %v", err)
	}
}

func TestNewRequiresHostAndModel(t *testing.T) {
	if _, err := New(Config{Model: "m"}); err == nil {
		t.Fatal("expected a host error")
	}
	if _, err := New(Config{Host: "http://x/v1"}); err == nil {
		t.Fatal("expected a model error")
	}
}

func TestAskSendsReasoningEffort(t *testing.T) {
	var seen map[string]any
	srv := sseServer(t, []string{delta("hi"), "data: [DONE]\n\n"}, &seen)

	c, _ := New(Config{Host: srv.URL, Model: "m", ReasoningEffort: "none"})
	if err := c.Ask(context.Background(), "q", Meta{}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if seen["reasoning_effort"] != "none" {
		t.Fatalf("reasoning_effort %v", seen["reasoning_effort"])
	}

	// Empty means the field is omitted, for a proxy that rejects it outright.
	c, _ = New(Config{Host: srv.URL, Model: "m"})
	if err := c.Ask(context.Background(), "q", Meta{}, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, ok := seen["reasoning_effort"]; ok {
		t.Fatalf("field must be omitted when unset: %v", seen)
	}
}

func TestAskIgnoresSeparateReasoningContent(t *testing.T) {
	// A well-behaved proxy streams the chain of thought in its own field.
	srv := sseServer(t, []string{
		`data: {"choices":[{"delta":{"reasoning_content":"the user asks about Sber"}}]}` + "\n\n",
		delta("He led reporting agents."),
		"data: [DONE]\n\n",
	}, nil)

	c, _ := New(Config{Host: srv.URL, Model: "m"})
	var got strings.Builder
	if err := c.Ask(context.Background(), "q", Meta{}, func(d string) error {
		got.WriteString(d)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got.String() != "He led reporting agents." {
		t.Fatalf("reasoning leaked: %q", got.String())
	}
}

func TestAskDropsInlineReasoningFromTheAnswer(t *testing.T) {
	srv := sseServer(t, []string{
		delta("<think>weighing "),
		delta("the answer</think>He led "),
		delta("reporting agents."),
		"data: [DONE]\n\n",
	}, nil)

	c, _ := New(Config{Host: srv.URL, Model: "m"})
	var got strings.Builder
	if err := c.Ask(context.Background(), "q", Meta{}, func(d string) error {
		got.WriteString(d)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got.String() != "He led reporting agents." {
		t.Fatalf("got %q", got.String())
	}
}
