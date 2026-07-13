package search

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/deja-vu/internal/model"
)

func TestSearchRanksAndFilters(t *testing.T) {
	now := time.Now()
	ss := []model.Session{{ID: "a", Harness: "claude", Project: "p", Updated: now, Messages: []model.Message{{Role: "user", Text: "needle needle", Time: now}}}, {ID: "b", Harness: "codex", Project: "p", Updated: now.Add(-24 * time.Hour), Messages: []model.Message{{Role: "assistant", Text: "needle", Time: now}}}}
	hits, err := Run(ss, Options{Query: "needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || hits[0].Session.ID != "a" || hits[0].Count != 2 {
		t.Fatalf("bad hits: %#v", hits)
	}
	hits, err = Run(ss, Options{Query: "needle", Harness: "codex", Role: "assistant"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Session.ID != "b" {
		t.Fatalf("bad filter: %#v", hits)
	}
}

func TestPrintPlainWhenNotTTY(t *testing.T) {
	now := time.Now()
	hits := []Hit{{Session: model.Session{ID: "abcdef1234567890", Harness: "opencode", Project: "deja", Updated: now}, Count: 1, Snippets: []string{"hello needle"}}}
	var b bytes.Buffer
	Print(&b, hits, Options{Query: "needle"})
	out := b.String()
	if strings.Contains(out, "\x1b[") || !strings.Contains(out, "[opencode]") || !strings.Contains(out, "1 matches") {
		t.Fatalf("bad plain output: %q", out)
	}
}
