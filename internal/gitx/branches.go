package gitx

import (
	"context"
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
