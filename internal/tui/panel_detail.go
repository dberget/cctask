package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderDetailPanel(s *model.TaskStore, projectRoot string, selectedItem *model.ListItem, width int) string {
	if selectedItem == nil {
		return styleGray.Render("Select a task or project to see details")
	}
	if selectedItem.Kind == model.ListItemTask {
		return renderTaskDetail(selectedItem.Task, projectRoot, width)
	}
	if selectedItem.Kind == model.ListItemProject {
		return renderGroupDetail(selectedItem.Project, s, projectRoot, width)
	}
	return ""
}

func renderTaskDetail(task *model.Task, projectRoot string, width int) string {
	if width > maxDetailWidth {
		width = maxDetailWidth
	}
	hasPlan := task.PlanFile != "" && store.PlanExists(projectRoot, task.PlanFile)

	var lines []string
	lines = append(lines, styleCyanBold.Render(fmt.Sprintf("%s · %s", task.ID, truncate(task.Title, width-6))))
	lines = append(lines, "")

	lines = append(lines, styleGray.Render(padRight("Status:", 10))+statusLabel(string(task.Status)))
	if len(task.Tags) > 0 {
		lines = append(lines, styleGray.Render(padRight("Tags:", 10))+styleMagenta.Render(strings.Join(task.Tags, ", ")))
	}
	if task.Group != "" {
		lines = append(lines, styleGray.Render(padRight("Project:", 10))+task.Group)
	}
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))

	if task.Description != "" {
		lines = append(lines, "")
		sepWidth := min(width-2, 50)
		lines = append(lines, styleGray.Render(padRight("── Description ", sepWidth)+"─"))
		lines = append(lines, "")
		lines = append(lines, wrapText(task.Description, width-2))
	}

	if hasPlan {
		plan, _ := store.LoadPlan(projectRoot, task.PlanFile)
		if plan != "" {
			lines = append(lines, "")
			sepWidth := min(width-2, 50)
			lines = append(lines, styleGray.Render(padRight("── Plan (preview) ", sepWidth)+"─"))
			lines = append(lines, "")
			lines = append(lines, truncateLines(wrapText(plan, width-2), 15))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().PaddingLeft(1).Width(width).Render(content)
}

func renderGroupDetail(group *model.Group, s *model.TaskStore, projectRoot string, width int) string {
	if width > maxDetailWidth {
		width = maxDetailWidth
	}
	tasks := store.GetTasksForGroup(s, group.ID)
	hasPlan := group.PlanFile != "" && store.PlanExists(projectRoot, group.PlanFile)

	var lines []string
	lines = append(lines, styleCyanBold.Render(truncate(group.Name, width-2)))

	if group.Description != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(group.Description, width-2))
	}

	lines = append(lines, "")
	lines = append(lines, styleGray.Render(padRight("Tasks:", 10))+fmt.Sprintf("%d", len(tasks)))
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))

	lines = append(lines, "")
	sepWidth := min(width-2, 50)
	lines = append(lines, styleGray.Render(padRight("── Tasks ", sepWidth)+"─"))
	lines = append(lines, "")

	if len(tasks) == 0 {
		lines = append(lines, styleGray.Render("No tasks in this project"))
	} else {
		for _, t := range tasks {
			lines = append(lines, styleGray.Render(padRight(t.ID, 5))+truncate(t.Title, width-10))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().PaddingLeft(1).Width(width).Render(content)
}
