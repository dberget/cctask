package tui

import (
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
)

func keyHint(key, label string) string {
	return styleCyanBold.Render(key) + styleGray.Render(":"+label)
}

func renderStatusBar(mode model.ViewMode, message string, cols int) string {
	lineWidth := cols - 4
	if lineWidth < 40 {
		lineWidth = 40
	}

	var hints []string
	switch mode {
	case model.ModeList:
		hints = []string{
			keyHint("a", "add"), keyHint("e", "edit"), keyHint("d", "delete"),
			keyHint("g", "project"), keyHint("r", "run"), keyHint("p", "plan"),
			keyHint("s", "status"), keyHint("/", "filter"), keyHint("v", "view"),
			keyHint("m", "merge"), keyHint("Space", "collapse"), keyHint("Tab", "switch"),
		}
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
	case model.ModeTaskViewAsk:
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
