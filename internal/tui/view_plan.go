package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderMarkdown(content string, width int) string {
	if width < 40 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(out)
}

func renderPlanView(projectRoot string, task *model.Task, group *model.Group, width int) string {
	if width > maxDetailWidth {
		width = maxDetailWidth
	}
	var planFile, title string
	if task != nil {
		planFile = task.PlanFile
		title = task.ID + ": " + task.Title
	} else if group != nil {
		planFile = group.PlanFile
		title = group.Name
	}

	sepWidth := min(width-2, 50)
	var lines []string
	lines = append(lines, styleCyanBold.Render("Plan — "+title))
	lines = append(lines, "")
	lines = append(lines, horizontalLine(sepWidth))
	lines = append(lines, "")

	if planFile != "" {
		plan, err := store.LoadPlan(projectRoot, planFile)
		if err == nil && plan != "" {
			lines = append(lines, renderMarkdown(plan, width-2))
		} else {
			lines = append(lines, styleGray.Render("No plan generated yet. Press 'p' to generate with Claude, or 'e' to write manually."))
		}
	} else {
		lines = append(lines, styleGray.Render("No plan generated yet. Press 'p' to generate with Claude, or 'e' to write manually."))
	}

	return strings.Join(lines, "\n")
}
