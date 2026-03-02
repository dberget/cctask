package claude

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ClaudeExitMsg is sent when interactive claude exits.
type ClaudeExitMsg struct {
	Err error
}

// ExecInteractive returns a tea.Cmd that hands off the terminal to an interactive claude process.
// Bubbletea suspends, gives terminal to claude, resumes on exit.
func ExecInteractive(projectRoot string, systemPrompt string) tea.Cmd {
	c := exec.Command("claude", "--append-system-prompt", systemPrompt)
	c.Dir = projectRoot
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ClaudeExitMsg{Err: err}
	})
}

// ExecContinue returns a tea.Cmd that hands off the terminal to claude --continue,
// resuming the most recent conversation in the project directory.
func ExecContinue(projectRoot string) tea.Cmd {
	c := exec.Command("claude", "--continue")
	c.Dir = projectRoot
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ClaudeExitMsg{Err: err}
	})
}

// detectTerminalApp returns the name of the frontmost terminal application.
// Falls back to "Terminal" if detection fails.
func detectTerminalApp() string {
	// Ask macOS which app is frontmost — since cctask is running in a terminal,
	// the frontmost app is the user's terminal emulator.
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		// Validate it's a known terminal app
		switch name {
		case "iTerm2", "Terminal", "Alacritty", "kitty", "Hyper", "WezTerm", "Warp":
			return name
		}
	}
	return "Terminal"
}

// SpawnInTerminal opens a new window in the user's active terminal app with an
// interactive claude session. Uses a launcher script to avoid shell/AppleScript
// escaping issues. This does not block the TUI.
func SpawnInTerminal(projectRoot string, systemPrompt string) tea.Cmd {
	return func() tea.Msg {
		// Write prompt to temp file
		promptFile, err := os.CreateTemp("", "cctask-prompt-*.txt")
		if err != nil {
			return nil
		}
		promptFile.WriteString(systemPrompt)
		promptFile.Close()

		// Write a launcher script — avoids all AppleScript escaping issues
		launchFile, err := os.CreateTemp("", "cctask-launch-*.sh")
		if err != nil {
			os.Remove(promptFile.Name())
			return nil
		}
		fmt.Fprintf(launchFile, "#!/bin/bash\ncd %q\nclaude --append-system-prompt \"$(cat %q)\"\nrm -f %q %q\n",
			projectRoot, promptFile.Name(), promptFile.Name(), launchFile.Name())
		launchFile.Close()
		os.Chmod(launchFile.Name(), 0o755)

		termApp := detectTerminalApp()
		launchPath := launchFile.Name()

		var script string
		switch termApp {
		case "iTerm2":
			script = fmt.Sprintf(
				`tell application "iTerm2"
	activate
	create window with default profile
	tell current session of current window
		write text "%s"
	end tell
end tell`, launchPath)
		default:
			script = fmt.Sprintf(
				`tell application "%s"
	activate
	do script "%s"
end tell`, termApp, launchPath)
		}

		exec.Command("osascript", "-e", script).Run()
		return nil
	}
}

// RunInteractive runs claude with stdio inherited (for non-TUI CLI path).
func RunInteractive(projectRoot string, systemPrompt string) error {
	c := exec.Command("claude", "--append-system-prompt", systemPrompt)
	c.Dir = projectRoot
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
