package tui

import (
	"fmt"
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

	// Slash-command autocomplete
	acActive bool     // autocomplete dropdown is showing
	acQuery  string   // text typed after /
	acIndex  int      // selected index in filtered list
	acItems  []string // filtered skill names matching query
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
	// Dismiss autocomplete on Escape/Tab before global key handlers
	if m.acActive {
		if msg.Type == tea.KeyEscape {
			m.acActive = false
			return m, nil
		}
		if msg.Type == tea.KeyTab {
			if len(m.acItems) > 0 && m.acIndex < len(m.acItems) {
				m.insertAutocomplete(m.acItems[m.acIndex])
			}
			m.acActive = false
			return m, nil
		}
	}

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
		m.acActive = false
		m.Active = formField((int(m.Active) + 1) % int(fieldCount))
		m.focusActive()
		return m, nil
	case msg.Type == tea.KeyShiftTab:
		m.acActive = false
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
		if m.acActive {
			// Handle autocomplete keys (Escape and Tab handled in guard above)
			switch {
			case msg.Type == tea.KeyEnter:
				if len(m.acItems) > 0 && m.acIndex < len(m.acItems) {
					m.insertAutocomplete(m.acItems[m.acIndex])
				}
				m.acActive = false
				return m, nil
			case msg.Type == tea.KeyUp:
				if m.acIndex > 0 {
					m.acIndex--
				}
				return m, nil
			case msg.Type == tea.KeyDown:
				if m.acIndex < len(m.acItems)-1 {
					m.acIndex++
				}
				return m, nil
			default:
				// Pass through to textarea, then update filter
				m.desc, cmd = m.desc.Update(msg)
				m.updateAutocompleteFilter()
				if len(m.acItems) == 0 {
					m.acActive = false
				}
				return m, cmd
			}
		}
		// Normal description handling
		m.desc, cmd = m.desc.Update(msg)
		// Check if / was just typed
		if msg.Type == tea.KeyRunes && string(msg.Runes) == "/" {
			m.tryActivateAutocomplete()
		}
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

func (m *FormModel) tryActivateAutocomplete() {
	if len(m.availableSkills) == 0 {
		return
	}
	val := m.desc.Value()
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		return
	}
	line := lines[row]
	col := m.desc.LineInfo().CharOffset
	if col <= 0 || col > len(line) {
		return
	}
	if line[col-1] != '/' {
		return
	}
	// Only activate if / is at start of line or after whitespace
	if col == 1 || line[col-2] == ' ' || line[col-2] == '\t' {
		m.acActive = true
		m.acQuery = ""
		m.acIndex = 0
		m.acItems = make([]string, len(m.availableSkills))
		copy(m.acItems, m.availableSkills)
	}
}

func (m *FormModel) updateAutocompleteFilter() {
	val := m.desc.Value()
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		m.acActive = false
		return
	}
	line := lines[row]
	col := m.desc.LineInfo().CharOffset
	if col > len(line) {
		col = len(line)
	}

	// Find the / that started this autocomplete
	slashPos := -1
	for i := col - 1; i >= 0; i-- {
		if line[i] == '/' {
			slashPos = i
			break
		}
		if line[i] == ' ' || line[i] == '\t' {
			break
		}
	}
	if slashPos == -1 {
		m.acActive = false
		return
	}

	m.acQuery = strings.ToLower(line[slashPos+1 : col])
	m.acItems = nil
	for _, name := range m.availableSkills {
		if strings.Contains(strings.ToLower(name), m.acQuery) {
			m.acItems = append(m.acItems, name)
		}
	}
	if m.acIndex >= len(m.acItems) {
		m.acIndex = 0
	}
}

func (m *FormModel) insertAutocomplete(skillName string) {
	val := m.desc.Value()
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		return
	}
	line := lines[row]
	col := m.desc.LineInfo().CharOffset
	if col > len(line) {
		col = len(line)
	}

	// Find the / position
	slashPos := -1
	for i := col - 1; i >= 0; i-- {
		if line[i] == '/' {
			slashPos = i
			break
		}
	}
	if slashPos == -1 {
		return
	}

	// Replace /query with /skillName followed by a space
	newLine := line[:slashPos] + "/" + skillName + " "
	if col < len(line) {
		newLine += line[col:]
	}
	lines[row] = newLine
	m.desc.SetValue(strings.Join(lines, "\n"))
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
			// Autocomplete dropdown
			if m.acActive && m.Active == fieldDescription && len(m.acItems) > 0 {
				indent := strings.Repeat(" ", 16)
				lines = append(lines, indent+styleGray.Render("Skills:"))
				maxShow := 8
				// Compute scroll window that keeps acIndex visible
				start := 0
				if len(m.acItems) > maxShow {
					if m.acIndex >= maxShow {
						start = m.acIndex - maxShow + 1
					}
					if start+maxShow > len(m.acItems) {
						start = len(m.acItems) - maxShow
					}
				}
				end := start + maxShow
				if end > len(m.acItems) {
					end = len(m.acItems)
				}
				for idx := start; idx < end; idx++ {
					prefix := "  "
					style := lipgloss.NewStyle().Foreground(colorWhite)
					if idx == m.acIndex {
						prefix = "▸ "
						style = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
					}
					lines = append(lines, indent+style.Render(prefix+"/"+m.acItems[idx]))
				}
				if end < len(m.acItems) {
					lines = append(lines, indent+styleGray.Render(fmt.Sprintf("  ... %d more", len(m.acItems)-end)))
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
