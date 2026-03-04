package claude

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/model"
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

// CancelAll cancels every registered process.
func (pc *ProcessCancels) CancelAll() {
	pc.m.Range(func(key, value any) bool {
		value.(context.CancelFunc)()
		pc.m.Delete(key)
		return true
	})
}

// HasRunning returns true if any processes are registered.
func (pc *ProcessCancels) HasRunning() bool {
	running := false
	pc.m.Range(func(_, _ any) bool {
		running = true
		return false // stop iteration
	})
	return running
}

// ProcessInputs is a registry of input channels for keep-alive interactive processes.
type ProcessInputs struct {
	m sync.Map
}

// Register stores an input channel for the given process ID.
func (pi *ProcessInputs) Register(procID string, ch chan string) {
	pi.m.Store(procID, ch)
}

// Send sends a message to the process's input channel.
// Returns true if the channel was found and the message was sent.
func (pi *ProcessInputs) Send(procID string, message string) bool {
	if v, ok := pi.m.Load(procID); ok {
		ch := v.(chan string)
		ch <- message
		return true
	}
	return false
}

// Close closes the input channel and removes it from the registry.
func (pi *ProcessInputs) Close(procID string) {
	if v, ok := pi.m.LoadAndDelete(procID); ok {
		close(v.(chan string))
	}
}

// Remove removes a process from the registry without closing the channel.
func (pi *ProcessInputs) Remove(procID string) {
	pi.m.Delete(procID)
}

// Has returns true if the process has a registered input channel.
func (pi *ProcessInputs) Has(procID string) bool {
	_, ok := pi.m.Load(procID)
	return ok
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

// StreamWaitingMsg signals that a streaming process has completed a turn
// but the subprocess is still alive and waiting for user input.
type StreamWaitingMsg struct {
	ProcessID string
	SessionID string
	TurnCount int
	CostUSD   float64
	FinalText string
}

// ChatSubmitMsg is sent when the user submits a follow-up message in embedded chat.
type ChatSubmitMsg struct {
	ProcessID string
	SessionID string
	Message   string
}

// SpawnStreamCmd creates a tea.Cmd that runs a Claude query via the CLI,
// streaming structured events via p.Send() and returning StreamDoneMsg on completion.
// inputCh is optional — pass nil for non-interactive (single-turn) processes,
// or a channel for interactive (multi-turn keep-alive) processes.
func SpawnStreamCmd(p *tea.Program, cwd string, procID string, prompt string, cancels *ProcessCancels, timeout time.Duration, opts CLIOptions, inputCh <-chan string) tea.Cmd {
	return func() tea.Msg {
		return runStream(p, cwd, procID, prompt, cancels, timeout, opts, inputCh)
	}
}

// SpawnStreamResumeCmd creates a tea.Cmd that resumes an existing Claude session.
// Resume always creates a new single-turn process (no keep-alive).
func SpawnStreamResumeCmd(p *tea.Program, cwd string, procID string, sessionID string, prompt string, cancels *ProcessCancels, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		opts := CLIOptions{Resume: sessionID}
		return runStream(p, cwd, procID, prompt, cancels, timeout, opts, nil)
	}
}

