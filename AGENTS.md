# Agent Guidelines

## Build & Test
- **Build**: `make build` or `make build-all` (for multi-platform)
- **Test All**: `make test` (includes race detection & coverage)
- **Test Single**: `go test -v ./internal/yourpkg -run TestName`
- **Lint/Format**: `make lint` (golangci-lint) and `make fmt` (goimports)

## Code Style & Conventions
- **Formatting**: Enforce `gofmt` and `goimports`. Always run `make fmt` before changes.
- **Imports**: Group standard lib, third-party, then internal project packages.
- **Naming**: PascalCase for exported symbols, camelCase for private. Use descriptive names.
- **Error Handling**: Explicit `if err != nil`. Return errors with context; log only at top level or in background tasks.
- **Logging**: Use the internal `logger` package (`logger.Debug`, `logger.Info`, `logger.Error`).
- **Configuration**: Prefer `flag` and environment variables. See `cmd/scheduled-db/main.go` for patterns.
- **Architecture**: Keep core logic in `internal/`, entry points in `cmd/`. Respect existing package boundaries.
- **Concurrency**: Use Go channels/mutexes. Always verify with `make test` (enables `-race`).
