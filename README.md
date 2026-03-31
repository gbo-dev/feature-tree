# ft: featuretree

A lightweight Git worktree helper for bare-in-`.git` repositories, focused on feature-branch workflows.

## Dependencies

- **Go 1.24+** to build from source
- **Git** must be available on `$PATH`
- **bash or zsh** for shell integration (auto-`cd` on switch/create)

## Install

Build the binary and place it somewhere on your `$PATH`.

**Recommended location on Linux/macOS:** `$HOME/.local/bin/ft` (already on `$PATH` in most modern systems; create the directory if needed: `mkdir -p ~/.local/bin`).

**Build from source:**

```sh
git clone https://github.com/gbo-dev/ft.git
cd ft
go build -buildvcs=false -o ~/.local/bin/ft ./cmd/ft
```

> **Why `-buildvcs=false` in bare repos?**
> Go 1.18+ tries to embed VCS info (commit hash, dirty flag) into the binary by running `git` subprocesses from the module directory. Inside a bare-in-`.git` worktree, these calls fail because git's working-tree detection gets confused. Pass `-buildvcs=false` to disable VCS stamping and avoid the error. Not needed when building from a regular clone.

## Shell integration (auto-cd)

A Go binary cannot change its caller's working directory — this is an OS constraint that applies in any language. `ft` prints a `__FT_CD__=<path>` marker on stdout that the shell wrapper intercepts and turns into a `cd` call.

Add this line to your `~/.zshrc` or `~/.bashrc` (it evaluates the shell function once at shell startup):

```sh
eval "$(ft init)"
```

`ft init` auto-detects your shell from `$SHELL`. To be explicit:

```sh
eval "$(ft init zsh)"    # or bash
```

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

Create your first worktree (the default branch):

```sh
cd <repo>
git --git-dir=.git worktree add main main # TODO CHECK THIS
```

Then use `ft create <branch>` for any subsequent branches — it handles worktree creation and copies the include manifest automatically.

> **Note:** `git clone --bare` does not set up `origin/HEAD` or branch tracking entries by default. Run `git --git-dir=.git remote set-head origin --auto` after cloning so `ft` can auto-detect the default branch. A `ft clone` command that handles this automatically is planned (see below).

## Commands

| Command | Description |
|---|---|
| `ft clone <url> [dir]` | Clone a repo into bare-in-`.git` layout with an initial worktree |
| `ft switch [branch]` | Switch to an existing worktree; opens fzf picker if no branch given |
| `ft create <branch>` | Create a new branch + worktree |
| `ft list` | List worktrees with status |
| `ft remove [branch]` | Remove a worktree (and optionally its branch) |
| `ft squash [--base <branch>]` | Squash current branch commits into one |
| `ft copy-include` | Sync include-manifest files across worktrees |
| `ft init [bash\|zsh]` | Print shell integration snippet |
| `ft completion [bash\|zsh]` | Print tab-completion script |

Run `ft help` for flag details.

## Removal safety

`ft remove` blocks if:
- the worktree has uncommitted changes
- the branch has local commits not pushed to its upstream

Pass `-f` / `--force-worktree` to override. Pass `-D` / `--force-branch` to force-delete the branch regardless of merge status.

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
wt-shell/       original shell-script predecessor (wt + wt.zsh)
```

Future work notes:

- [Switch picker view paging notes](references/switch-view-paging-notes.md)

## Approach

- Bare-in-`.git` layout required (worktrees live alongside `.git/`)
- Default branch is auto-detected from `origin/HEAD`, then falls back to `main/master/trunk`
- Include manifest (`.worktreeinclude`) is copied from the default branch on worktree creation
- fzf is embedded via `github.com/junegunn/fzf/src` — no system fzf required

## License

MIT — see [LICENSE](LICENSE).
