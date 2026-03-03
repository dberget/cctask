package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
)

// TaskListItem adapts a model.Task to implement list.Item.
type TaskListItem struct {
	task model.Task
}

func (i TaskListItem) Title() string       { return i.task.Title }
func (i TaskListItem) Description() string { return string(i.task.Status) }
func (i TaskListItem) FilterValue() string { return i.task.Title + " " + strings.Join(i.task.Tags, " ") }

// taskListDelegate renders each item in the list.
type taskListDelegate struct{}

func (d taskListDelegate) Height() int                             { return 2 }
func (d taskListDelegate) Spacing() int                            { return 0 }
func (d taskListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d taskListDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(TaskListItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	icon := statusIcon(string(item.task.Status))
	title := item.task.Title
	if len(title) > 60 {
		title = title[:57] + "..."
	}

	var tags string
	if len(item.task.Tags) > 0 {
		tags = " " + styleMagenta.Render("["+strings.Join(item.task.Tags, ", ")+"]")
	}

	indicator := "  "
	if isSelected {
		indicator = "▸ "
	}

	titleStyle := lipgloss.NewStyle().Foreground(colorWhite)
	if isSelected {
		titleStyle = titleStyle.Foreground(colorPrimary).Bold(true)
	}

	line1 := indicator + icon + " " + titleStyle.Render(title) + tags
	line2 := "    " + styleGray.Render(fmt.Sprintf("%s · %s", item.task.ID, item.task.Status))
	if item.task.Group != "" {
		line2 += styleGray.Render(" · " + item.task.Group)
	}

	fmt.Fprint(w, line1+"\n"+line2)
}

func newTaskList(tasks []model.Task, width, height int) list.Model {
	var items []list.Item
	for _, t := range tasks {
		items = append(items, TaskListItem{task: t})
	}

	delegate := taskListDelegate{}
	l := list.New(items, delegate, width, height)
	l.Title = "All Tasks"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styleCyanBold
	l.Styles.FilterPrompt = styleCyanBold
	l.Styles.FilterCursor = styleCursor

	return l
}
