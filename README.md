# ft: feature-tree (WIP)

A simple git worktree layer that equates worktrees to branches to avoid using stash.
Repos cloned with `ft clone` are set up as bare, and focuses on parallel feature-branch workflows.

Inspired by [Worktrunk](https://github.com/max-sixty/worktrunk), where I wanted to explore Go, [fzf](https://pkg.go.dev/github.com/junegunn/fzf/src), and have something simple with few dependencies. Check out Worktrunk if you're unsure.

Built with OpenCode and GitHub Copilot, using mainly GPT-5.3 Codex.

## Dependencies

- **Go 1.24+** to build from source
- **Git**
- **bash or zsh** for shell integration (auto-`cd` on switch/create)

## Install

Build the binary and place it somewhere on your `$PATH`.

**Recommended location on Linux/macOS:** `$HOME/.local/bin/ft` (on `$PATH` in most modern systems)

**Build from source:**

```sh
go build -buildvcs=false -o ~/.local/bin/ft ./cmd/ft
```
If you have [just](https://github.com/casey/just) installed: 

```bash
just install
```

<details>
<summary>Why `-buildvcs=false` in bare repos?</summary>

> Go 1.18+ tries to embed VCS info (commit hash, dirty flag) into the binary by running `git` subprocesses from the module directory. Inside a bare-in-`.git` worktree, these calls fail because git's working-tree detection gets confused. Pass `-buildvcs=false` to disable VCS stamping and avoid the error. Not needed when building from a regular clone.

</details>

## Shell integration (auto-cd)

A Go binary cannot change its caller's working directory: this is an OS constraint that applies in any language. `ft` prints a `__FT_CD__=<path>` marker on stdout that the shell wrapper intercepts and turns into a `cd` call.

Add this line to your `~/.zshrc` or `~/.bashrc` (it evaluates the shell function once at shell startup):

```sh
eval "$(ft init zsh)"    # or bash
```

Or better yet, copy the output of `ft init <shell>` to your shell rc file.

Remember to open a new shell (or `source ~/.zshrc`) for it to take effect.

## Expected repository structure

`ft` requires a bare-in-`.git` layout. Clone a repo with:

```sh
git clone --bare https://github.com/org/<repo>.git <repo>/.git
```

This places the bare git contents inside `<repo>/.git/`. Worktrees (one per branch) then live alongside it:

```
repo/
  .git/          ← bare repo (fetch, objects, refs, config…)
  main/          ← worktree for branch 'main'
  my-feature/    ← worktree for branch 'my-feature'
```

Use `ft create <branch>` for any subsequent branches: it handles worktree creation and copies the include manifest automatically.

> [!TIP]
> Running `git clone --bare` or using worktrees natively does not set up `origin/HEAD` or branch tracking entries by default. Run `git --git-dir=.git remote set-head origin --auto` after cloning so `ft` can auto-detect the default branch. 
> 
> Or run `ft clone`, as it already handles this bootstrap automatically (see below).

## Commands

| Command | Description |
|---|---|
| `ft clone <url> [dir]` | Clone a repo into bare-in-`.git` layout with an initial worktree |
| `ft switch [--create] [--base <branch>] [branch]` | Switch to an existing worktree; optionally create missing worktree; opens fzf picker if no branch given (`tab`/`s-tab` preview tabs in picker) |
| `ft create [--all-branches] [--base <branch>] [branch]` | Create a branch worktree; picker opens only with `--all-branches` and no branch |
| `ft list` | List worktrees with status |
| `ft remove [branch]` | Remove a worktree (and optionally its branch) |
| `ft squash [--base <branch>]` | Squash current branch commits into one |
| `ft pr <num> [--use-pr-ref]` | Fetch and checkout a PR as a worktree (`--use-pr-ref` forces `pull/<num>` naming) |
| `ft copy-include [--from <branch>] [--to <branch>]` | Sync include-manifest files across worktrees |
| `ft init [bash\|zsh]` | Print shell integration snippet |
| `ft completion [bash\|zsh]` | Print tab-completion script |

Run `ft help` for flag details.

## Removal safety

`ft remove` blocks if the worktree has uncommitted changes or the branch has local commits not pushed to its upstream.

Pass `-f` / `--force-worktree` to override. Pass `-D` / `--force-branch` to force-delete the branch regardless of merge status.

## STATE values

In list and picker views, `STATE` is shown as:
- `clean` when there are no local changes
- `+` for staged changes
- `!` for unstaged changes
- `?` for untracked files
- combinations like `+!`, `!?`, or `+!?` when multiple apply

## `ft clone`

`ft clone` sets up the bare-in-`.git` layout in one step. `git clone --bare` alone leaves several things broken:

- `origin/HEAD` is not resolved → default-branch detection fails
- Branch tracking entries (`branch.<name>.remote/merge`) are not written → `git pull` fails from a worktree

`ft clone` fixes all of this and creates the initial worktree:

```sh
ft clone https://github.com/org/repo.git        # dir inferred from URL
ft clone https://github.com/org/repo.git mydir  # explicit dir
```

```
Cloned into /home/user/repo
Default branch: main
Worktree: /home/user/repo/main

cd /home/user/repo/main
```

The resulting structure:

```
repo/
  .git/    ← bare repo
  main/    ← initial worktree for the default branch
```

`.git/config` after clone:

```ini
[core]
    repositoryformatversion = 0
    filemode = true
    bare = true
[remote "origin"]
    url = https://github.com/org/repo.git
    fetch = +refs/heads/*:refs/remotes/origin/*
[branch "main"]
    remote = origin
    merge = refs/heads/main
```

## Repo layout

```
cmd/ft/         entry point
internal/
  cli/          cobra commands and tab-completion
  core/         worktree service logic
  gitx/         git subprocess helpers
  shell/        shell integration script generation
  tui/          embedded fzf picker
references/     design notes and option references
```

## Developer checks

Optional dead code analysis is available with `deadcode`:

```sh
go install golang.org/x/tools/cmd/deadcode@latest
deadcode ./...
deadcode -test ./...
```

Use `deadcode -test ./...` when you want test helpers counted as live; plain `deadcode ./...` reports only production-reachable code from main packages.

<!-- ## TODO

- Harden and verify the cancellation path end-to-end: Ctrl+C should cancel the Cobra command context and terminate in-flight git subprocesses immediately (not only via timeout), with integration coverage for long-running operations.
- Fix Unicode visible-width truncation/alignment issues in TUI rendering: current truncation mixes byte-based slicing with terminal-column assumptions, which can misalign rows or truncate incorrectly for wide glyphs, combining marks, emoji, and other multi-codepoint grapheme clusters (including cases where ellipsis width appears inconsistent across terminals/fonts).
-->

## Approach

- Bare-in-`.git` layout required (worktrees live alongside `.git/`)
- Default branch is auto-detected from `origin/HEAD`, then falls back to `main/master/trunk`
- Include manifest (`.worktreeinclude`) is copied from the default branch on worktree creation
- fzf is embedded via `github.com/junegunn/fzf/src`: no system fzf required
- Git command failures are normalized in `internal/gitx` (`CommandError`/`ExpectSuccess`); core logic wraps operation context, and CLI prints the final user-facing error once

## License

MIT: see [LICENSE](LICENSE).
