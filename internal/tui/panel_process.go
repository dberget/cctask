package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
)

func renderProcessPanel(processes []model.ClaudeProcess, selectedIndex int, isFocused bool) string {
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

	for i, proc := range processes {
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

		line := lipgloss.NewStyle().Foreground(nameColor).Render(indicator) +
			processStatusSymbol(string(proc.Status)) + " " +
			lipgloss.NewStyle().Bold(isSelected).Foreground(nameColor).Render(truncate(proc.Label, 20)) +
			sessionTag
		lines = append(lines, line)

		outputLine := "    " + styleGray.Render(lastLine(proc.Output))
		lines = append(lines, outputLine)
		lines = append(lines, "")
	}

	if isFocused {
		lines = append(lines, styleGray.Render("Enter: full view"))
		lines = append(lines, styleGray.Render("o: continue in claude"))
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
