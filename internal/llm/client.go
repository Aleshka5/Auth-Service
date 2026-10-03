// Package llm asks an OpenAI-compatible chat-completions endpoint one question
// at a time. The endpoint is the LiteLLM proxy (LLM_HOST), not the public
// host llm.filenkov.store; nothing here keeps conversation state.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// MaxQuestionRunes caps a single question. The owl answers one question at a
// time and remembers nothing, so there is no reason to accept an essay.
const MaxQuestionRunes = 255

// ErrEmptyQuestion and ErrQuestionTooLong are the two ways a question is
// rejected before any token is spent.
var (
	ErrEmptyQuestion   = errors.New("question is empty")
	ErrQuestionTooLong = fmt.Errorf("question is longer than %d characters", MaxQuestionRunes)
)

type Config struct {
	// Host is the OpenAI-compatible base URL, e.g. http://litellm-proxy-server:4000/v1
	Host   string
	APIKey string
	Model  string
	// ReasoningEffort is sent as `reasoning_effort`. "none" asks a reasoning
	// model not to think — the owl answers in two sentences, so a chain of
	// thought is only latency and tokens. Empty omits the field, for a proxy
	// that rejects it.
	ReasoningEffort string
	// Timeout bounds the whole streamed answer. Zero means 5 minutes — the
	// answer is not capped in length, so it has to have room to finish.
	Timeout time.Duration
	// Tracer receives one span per question. Nil disables tracing; it never
	// affects the answer either way.
	Tracer trace.Tracer
}

type Client struct {
	host      string
	apiKey    string
	model     string
	reasoning string
	tracer    trace.Tracer
	http      *http.Client
}

func New(cfg Config) (*Client, error) {
	host := strings.TrimRight(strings.TrimSpace(cfg.Host), "/")
	if host == "" {
		return nil, errors.New("LLM_HOST is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("LLM_MODEL is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Client{
		host:      host,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		model:     strings.TrimSpace(cfg.Model),
		reasoning: strings.TrimSpace(cfg.ReasoningEffort),
		tracer:    cfg.Tracer,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          20,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
	}, nil
}

// Meta is per-request attribution passed to the proxy for its logs. Email and
// UserID are set only for a signed-in visitor; SessionID is set for everyone.
type Meta struct {
	SessionID string
	UserID    string
	Email     string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string            `json:"model"`
	Messages    []chatMessage     `json:"messages"`
	Stream      bool              `json:"stream"`
	Temperature float64           `json:"temperature"`
	User        string            `json:"user,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	// Omitted when empty, so a proxy that does not know the field never sees it.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			// reasoning_content / reasoning are deliberately not mapped: the
			// chain of thought is dropped rather than streamed to the reader.
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ValidateQuestion trims the question and enforces the size limit.
func ValidateQuestion(raw string) (string, error) {
	q := strings.TrimSpace(raw)
	if q == "" {
		return "", ErrEmptyQuestion
	}
	if len([]rune(q)) > MaxQuestionRunes {
		return "", ErrQuestionTooLong
	}
	return q, nil
}

// Ask streams the answer to one question, calling onDelta for every fragment as
// it arrives. There is no history: the request carries the system prompt and
// this question, and nothing else.
func (c *Client) Ask(ctx context.Context, question string, meta Meta, onDelta func(string) error) error {
	q, err := ValidateQuestion(question)
	if err != nil {
		return err
	}

	// Opened after validation, so a rejected question never reaches MLflow.
	// The span carries the question and the assembled answer; a reader who
	// leaves mid-stream still produces one, with the partial answer and the
	// cancellation on it.
	ctx, span := startAskSpan(ctx, c.tracer, c.model, q, meta)
	var answer strings.Builder
	err = c.stream(ctx, q, meta, func(delta string) error {
		answer.WriteString(delta)
		return onDelta(delta)
	})
	span.end(answer.String(), err)
	return err
}

// stream is the generation call itself, with no knowledge of tracing.
func (c *Client) stream(ctx context.Context, q string, meta Meta, onDelta func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: SystemPrompt()},
			{Role: "user", Content: q},
		},
		// No max_tokens: a cap here truncates the answer mid-word, and the
		// model's own stop sequence is the only end this package wants.
		Stream:          true,
		Temperature:     0.3,
		User:            meta.SessionID,
		Metadata:        meta.fields(),
		ReasoningEffort: c.reasoning,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("llm upstream %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	// Anything the model thinks out loud is dropped here, not in the browser.
	filter := &thinkFilter{}
	err = readSSE(resp.Body, func(delta string) error {
		if visible := filter.feed(delta); visible != "" {
			return onDelta(visible)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if tail := filter.flush(); tail != "" {
		return onDelta(tail)
	}
	return nil
}

func (m Meta) fields() map[string]string {
	f := map[string]string{"source": "owl-chat"}
	if m.SessionID != "" {
		f["session_id"] = m.SessionID
	}
	if m.UserID != "" {
		f["user_id"] = m.UserID
	}
	if m.Email != "" {
		f["user_email"] = m.Email
	}
	return f
}

// readSSE walks the `data:` lines of a chat-completions stream. A chunk that
// does not parse is skipped rather than failing the answer — proxies prepend
// keep-alive comments and vendor-specific events.
func readSSE(r io.Reader, onDelta func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 8192), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" {
				return nil
			}
			continue
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("llm upstream: %s", chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
