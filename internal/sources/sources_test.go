package sources

import (
	"path/filepath"
	"testing"
)

func TestParseClaudeFile(t *testing.T) {
	ss, err := ParseClaudeFile(filepath.Join("..", "..", "fixtures", "synthetic", "claude", "project", "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].ID != "claude-abc" || len(ss[0].Messages) != 2 {
		t.Fatalf("bad claude parse: %#v", ss)
	}
	if ss[0].Messages[1].Text != "The frobnicator bug is in parser.go" {
		t.Fatalf("bad text: %q", ss[0].Messages[1].Text)
	}
}

func TestParseCodexRollout(t *testing.T) {
	p := filepath.Join("..", "..", "fixtures", "synthetic", "codex", "sessions", "2026", "01", "02", "rollout-2026-01-02T03-04-05-codex-abc.jsonl")
	ss, err := ParseCodexRollout(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].ID != "codex-abc" || len(ss[0].Messages) != 2 {
		t.Fatalf("bad codex parse: %#v", ss)
	}
}

func TestParseCodexHistory(t *testing.T) {
	ss, err := ParseCodexHistory(filepath.Join("..", "..", "fixtures", "synthetic", "codex", "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 1 || ss[0].ID != "hist-abc" || ss[0].Messages[0].Text != "history needle" {
		t.Fatalf("bad history: %#v", ss)
	}
}
