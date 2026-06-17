package core

import (
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func setupServiceWithFeatureWorktree(t *testing.T) (*Service, string, string) {
	t.Helper()

	fixture := testutil.SetupFeatureWorktree(t, "feature-remove")

	svc := &Service{Ctx: &gitx.RepoContext{
		RepoRoot:      fixture.RepoRoot,
		GitCommonDir:  fixture.GitCommonDir,
		DefaultBranch: fixture.DefaultBranch,
		IncludeFile:   ".worktreeinclude",
	}}

	return svc, fixture.FeaturePath, fixture.Branch
}
