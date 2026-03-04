package tui

import (
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/store"
)

// ExternalEditorExitMsg is sent when the external editor process exits.
type ExternalEditorExitMsg struct {
	Err     error
	Content string // read-back content (for temp file round-trips)
}

// resolveEditor returns the user's preferred editor command.
func resolveEditor() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	return "vim"
}

// openFileInEditor returns a tea.Cmd that suspends the TUI and opens the
// given file path in $EDITOR. On exit it reads the file content back and
// sends ExternalEditorExitMsg.
func openFileInEditor(filePath string, tempFile bool) tea.Cmd {
	editor := resolveEditor()
	c := exec.Command(editor, filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		var content string
		if tempFile {
			data, _ := os.ReadFile(filePath)
			content = string(data)
			os.Remove(filePath)
		}
		return ExternalEditorExitMsg{
			Err:     err,
			Content: content,
		}
	})
}

// openPlanInEditor opens a plan file (already on disk) in $EDITOR.
func openPlanInEditor(projectRoot, planFile string) tea.Cmd {
	path := filepath.Join(store.PlansDir(projectRoot), planFile)
	return openFileInEditor(path, false)
}

// openContextInEditor opens the project context file in $EDITOR.
func openContextInEditor(projectRoot string) tea.Cmd {
	path := store.ContextPath(projectRoot)
	// Ensure the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(""), 0o644)
	}
	return openFileInEditor(path, false)
}

// openContentInEditor writes content to a temp file, opens it in $EDITOR,
// and reads it back on exit.
func openContentInEditor(content, prefix string) tea.Cmd {
	f, err := os.CreateTemp("", prefix+"-*.md")
	if err != nil {
		return func() tea.Msg {
			return ExternalEditorExitMsg{Err: err}
		}
	}
	f.WriteString(content)
	f.Close()
	return openFileInEditor(f.Name(), true)
}
