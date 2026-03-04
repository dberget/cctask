package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TaskFormData struct {
	Title       string
	Description string
	Tags        string
	WorkDir     string
	Skills      []string
}

type formField int

const (
	fieldTitle formField = iota
	fieldDescription
	fieldTags
	fieldWorkDir
	fieldSkills
	fieldCount
)

type FormModel struct {
	Heading string
	Active  formField
	Width   int

	title   textinput.Model
	desc    textarea.Model
	tags    textinput.Model
	workDir textinput.Model

	// Skills picker
	skills          []string // selected skill names
	availableSkills []string // all available skill names for display
}

func NewForm(heading string, initial *TaskFormData, width int, availableSkills []string) FormModel {
	// Title field
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 0
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSecondary)
	ti.CursorStyle = styleCursor
	ti.Placeholder = "task title (required)"
	ti.Focus()

	// Description field
	ta := textarea.New()
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.SetHeight(6)
	ta.Placeholder = "task details, context, acceptance criteria"
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorSecondary)
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(colorDim)
	ta.Blur()

	// Tags field
	tg := textinput.New()
	tg.Prompt = ""
	tg.CharLimit = 0
	tg.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	tg.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSecondary)
	tg.CursorStyle = styleCursor
	tg.Placeholder = "comma-separated"
	tg.Blur()

	// WorkDir field
	wd := textinput.New()
	wd.Prompt = ""
	wd.CharLimit = 0
	wd.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	wd.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSecondary)
	wd.CursorStyle = styleCursor
	wd.Placeholder = "relative or absolute path"
	wd.Blur()

	m := FormModel{
		Heading:         heading,
		Width:           width,
		title:           ti,
		desc:            ta,
		tags:            tg,
		workDir:         wd,
		availableSkills: availableSkills,
	}

	if initial != nil {
		m.title.SetValue(initial.Title)
		m.desc.SetValue(initial.Description)
		m.tags.SetValue(initial.Tags)
		m.workDir.SetValue(initial.WorkDir)
		if initial.Skills != nil {
			m.skills = initial.Skills
		}
	}

	// Set widths
	fieldWidth := width - 24
	if fieldWidth < 30 {
		fieldWidth = 60
	}
	m.title.Width = fieldWidth
	m.desc.SetWidth(fieldWidth)
	m.tags.Width = fieldWidth
	m.workDir.Width = fieldWidth

	return m
}

func (m FormModel) Data() TaskFormData {
	return TaskFormData{
		Title:       m.title.Value(),
		Description: m.desc.Value(),
		Tags:        m.tags.Value(),
		WorkDir:     m.workDir.Value(),
		Skills:      m.skills,
	}
}

// FormSkillPickerMsg is sent when the user presses Enter on the Skills field,
// signaling app.go to open the MultiCheck skill picker.
type FormSkillPickerMsg struct{}

func (m FormModel) Update(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	// Global keys
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return FormCancelMsg{} }
	case msg.Type == tea.KeyCtrlS || msg.Type == tea.KeyCtrlD:
		if strings.TrimSpace(m.title.Value()) != "" {
			data := m.Data()
			return m, func() tea.Msg { return FormSubmitMsg{Data: data} }
		}
		return m, nil
	case msg.Type == tea.KeyTab:
		m.Active = formField((int(m.Active) + 1) % int(fieldCount))
		m.focusActive()
		return m, nil
	case msg.Type == tea.KeyShiftTab:
		m.Active = formField((int(m.Active) - 1 + int(fieldCount)) % int(fieldCount))
		m.focusActive()
		return m, nil
	}

	// Enter on single-line fields advances to next field / opens skill picker
	if msg.Type == tea.KeyEnter && m.Active != fieldDescription {
		if m.Active == fieldSkills {
			// Open the skill picker overlay
			return m, func() tea.Msg { return FormSkillPickerMsg{} }
		}
		// All other single-line fields advance to next
		m.Active = formField(int(m.Active) + 1)
		m.focusActive()
		return m, nil
	}

	// Delegate to active field
	var cmd tea.Cmd
	switch m.Active {
	case fieldTitle:
		m.title, cmd = m.title.Update(msg)
	case fieldDescription:
		m.desc, cmd = m.desc.Update(msg)
	case fieldTags:
		m.tags, cmd = m.tags.Update(msg)
	case fieldWorkDir:
		m.workDir, cmd = m.workDir.Update(msg)
	case fieldSkills:
		// Skills field is not editable; input handled via Enter -> picker.
	}
	return m, cmd
}

func (m *FormModel) focusActive() {
	m.title.Blur()
	m.desc.Blur()
	m.tags.Blur()
	m.workDir.Blur()
	switch m.Active {
	case fieldTitle:
		m.title.Focus()
	case fieldDescription:
		m.desc.Focus()
	case fieldTags:
		m.tags.Focus()
	case fieldWorkDir:
		m.workDir.Focus()
	case fieldSkills:
		// Skills field is display-only; no input widget to focus.
	}
}

func (m FormModel) View() string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(m.Heading))
	lines = append(lines, styleGray.Render("Tab: next field  Enter: next/newline  Ctrl+S: save  Ctrl+B: browse dir  Esc: cancel"))
	lines = append(lines, "")

	labels := [fieldCount]string{"Title", "Description", "Tags", "WorkDir", "Skills"}

	for i := formField(0); i < fieldCount; i++ {
		isActive := i == m.Active
		labelColor := colorSecondary
		if isActive {
			labelColor = colorPrimary
		}
		indicator := "  "
		if isActive {
			indicator = "▸ "
		}
		labelStr := lipgloss.NewStyle().Foreground(labelColor).Bold(isActive).Render(indicator + labels[i] + ":")
		labelPadded := lipgloss.NewStyle().Width(16).Render(labelStr)

		switch i {
		case fieldTitle:
			lines = append(lines, labelPadded+m.title.View())
		case fieldDescription:
			// Render description with indented multiline
			descView := m.desc.View()
			descLines := strings.Split(descView, "\n")
			for j, dl := range descLines {
				if j == 0 {
					lines = append(lines, labelPadded+dl)
				} else {
					lines = append(lines, strings.Repeat(" ", 16)+dl)
				}
			}
		case fieldTags:
			lines = append(lines, labelPadded+m.tags.View())
		case fieldWorkDir:
			lines = append(lines, labelPadded+m.workDir.View())
		case fieldSkills:
			skillsDisplay := "(none - press Enter to select)"
			if len(m.skills) > 0 {
				skillsDisplay = strings.Join(m.skills, ", ")
			}
			lines = append(lines, labelPadded+lipgloss.NewStyle().Foreground(colorWhite).Render(skillsDisplay))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
