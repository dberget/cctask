package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	listPanelWidth    = 38
	processPanelWidth = 32
	minDetailWidth    = 20
	maxDetailWidth    = 80
	separatorWidth    = 5 // "  │  "
)

func horizontalLine(width int) string {
	if width < 1 {
		width = 40
	}
	return styleGray.Render(strings.Repeat("─", width))
}

func verticalSeparator(height int) string {
	if height < 1 {
		height = 20
	}
	var lines []string
	for i := 0; i < height; i++ {
		lines = append(lines, styleGray.Render("│"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func truncate(s string, max int) string {
	if len(s) > max {
		if max <= 1 {
			return "…"
		}
		return s[:max-1] + "…"
	}
	return s
}

func truncateLines(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

func lastLine(text string) string {
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	line := lines[len(lines)-1]
	return truncate(line, 26)
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// wrapText wraps each line of text to fit within width, breaking on word boundaries.
func wrapText(text string, width int) string {
	if width < 10 {
		width = 10
	}
	lines := strings.Split(text, "\n")
	var wrapped []string
	for _, line := range lines {
		if len(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}
		for len(line) > width {
			breakAt := width
			// Try to break at a space
			for i := width; i > width/2; i-- {
				if line[i] == ' ' {
					breakAt = i
					break
				}
			}
			wrapped = append(wrapped, line[:breakAt])
			line = line[breakAt:]
			if len(line) > 0 && line[0] == ' ' {
				line = line[1:]
			}
		}
		if len(line) > 0 {
			wrapped = append(wrapped, line)
		}
	}
	return strings.Join(wrapped, "\n")
}

// renderScrollable applies vertical scrolling to content.
// Returns the visible portion. Offset is clamped to valid range.
func renderScrollable(content string, offset int, viewHeight int) string {
	lines := strings.Split(content, "\n")
	if viewHeight < 1 {
		viewHeight = 20
	}
	maxOffset := max(0, len(lines)-viewHeight)
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	end := min(offset+viewHeight, len(lines))
	visible := lines[offset:end]

	// Scroll indicator
	var indicator string
	if len(lines) > viewHeight {
		pct := 0
		if maxOffset > 0 {
			pct = (offset * 100) / maxOffset
		}
		indicator = styleGray.Render(fmt.Sprintf(" [%d%%  j/k:scroll  d/u:page  g/G:top/bottom]", pct))
	}

	return strings.Join(visible, "\n") + "\n" + indicator
}
