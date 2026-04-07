---
name: feature-tree-skill
description: Use the `ft` (feature-tree) CLI to run worktree-per-branch Git workflows in bare-in-`.git` repositories. Covers clone/bootstrap, worktree listing, worktree switch/create/remove, PR checkout, include-manifest sync, squash, and shell integration. Use when the user mentions `ft`, feature-tree, or managing Git worktrees in this layout.
compatibility: Requires `ft` and `git`; target repository must use bare-in-`.git` layout.
---

# feature-tree (`ft`) skill

Use this skill when a task is about the `ft` CLI in this repository.

`ft` manages one worktree per branch in a **bare-in-`.git`** repo layout:

```text
repo/
  .git/         # bare common git dir
  main/         # default-branch worktree
  my-feature/   # feature branch worktree
```

## Quick operating rules

1. Run `ft` from inside a worktree in a bare-in-`.git` repo.
2. Branch shortcuts:
   - `^` = default branch
   - `@` = current branch (unavailable on detached HEAD)
3. Auto-directory switching requires shell integration:
   - `eval "$(ft init zsh)"` (or `bash`)
4. Interactive pickers require a TTY.

## Command reference

### `ft clone <url> [dir]`

- Clones a remote into `dir/.git` as a bare repo and creates the first worktree for the detected default branch.
- If `dir` is omitted, it is inferred from the repo URL.
- Also bootstraps `origin/HEAD` and branch tracking so later `git pull` in worktrees works.
- Prints repo root, default branch, worktree path, and a `cd` hint.

### `ft list`

- Lists known worktrees in a table-like view.
- Markers:
  - `@` = current branch worktree
  - `^` = default branch worktree
- `STATE` values:
  - `clean` or `dirty` with symbols
  - `+` staged, `!` unstaged, `?` untracked (combined when multiple apply)
- `RELATION` is relative to default branch:
  - `A: <n>` commits ahead
  - `B: <n>` commits behind

### `ft switch [branch]`

- Switches to an existing worktree branch.
- Without `branch`, opens an interactive picker (TTY required).
- If no worktree exists for a branch, it errors unless `--create` is used.
- Flags:
  - `-c, --create`: create missing worktree while switching
  - `-b, --base <branch>`: base branch used with `--create`

### `ft create [branch]`

- Creates a worktree for `branch`.
- If local branch exists, attaches a worktree to it.
- If local branch does not exist, creates it from base branch (default: detected default branch).
- If the worktree already exists, returns `Already exists`.
- For non-default branches, copies include-manifest files from default branch worktree.
- Flags:
  - `-b, --base <branch>`: base branch for new branch creation
  - `-a, --all-branches`: when `branch` is omitted, open picker that includes local branches without worktrees (TTY required)

### `ft pr <num>`

- Fetches PR refs from `origin` and creates/switches to worktree branch `pull/<num>`.
- Reuses the worktree if it already exists.
- Useful for quickly checking out PRs without disturbing current branch worktree.

### `ft copy-include [--from <branch>] [--to <branch>]`

- Copies files defined by `.worktreeinclude` patterns from one worktree to another.
- Defaults:
  - `--from`: default branch
  - `--to`: current branch
- Supports branch shortcuts (`^`, `@`).
- Manifest behavior:
  - Empty lines and `#` comments are ignored
  - Leading `/` in patterns is ignored
  - Glob patterns are expanded in source worktree
  - Files, directories, and symlinks are copied preserving shape

### `ft squash [--base <branch>]`

- Squashes commits on current branch since merge-base with base branch into one commit.
- Defaults base branch to detected default branch.
- Preconditions:
  - Current branch must not equal base branch
  - Base branch must exist locally
  - Working tree must be clean
  - At least 2 commits ahead of base
- Writes a generated commit message that lists squashed commit subjects.
- This rewrites branch history (force-push may be needed later if already pushed).

### `ft remove [branch]`

- Removes a worktree and optionally deletes its branch.
- Default branch worktree cannot be removed.
- If `branch` omitted:
  - Uses current branch when not on default
  - On default branch with TTY, opens picker for removable branches
- Safety checks (unless forced):
  - Blocks dirty worktrees
  - Blocks branches with unpushed commits
- Branch deletion behavior:
  - Deletes safely when merged/identical/equivalent to target ref
  - Keeps branch otherwise unless forced
- Flags:
  - `-f, --force-worktree` (or `--force`): remove even if dirty/unpushed safeguards fail
  - `-D, --force-branch`: force-delete branch
  - `--no-delete-branch`: remove worktree only, keep branch

### `ft init [bash|zsh]`

- Prints shell wrapper that enables automatic `cd` after `ft switch`, `ft create`, and `ft pr`.
- Add once to shell config, or copy output of `ft init <shell>` directly to `.bashrc`/`.zshrc`:

```sh
eval "$(ft init zsh)"
```

- `ft` binaries cannot change parent shell cwd directly; wrapper handles the `__FT_CD__=` marker.

### `ft completion [bash|zsh]`

- Prints completion definitions only.
- Example setup:

```sh
ft completion zsh > ~/.ft-completion.zsh
source ~/.ft-completion.zsh
```

## Common workflows

### Bootstrap repo

```sh
ft clone https://github.com/org/repo.git
cd repo/main
eval "$(ft init zsh)"
```

### Start a feature branch

```sh
ft create feat/my-change
```

### Move between branches

```sh
ft switch            # picker
ft switch my-branch  # direct
```

### Remove completed work

```sh
ft remove my-branch
```

## Failure patterns and fixes

- `only bare-in-.git repositories are supported`
  - Run in correct layout or bootstrap with `ft clone`.
- `could not determine default branch`
  - Run `git --git-dir=.git remote set-head origin --auto`.
- `HEAD is detached` / `@ is unavailable`
  - Check out a branch first; do not use `@` on detached HEAD.
- `no interactive TTY available`
  - Pass explicit branch arguments instead of relying on picker UI.
