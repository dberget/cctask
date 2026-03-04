package tui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommandSubmitMsg is emitted when the user presses Enter to execute a command.
type CommandSubmitMsg struct{ Input string }

// CommandCancelMsg is emitted when the user presses Esc to cancel the command bar.
type CommandCancelMsg struct{}

// CommandBarModel is a vim-style command bar (:command) with completion and history.
type CommandBarModel struct {
	Active bool
	buffer []rune
	cursor int

	// Suggestion state
	suggestions     []string
	suggestionIdx   int
	showSuggestions bool

	// History
	history      []string
	historyIdx   int
	historyDraft string

	// Registry for completions (nil-safe)
	registry *CommandRegistry
}

// NewCommandBar creates an inactive command bar.
func NewCommandBar() CommandBarModel {
	return CommandBarModel{}
}

// Activate resets the command bar state and sets it active.
func (cb CommandBarModel) Activate() CommandBarModel {
	cb.Active = true
	cb.buffer = nil
	cb.cursor = 0
	cb.suggestions = nil
	cb.suggestionIdx = 0
	cb.showSuggestions = false
	cb.historyIdx = -1
	cb.historyDraft = ""
	return cb
}

// Input returns the current input string.
func (cb CommandBarModel) Input() string {
	return string(cb.buffer)
}

// Update handles key events for the command bar.
func (cb CommandBarModel) Update(msg tea.KeyMsg) (CommandBarModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		cb.Active = false
		cb.showSuggestions = false
		return cb, func() tea.Msg { return CommandCancelMsg{} }

	case msg.Type == tea.KeyEnter:
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			// Accept the current suggestion
			accepted := cb.suggestions[cb.suggestionIdx]
			cb.buffer = []rune(accepted + " ")
			cb.cursor = len(cb.buffer)
			cb.showSuggestions = false
			cb.suggestions = nil
			cb = cb.updateSuggestions()
			return cb, nil
		}
		trimmed := strings.TrimSpace(string(cb.buffer))
		if trimmed != "" {
			cb.history = append(cb.history, trimmed)
		}
		cb.Active = false
		cb.showSuggestions = false
		input := trimmed
		return cb, func() tea.Msg { return CommandSubmitMsg{Input: input} }

	case msg.Type == tea.KeyTab:
		if len(cb.suggestions) > 0 {
			cb.showSuggestions = true
			cb.suggestionIdx = (cb.suggestionIdx + 1) % len(cb.suggestions)
		}
		return cb, nil

	case msg.Type == tea.KeyShiftTab:
		if len(cb.suggestions) > 0 {
			cb.showSuggestions = true
			cb.suggestionIdx--
			if cb.suggestionIdx < 0 {
				cb.suggestionIdx = len(cb.suggestions) - 1
			}
		}
		return cb, nil

	case msg.Type == tea.KeyUp:
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			cb.suggestionIdx--
			if cb.suggestionIdx < 0 {
				cb.suggestionIdx = len(cb.suggestions) - 1
			}
			return cb, nil
		}
		// History navigation
		if len(cb.history) == 0 {
			return cb, nil
		}
		if cb.historyIdx == -1 {
			cb.historyDraft = string(cb.buffer)
			cb.historyIdx = len(cb.history) - 1
		} else if cb.historyIdx > 0 {
			cb.historyIdx--
		}
		cb.buffer = []rune(cb.history[cb.historyIdx])
		cb.cursor = len(cb.buffer)
		return cb, nil

	case msg.Type == tea.KeyDown:
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			cb.suggestionIdx = (cb.suggestionIdx + 1) % len(cb.suggestions)
			return cb, nil
		}
		// History navigation
		if cb.historyIdx == -1 {
			return cb, nil
		}
		if cb.historyIdx < len(cb.history)-1 {
			cb.historyIdx++
			cb.buffer = []rune(cb.history[cb.historyIdx])
			cb.cursor = len(cb.buffer)
		} else {
			cb.historyIdx = -1
			cb.buffer = []rune(cb.historyDraft)
			cb.cursor = len(cb.buffer)
		}
		return cb, nil

	case msg.Type == tea.KeyBackspace:
		if cb.cursor > 0 {
			cb.buffer = append(cb.buffer[:cb.cursor-1], cb.buffer[cb.cursor:]...)
			cb.cursor--
			cb = cb.updateSuggestions()
		}
		return cb, nil

	case msg.Type == tea.KeyCtrlA:
		cb.cursor = 0
		return cb, nil

	case msg.Type == tea.KeyCtrlE:
		cb.cursor = len(cb.buffer)
		return cb, nil

	case msg.Type == tea.KeyCtrlW:
		// Delete word backwards
		if cb.cursor == 0 {
			return cb, nil
		}
		// Skip trailing spaces
		pos := cb.cursor
		for pos > 0 && unicode.IsSpace(cb.buffer[pos-1]) {
			pos--
		}
		// Delete until next space or start
		for pos > 0 && !unicode.IsSpace(cb.buffer[pos-1]) {
			pos--
		}
		cb.buffer = append(cb.buffer[:pos], cb.buffer[cb.cursor:]...)
		cb.cursor = pos
		cb = cb.updateSuggestions()
		return cb, nil

	case msg.Type == tea.KeyLeft:
		if cb.cursor > 0 {
			cb.cursor--
		}
		return cb, nil

	case msg.Type == tea.KeyRight:
		if cb.cursor < len(cb.buffer) {
			cb.cursor++
		}
		return cb, nil

	case msg.Type == tea.KeyRunes:
		// Insert runes at cursor
		runes := msg.Runes
		newBuf := make([]rune, 0, len(cb.buffer)+len(runes))
		newBuf = append(newBuf, cb.buffer[:cb.cursor]...)
		newBuf = append(newBuf, runes...)
		newBuf = append(newBuf, cb.buffer[cb.cursor:]...)
		cb.buffer = newBuf
		cb.cursor += len(runes)
		cb = cb.updateSuggestions()
		return cb, nil
	}

	return cb, nil
}

