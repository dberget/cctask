package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/davidberget/cctask-go/internal/model"
)

// KeyBindings contains all key bindings for the TUI, satisfying help.KeyMap.
type KeyBindings struct {
	// Navigation
	Up   key.Binding
	Down key.Binding
	Tab  key.Binding

	// Scroll (fullscreen views)
	ScrollDown     key.Binding
	ScrollUp       key.Binding
	HalfPageDown   key.Binding
	HalfPageUp     key.Binding
	GotoTop        key.Binding
	GotoBottom     key.Binding

	// List actions
	Add       key.Binding
	Edit      key.Binding
	Delete    key.Binding
	CycleStatus key.Binding
	AssignGroup key.Binding
	Run       key.Binding
	Plan      key.Binding
	Prompt    key.Binding
	Merge     key.Binding
	Filter    key.Binding
	HideDone  key.Binding
	Theme     key.Binding
	Context   key.Binding
	Collapse  key.Binding
	Enter     key.Binding
	View      key.Binding
	TableView key.Binding
	ListView  key.Binding
	FilePick  key.Binding
	BulkAdd   key.Binding

	// Process actions
	Cancel    key.Binding
	Chat      key.Binding
	OpenFull  key.Binding
	PrevPage  key.Binding
	NextPage  key.Binding

	// Proof
	OpenProof key.Binding

	// Editor
	EditorSave key.Binding

	// External editor
	OpenExtEditor key.Binding

	// Form
	FormSave   key.Binding
	FormNext   key.Binding
	FormPrev   key.Binding

	// Global
	Help key.Binding
	Quit key.Binding
	Back key.Binding
}

func NewKeyBindings() KeyBindings {
	return KeyBindings{
		Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("j/k", "navigate")),
		Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("", "")),
		Tab:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "switch panel")),

		ScrollDown:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/k", "scroll")),
		ScrollUp:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("", "")),
		HalfPageDown: key.NewBinding(key.WithKeys("d", "ctrl+d"), key.WithHelp("d/u", "half-page")),
		HalfPageUp:   key.NewBinding(key.WithKeys("u", "ctrl+u"), key.WithHelp("", "")),
		GotoTop:      key.NewBinding(key.WithKeys("g"), key.WithHelp("gg/G", "top/bottom")),
		GotoBottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("", "")),

		Add:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add task")),
		Edit:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		CycleStatus: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "cycle status")),
		AssignGroup: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "project")),
		Run:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run")),
		Plan:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "plan")),
		Prompt:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "prompt")),
		Merge:       key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "merge plans")),
		Filter:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		HideDone:    key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "hide done")),
		Theme:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),
		Context:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "context")),
		Collapse:    key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "collapse")),
		Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "open")),
		View:        key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		TableView:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "table view")),
		ListView:    key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "list view")),
		FilePick:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "import file")),
		BulkAdd:     key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "bulk add")),

		Cancel:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancel")),
		Chat:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "chat")),
		OpenFull:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "full claude")),
		PrevPage:  key.NewBinding(key.WithKeys("["), key.WithHelp("[/]", "page")),
		NextPage:  key.NewBinding(key.WithKeys("]"), key.WithHelp("", "")),
		OpenProof: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open proof")),

		EditorSave: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("Ctrl+S", "save")),
		OpenExtEditor: key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "vim")),

		FormSave: key.NewBinding(key.WithKeys("ctrl+s", "ctrl+d"), key.WithHelp("Ctrl+S", "save")),
		FormNext: key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next field")),
		FormPrev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-Tab", "prev field")),

		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit: key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")),
	}
}

// ShortHelp returns context-sensitive bindings for the status bar (1 row).
func (k KeyBindings) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Add, k.Edit, k.Delete, k.Run, k.Plan,
		k.Filter, k.Help, k.Quit,
	}
}

// FullHelp returns all bindings organized into columns for the help view.
func (k KeyBindings) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Enter, k.View, k.Tab, k.Collapse},
		{k.Add, k.BulkAdd, k.Edit, k.Delete, k.CycleStatus, k.AssignGroup},
		{k.Run, k.Plan, k.Prompt, k.Merge, k.Filter},
		{k.HideDone, k.Theme, k.Context, k.TableView, k.ListView},
		{k.Help, k.Quit, k.Back},
	}
}

// modeShortHelp returns the context-sensitive key hints for the status bar.
func modeShortHelp(keys KeyBindings, mode model.ViewMode, selected *model.ListItem) []key.Binding {
	switch mode {
	case model.ModeList:
		return listModeBindings(keys, selected)
	case model.ModeDetail:
		return []key.Binding{keys.Edit, keys.Run, keys.Plan, keys.CycleStatus, keys.Back}
	case model.ModePlan:
		return []key.Binding{keys.Run, keys.Edit, keys.Plan, keys.OpenExtEditor, keys.Back}
	case model.ModeGroupDetail:
		return []key.Binding{keys.Run, keys.Plan, keys.Delete, keys.Back}
	case model.ModeTaskForm:
		return []key.Binding{keys.FormNext, keys.Enter, keys.FormSave, keys.Back}
	case model.ModeCombineSelect:
		return []key.Binding{keys.Collapse, keys.Enter, keys.Back}
	case model.ModeProcessDetail:
		return []key.Binding{keys.Cancel, keys.Chat, keys.OpenFull, keys.Back}
	case model.ModeProcessChat:
		return []key.Binding{keys.Enter, keys.Back}
	case model.ModeEditPlan, model.ModeEditContext, model.ModeBulkAdd:
		return []key.Binding{keys.EditorSave, keys.OpenExtEditor, keys.Back}
	case model.ModeContextView:
		return []key.Binding{keys.Edit, keys.OpenExtEditor, keys.Back}
	case model.ModeTaskView:
		return []key.Binding{keys.Run, keys.Edit, keys.Plan, keys.Prompt, keys.CycleStatus, keys.OpenProof, keys.OpenExtEditor, keys.Back}
	case model.ModeHelp:
		return []key.Binding{keys.Help, keys.Back}
	case model.ModeTaskViewAsk, model.ModeGroupPrompt:
		return []key.Binding{keys.Enter, keys.Back}
	default:
		return []key.Binding{keys.Enter, keys.Back}
	}
}

func listModeBindings(keys KeyBindings, sel *model.ListItem) []key.Binding {
	isTask := sel != nil && sel.Kind == model.ListItemTask
	isGroup := sel != nil && sel.Kind == model.ListItemProject
	hasSelection := isTask || isGroup

	var bindings []key.Binding
	bindings = append(bindings, keys.Add, keys.BulkAdd)
	if hasSelection {
		bindings = append(bindings, keys.Edit, keys.Delete)
	}
	bindings = append(bindings, keys.AssignGroup)
	if hasSelection {
		bindings = append(bindings, keys.Run, keys.Plan)
	}
	if isTask {
		bindings = append(bindings, keys.CycleStatus)
	}
	bindings = append(bindings, keys.Prompt, keys.Filter)
	if isGroup && sel.Project != nil && sel.Project.PlanFile != "" {
		bindings = append(bindings, keys.View)
	}
	bindings = append(bindings, keys.Merge, keys.HideDone)
	if hasSelection {
		bindings = append(bindings, keys.Enter)
	}
	if isGroup {
		bindings = append(bindings, keys.Collapse)
	}
	bindings = append(bindings, keys.Help, keys.Quit)
	return bindings
}

func newHelpModel() help.Model {
	h := help.New()
	h.ShortSeparator = "  "
	h.FullSeparator = "  "
	return h
}
