package tui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	fzf "github.com/junegunn/fzf/src"

	"github.com/gbo-dev/wt/go-port/internal/gitx"
)

var ErrSelectionCancelled = errors.New("selection cancelled")

// Pastel ANSI 256-colour helpers (zero-width for layout purposes).
const (
	ansiReset      = "\x1b[0m"
	ansiGrey       = "\x1b[38;5;244m" // muted grey – paths
	ansiBold       = "\x1b[1m"        // bold
	ansiGreen      = "\x1b[38;5;114m" // pastel green – current marker
	ansiYellow     = "\x1b[38;5;228m" // pastel yellow – dirty state
	ansiPeriwinkle = "\x1b[38;5;111m" // periwinkle – default branch marker
)

// Column width caps in visible terminal columns (ANSI codes excluded).
const (
	branchDisplayMax   = 30 // right-truncated with ellipsis suffix (preserves meaningful prefix)
	pathDisplayMax     = 20 // right-truncated with ellipsis suffix (preserves relative-prefix context)
	commitDisplayMax   = 72 // subject-only commit column (hash omitted)
	listCommitMax      = 26 // list COMMIT needs to be narrower than fzf (max line len <= 105)
	stateDisplayMax    = 11 // longest value: "dirty (+!?)" = 11 chars
	relationDisplayMax = 12 // longest typical value: "A: 99  B: 99" = 12 chars
	listLineMaxWidth   = 145
)

const (
	colTitleBranch   = "BRANCH"
	colTitlePath     = "PATH"
	colTitleState    = "STATE"
	colTitleRelation = "RELATION"
	colTitleCommit   = "COMMIT"
)

const (
	colWidthBranch   = len(colTitleBranch)
	colWidthPath     = len(colTitlePath)
	colWidthState    = len(colTitleState)
	colWidthRelation = len(colTitleRelation)
	colWidthCommit   = len(colTitleCommit)
)

// Keep branch width in sync with the branch header title.
const branchColMinWidth = colWidthBranch

// ellipsis is the Unicode horizontal ellipsis (U+2026) used to mark truncated
// column values. It is 3 UTF-8 bytes and is treated as 1 visible terminal
// column for layout. All other content strings are ASCII, so only ellipsis
// needs special handling in width calculations.
const ellipsis = "\u2026"
const ellipsisWidth = 1

// visWidth returns visible terminal columns for s.
func visWidth(s string) int {
	return len(s) - strings.Count(s, ellipsis)*(3-ellipsisWidth)
}

func truncatePath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	return p[:max-ellipsisWidth] + ellipsis
}

func truncateBranch(b string, max int) string {
	if len(b) <= max {
		return b
	}
	return b[:max-ellipsisWidth] + ellipsis
}

func truncateCell(s string, max int) string {
	if visWidth(s) <= max {
		return s
	}
	if max <= ellipsisWidth {
		return ellipsis
	}
	return s[:max-ellipsisWidth] + ellipsis
}

type pickerRow struct {
	branch   string
	commit   gitx.CommitInfo
	path     string
	state    string // rendered state: "clean", "dirty", "dirty (+!?)" for switch/list/remove
	relation string // "A: 3  B: 0", "-" – remove and list
	current  bool
	marker   string // "^": default branch marker when not current
}

type headerCol struct {
	title string
	width int
}

// rowLayout stores effective visible widths; zero means optional column absent.
type rowLayout struct {
	branchWidth   int // ≤ branchDisplayMax; ≥ branchColMinWidth
	pathWidth     int // ≤ pathDisplayMax
	stateWidth    int // 0 = absent
	relationWidth int // 0 = absent
	commitWidth   int // 0 = absent
}

