package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

// listLineForIndex returns the rendered line number for a given list item index.
// This is used to auto-scroll the list panel to keep the selection visible.
func listLineForIndex(s *model.TaskStore, items []model.ListItem, targetIndex int) int {
	hasProjects := len(s.Groups) > 0
	lineNum := 2 // header + separator

	for i, item := range items {
		if i == targetIndex {
			return lineNum
		}

		if item.Kind == model.ListItemAllTasks {
			lineNum++
			continue
		}

		if item.Kind == model.ListItemProject {
			if i > 0 {
				lineNum++ // blank line before project
			}
			lineNum++
			continue
		}

		// Task item
		task := item.Task
		if hasProjects && task.Group == "" {
			prevIsGrouped := i > 0 && (items[i-1].Kind == model.ListItemProject ||
				(items[i-1].Kind == model.ListItemTask && items[i-1].Task.Group != ""))
			if i == 0 || prevIsGrouped {
				lineNum += 2 // blank line + "Unassigned"
			}
		}
		lineNum++
	}
	return lineNum
}

func renderListPanel(s *model.TaskStore, projectRoot string, items []model.ListItem, selectedIndex int, isFocused bool, height int, collapsed map[string]bool, scrollOffset int, spinnerFrame string) string {
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

			if item.Kind == model.ListItemAllTasks {
				nameStyle := lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
				if isSelected {
					nameStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
				}
				line := nameStyle.Render("● All Tasks") +
					styleGray.Render(fmt.Sprintf("  (%d)", len(s.Tasks)))
				lines = append(lines, line)
				continue
			}

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
				hasPlan := item.Project.PlanFile != "" && store.PlanExists(projectRoot, item.Project.PlanFile)
				planMark := ""
				if hasPlan {
					planMark = styleCyan.Render(" ✓")
				}
				line := depthIndent + chevronStyle.Render(chevron+" ") +
					nameStyle.Render(truncate(item.Project.Name, maxName)) +
					styleGray.Render(countStr) +
					planMark
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

			isMerged := task.Status == model.StatusMerged
			nameColor := colorWhite
			if isSelected {
				nameColor = colorPrimary
			}
			if isMerged && !isSelected {
				nameColor = colorDim
			}

			hasPlan := task.PlanFile != "" && store.PlanExists(projectRoot, task.PlanFile)

			idStyle := styleGray
			if isMerged && !isSelected {
				idStyle = styleDim
			}

			// Single icon: plan ✓ (indigo) replaces status circle when plan exists,
			// done ✓ (green) takes priority when task is done
			icon := statusIcon(string(task.Status))
			if task.Status == model.StatusPlanning {
				icon = styleMagenta.Render(spinnerFrame)
			} else if hasPlan && task.Status != model.StatusDone && task.Status != model.StatusMerged {
				icon = styleCyan.Render("✓")
			}

			line := lipgloss.NewStyle().Foreground(nameColor).Render(indent) +
				idStyle.Render(padRight(task.ID, 5)) +
				lipgloss.NewStyle().Foreground(nameColor).Render(truncate(task.Title, nameWidth)) +
				"  " + icon
			lines = append(lines, line)
		}
	}

	// Apply vertical scrolling to fit within height
	if height > 0 && len(lines) > height {
		maxOffset := len(lines) - height
		if scrollOffset > maxOffset {
			scrollOffset = maxOffset
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		end := scrollOffset + height
		if end > len(lines) {
			end = len(lines)
		}
		lines = lines[scrollOffset:end]
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().Width(listPanelWidth).Render(content)
}
