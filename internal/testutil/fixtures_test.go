package testutil

import (
	"os"
	"testing"
)

func TestSetupBareRemoteCreatesBareOrigin(t *testing.T) {
	bare := SetupBareRemote(t)

	if _, err := os.Stat(bare.BareDir); err != nil {
		t.Fatalf("bare dir %q missing: %v", bare.BareDir, err)
	}
	if _, err := os.Stat(bare.SourceDir); err != nil {
		t.Fatalf("source dir %q missing: %v", bare.SourceDir, err)
	}

	head := RunGit(t, "", "--git-dir", bare.BareDir, "rev-parse", "--verify", "refs/heads/main")
	if head == "" {
		t.Fatalf("bare remote should have main branch")
	}
}
