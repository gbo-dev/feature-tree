package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	fzf "github.com/junegunn/fzf/src"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/textwidth"
	"github.com/gbo-dev/feature-tree/internal/uiansi"
)

var ErrSelectionCancelled = errors.New("selection cancelled")

const ansiBold = "\x1b[1m"

const (
	ansiPromptSwitch = "\x1b[38;2;135;135;255m"
	ansiPromptCreate = "\x1b[38;2;127;212;255m"
	ansiPromptRemove = "\x1b[38;2;255;143;143m"
)

// Column width caps in visible terminal columns (ANSI codes excluded).
const (
	branchDisplayMax   = 30 // right-truncated with ellipsis suffix (preserves meaningful prefix)
	pathDisplayMax     = 20 // right-truncated with ellipsis suffix (preserves relative-prefix context)
	commitDisplayMax   = 72 // subject-only commit column (hash omitted)
	pickerCommitMax    = 120
	pickerBranchBonus  = 4
	listCommitMax      = 26 // list HEAD needs to be narrower than fzf (max line len <= 105)
	stateDisplayMax    = 5  // longest value: "+!?" = 3 chars
	relationDisplayMax = 12 // longest typical value: "A: 99  B: 99" = 12 chars
	listLineMaxWidth   = 145
)

const (
	colTitleBranch   = "BRANCH"
	colTitlePath     = "PATH"
	colTitleState    = "STATE"
	colTitleRelation = "RELATION"
	colTitleHead     = "HEAD"
	colTitleCommit   = "COMMIT"
)

const (
	colWidthBranch   = len(colTitleBranch)
	colWidthPath     = len(colTitlePath)
	colWidthState    = len(colTitleState)
	colWidthRelation = len(colTitleRelation)
	colWidthCommit   = len(colTitleCommit)
)

const branchColMinWidth = colWidthBranch

func truncatePath(p string, max int) string {
	return textwidth.Truncate(p, max)
}

func truncateBranch(b string, max int) string {
	return textwidth.Truncate(b, max)
}

func truncateCell(s string, max int) string {
	return textwidth.Truncate(s, max)
}

type pickerRow struct {
	branch   string
	commit   gitx.CommitInfo
	path     string
	state    string
	relation string
	current  bool
	marker   string
	hidden   []string
}

type headerCol struct {
	title string
	width int
}

type rowLayout struct {
	branchWidth   int
	pathWidth     int
	stateWidth    int
	relationWidth int
	commitWidth   int
}

// computeLayout derives effective widths from content, then floors at title width.
func computeLayout(rows []pickerRow) rowLayout {
	l := rowLayout{branchWidth: branchColMinWidth}
	for _, row := range rows {
		if n := min(textwidth.Width(row.branch), branchDisplayMax); n > l.branchWidth {
			l.branchWidth = n
		}
		if n := min(textwidth.Width(row.path), pathDisplayMax); n > l.pathWidth {
			l.pathWidth = n
		}
		if row.state != "" {
			if n := min(textwidth.Width(row.state), stateDisplayMax); n > l.stateWidth {
				l.stateWidth = n
			}
		}
		if row.relation != "" {
			if n := min(textwidth.Width(row.relation), relationDisplayMax); n > l.relationWidth {
				l.relationWidth = n
			}
		}
		if s := row.commit.Display(commitDisplayMax); s != "" {
			if n := textwidth.Width(s); n > l.commitWidth {
				l.commitWidth = n
			}
		}
	}

	if l.pathWidth < colWidthPath {
		l.pathWidth = colWidthPath
	}
	if l.stateWidth > 0 && l.stateWidth < colWidthState {
		l.stateWidth = colWidthState
	}
	if l.relationWidth > 0 && l.relationWidth < colWidthRelation {
		l.relationWidth = colWidthRelation
	}
	if l.commitWidth > 0 && l.commitWidth < colWidthCommit {
		l.commitWidth = colWidthCommit
	}
	return l
}

func lineWidth(l rowLayout) int {
	width := 2 + l.branchWidth
	if l.pathWidth > 0 {
		width += 2 + l.pathWidth
	}
	if l.stateWidth > 0 {
		width += 2 + l.stateWidth
	}
	if l.relationWidth > 0 {
		width += 2 + l.relationWidth
	}
	if l.commitWidth > 0 {
		width += 2 + l.commitWidth
	}
	return width
}

