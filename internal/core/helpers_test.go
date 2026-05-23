package core

import (
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gittest"
)

func setupServiceWithFeatureWorktree(t *testing.T) (*Service, string, string) {
	t.Helper()

	const branch = "feature-remove"
	clone, featurePath := gittest.SetupClonedRepoWithWorktree(t, branch)
	svc := &Service{Ctx: gittest.RepoContextFromClone(clone)}
	return svc, featurePath, branch
}
