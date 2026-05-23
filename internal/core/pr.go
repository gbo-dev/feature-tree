package core

import (
	"context"
	"fmt"
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

type prPushUpstream struct {
	Remote string
	Branch string
}

func (s *Service) FetchAndCheckoutPRWithOptions(commandCtx context.Context, prNumber int, options PRCheckoutOptions) (*PRResult, error) {
	if commandCtx == nil {
		return nil, fmt.Errorf("missing command context")
	}
	prInfo, err := s.getPRInfo(commandCtx, prNumber, options.UsePRRef)
	if err != nil {
		return nil, err
	}

	warnings := make([]string, 0, 1)
	hints := make([]string, 0, 1)

	updatedSHA, warning, err := s.ensureLocalRefUpdated(commandCtx, prInfo.Number, prInfo.HeadSHA)
	if err != nil {
		return nil, err
	}
	prInfo.HeadSHA = updatedSHA
	if strings.TrimSpace(warning) != "" {
		warnings = append(warnings, warning)
	}

	metadata, _ := s.resolvePRMetadata(commandCtx, prNumber)

	headBranchName := strings.TrimSpace(metadata.HeadRefName)
	if headBranchName == "" {
		headBranchName = strings.TrimSpace(prInfo.HeadRemote)
	}
	if headBranchName == "" {
		headBranchName = s.findBranchNameBySHA(commandCtx, "refs/remotes/origin", prInfo.HeadSHA, true)
	}

	if options.UsePRRef {
		prInfo.HeadRef = fmt.Sprintf("pull/%d", prNumber)
	} else if headBranchName != "" {
		prInfo.HeadRef = headBranchName
	} else {
		prInfo.HeadRef = s.resolvePRBranchName(commandCtx, prNumber, prInfo.HeadSHA)
		headBranchName = prInfo.HeadRef
	}

	if err := s.validateLocalBranchForPR(commandCtx, prInfo.HeadRef, prInfo.HeadSHA); err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(commandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	var upstream *prPushUpstream
	if options.UsePRRef {
		if headBranchName != "" && headBranchName != prInfo.HeadRef {
			hints = append(hints, fmt.Sprintf("local branch is %q; plain git push may fail with push.default=simple — use: git push origin HEAD:%s", prInfo.HeadRef, headBranchName))
		}
	} else {
		var pushHints []string
		upstream, pushHints, err = s.resolvePRPushUpstream(commandCtx, prInfo.HeadRef, headBranchName, metadata)
		if err != nil {
			return nil, err
		}
		hints = append(hints, pushHints...)
	}

	if existingPath := FindWorktreePath(worktrees, prInfo.HeadRef); existingPath != "" {
		if err := s.applyPRPushUpstream(commandCtx, prInfo.HeadRef, upstream); err != nil {
			return nil, err
		}

		return &PRResult{
			Number:   prInfo.Number,
			Path:     existingPath,
			Branch:   prInfo.HeadRef,
			Created:  false,
			Warnings: warnings,
			Hints:    hints,
		}, nil
	}

	if err := s.syncLocalPRBranchToHead(commandCtx, prInfo.HeadRef, prInfo.HeadSHA); err != nil {
		return nil, err
	}

	result, err := s.CreateWorktree(commandCtx, prInfo.HeadRef, prInfo.BaseBranch)
	if err != nil {
		return nil, err
	}

	if err := s.applyPRPushUpstream(commandCtx, prInfo.HeadRef, upstream); err != nil {
		return nil, err
	}

	return &PRResult{
		Number:   prInfo.Number,
		Path:     result.Path,
		Branch:   prInfo.HeadRef,
		Created:  result.Created,
		Warnings: warnings,
		Hints:    hints,
	}, nil
}

func (s *Service) getPRInfo(commandCtx context.Context, prNumber int, usePRRef bool) (*PRInfo, error) {
	headSHA, err := s.resolvePRHeadSHA(commandCtx, prNumber)
	if err != nil {
		return nil, err
	}

	var headRef string
	if usePRRef {
		headRef = fmt.Sprintf("pull/%d", prNumber)
	} else {
		headRef = s.resolvePRBranchName(commandCtx, prNumber, headSHA)
	}

	headRemote := s.findBranchNameBySHA(commandCtx, "refs/remotes/origin", headSHA, true)

	title, err := s.resolvePRTitle(commandCtx, headSHA, prNumber)
	if err != nil {
		return nil, err
	}

	baseBranch := s.Ctx.DefaultBranch
	baseSHA, _ := s.resolveSHAForRefs(commandCtx, []string{
		fmt.Sprintf("refs/heads/%s", baseBranch),
		fmt.Sprintf("refs/remotes/origin/%s", baseBranch),
	})

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

func (s *Service) resolvePRHeadSHA(commandCtx context.Context, prNumber int) (string, error) {
	refsToTry := []string{
		fmt.Sprintf("refs/pull/%d/head", prNumber),
		fmt.Sprintf("refs/pull/%d/merge", prNumber),
	}

	if sha, ok := s.resolveSHAForRefs(commandCtx, refsToTry); ok {
		return sha, nil
	}

	return s.fetchAndResolvePRHead(commandCtx, prNumber)
}

func (s *Service) resolveSHAForRefs(commandCtx context.Context, refs []string) (string, bool) {
	for _, ref := range refs {
		stdout, _, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "rev-parse", "--verify", ref)
		if runErr == nil && exitCode == 0 {
			return strings.TrimSpace(stdout), true
		}
	}
	return "", false
}

func (s *Service) fetchAndResolvePRHead(commandCtx context.Context, prNumber int) (string, error) {
	ref := fmt.Sprintf("refs/pull/%d/head", prNumber)
	_, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "fetch", "origin", fmt.Sprintf("pull/%d/head:%s", prNumber, ref))
	if err := gitx.CommandError("fetch PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
		return "", fmt.Errorf("failed to fetch PR #%d: %w", prNumber, err)
	}

	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "rev-parse", "--verify", ref)
	if err := gitx.CommandError("resolve PR commit", stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
		return "", fmt.Errorf("failed to resolve PR #%d commit: %w", prNumber, err)
	}
	return strings.TrimSpace(stdout), nil
}