// computeLayout derives effective widths from content, then floors at title width.
func computeLayout(rows []pickerRow) rowLayout {
	l := rowLayout{branchWidth: branchColMinWidth}
	for _, row := range rows {
		if n := min(len(row.branch), branchDisplayMax); n > l.branchWidth {
			l.branchWidth = n
		}
		if n := min(len(row.path), pathDisplayMax); n > l.pathWidth {
			l.pathWidth = n
		}
		if row.state != "" {
			if n := min(len(row.state), stateDisplayMax); n > l.stateWidth {
				l.stateWidth = n
			}
		}
		if row.relation != "" {
			if n := min(len(row.relation), relationDisplayMax); n > l.relationWidth {
				l.relationWidth = n
			}
		}
		if s := row.commit.Display(commitDisplayMax); s != "" {
			if n := visWidth(s); n > l.commitWidth {
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
	width := 2 + l.branchWidth + 2 + l.pathWidth
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
// PATH -> COMMIT -> RELATION -> BRANCH -> STATE.
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

// capListCommitWidth applies the stricter list-mode commit cap.
func capListCommitWidth(l rowLayout) rowLayout {
	if l.commitWidth > listCommitMax {
		l.commitWidth = listCommitMax
	}
	return l
}

// currentWorktreePath returns currentBranch's worktree path, if present.
func currentWorktreePath(entries []gitx.Worktree, currentBranch string) string {
	for _, wt := range entries {
		if wt.Branch == currentBranch {
			return wt.Path
		}
	}
	return ""
}

func PickSwitchBranch(entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext) (string, error) {
	branches := make([]string, len(entries))
	for i, wt := range entries {
		branches[i] = wt.Branch
	}
	commits := gitx.FetchCommitsParallel(ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(entries))
	for i, wt := range entries {
		dirty, err := gitx.DirtySymbols(wt.Path)
		if err != nil {
			dirty = "?"
		}
		relation, err := gitx.BranchRelation(ctx, wt.Branch)
		if err != nil {
			relation = "?"
		}
		m := ""
		if wt.Branch == ctx.DefaultBranch && wt.Branch != currentBranch {
			m = "^"
		}
		rows = append(rows, pickerRow{
			branch:   wt.Branch,
			commit:   commits[i],
			path:     gitx.RelativePath(wt.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  wt.Branch == currentBranch,
			marker:   m,
		})
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("no worktrees available")
	}

	l := fitListLayout(computeLayout(rows))
	lines := buildFZFLines(rows, l)
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitlePath, l.pathWidth},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleCommit, l.commitWidth},
	})
	const promptColor = "#8787ff"
	return pickBranch(lines, "switch> ",
		"--color=prompt:"+promptColor,
		"--header="+header,
	)
}

func PickRemoveBranch(entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext) (string, error) {
	branches := make([]string, 0, len(entries))
	for _, wt := range entries {
		if wt.Branch != ctx.DefaultBranch {
			branches = append(branches, wt.Branch)
		}
	}
	commits := gitx.FetchCommitsParallel(ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(branches))
	ci := 0
	for _, wt := range entries {
		if wt.Branch == ctx.DefaultBranch {
			continue
		}

		dirty, err := gitx.DirtySymbols(wt.Path)
		if err != nil {
			dirty = "?"
		}
		relation, err := gitx.BranchRelation(ctx, wt.Branch)
		if err != nil {
			relation = "?"
		}

		rows = append(rows, pickerRow{
			branch:   wt.Branch,
			commit:   commits[ci],
			path:     gitx.RelativePath(wt.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  wt.Branch == currentBranch,
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
		{colTitleCommit, l.commitWidth},
	})
	const promptColor = "#ff8f8f"
	return pickBranch(lines, "remove> ",
		"--color=prompt:"+promptColor,
		"--header="+header,
	)
}

// pickerHeader builds the styled title row used by fzf --header.
func pickerHeader(branchWidth int, cols []headerCol) string {
	branchTitle := cols[0].title
	// +2 matches the two-space separator appended after branchField in buildFZFLines.
	pad := branchWidth - len(branchTitle) + 2

	line := ansiGrey + ansiBold + "  " + branchTitle + strings.Repeat(" ", pad)
	for _, col := range cols[1:] {
		if col.width > len(col.title) {
			line += col.title + strings.Repeat(" ", col.width-len(col.title)) + "  "
		} else {
			line += col.title + "  "
		}
	}
	return strings.TrimRight(line, " ") + ansiReset
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
		"--height=~40%",
		"--border=sharp",
		"--pointer=◆",
		"--marker=>",
		"--gutter= ",
		"--separator=-",
		"--scrollbar=│",
		"--info=right",
		"--color=fg:-1,fg+:#f4ede0:regular,bg:-1,bg+:#1a2e2c",
		"--color=hl:#48b08f,hl+:#6ce4be,info:#8788b0,marker:#00a6ff",
		"--color=prompt:#00d6ba,spinner:#a2b9b9,pointer:#5e7eff,header:#87afaf",
		"--color=gutter:-1,border:#202020,label:#aeaeae,query:#d9d9d9:regular",
	}
}

// runFZF runs the embedded fzf engine over pre-built display lines.
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

// buildFZFLines emits "display\tbranch". The hidden branch payload after the
// tab is used by parseSelectedBranch to extract the raw selection.
func buildFZFLines(rows []pickerRow, l rowLayout) []string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		var prefix string
		switch {
		case row.current:
			prefix = ansiGreen + "@ " + ansiReset
		case row.marker == "^":
			prefix = ansiPeriwinkle + "^ " + ansiReset
		default:
			prefix = "  "
		}

		b := truncateBranch(row.branch, l.branchWidth)
		branchField := b + strings.Repeat(" ", l.branchWidth-visWidth(b)+2)

		var parts []string

		rendered := truncatePath(row.path, l.pathWidth)
		parts = append(parts, ansiGrey+rendered+ansiReset+strings.Repeat(" ", l.pathWidth-visWidth(rendered)))

		if l.stateWidth > 0 && row.state != "" {
			st := truncateCell(row.state, l.stateWidth)
			pad := strings.Repeat(" ", l.stateWidth-visWidth(st))
			if strings.HasPrefix(row.state, "dirty") {
				parts = append(parts, ansiYellow+st+ansiReset+pad)
			} else {
				parts = append(parts, st+pad)
			}
		}

		if l.relationWidth > 0 && row.relation != "" {
			rel := row.relation
			if len(rel) > l.relationWidth {
				rel = rel[:l.relationWidth]
			}
			parts = append(parts, ansiGrey+rel+ansiReset+strings.Repeat(" ", l.relationWidth-len(rel)))
		}

		// Use visWidth (not len): commit Display may include ellipsis.
		if l.commitWidth > 0 {
			if s := row.commit.Display(l.commitWidth); s != "" {
				parts = append(parts, ansiGrey+s+ansiReset+strings.Repeat(" ", l.commitWidth-visWidth(s)))
			}
		}

		display := prefix + branchField + strings.Join(parts, "  ")
		lines = append(lines, strings.TrimRight(display, " ")+"\t"+row.branch)
	}
	return lines
}

