# Switch Picker View Paging Notes

This note captures a possible future enhancement for `ft switch`: switching between multiple column views while the picker is open.

## Goal

Keep `BRANCH` visible at all times, but allow cycling secondary columns, for example:

- Page A: `PATH`, `STATE`, `COMMIT`
- Page B: `RELATION`, `STATE`, `COMMIT`

This helps when terminal width is constrained.

## Recommended approach

Use a small control loop that reopens fzf on explicit page-switch keys.

- Start with one page configuration.
- Run fzf with an expected page-switch key binding.
- On page-switch key: rebuild display lines/header for the next page and reopen fzf.
- Preserve query text and best-effort highlighted branch between iterations.

## Why loop/reopen is preferred

- fzf owns terminal input while running; external key listeners are unreliable.
- In-place external replacement of the rendered list is not a stable integration surface.
- Reopen-on-key is event-driven (not polling) and cheap for this workload.
- Keeps implementation simple and robust compared to concurrent stdin orchestration.

## Suggested key bindings

Prefer non-text keys to avoid collisions with search input.

- Recommended: `alt-left`, `alt-right`
- Acceptable: `ctrl-h`, `ctrl-l`
- Avoid by default: plain `h`/`l`

## Caveats and edge cases

- Query preservation: keep typed query across pages.
- Selection persistence: keep highlighted branch when possible.
- Narrow terminals: continue dynamic width fitting and truncation.
- Data freshness: decide between frozen snapshot vs recalculation on page switch.
- Discoverability: include short header hint, e.g. `Page 1/2 • alt-left/alt-right`.

## Scope

This is future work. Current implementation remains a single-page picker.
