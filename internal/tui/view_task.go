package tui

import (
	"fmt"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderTaskView(task *model.Task, projectRoot string, width int) string {
	if width > maxDetailWidth {
		width = maxDetailWidth
	}
	hasPlan := task.PlanFile != "" && store.PlanExists(projectRoot, task.PlanFile)

	var lines []string
	lines = append(lines, styleCyanBold.Render(fmt.Sprintf("%s · %s", task.ID, task.Title)))
	lines = append(lines, "")

	lines = append(lines, styleGray.Render(padRight("Status:", 10))+statusLabel(string(task.Status)))
	if len(task.Tags) > 0 {
		lines = append(lines, styleGray.Render(padRight("Tags:", 10))+styleMagenta.Render(strings.Join(task.Tags, ", ")))
	}
	if task.Group != "" {
		lines = append(lines, styleGray.Render(padRight("Project:", 10))+task.Group)
	}
	if task.WorkDir != "" {
		lines = append(lines, styleGray.Render(padRight("WorkDir:", 10))+task.WorkDir)
	}
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))

	if task.Description != "" {
		lines = append(lines, "")
		sepWidth := min(width-2, 50)
		lines = append(lines, sectionHeader("Description", sepWidth))
		lines = append(lines, "")
		lines = append(lines, renderMarkdown(task.Description, width-2))
	}

	lines = append(lines, "")
	sepWidth := min(width-2, 50)
	lines = append(lines, sectionHeader("Plan", sepWidth))
	lines = append(lines, "")

	if hasPlan {
		plan, _ := store.LoadPlan(projectRoot, task.PlanFile)
		if plan != "" {
			lines = append(lines, renderMarkdown(plan, width-2))
		}
	} else {
		lines = append(lines, styleGray.Render("No plan yet. Press 'p' to generate with Claude."))
	}

	return strings.Join(lines, "\n")
}