// parseSelectedBranch reads the hidden branch payload after the tab separator.
func parseSelectedBranch(selected string) (string, error) {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return "", ErrSelectionCancelled
	}

	if idx := strings.Index(selected, "\t"); idx >= 0 {
		branch := strings.TrimSpace(selected[idx+1:])
		if branch != "" {
			return branch, nil
		}
	}

	return "", fmt.Errorf("could not extract branch from fzf output")
}

// PrintWorktreeList writes a non-interactive table of worktrees.
func PrintWorktreeList(entries []gitx.Worktree, currentBranch string, ctx *gitx.RepoContext, w io.Writer) error {
	branches := make([]string, len(entries))
	for i, wt := range entries {
		branches[i] = wt.Branch
	}
	commits := gitx.FetchCommitsParallel(ctx, branches)

	fromPath := currentWorktreePath(entries, currentBranch)

	rows := make([]pickerRow, 0, len(entries))
	for i, wt := range entries {
		m := ""
		if wt.Branch == ctx.DefaultBranch && wt.Branch != currentBranch {
			m = "^"
		}

		dirty, err := gitx.DirtySymbols(wt.Path)
		if err != nil {
			dirty = "?"
		}

		relation, err := gitx.BranchRelation(ctx, wt.Branch)
		if err != nil {
			relation = "?"
		}

		rows = append(rows, pickerRow{
			branch:   wt.Branch,
			commit:   commits[i],
			path:     gitx.RelativePath(wt.Path, fromPath),
			state:    gitx.DirtyState(dirty),
			relation: relation,
			current:  wt.Branch == currentBranch,
			marker:   m,
		})
	}

	l := capListCommitWidth(fitListLayout(computeLayout(rows)))
	header := pickerHeader(l.branchWidth, []headerCol{
		{colTitleBranch, 0},
		{colTitlePath, l.pathWidth},
		{colTitleState, l.stateWidth},
		{colTitleRelation, l.relationWidth},
		{colTitleCommit, l.commitWidth},
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