// reduceWidth shrinks *v toward minWidth by up to overflow columns.
func reduceWidth(v *int, minWidth int, overflow int) int {
	if overflow <= 0 || *v <= minWidth {
		return overflow
	}
	canDrop := *v - minWidth
	drop := min(canDrop, overflow)
	*v -= drop
	return overflow - drop
}

// fitListLayout shrinks flexible columns until the rendered list row width is
// <= listLineMaxWidth. Shrink order prioritizes preserving branch/state context:
// PATH -> HEAD -> RELATION -> BRANCH -> STATE.
func fitListLayout(l rowLayout) rowLayout {
	overflow := lineWidth(l) - listLineMaxWidth
	if overflow <= 0 {
		return l
	}

	overflow = reduceWidth(&l.pathWidth, colWidthPath, overflow)
	overflow = reduceWidth(&l.commitWidth, colWidthCommit, overflow)
	overflow = reduceWidth(&l.relationWidth, colWidthRelation, overflow)
	overflow = reduceWidth(&l.branchWidth, branchColMinWidth, overflow)
	overflow = reduceWidth(&l.stateWidth, colWidthState, overflow)

	return l
}

func capListCommitWidth(l rowLayout) rowLayout {
	if l.commitWidth > listCommitMax {
		l.commitWidth = listCommitMax
	}
	return l
}

func maxBranchWidth(rows []pickerRow) int {
	w := branchColMinWidth
	for _, row := range rows {
		if n := textwidth.Width(row.branch); n > w {
			w = n
		}
	}
	return w
}

func maxCommitWidth(rows []pickerRow) int {
	w := 0
	for _, row := range rows {
		if s := row.commit.Display(pickerCommitMax); s != "" {
			if n := textwidth.Width(s); n > w {
				w = n
			}
		}
	}
	if w > 0 && w < colWidthCommit {
		w = colWidthCommit
	}
	return w
}

func expandBranchCommitLayout(rows []pickerRow, l rowLayout) rowLayout {
	spare := listLineMaxWidth - lineWidth(l)
	if spare <= 0 {
		return l
	}

	branchTarget := min(maxBranchWidth(rows), l.branchWidth+pickerBranchBonus)
	if branchTarget > l.branchWidth {
		add := min(branchTarget-l.branchWidth, spare)
		l.branchWidth += add
		spare -= add
	}

	if spare <= 0 {
		return l
	}

	commitTarget := maxCommitWidth(rows)
	if commitTarget > l.commitWidth {
		add := min(commitTarget-l.commitWidth, spare)
		l.commitWidth += add
	}

	return l
}

func promptLabel(name string, color string) string {
	return color + name + uiansi.Reset + "> "
}

func currentWorktreePath(entries []gitx.Worktree, currentBranch string) string {
	for _, worktree := range entries {
		if worktree.Branch == currentBranch {
			return worktree.Path
		}
	}
	return ""
}

