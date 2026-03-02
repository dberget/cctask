package claude

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/model"
	claudecode "github.com/severity1/claude-agent-sdk-go"
)

// ProcessCancels is a registry of cancel functions for running processes.
type ProcessCancels struct {
	m sync.Map
}

// Register stores a cancel function for the given process ID.
func (pc *ProcessCancels) Register(procID string, cancel context.CancelFunc) {
	pc.m.Store(procID, cancel)
}

// Cancel cancels the process with the given ID and removes it from the registry.
// Returns true if the process was found and cancelled.
func (pc *ProcessCancels) Cancel(procID string) bool {
	if v, ok := pc.m.LoadAndDelete(procID); ok {
		v.(context.CancelFunc)()
		return true
	}
	return false
}

// Remove removes a process from the registry without cancelling it.
func (pc *ProcessCancels) Remove(procID string) {
	pc.m.Delete(procID)
}

// StreamEventMsg delivers a single structured event to the TUI.
type StreamEventMsg struct {
	ProcessID string
	Event     model.StreamEvent
	SessionID string
}

// StreamDoneMsg signals that a streaming process has finished.
type StreamDoneMsg struct {
	ProcessID string
	SessionID string
	Err       error
	TurnCount int
	CostUSD   float64
	FinalText string // Collected text output for plan saving etc.
}

// ChatSubmitMsg is sent when the user submits a follow-up message in embedded chat.
type ChatSubmitMsg struct {
	ProcessID string
	SessionID string
	Message   string
}

// SpawnStreamCmd creates a tea.Cmd that runs a Claude query using the SDK,
// streaming structured events via p.Send() and returning StreamDoneMsg on completion.
func SpawnStreamCmd(p *tea.Program, projectRoot string, procID string, prompt string, permissionMode string, cancels *ProcessCancels) tea.Cmd {
	return func() tea.Msg {
		return runStream(p, projectRoot, procID, prompt, "", permissionMode, cancels)
	}
}

// SpawnStreamResumeCmd creates a tea.Cmd that resumes an existing Claude session.
func SpawnStreamResumeCmd(p *tea.Program, projectRoot string, procID string, sessionID string, prompt string, cancels *ProcessCancels) tea.Cmd {
	return func() tea.Msg {
		return runStream(p, projectRoot, procID, prompt, sessionID, "", cancels)
	}
}

