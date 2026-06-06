package cli

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func completionContext(cmd *cobra.Command) context.Context {
	if cmd == nil {
		return context.Background()
	}
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	if root := cmd.Root(); root != nil && root.Context() != nil {
		return root.Context()
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

	branches, err := gitx.ListLocalBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	candidates := append([]string{"^", "@"}, branches...)
	return filterPrefixUniqueSorted(candidates, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeSwitchBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeWorktreeAndLocalBranches(cmd, args, toComplete, true)
}

func completeCreateBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeWorktreeAndLocalBranches(cmd, args, toComplete, false)
}

func completeWorktreeAndLocalBranches(cmd *cobra.Command, args []string, toComplete string, includeShortcuts bool) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx, err := gitx.DiscoverRepoContext(completionContext(cmd))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	entries, err := gitx.ListWorktrees(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	worktreeBranches := make([]string, 0, len(entries))
	for _, worktree := range entries {
		if worktree.Branch == "" {
			continue
		}
		worktreeBranches = append(worktreeBranches, worktree.Branch)
	}

	localBranches, err := gitx.ListLocalBranches(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var candidates []string
	if includeShortcuts {
		candidates = append([]string{"^", "@"}, worktreeBranches...)
	} else {
		candidates = append([]string{}, worktreeBranches...)
	}
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

	entries, err := gitx.ListWorktrees(completionContext(cmd), ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	filtered := make([]string, 0, len(entries)+1)
	filtered = append(filtered, "@")
	for _, worktree := range entries {
		if worktree.Branch == "" || worktree.Branch == ctx.DefaultBranch {
			continue
		}
		filtered = append(filtered, worktree.Branch)
	}

	return filterPrefixUniqueSorted(filtered, toComplete), cobra.ShellCompDirectiveNoFileComp
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
