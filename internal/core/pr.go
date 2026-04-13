package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type PRInfo struct {
	Number     int
	HeadRef    string
	HeadRemote string
	HeadSHA    string
	BaseBranch string
	BaseSHA    string
	Title      string
}

type PRCheckoutOptions struct {
	UsePRRef bool
}

func (s *Service) FetchAndCheckoutPRWithOptions(prNumber int, options PRCheckoutOptions) (*PRResult, error) {
	prInfo, err := s.getPRInfo(prNumber, options.UsePRRef)
	if err != nil {
		return nil, err
	}

	if err := s.ensureLocalRefUpdated(prInfo); err != nil {
		return nil, err
	}

	if options.UsePRRef {
		prInfo.HeadRef = fmt.Sprintf("pull/%d", prNumber)
	} else {
		prInfo.HeadRef = s.resolvePRBranchName(prNumber, prInfo.HeadSHA)
	}
	prInfo.HeadRemote = s.findBranchNameBySHA("refs/remotes/origin", prInfo.HeadSHA, true)

	worktrees, err := gitx.ListWorktrees(s.CommandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	if existingPath := FindWorktreePath(worktrees, prInfo.HeadRef); existingPath != "" {
		if err := s.ensurePRBranchTracking(prInfo.HeadRef, prInfo.HeadRemote); err != nil {
			return nil, err
		}

		return &PRResult{
			Number:  prInfo.Number,
			Path:    existingPath,
			Branch:  prInfo.HeadRef,
			Created: false,
		}, nil
	}

	if err := s.syncLocalPRBranchToHead(prInfo.HeadRef, prInfo.HeadSHA); err != nil {
		return nil, err
	}

	result, err := s.CreateWorktree(prInfo.HeadRef, prInfo.BaseBranch)
	if err != nil {
		return nil, err
	}

	if err := s.ensurePRBranchTracking(prInfo.HeadRef, prInfo.HeadRemote); err != nil {
		return nil, err
	}

	return &PRResult{
		Number:  prInfo.Number,
		Path:    result.Path,
		Branch:  prInfo.HeadRef,
		Created: result.Created,
	}, nil
}
func (s *Service) getPRInfo(prNumber int, usePRRef bool) (*PRInfo, error) {
	refsToTry := []string{
		fmt.Sprintf("refs/pull/%d/head", prNumber),
		fmt.Sprintf("refs/pull/%d/merge", prNumber),
	}

	var headRef string
	var headSHA string

	for _, ref := range refsToTry {
		stdout, _, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
		if runErr == nil && exitCode == 0 {
			headSHA = strings.TrimSpace(stdout)
			break
		}
	}

	if headSHA == "" {
		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "fetch", "origin", fmt.Sprintf("pull/%d/head:refs/pull/%d/head", prNumber, prNumber))
		if err := gitx.CommandError("fetch PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
			return nil, fmt.Errorf("ft: failed to fetch PR #%d: %w", prNumber, err)
		}

		stdout, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", fmt.Sprintf("refs/pull/%d/head", prNumber))
		if err := gitx.CommandError("resolve PR commit", stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
			return nil, fmt.Errorf("ft: failed to resolve PR #%d commit: %w", prNumber, err)
		}
		headSHA = strings.TrimSpace(stdout)
	}

	if usePRRef {
		headRef = fmt.Sprintf("pull/%d", prNumber)
	} else {
		headRef = s.resolvePRBranchName(prNumber, headSHA)
	}

	headRemote := s.findBranchNameBySHA("refs/remotes/origin", headSHA, true)

	stdout, _, _, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "log", "--oneline", "-1", headSHA)
	title := strings.TrimSpace(stdout)
	if runErr != nil {
		title = fmt.Sprintf("PR #%d", prNumber)
	}

	baseBranch := s.Ctx.DefaultBranch
	baseSHA := ""

	baseRefsToTry := []string{
		fmt.Sprintf("refs/heads/%s", baseBranch),
		fmt.Sprintf("refs/remotes/origin/%s", baseBranch),
	}

	for _, ref := range baseRefsToTry {
		stdout, _, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
		if runErr == nil && exitCode == 0 {
			baseSHA = strings.TrimSpace(stdout)
			break
		}
	}

	return &PRInfo{
		Number:     prNumber,
		HeadRef:    headRef,
		HeadRemote: headRemote,
		HeadSHA:    headSHA,
		BaseBranch: baseBranch,
		BaseSHA:    baseSHA,
		Title:      title,
	}, nil
}

