package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vshulcz/deja-vu/internal/model"
	"github.com/vshulcz/deja-vu/internal/search"
	"github.com/vshulcz/deja-vu/internal/sources"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "deja:", err)
		os.Exit(1)
	}
}

func loadAll(h string) []model.Session {
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

func loadForSearch(o search.Options) []model.Session {
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
	return ss
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	if args[0] == "sources" {
		printSources()
		return nil
	}
	if args[0] == "show" {
		if len(args) < 2 {
			return fmt.Errorf("show needs id-prefix")
		}
		ss := append(loadAll("claude"), loadAll("codex")...)
		ss = append(ss, sources.LoadOpencodePrefix(args[1])...)
		s, ok := search.FindByPrefix(ss, args[1])
		if !ok {
			return fmt.Errorf("no session matches %q", args[1])
		}
		search.PrintSession(os.Stdout, s)
		return nil
	}
	if args[0] == "last" {
		n := 10
		if len(args) > 1 {
			if x, err := strconv.Atoi(args[1]); err == nil {
				n = x
			}
		}
		ss := append(loadAll("claude"), loadAll("codex")...)
		ss = append(ss, sources.LoadOpencodeRecent(n)...)
		for _, s := range search.Recent(ss, n) {
			fmt.Printf("[%s · %s · %s · %s]\n", s.Harness, s.Project, s.Updated.Format("2006-01-02"), s.ID)
		}
		return nil
	}
	o, err := parseSearch(args)
	if err != nil {
		return err
	}
	hits, err := search.Run(loadForSearch(o), o)
	if err != nil {
		return err
	}
	search.Print(os.Stdout, hits, o)
	return nil
}

func parseSearch(args []string) (search.Options, error) {
	o := search.Options{}
	var q []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			o.JSON = true
		case "--re":
			o.Regex = true
		case "--all":
			o.All = true
		case "--harness", "--project", "--since", "--role":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s needs value", a)
			}
			i++
			v := args[i]
			if a == "--harness" {
				o.Harness = v
			} else if a == "--project" {
				o.Project = v
			} else if a == "--role" {
				o.Role = v
			} else {
				d, err := parseDur(v)
				if err != nil {
					return o, err
				}
				o.Since = d
			}
		default:
			q = append(q, a)
		}
	}
	o.Query = strings.Join(q, " ")
	if o.Query == "" {
		return o, fmt.Errorf("query required")
	}
	return o, nil
}
func parseDur(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		return time.Duration(n) * 24 * time.Hour, err
	}
	return time.ParseDuration(s)
}

func printSources() {
	items := []struct {
		name, root string
		load       func() []model.Session
	}{{"claude", sources.ClaudeRoot(), sources.LoadClaude}, {"codex", sources.CodexRoot(), sources.LoadCodex}}
	for _, it := range items {
		var size int64
		if fi, err := os.Stat(it.root); err == nil {
			size = fi.Size()
		}
		ss := it.load()
		msg := 0
		for _, s := range ss {
			msg += len(s.Messages)
		}
		fmt.Printf("%s\t%s\tsessions=%d messages=%d size=%d\n", it.name, it.root, len(ss), msg, size)
	}
	var size int64
	if fi, err := os.Stat(sources.OpencodeDB()); err == nil {
		size = fi.Size()
	}
	s, m, _ := sources.OpencodeCounts()
	fmt.Printf("opencode\t%s\tsessions=%d messages=%d size=%d\n", sources.OpencodeDB(), s, m, size)
}
func usage() {
	fmt.Println("usage: deja [--json] [--re] [--harness name] [--project p] [--since 30d] [--role user] <query>\n       deja show <id-prefix>\n       deja last [n]\n       deja sources")
}
