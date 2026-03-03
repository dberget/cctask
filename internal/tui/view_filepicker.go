package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/model"
)

// filePickerResultMsg is sent when a file is selected from the file picker.
type filePickerResultMsg struct {
	path string
}

func newFilePicker(projectRoot string) filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".md", ".txt", ".json"}
	fp.CurrentDirectory = projectRoot
	fp.ShowHidden = false
	fp.Height = 15
	return fp
}

func (m Model) handleFilePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEscape {
		m.mode = model.ModeContextView
		return m, nil
	}

	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)

	// Check if a file was selected
	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		return m, func() tea.Msg { return filePickerResultMsg{path: path} }
	}

	return m, cmd
}

func renderFilePickerView(fp filepicker.Model) string {
	return styleCyanBold.Render("Import File") + "  " + styleGray.Render("Enter: select  Esc: cancel") +
		"\n\n" + fp.View()
}

// dirPickerResultMsg is sent when a directory is selected from the dir picker.
type dirPickerResultMsg struct {
	path string
}

func newDirPicker(startDir string) filepicker.Model {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.CurrentDirectory = startDir
	fp.ShowHidden = false
	fp.Height = 15
	return fp
}

func (m Model) handleDirPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEscape {
		m.mode = model.ModeTaskForm
		return m, nil
	}

	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)

	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		return m, func() tea.Msg { return dirPickerResultMsg{path: path} }
	}

	return m, cmd
}

func renderDirPickerView(fp filepicker.Model) string {
	return styleCyanBold.Render("Select Directory") + "  " + styleGray.Render("Enter: select  Esc: cancel") +
		"\n\n" + fp.View()
}

// readFileContent reads a file and returns its content, or an error message.
func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
