package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gbo-dev/wt/go-port/internal/gitx"
)

type Service struct {
	Ctx *gitx.RepoContext
}

type ListRow struct {
	Marker   string
	Branch   string
	Path     string
	State    string
	Relation string
	Commit   gitx.CommitInfo
	Current  bool
}

func NewService() (*Service, error) {
	ctx, err := gitx.DiscoverRepoContext()
	if err != nil {
		return nil, err
	}
	return &Service{Ctx: ctx}, nil
}

func (s *Service) ResolveBranchShortcut(value string) (string, error) {
	switch value {
	case "^":
		return s.Ctx.DefaultBranch, nil
	case "@":
		current, err := gitx.CurrentBranch("")
		if err != nil {
			return "", fmt.Errorf("ft: HEAD is detached; @ is unavailable")
		}
		return current, nil
	default:
		return value, nil
	}
}

func SanitizeBranchName(branch string) string {
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = strings.ReplaceAll(branch, "\\", "-")
	return branch
}

func FindWorktreePath(entries []gitx.Worktree, branch string) string {
	for _, wt := range entries {
		if wt.Branch == branch {
			return wt.Path
		}
	}
	return ""
}

func (s *Service) ListRows() ([]ListRow, error) {
	entries, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return nil, err
	}

	branches := make([]string, len(entries))
	for i, wt := range entries {
		branches[i] = wt.Branch
	}
	commits := gitx.FetchCommitsParallel(s.Ctx, branches)

	current, _ := gitx.CurrentBranch("")
	rows := make([]ListRow, 0, len(entries))

	for i, wt := range entries {
		marker := " "
		isCurrent := wt.Branch == current
		if wt.Branch == current {
			marker = "@"
		} else if wt.Branch == s.Ctx.DefaultBranch {
			marker = "^"
		}

		dirty, err := gitx.DirtySymbols(wt.Path)
		if err != nil {
			dirty = "?"
		}

		relation, err := gitx.BranchRelation(s.Ctx, wt.Branch)
		if err != nil {
			relation = "?"
		}

		rows = append(rows, ListRow{
			Marker:   marker,
			Branch:   wt.Branch,
			Path:     wt.Path,
			State:    gitx.DirtyState(dirty),
			Relation: relation,
			Commit:   commits[i],
			Current:  isCurrent,
		})
	}

	return rows, nil
}

type CreateResult struct {
	Path     string
	Created  bool
	Branch   string
	FromBase string
}

func (s *Service) CreateWorktree(branch string, baseBranch string) (*CreateResult, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, fmt.Errorf("ft: branch name is required")
	}
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = s.Ctx.DefaultBranch
	}

	resolvedBase, err := s.ResolveBranchShortcut(baseBranch)
	if err != nil {
		return nil, err
	}

	entries, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return nil, err
	}

	if existing := FindWorktreePath(entries, branch); existing != "" {
		return &CreateResult{Path: existing, Created: false, Branch: branch, FromBase: resolvedBase}, nil
	}

	safeBranch := SanitizeBranchName(branch)
	worktreePath := filepath.Join(s.Ctx.RepoRoot, safeBranch)

	for _, wt := range entries {
		if wt.Path == worktreePath && wt.Branch != branch {
			return nil, fmt.Errorf("ft: path collision: %q maps to %s, already used by %q", branch, worktreePath, wt.Branch)
		}
	}

	if _, err := os.Stat(worktreePath); err == nil {
		return nil, fmt.Errorf("ft: target path already exists: %s", worktreePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("ft: inspect target path: %w", err)
	}

	branchExists, err := gitx.BranchExistsLocal(s.Ctx, branch)
	if err != nil {
		return nil, err
	}

	if branchExists {
		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "worktree", "add", worktreePath, branch)
		if runErr != nil {
			return nil, fmt.Errorf("ft: create worktree: %w", runErr)
		}
		if exitCode != 0 {
			if stderr == "" {
				stderr = "git worktree add failed"
			}
			return nil, fmt.Errorf("ft: %s", stderr)
		}
	} else {
		baseExists, err := gitx.BranchExistsLocal(s.Ctx, resolvedBase)
		if err != nil {
			return nil, err
		}
		if !baseExists {
			return nil, fmt.Errorf("ft: base branch not found locally: %s", resolvedBase)
		}

		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "worktree", "add", "-b", branch, worktreePath, resolvedBase)
		if runErr != nil {
			return nil, fmt.Errorf("ft: create worktree: %w", runErr)
		}
		if exitCode != 0 {
			if stderr == "" {
				stderr = "git worktree add failed"
			}
			return nil, fmt.Errorf("ft: %s", stderr)
		}
	}

	if branch != s.Ctx.DefaultBranch {
		if err := s.CopyIncludeBetweenBranches(s.Ctx.DefaultBranch, branch); err != nil {
			return nil, err
		}
	}

	return &CreateResult{Path: worktreePath, Created: true, Branch: branch, FromBase: resolvedBase}, nil
}

