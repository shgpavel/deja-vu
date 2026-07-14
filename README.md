Your agents already solved this. deja finds it.

<!-- TODO: add demo gif here. -->

# deja-vu

`deja` is a fast local CLI for searching AI coding-agent session histories across Claude Code, Codex CLI, and opencode.

```sh
deja "incremental indexing bug"
deja --harness claude --since 30d "panic|race" --re
deja ctx "database migration" > /tmp/deja-context.md
claude "Use this prior context: $(deja ctx 'auth refactor')"
opencode run "Use this prior context: $(deja ctx c63004c3)"
```

## Install

```sh
go install github.com/vshulcz/deja-vu/cmd/deja@latest
```

From this checkout:

```sh
go build ./cmd/deja
./deja sources
```

## Harness support

| Harness | Store | Status |
| --- | --- | --- |
| Claude Code | `~/.claude/projects/<project-dir>/**/*.jsonl` including `subagents/*.jsonl` | supported |
| Codex CLI | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` and `~/.codex/history.jsonl` | supported |
| opencode | `~/.local/share/opencode/opencode.db` | supported |
| aider | local chat history | planned |
| gemini | local chat history | planned |

## Context pipes

`deja ctx <query|session-id-prefix>` prints a compact markdown digest for agent context: session metadata, matching user problem statements, and nearby assistant conclusions, capped around 8KB.

```sh
claude "Use this prior context: $(deja ctx 'incremental indexing bug')"
opencode run "Use this prior context: $(deja ctx c63004c3)"
```

## Performance

On a real mixed corpus (327 Claude/Codex sessions + 926 opencode sessions, about 3GB of local history), warm search is about 35ms. The index is incremental: appending to one session file updates that one source file instead of rebuilding the corpus.

## How it works

`deja` builds a local inverted index in `~/.cache/deja` and refreshes only changed source files. Claude and Codex stores are streamed from JSONL. opencode is read from the local SQLite database via the `sqlite3` command-line tool because Go stdlib has no SQLite driver.

Privacy: nothing leaves your machine. `deja` reads local history files and writes a local cache only.

Environment overrides for tests or custom stores:

```sh
DEJA_CLAUDE_ROOT=/path/to/claude/projects
DEJA_CODEX_ROOT=/path/to/codex
DEJA_OPENCODE_DB=/path/to/opencode.db
DEJA_INDEX_DIR=/path/to/deja/index.db
```
