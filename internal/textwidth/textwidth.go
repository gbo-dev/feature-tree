package textwidth

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

const Ellipsis = "…"

var ellipsisWidth = Width(Ellipsis)

func Width(s string) int {
	g := uniseg.NewGraphemes(s)
	width := 0
	for g.Next() {
		width += runewidth.StringWidth(g.Str())
	}
	return width
}

func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if Width(s) <= max {
		return s
	}
	if max <= ellipsisWidth {
		return Ellipsis
	}

	allowed := max - ellipsisWidth
	g := uniseg.NewGraphemes(s)
	var b strings.Builder
	used := 0
	for g.Next() {
		cluster := g.Str()
		clusterWidth := runewidth.StringWidth(cluster)
		if used+clusterWidth > allowed {
			break
		}
		b.WriteString(cluster)
		used += clusterWidth
	}

	b.WriteString(Ellipsis)
	return b.String()
}
