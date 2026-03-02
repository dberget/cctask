package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TaskFormData struct {
	Title       string
	Description string
	Tags        string
}

type formField int

const (
	fieldTitle formField = iota
	fieldDescription
	fieldTags
	fieldCount
)

var fieldLabels = [fieldCount]string{"Title", "Description", "Tags"}
var fieldHints = [fieldCount]string{"", "task details, context, acceptance criteria", "comma-separated"}

type FormModel struct {
	Heading string
	Active  formField
	Values  [fieldCount]string
	Cursors [fieldCount]int
}

func NewForm(heading string, initial *TaskFormData) FormModel {
	m := FormModel{Heading: heading}
	if initial != nil {
		m.Values[fieldTitle] = initial.Title
		m.Values[fieldDescription] = initial.Description
		m.Values[fieldTags] = initial.Tags
		m.Cursors[fieldTitle] = len(initial.Title)
		m.Cursors[fieldDescription] = len(initial.Description)
		m.Cursors[fieldTags] = len(initial.Tags)
	}
	return m
}

func (m FormModel) Data() TaskFormData {
	return TaskFormData{
		Title:       m.Values[fieldTitle],
		Description: m.Values[fieldDescription],
		Tags:        m.Values[fieldTags],
	}
}

func (m FormModel) Update(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	f := m.Active
	val := m.Values[f]
	cursor := m.Cursors[f]

	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return FormCancelMsg{} }
	case msg.Type == tea.KeyCtrlS || msg.Type == tea.KeyCtrlD:
		if strings.TrimSpace(m.Values[fieldTitle]) != "" {
			data := m.Data()
			return m, func() tea.Msg { return FormSubmitMsg{Data: data} }
		}
	case msg.Type == tea.KeyTab:
		m.Active = formField((int(m.Active) + 1) % int(fieldCount))
	case msg.Type == tea.KeyShiftTab:
		m.Active = formField((int(m.Active) - 1 + int(fieldCount)) % int(fieldCount))
	case msg.Type == tea.KeyEnter:
		if f == fieldTags {
			if strings.TrimSpace(m.Values[fieldTitle]) != "" {
				data := m.Data()
				return m, func() tea.Msg { return FormSubmitMsg{Data: data} }
			}
		} else {
			m.Active = formField(int(m.Active) + 1)
		}
	case msg.Type == tea.KeyBackspace:
		if cursor > 0 {
			m.Values[f] = val[:cursor-1] + val[cursor:]
			m.Cursors[f] = cursor - 1
		}
	case msg.Type == tea.KeyLeft:
		if cursor > 0 {
			m.Cursors[f] = cursor - 1
		}
	case msg.Type == tea.KeyRight:
		if cursor < len(val) {
			m.Cursors[f] = cursor + 1
		}
	case msg.Type == tea.KeyCtrlA:
		m.Cursors[f] = 0
	case msg.Type == tea.KeyCtrlE:
		m.Cursors[f] = len(val)
	case msg.Type == tea.KeySpace:
		m.Values[f] = val[:cursor] + " " + val[cursor:]
		m.Cursors[f] = cursor + 1
	case msg.Type == tea.KeyRunes:
		ch := string(msg.Runes)
		m.Values[f] = val[:cursor] + ch + val[cursor:]
		m.Cursors[f] = cursor + len(ch)
	}
	return m, nil
}

func (m FormModel) View() string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(m.Heading))
	lines = append(lines, styleGray.Render("Tab: next field  Enter: next/submit  Ctrl+S: save  Esc: cancel"))
	lines = append(lines, "")

	for i := formField(0); i < fieldCount; i++ {
		isActive := i == m.Active
		val := m.Values[i]
		cursor := m.Cursors[i]

		labelColor := colorSecondary
		if isActive {
			labelColor = colorPrimary
		}
		indicator := "  "
		if isActive {
			indicator = "▸ "
		}
		labelStr := lipgloss.NewStyle().Foreground(labelColor).Bold(isActive).Render(indicator + fieldLabels[i] + ":")
		labelPadded := lipgloss.NewStyle().Width(16).Render(labelStr)

		var valueStr string
		if isActive {
			valueStr = renderEditableLine(val, cursor)
		} else if val != "" {
			valueStr = val
		} else {
			hint := fieldHints[i]
			if hint == "" {
				hint = "empty"
			}
			valueStr = styleGray.Render("(" + hint + ")")
		}

		lines = append(lines, labelPadded+valueStr)
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func renderEditableLine(value string, cursor int) string {
	if value == "" && cursor == 0 {
		return styleCursor.Render(" ")
	}
	before := safeSlice(value, 0, cursor)
	cursorChar := " "
	if cursor < len(value) {
		cursorChar = string(value[cursor])
	}
	after := safeSlice(value, cursor+1, len(value))
	return before + styleCursor.Render(cursorChar) + after
}
