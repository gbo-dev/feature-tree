package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/textwidth"
	"github.com/gbo-dev/feature-tree/internal/uiansi"
	"github.com/spf13/cobra"
)

const switchPreviewTabCount = 4

var switchPreviewTabLabels = []string{"Overview", "Commit log", "vs. default", "vs. upstream"}

func newPickerPreviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "__picker-preview <cache-file>",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected preview cache file path")
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("preview cache file path is empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolve preview cache file path: %w", err)
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return fmt.Errorf("read preview cache file: %w", err)
			}

			if _, err := fmt.Fprint(cmd.OutOrStdout(), string(data)); err != nil {
				return fmt.Errorf("write preview cache output: %w", err)
			}
			return nil
		},
	}

	return cmd
}

func newPickerPreviewStateCmd() *cobra.Command {
	var stateFile string
	var step int

	cmd := &cobra.Command{
		Use:    "__picker-preview-state",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(stateFile) == "" {
				return fmt.Errorf("state file is required")
			}
			if step == 0 {
				return nil
			}

			current, err := readPreviewTabState(stateFile)
			if err != nil {
				return err
			}

			next := current + step
			for next < 1 {
				next += switchPreviewTabCount
			}
			for next > switchPreviewTabCount {
				next -= switchPreviewTabCount
			}

			if err := os.WriteFile(stateFile, []byte(strconv.Itoa(next)), 0o600); err != nil {
				return fmt.Errorf("write preview tab state: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to preview tab state file")
	cmd.Flags().IntVar(&step, "step", 0, "Tab step increment (1 or -1)")
	return cmd
}

func newPickerPreviewTabCmd() *cobra.Command {
	var stateFile string

	cmd := &cobra.Command{
		Use:    "__picker-preview-tab <tab1-file> <tab2-file> <tab3-file> <tab4-file>",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != switchPreviewTabCount {
				return fmt.Errorf("expected %d tab files", switchPreviewTabCount)
			}
			if strings.TrimSpace(stateFile) == "" {
				return fmt.Errorf("state file is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			tab, err := readPreviewTabState(stateFile)
			if err != nil {
				return err
			}
			if tab < 1 || tab > switchPreviewTabCount {
				tab = 1
			}

			tabFile := args[tab-1]
			resolved, err := filepath.Abs(tabFile)
			if err != nil {
				return fmt.Errorf("resolve preview cache file path: %w", err)
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return fmt.Errorf("read preview cache file: %w", err)
			}

			header := renderPreviewHeaderLine(tab)
			if _, err := fmt.Fprint(cmd.OutOrStdout(), header+"\n"+string(data)); err != nil {
				return fmt.Errorf("write preview tab output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to preview tab state file")
	return cmd
}

func readPreviewTabState(stateFile string) (int, error) {
	resolved, err := filepath.Abs(stateFile)
	if err != nil {
		return 0, fmt.Errorf("resolve preview state path: %w", err)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return 0, fmt.Errorf("read preview tab state: %w", err)
	}

	tab, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse preview tab state: %w", err)
	}
	if tab < 1 || tab > switchPreviewTabCount {
		tab = 1
	}
	return tab, nil
}

func renderPreviewHeaderLine(activeTab int) string {
	parts := make([]string, 0, len(switchPreviewTabLabels))
	plainParts := make([]string, 0, len(switchPreviewTabLabels))
	for idx, label := range switchPreviewTabLabels {
		tab := idx + 1
		padded := " " + label + " "
		plainParts = append(plainParts, padded)
		if tab == activeTab {
			parts = append(parts, uiansi.TabActiveBg+uiansi.TabActiveFg+padded+uiansi.Reset)
		} else {
			parts = append(parts, uiansi.Grey+padded+uiansi.Reset)
		}
	}

	tabsPlain := strings.Join(plainParts, " ")
	tabsStyled := strings.Join(parts, " ")
	hint := "[tab/s-tab]"
	hintStyled := uiansi.InfoPurple + hint + uiansi.Reset

	cols := previewWidthColumns()
	if cols <= 0 {
		return tabsStyled + "  " + hintStyled
	}

	padding := cols - textwidth.Width(tabsPlain) - textwidth.Width(hint)
	if padding < 2 {
		padding = 2
	}
	return tabsStyled + strings.Repeat(" ", padding) + hintStyled
}

func previewWidthColumns() int {
	raw := strings.TrimSpace(os.Getenv("FZF_PREVIEW_COLUMNS"))
	if raw == "" {
		return 0
	}
	cols, err := strconv.Atoi(raw)
	if err != nil || cols <= 0 {
		return 0
	}
	return cols
}
