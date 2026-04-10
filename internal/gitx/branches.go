package gitx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func ListLocalBranches(commandCtx context.Context, ctx *RepoContext) ([]string, error) {
	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err := CommandError("list local branches", stderr, exitCode, runErr, "git for-each-ref failed"); err != nil {
		return nil, err
	}

	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

type LocalBranchSnapshot struct {
	Branch   string
	Commit   CommitInfo
	Relation string
}

func ListLocalBranchSnapshots(commandCtx context.Context, ctx *RepoContext) ([]LocalBranchSnapshot, error) {
	snapshots, err := listLocalBranchSnapshotsBatch(commandCtx, ctx)
	if err == nil {
		return snapshots, nil
	}
	return listLocalBranchSnapshotsFallback(commandCtx, ctx)
}

func listLocalBranchSnapshotsBatch(commandCtx context.Context, ctx *RepoContext) ([]LocalBranchSnapshot, error) {
	format := fmt.Sprintf("%%(refname:short)%%x09%%(objectname:short=4)%%x09%%(ahead-behind:%s)%%x09%%(subject)", ctx.DefaultBranch)
	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "for-each-ref", "--format="+format, "refs/heads")
	if err := CommandError("list local branch snapshots", stderr, exitCode, runErr, "git for-each-ref failed"); err != nil {
		return nil, err
	}

	snapshots := make([]LocalBranchSnapshot, 0)
	parseFailures := 0
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			parseFailures++
			continue
		}

		branch := strings.TrimSpace(parts[0])
		if branch == "" {
			parseFailures++
			continue
		}

		hash := strings.TrimSpace(parts[1])
		subject := strings.TrimSpace(parts[3])
		commit := CommitInfo{}
		if hash != "" && subject != "" {
			commit = CommitInfo{Hash: hash, Subject: subject}
		}

		relation := relationFromAheadBehind(branch, ctx.DefaultBranch, strings.TrimSpace(parts[2]))
		snapshots = append(snapshots, LocalBranchSnapshot{
			Branch:   branch,
			Commit:   commit,
			Relation: relation,
		})
	}
	if parseFailures > 0 {
		return nil, fmt.Errorf("parse local branch snapshots")
	}
	return snapshots, nil
}

func relationFromAheadBehind(branch string, defaultBranch string, aheadBehind string) string {
	if branch == defaultBranch {
		return "-"
	}

	parts := strings.Fields(aheadBehind)
	if len(parts) != 2 {
		return "?"
	}

	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return "?"
	}
	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return "?"
	}
	return fmt.Sprintf("A: %d B: %d", ahead, behind)
}

func listLocalBranchSnapshotsFallback(commandCtx context.Context, ctx *RepoContext) ([]LocalBranchSnapshot, error) {
	branches, err := ListLocalBranches(commandCtx, ctx)
	if err != nil {
		return nil, err
	}

	commits := FetchCommitsParallel(commandCtx, ctx, branches)
	relations := FetchBranchRelationsParallel(commandCtx, ctx, branches)

	snapshots := make([]LocalBranchSnapshot, 0, len(branches))
	for i, branch := range branches {
		snapshots = append(snapshots, LocalBranchSnapshot{
			Branch:   branch,
			Commit:   commits[i],
			Relation: relations[i],
		})
	}
	return snapshots, nil
}
