package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- TextInputModel ---

type TextInputModel struct {
	Label       string
	Placeholder string
	Value       string
	Cursor      int
}

func NewTextInput(label, initial string) TextInputModel {
	return TextInputModel{
		Label:  label,
		Value:  initial,
		Cursor: len(initial),
	}
}

func (m TextInputModel) Update(msg tea.KeyMsg) (TextInputModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return TextCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		if strings.TrimSpace(m.Value) != "" {
			return m, func() tea.Msg { return TextSubmitMsg{Value: strings.TrimSpace(m.Value)} }
		}
	case msg.Type == tea.KeyBackspace:
		if m.Cursor > 0 {
			m.Value = m.Value[:m.Cursor-1] + m.Value[m.Cursor:]
			m.Cursor--
		}
	case msg.Type == tea.KeyLeft:
		if m.Cursor > 0 {
			m.Cursor--
		}
	case msg.Type == tea.KeyRight:
		if m.Cursor < len(m.Value) {
			m.Cursor++
		}
	case msg.Type == tea.KeySpace:
		m.Value = m.Value[:m.Cursor] + " " + m.Value[m.Cursor:]
		m.Cursor++
	case msg.Type == tea.KeyRunes:
		ch := string(msg.Runes)
		m.Value = m.Value[:m.Cursor] + ch + m.Value[m.Cursor:]
		m.Cursor += len(ch)
	}
	return m, nil
}

func (m TextInputModel) View() string {
	displayValue := m.Value
	isPlaceholder := false
	if displayValue == "" && m.Placeholder != "" {
		displayValue = m.Placeholder
		isPlaceholder = true
	}

	textColor := colorWhite
	if isPlaceholder {
		textColor = colorSecondary
	}

	before := lipgloss.NewStyle().Foreground(textColor).Render(safeSlice(displayValue, 0, m.Cursor))
	cursorChar := " "
	if m.Cursor < len(displayValue) {
		cursorChar = string(displayValue[m.Cursor])
	}
	cursor := styleCursor.Render(cursorChar)
	after := lipgloss.NewStyle().Foreground(textColor).Render(safeSlice(displayValue, m.Cursor+1, len(displayValue)))

	return styleCyanBold.Render(m.Label+": ") + before + cursor + after
}

// --- SelectInputModel ---

type SelectItem struct {
	Label string
	Value string
}

type SelectInputModel struct {
	Label string
	Items []SelectItem
	Index int
}

func NewSelectInput(label string, items []SelectItem) SelectInputModel {
	return SelectInputModel{
		Label: label,
		Items: items,
	}
}

func (m SelectInputModel) Update(msg tea.KeyMsg) (SelectInputModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return SelectCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		if len(m.Items) > 0 {
			val := m.Items[m.Index].Value
			return m, func() tea.Msg { return SelectSubmitMsg{Value: val} }
		}
	case msg.Type == tea.KeyUp:
		if m.Index > 0 {
			m.Index--
		}
	case msg.Type == tea.KeyDown:
		if m.Index < len(m.Items)-1 {
			m.Index++
		}
	}
	return m, nil
}

func (m SelectInputModel) View() string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(m.Label))
	for i, item := range m.Items {
		indicator := "  "
		color := colorWhite
		if i == m.Index {
			indicator = "▸ "
			color = colorPrimary
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(color).Render(indicator+item.Label))
	}
	return strings.Join(lines, "\n")
}

// --- MultiCheckModel ---

type CheckItem struct {
	Label    string
	Value    string
	Disabled bool
}

type MultiCheckModel struct {
	Label    string
	Items    []CheckItem
	Index    int
	Selected map[string]bool
}

func NewMultiCheck(label string, items []CheckItem) MultiCheckModel {
	return MultiCheckModel{
		Label:    label,
		Items:    items,
		Selected: make(map[string]bool),
	}
}

func (m MultiCheckModel) Update(msg tea.KeyMsg) (MultiCheckModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return MultiCheckCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		if len(m.Selected) > 0 {
			var selected []string
			for v := range m.Selected {
				selected = append(selected, v)
			}
			return m, func() tea.Msg { return MultiCheckSubmitMsg{Selected: selected} }
		}
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		if m.Index > 0 {
			m.Index--
		}
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		if m.Index < len(m.Items)-1 {
			m.Index++
		}
	case msg.Type == tea.KeyRunes && string(msg.Runes) == " ":
		item := m.Items[m.Index]
		if !item.Disabled {
			if m.Selected[item.Value] {
				delete(m.Selected, item.Value)
			} else {
				m.Selected[item.Value] = true
			}
		}
	}
	return m, nil
}

func (m MultiCheckModel) View() string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(m.Label))
	lines = append(lines, styleGray.Render(fmt.Sprintf("Space: toggle  Enter: confirm (%d selected)  Esc: cancel", len(m.Selected))))

	for i, item := range m.Items {
		isFocused := i == m.Index
		isChecked := m.Selected[item.Value]

		color := colorWhite
		if item.Disabled {
			color = colorSecondary
		} else if isFocused {
			color = colorPrimary
		}

		indicator := "  "
		if isFocused {
			indicator = "▸ "
		}
		check := "[ ] "
		if isChecked {
			check = "[✓] "
		}
		suffix := ""
		if item.Disabled {
			suffix = " (no plan)"
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(color).Render(indicator+check+item.Label+suffix))
	}
	return strings.Join(lines, "\n")
}

func safeSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}
