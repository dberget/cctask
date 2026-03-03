package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderGroupView(group *model.Group, s *model.TaskStore, projectRoot string, prog progress.Model) string {
	width := maxDetailWidth
	tasks := store.GetTasksForGroup(s, group.ID)
	children := store.GetChildGroups(s, group.ID)
	hasPlan := group.PlanFile != "" && store.PlanExists(projectRoot, group.PlanFile)

	sepWidth := min(width-2, 50)
	var lines []string

	// Show breadcrumb path if subgroup
	if group.ParentGroup != "" {
		path := store.GetGroupPath(s, group.ID)
		var names []string
		for _, g := range path {
			names = append(names, g.Name)
		}
		lines = append(lines, styleCyanBold.Render("Project: "+strings.Join(names, " > ")))
	} else {
		lines = append(lines, styleCyanBold.Render("Project: "+group.Name))
	}
	lines = append(lines, "")
	lines = append(lines, horizontalLine(sepWidth))

	if group.Description != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(group.Description, width-2))
	}

	lines = append(lines, "")
	if group.WorkDir != "" {
		lines = append(lines, styleGray.Render(padRight("WorkDir:", 10))+group.WorkDir)
	}
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))

	// Progress bar
	if len(tasks) > 0 {
		done := 0
		for _, t := range tasks {
			if t.Status == model.StatusDone || t.Status == model.StatusMerged {
				done++
			}
		}
		pct := float64(done) / float64(len(tasks))
		lines = append(lines, "")
		lines = append(lines, styleGray.Render(padRight("Progress:", 10))+prog.ViewAs(pct)+
			styleGray.Render(fmt.Sprintf("  %d/%d", done, len(tasks))))
	}

	// Subgroups section
	if len(children) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionHeader(fmt.Sprintf("Subgroups (%d)", len(children)), 50))
		lines = append(lines, "")
		for _, child := range children {
			childTasks := store.GetTasksForGroup(s, child.ID)
			lines = append(lines, styleBold.Render(child.Name)+styleGray.Render(fmt.Sprintf("  (%d tasks)", len(childTasks))))
		}
	}

	lines = append(lines, "")
	lines = append(lines, sectionHeader(fmt.Sprintf("Tasks (%d)", len(tasks)), 50))
	lines = append(lines, "")

	if len(tasks) == 0 {
		lines = append(lines, styleGray.Render("No tasks in this project"))
	} else {
		for _, task := range tasks {
			lines = append(lines, styleGray.Render(padRight(task.ID, 6))+truncate(task.Title, width-12)+"  "+statusIcon(string(task.Status)))
		}
	}

	return strings.Join(lines, "\n")
}
