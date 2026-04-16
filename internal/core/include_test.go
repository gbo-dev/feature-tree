package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func TestCopyIncludeBetweenBranchesCopiesMatchedFiles(t *testing.T) {
	svc, featurePath, featureBranch := setupServiceWithFeatureWorktree(t)

	worktrees, err := gitx.ListWorktrees(context.Background(), svc.Ctx)
	if err != nil {
		t.Fatalf("ListWorktrees returned error: %v", err)
	}
	mainPath := FindWorktreePath(worktrees, svc.Ctx.DefaultBranch)
	if mainPath == "" {
		t.Fatalf("default branch worktree path not found")
	}

	manifestPath := filepath.Join(mainPath, svc.Ctx.IncludeFile)
	if err := os.WriteFile(manifestPath, []byte("config/*.txt\n"), 0o644); err != nil {
		t.Fatalf("write include manifest: %v", err)
	}

	sourceFile := filepath.Join(mainPath, "config", "settings.txt")
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0o755); err != nil {
		t.Fatalf("mkdir source include dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("include-me\n"), 0o644); err != nil {
		t.Fatalf("write source include file: %v", err)
	}

	if err := svc.CopyIncludeBetweenBranches(context.Background(), svc.Ctx.DefaultBranch, featureBranch); err != nil {
		t.Fatalf("CopyIncludeBetweenBranches returned error: %v", err)
	}

	destinationFile := filepath.Join(featurePath, "config", "settings.txt")
	content, err := os.ReadFile(destinationFile)
	if err != nil {
		t.Fatalf("read copied include file: %v", err)
	}
	if string(content) != "include-me\n" {
		t.Fatalf("copied include content = %q, want %q", string(content), "include-me\\n")
	}
}
