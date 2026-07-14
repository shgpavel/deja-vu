# Contributing

## Build and test

```sh
go test ./...
go vet ./...
go build ./cmd/deja
```

Useful local checks:

```sh
go test ./internal/index ./internal/sources ./cmd/deja
go test -run TestMCP ./cmd/deja
```

## Pull requests

- Keep changes small and explain the user-visible behavior.
- Add or update tests for parser, index, CLI, or MCP changes.
- Run `gofmt` on changed Go files.
- Do not commit private history, generated local indexes, or machine-specific logs.
- No CLA.