func PickSwitchBranch(commandCtx context.Context, entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext) (string, error) {
	branches := make([]string, len(entries))
	for i, worktree := range entries {
		branches[i] = worktree.Branch
	}
	commits := gitx.FetchCommitsParallel(commandCtx, ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(entries))
	for i, worktree := range entries {
		dirty, err := gitx.DirtySymbols(commandCtx, worktree.Path)
		if err != nil {
			dirty = "?"
		}
		relation, err := gitx.BranchRelation(commandCtx, ctx, worktree.Branch)
		if err != nil {
			relation = "?"
		}
		m := ""
		if worktree.Branch == ctx.DefaultBranch && worktree.Branch != currentBranch {
			m = "^"
		}
		rows = append(rows, pickerRow{
			branch:   worktree.Branch,
			commit:   commits[i],
			path:     gitx.RelativePath(worktree.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  worktree.Branch == currentBranch,
			marker:   m,
		})
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("no worktrees available")
	}

	previewCache, cleanupPreviewCache, cacheErr := buildSwitchPreviewCache(commandCtx, ctx, rows)
	if cacheErr == nil {
		defer cleanupPreviewCache()
		for i := range rows {
			if tabs, ok := previewCache.tabsByBranch[rows[i].branch]; ok {
				rows[i].hidden = []string{tabs.headPath, tabs.logPath, tabs.defaultDiffPath, tabs.upstreamDiffPath}
			}
		}
	}

	l := computeLayout(rows)
	l.pathWidth = 0
	l = fitListLayout(l)
	l = expandBranchCommitLayout(rows, l)
	lines := buildFZFLines(rows, l)
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleHead, l.commitWidth},
	})
	extraArgs := []string{
		"--header=" + header,
	}
	if cacheErr == nil {
		nextTabCommand := switchPreviewStateCommand(previewCache.stateFile, 1)
		prevTabCommand := switchPreviewStateCommand(previewCache.stateFile, -1)
		renderCommand := switchPreviewRenderCommand(previewCache.stateFile)
		extraArgs = append(extraArgs,
			"--preview="+renderCommand,
			"--preview-window=down,70%,wrap,border-top,~1,noinfo",
			"--bind=tab:execute-silent("+nextTabCommand+")+refresh-preview",
			"--bind=btab:execute-silent("+prevTabCommand+")+refresh-preview",
			"--bind=shift-tab:execute-silent("+prevTabCommand+")+refresh-preview",
			"--bind=right:execute-silent("+nextTabCommand+")+refresh-preview",
			"--bind=left:execute-silent("+prevTabCommand+")+refresh-preview",
		)
	}

	return pickBranch(lines, promptLabel("switch", ansiPromptSwitch), extraArgs...)
}

func PickCreateBranch(commandCtx context.Context, entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext, includeAllBranches bool) (string, error) {
	branches := make([]string, 0, len(entries))
	worktreeByBranch := make(map[string]gitx.Worktree, len(entries))
	for _, worktree := range entries {
		branches = append(branches, worktree.Branch)
		worktreeByBranch[worktree.Branch] = worktree
	}

	if includeAllBranches {
		localBranches, err := gitx.ListLocalBranches(commandCtx, ctx)
		if err != nil {
			return "", err
		}
		for _, branch := range localBranches {
			if _, ok := worktreeByBranch[branch]; ok {
				continue
			}
			branches = append(branches, branch)
		}
	}

	commits := gitx.FetchCommitsParallel(commandCtx, ctx, branches)
	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(branches))
	for i, branch := range branches {
		worktree, hasWorktree := worktreeByBranch[branch]
		path := "(no worktree)"
		state := "-"
		if hasWorktree {
			path = gitx.RelativePath(worktree.Path, fromPath)
			dirty, err := gitx.DirtySymbols(commandCtx, worktree.Path)
			if err != nil {
				dirty = "?"
			}
			state = gitx.DirtyState(dirty)
		}
		relation, err := gitx.BranchRelation(commandCtx, ctx, branch)
		if err != nil {
			relation = "?"
		}
		m := ""
		if branch == ctx.DefaultBranch && branch != currentBranch {
			m = "^"
		}
		rows = append(rows, pickerRow{
			branch:   branch,
			commit:   commits[i],
			path:     path,
			state:    state,
			relation: relation,
			current:  branch == currentBranch,
			marker:   m,
		})
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("no branches available")
	}

	l := computeLayout(rows)
	l.pathWidth = 0
	l = fitListLayout(l)
	l = expandBranchCommitLayout(rows, l)
	lines := buildFZFLines(rows, l)
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleCommit, l.commitWidth},
	})
	return pickBranch(lines, promptLabel("create", ansiPromptCreate),
		"--header="+header,
	)
}

func PickRemoveBranch(commandCtx context.Context, entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext) (string, error) {
	branches := make([]string, 0, len(entries))
	for _, worktree := range entries {
		if worktree.Branch != ctx.DefaultBranch {
			branches = append(branches, worktree.Branch)
		}
	}
	commits := gitx.FetchCommitsParallel(commandCtx, ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(branches))
	ci := 0
	for _, worktree := range entries {
		if worktree.Branch == ctx.DefaultBranch {
			continue
		}

		dirty, err := gitx.DirtySymbols(commandCtx, worktree.Path)
		if err != nil {
			dirty = "?"
		}
		relation, err := gitx.BranchRelation(commandCtx, ctx, worktree.Branch)
		if err != nil {
			relation = "?"
		}

		rows = append(rows, pickerRow{
			branch:   worktree.Branch,
			commit:   commits[ci],
			path:     gitx.RelativePath(worktree.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  worktree.Branch == currentBranch,
		})
		ci++
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("no removable worktrees available")
	}

	l := computeLayout(rows)
	lines := buildFZFLines(rows, l)
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitlePath, l.pathWidth},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleHead, l.commitWidth},
	})
	return pickBranch(lines, promptLabel("remove", ansiPromptRemove),
		"--header="+header,
	)
}

