package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/davidberget/cctask-go/internal/model"
)

func keyHint(k, label string) string {
	return styleCyanBold.Render(k) + " " + styleDim.Render(label)
}

func renderStatusBar(h help.Model, keys KeyBindings, mode model.ViewMode, selected *model.ListItem, message string, cols int, serverRunning bool) string {
	lineWidth := cols - 4
	if lineWidth < 40 {
		lineWidth = 40
	}

	bindings := modeShortHelp(keys, mode, selected)
	var hints []string
	for _, b := range bindings {
		if b.Help().Key != "" {
			hints = append(hints, keyHint(b.Help().Key, b.Help().Desc))
		}
	}

	hintLine := strings.Join(hints, "  ")
	if message != "" {
		hintLine += "  " + styleYellow.Render(message)
	}
	if serverRunning {
		hintLine += "  " + styleGreen.Render("●")
	}

	return horizontalLine(lineWidth) + "\n" + hintLine
}

func renderHelp(h help.Model, keys KeyBindings) string {
	hdr := func(title string) string { return styleCyanBold.Render(title) }
	k := func(ky, desc string) string {
		return "  " + styleCyanBold.Render(padRight(ky, 14)) + desc
	}

	var lines []string
	lines = append(lines, hdr("List View"))
	lines = append(lines, k("j/k, Up/Down", "Navigate items"))
	lines = append(lines, k("Enter", "Open detail / group view"))
	lines = append(lines, k("v", "Full-screen task view"))
	lines = append(lines, k("a", "Add new task"))
	lines = append(lines, k("A", "Bulk add tasks (paste list, Claude parses)"))
	lines = append(lines, k("e", "Edit task / plan"))
	lines = append(lines, k("d", "Delete task / project"))
	lines = append(lines, k("s", "Cycle status"))
	lines = append(lines, k("g", "Assign project"))
	lines = append(lines, k("r", "Run task with Claude"))
	lines = append(lines, k("p", "View / generate plan"))
	lines = append(lines, k("c", "Prompt Claude on scope"))
	lines = append(lines, k("m", "Merge plans"))
	lines = append(lines, k("x", "View/edit project context"))
	lines = append(lines, k("/", "Filter tasks"))
	lines = append(lines, k("H", "Toggle hide completed"))
	lines = append(lines, k("t", "Change theme"))
	lines = append(lines, k("T", "Table view"))
	lines = append(lines, k("L", "Flat list view"))
	lines = append(lines, k("Space", "Collapse / expand group"))
	lines = append(lines, k("Tab", "Switch to process panel"))
	lines = append(lines, k("q", "Quit"))
	lines = append(lines, "")

	lines = append(lines, hdr("Scroll (fullscreen views)"))
	lines = append(lines, k("j/k", "Scroll line"))
	lines = append(lines, k("d / Ctrl+D", "Half-page down"))
	lines = append(lines, k("u / Ctrl+U", "Half-page up"))
	lines = append(lines, k("gg", "Go to top"))
	lines = append(lines, k("G", "Go to bottom"))
	lines = append(lines, "")

	lines = append(lines, hdr("Task View"))
	lines = append(lines, k("r", "Run"))
	lines = append(lines, k("e", "Edit"))
	lines = append(lines, k("p", "Plan"))
	lines = append(lines, k("c", "Ask Claude"))
	lines = append(lines, k("s", "Cycle status"))
	lines = append(lines, k("o", "Open proof (PROOF-tagged tasks)"))
	lines = append(lines, "")

	lines = append(lines, hdr("Process Detail"))
	lines = append(lines, k("x", "Interrupt (cancel turn, keep session for resume)"))
	lines = append(lines, k("c", "Follow-up / queue message (works while running)"))
	lines = append(lines, k("o", "Open in full Claude"))
	lines = append(lines, k("[/]", "Page processes"))
	lines = append(lines, "")

	lines = append(lines, hdr("Context View"))
	lines = append(lines, k("e", "Edit context"))
	lines = append(lines, k("i", "Import file"))
	lines = append(lines, "  "+styleGray.Render("Global context from .cctask/context.md is prepended to all Claude prompts"))
	lines = append(lines, "")

	lines = append(lines, hdr("Text Input"))
	lines = append(lines, k("Enter", "Submit"))
	lines = append(lines, k("Esc", "Cancel"))
	lines = append(lines, k("Ctrl+A", "Jump to start"))
	lines = append(lines, k("Ctrl+E", "Jump to end"))
	lines = append(lines, k("Ctrl+W", "Delete word"))
	lines = append(lines, "")

	lines = append(lines, hdr("Plan Editor"))
	lines = append(lines, k("Ctrl+S", "Save"))
	lines = append(lines, k("Esc", "Cancel"))
	lines = append(lines, "")

	lines = append(lines, hdr("Command Bar"))
	lines = append(lines, k(":", "Open command bar"))
	lines = append(lines, k("Tab", "Complete command/arg"))
	lines = append(lines, k("Up/Down", "History / suggestions"))
	lines = append(lines, k("Enter", "Execute"))
	lines = append(lines, k("Esc", "Cancel"))
	lines = append(lines, "")

	lines = append(lines, styleGray.Render("Press ? or Esc to close"))

	_ = h     // help model available for future auto-generation
	_ = keys  // key bindings available for future auto-generation
	return strings.Join(lines, "\n")
}

// suppress unused import warning
var _ key.Binding
