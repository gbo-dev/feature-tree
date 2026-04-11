package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func (s *Service) CopyIncludeBetweenBranches(fromBranch string, toBranch string) (err error) {
	worktrees, err := gitx.ListWorktrees(s.CommandCtx, s.Ctx)
	if err != nil {
		return err
	}

	sourceWorktreePath := FindWorktreePath(worktrees, fromBranch)
	if sourceWorktreePath == "" {
		return fmt.Errorf("ft: no worktree found for branch %q", fromBranch)
	}

	destinationWorktreePath := FindWorktreePath(worktrees, toBranch)
	if destinationWorktreePath == "" {
		return fmt.Errorf("ft: no worktree found for branch %q", toBranch)
	}

	if sourceWorktreePath == destinationWorktreePath {
		return nil
	}

	includeManifestPath := filepath.Join(sourceWorktreePath, s.Ctx.IncludeFile)
	includeManifestFile, err := os.Open(includeManifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ft: read include file %s: %w", includeManifestPath, err)
	}
	defer func() {
		if closeErr := includeManifestFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("ft: close include file %s: %w", includeManifestPath, closeErr)
		}
	}()

	scanner := bufio.NewScanner(includeManifestFile)
	for scanner.Scan() {
		raw := scanner.Text()
		pattern := raw
		if idx := strings.Index(pattern, "#"); idx >= 0 {
			pattern = pattern[:idx]
		}
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		pattern = strings.TrimPrefix(pattern, "/")

		matches, err := filepath.Glob(filepath.Join(sourceWorktreePath, pattern))
		if err != nil {
			return fmt.Errorf("ft: parse include pattern %q: %w", pattern, err)
		}

		for _, match := range matches {
			rel, err := filepath.Rel(sourceWorktreePath, match)
			if err != nil {
				return fmt.Errorf("ft: compute include relative path: %w", err)
			}
			if strings.HasPrefix(rel, "..") {
				continue
			}

			dest := filepath.Join(destinationWorktreePath, rel)
			if err := copyPreservingShape(match, dest); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ft: read include file %s: %w", includeManifestPath, err)
	}

	return nil
}

func copyPreservingShape(src string, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("ft: inspect include source %s: %w", src, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("ft: read symlink %s: %w", src, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("ft: create include destination parent: %w", err)
		}
		_ = os.Remove(dst)
		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("ft: create symlink %s: %w", dst, err)
		}
		return nil
	}

	if info.IsDir() {
		return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := dst
			if rel != "." {
				target = filepath.Join(dst, rel)
			}

			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}

			if d.Type()&os.ModeSymlink != 0 {
				lnk, err := os.Readlink(path)
				if err != nil {
					return err
				}
				_ = os.Remove(target)
				return os.Symlink(lnk, target)
			}

			return copyFile(path, target)
		})
	}

	return copyFile(src, dst)
}

func copyFile(src string, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("ft: open include source file %s: %w", src, err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("ft: close include source file %s: %w", src, closeErr)
		}
	}()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("ft: stat include source file %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ft: create include destination parent: %w", err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("ft: open include destination file %s: %w", dst, err)
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("ft: close include destination file %s: %w", dst, closeErr)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("ft: copy %s -> %s: %w", src, dst, err)
	}

	if err = out.Sync(); err != nil {
		return fmt.Errorf("ft: sync include destination file %s: %w", dst, err)
	}

	return nil
}
