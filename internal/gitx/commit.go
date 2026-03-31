package gitx

import (
	"context"
	"strings"
	"sync"

	"github.com/gbo-dev/feature-tree/internal/textwidth"
)

const maxConcurrentHeadCommits = 8

type CommitInfo struct {
	Hash    string // 4-character abbreviated hash; empty when unavailable
	Subject string // first line of the commit message; empty when unavailable
}

// Display returns the subject (hash omitted), truncated to max visible columns.
func (c CommitInfo) Display(max int) string {
	if c.Hash == "" || strings.TrimSpace(c.Subject) == "" {
		return ""
	}
	if max <= 0 {
		return ""
	}
	return textwidth.Truncate(c.Subject, max)
}

// HeadCommit returns abbreviated head-commit info for branch.
func HeadCommit(commandCtx context.Context, ctx *RepoContext, branch string) CommitInfo {
	out, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "log", "-1", "--abbrev=4", "--format=%h\t%s", branch)
	if err := CommandError("read branch head commit", stderr, exitCode, runErr, "git log failed"); err != nil || out == "" {
		return CommitInfo{}
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	if len(parts) != 2 {
		return CommitInfo{}
	}
	return CommitInfo{
		Hash:    strings.TrimSpace(parts[0]),
		Subject: strings.TrimSpace(parts[1]),
	}
}

// FetchCommitsParallel returns head commits for branches in input order.
func FetchCommitsParallel(commandCtx context.Context, ctx *RepoContext, branches []string) []CommitInfo {
	results := make([]CommitInfo, len(branches))
	if len(branches) == 0 {
		return results
	}

	limit := maxConcurrentHeadCommits
	if len(branches) < limit {
		limit = len(branches)
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, b := range branches {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, branch string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = HeadCommit(commandCtx, ctx, branch)
		}(i, b)
	}
	wg.Wait()
	return results
}