type SwitchResult struct {
	Path      string
	Branch    string
	Created   bool
	FromBase  string
	DidSwitch bool
}

type RemoveResult struct {
	Branch         string
	Path           string
	FallbackPath   string
	TargetRef      string
	DeletedMerged  bool
	DeletedForced  bool
	KeptBranch     bool
	NoDeleteBranch bool
}

func (s *Service) Switch(branch string, createIfMissing bool, baseBranch string) (*SwitchResult, error) {
	resolvedBranch, err := s.ResolveBranchShortcut(branch)
	if err != nil {
		return nil, err
	}

	entries, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return nil, err
	}

	if path := FindWorktreePath(entries, resolvedBranch); path != "" {
		return &SwitchResult{Path: path, Branch: resolvedBranch, DidSwitch: true}, nil
	}

	if !createIfMissing {
		return nil, fmt.Errorf("ft: branch %q has no worktree (use ft create %s or ft switch --create %s)", resolvedBranch, resolvedBranch, resolvedBranch)
	}

	result, err := s.CreateWorktree(resolvedBranch, baseBranch)
	if err != nil {
		return nil, err
	}

	return &SwitchResult{
		Path:      result.Path,
		Branch:    result.Branch,
		Created:   result.Created,
		FromBase:  result.FromBase,
		DidSwitch: true,
	}, nil
}

func (s *Service) RemoveWorktree(branch string, forceWorktree bool, forceBranch bool, noDeleteBranch bool) (*RemoveResult, error) {
	if noDeleteBranch && forceBranch {
		return nil, fmt.Errorf("ft: cannot use --force-branch with --no-delete-branch")
	}

	resolvedBranch, err := s.ResolveBranchShortcut(branch)
	if err != nil {
		return nil, err
	}

	entries, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return nil, err
	}

	var targetWorktree *gitx.Worktree
	for i := range entries {
		if entries[i].Branch == resolvedBranch {
			targetWorktree = &entries[i]
			break
		}
	}
	if targetWorktree == nil {
		return nil, fmt.Errorf("ft: no worktree found for branch %q", resolvedBranch)
	}

	if resolvedBranch == s.Ctx.DefaultBranch {
		return nil, fmt.Errorf("ft: refusing to remove default branch worktree %q", s.Ctx.DefaultBranch)
	}

	if strings.TrimSpace(targetWorktree.LockedReason) != "" {
		return nil, fmt.Errorf("ft: worktree is locked: %s (%s)", targetWorktree.Path, targetWorktree.LockedReason)
	}

	result := &RemoveResult{
		Branch:         resolvedBranch,
		Path:           targetWorktree.Path,
		NoDeleteBranch: noDeleteBranch,
	}

	currentBranch, err := gitx.CurrentBranch("")
	if err == nil && currentBranch == resolvedBranch {
		for _, wt := range entries {
			if wt.Branch == s.Ctx.DefaultBranch {
				result.FallbackPath = wt.Path
				break
			}
		}
		if result.FallbackPath == "" {
			return nil, fmt.Errorf("ft: default branch worktree %q not found", s.Ctx.DefaultBranch)
		}
	}

	targetRef, err := s.deletionTargetRef()
	if err != nil {
		return nil, err
	}
	result.TargetRef = targetRef

	if !forceWorktree {
		if err := s.ensureWorktreeSafeToRemove(targetWorktree.Path, resolvedBranch, targetRef); err != nil {
			return nil, err
		}
	}

	removeArgs := []string{"worktree", "remove"}
	if forceWorktree {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, targetWorktree.Path)

	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, removeArgs...)
	if runErr != nil {
		return nil, fmt.Errorf("ft: remove worktree: %w", runErr)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "git worktree remove failed"
		}
		return nil, fmt.Errorf("ft: %s", stderr)
	}

	if noDeleteBranch {
		return result, nil
	}

	deletable, err := s.branchDeletable(resolvedBranch, targetRef)
	if err != nil {
		return nil, err
	}

	if deletable {
		_, stderr, exitCode, runErr = gitx.RunGitCommon(s.Ctx, "branch", "-d", resolvedBranch)
		if runErr != nil {
			return nil, fmt.Errorf("ft: delete branch %q: %w", resolvedBranch, runErr)
		}
		if exitCode != 0 {
			if stderr == "" {
				stderr = "git branch -d failed"
			}
			return nil, fmt.Errorf("ft: %s", stderr)
		}
		result.DeletedMerged = true
		return result, nil
	}

	if forceBranch {
		_, stderr, exitCode, runErr = gitx.RunGitCommon(s.Ctx, "branch", "-D", resolvedBranch)
		if runErr != nil {
			return nil, fmt.Errorf("ft: force delete branch %q: %w", resolvedBranch, runErr)
		}
		if exitCode != 0 {
			if stderr == "" {
				stderr = "git branch -D failed"
			}
			return nil, fmt.Errorf("ft: %s", stderr)
		}
		result.DeletedForced = true
		return result, nil
	}

	result.KeptBranch = true
	return result, nil
}

