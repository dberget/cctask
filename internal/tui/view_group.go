package tui

import (
	"fmt"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderGroupView(group *model.Group, s *model.TaskStore, projectRoot string) string {
	width := maxDetailWidth
	tasks := store.GetTasksForGroup(s, group.ID)
	hasPlan := group.PlanFile != "" && store.PlanExists(projectRoot, group.PlanFile)

	sepWidth := min(width-2, 50)
	var lines []string
	lines = append(lines, styleCyanBold.Render("Project: "+group.Name))
	lines = append(lines, "")
	lines = append(lines, horizontalLine(sepWidth))

	if group.Description != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(group.Description, width-2))
	}

	lines = append(lines, "")
	lines = append(lines, styleGray.Render(padRight("Plan:", 10))+planStatus(hasPlan))

	lines = append(lines, "")
	lines = append(lines, styleBold.Render(fmt.Sprintf("Tasks (%d)", len(tasks))))
	lines = append(lines, horizontalLine(40))
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