func runStream(p *tea.Program, cwd string, procID string, prompt string, cancels *ProcessCancels, timeout time.Duration, opts CLIOptions, inputCh <-chan string) tea.Msg {
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

	args := buildCLIArgs(opts)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cwd
	// Build a clean environment: inherit parent env but override
	// CLAUDE_CODE_ENTRYPOINT (to bill against subscription, not API credits)
	// and remove CLAUDECODE (to avoid "nested session" detection).
	cmd.Env = buildSubprocessEnv()

	// Capture stderr for error diagnostics
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Set up stdin pipe for interactive streaming mode
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return StreamDoneMsg{ProcessID: procID, Err: fmt.Errorf("stdin pipe: %w", err)}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return StreamDoneMsg{ProcessID: procID, Err: fmt.Errorf("stdout pipe: %w", err)}
	}

	if err := cmd.Start(); err != nil {
		return StreamDoneMsg{ProcessID: procID, Err: fmt.Errorf("start: %w", err)}
	}

	// Send the prompt as a JSON message on stdin
	msgData, err := formatStdinMessage(prompt)
	if err != nil {
		stdinPipe.Close()
		return StreamDoneMsg{ProcessID: procID, Err: fmt.Errorf("format message: %w", err)}
	}
	if _, err := stdinPipe.Write(msgData); err != nil {
		stdinPipe.Close()
		return StreamDoneMsg{ProcessID: procID, Err: fmt.Errorf("write stdin: %w", err)}
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer for large tool results

outerLoop:
	for {
		// Inner loop: scan JSONL events until a result message
		gotResult := false
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			events, result, sessionID, parseErr := parseJSONLine(line)
			if parseErr != nil {
				continue // skip unparseable lines
			}

			if sessionID != "" {
				finalSessionID = sessionID
			}

			for _, ev := range events {
				if ev.Kind == model.EventToolUse {
					// Reset text buffer on each tool use so FinalText
					// only contains output after the last tool call.
					allText.Reset()
				}
				if ev.Kind == model.EventText {
					allText.WriteString(ev.Text)
				}
				p.Send(StreamEventMsg{
					ProcessID: procID,
					Event:     ev,
					SessionID: finalSessionID,
				})
			}

			if result != nil {
				finalSessionID = result.SessionID
				turnCount = result.NumTurns
				costUSD = result.CostUSD
				gotResult = true
				break // break inner scan loop
			}
		}

		// If non-interactive or scanner hit EOF without result, we're done
		if !gotResult || inputCh == nil {
			break outerLoop
		}

		// Interactive: notify TUI that we're waiting for user input
		p.Send(StreamWaitingMsg{
			ProcessID: procID,
			SessionID: finalSessionID,
			TurnCount: turnCount,
			CostUSD:   costUSD,
			FinalText: allText.String(),
		})

		// Block until user sends input or context is cancelled
		select {
		case msg, ok := <-inputCh:
			if !ok {
				// Channel closed — user ended the conversation
				break outerLoop
			}
			// Reset text buffer for the new turn
			allText.Reset()

			// Write the follow-up message to stdin
			followUp, fmtErr := formatStdinMessage(msg)
			if fmtErr != nil {
				break outerLoop
			}
			if _, writeErr := stdinPipe.Write(followUp); writeErr != nil {
				break outerLoop
			}

			// Add user message event to the TUI
			p.Send(StreamEventMsg{
				ProcessID: procID,
				Event: model.StreamEvent{
					Kind:      model.EventUserMsg,
					Text:      msg,
					Timestamp: time.Now(),
				},
				SessionID: finalSessionID,
			})
			// Continue outer loop to scan the next turn's output
			continue outerLoop

		case <-ctx.Done():
			break outerLoop
		}
	}

	// Close stdin to signal we're done — the process will exit
	stdinPipe.Close()

	// Wait for the process to exit
	waitErr := cmd.Wait()
	if waitErr != nil {
		if ctx.Err() != nil {
			// Context was cancelled — report the context error
			return StreamDoneMsg{
				ProcessID: procID,
				SessionID: finalSessionID,
				Err:       ctx.Err(),
				TurnCount: turnCount,
				CostUSD:   costUSD,
				FinalText: allText.String(),
			}
		}
		// Process failed on its own — include stderr in error
		errMsg := stderrBuf.String()
		if errMsg == "" {
			errMsg = waitErr.Error()
		}
		return StreamDoneMsg{
			ProcessID: procID,
			SessionID: finalSessionID,
			Err:       fmt.Errorf("claude: %s", strings.TrimSpace(errMsg)),
			TurnCount: turnCount,
			CostUSD:   costUSD,
			FinalText: allText.String(),
		}
	}

	return StreamDoneMsg{
		ProcessID: procID,
		SessionID: finalSessionID,
		TurnCount: turnCount,
		CostUSD:   costUSD,
		FinalText: allText.String(),
	}
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

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}

// buildSubprocessEnv returns environment variables for the Claude subprocess.
// It inherits the parent environment but:
//   - Removes ANTHROPIC_API_KEY so the CLI uses the subscription, not API credits
//   - Removes CLAUDECODE to avoid "nested session" detection
//   - Overrides CLAUDE_CODE_ENTRYPOINT for SDK identification
func buildSubprocessEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDECODE=") ||
			strings.HasPrefix(e, "CLAUDE_CODE_ENTRYPOINT=") ||
			strings.HasPrefix(e, "ANTHROPIC_API_KEY=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go-client")
	return env
}
