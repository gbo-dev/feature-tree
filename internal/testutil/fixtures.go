package testutil

import (
	"path/filepath"
	"testing"
)

// BareFixture holds paths for a source repo and its bare remote under one temp base.
type BareFixture struct {
	BaseDir   string
	SourceDir string
	BareDir   string
}

// SetupBareRemote creates a source repo with main and a bare origin.git remote.
func SetupBareRemote(t *testing.T) BareFixture {
	t.Helper()
	return setupBareRemoteIn(t, t.TempDir())
}

func setupBareRemoteIn(t *testing.T, base string) BareFixture {
	t.Helper()

	source := filepath.Join(base, "source")
	InitRepoWithMain(t, source)

	bare := filepath.Join(base, "origin.git")
	RunGit(t, "", "clone", "--bare", source, bare)

	return BareFixture{
		BaseDir:   base,
		SourceDir: source,
		BareDir:   bare,
	}
}

// SetupBareRemoteFromSource bare-clones an existing source directory into origin.git.
func SetupBareRemoteFromSource(t *testing.T, baseDir, sourceDir string) BareFixture {
	t.Helper()

	bare := filepath.Join(baseDir, "origin.git")
	RunGit(t, "", "clone", "--bare", sourceDir, bare)

	return BareFixture{
		BaseDir:   baseDir,
		SourceDir: sourceDir,
		BareDir:   bare,
	}
}
