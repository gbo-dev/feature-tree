package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func completeLocalBranchesWithShortcuts(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	branches, err := listLocalBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{"^", "@"}, branches...)
	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeSwitchBranches(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches, err := listWorktreeBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	localBranches, err := listLocalBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{"^", "@"}, worktreeBranches...)
	candidates = append(candidates, localBranches...)

	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeRemovableWorktreeBranches(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches, err := listWorktreeBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	filtered := make([]string, 0, len(worktreeBranches)+1)
	filtered = append(filtered, "@")
	for _, branch := range worktreeBranches {
		if branch == ctx.DefaultBranch {
			continue
		}
		filtered = append(filtered, branch)
	}

	return filterPrefixUniqueSorted(filtered, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func listLocalBranches(ctx *gitx.RepoContext) ([]string, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err := gitx.CommandError("list local branches for completion", stderr, exitCode, runErr, "git for-each-ref failed"); err != nil {
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

func listWorktreeBranches(ctx *gitx.RepoContext) ([]string, error) {
	entries, err := gitx.ListWorktrees(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(entries))
	for _, worktree := range entries {
		if worktree.Branch == "" {
			continue
		}
		out = append(out, worktree.Branch)
	}

	return out, nil
}

func filterPrefixUniqueSorted(values []string, prefix string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(value, prefix) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	sort.Strings(out)
	return out
}
