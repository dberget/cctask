package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type editorMode int

const (
	editorNormal editorMode = iota
	editorInsert
)

type EditorModel struct {
	Heading string
	Lines   []string
	Line    int
	Col     int
	Mode    editorMode
	Scroll  int
	VH      int
	VW      int
}

func NewEditor(heading string, initial string, vh, vw int) EditorModel {
	lines := strings.Split(initial, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	if vh < 10 {
		vh = 10
	}
	if vw < 40 {
		vw = 40
	}
	return EditorModel{
		Heading: heading,
		Lines:   lines,
		VH:      vh,
		VW:      vw,
	}
}

func (m EditorModel) Content() string {
	return strings.Join(m.Lines, "\n")
}

func (m EditorModel) Update(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	if m.Mode == editorNormal {
		return m.updateNormal(msg)
	}
	return m.updateInsert(msg)
}

func (m EditorModel) updateNormal(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	key := msg.String()
	switch {
	case key == "ctrl+s" || key == ":":
		content := m.Content()
		return m, func() tea.Msg { return EditorSaveMsg{Content: content} }
	case key == "q" || key == "esc":
		return m, func() tea.Msg { return EditorCancelMsg{} }

	case key == "h" || key == "left":
		m.Col = max(0, m.Col-1)
	case key == "l" || key == "right":
		m.Col = m.clampCol(m.Col + 1)
	case key == "j" || key == "down":
		m.Line = min(len(m.Lines)-1, m.Line+1)
		m.Col = m.clampCol(m.Col)
	case key == "k" || key == "up":
		m.Line = max(0, m.Line-1)
		m.Col = m.clampCol(m.Col)
	case key == "0":
		m.Col = 0
	case key == "$":
		m.Col = m.maxCol()
	case key == "w":
		m.wordForward()
	case key == "b":
		m.wordBackward()
	case key == "G":
		m.Line = len(m.Lines) - 1
		m.Col = 0
	case key == "ctrl+g":
		m.Line = 0
		m.Col = 0

	case key == "i":
		m.Mode = editorInsert
	case key == "a":
		m.Mode = editorInsert
		m.Col = min(m.Col+1, len(m.Lines[m.Line]))
	case key == "I":
		m.Mode = editorInsert
		m.Col = 0
	case key == "A":
		m.Mode = editorInsert
		m.Col = len(m.Lines[m.Line])
	case key == "o":
		m.openLine(true)
	case key == "O":
		m.openLine(false)
	case key == "d":
		m.deleteLine()
	case key == "x":
		m.deleteChar()
	}

	m.ensureVisible()
	return m, nil
}

func (m EditorModel) updateInsert(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	key := msg.String()
	switch {
	case key == "esc":
		m.Mode = editorNormal
		m.Col = max(0, m.Col-1)
	case key == "ctrl+s":
		content := m.Content()
		return m, func() tea.Msg { return EditorSaveMsg{Content: content} }
	case key == "enter":
		m.insertNewline()
	case key == "backspace":
		m.backspace()
	case key == "left":
		m.Col = max(0, m.Col-1)
	case key == "right":
		m.Col = min(len(m.Lines[m.Line]), m.Col+1)
	case key == "up":
		m.Line = max(0, m.Line-1)
		m.Col = min(m.Col, len(m.Lines[m.Line]))
	case key == "down":
		m.Line = min(len(m.Lines)-1, m.Line+1)
		m.Col = min(m.Col, len(m.Lines[m.Line]))
	case key == "tab":
		m.insertText("  ")
	default:
		if msg.Type == tea.KeyRunes {
			m.insertText(string(msg.Runes))
		}
	}
	m.ensureVisible()
	return m, nil
}

func (m *EditorModel) clampCol(col int) int {
	maxC := max(0, len(m.Lines[m.Line])-1)
	if m.Mode == editorInsert {
		maxC = len(m.Lines[m.Line])
	}
	return min(col, maxC)
}

func (m *EditorModel) maxCol() int {
	return max(0, len(m.Lines[m.Line])-1)
}

func (m *EditorModel) ensureVisible() {
	if m.Line < m.Scroll {
		m.Scroll = m.Line
	}
	if m.Line >= m.Scroll+m.VH {
		m.Scroll = m.Line - m.VH + 1
	}
}

func (m *EditorModel) wordForward() {
	line := m.Lines[m.Line]
	c := m.Col
	for c < len(line) && line[c] != ' ' {
		c++
	}
	for c < len(line) && line[c] == ' ' {
		c++
	}
	m.Col = c
}

func (m *EditorModel) wordBackward() {
	line := m.Lines[m.Line]
	c := m.Col
	if c > 0 {
		c--
	}
	for c > 0 && line[c] == ' ' {
		c--
	}
	for c > 0 && line[c-1] != ' ' {
		c--
	}
	m.Col = c
}

func (m *EditorModel) insertText(text string) {
	line := m.Lines[m.Line]
	m.Lines[m.Line] = line[:m.Col] + text + line[m.Col:]
	m.Col += len(text)
}

func (m *EditorModel) insertNewline() {
	line := m.Lines[m.Line]
	before := line[:m.Col]
	after := line[m.Col:]
	m.Lines[m.Line] = before
	newLines := make([]string, 0, len(m.Lines)+1)
	newLines = append(newLines, m.Lines[:m.Line+1]...)
	newLines = append(newLines, after)
	newLines = append(newLines, m.Lines[m.Line+1:]...)
	m.Lines = newLines
	m.Line++
	m.Col = 0
}

func (m *EditorModel) backspace() {
	if m.Col > 0 {
		line := m.Lines[m.Line]
		m.Lines[m.Line] = line[:m.Col-1] + line[m.Col:]
		m.Col--
	} else if m.Line > 0 {
		prevLen := len(m.Lines[m.Line-1])
		m.Lines[m.Line-1] = m.Lines[m.Line-1] + m.Lines[m.Line]
		m.Lines = append(m.Lines[:m.Line], m.Lines[m.Line+1:]...)
		m.Line--
		m.Col = prevLen
	}
}

func (m *EditorModel) deleteLine() {
	if len(m.Lines) == 1 {
		m.Lines[0] = ""
		m.Col = 0
		return
	}
	m.Lines = append(m.Lines[:m.Line], m.Lines[m.Line+1:]...)
	m.Line = min(m.Line, len(m.Lines)-1)
	m.Col = m.clampCol(m.Col)
}

func (m *EditorModel) deleteChar() {
	line := m.Lines[m.Line]
	if len(line) == 0 {
		return
	}
	m.Lines[m.Line] = line[:m.Col] + line[min(m.Col+1, len(line)):]
	m.Col = m.clampCol(m.Col)
}

func (m *EditorModel) openLine(below bool) {
	insertAt := m.Line
	if below {
		insertAt = m.Line + 1
	}
	newLines := make([]string, 0, len(m.Lines)+1)
	newLines = append(newLines, m.Lines[:insertAt]...)
	newLines = append(newLines, "")
	newLines = append(newLines, m.Lines[insertAt:]...)
	m.Lines = newLines
	if below {
		m.Line++
	}
	m.Col = 0
	m.Mode = editorInsert
}

func (m EditorModel) View() string {
	var sb strings.Builder

	modeText := styleYellow.Render("-- NORMAL --")
	if m.Mode == editorInsert {
		modeText = styleGreen.Render("-- INSERT --")
	}
	sb.WriteString(styleCyanBold.Render(m.Heading) + "  " + modeText + "\n\n")

	gutterWidth := len(fmt.Sprintf("%d", len(m.Lines))) + 1
	visibleEnd := min(m.Scroll+m.VH, len(m.Lines))
	for i := m.Scroll; i < visibleEnd; i++ {
		line := m.Lines[i]
		if len(line) > m.VW {
			line = line[:m.VW-1] + "…"
		}

		gutter := styleGray.Render(fmt.Sprintf("%*d ", gutterWidth, i+1))

		if i == m.Line {
			sb.WriteString(gutter + renderCursorLine(line, m.Col) + "\n")
		} else {
			display := line
			if display == "" {
				display = " "
			}
			sb.WriteString(gutter + display + "\n")
		}
	}

	var hints string
	if m.Mode == editorNormal {
		hints = "i:insert  a:append  o:newline  d:del-line  x:del-char  Ctrl+S:save  q/Esc:cancel"
	} else {
		hints = "Esc:normal  Ctrl+S:save  Enter:newline  Tab:indent"
	}
	sb.WriteString("\n" + styleGray.Render(hints))
	sb.WriteString(fmt.Sprintf("\n"+styleGray.Render("Ln %d, Col %d  |  %d lines"), m.Line+1, m.Col+1, len(m.Lines)))

	return lipgloss.NewStyle().Render(sb.String())
}

func renderCursorLine(value string, cursor int) string {
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
