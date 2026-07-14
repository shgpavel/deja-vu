# Architecture

This document is for people changing `deja` internals.

## Source parsers

Parsers live in `internal/sources` and return `[]model.Session`.

| Source | Code | Input |
| --- | --- | --- |
| Claude Code | `claude.go` | JSONL files under `~/.claude/projects` |
| Codex CLI | `codex.go` | rollout JSONL files plus `history.jsonl` under `~/.codex` |
| opencode | `opencode.go` | SQLite database at `~/.local/share/opencode/opencode.db` |

Claude and Codex JSONL files are parsed with a worker pool sized to `runtime.NumCPU()`. Results are collected by input file index and then appended in sorted path order, so parsing can be parallel while index writes stay deterministic.

opencode is read through the local `sqlite3` command. There is no CGO SQLite dependency.

## Index format

Default path: `~/.cache/deja/index.db`.

Files:

- `records.bin`: length-prefixed JSON records. Each record stores session key, source path, role, text, and timestamp.
- `buckets/*.gob`: token bucket files. A token maps to record offsets in `records.bin`.
- `manifest.json`: index version, source file state, session metadata, build time, and search scope.

Search flow:

1. Tokenize the query.
2. Read posting lists from the token buckets.
3. Intersect posting lists for multi-word searches.
4. Read matching records from `records.bin`.
5. Group records back into sessions and rank in `internal/search`.

Regex search scans records because arbitrary regex cannot use token postings safely.

## Incremental algorithm

`currentFiles` records path, size, and mtime for known stores.

`EnsureForSearch` compares the current file set with `manifest.json`:

- fresh manifest: do nothing;
- version or scope mismatch: rebuild;
- append-only JSONL/opencode changes: append new records and update touched buckets;
- removed files or non-append changes: rewrite the index while preserving unchanged records and replacing changed sessions.

Cold rebuild does all parsing first, then writes `records.bin`, buckets, and manifest from one goroutine. That keeps the on-disk index coherent and avoids concurrent writers.

## MCP server design

`cmd/deja/mcp.go` implements a small JSON-RPC stdio server.

Supported methods:

- `initialize`
- `tools/list`
- `tools/call`

Tools:

- `recall`: returns compact snippets for matching sessions.
- `recall_context`: returns the markdown digest used by `deja ctx`.

The MCP server calls the same index/search code as the CLI. It writes protocol responses to stdout and keeps logs/progress off stdout so agents receive valid JSON-RPC.

## Add a new harness

Implement the same shape as the existing sources.

Interface:

```go
func LoadNewHarness() []model.Session
func ParseNewHarnessFile(path string) ([]model.Session, error)
func ParseNewHarnessFileFromOffset(path string, offset int64) ([]model.Session, error) // if append-only
```

Five steps:

1. Add parser code in `internal/sources` that returns `model.Session` with stable `Harness`, `ID`, `Project`, `Path`, `Started`, `Updated`, and `Messages`.
2. Add file discovery to `currentFiles` and harness detection to `harnessForPath` in `internal/index/index.go`.
3. Add load and incremental parse paths in `load`, `parseChangedFile`, and `parseAppendedFile`.
4. Add install/uninstall config handling in `cmd/deja/install.go` if the harness supports MCP.
5. Add fixtures and tests for parsing, indexing, search, and install behavior.
