package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- TextInputModel wraps bubbles textinput ---

type TextInputModel struct {
	Label string
	inner textinput.Model
}

func NewTextInput(label, initial string) TextInputModel {
	ti := textinput.New()
	ti.SetValue(initial)
	ti.Focus()
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSecondary)
	ti.CursorStyle = styleCursor
	ti.CharLimit = 0 // no limit
	return TextInputModel{
		Label: label,
		inner: ti,
	}
}

func (m TextInputModel) Update(msg tea.KeyMsg) (TextInputModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return TextCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		if strings.TrimSpace(m.inner.Value()) != "" {
			val := strings.TrimSpace(m.inner.Value())
			return m, func() tea.Msg { return TextSubmitMsg{Value: val} }
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

func (m TextInputModel) View() string {
	return styleCyanBold.Render(m.Label+": ") + m.inner.View()
}

func (m TextInputModel) Value() string {
	return m.inner.Value()
}

// SetPlaceholder sets placeholder text on the underlying textinput.
func (m *TextInputModel) SetPlaceholder(p string) {
	m.inner.Placeholder = p
}

// --- SelectInputModel (kept as-is, too simple to benefit from replacement) ---

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
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		if m.Index > 0 {
			m.Index--
		}
	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
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
	case msg.Type == tea.KeySpace:
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
