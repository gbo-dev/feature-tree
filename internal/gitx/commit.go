package gitx

import (
	"strings"
	"sync"
)

// ellipsis marks truncation and renders as one terminal column.
const ellipsis = "\u2026"

const ellipsisWidth = 1

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
	subject := c.Subject
	if len(subject) > max {
		subject = subject[:max-ellipsisWidth] + ellipsis
	}
	return subject
}

// HeadCommit returns abbreviated head-commit info for branch.
func HeadCommit(ctx *RepoContext, branch string) CommitInfo {
	out, _, exitCode, err := RunGitCommon(ctx, "log", "-1", "--abbrev=4", "--format=%h\t%s", branch)
	if err != nil || exitCode != 0 || out == "" {
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
func FetchCommitsParallel(ctx *RepoContext, branches []string) []CommitInfo {
	results := make([]CommitInfo, len(branches))
	var wg sync.WaitGroup
	for i, b := range branches {
		wg.Add(1)
		go func(idx int, branch string) {
			defer wg.Done()
			results[idx] = HeadCommit(ctx, branch)
		}(i, b)
	}
	wg.Wait()
	return results
}
