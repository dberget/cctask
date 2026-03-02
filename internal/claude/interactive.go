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

type terminalApp string

const (
	termTerminal  terminalApp = "Terminal"
	termITerm2    terminalApp = "iTerm2"
	termGhostty   terminalApp = "Ghostty"
	termAlacritty terminalApp = "Alacritty"
	termKitty     terminalApp = "kitty"
	termWezTerm   terminalApp = "WezTerm"
	termWarp      terminalApp = "Warp"
	termUnknown   terminalApp = ""
)

// ExecInteractive returns a tea.Cmd that hands off the terminal to an interactive claude process.
// Bubbletea suspends, gives terminal to claude, resumes on exit.
func ExecInteractive(projectRoot string, systemPrompt string) tea.Cmd {
	c := exec.Command("claude", "--append-system-prompt", systemPrompt)
	c.Dir = projectRoot
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ClaudeExitMsg{Err: err}
	})
}

// ExecContinue returns a tea.Cmd that hands off the terminal to claude,
// resuming a specific session if provided, or the most recent conversation otherwise.
func ExecContinue(projectRoot string, sessionID string) tea.Cmd {
	var c *exec.Cmd
	if sessionID != "" {
		c = exec.Command("claude", "--resume", sessionID)
	} else {
		c = exec.Command("claude", "--continue")
	}
	c.Dir = projectRoot
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ClaudeExitMsg{Err: err}
	})
}

// detectTerminalApp identifies the user's terminal emulator.
// Checks $TERM_PROGRAM first (most reliable), then kitty-specific env var,
// then falls back to osascript frontmost-app query.
func detectTerminalApp() terminalApp {
	// 1. Check $TERM_PROGRAM (set by most modern terminals)
	if tp := os.Getenv("TERM_PROGRAM"); tp != "" {
		switch strings.ToLower(tp) {
		case "apple_terminal":
			return termTerminal
		case "iterm.app":
			return termITerm2
		case "ghostty":
			return termGhostty
		case "alacritty":
			return termAlacritty
		case "wezterm":
			return termWezTerm
		case "warp":
			return termWarp
		}
	}

	// 2. Check kitty-specific env var (kitty sometimes sets TERM_PROGRAM=xterm-kitty)
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return termKitty
	}

	// 3. Fall back to osascript frontmost-app query (macOS only)
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		switch name {
		case "Terminal":
			return termTerminal
		case "iTerm2":
			return termITerm2
		case "Ghostty":
			return termGhostty
		case "Alacritty":
			return termAlacritty
		case "kitty":
			return termKitty
		case "WezTerm":
			return termWezTerm
		case "Warp":
			return termWarp
		}
	}

	return termUnknown
}

// findBinary checks if a terminal binary is available on PATH.
func findBinary(name string) (string, bool) {
	path, err := exec.LookPath(name)
	return path, err == nil
}

// spawnAppleScriptTerminal opens a new Terminal.app window via AppleScript.
func spawnAppleScriptTerminal(launchPath string) {
	script := fmt.Sprintf(
		`tell application "Terminal"
	activate
	do script "%s"
end tell`, launchPath)
	exec.Command("osascript", "-e", script).Run()
}

// spawnAppleScriptITerm2 opens a new iTerm2 window via its AppleScript API.
func spawnAppleScriptITerm2(launchPath string) {
	script := fmt.Sprintf(
		`tell application "iTerm2"
	activate
	create window with default profile
	tell current session of current window
		write text "%s"
	end tell
end tell`, launchPath)
	exec.Command("osascript", "-e", script).Run()
}

// spawnAppleScriptGeneric opens a new window via AppleScript do script for apps that support it.
func spawnAppleScriptGeneric(app string, launchPath string) {
	script := fmt.Sprintf(
		`tell application "%s"
	activate
	do script "%s"
end tell`, app, launchPath)
	exec.Command("osascript", "-e", script).Run()
}

// spawnGhosttyWindow opens a new window in the running Ghostty instance via AppleScript
// menu automation, then types the launch command. Ghostty lacks AppleScript `do script`
// support and `open -na` spawns a duplicate process, so this is the best available approach.
func spawnGhosttyWindow(launchPath string) {
	script := fmt.Sprintf(
		`tell application "Ghostty" to activate
delay 0.3
tell application "System Events"
	tell process "Ghostty"
		click menu item "New Window" of menu "File" of menu bar 1
		delay 0.5
		keystroke "%s"
		keystroke return
	end tell
end tell`, launchPath)
	exec.Command("osascript", "-e", script).Run()
}

// spawnCLI launches a terminal via its CLI binary with the given args.
// Runs in the background so it doesn't block the TUI.
func spawnCLI(binary string, args ...string) {
	cmd := exec.Command(binary, args...)
	if err := cmd.Start(); err != nil {
		return
	}
	go cmd.Wait()
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

		term := detectTerminalApp()
		launchPath := launchFile.Name()

		switch term {
		case termITerm2:
			spawnAppleScriptITerm2(launchPath)
		case termGhostty:
			spawnGhosttyWindow(launchPath)
		case termAlacritty:
			if bin, ok := findBinary("alacritty"); ok {
				spawnCLI(bin, "-e", "bash", launchPath)
			} else {
				spawnAppleScriptTerminal(launchPath)
			}
		case termKitty:
			if bin, ok := findBinary("kitty"); ok {
				spawnCLI(bin, "bash", launchPath)
			} else {
				spawnAppleScriptTerminal(launchPath)
			}
		case termWezTerm:
			if bin, ok := findBinary("wezterm"); ok {
				spawnCLI(bin, "start", "--", "bash", launchPath)
			} else {
				spawnAppleScriptTerminal(launchPath)
			}
		case termWarp:
			spawnAppleScriptGeneric("Warp", launchPath)
		case termTerminal:
			spawnAppleScriptTerminal(launchPath)
		default:
			// Unknown terminal — fall back to Terminal.app
			spawnAppleScriptTerminal(launchPath)
		}

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
