package index

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/vshulcz/deja-vu/internal/model"
	"github.com/vshulcz/deja-vu/internal/search"
	"github.com/vshulcz/deja-vu/internal/sources"
)

const version = 3
const maxIndexedText = 64 * 1024

type FileState struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
}

type SessionMeta struct {
	ID, Harness, Project, Path string
	Started, Updated           time.Time
}

type Manifest struct {
	Version  int                    `json:"version"`
	Files    map[string]FileState   `json:"files"`
	Sessions map[string]SessionMeta `json:"sessions"`
	BuiltAt  time.Time              `json:"built_at"`
	Scope    string                 `json:"scope"`
}

type Record struct {
	Key       string
	Role      string
	Text      string
	Time      time.Time
	LowerText string `json:"-"`
}

func DefaultDir() string {
	if v := os.Getenv("DEJA_INDEX_DIR"); v != "" {
		return v
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".cache", "deja", "index.db")
}

func Ensure(dir string, harness string, force bool, progress io.Writer) error {
	if dir == "" {
		dir = DefaultDir()
	}
	want := currentFiles(harness)
	m, err := readManifest(dir)
	if !force && err == nil && manifestFresh(m, want, "") {
		return nil
	}
	if progress != nil {
		fmt.Fprintf(progress, "deja: indexing sessions into %s ...\n", dir)
	}
	return rebuild(dir, harness, "", want)
}

func EnsureForSearch(dir string, o search.Options, force bool, progress io.Writer) error {
	if dir == "" {
		dir = DefaultDir()
	}
	want := currentFiles(o.Harness)
	scope := scopeFor(o)
	m, err := readManifest(dir)
	if !force && err == nil && manifestFresh(m, want, scope) {
		return nil
	}
	if progress != nil {
		fmt.Fprintf(progress, "deja: indexing sessions into %s ...\n", dir)
	}
	return rebuildForSearch(dir, o, scope, want)
}

func Search(dir string, o search.Options) ([]model.Session, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	m, err := readManifest(dir)
	if err != nil {
		return nil, err
	}
	var offsets []int64
	if !o.Regex {
		if keys := queryKeys(o.Query); len(keys) > 0 {
			offsets, _ = postingsFor(dir, keys[0])
		}
	}
	if len(offsets) == 0 {
		return scanRecords(dir, m, o, nil)
	}
	return scanRecords(dir, m, o, offsets)
}

func rebuild(dir string, harness string, scope string, files map[string]FileState) error {
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "buckets"), 0o755); err != nil {
		return err
	}
	ss := load(harness)
	m := Manifest{Version: version, Files: files, Sessions: map[string]SessionMeta{}, BuiltAt: time.Now(), Scope: scope}
	recPath := filepath.Join(tmp, "records.bin")
	rf, err := os.Create(recPath)
	if err != nil {
		return err
	}
	buckets := map[string]map[string][]int64{}
	for _, s := range ss {
		key := s.Harness + ":" + s.ID
		if old, ok := m.Sessions[key]; ok {
			if s.Started.IsZero() || (!old.Started.IsZero() && old.Started.Before(s.Started)) {
				s.Started = old.Started
			}
			if old.Updated.After(s.Updated) {
				s.Updated = old.Updated
			}
		}
		m.Sessions[key] = SessionMeta{ID: s.ID, Harness: s.Harness, Project: s.Project, Path: s.Path, Started: s.Started, Updated: s.Updated}
		for _, msg := range s.Messages {
			text := msg.Text
			if len(text) > maxIndexedText {
				text = text[:maxIndexedText]
			}
			off, err := writeRecord(rf, Record{Key: key, Role: msg.Role, Text: text, Time: msg.Time})
			if err != nil {
				rf.Close()
				return err
			}
			seen := map[string]bool{}
			for _, tok := range indexKeys(msg.Text) {
				if seen[tok] {
					continue
				}
				seen[tok] = true
				b := bucket(tok)
				if buckets[b] == nil {
					buckets[b] = map[string][]int64{}
				}
				buckets[b][tok] = append(buckets[b][tok], off)
			}
		}
	}
	if err := rf.Close(); err != nil {
		return err
	}
	for b, data := range buckets {
		if err := writeGob(filepath.Join(tmp, "buckets", b+".gob"), data); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(tmp, "manifest.json"), m); err != nil {
		return err
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

func load(h string) []model.Session {
	var ss []model.Session
	if h == "" || h == "claude" {
		ss = append(ss, sources.LoadClaude()...)
	}
	if h == "" || h == "codex" {
		ss = append(ss, sources.LoadCodex()...)
	}
	if h == "" || h == "opencode" {
		ss = append(ss, sources.LoadOpencode()...)
	}
	return ss
}

func rebuildForSearch(dir string, o search.Options, scope string, files map[string]FileState) error {
	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "buckets"), 0o755); err != nil {
		return err
	}
	var ss []model.Session
	if o.Harness == "" || o.Harness == "claude" {
		ss = append(ss, sources.LoadClaude()...)
	}
	if o.Harness == "" || o.Harness == "codex" {
		ss = append(ss, sources.LoadCodex()...)
	}
	if o.Harness == "" || o.Harness == "opencode" {
		if o.Regex {
			ss = append(ss, sources.LoadOpencodeRecent(200)...)
		} else {
			ss = append(ss, sources.LoadOpencodeMatching(o.Query)...)
		}
	}
	return writeSessions(tmp, dir, ss, files, scope)
}