func (s *Service) resolvePRTitle(commandCtx context.Context, headSHA string, prNumber int) (string, error) {
	stdout, _, _, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "log", "--oneline", "-1", headSHA)
	title := strings.TrimSpace(stdout)
	if runErr != nil {
		title = fmt.Sprintf("PR #%d", prNumber)
	}
	return title, nil
}

func (s *Service) validateLocalBranchForPR(commandCtx context.Context, localBranch string, headSHA string) error {
	localBranch = strings.TrimSpace(localBranch)
	headSHA = strings.TrimSpace(headSHA)
	if localBranch == "" || headSHA == "" {
		return nil
	}

	existingSHA, ok, err := s.localBranchSHA(commandCtx, localBranch)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if existingSHA == headSHA {
		return nil
	}
	return fmt.Errorf("local branch %q exists at %s but PR head is %s; remove or rename the branch before checking out this PR", localBranch, shortSHA(existingSHA), shortSHA(headSHA))
}

func (s *Service) localBranchSHA(commandCtx context.Context, branch string) (string, bool, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "rev-parse", "--verify", "refs/heads/"+branch)
	if exitCode == 0 && runErr == nil {
		return strings.TrimSpace(stdout), true, nil
	}
	if exitCode == 128 || exitCode == 1 {
		return "", false, nil
	}
	if err := gitx.CommandError(fmt.Sprintf("resolve local branch %q", branch), stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
		return "", false, err
	}
	return "", false, nil
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

func (s *Service) resolvePRPushUpstream(commandCtx context.Context, localBranch string, headBranchName string, metadata PRMetadata) (*prPushUpstream, []string, error) {
	localBranch = strings.TrimSpace(localBranch)
	headBranchName = strings.TrimSpace(headBranchName)

	if localBranch == "" || headBranchName == "" {
		return nil, nil, nil
	}
	if localBranch != headBranchName {
		return nil, []string{fmt.Sprintf("local branch %q differs from PR head branch %q; plain git push may require an explicit refspec", localBranch, headBranchName)}, nil
	}

	if metadata.IsCrossRepository && strings.TrimSpace(metadata.HeadRepositoryURL) != "" {
		return s.resolveForkPRPushUpstream(commandCtx, localBranch, metadata)
	}

	exists, err := gitx.RemoteBranchExists(commandCtx, s.Ctx, "origin", headBranchName)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		if err := gitx.FetchRemoteBranch(commandCtx, s.Ctx, "origin", headBranchName); err != nil {
			return nil, []string{fmt.Sprintf("PR head branch %q is not on origin yet; after committing run: git push -u origin %s", headBranchName, headBranchName)}, nil
		}
	}

	return &prPushUpstream{Remote: "origin", Branch: headBranchName}, nil, nil
}