// updateSuggestions refreshes the suggestion list from the registry.
func (cb CommandBarModel) updateSuggestions() CommandBarModel {
	if cb.registry == nil {
		cb.suggestions = nil
		cb.showSuggestions = false
		return cb
	}
	input := string(cb.buffer)
	// Don't show suggestions on empty input — wait for user to type something
	if strings.TrimSpace(input) == "" {
		cb.suggestions = nil
		cb.showSuggestions = false
		return cb
	}
	cb.suggestions = cb.registry.Complete(input)
	if len(cb.suggestions) > 0 {
		cb.showSuggestions = true
		cb.suggestionIdx = 0
	} else {
		cb.showSuggestions = false
		cb.suggestionIdx = 0
	}
	return cb
}

// View renders the command bar input line.
func (cb CommandBarModel) View(width int) string {
	prompt := styleCyanBold.Render(":")

	input := cb.buffer
	if len(input) == 0 {
		// Show just the cursor
		cursor := styleCursor.Render(" ")
		return prompt + cursor
	}

	// Render with cursor
	var out strings.Builder
	for i, r := range input {
		if i == cb.cursor {
			out.WriteString(styleCursor.Render(string(r)))
		} else {
			out.WriteRune(r)
		}
	}
	// If cursor is at end, show cursor block after text
	if cb.cursor >= len(input) {
		out.WriteString(styleCursor.Render(" "))
	}

	return prompt + out.String()
}

// SuggestionsView renders the suggestion popup above the command bar.
func (cb CommandBarModel) SuggestionsView(width int) string {
	if !cb.showSuggestions || len(cb.suggestions) == 0 {
		return ""
	}

	// Show a window of up to 5 suggestions around the selected index
	maxVisible := 5
	total := len(cb.suggestions)
	start := 0
	end := total
	if total > maxVisible {
		start = cb.suggestionIdx - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > total {
			end = total
			start = end - maxVisible
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		s := cb.suggestions[i]
		if i == cb.suggestionIdx {
			lines = append(lines, styleCyanBold.Render("  "+s))
		} else {
			lines = append(lines, styleGray.Render("  "+s))
		}
	}

	content := strings.Join(lines, "\n")
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Render(content)

	return box
}

// LoadHistory reads command history from .cctask/command_history.
func LoadHistory(projectRoot string) []string {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".cctask", "command_history"))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	return lines
}

// SaveHistory writes command history to .cctask/command_history.
func SaveHistory(projectRoot string, history []string) {
	dir := filepath.Join(projectRoot, ".cctask")
	os.MkdirAll(dir, 0o755)
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	data := strings.Join(history, "\n") + "\n"
	os.WriteFile(filepath.Join(dir, "command_history"), []byte(data), 0o644)
}
