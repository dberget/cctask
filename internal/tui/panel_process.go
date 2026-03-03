package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
)

func renderProcessPanel(processes []model.ClaudeProcess, selectedIndex int, isFocused bool, pg paginator.Model) string {
	if len(processes) == 0 {
		return ""
	}

	titleColor := colorSecondary
	if isFocused {
		titleColor = colorPrimary
	}

	var lines []string
	header := lipgloss.NewStyle().Bold(true).Foreground(titleColor).
		Render(fmt.Sprintf("Processes (%d)", len(processes)))
	lines = append(lines, header)
	lines = append(lines, horizontalLine(28))
	lines = append(lines, "")

	// Determine which processes to show on the current page
	start, end := pg.GetSliceBounds(len(processes))

	for i := start; i < end; i++ {
		proc := processes[i]
		isSelected := isFocused && i == selectedIndex
		indicator := "  "
		if isSelected {
			indicator = "▸ "
		}

		nameColor := colorWhite
		if isSelected {
			nameColor = colorPrimary
		}

		sessionTag := ""
		if proc.SessionID != "" {
			sessionTag = styleDim.Render(" ↻")
		}

		elapsed := processElapsed(&processes[i])
		elapsedStr := ""
		if elapsed != "" {
			elapsedStr = styleDim.Render(" " + elapsed)
		}
		line := lipgloss.NewStyle().Foreground(nameColor).Render(indicator) +
			processStatusSymbol(string(proc.Status)) + " " +
			lipgloss.NewStyle().Bold(isSelected).Foreground(nameColor).Render(truncate(proc.Label, 18)) +
			sessionTag + elapsedStr
		lines = append(lines, line)

		outputLine := "    " + styleGray.Render(lastLine(proc.Output))
		lines = append(lines, outputLine)
		lines = append(lines, "")
	}

	// Show pagination dots if there are multiple pages
	if pg.TotalPages > 1 {
		lines = append(lines, pg.View())
		lines = append(lines, "")
	}

	if isFocused {
		lines = append(lines, styleGray.Render("Enter: full view"))
		hints := "x: cancel  o: claude"
		if pg.TotalPages > 1 {
			hints += "  [/]: page"
		}
		lines = append(lines, styleGray.Render(hints))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(processPanelWidth).PaddingLeft(1).Render(content)
}

func renderProcessDetail(proc *model.ClaudeProcess) string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(proc.Label)+
		"  "+lipgloss.NewStyle().Foreground(processStatusColor(string(proc.Status))).Render("["+string(proc.Status)+"]"))
	lines = append(lines, horizontalLine(50))
	lines = append(lines, "")

	output := proc.Output
	if output == "" {
		output = "Waiting for output..."
	}
	lines = append(lines, output)

	lines = append(lines, "")
	lines = append(lines, styleGray.Render("Esc: back to list"))

	return strings.Join(lines, "\n")
}
