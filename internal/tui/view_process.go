package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/model"
)

// renderRichProcessDetail renders a structured view of streaming events.
// Falls back to legacy renderProcessDetail when no events are present.
func renderRichProcessDetail(proc *model.ClaudeProcess, width int) string {
	if len(proc.Events) == 0 {
		return renderProcessDetail(proc)
	}

	contentWidth := width - 2
	if contentWidth < 40 {
		contentWidth = 40
	}

	var lines []string

	// Header
	statusColor := processStatusColor(string(proc.Status))
	header := styleCyanBold.Render(proc.Label) + "  " +
		lipgloss.NewStyle().Foreground(statusColor).Render("["+string(proc.Status)+"]")
	if proc.SessionID != "" {
		header += "  " + styleDim.Render("session:"+proc.SessionID[:min(8, len(proc.SessionID))])
	}
	if proc.CostUSD > 0 {
		header += "  " + styleDim.Render(fmt.Sprintf("$%.4f", proc.CostUSD))
	}
	if elapsed := processElapsed(proc); elapsed != "" {
		header += "  " + styleDim.Render("⏱ " + elapsed)
	}
	lines = append(lines, header)
	lines = append(lines, horizontalLine(min(width, 60)))
	lines = append(lines, "")

	// Events
	for _, ev := range proc.Events {
		switch ev.Kind {
		case model.EventThinking:
			// Show thinking as a compact dim block, like Claude Code's collapsible thinking
			thinkingLines := strings.Split(ev.Text, "\n")
			preview := thinkingLines[0]
			if len(preview) > contentWidth-4 {
				preview = preview[:contentWidth-7] + "..."
			}
			label := styleDim.Render("💭 ") + styleGray.Render(preview)
			if len(thinkingLines) > 1 {
				label += styleDim.Render(fmt.Sprintf(" (+%d lines)", len(thinkingLines)-1))
			}
			lines = append(lines, label)

		case model.EventText:
			lines = append(lines, renderMarkdown(ev.Text, contentWidth))

		case model.EventToolUse:
			lines = append(lines, renderToolUse(ev, contentWidth))

		case model.EventToolResult:
			lines = append(lines, renderToolResult(ev, contentWidth))

		case model.EventUserMsg:
			lines = append(lines, styleMagenta.Render("You: ")+wrapText(ev.Text, contentWidth-5))
			lines = append(lines, "")

		case model.EventToolQuestion:
			lines = append(lines, styleYellow.Render("? "+ev.Text))
			lines = append(lines, "")

		case model.EventSystem:
			lines = append(lines, styleDim.Render("• "+ev.Text))
		}
	}

	// Queued message indicator
	if proc.QueuedMessage != "" {
		lines = append(lines, "")
		lines = append(lines, styleYellow.Render("📨 Queued: ")+styleDim.Render(truncate(proc.QueuedMessage, width-12)))
	}

	// Footer hints
	lines = append(lines, "")
	var hints []string
	if proc.Status == model.ProcessRunning {
		hints = append(hints, keyHint("x", "interrupt"))
		hints = append(hints, keyHint("c", "queue message"))
	} else {
		hints = append(hints, keyHint("x", "remove"))
	}
	if proc.Status == model.ProcessWaiting && proc.SessionID != "" {
		hints = append(hints, keyHint("c", "follow-up"))
	}
	if proc.Status != model.ProcessRunning {
		hints = append(hints, keyHint("o", "full claude"))
	}
	hints = append(hints, keyHint("Esc", "back"))
	lines = append(lines, strings.Join(hints, "  "))

	return strings.Join(lines, "\n")
}

// renderToolUse formats a tool use event similar to Claude Code's display.
// Shows: "⚡ ToolName  param" on one line.
func renderToolUse(ev model.StreamEvent, width int) string {
	name := ev.ToolName
	var display string

	switch name {
	case "Read":
		display = styleYellow.Render("📄 Read ") + styleDim.Render(truncate(ev.ToolInput, width-10))
	case "Write":
		display = styleYellow.Render("📝 Write ") + styleDim.Render(truncate(ev.ToolInput, width-11))
	case "Edit":
		display = styleYellow.Render("✏️  Edit ") + styleDim.Render(truncate(ev.ToolInput, width-11))
	case "Bash":
		display = styleYellow.Render("$ ") + styleDim.Render(truncate(ev.ToolInput, width-4))
	case "Glob":
		display = styleYellow.Render("🔍 Glob ") + styleDim.Render(truncate(ev.ToolInput, width-10))
	case "Grep":
		display = styleYellow.Render("🔍 Grep ") + styleDim.Render(truncate(ev.ToolInput, width-10))
	case "WebSearch":
		display = styleYellow.Render("🌐 Search ") + styleDim.Render(truncate(ev.ToolInput, width-12))
	case "WebFetch":
		display = styleYellow.Render("🌐 Fetch ") + styleDim.Render(truncate(ev.ToolInput, width-11))
	case "Agent":
		display = styleYellow.Render("🤖 Agent ") + styleDim.Render(truncate(ev.ToolInput, width-11))
	default:
		display = styleYellow.Render("⚡ "+name+" ")
		if ev.ToolInput != "" {
			display += styleDim.Render(truncate(ev.ToolInput, width-len(name)-4))
		}
	}
	return display
}

// renderToolResult formats a tool result as a compact success/failure indicator.
func renderToolResult(ev model.StreamEvent, width int) string {
	if ev.IsError {
		errText := truncate(ev.ToolResult, width-4)
		return styleRed.Render("  ✗ ") + styleRed.Render(truncateLines(errText, 2)) + "\n"
	}
	// For successful results, show a minimal indicator — the details are usually
	// less important than the tool invocation itself.
	if ev.ToolResult == "" {
		return styleDim.Render("  ✓") + "\n"
	}
	preview := firstLine(ev.ToolResult)
	if preview != ev.ToolResult {
		preview += "..."
	}
	return styleDim.Render("  ✓ "+truncate(preview, width-6)) + "\n"
}

// firstLine returns the first non-empty line of text.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return s
}
