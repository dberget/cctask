package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/claude"
)

// ChatInputModel wraps bubbles textinput for process follow-up chat.
type ChatInputModel struct {
	inner     textinput.Model
	ProcessID string
	SessionID string
}

func NewChatInput(processID, sessionID string) ChatInputModel {
	ti := textinput.New()
	ti.Focus()
	ti.Prompt = ""
	ti.Placeholder = "Type a follow-up message..."
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorWhite)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorSecondary)
	ti.CursorStyle = styleCursor
	ti.CharLimit = 0
	return ChatInputModel{
		inner:     ti,
		ProcessID: processID,
		SessionID: sessionID,
	}
}

func (m ChatInputModel) Update(msg tea.KeyMsg) (ChatInputModel, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		return m, func() tea.Msg { return chatCancelMsg{} }
	case msg.Type == tea.KeyEnter:
		text := strings.TrimSpace(m.inner.Value())
		if text != "" {
			return m, func() tea.Msg {
				return claude.ChatSubmitMsg{
					ProcessID: m.ProcessID,
					SessionID: m.SessionID,
					Message:   text,
				}
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

func (m ChatInputModel) View() string {
	return styleCyanBold.Render("Chat: ") + m.inner.View()
}

// chatCancelMsg is sent when chat input is cancelled via Esc.
type chatCancelMsg struct{}
