# Contributing

## Primary (Go backend)
- Go 1.23 or higher (`go version`)
- SQLite headers bundled via `modernc.org/sqlite` (no external system deps)
- Helpful tools: `curl`, `git`

Common commands:
```bash
go test ./...
go run ./cmd/temchik init
go run ./cmd/temchik up   # stop: go run ./cmd/temchik down
```

## Legacy TypeScript workspace (optional)
The Node/TypeScript monorepo under `app/` and `packages/` is retained for reference only and is not required to run the Go backend. If you must tinker there, you’ll need Node.js ≥22 and pnpm, but those tools are not part of the supported runtime path.