func pickerHeader(branchWidth int, cols []headerCol) string {
	branchTitle := cols[0].title
	pad := branchWidth - len(branchTitle) + 2

	line := uiansi.Grey + ansiBold + "  " + branchTitle + strings.Repeat(" ", pad)
	for _, col := range cols[1:] {
		if col.width > len(col.title) {
			line += col.title + strings.Repeat(" ", col.width-len(col.title)) + "  "
		} else {
			line += col.title + "  "
		}
	}
	return strings.TrimRight(line, " ") + uiansi.Reset
}

func pickBranch(lines []string, prompt string, extraArgs ...string) (string, error) {
	selected, err := runFZF(lines, prompt, extraArgs...)
	if err != nil {
		if errors.Is(err, ErrSelectionCancelled) {
			return "", err
		}
		return "", fmt.Errorf("interactive picker failed: %w", err)
	}
	return selected, nil
}

// fzfBaseArgs returns the shared fzf options.
// --gutter=' ' + gutter:-1 keeps the non-selected-row gutter invisible;
// the default '▌' half-block rendered in bg+ teal was visible on every row.
func fzfBaseArgs() []string {
	return []string{
		"--ansi",
		"--delimiter=\t",
		"--with-nth=1",
		"--layout=reverse",
		"--height=~100%",
		"--min-height=26+",
		"--border=sharp",
		"--pointer=◆",
		"--marker=>",
		"--gutter= ",
		"--separator=-",
		"--scrollbar=│",
		"--info=right",
		"--color=fg:-1,fg+:#f4ede0:regular,bg:-1,bg+:#1a2e2c",
		"--color=hl:#48b08f,hl+:#6ce4be,info:" + uiansi.InfoPurpleHex + ",marker:#00a6ff",
		"--color=prompt:-1,spinner:#a2b9b9,pointer:#5e7eff,header:#87afaf",
		"--color=gutter:-1,border:#202020,label:#aeaeae,query:#d9d9d9:regular",
	}
}

func runFZF(lines []string, prompt string, extraArgs ...string) (string, error) {
	inputCh := make(chan string)
	outputCh := make(chan string, 1)

	args := append(fzfBaseArgs(), "--prompt="+prompt)
	args = append(args, extraArgs...)

	opts, err := fzf.ParseOptions(true, args)
	if err != nil {
		return "", fmt.Errorf("fzf options: %w", err)
	}
	opts.Input = inputCh
	opts.Output = outputCh

	go func() {
		defer close(inputCh)
		for _, line := range lines {
			inputCh <- line
		}
	}()

	exitCode, runErr := fzf.Run(opts)

	switch exitCode {
	case fzf.ExitOk:
		var selected string
		select {
		case s, ok := <-outputCh:
			if ok {
				selected = s
			}
		default:
		}
		if strings.TrimSpace(selected) == "" {
			return "", ErrSelectionCancelled
		}
		return parseSelectedBranch(selected)
	case fzf.ExitNoMatch, fzf.ExitInterrupt:
		return "", ErrSelectionCancelled
	default:
		if runErr != nil {
			return "", fmt.Errorf("fzf: %w", runErr)
		}
		return "", fmt.Errorf("fzf exited with code %d", exitCode)
	}
}

