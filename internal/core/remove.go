package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func (s *Service) RemoveWorktree(branch string, forceWorktree bool, forceBranch bool, noDeleteBranch bool) (*RemoveResult, error) {
	if noDeleteBranch && forceBranch {
		return nil, fmt.Errorf("ft: cannot use --force-branch with --no-delete-branch")
	}

	resolvedBranch, err := s.ResolveBranchShortcut(branch)
	if err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(s.Ctx)
	if err != nil {
		return nil, err
	}

	var target *gitx.Worktree
	for i := range worktrees {
		if worktrees[i].Branch == resolvedBranch {
			target = &worktrees[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("ft: no worktree found for branch %q", resolvedBranch)
	}

	if resolvedBranch == s.Ctx.DefaultBranch {
		return nil, fmt.Errorf("ft: refusing to remove default branch worktree %q", s.Ctx.DefaultBranch)
	}

	if strings.TrimSpace(target.LockedReason) != "" {
		return nil, fmt.Errorf("ft: worktree is locked: %s (%s)", target.Path, target.LockedReason)
	}

	result := &RemoveResult{
		Branch:         resolvedBranch,
		Path:           target.Path,
		NoDeleteBranch: noDeleteBranch,
	}

	currentBranch, err := gitx.CurrentBranch("")
	if err == nil && currentBranch == resolvedBranch {
		for _, worktree := range worktrees {
			if worktree.Branch == s.Ctx.DefaultBranch {
				result.FallbackPath = worktree.Path
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
		if err := s.ensureWorktreeSafeToRemove(target.Path, resolvedBranch, targetRef); err != nil {
			return nil, err
		}
	}

	removeArgs := []string{"worktree", "remove"}
	if forceWorktree {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, target.Path)

	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, removeArgs...)
	if err := gitx.CommandError("remove worktree", stderr, exitCode, runErr, "git worktree remove failed"); err != nil {
		return nil, err
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
		if err := gitx.CommandError(fmt.Sprintf("delete branch %q", resolvedBranch), stderr, exitCode, runErr, "git branch -d failed"); err != nil {
			return nil, err
		}
		result.DeletedMerged = true
		return result, nil
	}

	if forceBranch {
		_, stderr, exitCode, runErr = gitx.RunGitCommon(s.Ctx, "branch", "-D", resolvedBranch)
		if err := gitx.CommandError(fmt.Sprintf("force delete branch %q", resolvedBranch), stderr, exitCode, runErr, "git branch -D failed"); err != nil {
			return nil, err
		}
		result.DeletedForced = true
		return result, nil
	}

	result.KeptBranch = true
	return result, nil
}

func (s *Service) deletionTargetRef() (string, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+s.Ctx.DefaultBranch)
	stdout, err := gitx.ExpectSuccess("resolve default branch upstream", stdout, stderr, exitCode, runErr, "failed to resolve default branch upstream")
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
	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "merge-base", "--is-ancestor", branch, target)
	if runErr != nil {
		return false, gitx.CommandError(fmt.Sprintf("check ancestry for %q and %q", branch, target), stderr, exitCode, runErr, "git merge-base failed")
	}
	if exitCode == 0 {
		return true, nil
	}
	if exitCode != 1 {
		return false, gitx.CommandError(fmt.Sprintf("check ancestry for %q and %q", branch, target), stderr, exitCode, nil, "git merge-base failed")
	}

	_, stderr, exitCode, runErr = gitx.RunGitCommon(s.Ctx, "diff", "--quiet", target+"..."+branch)
	if runErr != nil {
		return false, gitx.CommandError(fmt.Sprintf("compare %q and %q", target, branch), stderr, exitCode, runErr, "git diff failed")
	}
	if exitCode == 0 {
		return true, nil
	}
	if exitCode != 1 {
		return false, gitx.CommandError(fmt.Sprintf("compare %q and %q", target, branch), stderr, exitCode, nil, "git diff failed")
	}

	branchTree, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "rev-parse", "refs/heads/"+branch+"^{tree}")
	if runErr != nil {
		return false, gitx.CommandError(fmt.Sprintf("resolve tree for branch %q", branch), stderr, exitCode, runErr, "git rev-parse failed")
	}
	if exitCode != 0 {
		return false, nil
	}

	targetTree, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "rev-parse", target+"^{tree}")
	if runErr != nil {
		return false, gitx.CommandError(fmt.Sprintf("resolve tree for target %q", target), stderr, exitCode, runErr, "git rev-parse failed")
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
	if err := gitx.CommandError(fmt.Sprintf("compare branch %q to upstream %q", branch, upstream), stderr, exitCode, runErr, "failed to compare branch to upstream"); err != nil {
		return err
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
	stdout, stderr, exitCode, runErr := gitx.RunGit(path, "status", "--porcelain", "--untracked-files=normal")
	if err := gitx.CommandError("inspect worktree clean state", stderr, exitCode, runErr, "git status failed"); err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout) == "", nil
}

func worktreeUpstreamRef(path string) (string, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGit(path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if runErr != nil {
		return "", gitx.CommandError(fmt.Sprintf("resolve upstream for worktree %s", path), stderr, exitCode, runErr, "git rev-parse failed")
	}
	if exitCode == 0 {
		return strings.TrimSpace(stdout), nil
	}

	if isNoUpstreamConfigured(stderr) {
		return "", nil
	}

	return "", gitx.CommandError(fmt.Sprintf("resolve upstream for worktree %s", path), stderr, exitCode, nil, "git rev-parse failed")
}

func isNoUpstreamConfigured(stderr string) bool {
	message := strings.ToLower(strings.TrimSpace(stderr))
	if message == "" {
		return false
	}

	return strings.Contains(message, "no upstream configured") ||
		strings.Contains(message, "no upstream branch") ||
		strings.Contains(message, "does not point to a branch")
}

func (s *Service) revListCount(rangeExpr string) (int, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(s.Ctx, "rev-list", "--count", rangeExpr)
	if err := gitx.CommandError(fmt.Sprintf("count revisions for %s", rangeExpr), stderr, exitCode, runErr, "failed to count revisions"); err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		return 0, fmt.Errorf("ft: failed to parse revision count for %s", rangeExpr)
	}
	return count, nil
}
