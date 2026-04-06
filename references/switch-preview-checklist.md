# Switch Preview Tabs Checklist

This checklist tracks implementation of a stacked switch-picker preview with tabbed content.

## Scope

- Keep the existing `ft switch` fzf list as the primary picker.
- Add a stacked preview pane under the list.
- Add tab switching with `tab` / `shift-tab`.
- Include tabs for:
  1. HEAD+/- status
  2. Commit log (hash, +/-, age, message)
  3. Diff vs default branch
  4. Diff vs upstream tracking branch
- Skip AI summary for now.

## Cache Strategy (V1)

- Use an ephemeral per-session cache directory (`os.MkdirTemp`) created before launching fzf.
- Prefetch all tab payloads for all picker rows in parallel with bounded worker concurrency.
- Store each row/tab payload as a text file and pass hidden file paths in fzf payload fields.
- Render preview content by reading cached files from a hidden CLI command.
- Remove the cache directory when picker exits.

### Why this for V1

- Keeps fzf responsive while navigating rows/tabs.
- Avoids shelling out to heavy git commands on every cursor move.
- Avoids stale long-lived cache invalidation complexity.
- Easy to evolve into lazy fill later if startup cost is too high.

### Lazy cache location (future option)

- If we move to persisted lazy cache, use:
  - `$XDG_CACHE_HOME/ft/switch-preview/<repo-key>/...` when `XDG_CACHE_HOME` is set.
  - Fallback: `~/.cache/ft/switch-preview/<repo-key>/...`.
- `repo-key` should be a stable hash of `GitCommonDir`.

## Implementation Checklist

- [x] Add preview cache builder in `internal/tui` for switch rows/tabs.
- [x] Add commit-log parser/renderer for `+/-` totals per commit.
- [x] Add diff renderers for default branch and upstream branch tabs.
- [x] Add fzf preview configuration (`--preview-window=down,...`) and `tab` / `shift-tab` bindings.
- [x] Add hidden payload fields for tab file paths.
- [x] Update selected-branch parser to ignore extra hidden fields.
- [x] Add hidden CLI command to print cached preview files.
- [x] Add tests for branch parsing with extended payload.
- [x] Validate with `go test ./...`.
- [x] Update docs/help text after validating UX.

## Rollout Notes

- Start with `ft switch` only.
- Keep create/remove pickers unchanged.
- If startup latency is noticeable on large repos, next step is hybrid strategy:
  - eager tabs 1-2
  - lazy tabs 3-4 with on-demand file generation.
