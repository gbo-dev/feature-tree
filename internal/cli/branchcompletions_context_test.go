package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionContextUsesCommandContext(t *testing.T) {
	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "from-cmd")
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	got := completionContext(cmd)
	if got.Value(ctxKey{}) != "from-cmd" {
		t.Fatalf("completionContext = %#v, want command context value %q", got, "from-cmd")
	}
}

func TestCompletionContextFallsBackToRootContext(t *testing.T) {
	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "from-root")
	root := &cobra.Command{}
	root.SetContext(ctx)

	child := &cobra.Command{}
	root.AddCommand(child)

	got := completionContext(child)
	if got.Value(ctxKey{}) != "from-root" {
		t.Fatalf("completionContext = %#v, want root context value %q", got, "from-root")
	}
}

func TestCompletionContextFallsBackToBackground(t *testing.T) {
	cmd := &cobra.Command{}
	got := completionContext(cmd)
	if got == nil {
		t.Fatalf("completionContext returned nil")
	}
	if got.Err() != nil {
		t.Fatalf("background context should not be canceled: %v", got.Err())
	}
}
