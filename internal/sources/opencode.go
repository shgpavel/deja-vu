package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vshulcz/deja-vu/internal/model"
)

func OpencodeDB() string {
	return EnvPath("DEJA_OPENCODE_DB", filepath.Join(Home(), ".local", "share", "opencode", "opencode.db"))
}

func LoadOpencode() []model.Session {
	ss, _ := ParseOpencodeDBWhere(OpencodeDB(), "", 0)
	return ss
}

func LoadOpencodeMatching(q string) []model.Session {
	where := fmt.Sprintf(" and lower(p.data) like '%%%s%%'", sqlQuote(strings.ToLower(q)))
	ss, _ := ParseOpencodeDBWhere(OpencodeDB(), where, 5000)
	return ss
}

func LoadOpencodeRecent(n int) []model.Session {
	ss, _ := ParseOpencodeDBWhere(OpencodeDB(), "", n*20)
	return ss
}

func LoadOpencodePrefix(p string) []model.Session {
	where := fmt.Sprintf(" and s.id like '%s%%'", sqlQuote(p))
	ss, _ := ParseOpencodeDBWhere(OpencodeDB(), where, 0)
	return ss
}

func ParseOpencodeDB(db string) ([]model.Session, error) {
	return ParseOpencodeDBWhere(db, "", 0)
}

func ParseOpencodeDBWhere(db, where string, limit int) ([]model.Session, error) {
	lim := ""
	if limit > 0 {
		lim = fmt.Sprintf(" limit %d", limit)
	}
	q := `select s.id,s.directory,s.title,s.time_created,s.time_updated,m.data as mdata,p.data as pdata from session s join message m on m.session_id=s.id join part p on p.message_id=m.id where json_extract(p.data,'$.type')='text'` + where + ` order by s.time_updated desc,m.time_created,p.id` + lim
	cmd := exec.Command("sqlite3", "-json", db, q)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var rows []map[string]any
	if err := dec.Decode(&rows); err != nil {
		return nil, err
	}
	by := map[string]*model.Session{}
	for _, r := range rows {
		id, _ := r["id"].(string)
		if id == "" {
			continue
		}
		s := by[id]
		if s == nil {
			dir, _ := r["directory"].(string)
			s = &model.Session{Harness: "opencode", ID: id, Project: projectName(dir), Path: dir, Started: parseTimeAny(r["time_created"]), Updated: parseTimeAny(r["time_updated"])}
			by[id] = s
		}
		var md, pd map[string]any
		json.Unmarshal([]byte(str(r["mdata"])), &md)
		json.Unmarshal([]byte(str(r["pdata"])), &pd)
		role, _ := md["role"].(string)
		txt, _ := pd["text"].(string)
		if txt == "" {
			continue
		}
		t := parseNestedTime(pd, "time", "start")
		if t.IsZero() {
			t = parseNestedTime(md, "time", "created")
		}
		s.Touch(t)
		s.Messages = append(s.Messages, model.Message{Role: role, Text: txt, Time: t})
	}
	var out []model.Session
	for _, s := range by {
		out = append(out, *s)
	}
	return out, nil
}

func OpencodeCounts() (sessions, messages int, err error) {
	cmd := exec.Command("sqlite3", OpencodeDB(), "select (select count(*) from session),(select count(*) from part where json_extract(data,'$.type')='text')")
	b, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	f := strings.Split(strings.TrimSpace(string(b)), "|")
	if len(f) == 2 {
		fmt.Sscanf(f[0], "%d", &sessions)
		fmt.Sscanf(f[1], "%d", &messages)
	}
	return
}

func sqlQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func str(v any) string { s, _ := v.(string); return s }
func parseNestedTime(m map[string]any, k, sub string) time.Time {
	if x, ok := m[k].(map[string]any); ok {
		return parseTimeAny(x[sub])
	}
	return time.Time{}
}
