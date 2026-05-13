# Agent Guidelines

## Build & Test
- **Build**: `make build` — produces `./scheduled-db` binary from `cmd/scheduled-db/main.go`
- **Build multi-platform**: `make build-all` — outputs to `dist/`
- **Test All**: `make test` — runs with `-race` and generates `coverage.out`
- **Test Single Package**: `go test -v ./internal/store` (no raft needed for most unit tests)
- **Test Single Test**: `go test -v ./internal/store -run TestFSMApply`
- **Short Tests**: `make test-short` — skips slow tests via `-short` flag
- **Benchmarks**: `make bench`
- **Lint**: `make lint` (golangci-lint)
- **Format**: `make fmt` (runs both `go fmt ./...` and `goimports -w .`)
- **Deps**: `make install-deps` — installs golangci-lint and goimports
- **Order**: `make fmt` → `make lint` → `make test`

## WASM Simulator
- **Build WASM**: `make wasm` — produces `docs/scheduled-db.wasm` + `docs/wasm_exec.js`
- **Entrypoint**: `cmd/wasm/main.go` — exports simulator API via `syscall/js`
- **Simulator package**: `internal/simulator/` — clock, executor, worker, coordinator
- **Build tags**: `//go:build !wasm` on all files importing Raft, BoltDB, net/http, metrics
- **WASM-compatible files** (no build tags): `store/types.go`, `store/fsm.go`, `slots/types.go`, `slots/timing_wheel.go`, `logger/logger.go`
- **Release**: Push a tag `v*` to trigger `.github/workflows/wasm-release.yml` → GitHub Release with WASM artifacts
- **JS API**: `simulator.createJob()`, `.deleteJob()`, `.tick()`, `.advanceTime()`, `.getState()`, `.getStateJSON()`, `.setClockSpeed()`, `.setSuccessRate()`, `.setTime()`, `.getNow()`, `.reset()`

## Architecture
- **Entrypoint**: `cmd/scheduled-db/main.go` — CLI flags + env vars → `internal.NewApp()`
- **Module**: `scheduled-db` (not `github.com/...` — import paths use `scheduled-db/internal/...`)
- **Core packages** in `internal/`:
  - `store/` — Raft consensus + FSM + status tracker + cold slot store + DNS address provider (the distributed state layer)
  - `slots/` — time-slotted job queue, worker, execution manager, capacity limits (memory/job), slot evictor
  - `api/` — HTTP handlers + gorilla/mux router
  - `discovery/` — service discovery strategies (kubernetes, dns, gossip, static) + split-brain detection
  - `logger/` — custom structured logger (`logger.Info/Error/Debug/Warn/ClusterInfo`)
  - `metrics/` — Prometheus + OpenTelemetry
  - `e2e/` — end-to-end cluster tests (5 tests, require running cluster)
- **`internal/app.go`** wires everything together: Store → SlotQueue → Worker → HTTP server. Modification flow: Raft log → FSM → EventHandler → SlotQueue

## Key Conventions
- **Logs**: Always use `scheduled-db/internal/logger` (not `log` or `fmt`). Has cluster-aware helpers: `logger.ClusterInfo`, `logger.ClusterWarn`, `logger.ClusterError`
- **Errors**: Explicit `if err != nil`; return with `fmt.Errorf("context: %w", err)`. Never panic in production paths.
- **Tests**: Property-based tests use `pgregory.net/rapid` (not `testing/quick`). Files named `*_property_test.go`.
- **Configuration**: All flags have both CLI and env-var forms (e.g., `--node-id` / `NODE_ID`). See `cmd/scheduled-db/main.go` for the canonical list.
- **Go toolchain**: Requires Go 1.23+ with toolchain 1.24.2 (per go.mod). `CGO_ENABLED=0` for builds.

## Testing Quirks
- `internal/store/store_test.go` does NOT exist despite being listed in docs — Store tests require a live Raft cluster and aren't unit-testable. Tests target `fsm_test.go`, `fsm_property_test.go`, `fsm_capacity_test.go`, `cold_store_test.go`, `status_tracker_test.go` instead.
- Integration/cluster tests require `make dev-up` (Docker Compose 3-node cluster) then `make create-jobs`, `make test-proxy`, `make test-failover`.
- E2E Go tests: `E2E_API_BASE=http://localhost:80 go test -v ./internal/e2e` (5 tests: ClusterHasLeader, AllNodesHealthy, ClusterConfigurationHasThreeNodes, RaftReplication, WriteForwardingFromFollower).
- `make test` enables `-race`; if tests fail with race detector, the actual code has a data race — don't suppress it.

## Dev Environment
- **Local cluster**: `make dev-up` (Docker Compose, nginx LB on :80, nodes on :8080/8081/8082, Prometheus :9090, Grafana :3000)
- **Teardown**: `make dev-down` (includes `-v` to remove volumes)
- **K8s**: `make k8s-deploy` (uses `kubectl apply -k k8s/`). Skaffold config exists at repo root.
- **Job types**: `unico` (one-time) and `recurrente` (cron recurring). These Spanish names are intentional — don't rename them.

## Non-Obvious Commands
- `make test-proxy` / `make test-failover` — integration tests against running cluster
- `make cluster-info` — auto-detects Docker vs K8s and shows leader/nodes
- `make create-jobs` — auto-detects environment, creates 5 unico + 3 recurrente test jobs
- `make mod` — tidies and downloads modules
- `make security-scan` — runs gosec + nancy (needs both installed)

## Key Configuration Flags
- **Execution**: `--execution-timeout` / `JOB_EXECUTION_TIMEOUT` (5m), `--inprogress-timeout` / `JOB_INPROGRESS_TIMEOUT` (5m), `--max-attempts` / `MAX_EXECUTION_ATTEMPTS` (3)
- **Cold Spilling**: `--enable-cold-spilling` / `ENABLE_COLD_SPILLING` (false), `--cold-spilling-hot-window` / `COLD_SPILLING_HOT_WINDOW` (48h), `--cold-spilling-check-interval` / `COLD_SPILLING_CHECK_INTERVAL` (5m)
- **Health**: `--health-failure-threshold` / `HEALTH_FAILURE_THRESHOLD` (0.1)
- **History**: `--history-retention` / `EXECUTION_HISTORY_RETENTION` (720h)
- **Raft**: `--raft-advertise-host` / `RAFT_ADVERTISE_HOST`, `--raft-host` / `RAFT_HOST`, `--http-host` / `HTTP_HOST`
- **Observability**: `OTEL_EXPORTER_OTLP_ENDPOINT` (set to `disabled` to skip OTLP)
- **K8s**: `POD_IP` (overrides Raft advertise), `CLUSTER_SIZE` (for split-brain detection)
- **Split-brain**: Minority partition nodes exit with code 42 after 30s grace period (RA-style)