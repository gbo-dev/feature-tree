# legacy/wt (archived)

This directory is a legacy shell-script predecessor kept for reference only.

- Development focus is the Go CLI (`ft`) in this repository.
- The scripts here are not the source of truth for behavior and may diverge.
- New features and fixes should be implemented in the Go code, not in this folder.

`wt` is a lightweight Git worktree helper script for repositories that use a bare-in-`.git` layout.

## What it is used for

`wt` wraps common worktree workflows so you can quickly:

- switch between existing branch worktrees
- create a new branch + worktree from a base branch
- list worktrees with basic status indicators
- remove worktrees (optionally deleting the branch)
- squash your current branch into a single commit
- ensure bare-repo remote tracking refs are configured
- copy include-defined files from one worktree to another

The command was inspired by [worktrunk](https://github.com/max-sixty/worktrunk).

## Usage

Clone and bootstrap a repository (bare-in-`.git` + initial worktree):

```zsh
./wt clone https://github.com/VolvoGroup-Internal/<repo-name>.git
```

This creates the structure:

```text
<repo-name>/
  .git/      # bare repository
  <default-branch>/
```

Then move into the worktree you want to use (for example the default branch) before making commits:

```zsh
cd <repo-name>/<default-branch>
```

New worktrees are created as sibling directories under `<repo-name>/`, named after the branch.

Run `./wt help` to see all commands.

If you want directory switching in your current shell session, source the Zsh wrapper:

```zsh
[[ -f /path/to/repo/wt.zsh ]] && source /path/to/repo/wt.zsh
```

Then use:

```zsh
wt clone https://github.com/VolvoGroup-Internal/<repo-name>.git
wt switch
wt create my-feature
wt list
wt remove my-feature
wt setup
```

`wt` now auto-ensures this fetch refspec exists (for bare-in-`.git` setups):

```ini
[remote "origin"]
  fetch = +refs/heads/*:refs/remotes/origin/*
```

You can also run `wt setup` explicitly to enforce it and fetch `origin`.

`wt clone` runs this setup automatically on the cloned bare repo.

`wt remove` behavior:

- no argument while on a non-default branch worktree: removes the current worktree
- no argument while on the default branch worktree: opens an interactive picker
- with an explicit branch argument: removes that specific worktree
- safety check: target worktree must be clean; unpushed/no-upstream commits block removal unless branch content is already integrated with the default target
- safety check: locked worktrees are refused
- safe branch deletion includes content-equivalent trees (not only direct ancestry)

`wt list` columns:

- `DIRTY`: `+` staged, `!` unstaged, `?` untracked, or `clean`
- `VS <default-branch>`: commits `↑ahead ↓behind` compared to the default branch (for example, `VS main`)

## Current defaults

For now, runtime options are baked into the script:

- default branch: auto-detected from `origin/HEAD`, then current branch, else `main`
- include manifest file: `.worktreeinclude`

## Improvements / suggestions

Potential next steps:

- reintroduce optional config file support once settings stabilize
- add shell completion for commands and branch names
- add a small test harness for command parsing and helper functions
- add CI checks for shell syntax and basic behavior