func (s *Service) deletionTargetRef() (string, error) {
	stdout, _, _, err := gitx.RunGitCommon(s.Ctx, "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+s.Ctx.DefaultBranch)
	if err != nil {
		return "", err
	}

	upstream := ""
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			upstream = line
			break
		}
	}

	if upstream != "" {
		aheadCount, err := s.revListCount(s.Ctx.DefaultBranch + ".." + upstream)
		if err == nil && aheadCount > 0 {
			return upstream, nil
		}
	}

	return s.Ctx.DefaultBranch, nil
}

func (s *Service) branchDeletable(branch string, target string) (bool, error) {
	_, _, exitCode, err := gitx.RunGitCommon(s.Ctx, "merge-base", "--is-ancestor", branch, target)
	if err != nil {
		return false, err
	}
	if exitCode == 0 {
		return true, nil
	}

	_, _, exitCode, err = gitx.RunGitCommon(s.Ctx, "diff", "--quiet", target+"..."+branch)
	if err != nil {
		return false, err
	}
	if exitCode == 0 {
		return true, nil
	}

	branchTree, _, exitCode, err := gitx.RunGitCommon(s.Ctx, "rev-parse", "refs/heads/"+branch+"^{tree}")
	if err != nil {
		return false, err
	}
	if exitCode != 0 {
		return false, nil
	}

	targetTree, _, exitCode, err := gitx.RunGitCommon(s.Ctx, "rev-parse", target+"^{tree}")
	if err != nil {
		return false, err
	}
	if exitCode != 0 {
		return false, nil
	}

	return strings.TrimSpace(branchTree) == strings.TrimSpace(targetTree), nil
}

func (s *Service) ensureWorktreeSafeToRemove(path string, branch string, targetRef string) error {
	clean, err := isWorktreeClean(path)
	if err != nil {
		return err
	}
	if !clean {
		return fmt.Errorf("ft: worktree is dirty: %s (commit/stash/remove changes first, or use --force-worktree)", path)
	}

	upstream, err := worktreeUpstreamRef(path)
	if err != nil {
		return err
	}
	if upstream == "" {
		deletable, err := s.branchDeletable(branch, targetRef)
		if err != nil {
			return err
		}
		if deletable {
			return nil
		}
		return fmt.Errorf("ft: branch %q has no upstream tracking branch and differs from %s; push first, or use --force-worktree", branch, targetRef)
	}

	ahead, stderr, exitCode, runErr := gitx.RunGit(path, "rev-list", "--count", upstream+"..HEAD")
	if runErr != nil {
		return fmt.Errorf("ft: compare branch %q to upstream %q: %w", branch, upstream, runErr)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "failed to compare branch to upstream"
		}
		return fmt.Errorf("ft: %s", stderr)
	}

	aheadCount, err := strconv.Atoi(strings.TrimSpace(ahead))
	if err != nil {
		return fmt.Errorf("ft: failed to compare branch %q to upstream %q", branch, upstream)
	}

	if aheadCount > 0 {
		deletable, err := s.branchDeletable(branch, targetRef)
		if err != nil {
			return err
		}
		if deletable {
			return nil
		}
		return fmt.Errorf("ft: branch %q has commits not pushed to %s; push first, or use --force-worktree", branch, upstream)
	}

	return nil
}

