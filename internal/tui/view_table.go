package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
)

func newTaskTable(tasks []model.Task, width, height int) table.Model {
	columns := []table.Column{
		{Title: "Status", Width: 10},
		{Title: "ID", Width: 6},
		{Title: "Title", Width: max(20, width-50)},
		{Title: "Project", Width: 12},
		{Title: "Tags", Width: 16},
	}

	var rows []table.Row
	for _, t := range tasks {
		status := string(t.Status)
		tags := strings.Join(t.Tags, ", ")
		if len(tags) > 16 {
			tags = tags[:13] + "..."
		}
		project := t.Group
		if len(project) > 12 {
			project = project[:9] + "..."
		}
		title := t.Title
		maxTitle := max(20, width-50)
		if len(title) > maxTitle {
			title = title[:maxTitle-3] + "..."
		}
		rows = append(rows, table.Row{status, t.ID, title, project, tags})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorDim).
		BorderBottom(true).
		Bold(true).
		Foreground(colorPrimary)
	s.Selected = s.Selected.
		Foreground(colorBright).
		Background(lipgloss.Color("#1E293B")).
		Bold(true)
	s.Cell = s.Cell.Foreground(colorWhite)

	t.SetStyles(s)
	return t
}

func renderTableView(t table.Model) string {
	return styleCyanBold.Render("Task Table") + "  " + styleGray.Render("T: back to tree  Enter: detail  Esc: back") + "\n\n" + t.View()
}
