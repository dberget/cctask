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
	if selectedItem.Kind == model.ListItemAllTasks {
		return renderAllTasksDetail(s, width)
	}
	if selectedItem.Kind == model.ListItemTask {
		return renderTaskDetail(selectedItem.Task, projectRoot, width)
	}
	if selectedItem.Kind == model.ListItemProject {
		return renderGroupDetail(selectedItem.Project, s, projectRoot, width)
	}
	return ""
}

func renderAllTasksDetail(s *model.TaskStore, width int) string {
	if width > maxDetailWidth {
		width = maxDetailWidth
	}

	var lines []string
	lines = append(lines, styleCyanBold.Render("All Tasks"))
	lines = append(lines, "")

	pending, inProgress, done, merged := 0, 0, 0, 0
	for _, t := range s.Tasks {
		switch t.Status {
		case model.StatusPending:
			pending++
		case model.StatusInProgress:
			inProgress++
		case model.StatusDone:
			done++
		case model.StatusMerged:
			merged++
		}
	}

	lines = append(lines, styleGray.Render(padRight("Total:", 14))+fmt.Sprintf("%d", len(s.Tasks)))
	lines = append(lines, styleGray.Render(padRight("Pending:", 14))+fmt.Sprintf("%d", pending))
	lines = append(lines, styleGray.Render(padRight("In Progress:", 14))+fmt.Sprintf("%d", inProgress))
	lines = append(lines, styleGray.Render(padRight("Done:", 14))+fmt.Sprintf("%d", done))
	if merged > 0 {
		lines = append(lines, styleGray.Render(padRight("Merged:", 14))+fmt.Sprintf("%d", merged))
	}
	lines = append(lines, styleGray.Render(padRight("Projects:", 14))+fmt.Sprintf("%d", len(s.Groups)))

	lines = append(lines, "")
	sepWidth := min(width-2, 50)
	lines = append(lines, sectionHeader("Tasks", sepWidth))
	lines = append(lines, "")

	if len(s.Tasks) == 0 {
		lines = append(lines, styleGray.Render("No tasks yet"))
	} else {
		for _, t := range s.Tasks {
			lines = append(lines, styleGray.Render(padRight(t.ID, 5))+
				statusIcon(string(t.Status))+" "+
				truncate(t.Title, width-12))
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().PaddingLeft(1).Width(width).Render(content)
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
	if task.WorkDir != "" {
		lines = append(lines, styleGray.Render(padRight("WorkDir:", 10))+task.WorkDir)
	}
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))
	if task.MergedInto != "" {
		lines = append(lines, styleGray.Render(padRight("Merged:", 10))+styleDim.Render("into "+task.MergedInto))
	}

	if task.Description != "" {
		lines = append(lines, "")
		sepWidth := min(width-2, 50)
		lines = append(lines, sectionHeader("Description", sepWidth))
		lines = append(lines, "")
		lines = append(lines, wrapText(task.Description, width-2))
	}

	if hasPlan {
		plan, _ := store.LoadPlan(projectRoot, task.PlanFile)
		if plan != "" {
			lines = append(lines, "")
			sepWidth := min(width-2, 50)
			lines = append(lines, sectionHeader("Plan (preview)", sepWidth))
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
	children := store.GetChildGroups(s, group.ID)
	hasPlan := group.PlanFile != "" && store.PlanExists(projectRoot, group.PlanFile)

	var lines []string

	// Show breadcrumb path if this is a subgroup
	if group.ParentGroup != "" {
		path := store.GetGroupPath(s, group.ID)
		var names []string
		for _, g := range path {
			names = append(names, g.Name)
		}
		lines = append(lines, styleGray.Render(strings.Join(names, " > ")))
	}

	lines = append(lines, styleCyanBold.Render(truncate(group.Name, width-2)))

	if group.Description != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(group.Description, width-2))
	}

	lines = append(lines, "")
	if group.ParentGroup != "" {
		if parent := store.FindGroup(s, group.ParentGroup); parent != nil {
			lines = append(lines, styleGray.Render(padRight("Parent:", 12))+parent.Name)
		}
	}
	if group.WorkDir != "" {
		lines = append(lines, styleGray.Render(padRight("WorkDir:", 12))+group.WorkDir)
	}
	lines = append(lines, styleGray.Render(padRight("Tasks:", 12))+fmt.Sprintf("%d", len(tasks)))
	if len(children) > 0 {
		lines = append(lines, styleGray.Render(padRight("Subgroups:", 12))+fmt.Sprintf("%d", len(children)))
	}
	lines = append(lines, styleGray.Render(padRight("Plan:", 12))+planStatus(hasPlan))

	// Show subgroups section
	if len(children) > 0 {
		lines = append(lines, "")
		sepWidth := min(width-2, 50)
		lines = append(lines, sectionHeader("Subgroups", sepWidth))
		lines = append(lines, "")
		for _, child := range children {
			childTasks := store.GetTasksForGroup(s, child.ID)
			lines = append(lines, styleBold.Render(child.Name)+styleGray.Render(fmt.Sprintf("  (%d tasks)", len(childTasks))))
		}
	}

	lines = append(lines, "")
	sepWidth := min(width-2, 50)
	lines = append(lines, sectionHeader("Tasks", sepWidth))
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