func isWorktreeClean(path string) (bool, error) {
	stdout, stderr, exitCode, err := gitx.RunGit(path, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return false, fmt.Errorf("ft: inspect worktree clean state: %w", err)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "git status failed"
		}
		return false, fmt.Errorf("ft: %s", stderr)
	}
	return strings.TrimSpace(stdout) == "", nil
}

func worktreeUpstreamRef(path string) (string, error) {
	stdout, _, exitCode, err := gitx.RunGit(path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		return "", fmt.Errorf("ft: resolve upstream for worktree %s: %w", path, err)
	}
	if exitCode != 0 {
		return "", nil
	}
	return strings.TrimSpace(stdout), nil
}

func (s *Service) revListCount(rangeExpr string) (int, error) {
	stdout, stderr, exitCode, err := gitx.RunGitCommon(s.Ctx, "rev-list", "--count", rangeExpr)
	if err != nil {
		return 0, err
	}
	if exitCode != 0 {
		if stderr != "" {
			return 0, fmt.Errorf("ft: %s", stderr)
		}
		return 0, fmt.Errorf("ft: failed to count revisions for %s", rangeExpr)
	}
	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("ft: failed to parse revision count for %s", rangeExpr)
	}
	return count, nil
}

func (s *Service) CopyIncludeBetweenBranches(fromBranch string, toBranch string) error {
	fromEntries, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return err
	}

	fromPath := FindWorktreePath(fromEntries, fromBranch)
	if fromPath == "" {
		return fmt.Errorf("ft: no worktree found for branch %q", fromBranch)
	}

	toPath := FindWorktreePath(fromEntries, toBranch)
	if toPath == "" {
		return fmt.Errorf("ft: no worktree found for branch %q", toBranch)
	}

	if fromPath == toPath {
		return nil
	}

	manifestPath := filepath.Join(fromPath, s.Ctx.IncludeFile)
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ft: read include file %s: %w", manifestPath, err)
	}
	defer manifestFile.Close()

	scanner := bufio.NewScanner(manifestFile)
	for scanner.Scan() {
		raw := scanner.Text()
		pattern := raw
		if idx := strings.Index(pattern, "#"); idx >= 0 {
			pattern = pattern[:idx]
		}
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = strings.TrimPrefix(pattern, "/")

		matches, err := filepath.Glob(filepath.Join(fromPath, pattern))
		if err != nil {
			return fmt.Errorf("ft: parse include pattern %q: %w", pattern, err)
		}

		for _, match := range matches {
			rel, err := filepath.Rel(fromPath, match)
			if err != nil {
				return fmt.Errorf("ft: compute include relative path: %w", err)
			}
			if strings.HasPrefix(rel, "..") {
				continue
			}

			dest := filepath.Join(toPath, rel)
			if err := copyPreservingShape(match, dest); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ft: read include file %s: %w", manifestPath, err)
	}

	return nil
}

func copyPreservingShape(src string, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("ft: inspect include source %s: %w", src, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("ft: read symlink %s: %w", src, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("ft: create include destination parent: %w", err)
		}
		_ = os.Remove(dst)
		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("ft: create symlink %s: %w", dst, err)
		}
		return nil
	}

	if info.IsDir() {
		return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := dst
			if rel != "." {
				target = filepath.Join(dst, rel)
			}

			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}

			if d.Type()&os.ModeSymlink != 0 {
				lnk, err := os.Readlink(path)
				if err != nil {
					return err
				}
				_ = os.Remove(target)
				return os.Symlink(lnk, target)
			}

			return copyFile(path, target)
		})
	}

	return copyFile(src, dst)
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("ft: open include source file %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("ft: stat include source file %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ft: create include destination parent: %w", err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("ft: open include destination file %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("ft: copy %s -> %s: %w", src, dst, err)
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("ft: sync include destination file %s: %w", dst, err)
	}

	return nil
}
