package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditorModel wraps bubbles textarea, replacing the old vim-style editor.
type EditorModel struct {
	Heading string
	inner   textarea.Model
	VH      int // viewport height (used to size textarea)
	VW      int // viewport width
}

func NewEditor(heading string, initial string, vh, vw int) EditorModel {
	if vh < 10 {
		vh = 10
	}
	if vw < 40 {
		vw = 40
	}
	ta := textarea.New()
	ta.SetValue(initial)
	ta.ShowLineNumbers = true
	ta.Focus()
	ta.SetWidth(vw)
	ta.SetHeight(vh)
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(colorSecondary)
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(colorPrimary)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(colorDim)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(colorPrimary)

	return EditorModel{
		Heading: heading,
		inner:   ta,
		VH:      vh,
		VW:      vw,
	}
}

func (m EditorModel) Content() string {
	return m.inner.Value()
}

func (m EditorModel) Update(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlS:
		content := m.inner.Value()
		return m, func() tea.Msg { return EditorSaveMsg{Content: content} }
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return EditorCancelMsg{} }
	}
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

func (m EditorModel) View() string {
	heading := styleCyanBold.Render(m.Heading)
	hints := styleGray.Render("Ctrl+S: save  Esc: cancel")
	info := m.inner.LineInfo()
	status := styleGray.Render(fmt.Sprintf("Ln %d, Col %d  |  %d lines", m.inner.Line()+1, info.ColumnOffset+1, m.inner.LineCount()))

	return heading + "\n\n" +
		lipgloss.NewStyle().Render(m.inner.View()) + "\n" +
		hints + "\n" + status
}