func (s *Service) ensurePRBranchTracking(localBranch string, remoteBranch string) error {
	localBranch = strings.TrimSpace(localBranch)
	remoteBranch = strings.TrimSpace(remoteBranch)

	if localBranch == "" || remoteBranch == "" {
		return nil
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+remoteBranch)
	if exitCode == 1 && runErr == nil {
		return nil
	}
	if err := gitx.CommandError(fmt.Sprintf("verify remote branch %q", remoteBranch), stderr, exitCode, runErr, "git show-ref failed"); err != nil {
		return err
	}

	_, stderr, exitCode, runErr = gitx.RunGitCommon(s.CommandCtx, s.Ctx, "branch", "--set-upstream-to", "origin/"+remoteBranch, localBranch)
	if err := gitx.CommandError(fmt.Sprintf("set upstream for branch %q", localBranch), stderr, exitCode, runErr, "git branch failed"); err != nil {
		return err
	}

	return nil
}

func (s *Service) syncLocalPRBranchToHead(localBranch string, headSHA string) error {
	localBranch = strings.TrimSpace(localBranch)
	headSHA = strings.TrimSpace(headSHA)

	if localBranch == "" || headSHA == "" {
		return nil
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "update-ref", "refs/heads/"+localBranch, headSHA)
	if err := gitx.CommandError(fmt.Sprintf("move branch %q to PR head", localBranch), stderr, exitCode, runErr, "git update-ref failed"); err != nil {
		return err
	}

	return nil
}

func (s *Service) ensureLocalRefUpdated(prInfo *PRInfo) error {
	ref := fmt.Sprintf("refs/pull/%d/head", prInfo.Number)

	stdout, _, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
	currentSHA := ""
	if runErr == nil && exitCode == 0 {
		currentSHA = strings.TrimSpace(stdout)
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(
		s.CommandCtx,
		s.Ctx,
		"fetch",
		"origin",
		fmt.Sprintf("pull/%d/head:%s", prInfo.Number, ref),
	)
	if err := gitx.CommandError("update PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
		if currentSHA != "" {
			_, _ = fmt.Fprintf(os.Stderr, "ft: warning: failed to update PR #%d from origin; using cached ref %s\n", prInfo.Number, ref)
			prInfo.HeadSHA = currentSHA
			return nil
		}
		return fmt.Errorf("ft: failed to update PR #%d: %w", prInfo.Number, err)
	}

	stdout, stderr, exitCode, runErr = gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
	if err := gitx.CommandError("resolve updated PR commit", stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
		return fmt.Errorf("ft: failed to resolve PR #%d commit: %w", prInfo.Number, err)
	}
	prInfo.HeadSHA = strings.TrimSpace(stdout)

	return nil
}

func (s *Service) resolvePRBranchName(prNumber int, headSHA string) string {
	if branch := s.findBranchNameBySHA("refs/heads", headSHA, false); branch != "" {
		return branch
	}
	if branch := s.findBranchNameBySHA("refs/remotes/origin", headSHA, true); branch != "" {
		return branch
	}
	return fmt.Sprintf("pull/%d", prNumber)
}

func (s *Service) findBranchNameBySHA(refNamespace string, headSHA string, stripOriginPrefix bool) string {
	stdout, _, exitCode, runErr := gitx.RunGitCommon(
		s.CommandCtx,
		s.Ctx,
		"for-each-ref",
		"--format=%(refname:short)",
		"--points-at",
		headSHA,
		refNamespace,
	)
	if runErr != nil || exitCode != 0 {
		return ""
	}

	for _, line := range strings.Split(stdout, "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" || branch == "origin/HEAD" {
			continue
		}
		if stripOriginPrefix && (branch == "origin" || !strings.HasPrefix(branch, "origin/")) {
			continue
		}
		if stripOriginPrefix {
			branch = strings.TrimPrefix(branch, "origin/")
		}
		if branch == s.Ctx.DefaultBranch || strings.HasPrefix(branch, "pull/") {
			continue
		}
		return branch
	}

	return ""
}
