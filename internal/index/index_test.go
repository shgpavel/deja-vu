package index

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vshulcz/deja-vu/internal/search"
)

func TestIndexIngestSkipAndSearch(t *testing.T) {
	tmp := t.TempDir()
	claudeRoot := filepath.Join(tmp, "claude")
	proj := filepath.Join(claudeRoot, "-Users-shulcz-deja-vu")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"type":"user","sessionId":"s1","timestamp":"2026-01-02T03:04:05Z","message":{"role":"user","content":"fast opencode needle"}}` + "\n" +
		`{"type":"assistant","sessionId":"s1","timestamp":"2026-01-02T03:05:05Z","message":{"role":"assistant","content":"answer text"}}` + "\n"
	if err := os.WriteFile(filepath.Join(proj, "s1.jsonl"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEJA_CLAUDE_ROOT", claudeRoot)
	dir := filepath.Join(tmp, "index.db")
	var first bytes.Buffer
	if err := Ensure(dir, "claude", false, &first); err != nil {
		t.Fatal(err)
	}
	if first.Len() == 0 {
		t.Fatal("first build did not print progress")
	}
	var second bytes.Buffer
	if err := Ensure(dir, "claude", false, &second); err != nil {
		t.Fatal(err)
	}
	if second.Len() != 0 {
		t.Fatalf("fresh index rebuilt unexpectedly: %q", second.String())
	}
	ss, err := Search(dir, search.Options{Query: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].ID != "s1" || ss[0].Messages[0].Text != "fast opencode needle" {
		t.Fatalf("bad search sessions: %#v", ss)
	}
}