func runStream(p *tea.Program, projectRoot string, procID string, prompt string, sessionID string, permissionMode string, cancels *ProcessCancels) tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Register so the TUI can cancel this process
	if cancels != nil {
		cancels.Register(procID, cancel)
		defer cancels.Remove(procID)
	}

	var allText strings.Builder
	var finalSessionID string
	var turnCount int
	var costUSD float64

	opts := []claudecode.Option{
		claudecode.WithCwd(projectRoot),
	}
	if permissionMode != "" {
		opts = append(opts, claudecode.WithPermissionMode(claudecode.PermissionMode(permissionMode)))
	}
	if sessionID != "" {
		opts = append(opts, claudecode.WithResume(sessionID))
	}

	err := claudecode.WithClient(ctx, func(client claudecode.Client) error {
		if err := client.Query(ctx, prompt); err != nil {
			return fmt.Errorf("query: %w", err)
		}

		msgChan := client.ReceiveMessages(ctx)
		for {
			select {
			case msg, ok := <-msgChan:
				if !ok {
					return nil // channel closed
				}
				events := convertMessageToEvents(msg)
				for _, ev := range events {
					if ev.Kind == model.EventText {
						allText.WriteString(ev.Text)
					}
					p.Send(StreamEventMsg{
						ProcessID: procID,
						Event:     ev,
						SessionID: finalSessionID,
					})
				}

				// Extract session info from ResultMessage.
				// ResultMessage is the terminal message for a turn — break after
				// processing it. The SDK runs Claude in interactive streaming mode
				// (--input-format stream-json), so the subprocess stays alive after
				// sending the result. Without breaking, the loop blocks forever
				// waiting for the channel to close.
				if result, ok := msg.(*claudecode.ResultMessage); ok {
					finalSessionID = result.SessionID
					turnCount = result.NumTurns
					if result.TotalCostUSD != nil {
						costUSD = *result.TotalCostUSD
					}
					return nil
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}, opts...)

	return StreamDoneMsg{
		ProcessID: procID,
		SessionID: finalSessionID,
		Err:       err,
		TurnCount: turnCount,
		CostUSD:   costUSD,
		FinalText: allText.String(),
	}
}

// convertMessageToEvents maps an SDK message to one or more StreamEvents.
func convertMessageToEvents(msg claudecode.Message) []model.StreamEvent {
	now := time.Now()
	var events []model.StreamEvent

	switch m := msg.(type) {
	case *claudecode.AssistantMessage:
		for _, block := range m.Content {
			switch b := block.(type) {
			case *claudecode.TextBlock:
				if b.Text != "" {
					events = append(events, model.StreamEvent{
						Kind:      model.EventText,
						Text:      b.Text,
						Timestamp: now,
					})
				}
			case *claudecode.ThinkingBlock:
				if b.Thinking != "" {
					events = append(events, model.StreamEvent{
						Kind:      model.EventThinking,
						Text:      b.Thinking,
						Timestamp: now,
					})
				}
			case *claudecode.ToolUseBlock:
				events = append(events, model.StreamEvent{
					Kind:      model.EventToolUse,
					ToolName:  b.Name,
					ToolID:    b.ToolUseID,
					ToolInput: formatToolInput(b.Name, b.Input),
					Timestamp: now,
				})
			case *claudecode.ToolResultBlock:
				resultText := extractToolResultText(b.Content)
				isErr := false
				if b.IsError != nil {
					isErr = *b.IsError
				}
				events = append(events, model.StreamEvent{
					Kind:       model.EventToolResult,
					ToolID:     b.ToolUseID,
					ToolResult: truncateString(resultText, 500),
					IsError:    isErr,
					Timestamp:  now,
				})
			}
		}
	case *claudecode.UserMessage:
		content := extractUserMessageText(m.Content)
		if content != "" {
			events = append(events, model.StreamEvent{
				Kind:      model.EventUserMsg,
				Text:      content,
				Timestamp: now,
			})
		}
	case *claudecode.SystemMessage:
		events = append(events, model.StreamEvent{
			Kind:      model.EventSystem,
			Text:      m.Subtype,
			Timestamp: now,
		})
	case *claudecode.ResultMessage:
		// ResultMessage is handled in the caller for metadata extraction.
		// Optionally emit a system event for visibility.
		if m.IsError {
			errText := "Process completed with error"
			if m.Result != nil {
				errText = *m.Result
			}
			events = append(events, model.StreamEvent{
				Kind:      model.EventSystem,
				Text:      errText,
				IsError:   true,
				Timestamp: now,
			})
		}
	}

	return events
}

// CollectTextFromEvents joins all EventText content from a slice of events.
func CollectTextFromEvents(events []model.StreamEvent) string {
	var sb strings.Builder
	for _, ev := range events {
		if ev.Kind == model.EventText {
			sb.WriteString(ev.Text)
		}
	}
	return sb.String()
}

// extractUserMessageText extracts readable text from UserMessage.Content,
// which is interface{} — either a string or []ContentBlock (slice of pointers).
func extractUserMessageText(content interface{}) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	// Content is []ContentBlock — extract text from TextBlocks
	if blocks, ok := content.([]claudecode.ContentBlock); ok {
		var parts []string
		for _, block := range blocks {
			if tb, ok := block.(*claudecode.TextBlock); ok && tb.Text != "" {
				parts = append(parts, tb.Text)
			}
		}
		return strings.Join(parts, " ")
	}
	// Fallback: try []interface{} (JSON unmarshaled)
	if arr, ok := content.([]interface{}); ok {
		var parts []string
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " ")
		}
	}
	return ""
}

// formatToolInput returns a human-friendly summary of tool input,
// similar to what Claude Code displays (e.g. "Bash: grep -r pattern ." or "Read: src/main.go").
func formatToolInput(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}

	// Tool-specific formatting: show the most useful parameter
	switch toolName {
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			return truncateString(cmd, 200)
		}
	case "Read":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Write":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Edit":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return p
		}
	case "Grep":
		parts := []string{}
		if p, ok := input["pattern"].(string); ok {
			parts = append(parts, p)
		}
		if p, ok := input["path"].(string); ok {
			parts = append(parts, p)
		}
		return strings.Join(parts, " in ")
	case "WebSearch":
		if q, ok := input["query"].(string); ok {
			return q
		}
	case "WebFetch":
		if u, ok := input["url"].(string); ok {
			return u
		}
	case "Agent":
		if d, ok := input["description"].(string); ok {
			return d
		}
		if p, ok := input["prompt"].(string); ok {
			return truncateString(p, 100)
		}
	}

	// Generic fallback: show first string value
	for _, key := range []string{"query", "prompt", "description", "command", "file_path", "path", "pattern", "url"} {
		if v, ok := input[key].(string); ok {
			return truncateString(v, 200)
		}
	}
	return ""
}

// extractToolResultText extracts readable text from ToolResultBlock.Content.
func extractToolResultText(content interface{}) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
	// Content can be []interface{} of content blocks
	if arr, ok := content.([]interface{}); ok {
		var parts []string
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return fmt.Sprintf("%v", content)
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
