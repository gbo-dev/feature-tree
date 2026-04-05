package core

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type PRInfo struct {
	Number     int
	HeadRef    string
	HeadSHA    string
	BaseBranch string
	BaseSHA    string
	Title      string
}

func (s *Service) FetchAndCheckoutPR(prNumber int) (*PRResult, error) {
	prInfo, err := s.GetPRInfo(prNumber)
	if err != nil {
		return nil, err
	}

	if err := s.ensureLocalRefUpdated(prInfo); err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(s.CommandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	if existingPath := FindWorktreePath(worktrees, prInfo.HeadRef); existingPath != "" {
		return &PRResult{
			Number:  prInfo.Number,
			Path:    existingPath,
			Branch:  prInfo.HeadRef,
			Created: false,
		}, nil
	}

	result, err := s.CreateWorktree(prInfo.HeadRef, prInfo.BaseBranch)
	if err != nil {
		return nil, err
	}

	return &PRResult{
		Number:  prInfo.Number,
		Path:    result.Path,
		Branch:  prInfo.HeadRef,
		Created: result.Created,
	}, nil
}

func (s *Service) GetPRInfo(prNumber int) (*PRInfo, error) {
	refsToTry := []string{
		fmt.Sprintf("refs/pull/%d/head", prNumber),
		fmt.Sprintf("refs/pull/%d/merge", prNumber),
	}

	var headRef string
	var headSHA string

	for _, ref := range refsToTry {
		stdout, _, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
		if runErr == nil && exitCode == 0 {
			headRef = fmt.Sprintf("pull/%d", prNumber)
			headSHA = strings.TrimSpace(stdout)
			break
		}
	}

	if headSHA == "" {
		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "fetch", "origin", fmt.Sprintf("pull/%d/head:refs/pull/%d/head", prNumber, prNumber))
		if err := gitx.CommandError("fetch PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
			return nil, fmt.Errorf("ft: failed to fetch PR #%d: %w", prNumber, err)
		}

		headRef = fmt.Sprintf("pull/%d", prNumber)
		stdout, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", fmt.Sprintf("refs/pull/%d/head", prNumber))
		if err := gitx.CommandError("resolve PR commit", stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
			return nil, fmt.Errorf("ft: failed to resolve PR #%d commit: %w", prNumber, err)
		}
		headSHA = strings.TrimSpace(stdout)
	}

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
		HeadSHA:    headSHA,
		BaseBranch: baseBranch,
		BaseSHA:    baseSHA,
		Title:      title,
	}, nil
}

func (s *Service) ensureLocalRefUpdated(prInfo *PRInfo) error {
	ref := fmt.Sprintf("refs/pull/%d/head", prInfo.Number)

	stdout, _, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "rev-parse", "--verify", ref)
	currentSHA := ""
	if runErr == nil && exitCode == 0 {
		currentSHA = strings.TrimSpace(stdout)
	}

	if currentSHA != "" && currentSHA == prInfo.HeadSHA {
		return nil
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "fetch", "origin", fmt.Sprintf("pull/%d/head", prInfo.Number))
	if err := gitx.CommandError("update PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
		return fmt.Errorf("ft: failed to update PR #%d: %w", prInfo.Number, err)
	}

	return nil
}

func ParsePRNumber(input string) (int, error) {
	prNum, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("ft: %q is not a valid PR number", input)
	}
	if prNum <= 0 {
		return 0, fmt.Errorf("ft: PR number must be positive")
	}
	return prNum, nil
}
