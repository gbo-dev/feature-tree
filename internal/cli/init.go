package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/shell"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [bash|zsh]",
		Short: "Print shell integration snippet for auto-cd",
		Long: `Print the shell function required for automatic directory switching.

A Go binary cannot change its caller's working directory — this is an OS
constraint that applies in any language. The shell function wraps the ft
binary, reads the __FT_CD__ marker on stdout, and calls cd on your behalf.

Supported shells: bash, zsh.
Without an argument ft auto-detects from $SHELL.

Source once in your shell config:
  eval "$(ft init)"      # auto-detect from $SHELL
  eval "$(ft init zsh)"  # explicit`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("expected at most one shell argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			targetShell := shell.PreferredShell()
			if len(args) == 1 {
				targetShell = args[0]
			}

			script, err := shell.InitScript(targetShell)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprint(cmd.OutOrStdout(), script); err != nil {
				return fmt.Errorf("write init script: %w", err)
			}
			return nil
		},
	}

	return cmd
}
