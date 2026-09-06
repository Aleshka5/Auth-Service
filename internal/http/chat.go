package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aleshka5/auth-service/internal/llm"
	"github.com/google/uuid"
)

// chatSessionHeader carries the SPA's per-browser-session id. It is opaque
// attribution for the proxy's logs, never a credential — the handler is public,
// so nothing is authorized on it. The signed-in visitor's identity comes from
// the gateway-injected X-Auth-* headers instead, which the gateway strips from
// the incoming request before setting them itself.
const chatSessionHeader = "X-Chat-Session"

const maxChatBody = 4 << 10

type chatRequest struct {
	Question string `json:"question"`
}

// handleChat answers one question as a Server-Sent Events stream. Each request
// stands alone: no history is read, kept, or sent upstream.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.chat == nil {
		writeError(w, http.StatusServiceUnavailable, "chat is not configured")
		return
	}

	var body chatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxChatBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	question, err := llm.ValidateQuestion(body.Question)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	// nginx sits in front of nothing here, but the SPA's dev proxy and any
	// future buffering hop need to be told not to hold the stream.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	meta := chatMeta(r)
	slog.Info("chat_question", "session_id", meta.SessionID, "user_id", meta.UserID, "chars", len([]rune(question)))

	sendErr := s.chat.Ask(r.Context(), question, meta, func(delta string) error {
		if err := writeSSE(w, map[string]string{"delta": delta}); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if sendErr != nil {
		if errors.Is(sendErr, r.Context().Err()) {
			// The reader navigated away mid-answer; nothing left to write to.
			return
		}
		slog.Error("chat_stream", "err", sendErr, "session_id", meta.SessionID)
		// The status line is long gone, so the failure travels as an event.
		_ = writeSSE(w, map[string]string{"error": "The owl could not reach its brain. Try again."})
		flusher.Flush()
		return
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func writeSSE(w io.Writer, event map[string]string) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = w.Write(append(append([]byte("data: "), payload...), '\n', '\n'))
	return err
}

func chatMeta(r *http.Request) llm.Meta {
	return llm.Meta{
		SessionID: chatSessionID(r),
		UserID:    strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		Email:     strings.TrimSpace(r.Header.Get("X-Auth-Email")),
	}
}

// chatSessionID keeps the client's id only when it is a short opaque token, so
// a caller cannot smuggle arbitrary text into the proxy's logs.
func chatSessionID(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get(chatSessionHeader))
	if raw == "" || len(raw) > 64 {
		return uuid.NewString()
	}
	for _, c := range raw {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '_', c == '.':
		default:
			return uuid.NewString()
		}
	}
	return raw
}
