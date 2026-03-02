package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func renderListPanel(s *model.TaskStore, projectRoot string, items []model.ListItem, selectedIndex int, isFocused bool, height int, collapsed map[string]bool) string {
	hasProjects := len(s.Groups) > 0

	titleColor := colorWhite
	if isFocused {
		titleColor = colorPrimary
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(titleColor).Render("Projects") +
		styleGray.Render(fmt.Sprintf(" (%d)", len(s.Groups)))
	sep := horizontalLine(36)

	var lines []string
	lines = append(lines, header)
	lines = append(lines, sep)

	if len(items) == 0 {
		lines = append(lines, styleGray.Render("No tasks yet. Press 'a' to add."))
	} else {
		for i, item := range items {
			isSelected := isFocused && i == selectedIndex
			depthIndent := strings.Repeat("  ", item.Depth)

			if item.Kind == model.ListItemProject {
				if i > 0 {
					lines = append(lines, "")
				}
				taskCount := len(store.GetTasksForGroup(s, item.Project.ID))
				childCount := len(store.GetChildGroups(s, item.Project.ID))
				isCollapsed := collapsed[item.Project.ID]
				chevron := "▾"
				if isCollapsed {
					chevron = "▸"
				}
				nameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
				chevronStyle := styleGray
				if isSelected {
					nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
					chevronStyle = lipgloss.NewStyle().Foreground(colorPrimary)
				}
				maxName := 23 - item.Depth*2
				if maxName < 8 {
					maxName = 8
				}
				countStr := fmt.Sprintf("  (%d)", taskCount)
				if childCount > 0 {
					countStr = fmt.Sprintf("  (%d+%dsub)", taskCount, childCount)
				}
				line := depthIndent + chevronStyle.Render(chevron+" ") +
					nameStyle.Render(truncate(item.Project.Name, maxName)) +
					styleGray.Render(countStr)
				lines = append(lines, line)
				continue
			}

			// Task item
			task := item.Task

			// Show "Unassigned" separator before first unassigned task
			if hasProjects && task.Group == "" {
				prevIsGrouped := i > 0 && (items[i-1].Kind == model.ListItemProject ||
					(items[i-1].Kind == model.ListItemTask && items[i-1].Task.Group != ""))
				if i == 0 || prevIsGrouped {
					lines = append(lines, "")
					lines = append(lines, styleGray.Render("  Unassigned"))
				}
			}

			var indent string
			if hasProjects {
				if isSelected {
					indent = depthIndent + "  ▸ "
				} else {
					indent = depthIndent + "    "
				}
			} else {
				if isSelected {
					indent = "▸ "
				} else {
					indent = "  "
				}
			}

			nameWidth := 20 - item.Depth*2
			if nameWidth < 8 {
				nameWidth = 8
			}

			nameColor := colorWhite
			if isSelected {
				nameColor = colorPrimary
			}

			hasPlan := task.PlanFile != "" && store.PlanExists(projectRoot, task.PlanFile)
			planMark := ""
			if hasPlan {
				planMark = styleGreen.Render(" ✓")
			}

			line := lipgloss.NewStyle().Foreground(nameColor).Render(indent) +
				styleGray.Render(padRight(task.ID, 5)) +
				lipgloss.NewStyle().Foreground(nameColor).Render(truncate(task.Title, nameWidth)) +
				"  " + statusIcon(string(task.Status)) +
				planMark
			lines = append(lines, line)
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(listPanelWidth).Render(content)
}
