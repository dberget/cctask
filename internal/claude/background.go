package claude

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ProcessOutputMsg is sent when a background claude process produces output.
type ProcessOutputMsg struct {
	ID      string
	Output  string
	LogFile string
}

// ProcessDoneMsg is sent when a background claude process completes.
type ProcessDoneMsg struct {
	ID      string
	Output  string
	LogFile string
	Err     error
}

// SpawnPlanCmd creates a tea.Cmd that runs claude -p in the background,
// streaming output via ProcessOutputMsg and signaling completion via ProcessDoneMsg.
func SpawnPlanCmd(p *tea.Program, projectRoot string, procID string, procLabel string, prompt string) tea.Cmd {
	return func() tea.Msg {
		logsDir := filepath.Join(projectRoot, ".cctask", "logs")
		os.MkdirAll(logsDir, 0o755)
		logFile := filepath.Join(logsDir, procID+".log")

		f, err := os.Create(logFile)
		if err != nil {
			return ProcessDoneMsg{ID: procID, Err: fmt.Errorf("failed to create log: %w", err)}
		}
		fmt.Fprintf(f, "[%s] Started at %s\n\n", procLabel, time.Now().Format("15:04:05"))

		c := exec.Command("claude", "-p")
		c.Dir = projectRoot
		c.Stdin = strings.NewReader(prompt)

		stdout, err := c.StdoutPipe()
		if err != nil {
			f.Close()
			return ProcessDoneMsg{ID: procID, Err: err}
		}
		stderr, err := c.StderrPipe()
		if err != nil {
			f.Close()
			return ProcessDoneMsg{ID: procID, Err: err}
		}

		if err := c.Start(); err != nil {
			f.Close()
			return ProcessDoneMsg{ID: procID, Err: err}
		}

		combined := io.MultiReader(stdout, stderr)
		var fullOutput string
		buf := make([]byte, 4096)
		for {
			n, readErr := combined.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput += chunk
				f.WriteString(chunk)
				p.Send(ProcessOutputMsg{ID: procID, Output: fullOutput, LogFile: logFile})
			}
			if readErr != nil {
				break
			}
		}

		waitErr := c.Wait()

		if waitErr != nil {
			fmt.Fprintf(f, "\n\n[error] Finished at %s\n", time.Now().Format("15:04:05"))
		} else {
			fmt.Fprintf(f, "\n\n[done] Finished at %s\n", time.Now().Format("15:04:05"))
		}
		f.Close()

		if waitErr != nil {
			return ProcessDoneMsg{ID: procID, Output: fullOutput, LogFile: logFile, Err: waitErr}
		}
		return ProcessDoneMsg{ID: procID, Output: fullOutput, LogFile: logFile}
	}
}
