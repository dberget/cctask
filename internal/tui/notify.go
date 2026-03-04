package tui

import (
	"os/exec"
	"runtime"
)

// sendMacNotification sends a macOS notification.
// Prefers terminal-notifier if available, falls back to osascript.
// Silently does nothing on non-macOS platforms or if the command fails.
func sendMacNotification(title, message string) {
	if runtime.GOOS != "darwin" {
		return
	}
	// Prefer terminal-notifier (works regardless of terminal app)
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		exec.Command(path, "-title", title, "-message", message, "-sound", "default").Run() //nolint:errcheck
		return
	}
	// Fallback to osascript
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `" sound name "default"`
	exec.Command("/usr/bin/osascript", "-e", script).Run() //nolint:errcheck
}

func escapeAppleScript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
