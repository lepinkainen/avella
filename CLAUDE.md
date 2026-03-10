# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**Avella** — a lightweight file automation daemon in Go that watches directories, matches files against rules, and performs actions (move, exec, SCP upload). Config is YAML-based. Module: `github.com/lepinkainen/avella`.

## Build & Test Commands

```bash
task build          # Test + lint + build (use this as the main check)
task test           # Run tests (excludes llm-shared)
task lint           # goimports + go vet + golangci-lint
task clean          # Remove build artifacts
task build-ci       # Build only (no test/lint, for CI)
task test-ci        # Tests with coverage and -tags=ci
task build-linux    # Cross-compile for Linux amd64
```

Always use `task test` to run tests, unless running a single package: `go test ./actions/...`

Binary outputs to `build/avella`. Version injected via ldflags at build time.

## Architecture

Processing pipeline: **fsnotify event → skip temp files → wait for stable size → evaluate rules (first match wins) → execute actions**

| Package | Role |
|---------|------|
| `config/` | YAML config loading via Viper, path expansion (`~`), validation |
| `watcher/` | fsnotify wrapper, debounces events, hands off to stabilizer |
| `stabilizer/` | Polls file size until stable (2s interval, 3 checks). Skips `.part/.tmp/.crdownload/.download` |
| `rules/` | Pre-compiles regexes at init. `engine.Process()` evaluates rules, first match wins, dispatches actions |
| `actions/` | `Action` interface. `MoveAction` (rename, cross-device fallback), `ExecAction` (runs command with file as arg). SCP not yet implemented |
| `internal/pathutil/` | `ExpandHome()` for tilde expansion |

`main.go` wires everything together: kong CLI → config → engine → watcher → process loop. Graceful shutdown via SIGINT/SIGTERM context.

## Conventions

- **Build system**: Taskfile (not Makefile)
- **Formatting**: `goimports` (not `gofmt`) — the lint task runs this automatically
- **Logging**: `log/slog` with `github.com/lepinkainen/humanlog`
- **Config**: Viper for YAML loading
- **CLI**: `alecthomas/kong`
- **Errors**: Use `errors.Is()`/`errors.As()`, wrap with `%w`, sentinel errors for known cases
- **Shadow linting**: govet shadow checker is enabled — avoid variable shadowing in defer closures
- **llm-shared/**: Git submodule. Read-only. Excluded from tests and linting. Never modify.
