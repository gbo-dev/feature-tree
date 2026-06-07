package cli

import (
	"errors"
	"fmt"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/tui"
)

func errWriteOutput(action string, err error) error {
	return fmt.Errorf("write %s output: %w", action, err)
}

func errSelectionCancelled() error {
	return fmt.Errorf("selection cancelled")
}

func mapDetachedHead(err error, userMsg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gitx.ErrDetachedHead) {
		return fmt.Errorf("%s", userMsg)
	}
	return err
}

func mapCurrentBranchForList(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gitx.ErrDetachedHead) {
		return fmt.Errorf("cannot determine current branch while HEAD is detached; check out a branch and retry")
	}
	return fmt.Errorf("resolve current branch for list: %w", err)
}

func handlePickerError(err error) error {
	if errors.Is(err, tui.ErrSelectionCancelled) {
		return errSelectionCancelled()
	}
	return err
}
