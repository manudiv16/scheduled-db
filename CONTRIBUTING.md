# Contributing to Scheduled-DB

Thank you for considering contributing to Scheduled-DB! We welcome contributions of all kinds — bug reports, feature suggestions, documentation improvements, and code changes.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Environment](#development-environment)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Commit Conventions](#commit-conventions)
- [Pull Request Process](#pull-request-process)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

This project adheres to the [Contributor Covenant](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code. Report unacceptable behavior to the project maintainers.

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork:

   ```bash
   git clone git@github.com:YOUR_USERNAME/scheduled-db.git
   cd scheduled-db
   ```

3. **Add the upstream remote** to sync changes:

   ```bash
   git remote add upstream git@github.com:manudiv16/scheduled-db.git
   ```

4. **Create a branch** for your work:

   ```bash
   git checkout -b feat/my-feature
   ```

## Development Environment

### Prerequisites

- **Go** 1.23+ (toolchain 1.24.2 recommended)
- **Docker** and **Docker Compose** (for local cluster testing)
- **Golangci-lint** and **goimports** (install via `make install-deps`)

### Quick Setup

```bash
make install-deps  # install linting tools
make build         # verify the project compiles
make test          # run all tests
```

### Local Cluster

```bash
make dev-up        # start 3-node cluster with nginx load balancer
make create-jobs   # create test jobs
make test-proxy    # test write forwarding via LB
make test-failover # test leader failover
make dev-down      # teardown cluster
```

## Project Structure

```
├── cmd/
│   ├── scheduled-db/     # Main binary entrypoint
│   └── wasm/             # WASM simulator (browser)
├── internal/
│   ├── api/              # HTTP handlers + router
│   ├── e2e/              # End-to-end cluster tests
│   ├── logger/           # Structured logger
│   ├── metrics/          # Prometheus + OpenTelemetry
│   ├── simulator/        # WASM simulator internals
│   └── slots/            # Time-slotted job queue + worker
│   └── store/            # Raft consensus + FSM + state
├── k8s/                  # Kubernetes deployment manifests
├── docs/                 # WASM assets
└── Makefile              # Build, test, lint targets
```

## Making Changes

### Before You Start

1. Check [open issues](https://github.com/manudiv16/scheduled-db/issues) and [pull requests](https://github.com/manudiv16/scheduled-db/pulls) to avoid duplicate work.
2. For significant changes, open an issue first to discuss the approach.

### Workflow

1. Keep changes focused — one feature or fix per branch.
2. Write or update tests for your changes.
3. Ensure all tests pass (`make test`).
4. Run the linter (`make lint`) and formatter (`make fmt`).
5. Commit with a [conventional message](#commit-conventions).
6. Push and open a pull request.

## Coding Standards

- **Language**: Go 1.23+ with idiomatic Go style.
- **Formatting**: Run `make fmt` before committing (uses `gofmt` + `goimports`).
- **Linting**: Run `make lint` — the CI will reject unaddressed linter warnings.
- **Logging**: Always use the internal `scheduled-db/internal/logger` package. Never use `log` or `fmt` for logging.
- **Errors**: Always check errors explicitly with `if err != nil`. Return errors with `fmt.Errorf("context: %w", err)`. Never panic in production paths.
- **Naming**: Follow Go conventions (`MixedCaps`, not `snake_case`). Acronyms are uppercase (`HTTP`, `FSM`, `API`).
- **Imports**: Group as stdlib → internal → third-party, separated by blank lines.
- **Race safety**: `make test` runs with the Go race detector. Any detected race is treated as a bug.

### WASM-Specific Rules

- Files importing Raft, BoltDB, `net/http`, or Prometheus must have a `//go:build !wasm` build tag.
- Files under `cmd/wasm/` and `internal/simulator/` must be WASM-compatible.
- Simulator API functions use `syscall/js` for browser interop.

## Testing

```bash
make test         # full suite with race detector
make test-short   # skip slow tests (-short flag)
make bench        # run benchmarks
```

### Test Conventions

- Unit tests go next to the code, e.g., `fsm_test.go` for `fsm.go`.
- Property-based tests use `pgregory.net/rapid` and are named `*_property_test.go`.
- End-to-end tests live in `internal/e2e/` and require a running cluster.
- Integration tests (`make test-proxy`, `make test-failover`) require `make dev-up` first.

### Writing Tests

- Use `-short` to skip integration-heavy tests when the flag is set.
- Name tests clearly with `TestXxx` format.
- Use `t.Parallel()` for independent tests when safe.

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type       | Usage                              |
|------------|------------------------------------|
| `feat`     | A new feature                      |
| `fix`      | A bug fix                          |
| `docs`     | Documentation changes              |
| `refactor` | Code refactoring (no behavior change) |
| `test`     | Adding or updating tests           |
| `chore`    | Build, CI, tooling, dependencies   |
| `perf`     | Performance improvement            |
| `style`    | Formatting (no semantic change)    |

### Scope (optional)

Relevant package or area, e.g., `store`, `slots`, `api`, `discovery`, `wasm`, `docs`.

### Examples

```
feat(slots): add dry-run mode for job execution
fix(store): handle leader redirection during cluster shrink
docs: add API reference for job CRUD endpoints
test(wasm): add property tests for timing wheel
chore: bump golangci-lint to v1.60
```

## Pull Request Process

1. **Keep PRs focused** — prefer small, reviewable changes (< 400 lines).
2. **Draft PRs** are welcome for early feedback.
3. **Before requesting review**:
   - Ensure `make build && make test && make lint` passes.
   - Update documentation if your change affects the API or behavior.
   - Add or update tests to cover your changes.
4. **Address review feedback** with additional commits — we squash on merge.
5. **CI must pass** before merge.

### PR Title Format

Follow the same convention as commits:

```
feat(store): add cold spilling for historical slots
```

## Reporting Issues

- **Bug reports**: Use the GitHub issue tracker. Include the version, OS, steps to reproduce, expected behavior, and actual behavior.
- **Feature requests**: Describe the use case, proposed solution, and any alternatives considered.
- **Security vulnerabilities**: Do **not** open a public issue — follow the [security policy](SECURITY.md).

---

Thank you for contributing! 🚀