func writeSessions(tmp, dir string, ss []model.Session, files map[string]FileState, scope string) error {
	m := Manifest{Version: version, Files: files, Sessions: map[string]SessionMeta{}, BuiltAt: time.Now(), Scope: scope}
	recPath := filepath.Join(tmp, "records.bin")
	rf, err := os.Create(recPath)
	if err != nil {
		return err
	}
	buckets := map[string]map[string][]int64{}
	for _, s := range ss {
		key := s.Harness + ":" + s.ID
		if old, ok := m.Sessions[key]; ok {
			if s.Started.IsZero() || (!old.Started.IsZero() && old.Started.Before(s.Started)) {
				s.Started = old.Started
			}
			if old.Updated.After(s.Updated) {
				s.Updated = old.Updated
			}
		}
		m.Sessions[key] = SessionMeta{ID: s.ID, Harness: s.Harness, Project: s.Project, Path: s.Path, Started: s.Started, Updated: s.Updated}
		for _, msg := range s.Messages {
			text := msg.Text
			if len(text) > maxIndexedText {
				text = text[:maxIndexedText]
			}
			off, err := writeRecord(rf, Record{Key: key, Role: msg.Role, Text: text, Time: msg.Time})
			if err != nil {
				rf.Close()
				return err
			}
			seen := map[string]bool{}
			for _, tok := range indexKeys(text) {
				if seen[tok] {
					continue
				}
				seen[tok] = true
				b := bucket(tok)
				if buckets[b] == nil {
					buckets[b] = map[string][]int64{}
				}
				buckets[b][tok] = append(buckets[b][tok], off)
			}
		}
	}
	if err := rf.Close(); err != nil {
		return err
	}
	for b, data := range buckets {
		if err := writeGob(filepath.Join(tmp, "buckets", b+".gob"), data); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(tmp, "manifest.json"), m); err != nil {
		return err
	}
	os.RemoveAll(dir)
	return os.Rename(tmp, dir)
}

func currentFiles(h string) map[string]FileState {
	paths := map[string]bool{}
	addWalk := func(root string, pred func(string) bool) {
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && pred(p) {
				paths[p] = true
			}
			return nil
		})
	}
	if h == "" || h == "claude" {
		addWalk(sources.ClaudeRoot(), func(p string) bool { return strings.HasSuffix(p, ".jsonl") })
	}
	if h == "" || h == "codex" {
		addWalk(filepath.Join(sources.CodexRoot(), "sessions"), func(p string) bool {
			return strings.HasSuffix(p, ".jsonl") && strings.Contains(filepath.Base(p), "rollout-")
		})
		paths[filepath.Join(sources.CodexRoot(), "history.jsonl")] = true
	}
	if h == "" || h == "opencode" {
		paths[sources.OpencodeDB()] = true
	}
	out := map[string]FileState{}
	for p := range paths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			out[p] = FileState{Path: p, Size: fi.Size(), MTime: fi.ModTime().UnixNano()}
		}
	}
	return out
}

