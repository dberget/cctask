package tui

import (
	"os/exec"
	"runtime"
)

// sendMacNotification sends a macOS notification via osascript.
// Silently does nothing on non-macOS platforms or if osascript fails.
func sendMacNotification(title, message string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := `display notification "` + escapeAppleScript(message) + `" with title "` + escapeAppleScript(title) + `"`
	exec.Command("osascript", "-e", script).Run() //nolint:errcheck
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
