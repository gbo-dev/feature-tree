package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

const (
	maxSwitchPreviewWorkers = 6
	switchLogLimit          = 30
	ansiDiffRed             = "\x1b[38;5;203m"
)

type switchPreviewTabPaths struct {
	headPath         string
	logPath          string
	defaultDiffPath  string
	upstreamDiffPath string
}

type switchPreviewBuildJob struct {
	index int
	row   pickerRow
}

type switchPreviewBuildResult struct {
	branch string
	paths  switchPreviewTabPaths
	err    error
}

type switchPreviewCache struct {
	tabsByBranch map[string]switchPreviewTabPaths
	stateFile    string
}

func buildSwitchPreviewCache(commandCtx context.Context, repoCtx *gitx.RepoContext, rows []pickerRow) (*switchPreviewCache, func(), error) {
	cleanup := func() {}
	if len(rows) == 0 {
		return &switchPreviewCache{tabsByBranch: map[string]switchPreviewTabPaths{}}, cleanup, nil
	}

	tmpDir, err := os.MkdirTemp("", "ft-switch-preview-")
	if err != nil {
		return nil, cleanup, fmt.Errorf("create switch preview cache dir: %w", err)
	}

	stateFile := filepath.Join(tmpDir, "tab-state")
	if writeErr := os.WriteFile(stateFile, []byte("1"), 0o600); writeErr != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, cleanup, fmt.Errorf("create switch preview state file: %w", writeErr)
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}

	workers := min(maxSwitchPreviewWorkers, len(rows))
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan switchPreviewBuildJob)
	results := make(chan switchPreviewBuildResult, len(rows))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				paths, rowErr := buildSwitchPreviewRowCache(commandCtx, repoCtx, tmpDir, job)
				results <- switchPreviewBuildResult{branch: job.row.branch, paths: paths, err: rowErr}
			}
		}()
	}

	go func() {
		for idx, row := range rows {
			jobs <- switchPreviewBuildJob{index: idx, row: row}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	cache := make(map[string]switchPreviewTabPaths, len(rows))
	for result := range results {
		if result.err != nil {
			cleanup()
			return nil, func() {}, result.err
		}
		cache[result.branch] = result.paths
	}

	return &switchPreviewCache{tabsByBranch: cache, stateFile: stateFile}, cleanup, nil
}

func buildSwitchPreviewRowCache(commandCtx context.Context, repoCtx *gitx.RepoContext, tmpDir string, job switchPreviewBuildJob) (switchPreviewTabPaths, error) {
	headContent := renderSwitchHeadTab(job.row)
	logContent := renderSwitchLogTab(commandCtx, repoCtx, job.row.branch)
	defaultDiffContent := renderSwitchDefaultDiffTab(commandCtx, repoCtx, job.row.branch)
	upstreamDiffContent := renderSwitchUpstreamDiffTab(commandCtx, repoCtx, job.row.branch)

	prefix := fmt.Sprintf("b%03d", job.index)
	headPath, err := writeSwitchPreviewFile(tmpDir, prefix+"-head.txt", headContent)
	if err != nil {
		return switchPreviewTabPaths{}, err
	}
	logPath, err := writeSwitchPreviewFile(tmpDir, prefix+"-log.txt", logContent)
	if err != nil {
		return switchPreviewTabPaths{}, err
	}
	defaultDiffPath, err := writeSwitchPreviewFile(tmpDir, prefix+"-main.txt", defaultDiffContent)
	if err != nil {
		return switchPreviewTabPaths{}, err
	}
	upstreamDiffPath, err := writeSwitchPreviewFile(tmpDir, prefix+"-upstream.txt", upstreamDiffContent)
	if err != nil {
		return switchPreviewTabPaths{}, err
	}

	return switchPreviewTabPaths{
		headPath:         headPath,
		logPath:          logPath,
		defaultDiffPath:  defaultDiffPath,
		upstreamDiffPath: upstreamDiffPath,
	}, nil
}

func writeSwitchPreviewFile(tmpDir string, fileName string, content string) (string, error) {
	fullPath := filepath.Join(tmpDir, fileName)
	if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write switch preview cache %q: %w", fileName, err)
	}
	return fullPath, nil
}

