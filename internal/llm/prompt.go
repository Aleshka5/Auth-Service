package llm

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
)

// bio is the hand-written half of the system prompt: who the owner is, and the
// rules for answering. Edit it directly.
//
//go:embed prompt.md
var bio string

// nodesJSON is the generated half — one line per public resume-tree page, so
// the owl can point a reader at the right branch. Regenerate with
// `npm run gen:owl-context` in web/ after adding or renaming a node.
//
//go:embed nodes.json
var nodesJSON []byte

type node struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	Teaser string `json:"teaser"`
}

// SystemPrompt is the full prompt sent with every question. It never changes at
// runtime, so it is built once.
var SystemPrompt = sync.OnceValue(buildSystemPrompt)

func buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(bio, "\n"))

	var nodes []node
	if err := json.Unmarshal(nodesJSON, &nodes); err != nil {
		// A malformed generated file costs the owl its map, not its voice.
		slog.Error("llm_nodes_json", "err", err)
		return b.String() + "\n"
	}
	for _, n := range nodes {
		b.WriteString("\n- ")
		b.WriteString(n.Path)
		b.WriteString(" — ")
		b.WriteString(n.Title)
		if n.Teaser != "" {
			b.WriteString(": ")
			b.WriteString(n.Teaser)
		}
	}
	b.WriteString("\n")
	return b.String()
}
