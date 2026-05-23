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

## Architectural Constitution
- Keep command layers thin: `internal/cli` parses args, handles TTY/pickers/output, and delegates behavior to `internal/core`.
- Keep business decisions in `core.Service`; return result structs plus errors, not pre-rendered CLI text.
- Route git subprocesses through `internal/gitx`; use `CommandError`/`ExpectSuccess` so stderr, exit codes, cancellation, and context are normalized in one place.
- Propagate `cmd.Context()` through every operation. Do not introduce background git work that escapes command cancellation.
- Surface failures by returning contextual errors. Cobra stays `SilenceErrors`/`SilenceUsage`, and `cmd/ft/main.go` prints the final `ft: error: ...` once.
- Keep warnings and hints separate from fatal errors: return them on result structs or write explicit non-fatal notes to stderr at the CLI edge.
- Preserve the repo model: bare-in-`.git` root, sibling worktrees named from sanitized branch names, default branch from `origin/HEAD` with local fallbacks, and `.worktreeinclude` copied from the default worktree on creation.
- Preserve safety checks around destructive commands; require clean worktrees, pushed/merged/equivalent branches, or explicit force flags before removal/squash behavior proceeds.
- Keep auto-`cd` shell integration marker-based via `internal/shell.EmitCDOrWarning`; the Go binary never tries to change the parent shell directory directly.

## Maintainability Watchouts
- If `gitx` call sites keep expanding, prefer a small command-result abstraction over more direct `stdout, stderr, exitCode, err` handling.
- Let `core.Service` stay an orchestration layer; add narrow collaborators before growing broad IO, logging, or test-hook fields.
- Keep `internal/tui` presentation-focused; shared branch/worktree decisions belong in `internal/core` or pure helpers.

## Deadcode Note
- deadcode without test roots (example: deadcode ./...) can report test helpers as unreachable.
- This repo uses deadcode -test ./... in just deadcode so test-only helpers are treated correctly.

## Expected Pre-Completion Checks
- Run `just check` before finishing code changes.
- Run `just race` when touching concurrency-sensitive behavior.
