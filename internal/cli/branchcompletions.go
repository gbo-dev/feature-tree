package cli

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func completionContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func completeLocalBranchesWithShortcuts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext(completionContext(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	branches, err := listLocalBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{"^", "@"}, branches...)
	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeSwitchBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext(completionContext(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches, err := listWorktreeBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	localBranches, err := listLocalBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{"^", "@"}, worktreeBranches...)
	candidates = append(candidates, localBranches...)

	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeCreateBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext(completionContext(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches, err := listWorktreeBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	localBranches, err := listLocalBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{}, worktreeBranches...)
	candidates = append(candidates, localBranches...)

	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeRemovableWorktreeBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext(completionContext(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches, err := listWorktreeBranches(completionContext(cmd), ctx)
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

func listLocalBranches(commandCtx context.Context, ctx *gitx.RepoContext) ([]string, error) {
	return gitx.ListLocalBranches(commandCtx, ctx)
}

func listWorktreeBranches(commandCtx context.Context, ctx *gitx.RepoContext) ([]string, error) {
	entries, err := gitx.ListWorktrees(commandCtx, ctx)
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
