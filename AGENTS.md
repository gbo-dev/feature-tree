# AGENTS

## Repo Snapshot
- Language: Go.
- CLI entrypoint: cmd/ft/main.go.
- Main areas:
  - internal/cli: command wiring and integration behavior.
  - internal/core: core feature-tree logic.
  - internal/gitx: git command wrappers and repo context.
  - internal/tui: picker and preview rendering.

## Working Norms
- Keep changes small and readable; prefer straightforward, clean, idiomatic Go.

## Justfile
- Format: just fmt
- Unit/integration tests: just test
- Lint: just lint
- Dead code: just deadcode
- Full local gate: just check
- Race checks (slower): just race

## Deadcode Note
- deadcode without test roots (example: deadcode ./...) can report test helpers as unreachable.
- This repo uses deadcode -test ./... in just deadcode so test-only helpers are treated correctly.

## Expected Pre-Completion Checks
- Run just check before finishing code changes.
- Run just race when touching concurrency-sensitive behavior.