// buildFZFLines emits "display\tbranch[\thidden...]". The hidden payload fields
// after the first tab are for preview and parseSelectedBranch ignores them.
func buildFZFLines(rows []pickerRow, l rowLayout) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		var prefix string
		switch {
		case row.current:
			prefix = uiansi.Green + "@ " + uiansi.Reset
		case row.marker == "^":
			prefix = uiansi.Periwinkle + "^ " + uiansi.Reset
		default:
			prefix = "  "
		}

		b := truncateBranch(row.branch, l.branchWidth)
		branchField := b + strings.Repeat(" ", l.branchWidth-textwidth.Width(b)+2)

		var parts []string

		if l.pathWidth > 0 {
			rendered := truncatePath(row.path, l.pathWidth)
			parts = append(parts, uiansi.Grey+rendered+uiansi.Reset+strings.Repeat(" ", l.pathWidth-textwidth.Width(rendered)))
		}

		if l.stateWidth > 0 && row.state != "" {
			st := truncateCell(row.state, l.stateWidth)
			pad := strings.Repeat(" ", l.stateWidth-textwidth.Width(st))
			if row.state != "clean" {
				parts = append(parts, uiansi.Yellow+st+uiansi.Reset+pad)
			} else {
				parts = append(parts, st+pad)
			}
		}

		if l.relationWidth > 0 && row.relation != "" {
			rel := truncateCell(row.relation, l.relationWidth)
			parts = append(parts, uiansi.Grey+rel+uiansi.Reset+strings.Repeat(" ", l.relationWidth-textwidth.Width(rel)))
		}

		if l.commitWidth > 0 {
			if s := row.commit.Display(l.commitWidth); s != "" {
				parts = append(parts, uiansi.Grey+s+uiansi.Reset+strings.Repeat(" ", l.commitWidth-textwidth.Width(s)))
			}
		}

		display := prefix + branchField + strings.Join(parts, "  ")
		payload := row.branch
		if len(row.hidden) > 0 {
			payload += "\t" + strings.Join(row.hidden, "\t")
		}
		lines = append(lines, strings.TrimRight(display, " ")+"\t"+payload)
	}
	return lines
}

func parseSelectedBranch(selected string) (string, error) {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return "", ErrSelectionCancelled
	}

	if strings.Contains(selected, "\t") {
		parts := strings.Split(selected, "\t")
		if len(parts) < 2 {
			return "", fmt.Errorf("could not extract branch from fzf output")
		}
		branch := strings.TrimSpace(parts[1])
		if branch != "" {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not extract branch from fzf output")
}

func switchPreviewStateCommand(stateFile string, step int) string {
	exe := "ft"
	if resolved, err := os.Executable(); err == nil && strings.TrimSpace(resolved) != "" {
		exe = resolved
	}
	return strconv.Quote(exe) + " __picker-preview-state --state-file " + strconv.Quote(stateFile) + " --step " + strconv.Itoa(step)
}

func switchPreviewRenderCommand(stateFile string) string {
	exe := "ft"
	if resolved, err := os.Executable(); err == nil && strings.TrimSpace(resolved) != "" {
		exe = resolved
	}
	return strconv.Quote(exe) + " __picker-preview-tab --state-file " + strconv.Quote(stateFile) + " {3} {4} {5} {6}"
}

func PrintWorktreeList(commandCtx context.Context, entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext, w io.Writer) error {
	branches := make([]string, len(entries))
	for i, worktree := range entries {
		branches[i] = worktree.Branch
	}
	commits := gitx.FetchCommitsParallel(commandCtx, ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(entries))
	for i, worktree := range entries {
		m := ""
		if worktree.Branch == ctx.DefaultBranch && worktree.Branch != currentBranch {
			m = "^"
		}

		dirty, err := gitx.DirtySymbols(commandCtx, worktree.Path)
		if err != nil {
			dirty = "?"
		}

		relation, err := gitx.BranchRelation(commandCtx, ctx, worktree.Branch)
		if err != nil {
			relation = "?"
		}

		rows = append(rows, pickerRow{
			branch:   worktree.Branch,
			commit:   commits[i],
			path:     gitx.RelativePath(worktree.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  worktree.Branch == currentBranch,
			marker:   m,
		})
	}

	l := capListCommitWidth(fitListLayout(computeLayout(rows)))
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitlePath, l.pathWidth},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleHead, l.commitWidth},
	})
	fmt.Fprintln(w, header)

	for _, line := range buildFZFLines(rows, l) {
		// Strip the hidden "\t<branch>" suffix — only the display column is printed.
		if idx := strings.Index(line, "\t"); idx >= 0 {
			line = line[:idx]
		}
		fmt.Fprintln(w, line)
	}

	return nil
}
