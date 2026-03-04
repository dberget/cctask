package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/model"
)

// FlashMsg displays a temporary message in the status bar.
type FlashMsg struct{ Text string }

// FlashClearMsg clears the flash message.
type FlashClearMsg struct{}

// ProcessAutoRemoveMsg triggers removal of a completed process after delay.
type ProcessAutoRemoveMsg struct{ ID string }

// SetModeMsg changes the view mode.
type SetModeMsg struct{ Mode model.ViewMode }

// TextSubmitMsg is sent when a text input submits.
type TextSubmitMsg struct{ Value string }

// TextCancelMsg is sent when a text input is cancelled.
type TextCancelMsg struct{}

// FormSubmitMsg is sent when the task form submits.
type FormSubmitMsg struct{ Data TaskFormData }

// FormCancelMsg is sent when the task form is cancelled.
type FormCancelMsg struct{}

// SelectSubmitMsg is sent when a select input selects.
type SelectSubmitMsg struct{ Value string }

// SelectCancelMsg is sent when a select input is cancelled.
type SelectCancelMsg struct{}

// MultiCheckSubmitMsg is sent when multi-check confirms.
type MultiCheckSubmitMsg struct{ Selected []string }

// MultiCheckCancelMsg is sent when multi-check is cancelled.
type MultiCheckCancelMsg struct{}

// EditorSaveMsg is sent when the editor saves.
type EditorSaveMsg struct{ Content string }

// EditorCancelMsg is sent when the editor is cancelled.
type EditorCancelMsg struct{}

// storeCheckTickMsg triggers a check for external changes to tasks.json.
type storeCheckTickMsg struct{}

func flashCmd(text string) tea.Cmd {
	return func() tea.Msg { return FlashMsg{Text: text} }
}

func clearFlashCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return FlashClearMsg{}
	})
}

func storeCheckTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return storeCheckTickMsg{}
	})
}
