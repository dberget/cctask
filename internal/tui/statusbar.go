package tui

import (
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
)

func keyHint(key, label string) string {
	return styleCyanBold.Render(key) + " " + styleDim.Render(label)
}

func renderStatusBar(mode model.ViewMode, selected *model.ListItem, message string, cols int) string {
	lineWidth := cols - 4
	if lineWidth < 40 {
		lineWidth = 40
	}

	var hints []string
	switch mode {
	case model.ModeList:
		hints = listHints(selected)
	case model.ModeDetail:
		hints = []string{
			keyHint("e", "edit desc"), keyHint("r", "run"), keyHint("p", "plan"),
			keyHint("s", "status"), keyHint("Esc", "back"),
		}
	case model.ModePlan:
		hints = []string{
			keyHint("r", "run"), keyHint("e", "edit"), keyHint("Esc", "back"),
		}
	case model.ModeGroupDetail:
		hints = []string{
			keyHint("r", "run"), keyHint("p", "plan"), keyHint("d", "delete"),
			keyHint("Esc", "back"),
		}
	case model.ModeTaskForm:
		hints = []string{
			keyHint("Tab", "next field"), keyHint("Enter", "next/submit"),
			keyHint("Ctrl+S", "save"), keyHint("Esc", "cancel"),
		}
	case model.ModeCombineSelect:
		hints = []string{
			keyHint("Space", "toggle"), keyHint("Enter", "confirm"),
			keyHint("Esc", "cancel"),
		}
	case model.ModeProcessDetail:
		hints = []string{keyHint("o", "continue in claude"), keyHint("Esc", "back")}
	case model.ModeEditPlan:
		hints = []string{
			keyHint("i", "insert"), keyHint("Ctrl+S", "save"),
			keyHint("q/Esc", "cancel"),
		}
	case model.ModeTaskView:
		hints = []string{
			keyHint("r", "run"), keyHint("e", "edit"), keyHint("p", "plan"),
			keyHint("c", "ask claude"), keyHint("s", "status"),
			keyHint("Esc", "back"),
		}
	case model.ModeHelp:
		hints = []string{
			keyHint("?/Esc", "back"),
		}
	case model.ModeTaskViewAsk, model.ModeGroupPrompt:
		hints = []string{
			keyHint("Enter", "send"), keyHint("Esc", "cancel"),
		}
	default:
		hints = []string{
			keyHint("Enter", "confirm"), keyHint("Esc", "cancel"),
		}
	}

	hintLine := strings.Join(hints, "  ")
	if message != "" {
		hintLine += "  " + styleYellow.Render(message)
	}

	return horizontalLine(lineWidth) + "\n" + hintLine
}

func listHints(sel *model.ListItem) []string {
	isTask := sel != nil && sel.Kind == model.ListItemTask
	isGroup := sel != nil && sel.Kind == model.ListItemProject
	hasSelection := isTask || isGroup

	var h []string
	h = append(h, keyHint("a", "add"))
	if hasSelection {
		h = append(h, keyHint("e", "edit"))
		h = append(h, keyHint("d", "delete"))
	}
	if isTask {
		h = append(h, keyHint("g", "project"))
	} else if isGroup {
		h = append(h, keyHint("g", "subgroup"))
	} else {
		h = append(h, keyHint("g", "project"))
	}
	if hasSelection {
		h = append(h, keyHint("r", "run"))
		h = append(h, keyHint("p", "plan"))
	}
	if isTask {
		h = append(h, keyHint("s", "status"))
	}
	h = append(h, keyHint("c", "prompt"))
	h = append(h, keyHint("/", "filter"))
	if isGroup && sel.Project != nil && sel.Project.PlanFile != "" {
		h = append(h, keyHint("v", "view plan"))
	}
	h = append(h, keyHint("m", "merge"))
	if hasSelection {
		h = append(h, keyHint("Enter", "detail"))
	}
	if isGroup {
		h = append(h, keyHint("Space", "collapse"))
	}
	h = append(h, keyHint("?", "help"), keyHint("q", "quit"))
	return h
}

func renderHelp() string {
	h := func(title string) string { return styleCyanBold.Render(title) }
	k := func(key, desc string) string {
		return "  " + styleCyanBold.Render(padRight(key, 14)) + desc
	}

	var lines []string
	lines = append(lines, h("List View"))
	lines = append(lines, k("j/k, Up/Down", "Navigate items"))
	lines = append(lines, k("Enter", "Open detail / group view"))
	lines = append(lines, k("v", "Full-screen task view"))
	lines = append(lines, k("a", "Add new task"))
	lines = append(lines, k("e", "Edit task / plan"))
	lines = append(lines, k("d", "Delete task / project"))
	lines = append(lines, k("s", "Cycle status"))
	lines = append(lines, k("g", "Assign project"))
	lines = append(lines, k("r", "Run task with Claude"))
	lines = append(lines, k("p", "View / generate plan"))
	lines = append(lines, k("c", "Prompt Claude on scope"))
	lines = append(lines, k("m", "Merge plans"))
	lines = append(lines, k("/", "Filter tasks"))
	lines = append(lines, k("t", "Change theme"))
	lines = append(lines, k("Space", "Collapse / expand group"))
	lines = append(lines, k("Tab", "Switch to process panel"))
	lines = append(lines, k("q", "Quit"))
	lines = append(lines, "")

	lines = append(lines, h("Scroll (fullscreen views)"))
	lines = append(lines, k("j/k", "Scroll line"))
	lines = append(lines, k("d / Ctrl+D", "Half-page down"))
	lines = append(lines, k("u / Ctrl+U", "Half-page up"))
	lines = append(lines, k("gg", "Go to top"))
	lines = append(lines, k("G", "Go to bottom"))
	lines = append(lines, "")

	lines = append(lines, h("Task View"))
	lines = append(lines, k("r", "Run"))
	lines = append(lines, k("e", "Edit"))
	lines = append(lines, k("p", "Plan"))
	lines = append(lines, k("c", "Ask Claude"))
	lines = append(lines, k("s", "Cycle status"))
	lines = append(lines, "")

	lines = append(lines, h("Text Input"))
	lines = append(lines, k("Enter", "Submit"))
	lines = append(lines, k("Esc", "Cancel"))
	lines = append(lines, k("Ctrl+A", "Jump to start"))
	lines = append(lines, k("Ctrl+E", "Jump to end"))
	lines = append(lines, k("Ctrl+W", "Delete word"))
	lines = append(lines, "")

	lines = append(lines, h("Plan Editor"))
	lines = append(lines, k("i/a/o", "Enter insert mode"))
	lines = append(lines, k("Ctrl+S", "Save"))
	lines = append(lines, k("q / Esc", "Cancel"))
	lines = append(lines, "")

	lines = append(lines, styleGray.Render("Press ? or Esc to close"))

	return strings.Join(lines, "\n")
}
