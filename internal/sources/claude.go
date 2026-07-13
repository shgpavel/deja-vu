package sources

import (
	"path/filepath"
	"strings"

	"github.com/vshulcz/deja-vu/internal/model"
)

func ClaudeRoot() string {
	return EnvPath("DEJA_CLAUDE_ROOT", filepath.Join(Home(), ".claude", "projects"))
}

func LoadClaude() []model.Session {
	root := ClaudeRoot()
	files := walkFiles(root, func(p string) bool { return strings.HasSuffix(p, ".jsonl") })
	return parseFiles(files, ParseClaudeFile)
}

func ParseClaudeFile(path string) ([]model.Session, error) {
	s := model.Session{Harness: "claude", ID: strings.TrimSuffix(filepath.Base(path), ".jsonl"), Project: projectName(filepath.Dir(path)), Path: path}
	if filepath.Base(filepath.Dir(path)) == "subagents" {
		s.Project = projectName(filepath.Dir(filepath.Dir(path)))
	}
	err := scanJSONL(path, func(m map[string]any) {
		typ, _ := m["type"].(string)
		if typ != "user" && typ != "assistant" {
			return
		}
		if id, _ := m["sessionId"].(string); id != "" {
			s.ID = id
		}
		if cwd, _ := m["cwd"].(string); cwd != "" {
			s.Project = projectName(cwd)
		}
		t := parseTimeAny(m["timestamp"])
		s.Touch(t)
		role := typ
		txt := ""
		if msg, ok := m["message"].(map[string]any); ok {
			if r, _ := msg["role"].(string); r != "" {
				role = r
			}
			txt = textFromContent(msg["content"])
		}
		if txt != "" {
			s.Messages = append(s.Messages, model.Message{Role: role, Text: txt, Time: t})
		}
	})
	if len(s.Messages) == 0 {
		return nil, err
	}
	return []model.Session{s}, err
}
