package claude

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/davidberget/cctask-go/internal/model"
)

// PlanModeAllowedTools are read/research tools granted to plan-mode background processes
// so they can fetch web content and search without prompting for permission.
var PlanModeAllowedTools = []string{
	"Read", "Glob", "Grep",
	"WebFetch", "WebSearch",
	"Agent", "Skill",
	"Bash(git log:*)", "Bash(git diff:*)", "Bash(git show:*)", "Bash(git status:*)", "Bash(git branch:*)",
	"Bash(ls:*)", "Bash(find:*)", "Bash(wc:*)", "Bash(file:*)",
	"Bash(tree:*)", "Bash(cat:*)", "Bash(head:*)", "Bash(tail:*)",
	"Bash(which:*)", "Bash(env:*)", "Bash(pwd:*)",
	"Bash(go doc:*)", "Bash(go list:*)", "Bash(go version:*)",
}

// CLIOptions configures the Claude CLI subprocess.
type CLIOptions struct {
	PermissionMode     string
	Resume             string
	Model              string
	AppendSystemPrompt string
	AllowedTools       []string
	DisallowedTools    []string
}

// cliMessage represents a single JSONL line from `claude -p --output-format stream-json`.
type cliMessage struct {
	Type    string          `json:"type"`    // "system", "assistant", "user", "result"
	Subtype string          `json:"subtype"` // for system/result: "init", "success", "error"
	Message json.RawMessage `json:"message"` // for assistant/user messages

	// Result fields (only when type == "result")
	SessionID    string   `json:"session_id"`
	TotalCostUSD *float64 `json:"total_cost_usd"`
	NumTurns     int      `json:"num_turns"`
	Result       *string  `json:"result"`
	IsError      bool     `json:"is_error"`
}

// cliContentBlock represents a content block within a message.
type cliContentBlock struct {
	Type      string         `json:"type"` // "text", "thinking", "tool_use", "tool_result"
	Text      string         `json:"text,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	ID        string         `json:"id,omitempty"`          // tool_use block ID
	Name      string         `json:"name,omitempty"`        // tool name
	Input     map[string]any `json:"input,omitempty"`       // tool input
	ToolUseID string         `json:"tool_use_id,omitempty"` // tool_result reference
	Content   any            `json:"content,omitempty"`     // tool_result content (string or []blocks)
	IsErrorP  *bool          `json:"is_error,omitempty"`    // tool_result error flag
}

// cliInnerMessage is the message body within assistant/user lines.
type cliInnerMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// resultInfo carries metadata extracted from a result line.
type resultInfo struct {
	SessionID string
	NumTurns  int
	CostUSD   float64
	IsError   bool
	ErrorText string
}

// buildCLIArgs constructs the CLI argument list for claude.
// Uses --input-format stream-json (interactive streaming mode) which runs under
// the user's Claude Code subscription, unlike --print which uses API credits.
// The prompt is sent as a JSON message on stdin.
func buildCLIArgs(opts CLIOptions) []string {
	args := []string{
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
	}
	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", opts.PermissionMode)
	}
	if opts.Resume != "" {
		args = append(args, "--resume", opts.Resume)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.AppendSystemPrompt)
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(opts.AllowedTools, ","))
	}
	if len(opts.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(opts.DisallowedTools, ","))
	}
	return args
}

// formatStdinMessage builds the JSON message to send on stdin for interactive streaming mode.
func formatStdinMessage(prompt string) ([]byte, error) {
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": prompt},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// parseJSONLine converts one JSONL line from the CLI into StreamEvents.
// Returns events, an optional resultInfo for result lines, a session_id if
// found in any message, and any parse error.
func parseJSONLine(line []byte) ([]model.StreamEvent, *resultInfo, string, error) {
	var msg cliMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, nil, "", err
	}

	now := time.Now()
	var events []model.StreamEvent
	sessionID := msg.SessionID // present on system init and result lines

	switch msg.Type {
	case "assistant":
		blocks := parseContentBlocks(msg.Message)
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					events = append(events, model.StreamEvent{
						Kind:      model.EventText,
						Text:      b.Text,
						Timestamp: now,
					})
				}
			case "thinking":
				if b.Thinking != "" {
					events = append(events, model.StreamEvent{
						Kind:      model.EventThinking,
						Text:      b.Thinking,
						Timestamp: now,
					})
				}
			case "tool_use":
				events = append(events, model.StreamEvent{
					Kind:      model.EventToolUse,
					ToolName:  b.Name,
					ToolID:    b.ID,
					ToolInput: formatToolInput(b.Name, b.Input),
					Timestamp: now,
				})
			case "tool_result":
				resultText := extractContentText(b.Content)
				isErr := b.IsErrorP != nil && *b.IsErrorP
				events = append(events, model.StreamEvent{
					Kind:       model.EventToolResult,
					ToolID:     b.ToolUseID,
					ToolResult: truncateString(resultText, 500),
					IsError:    isErr,
					Timestamp:  now,
				})
			}
		}

	case "user":
		blocks := parseContentBlocks(msg.Message)
		for _, b := range blocks {
			switch b.Type {
			case "text":
				if b.Text != "" {
					events = append(events, model.StreamEvent{
						Kind:      model.EventUserMsg,
						Text:      b.Text,
						Timestamp: now,
					})
				}
			case "tool_result":
				resultText := extractContentText(b.Content)
				isErr := b.IsErrorP != nil && *b.IsErrorP
				events = append(events, model.StreamEvent{
					Kind:       model.EventToolResult,
					ToolID:     b.ToolUseID,
					ToolResult: truncateString(resultText, 500),
					IsError:    isErr,
					Timestamp:  now,
				})
			}
		}

	case "system":
		if msg.Subtype != "" {
			events = append(events, model.StreamEvent{
				Kind:      model.EventSystem,
				Text:      msg.Subtype,
				Timestamp: now,
			})
		}

	case "result":
		ri := &resultInfo{
			SessionID: msg.SessionID,
			NumTurns:  msg.NumTurns,
			IsError:   msg.IsError,
		}
		if msg.TotalCostUSD != nil {
			ri.CostUSD = *msg.TotalCostUSD
		}
		if msg.IsError && msg.Result != nil {
			ri.ErrorText = *msg.Result
			events = append(events, model.StreamEvent{
				Kind:      model.EventSystem,
				Text:      *msg.Result,
				IsError:   true,
				Timestamp: now,
			})
		}
		return events, ri, sessionID, nil
	}

	return events, nil, sessionID, nil
}

// parseContentBlocks extracts content blocks from a message's raw JSON.
func parseContentBlocks(raw json.RawMessage) []cliContentBlock {
	if raw == nil {
		return nil
	}
	var inner cliInnerMessage
	if err := json.Unmarshal(raw, &inner); err != nil {
		return nil
	}
	// Try parsing content as array of blocks
	var blocks []cliContentBlock
	if err := json.Unmarshal(inner.Content, &blocks); err != nil {
		// Content might be a plain string
		var text string
		if err := json.Unmarshal(inner.Content, &text); err == nil && text != "" {
			return []cliContentBlock{{Type: "text", Text: text}}
		}
		return nil
	}
	return blocks
}

// extractContentText extracts readable text from a tool_result content field.
func extractContentText(content any) string {
	if content == nil {
		return ""
	}
	if s, ok := content.(string); ok {
		return s
	}
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
	return ""
}
