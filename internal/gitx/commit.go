package gitx

import (
	"context"
	"errors"
	"fmt"
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
func HeadCommit(commandCtx context.Context, ctx *RepoContext, branch string) (CommitInfo, error) {
	out, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "log", "-1", "--abbrev=4", "--format=%h\t%s", branch)
	if err := CommandError("read branch head commit", stderr, exitCode, runErr, "git log failed"); err != nil {
		return CommitInfo{}, err
	}
	if out == "" {
		return CommitInfo{}, fmt.Errorf("read branch head commit: empty output")
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	if len(parts) != 2 {
		return CommitInfo{}, fmt.Errorf("read branch head commit: unexpected output format")
	}
	return CommitInfo{
		Hash:    strings.TrimSpace(parts[0]),
		Subject: strings.TrimSpace(parts[1]),
	}, nil
}

// FetchCommitsParallel returns head commits for branches in input order.
func FetchCommitsParallel(commandCtx context.Context, ctx *RepoContext, branches []string) ([]CommitInfo, error) {
	results := make([]CommitInfo, len(branches))
	if len(branches) == 0 {
		return results, nil
	}

	limit := maxConcurrentHeadCommits
	if len(branches) < limit {
		limit = len(branches)
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for i, b := range branches {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, branch string) {
			defer wg.Done()
			defer func() { <-sem }()
			ci, err := HeadCommit(commandCtx, ctx, branch)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("head commit for %q: %w", branch, err))
				mu.Unlock()
				return
			}
			results[idx] = ci
		}(i, b)
	}
	wg.Wait()

	if len(errs) > 0 {
		return results, errors.Join(errs...)
	}
	return results, nil
}
