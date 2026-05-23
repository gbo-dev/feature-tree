package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

// PRMetadata describes a pull request head for checkout and push setup.
type PRMetadata struct {
	HeadRefName         string
	HeadRepositoryURL   string
	HeadRepositoryOwner string
	IsCrossRepository   bool
}

type ghPRViewJSON struct {
	HeadRefName         string `json:"headRefName"`
	IsCrossRepository   bool   `json:"isCrossRepository"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	HeadRepository struct {
		NameWithOwner string `json:"nameWithOwner"`
		URL           string `json:"url"`
	} `json:"headRepository"`
}

func forkRemoteName(owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "ft-fork-head"
	}
	var b strings.Builder
	b.WriteString("ft-fork-")
	for _, r := range strings.ToLower(owner) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune('-')
		default:
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" || name == "ft-fork-" {
		return "ft-fork-head"
	}
	return name
}

func (s *Service) resolvePRMetadata(commandCtx context.Context, prNumber int) (PRMetadata, error) {
	if s.prMetadataResolver != nil {
		return s.prMetadataResolver(commandCtx, prNumber)
	}
	return resolvePRMetadataFromGH(commandCtx, s.Ctx, prNumber)
}

func resolvePRMetadataFromGH(commandCtx context.Context, repoCtx *gitx.RepoContext, prNumber int) (PRMetadata, error) {
	if commandCtx == nil {
		return PRMetadata{}, fmt.Errorf("missing command context")
	}
	if repoCtx == nil {
		return PRMetadata{}, fmt.Errorf("missing repository context")
	}

	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNumber),
		"--json", "headRefName,headRepository,headRepositoryOwner,isCrossRepository",
	}
	cmd := exec.CommandContext(commandCtx, "gh", args...)
	cmd.Dir = strings.TrimSpace(repoCtx.RepoRoot)

	out, err := cmd.Output()
	if err != nil {
		return PRMetadata{}, fmt.Errorf("gh pr view: %w", err)
	}

	var payload ghPRViewJSON
	if err := json.Unmarshal(out, &payload); err != nil {
		return PRMetadata{}, fmt.Errorf("parse gh pr view output: %w", err)
	}

	meta := PRMetadata{
		HeadRefName:         strings.TrimSpace(payload.HeadRefName),
		HeadRepositoryURL:   strings.TrimSpace(payload.HeadRepository.URL),
		HeadRepositoryOwner: strings.TrimSpace(payload.HeadRepositoryOwner.Login),
		IsCrossRepository:   payload.IsCrossRepository,
	}
	if meta.HeadRepositoryOwner == "" {
		meta.HeadRepositoryOwner = ownerFromNameWithOwner(payload.HeadRepository.NameWithOwner)
	}
	return meta, nil
}

func ownerFromNameWithOwner(nameWithOwner string) string {
	nameWithOwner = strings.TrimSpace(nameWithOwner)
	if nameWithOwner == "" {
		return ""
	}
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
