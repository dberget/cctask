package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/claude"
)

// ChatInputModel is a text input for the embedded process chat.
type ChatInputModel struct {
	Value     string
	Cursor    int
	ProcessID string
	SessionID string
}

func NewChatInput(processID, sessionID string) ChatInputModel {
	return ChatInputModel{
		ProcessID: processID,
		SessionID: sessionID,
	}
}

func (m ChatInputModel) Update(msg tea.KeyMsg) (ChatInputModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return chatCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		text := strings.TrimSpace(m.Value)
		if text != "" {
			return m, func() tea.Msg {
				return claude.ChatSubmitMsg{
					ProcessID: m.ProcessID,
					SessionID: m.SessionID,
					Message:   text,
				}
			}
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
	case msg.Type == tea.KeyCtrlA:
		m.Cursor = 0
	case msg.Type == tea.KeyCtrlE:
		m.Cursor = len(m.Value)
	case msg.Type == tea.KeyCtrlW:
		if m.Cursor > 0 {
			i := m.Cursor - 1
			for i > 0 && m.Value[i-1] == ' ' {
				i--
			}
			for i > 0 && m.Value[i-1] != ' ' {
				i--
			}
			m.Value = m.Value[:i] + m.Value[m.Cursor:]
			m.Cursor = i
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

func (m ChatInputModel) View() string {
	displayValue := m.Value
	placeholder := "Type a follow-up message..."
	isPlaceholder := false
	if displayValue == "" {
		displayValue = placeholder
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

	return styleCyanBold.Render("Chat: ") + before + cursor + after
}

// chatCancelMsg is sent when chat input is cancelled via Esc.
type chatCancelMsg struct{}
