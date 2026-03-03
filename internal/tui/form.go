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
	WorkDir     string
}

type formField int

const (
	fieldTitle formField = iota
	fieldDescription
	fieldTags
	fieldWorkDir
	fieldCount
)

var fieldLabels = [fieldCount]string{"Title", "Description", "Tags", "WorkDir"}
var fieldHints = [fieldCount]string{"", "task details, context, acceptance criteria", "comma-separated", "relative or absolute path"}

type FormModel struct {
	Heading string
	Active  formField
	Values  [fieldCount]string
	Cursors [fieldCount]int
	Width   int

	// Multiline description state
	DescLines []string
	DescLine  int
	DescCol   int
}

func NewForm(heading string, initial *TaskFormData, width int) FormModel {
	m := FormModel{Heading: heading, Width: width}
	if initial != nil {
		m.Values[fieldTitle] = initial.Title
		m.Values[fieldDescription] = initial.Description
		m.Values[fieldTags] = initial.Tags
		m.Values[fieldWorkDir] = initial.WorkDir
		m.Cursors[fieldTitle] = len(initial.Title)
		m.Cursors[fieldTags] = len(initial.Tags)
		m.Cursors[fieldWorkDir] = len(initial.WorkDir)
	}
	// Initialize DescLines from description
	desc := m.Values[fieldDescription]
	if desc == "" {
		m.DescLines = []string{""}
	} else {
		m.DescLines = strings.Split(desc, "\n")
	}
	m.DescLine = len(m.DescLines) - 1
	m.DescCol = len(m.DescLines[m.DescLine])
	return m
}

func (m FormModel) Data() TaskFormData {
	return TaskFormData{
		Title:       m.Values[fieldTitle],
		Description: strings.Join(m.DescLines, "\n"),
		Tags:        m.Values[fieldTags],
		WorkDir:     m.Values[fieldWorkDir],
	}
}

func (m FormModel) Update(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	// Global keys
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return FormCancelMsg{} }
	case msg.Type == tea.KeyCtrlS || msg.Type == tea.KeyCtrlD:
		if strings.TrimSpace(m.Values[fieldTitle]) != "" {
			data := m.Data()
			return m, func() tea.Msg { return FormSubmitMsg{Data: data} }
		}
		return m, nil
	case msg.Type == tea.KeyTab:
		m.Active = formField((int(m.Active) + 1) % int(fieldCount))
		return m, nil
	case msg.Type == tea.KeyShiftTab:
		m.Active = formField((int(m.Active) - 1 + int(fieldCount)) % int(fieldCount))
		return m, nil
	}

	if m.Active == fieldDescription {
		return m.updateDescription(msg)
	}
	return m.updateSingleLine(msg)
}