func renderSwitchHeadTab(row pickerRow) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("Path:   ")
	b.WriteString(row.path)
	b.WriteString("\n")
	b.WriteString("State:  ")
	b.WriteString(row.state)
	b.WriteString("\n")
	b.WriteString("Main:   ")
	b.WriteString(row.relation)
	b.WriteString("\n")
	if strings.TrimSpace(row.commit.Hash) != "" && strings.TrimSpace(row.commit.Subject) != "" {
		b.WriteString("HEAD:   ")
		b.WriteString(row.commit.Hash)
		b.WriteString(" ")
		b.WriteString(row.commit.Subject)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if row.state == "clean" {
		b.WriteString(row.branch)
		b.WriteString(" has no uncommitted changes.\n")
	} else {
		b.WriteString(row.branch)
		b.WriteString(" has uncommitted changes (")
		b.WriteString(row.state)
		b.WriteString(").\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderSwitchLogTab(commandCtx context.Context, repoCtx *gitx.RepoContext, branch string) string {
	var b strings.Builder
	b.WriteString("\n")

	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(
		commandCtx,
		repoCtx,
		"log",
		"-n",
		strconv.Itoa(switchLogLimit),
		"--date=relative",
		"--format=%H%x09%h%x09%ar%x09%s",
		"--numstat",
		branch,
	)
	if err := gitx.CommandError(fmt.Sprintf("read commit log for %q", branch), stderr, exitCode, runErr, "git log failed"); err != nil {
		b.WriteString("Preview unavailable: ")
		b.WriteString(err.Error())
		b.WriteString("\n")
		return strings.TrimRight(b.String(), "\n")
	}

	entries := parseSwitchLogEntries(stdout)
	if len(entries) == 0 {
		b.WriteString("No commits found.\n")
		return strings.TrimRight(b.String(), "\n")
	}

	b.WriteString(renderSwitchLogTable(entries))

	return strings.TrimRight(b.String(), "\n")
}

func renderSwitchLogTable(entries []switchLogEntry) string {
	var b strings.Builder
	b.WriteString(ansiGrey + "HASH     DIFF         AGE          MESSAGE" + ansiReset + "\n")
	for _, entry := range entries {
		hash := fmt.Sprintf("%-7s", entry.shortHash)
		diff := fmt.Sprintf("+%d -%d", entry.added, entry.deleted)
		b.WriteString(ansiGrey + hash + ansiReset)
		b.WriteString("  ")
		b.WriteString(ansiGreen + "+" + strconv.Itoa(entry.added) + ansiReset)
		b.WriteString(" ")
		b.WriteString(ansiDiffRed + "-" + strconv.Itoa(entry.deleted) + ansiReset)
		if pad := 11 - len(diff); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString("  ")
		b.WriteString(ansiGrey + fmt.Sprintf("%-12s", entry.age) + ansiReset)
		b.WriteString(" ")
		b.WriteString(entry.subject)
		b.WriteString("\n")
	}
	return b.String()
}

func renderSwitchDefaultDiffTab(commandCtx context.Context, repoCtx *gitx.RepoContext, branch string) string {
	defaultBranch := repoCtx.DefaultBranch
	if strings.TrimSpace(defaultBranch) == "" {
		defaultBranch = "main"
	}
	return renderSwitchDiffTab(commandCtx, repoCtx, branch, defaultBranch, "default branch")
}

func renderSwitchUpstreamDiffTab(commandCtx context.Context, repoCtx *gitx.RepoContext, branch string) string {
	upstream, err := branchUpstream(commandCtx, repoCtx, branch)
	if err != nil {
		return "Diff vs upstream\n----------------\nPreview unavailable: " + err.Error()
	}
	if strings.TrimSpace(upstream) == "" {
		return "Diff vs upstream\n----------------\nBranch has no upstream tracking branch."
	}
	return renderSwitchDiffTab(commandCtx, repoCtx, branch, upstream, "upstream")
}

func renderSwitchDiffTab(commandCtx context.Context, repoCtx *gitx.RepoContext, branch string, againstRef string, refLabel string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("Branch: ")
	b.WriteString(branch)
	b.WriteString("\n")
	b.WriteString("Compare: ")
	b.WriteString(againstRef)
	b.WriteString("...")
	b.WriteString(branch)
	b.WriteString("\n\n")

	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(
		commandCtx,
		repoCtx,
		"diff",
		"--stat",
		"--stat-width=100",
		"--stat-name-width=56",
		"--stat-graph-width=16",
		againstRef+"..."+branch,
	)
	if err := gitx.CommandError(fmt.Sprintf("render diff for %q against %q", branch, againstRef), stderr, exitCode, runErr, "git diff failed"); err != nil {
		b.WriteString("Preview unavailable: ")
		b.WriteString(err.Error())
		b.WriteString("\n")
		return strings.TrimRight(b.String(), "\n")
	}

	if strings.TrimSpace(stdout) == "" {
		b.WriteString("No diff between ")
		b.WriteString(branch)
		b.WriteString(" and ")
		b.WriteString(refLabel)
		b.WriteString(" (")
		b.WriteString(againstRef)
		b.WriteString(").\n")
		return strings.TrimRight(b.String(), "\n")
	}

	b.WriteString(colorizeDiffStat(stdout))
	return strings.TrimRight(b.String(), "\n")
}

func colorizeDiffStat(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if strings.Contains(trimmed, "|") {
			lines[i] = colorizeDiffStatLine(trimmed)
			continue
		}
		if strings.Contains(trimmed, " files changed") || strings.Contains(trimmed, " file changed") {
			lines[i] = colorizeSummaryLine(trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

func colorizeDiffStatLine(line string) string {
	parts := strings.SplitN(line, "|", 2)
	if len(parts) != 2 {
		return line
	}
	left := strings.TrimRight(parts[0], " ")
	right := strings.TrimSpace(parts[1])

	rightFields := strings.Fields(right)
	if len(rightFields) < 2 {
		return line
	}

	changed := rightFields[0]
	graph := strings.Join(rightFields[1:], " ")
	graphColored := strings.ReplaceAll(graph, "+", ansiGreen+"+"+ansiReset)
	graphColored = strings.ReplaceAll(graphColored, "-", ansiDiffRed+"-"+ansiReset)

	return fmt.Sprintf("%s%-56s%s | %2s %s", ansiGrey, left, ansiReset, changed, graphColored)
}

func colorizeSummaryLine(line string) string {
	fields := strings.Split(line, ",")
	for i, segment := range fields {
		segment = strings.TrimSpace(segment)
		if strings.Contains(segment, "insertion") {
			segment = strings.ReplaceAll(segment, "(", "")
			segment = strings.ReplaceAll(segment, ")", "")
			fields[i] = ansiGreen + segment + ansiReset
			continue
		}
		if strings.Contains(segment, "deletion") {
			segment = strings.ReplaceAll(segment, "(", "")
			segment = strings.ReplaceAll(segment, ")", "")
			fields[i] = ansiDiffRed + segment + ansiReset
			continue
		}
		fields[i] = ansiGrey + segment + ansiReset
	}
	return strings.Join(fields, ", ")
}

func branchUpstream(commandCtx context.Context, repoCtx *gitx.RepoContext, branch string) (string, error) {
	stdout, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, repoCtx, "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	if err := gitx.CommandError(fmt.Sprintf("resolve upstream for %q", branch), stderr, exitCode, runErr, "git for-each-ref failed"); err != nil {
		return "", err
	}

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", nil
}

type switchLogEntry struct {
	fullHash  string
	shortHash string
	age       string
	subject   string
	added     int
	deleted   int
}

func parseSwitchLogEntries(stdout string) []switchLogEntry {
	entries := make([]switchLogEntry, 0, switchLogLimit)
	var current *switchLogEntry

	flush := func() {
		if current != nil {
			entries = append(entries, *current)
			current = nil
		}
	}

	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if fullHash, shortHash, age, subject, ok := parseSwitchLogHeader(line); ok {
			flush()
			current = &switchLogEntry{
				fullHash:  fullHash,
				shortHash: shortHash,
				age:       age,
				subject:   subject,
			}
			continue
		}

		if current == nil {
			continue
		}

		added, deleted, ok := parseSwitchNumstatLine(line)
		if !ok {
			continue
		}
		current.added += added
		current.deleted += deleted
	}

	flush()
	return entries
}

func parseSwitchLogHeader(line string) (fullHash string, shortHash string, age string, subject string, ok bool) {
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	fullHash = strings.TrimSpace(parts[0])
	if !looksLikeFullCommitHash(fullHash) {
		return "", "", "", "", false
	}
	shortHash = strings.TrimSpace(parts[1])
	age = strings.TrimSpace(parts[2])
	subject = strings.TrimSpace(parts[3])
	if shortHash == "" || age == "" || subject == "" {
		return "", "", "", "", false
	}
	return fullHash, shortHash, age, subject, true
}

func parseSwitchNumstatLine(line string) (added int, deleted int, ok bool) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		return 0, 0, false
	}

	addedVal := 0
	if parts[0] != "-" {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, false
		}
		addedVal = n
	}

	deletedVal := 0
	if parts[1] != "-" {
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
		deletedVal = n
	}

	return addedVal, deletedVal, true
}

func looksLikeFullCommitHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}
	for _, r := range hash {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
