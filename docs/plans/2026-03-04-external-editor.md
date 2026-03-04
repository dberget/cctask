# External $EDITOR Support — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `V` keybinding that opens editable content in the user's `$EDITOR`, suspending the TUI and picking up changes on return.

**Architecture:** New `ExternalEditorMsg` message type + `openInEditor()` helper that resolves the editor, handles temp files for non-file-backed content, and uses `tea.ExecProcess` to suspend. A `V` key binding is added to `KeyBindings` and handled in `handleNavKey` for the relevant modes.

**Tech Stack:** Go, Bubble Tea (`tea.ExecProcess`), `os/exec`

---

### Task 1: Add `ExternalEditorExitMsg` and `openInEditor` helper

**Files:**
- Create: `internal/tui/external_editor.go`

**Step 1: Create the file with message type and helper function**

```go
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
	Err      error
	Content  string // read-back content (for temp file round-trips)
	TempFile string // non-empty if a temp file was used (for cleanup)
	// Context for applying changes on return
	Action       actionContext
	ActionTaskID string
	PlanFile     string
	ReturnMode   string // not used — we store returnMode on Model before exec
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
		tf := ""
		if tempFile {
			tf = filePath
		}
		return ExternalEditorExitMsg{
			Err:      err,
			Content:  content,
			TempFile: tf,
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
```

**Step 2: Verify it compiles**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: compiles with no errors (msg type unused is fine)

**Step 3: Commit**

```bash
git add internal/tui/external_editor.go
git commit -m "feat: add external editor helper for \$EDITOR support"
```

---

### Task 2: Add `OpenExtEditor` key binding

**Files:**
- Modify: `internal/tui/keys.go`

**Step 1: Add the binding to `KeyBindings` struct**

In `keys.go`, add to the struct after `EditorSave`:

```go
	// External editor
	OpenExtEditor key.Binding
```

**Step 2: Initialize it in `NewKeyBindings()`**

After the `EditorSave` line:

```go
		OpenExtEditor: key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "vim")),
```

**Step 3: Add to status bar hints for relevant modes**

In `modeShortHelp`, update these cases to include `keys.OpenExtEditor`:

- `model.ModePlan`: add before `keys.Back`
- `model.ModeTaskView`: add before `keys.Back`
- `model.ModeContextView`: add before `keys.Back`
- `model.ModeEditPlan, model.ModeEditContext, model.ModeBulkAdd`: add before `keys.Back`

**Step 4: Verify it compiles**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: compiles

**Step 5: Commit**

```bash
git add internal/tui/keys.go
git commit -m "feat: add V key binding for external editor"
```

---

### Task 3: Handle `V` key in `handleNavKey` — open external editor

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add `V` handling in `handleNavKey`**

After the `key.Matches(msg, m.keys.Edit)` block (~line 1201), add:

```go
	if key.Matches(msg, m.keys.OpenExtEditor) {
		return m.handleOpenExtEditor()
	}
```

**Step 2: Add `handleOpenExtEditor` method**

Add this method to `app.go`:

```go
func (m Model) handleOpenExtEditor() (tea.Model, tea.Cmd) {
	switch m.mode {
	case model.ModePlan:
		// Open plan file directly
		planFile := ""
		if t := m.selectedTask(); t != nil {
			planFile = t.PlanFile
		} else if g := m.selectedGroup(); g != nil {
			planFile = g.PlanFile
		}
		if planFile == "" {
			return m, flashCmd("No plan file to edit")
		}
		m.returnMode = model.ModePlan
		m.action = actionEditPlanContent
		m.actionPlanFile = planFile
		return m, openPlanInEditor(m.projectRoot, planFile)

	case model.ModeEditPlan:
		// Already editing a plan — open the same file externally
		if m.actionPlanFile == "" {
			return m, flashCmd("No plan file")
		}
		// Save current editor content first, then open externally
		store.SavePlan(m.projectRoot, m.actionPlanFile, m.editor.Content())
		return m, openPlanInEditor(m.projectRoot, m.actionPlanFile)

	case model.ModeContextView:
		m.returnMode = model.ModeContextView
		m.action = actionEditContext
		return m, openContextInEditor(m.projectRoot)

	case model.ModeEditContext:
		// Save current editor content first, then open externally
		store.SaveContext(m.projectRoot, m.editor.Content())
		return m, openContextInEditor(m.projectRoot)

	case model.ModeTaskView:
		if t := m.selectedTask(); t != nil {
			m.returnMode = model.ModeTaskView
			m.action = actionEditTask
			m.actionTaskID = t.ID
			return m, openContentInEditor(t.Description, "cctask-desc")
		}

	case model.ModeBulkAdd:
		// Open current bulk-add content in editor
		m.returnMode = model.ModeList
		m.action = actionBulkAdd
		return m, openContentInEditor(m.editor.Content(), "cctask-bulk")
	}

	return m, nil
}
```

**Step 3: Verify it compiles**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: compiles

**Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: handle V key to open content in external editor"
```

---

### Task 4: Handle `ExternalEditorExitMsg` — apply changes on return

**Files:**
- Modify: `internal/tui/app.go`

**Step 1: Add case in the `Update` switch**

In the `Update` method's type switch (around line 312 where `ClaudeExitMsg` is handled), add:

```go
	case ExternalEditorExitMsg:
		return m.handleExternalEditorExit(msg)
```

**Step 2: Add `handleExternalEditorExit` method**

```go
func (m Model) handleExternalEditorExit(msg ExternalEditorExitMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.mode = m.returnMode
		m.action = actionNone
		return m, flashCmd("Editor error: " + msg.Err.Error())
	}

	switch m.action {
	case actionEditPlanContent:
		// File-backed: just reload from disk
		m.reload()
		m.mode = m.returnMode
		m.action = actionNone
		m.actionPlanFile = ""
		return m, flashCmd("Plan updated")

	case actionEditContext:
		// File-backed: just reload
		m.mode = m.returnMode
		m.action = actionNone
		return m, flashCmd("Context updated")

	case actionEditTask:
		// Temp file round-trip: update task description
		if msg.Content != "" && m.actionTaskID != "" {
			for i, t := range m.store.Tasks {
				if t.ID == m.actionTaskID {
					m.store.Tasks[i].Description = msg.Content
					break
				}
			}
			store.SaveStore(m.projectRoot, m.store)
			m.reload()
		}
		m.mode = m.returnMode
		m.action = actionNone
		m.actionTaskID = ""
		return m, flashCmd("Description updated")

	case actionBulkAdd:
		// Temp file round-trip: pass content to bulk add flow
		if msg.Content != "" {
			return m.spawnBulkAdd(msg.Content)
		}
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("No content to process")
	}

	m.mode = m.returnMode
	m.action = actionNone
	return m, nil
}
```

**Step 3: Verify it compiles**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: compiles

**Step 4: Manual test**

Run: `cd /Users/davidberget/github/cctask-go && go install ./... && cp ~/go/bin/cctask-go /opt/homebrew/bin/cctask`

Test these flows:
1. Open a task with a plan → press `V` → vim opens plan file → save/quit → TUI resumes with "Plan updated"
2. Open context view (`x`) → press `V` → vim opens context.md → save/quit → "Context updated"
3. Open task view (Enter on task) → press `V` → vim opens temp file with description → edit → save/quit → "Description updated"
4. Bulk add (`A`) → type some text → press `V` → vim opens temp file → edit → save/quit → bulk add processes

**Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: handle external editor exit and apply changes"
```