func (m FormModel) updateSingleLine(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	f := m.Active
	val := m.Values[f]
	cursor := m.Cursors[f]

	switch {
	case msg.Type == tea.KeyEnter:
		if f == fieldWorkDir {
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

// descVisualRows builds a flat list of visual rows across all logical lines,
// each tagged with the logical line index and byte offset within that line.
type descVRow struct {
	logLine int
	offset  int
	length  int
}

func (m FormModel) descVisualRows() []descVRow {
	w := m.descAvailWidth()
	var all []descVRow
	for i, dl := range m.DescLines {
		_, offsets := softWrapLine(dl, w)
		for vi, off := range offsets {
			var ln int
			if vi+1 < len(offsets) {
				ln = offsets[vi+1] - off
			} else {
				ln = len(dl) - off
			}
			all = append(all, descVRow{logLine: i, offset: off, length: ln})
		}
	}
	return all
}

func (m FormModel) updateDescription(msg tea.KeyMsg) (FormModel, tea.Cmd) {
	line := m.DescLine
	col := m.DescCol
	cur := m.DescLines[line]

	switch {
	case msg.Type == tea.KeyEnter:
		// Split current line at cursor
		before := cur[:col]
		after := cur[col:]
		newLines := make([]string, 0, len(m.DescLines)+1)
		newLines = append(newLines, m.DescLines[:line]...)
		newLines = append(newLines, before, after)
		if line+1 < len(m.DescLines) {
			newLines = append(newLines, m.DescLines[line+1:]...)
		}
		m.DescLines = newLines
		m.DescLine = line + 1
		m.DescCol = 0

	case msg.Type == tea.KeyBackspace:
		if col > 0 {
			m.DescLines[line] = cur[:col-1] + cur[col:]
			m.DescCol = col - 1
		} else if line > 0 {
			// Join with previous line
			prevLen := len(m.DescLines[line-1])
			m.DescLines[line-1] += cur
			m.DescLines = append(m.DescLines[:line], m.DescLines[line+1:]...)
			m.DescLine = line - 1
			m.DescCol = prevLen
		}

	case msg.Type == tea.KeyUp, msg.Type == tea.KeyDown:
		vrows := m.descVisualRows()
		// Find current visual row
		curVRow := 0
		for vi, vr := range vrows {
			if vr.logLine == line && col >= vr.offset && col <= vr.offset+vr.length {
				curVRow = vi
				break
			}
		}
		localCol := col - vrows[curVRow].offset
		var targetVRow int
		if msg.Type == tea.KeyUp {
			targetVRow = curVRow - 1
		} else {
			targetVRow = curVRow + 1
		}
		if targetVRow >= 0 && targetVRow < len(vrows) {
			tv := vrows[targetVRow]
			m.DescLine = tv.logLine
			newCol := tv.offset + localCol
			maxCol := tv.offset + tv.length
			if newCol > maxCol {
				newCol = maxCol
			}
			m.DescCol = newCol
		}

	case msg.Type == tea.KeyLeft:
		if col > 0 {
			m.DescCol = col - 1
		} else if line > 0 {
			m.DescLine = line - 1
			m.DescCol = len(m.DescLines[line-1])
		}

	case msg.Type == tea.KeyRight:
		if col < len(cur) {
			m.DescCol = col + 1
		} else if line < len(m.DescLines)-1 {
			m.DescLine = line + 1
			m.DescCol = 0
		}

	case msg.Type == tea.KeyCtrlA:
		m.DescCol = 0

	case msg.Type == tea.KeyCtrlE:
		m.DescCol = len(cur)

	case msg.Type == tea.KeySpace:
		m.DescLines[line] = cur[:col] + " " + cur[col:]
		m.DescCol = col + 1

	case msg.Type == tea.KeyRunes:
		ch := string(msg.Runes)
		m.DescLines[line] = cur[:col] + ch + cur[col:]
		m.DescCol = col + len(ch)
	}
	return m, nil
}

func (m FormModel) View() string {
	var lines []string
	lines = append(lines, styleCyanBold.Render(m.Heading))
	lines = append(lines, styleGray.Render("Tab: next field  Enter: next/newline  Ctrl+S: save  Esc: cancel"))
	lines = append(lines, "")

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
		labelStr := lipgloss.NewStyle().Foreground(labelColor).Bold(isActive).Render(indicator + fieldLabels[i] + ":")
		labelPadded := lipgloss.NewStyle().Width(16).Render(labelStr)

		if i == fieldDescription {
			lines = append(lines, m.viewDescription(isActive, labelPadded)...)
		} else {
			val := m.Values[i]
			cursor := m.Cursors[i]
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
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// softWrapLine breaks a single line into visual rows of at most width chars.
// Returns the visual rows and, for each row, the starting byte offset into the original line.
func softWrapLine(line string, width int) (rows []string, offsets []int) {
	if width < 10 {
		width = 10
	}
	if len(line) <= width {
		return []string{line}, []int{0}
	}
	pos := 0
	for pos < len(line) {
		end := pos + width
		if end >= len(line) {
			rows = append(rows, line[pos:])
			offsets = append(offsets, pos)
			break
		}
		// Try to break at a space
		breakAt := end
		for i := end; i > pos+width/2; i-- {
			if line[i] == ' ' {
				breakAt = i
				break
			}
		}
		rows = append(rows, line[pos:breakAt])
		offsets = append(offsets, pos)
		pos = breakAt
		if pos < len(line) && line[pos] == ' ' {
			pos++
		}
	}
	if len(rows) == 0 {
		return []string{""}, []int{0}
	}
	return rows, offsets
}

func (m FormModel) descAvailWidth() int {
	// Width is the full terminal; subtract 4 for outer padding (2 left + 2 right)
	// and 16 for the label column
	w := m.Width - 4 - 16
	if w < 20 {
		w = 60
	}
	return w
}

func (m FormModel) viewDescription(isActive bool, labelPadded string) []string {
	var rows []string
	indent := strings.Repeat(" ", 16)
	availWidth := m.descAvailWidth()

	if isActive {
		firstRow := true
		for i, dl := range m.DescLines {
			vRows, vOffsets := softWrapLine(dl, availWidth)
			isCursorLine := i == m.DescLine

			for vi, vr := range vRows {
				prefix := indent
				if firstRow {
					prefix = labelPadded
					firstRow = false
				}

				if isCursorLine {
					// Check if cursor falls on this visual row
					rowStart := vOffsets[vi]
					var rowEnd int
					if vi+1 < len(vOffsets) {
						rowEnd = vOffsets[vi+1]
					} else {
						rowEnd = len(dl)
					}
					if m.DescCol >= rowStart && m.DescCol <= rowEnd {
						localCol := m.DescCol - rowStart
						rows = append(rows, prefix+renderEditableLine(vr, localCol))
					} else {
						rows = append(rows, prefix+vr)
					}
				} else {
					rows = append(rows, prefix+vr)
				}
			}
		}
	} else {
		joined := strings.Join(m.DescLines, "\n")
		if strings.TrimSpace(joined) == "" {
			hint := fieldHints[fieldDescription]
			rows = append(rows, labelPadded+styleGray.Render("("+hint+")"))
		} else {
			wrapped := wrapText(joined, availWidth)
			for i, wl := range strings.Split(wrapped, "\n") {
				prefix := indent
				if i == 0 {
					prefix = labelPadded
				}
				rows = append(rows, prefix+wl)
			}
		}
	}
	return rows
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