func (s *Service) resolveForkPRPushUpstream(commandCtx context.Context, localBranch string, metadata PRMetadata) (*prPushUpstream, []string, error) {
	headBranchName := strings.TrimSpace(metadata.HeadRefName)
	if headBranchName == "" {
		headBranchName = localBranch
	}

	remoteName := forkRemoteName(metadata.HeadRepositoryOwner)
	remoteURL := strings.TrimSpace(metadata.HeadRepositoryURL)
	if remoteURL == "" {
		return nil, []string{fmt.Sprintf("fork PR: add the head repository remote and push with: git push -u <remote> %s", headBranchName)}, nil
	}

	existingRemote, err := gitx.FindRemoteByURL(commandCtx, s.Ctx, remoteURL)
	if err != nil {
		return nil, nil, err
	}
	if existingRemote != "" {
		remoteName = existingRemote
	} else {
		if err := gitx.AddRemote(commandCtx, s.Ctx, remoteName, remoteURL); err != nil {
			return nil, nil, err
		}
	}

	if err := gitx.FetchRemoteBranch(commandCtx, s.Ctx, remoteName, headBranchName); err != nil {
		return nil, []string{fmt.Sprintf("fork PR: fetch and push with: git fetch %s %s && git push -u %s %s", remoteName, headBranchName, remoteName, headBranchName)}, nil
	}

	return &prPushUpstream{Remote: remoteName, Branch: headBranchName}, nil, nil
}

func (s *Service) applyPRPushUpstream(commandCtx context.Context, localBranch string, upstream *prPushUpstream) error {
	if upstream == nil {
		return nil
	}
	localBranch = strings.TrimSpace(localBranch)
	if localBranch == "" || localBranch != strings.TrimSpace(upstream.Branch) {
		return nil
	}

	exists, err := gitx.RemoteBranchExists(commandCtx, s.Ctx, upstream.Remote, upstream.Branch)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	return gitx.SetBranchUpstream(commandCtx, s.Ctx, localBranch, upstream.Remote+"/"+upstream.Branch)
}

func (s *Service) syncLocalPRBranchToHead(commandCtx context.Context, localBranch string, headSHA string) error {
	localBranch = strings.TrimSpace(localBranch)
	headSHA = strings.TrimSpace(headSHA)

	if localBranch == "" || headSHA == "" {
		return nil
	}

	existingSHA, ok, err := s.localBranchSHA(commandCtx, localBranch)
	if err != nil {
		return err
	}
	if ok && existingSHA == headSHA {
		return nil
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "update-ref", "refs/heads/"+localBranch, headSHA)
	if err := gitx.CommandError(fmt.Sprintf("move branch %q to PR head", localBranch), stderr, exitCode, runErr, "git update-ref failed"); err != nil {
		return err
	}

	return nil
}

func (s *Service) ensureLocalRefUpdated(commandCtx context.Context, prNumber int, fallbackSHA string) (string, string, error) {
	ref := fmt.Sprintf("refs/pull/%d/head", prNumber)

	stdout, _, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "rev-parse", "--verify", ref)
	cachedSHA := ""
	if runErr == nil && exitCode == 0 {
		cachedSHA = strings.TrimSpace(stdout)
	}

	_, stderr, exitCode, runErr := gitx.RunGitCommon(
		commandCtx,
		s.Ctx,
		"fetch",
		"origin",
		fmt.Sprintf("pull/%d/head:%s", prNumber, ref),
	)
	if err := gitx.CommandError("update PR ref", stderr, exitCode, runErr, "git fetch failed"); err != nil {
		if cachedSHA != "" {
			return cachedSHA, fmt.Sprintf("failed to update PR #%d from origin; using cached ref %s", prNumber, ref), nil
		}
		if fallbackSHA != "" {
			return fallbackSHA, fmt.Sprintf("failed to update PR #%d from origin; using cached ref %s", prNumber, ref), nil
		}
		return "", "", fmt.Errorf("failed to update PR #%d: %w", prNumber, err)
	}

	stdout, stderr, exitCode, runErr = gitx.RunGitCommon(commandCtx, s.Ctx, "rev-parse", "--verify", ref)
	if err := gitx.CommandError("resolve updated PR commit", stderr, exitCode, runErr, "git rev-parse failed"); err != nil {
		return "", "", fmt.Errorf("failed to resolve PR #%d commit: %w", prNumber, err)
	}
	return strings.TrimSpace(stdout), "", nil
}

func (s *Service) resolvePRBranchName(commandCtx context.Context, prNumber int, headSHA string) string {
	if branch := s.findBranchNameBySHA(commandCtx, "refs/heads", headSHA, false); branch != "" {
		return branch
	}
	if branch := s.findBranchNameBySHA(commandCtx, "refs/remotes/origin", headSHA, true); branch != "" {
		return branch
	}
	return fmt.Sprintf("pull/%d", prNumber)
}

func (s *Service) findBranchNameBySHA(commandCtx context.Context, refNamespace string, headSHA string, stripOriginPrefix bool) string {
	stdout, _, exitCode, runErr := gitx.RunGitCommon(
		commandCtx,
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