func manifestFresh(m Manifest, files map[string]FileState, scope string) bool {
	if m.Version != version || len(m.Files) != len(files) {
		return false
	}
	if m.Scope != scope {
		return false
	}
	for p, f := range files {
		if m.Files[p] != f {
			return false
		}
	}
	return true
}

func scopeFor(o search.Options) string {
	if o.Regex {
		return "re:" + o.Harness + ":" + o.Query
	}
	return "q:" + o.Harness + ":" + strings.ToLower(o.Query)
}

func scanRecords(dir string, m Manifest, o search.Options, offsets []int64) ([]model.Session, error) {
	by := map[string]*model.Session{}
	add := func(r Record) {
		meta, ok := m.Sessions[r.Key]
		if !ok {
			return
		}
		if o.Harness != "" && meta.Harness != o.Harness {
			return
		}
		if o.Project != "" && !strings.Contains(strings.ToLower(meta.Project), strings.ToLower(o.Project)) {
			return
		}
		if o.Since > 0 && meta.Updated.Before(time.Now().Add(-o.Since)) {
			return
		}
		if o.Role != "" && r.Role != o.Role {
			return
		}
		s := by[r.Key]
		if s == nil {
			cp := model.Session{ID: meta.ID, Harness: meta.Harness, Project: meta.Project, Path: meta.Path, Started: meta.Started, Updated: meta.Updated}
			s = &cp
			by[r.Key] = s
		}
		s.Messages = append(s.Messages, model.Message{Role: r.Role, Text: r.Text, Time: r.Time})
	}
	if len(offsets) > 0 {
		f, err := os.Open(filepath.Join(dir, "records.bin"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		for _, off := range offsets {
			if r, err := readRecordAt(f, off); err == nil {
				add(r)
			}
		}
	} else {
		if err := eachRecord(filepath.Join(dir, "records.bin"), add); err != nil {
			return nil, err
		}
	}
	out := make([]model.Session, 0, len(by))
	for _, s := range by {
		out = append(out, *s)
	}
	return out, nil
}

func writeRecord(f *os.File, r Record) (int64, error) {
	off, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return 0, err
	}
	if len(b) > 1<<31 {
		return 0, fmt.Errorf("record too large")
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := f.Write(hdr[:]); err != nil {
		return 0, err
	}
	_, err = f.Write(b)
	return off, err
}

func readRecordAt(f *os.File, off int64) (Record, error) {
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return Record{}, err
	}
	return readRecord(bufio.NewReader(f))
}

func eachRecord(path string, fn func(Record)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1024*1024)
	for {
		rec, err := readRecord(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		fn(rec)
	}
}

func readRecord(r io.Reader) (Record, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Record{}, err
	}
	b := make([]byte, binary.LittleEndian.Uint32(hdr[:]))
	if _, err := io.ReadFull(r, b); err != nil {
		return Record{}, err
	}
	var rec Record
	return rec, json.Unmarshal(b, &rec)
}

func postingsFor(dir, tok string) ([]int64, error) {
	var data map[string][]int64
	if err := readGob(filepath.Join(dir, "buckets", bucket(tok)+".gob"), &data); err != nil {
		return nil, err
	}
	return data[tok], nil
}

func tokens(s string) []string {
	seen := map[string]bool{}
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 2 {
			t := b.String()
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
		b.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
			if b.Len() > 64 {
				flush()
			}
		} else {
			flush()
		}
	}
	flush()
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func indexKeys(s string) []string {
	var out []string
	for _, tok := range tokens(s) {
		out = append(out, "t"+tok)
	}
	return out
}

func queryKeys(s string) []string {
	toks := tokens(s)
	if len(toks) == 0 {
		return nil
	}
	return []string{"t" + toks[0]}
}

func bucket(tok string) string {
	if len(tok) >= 2 {
		return safe(tok[:2])
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(tok))
	return fmt.Sprintf("x%02x", h.Sum32()%256)
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, s)
}

func readManifest(dir string) (Manifest, error) {
	var m Manifest
	err := readJSON(filepath.Join(dir, "manifest.json"), &m)
	return m, err
}
func writeJSON(p string, v any) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(v)
}
func readJSON(p string, v any) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}
func writeGob(p string, v any) error {
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(v)
}
func readGob(p string, v any) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewDecoder(f).Decode(v)
}
