package llm

import (
	"strings"
	"testing"
)

// feedAll streams the chunks through one filter, as Ask does.
func feedAll(chunks ...string) string {
	f := &thinkFilter{}
	var out strings.Builder
	for _, c := range chunks {
		out.WriteString(f.feed(c))
	}
	out.WriteString(f.flush())
	return out.String()
}

func TestThinkFilterDropsInlineReasoning(t *testing.T) {
	cases := map[string]struct {
		chunks []string
		want   string
	}{
		"whole block in one delta": {
			[]string{"<think>the user asks about Sber</think>He led reporting agents."},
			"He led reporting agents.",
		},
		"block split across deltas": {
			[]string{"<thi", "nk>weighing", " the answer</thi", "nk>He led ", "reporting agents."},
			"He led reporting agents.",
		},
		"reasoning after the answer": {
			[]string{"He led agents.", "<think>done</think>"},
			"He led agents.",
		},
		"two blocks": {
			[]string{"<think>a</think>One.", "<think>b</think> Two."},
			"One. Two.",
		},
		"no reasoning at all": {
			[]string{"He led ", "reporting agents."},
			"He led reporting agents.",
		},
		"unclosed block swallows the rest": {
			[]string{"Fine.<think>never closed"},
			"Fine.",
		},
	}
	for name, tc := range cases {
		if got := feedAll(tc.chunks...); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestThinkFilterKeepsATrailingPartialTag(t *testing.T) {
	// "<thi" that never becomes a tag is real text and must survive the stream.
	if got := feedAll("cost 3 <thi"); got != "cost 3 <thi" {
		t.Fatalf("got %q", got)
	}
	// A lone "<" must not be held forever either.
	if got := feedAll("a < b"); got != "a < b" {
		t.Fatalf("got %q", got)
	}
}

func TestThinkFilterHoldsBackOnlyThePartialTag(t *testing.T) {
	f := &thinkFilter{}
	// The safe prefix goes out immediately; only "<thi" waits for more.
	if got := f.feed("He led agents.<thi"); got != "He led agents." {
		t.Fatalf("first delta %q", got)
	}
	if got := f.feed("nk>hidden</think> Done."); got != " Done." {
		t.Fatalf("second delta %q", got)
	}
}
