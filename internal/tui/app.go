package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/claude"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/prompt"
	"github.com/davidberget/cctask-go/internal/store"
)

// programReadyMsg carries the tea.Program reference for streaming processes.
type programReadyMsg struct{ p *tea.Program }

// actionContext tracks what action is in progress so we know
// how to handle submit/cancel messages from sub-models.
type actionContext int

const (
	actionNone actionContext = iota
	actionAddTask
	actionEditTask
	actionFilter
	actionNewProject
	actionNewProjectAssign // new project + assign task
	actionNewSubgroup      // new subgroup under selected group
	actionAssignGroup
	actionCombineName
	actionFollowUp
	actionEditPlanContent
	actionGroupPrompt
)

type Model struct {
	projectRoot string
	store       *model.TaskStore
	mode        model.ViewMode
	focusPanel  model.FocusPanel
	listIndex   int
	processIdx  int
	filter      string
	message     string
	width       int
	height      int

	// Derived state
	listItems    []model.ListItem
	selectedItem *model.ListItem

	// Sub-models
	textInput   TextInputModel
	selectInput SelectInputModel
	multiCheck  MultiCheckModel
	form        FormModel
	editor      EditorModel

	// Action context
	action           actionContext
	actionTaskID     string         // task ID for context-dependent actions
	actionPlanFile   string         // plan file for editor actions
	actionScopeGroup string         // group ID for group prompt actions ("" = unassigned)
	actionParentID   string         // parent group ID for subgroup creation
	returnMode       model.ViewMode // mode to return to on cancel

	// Processes
	processes     []model.ClaudeProcess
	runningLabels map[string]bool

	// Combine flow state
	combineSelectedIDs []string

	// Confirm
	confirmMsg      string
	confirmTaskID   string
	confirmGroupID  string
	confirmIsGroup  bool

	// Collapsed groups in list view
	collapsedGroups map[string]bool

	// Scroll offset for fullscreen views
	scrollOffset int

	// Scroll offset for list panel
	listScrollOffset int

	// Theme picker state
	previousTheme string

	// Vim gg state
	pendingG bool

	// Program reference for streaming processes
	program *tea.Program
}

func NewModel(projectRoot string) Model {
	s, _ := store.LoadStore(projectRoot)

	// Load theme from config
	cfg := store.LoadConfig(projectRoot)
	if cfg.Theme != "" {
		ApplyTheme(cfg.Theme)
	}

	m := Model{
		projectRoot:     projectRoot,
		store:           s,
		mode:            model.ModeList,
		collapsedGroups: make(map[string]bool),
		runningLabels:   make(map[string]bool),
		width:           80,
		height:          24,
	}
	m.rebuildList()
	return m
}

var programRef *tea.Program

func Run(projectRoot string) {
	m := NewModel(projectRoot)
	p := tea.NewProgram(m, tea.WithAltScreen())
	programRef = p
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		return programReadyMsg{p: programRef}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case programReadyMsg:
		m.program = msg.p
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.form.Width = msg.Width
		if m.mode == model.ModeEditPlan {
			m.editor.VH = msg.Height - 10
			m.editor.VW = msg.Width - 12
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case FlashMsg:
		m.message = msg.Text
		return m, clearFlashCmd()

	case FlashClearMsg:
		m.message = ""
		return m, nil

	case claude.ClaudeExitMsg:
		m.reload()
		return m, nil

	case claude.ProcessOutputMsg:
		for i := range m.processes {
			if m.processes[i].ID == msg.ID {
				m.processes[i].Output = msg.Output
				if msg.LogFile != "" {
					m.processes[i].LogFile = msg.LogFile
				}
				break
			}
		}
		return m, nil

	case claude.ProcessDoneMsg:
		return m.handleProcessDone(msg)

	case ProcessAutoRemoveMsg:
		return m.handleProcessAutoRemove(msg)

	// Sub-model result messages
	case TextSubmitMsg:
		return m.handleTextSubmit(msg.Value)
	case TextCancelMsg:
		return m.handleTextCancel()
	case FormSubmitMsg:
		return m.handleFormSubmit(msg.Data)
	case FormCancelMsg:
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil
	case SelectSubmitMsg:
		return m.handleSelectSubmit(msg.Value)
	case SelectCancelMsg:
		// Restore previous theme if cancelling theme picker
		if m.mode == model.ModeThemePicker && m.previousTheme != "" {
			ApplyTheme(m.previousTheme)
		}
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil
	case MultiCheckSubmitMsg:
		return m.handleMultiCheckSubmit(msg.Selected)
	case MultiCheckCancelMsg:
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil
	case EditorSaveMsg:
		return m.handleEditorSave(msg.Content)
	case EditorCancelMsg:
		m.mode = m.returnMode
		m.action = actionNone
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	projectName := m.projectRoot
	parts := strings.Split(projectName, "/")
	if len(parts) > 0 {
		projectName = parts[len(parts)-1]
	}

	header := m.renderHeader(projectName)
	statusBar := renderStatusBar(m.mode, m.message, m.width)
	statusRendered := lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1).Render(statusBar)

	// Measure actual rendered heights to calculate available content space
	headerHeight := len(strings.Split(header, "\n"))
	statusHeight := len(strings.Split(statusRendered, "\n"))
	maxContentLines := m.height - headerHeight - statusHeight
	if maxContentLines < 5 {
		maxContentLines = 5
	}

	content := m.renderContent()

	// Clip content to fit available space
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > maxContentLines {
		contentLines = contentLines[:maxContentLines]
		content = strings.Join(contentLines, "\n")
	}

	page := lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2).Render(content),
		statusRendered,
	)

	// Safety: ensure output never exceeds terminal height
	pageLines := strings.Split(page, "\n")
	if len(pageLines) > m.height {
		pageLines = pageLines[:m.height]
		page = strings.Join(pageLines, "\n")
	}

	return page
}

// --- State management ---

func (m *Model) reload() {
	s, _ := store.LoadStore(m.projectRoot)
	m.store = s
	m.rebuildList()
}

func (m *Model) rebuildList() {
	m.listItems = store.BuildListItems(m.store, m.filter, m.collapsedGroups)
	if m.listIndex >= len(m.listItems) && len(m.listItems) > 0 {
		m.listIndex = len(m.listItems) - 1
	}
	m.updateSelectedItem()
}

func (m *Model) updateSelectedItem() {
	if m.focusPanel == model.FocusMain && m.listIndex >= 0 && m.listIndex < len(m.listItems) {
		item := m.listItems[m.listIndex]
		m.selectedItem = &item
	} else {
		m.selectedItem = nil
	}
	m.ensureListScrollVisible()
}

func (m *Model) ensureListScrollVisible() {
	listHeight := m.height - 8
	if listHeight <= 0 || len(m.listItems) == 0 {
		return
	}
	selectedLine := listLineForIndex(m.store, m.listItems, m.listIndex)
	if selectedLine < m.listScrollOffset+2 {
		m.listScrollOffset = max(0, selectedLine-2)
	} else if selectedLine >= m.listScrollOffset+listHeight-1 {
		m.listScrollOffset = selectedLine - listHeight + 2
	}
}

func (m *Model) selectedTask() *model.Task {
	if m.selectedItem != nil && m.selectedItem.Kind == model.ListItemTask {
		return m.selectedItem.Task
	}
	return nil
}

func (m *Model) selectedGroup() *model.Group {
	if m.selectedItem != nil && m.selectedItem.Kind == model.ListItemProject {
		return m.selectedItem.Project
	}
	return nil
}

func (m *Model) selectedAllTasks() bool {
	return m.selectedItem != nil && m.selectedItem.Kind == model.ListItemAllTasks
}

// --- Rendering ---

func (m Model) renderHeader(projectName string) string {
	lineWidth := m.width - 4
	if lineWidth < 40 {
		lineWidth = 40
	}
	title := styleCyanBold.Render("cctask") + " " +
		styleTitle.Render("~/"+projectName) +
		styleGray.Render(fmt.Sprintf("  %d task%s · %d project%s",
			len(m.store.Tasks), pluralize(len(m.store.Tasks)),
			len(m.store.Groups), pluralize(len(m.store.Groups))))

	return lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Render(
		title + "\n\n" + horizontalLine(lineWidth))
}

func (m Model) renderContent() string {
	switch m.mode {
	case model.ModeAddTask, model.ModeEditTask, model.ModeFilter,
		model.ModeEditTags, model.ModeEditDescription, model.ModeTaskViewAsk,
		model.ModeGroupPrompt:
		return m.textInput.View()

	case model.ModeTaskForm:
		return m.form.View()

	case model.ModeAddToGroup, model.ModeThemePicker:
		return m.selectInput.View()

	case model.ModeCombineSelect:
		return m.multiCheck.View()

	case model.ModeCombineName:
		info := styleGray.Render("Combining plans from: " + strings.Join(m.combineSelectedIDs, ", "))
		return info + "\n" + m.textInput.View()

	case model.ModeConfirmDelete:
		return styleYellow.Render(m.confirmMsg) + " " + styleGray.Render("(y/n)")

	case model.ModeEditPlan:
		return m.editor.View()

	case model.ModePlan:
		content := renderPlanView(m.projectRoot, m.selectedTask(), m.selectedGroup(), m.width-8)
		return renderScrollable(content, m.scrollOffset, m.height-8)

	case model.ModeGroupDetail:
		if g := m.selectedGroup(); g != nil {
			content := renderGroupView(g, m.store, m.projectRoot)
			return renderScrollable(content, m.scrollOffset, m.height-8)
		}
		return ""

	case model.ModeTaskView:
		if t := m.selectedTask(); t != nil {
			content := renderTaskView(t, m.projectRoot, m.width-8)
			return renderScrollable(content, m.scrollOffset, m.height-8)
		}
		return ""

	case model.ModeProcessDetail:
		if m.processIdx < len(m.processes) {
			content := renderProcessDetail(&m.processes[m.processIdx])
			return renderScrollable(content, m.scrollOffset, m.height-8)
		}
		return ""

	case model.ModeHelp:
		return renderScrollable(renderHelp(), m.scrollOffset, m.height-8)

	default:
		return m.renderListView()
	}
}

func (m Model) renderListView() string {
	hasProcesses := len(m.processes) > 0

	listHeight := m.height - 8
	listPanel := renderListPanel(m.store, m.projectRoot, m.listItems, m.listIndex,
		m.focusPanel == model.FocusMain, listHeight, m.collapsedGroups, m.listScrollOffset)

	detailWidth := m.width - listPanelWidth - separatorWidth*2 - 4
	if hasProcesses {
		detailWidth -= processPanelWidth + separatorWidth
	}
	if detailWidth < minDetailWidth {
		detailWidth = minDetailWidth
	}
	if detailWidth > maxDetailWidth {
		detailWidth = maxDetailWidth
	}

	detailPanel := renderDetailPanel(m.store, m.projectRoot, m.selectedItem, detailWidth)
	sep := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2).Render(verticalSeparator(listHeight))
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, sep, detailPanel)

	if hasProcesses {
		processPanel := renderProcessPanel(m.processes, m.processIdx, m.focusPanel == model.FocusProcesses)
		panels = lipgloss.JoinHorizontal(lipgloss.Top, panels, sep, processPanel)
	}

	// Clip to available height so the terminal doesn't scroll past the top
	panelLines := strings.Split(panels, "\n")
	if len(panelLines) > listHeight {
		panelLines = panelLines[:listHeight]
	}
	return strings.Join(panelLines, "\n")
}

// --- Key handling ---

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case model.ModeAddTask, model.ModeEditTask, model.ModeFilter,
		model.ModeEditTags, model.ModeEditDescription,
		model.ModeTaskViewAsk, model.ModeCombineName,
		model.ModeGroupPrompt:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd

	case model.ModeTaskForm:
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		return m, cmd

	case model.ModeAddToGroup, model.ModeThemePicker:
		var cmd tea.Cmd
		m.selectInput, cmd = m.selectInput.Update(msg)
		// Live preview: apply theme as user navigates
		if m.mode == model.ModeThemePicker && m.selectInput.Index < len(m.selectInput.Items) {
			ApplyTheme(m.selectInput.Items[m.selectInput.Index].Value)
		}
		return m, cmd

	case model.ModeCombineSelect:
		var cmd tea.Cmd
		m.multiCheck, cmd = m.multiCheck.Update(msg)
		return m, cmd

	case model.ModeConfirmDelete:
		return m.handleConfirm(msg)

	case model.ModeEditPlan:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd
	}

	return m.handleNavKey(msg)
}

func (m Model) isFullscreenMode() bool {
	switch m.mode {
	case model.ModePlan, model.ModeTaskView, model.ModeGroupDetail, model.ModeProcessDetail, model.ModeHelp:
		return true
	}
	return false
}

func (m Model) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Clear pending g on any non-g key
	if key != "g" {
		m.pendingG = false
	}

	if key == "ctrl+c" || (key == "q" && m.mode == model.ModeList) {
		return m, tea.Quit
	}

	if key == "?" {
		if m.mode == model.ModeHelp {
			m.mode = model.ModeList
		} else {
			m.mode = model.ModeHelp
			m.scrollOffset = 0
		}
		return m, nil
	}

	if key == "esc" && m.mode != model.ModeList {
		m.mode = model.ModeList
		m.scrollOffset = 0
		return m, nil
	}

	// Scroll in fullscreen views (including help)
	if m.isFullscreenMode() {
		halfPage := max(1, (m.height-8)/2)
		switch key {
		case "j", "down":
			m.scrollOffset++
			return m, nil
		case "k", "up":
			m.scrollOffset = max(0, m.scrollOffset-1)
			return m, nil
		case "d", "ctrl+d":
			m.scrollOffset += halfPage
			return m, nil
		case "u", "ctrl+u":
			m.scrollOffset = max(0, m.scrollOffset-halfPage)
			return m, nil
		case "G":
			m.scrollOffset = 999999 // clamped in renderScrollable
			return m, nil
		case "g":
			if m.pendingG {
				m.pendingG = false
				m.scrollOffset = 0
			} else {
				m.pendingG = true
			}
			return m, nil
		}
	}

	// Help mode only responds to scroll, ?, Esc, Ctrl+C (handled above)
	if m.mode == model.ModeHelp {
		return m, nil
	}

	if key == "k" || key == "up" {
		if m.focusPanel == model.FocusMain && len(m.listItems) > 0 {
			m.listIndex = max(0, m.listIndex-1)
			m.updateSelectedItem()
		} else if m.focusPanel == model.FocusProcesses && len(m.processes) > 0 {
			m.processIdx = max(0, m.processIdx-1)
		}
		return m, nil
	}
	if key == "j" || key == "down" {
		if m.focusPanel == model.FocusMain && len(m.listItems) > 0 {
			m.listIndex = min(len(m.listItems)-1, m.listIndex+1)
			m.updateSelectedItem()
		} else if m.focusPanel == model.FocusProcesses && len(m.processes) > 0 {
			m.processIdx = min(len(m.processes)-1, m.processIdx+1)
		}
		return m, nil
	}

	if key == "tab" {
		if m.focusPanel == model.FocusMain && len(m.processes) > 0 {
			m.focusPanel = model.FocusProcesses
		} else {
			m.focusPanel = model.FocusMain
		}
		m.updateSelectedItem()
		return m, nil
	}

	if key == " " && m.mode == model.ModeList && m.focusPanel == model.FocusMain {
		if g := m.selectedGroup(); g != nil {
			m.collapsedGroups[g.ID] = !m.collapsedGroups[g.ID]
			m.rebuildList()
			return m, nil
		}
	}

	if key == "enter" && (m.mode == model.ModeList || m.mode == model.ModeDetail) {
		if m.focusPanel == model.FocusMain {
			if m.selectedTask() != nil {
				m.mode = model.ModeDetail
			} else if m.selectedGroup() != nil {
				m.scrollOffset = 0
				m.mode = model.ModeGroupDetail
			}
		} else if m.focusPanel == model.FocusProcesses && m.processIdx < len(m.processes) {
			m.scrollOffset = 0
			m.mode = model.ModeProcessDetail
		}
		return m, nil
	}

	if key == "v" && (m.mode == model.ModeList || m.mode == model.ModeDetail) && m.selectedTask() != nil {
		m.scrollOffset = 0
		m.mode = model.ModeTaskView
		return m, nil
	}

	if key == "c" && m.mode == model.ModeTaskView && m.selectedTask() != nil {
		m.action = actionFollowUp
		m.actionTaskID = m.selectedTask().ID
		m.textInput = NewTextInput("Question for Claude", "")
		m.mode = model.ModeTaskViewAsk
		return m, nil
	}

	if key == "o" && m.processIdx < len(m.processes) {
		if m.focusPanel == model.FocusProcesses || m.mode == model.ModeProcessDetail {
			proc := &m.processes[m.processIdx]
			if proc.Status == model.ProcessRunning {
				return m, flashCmd("Still running — press Enter to view output")
			}
			// Completed: launch interactive claude --continue
			return m, claude.ExecContinue(m.projectRoot)
		}
	}

	if key == "a" && m.mode == model.ModeList {
		m.action = actionAddTask
		m.form = NewForm("New Task", nil, m.width)
		m.mode = model.ModeTaskForm
		return m, nil
	}

	if key == "e" {
		return m.handleEdit()
	}

	if key == "d" && (m.mode == model.ModeList || m.mode == model.ModeDetail) {
		return m.handleDelete()
	}

	if key == "s" && m.selectedTask() != nil {
		return m.cycleStatus()
	}

	if key == "g" && m.mode == model.ModeList {
		return m.handleAssignGroup()
	}

	if key == "r" {
		return m.handleRun()
	}

	if key == "p" && (m.mode == model.ModeList || m.mode == model.ModeDetail || m.mode == model.ModeTaskView) {
		return m.handlePlan()
	}

	if key == "/" && m.mode == model.ModeList {
		m.action = actionFilter
		m.textInput = NewTextInput("Filter", m.filter)
		m.mode = model.ModeFilter
		return m, nil
	}

	if key == "m" && m.mode == model.ModeList {
		return m.handleCombine()
	}

	if key == "c" && m.mode == model.ModeList && m.focusPanel == model.FocusMain {
		return m.handleGroupPrompt()
	}

	if key == "t" && m.mode == model.ModeList {
		cfg := store.LoadConfig(m.projectRoot)
		m.previousTheme = cfg.Theme
		if m.previousTheme == "" {
			m.previousTheme = "default"
		}
		var items []SelectItem
		for _, name := range ThemeNames {
			tc := Themes[name]
			items = append(items, SelectItem{Label: tc.Name, Value: name})
		}
		m.selectInput = NewSelectInput("Theme", items)
		// Set initial index to current theme
		for i, name := range ThemeNames {
			if name == m.previousTheme {
				m.selectInput.Index = i
				break
			}
		}
		m.mode = model.ModeThemePicker
		return m, nil
	}

	return m, nil
}

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "y" || key == "enter" {
		if m.confirmIsGroup {
			store.DeleteGroup(m.projectRoot, m.confirmGroupID)
			m.reload()
			m.listIndex = max(0, m.listIndex-1)
			m.updateSelectedItem()
			m.mode = model.ModeList
			return m, flashCmd("Project deleted")
		}
		store.DeleteTask(m.projectRoot, m.confirmTaskID)
		m.reload()
		m.listIndex = max(0, m.listIndex-1)
		m.updateSelectedItem()
		m.mode = model.ModeList
		return m, flashCmd("Task deleted")
	}
	if key == "n" || key == "esc" {
		m.mode = model.ModeList
	}
	return m, nil
}

// --- Submit/Cancel handlers ---

func (m Model) handleTextSubmit(value string) (tea.Model, tea.Cmd) {
	switch m.action {
	case actionFilter:
		m.filter = value
		m.listIndex = 0
		m.listScrollOffset = 0
		m.rebuildList()
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil

	case actionNewProject:
		store.AddGroup(m.projectRoot, value, "")
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd(fmt.Sprintf("Project \"%s\" created", value))

	case actionNewSubgroup:
		store.AddGroupWithParent(m.projectRoot, value, "", m.actionParentID)
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd(fmt.Sprintf("Subgroup \"%s\" created", value))

	case actionNewProjectAssign:
		g, _ := store.AddGroup(m.projectRoot, value, "")
		if g != nil {
			store.UpdateTask(m.projectRoot, m.actionTaskID, map[string]interface{}{"group": g.ID})
		}
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd(fmt.Sprintf("Project \"%s\" created, task assigned", value))

	case actionCombineName:
		return m.executeCombinePlans(m.combineSelectedIDs, value)

	case actionFollowUp:
		return m.executeFollowUp(m.actionTaskID, value)

	case actionGroupPrompt:
		return m.spawnGroupAction(m.actionScopeGroup, value)
	}

	m.mode = model.ModeList
	m.action = actionNone
	return m, nil
}

func (m Model) handleTextCancel() (tea.Model, tea.Cmd) {
	switch m.action {
	case actionFollowUp:
		m.mode = model.ModeTaskView
	case actionGroupPrompt:
		m.mode = model.ModeList
	default:
		m.mode = model.ModeList
	}
	m.action = actionNone
	return m, nil
}

func (m Model) handleFormSubmit(data TaskFormData) (tea.Model, tea.Cmd) {
	var tags []string
	for _, t := range strings.Split(data.Tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}

	switch m.action {
	case actionAddTask:
		store.AddTask(m.projectRoot, data.Title, data.Description, tags, "")
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Task added")

	case actionEditTask:
		store.UpdateTask(m.projectRoot, m.actionTaskID, map[string]interface{}{
			"title":       data.Title,
			"description": data.Description,
			"tags":        tags,
		})
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Task updated")
	}

	m.mode = model.ModeList
	m.action = actionNone
	return m, nil
}

func (m Model) handleSelectSubmit(value string) (tea.Model, tea.Cmd) {
	if m.mode == model.ModeThemePicker {
		ApplyTheme(value)
		cfg := store.LoadConfig(m.projectRoot)
		cfg.Theme = value
		store.SaveConfig(m.projectRoot, cfg)
		m.mode = model.ModeList
		tc := Themes[value]
		return m, flashCmd("Theme: " + tc.Name)
	}

	switch value {
	case "__new__":
		m.action = actionNewProjectAssign
		m.textInput = NewTextInput("New project name", "")
		m.mode = model.ModeAddTask
		return m, nil
	case "__remove__":
		store.UpdateTask(m.projectRoot, m.actionTaskID, map[string]interface{}{"group": ""})
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Removed from project")
	default:
		store.UpdateTask(m.projectRoot, m.actionTaskID, map[string]interface{}{"group": value})
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Task assigned to project")
	}
}

func (m Model) handleMultiCheckSubmit(selected []string) (tea.Model, tea.Cmd) {
	m.combineSelectedIDs = selected
	m.action = actionCombineName
	m.textInput = NewTextInput("Combined plan name", "")
	m.textInput.Placeholder = "e.g. auth-full-plan"
	m.mode = model.ModeCombineName
	return m, nil
}

func (m Model) handleEditorSave(content string) (tea.Model, tea.Cmd) {
	if m.actionPlanFile != "" {
		store.SavePlan(m.projectRoot, m.actionPlanFile, content)
		m.reload()
		m.mode = m.returnMode
		m.action = actionNone
		return m, flashCmd("Plan saved")
	}
	m.mode = model.ModeList
	m.action = actionNone
	return m, nil
}

// --- Action initiators ---

func (m Model) handleEdit() (tea.Model, tea.Cmd) {
	if m.mode == model.ModePlan {
		planFile := ""
		title := ""
		if t := m.selectedTask(); t != nil {
			planFile = t.PlanFile
			title = fmt.Sprintf("Edit Plan — %s: %s", t.ID, t.Title)
		} else if g := m.selectedGroup(); g != nil {
			planFile = g.PlanFile
			title = fmt.Sprintf("Edit Plan — %s", g.Name)
		}
		if planFile != "" {
			content, _ := store.LoadPlan(m.projectRoot, planFile)
			m.action = actionEditPlanContent
			m.actionPlanFile = planFile
			m.returnMode = model.ModePlan
			m.editor = NewEditor(title, content, m.height-10, m.width-12)
			m.mode = model.ModeEditPlan
		}
		return m, nil
	}

	if t := m.selectedTask(); t != nil {
		if t.PlanFile != "" {
			content, _ := store.LoadPlan(m.projectRoot, t.PlanFile)
			title := fmt.Sprintf("Edit Plan — %s: %s", t.ID, t.Title)
			m.action = actionEditPlanContent
			m.actionPlanFile = t.PlanFile
			m.returnMode = m.mode
			m.editor = NewEditor(title, content, m.height-10, m.width-12)
			m.mode = model.ModeEditPlan
		} else {
			m.action = actionEditTask
			m.actionTaskID = t.ID
			initial := &TaskFormData{
				Title:       t.Title,
				Description: t.Description,
				Tags:        strings.Join(t.Tags, ", "),
			}
			m.form = NewForm(fmt.Sprintf("Edit Task — %s", t.ID), initial, m.width)
			m.mode = model.ModeTaskForm
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleDelete() (tea.Model, tea.Cmd) {
	if t := m.selectedTask(); t != nil {
		m.confirmMsg = fmt.Sprintf("Delete \"%s\"?", t.Title)
		m.confirmTaskID = t.ID
		m.confirmIsGroup = false
		m.mode = model.ModeConfirmDelete
	} else if g := m.selectedGroup(); g != nil {
		children := store.GetChildGroups(m.store, g.ID)
		if len(children) > 0 {
			m.confirmMsg = fmt.Sprintf("Delete project \"%s\" and %d subgroup%s?", g.Name, len(children), pluralize(len(children)))
		} else {
			m.confirmMsg = fmt.Sprintf("Delete project \"%s\"?", g.Name)
		}
		m.confirmGroupID = g.ID
		m.confirmIsGroup = true
		m.mode = model.ModeConfirmDelete
	}
	return m, nil
}

func (m Model) cycleStatus() (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	next := t.Status.Next()
	store.UpdateTask(m.projectRoot, t.ID, map[string]interface{}{"status": next})
	m.reload()
	return m, flashCmd(fmt.Sprintf("Status → %s", next))
}

func (m Model) handleAssignGroup() (tea.Model, tea.Cmd) {
	if t := m.selectedTask(); t != nil {
		m.actionTaskID = t.ID
		if len(m.store.Groups) == 0 {
			m.action = actionNewProjectAssign
			m.textInput = NewTextInput("New project name", "")
			m.mode = model.ModeAddTask
			return m, nil
		}
		items := buildHierarchicalGroupSelect(m.store, "", 0)
		items = append(items, SelectItem{Label: "+ New project", Value: "__new__"})
		items = append(items, SelectItem{Label: "Remove from project", Value: "__remove__"})
		m.action = actionAssignGroup
		m.selectInput = NewSelectInput("Assign to project:", items)
		m.mode = model.ModeAddToGroup
		return m, nil
	}

	// No task selected — if group is selected, create subgroup under it
	if g := m.selectedGroup(); g != nil {
		m.actionParentID = g.ID
		m.action = actionNewSubgroup
		m.textInput = NewTextInput(fmt.Sprintf("New subgroup under \"%s\"", g.Name), "")
		m.mode = model.ModeAddTask
		return m, nil
	}

	m.action = actionNewProject
	m.textInput = NewTextInput("New project name", "")
	m.mode = model.ModeAddTask
	return m, nil
}

// buildHierarchicalGroupSelect builds a flat list of SelectItems with indentation for tree display.
func buildHierarchicalGroupSelect(s *model.TaskStore, parentID string, depth int) []SelectItem {
	var items []SelectItem
	for _, g := range s.Groups {
		if g.ParentGroup != parentID {
			continue
		}
		prefix := strings.Repeat("  ", depth)
		items = append(items, SelectItem{Label: prefix + g.Name, Value: g.ID})
		items = append(items, buildHierarchicalGroupSelect(s, g.ID, depth+1)...)
	}
	return items
}

func (m Model) handleRun() (tea.Model, tea.Cmd) {
	if t := m.selectedTask(); t != nil {
		p := prompt.BuildTaskPrompt(m.projectRoot, t)
		return m, tea.Batch(
			claude.SpawnInTerminal(m.projectRoot, p),
			flashCmd("Opening claude in new terminal..."),
		)
	}
	if g := m.selectedGroup(); g != nil {
		p := prompt.BuildGroupPrompt(m.projectRoot, g, m.store)
		return m, tea.Batch(
			claude.SpawnInTerminal(m.projectRoot, p),
			flashCmd("Opening claude in new terminal..."),
		)
	}
	return m, nil
}

func (m Model) handlePlan() (tea.Model, tea.Cmd) {
	if t := m.selectedTask(); t != nil {
		if t.PlanFile != "" {
			m.scrollOffset = 0
			m.mode = model.ModePlan
			return m, nil
		}
		return m.spawnPlanGeneration(t)
	}
	if g := m.selectedGroup(); g != nil {
		if g.PlanFile != "" {
			m.scrollOffset = 0
			m.mode = model.ModePlan
			return m, nil
		}
		return m.spawnGroupPlanGeneration(g)
	}
	return m, nil
}

func (m Model) handleCombine() (tea.Model, tea.Cmd) {
	withPlans := store.GetTasksWithPlans(m.projectRoot, m.store)
	if len(withPlans) < 2 {
		return m, flashCmd("Need at least 2 tasks with plans to combine")
	}

	var items []CheckItem
	for _, t := range m.store.Tasks {
		hasPlan := false
		for _, wp := range withPlans {
			if wp.ID == t.ID {
				hasPlan = true
				break
			}
		}
		items = append(items, CheckItem{
			Label:    fmt.Sprintf("%s  %s", t.ID, t.Title),
			Value:    t.ID,
			Disabled: !hasPlan,
		})
	}

	m.multiCheck = NewMultiCheck("Select tasks to combine plans:", items)
	m.mode = model.ModeCombineSelect
	return m, nil
}

// --- Group prompt ---

func (m Model) handleGroupPrompt() (tea.Model, tea.Cmd) {
	var scopeGroupID string
	var scopeLabel string

	if m.selectedAllTasks() {
		scopeGroupID = "__all__"
		scopeLabel = "All Tasks"
	} else if g := m.selectedGroup(); g != nil {
		scopeGroupID = g.ID
		scopeLabel = g.Name
	} else if t := m.selectedTask(); t != nil {
		if t.Group != "" {
			scopeGroupID = t.Group
			if g := store.FindGroup(m.store, t.Group); g != nil {
				scopeLabel = g.Name
			} else {
				scopeLabel = t.Group
			}
		} else {
			scopeGroupID = ""
			scopeLabel = "Unassigned"
		}
	} else {
		return m, nil
	}

	m.actionScopeGroup = scopeGroupID
	m.action = actionGroupPrompt
	m.textInput = NewTextInput(fmt.Sprintf("Prompt for %s", scopeLabel), "")
	m.textInput.Placeholder = "e.g. fill out tags, regroup tasks, review descriptions..."
	m.mode = model.ModeGroupPrompt
	return m, nil
}

// groupActionResult is the JSON structure Claude returns from a group action prompt.
type groupActionResult struct {
	Tasks         []groupActionTask         `json:"tasks"`
	NewGroups     []groupActionGroup        `json:"newGroups"`
	UpdatedGroups []groupActionGroupUpdate  `json:"updatedGroups"`
	Summary       string                    `json:"summary"`
}

type groupActionTask struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Group       string   `json:"group"`
}

type groupActionGroup struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentGroup string `json:"parentGroup"`
}

type groupActionGroupUpdate struct {
	ID          string `json:"id"`
	ParentGroup string `json:"parentGroup"`
}

func (m Model) spawnGroupAction(scopeGroupID string, instruction string) (tea.Model, tea.Cmd) {
	// Gather tasks in scope
	var scopeTasks []model.Task
	if scopeGroupID == "__all__" {
		scopeTasks = append(scopeTasks, m.store.Tasks...)
	} else if scopeGroupID != "" {
		scopeTasks = store.GetTasksForGroup(m.store, scopeGroupID)
	} else {
		for _, t := range m.store.Tasks {
			if t.Group == "" {
				scopeTasks = append(scopeTasks, t)
			}
		}
	}
	if len(scopeTasks) == 0 {
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("No tasks in scope")
	}

	scopeLabel := "Unassigned"
	if scopeGroupID != "" {
		if g := store.FindGroup(m.store, scopeGroupID); g != nil {
			scopeLabel = g.Name
		}
	}

	promptText := prompt.BuildGroupActionPrompt(scopeTasks, m.store.Groups, scopeLabel, instruction)

	label := "Prompt: " + scopeLabel
	if m.runningLabels[label] {
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Already running...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("group-prompt-%d", time.Now().Unix())
	logFile := filepath.Join(m.projectRoot, ".cctask", "logs", procID+".log")
	proc := model.ClaudeProcess{
		ID:      procID,
		Label:   label,
		Status:  model.ProcessRunning,
		Output:  "Processing tasks...",
		LogFile: logFile,
	}
	m.processes = append(m.processes, proc)

	m.mode = model.ModeList
	m.action = actionNone

	projectRoot := m.projectRoot

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnPlanCmd(m.program, projectRoot, procID, label, promptText)
		innerCmd := cmd
		cmd = func() tea.Msg {
			result := innerCmd()
			if done, ok := result.(claude.ProcessDoneMsg); ok && done.Err == nil {
				applyGroupActionResult(projectRoot, done.Output)
			}
			return result
		}
	} else {
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			applyGroupActionResult(projectRoot, output)
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd(fmt.Sprintf("Running prompt on %s...", scopeLabel)))
}

// extractJSON strips markdown code block fences from Claude's output.
func extractJSON(output string) string {
	output = strings.TrimSpace(output)
	if strings.HasPrefix(output, "```") {
		lines := strings.Split(output, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
		}
		output = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(output)
}

func applyGroupActionResult(projectRoot string, output string) {
	cleaned := extractJSON(output)
	var result groupActionResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return
	}

	s, err := store.LoadStore(projectRoot)
	if err != nil {
		return
	}

	// Create new groups first so task reassignments can reference them
	for _, ng := range result.NewGroups {
		if ng.Name == "" {
			continue
		}
		id := store.Slugify(ng.Name)
		// Skip if group already exists
		if store.FindGroup(s, id) != nil {
			continue
		}
		s.Groups = append(s.Groups, model.Group{
			ID:          id,
			Name:        ng.Name,
			Description: ng.Description,
			ParentGroup: ng.ParentGroup,
			Created:     model.Now(),
		})
	}

	// Apply group hierarchy updates (re-parenting existing groups)
	for _, ug := range result.UpdatedGroups {
		if ug.ID == "" {
			continue
		}
		g := store.FindGroup(s, ug.ID)
		if g == nil {
			continue
		}
		g.ParentGroup = ug.ParentGroup
	}

	// Apply task updates
	for _, ut := range result.Tasks {
		t := store.FindTask(s, ut.ID)
		if t == nil {
			continue
		}
		if ut.Title != "" {
			t.Title = ut.Title
		}
		t.Description = ut.Description
		if ut.Status != "" {
			t.Status = model.TaskStatus(ut.Status)
		}
		if ut.Tags != nil {
			t.Tags = ut.Tags
		}
		t.Group = ut.Group
		t.Updated = model.Now()
	}

	store.SaveStore(projectRoot, s)
}

// --- Process management ---

func (m Model) spawnPlanGeneration(task *model.Task) (tea.Model, tea.Cmd) {
	label := "Plan: " + task.Title
	if m.runningLabels[label] {
		return m, flashCmd("Already generating...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("plan-%s-%d", task.ID, time.Now().Unix())
	logFile := filepath.Join(m.projectRoot, ".cctask", "logs", procID+".log")
	proc := model.ClaudeProcess{
		ID:      procID,
		Label:   label,
		Status:  model.ProcessRunning,
		Output:  "Waiting for claude...",
		LogFile: logFile,
	}
	m.processes = append(m.processes, proc)


	promptText := prompt.BuildPlanGenerationPrompt(task)
	taskID := task.ID
	taskTitle := task.Title
	projectRoot := m.projectRoot

	var cmd tea.Cmd
	if m.program != nil {
		// Use streaming mode — output updates appear live
		cmd = claude.SpawnPlanCmd(m.program, projectRoot, procID, label, promptText)
		// Wrap to save plan on completion
		innerCmd := cmd
		cmd = func() tea.Msg {
			result := innerCmd()
			if done, ok := result.(claude.ProcessDoneMsg); ok && done.Err == nil {
				filename := store.PlanFilenameForTask(&model.Task{ID: taskID, Title: taskTitle})
				store.SavePlan(projectRoot, filename, done.Output)
				store.UpdateTask(projectRoot, taskID, map[string]interface{}{"planFile": filename})
			}
			return result
		}
	} else {
		// Fallback: synchronous pipe
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			filename := store.PlanFilenameForTask(&model.Task{ID: taskID, Title: taskTitle})
			store.SavePlan(projectRoot, filename, output)
			store.UpdateTask(projectRoot, taskID, map[string]interface{}{"planFile": filename})
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd(fmt.Sprintf("Generating plan for %s...", taskID)))
}

func (m Model) spawnGroupPlanGeneration(group *model.Group) (tea.Model, tea.Cmd) {
	label := "Plan: " + group.Name
	if m.runningLabels[label] {
		return m, flashCmd("Already generating...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("plan-%s-%d", group.ID, time.Now().Unix())
	logFile := filepath.Join(m.projectRoot, ".cctask", "logs", procID+".log")
	proc := model.ClaudeProcess{
		ID:      procID,
		Label:   label,
		Status:  model.ProcessRunning,
		Output:  "Waiting for claude...",
		LogFile: logFile,
	}
	m.processes = append(m.processes, proc)


	tasks := store.GetTasksForGroup(m.store, group.ID)
	promptText := prompt.BuildGroupPlanGenerationPrompt(group, tasks)
	groupID := group.ID
	projectRoot := m.projectRoot

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnPlanCmd(m.program, projectRoot, procID, label, promptText)
		innerCmd := cmd
		cmd = func() tea.Msg {
			result := innerCmd()
			if done, ok := result.(claude.ProcessDoneMsg); ok && done.Err == nil {
				filename := store.PlanFilenameForGroup(&model.Group{ID: groupID})
				store.SavePlan(projectRoot, filename, done.Output)
				s, _ := store.LoadStore(projectRoot)
				if g := store.FindGroup(s, groupID); g != nil {
					g.PlanFile = filename
					store.SaveStore(projectRoot, s)
				}
			}
			return result
		}
	} else {
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			filename := store.PlanFilenameForGroup(&model.Group{ID: groupID})
			store.SavePlan(projectRoot, filename, output)
			s, _ := store.LoadStore(projectRoot)
			if g := store.FindGroup(s, groupID); g != nil {
				g.PlanFile = filename
				store.SaveStore(projectRoot, s)
			}
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd("Generating project plan..."))
}

func (m Model) executeCombinePlans(taskIDs []string, name string) (tea.Model, tea.Cmd) {
	label := "Combine: " + name
	if m.runningLabels[label] {
		return m, flashCmd("Already combining...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("combine-%d", time.Now().Unix())
	logFile := filepath.Join(m.projectRoot, ".cctask", "logs", procID+".log")
	proc := model.ClaudeProcess{
		ID:      procID,
		Label:   label,
		Status:  model.ProcessRunning,
		Output:  "Waiting for claude...",
		LogFile: logFile,
	}
	m.processes = append(m.processes, proc)

	m.mode = model.ModeList
	m.action = actionNone

	var tasks []model.Task
	for _, id := range taskIDs {
		if t := store.FindTask(m.store, id); t != nil {
			tasks = append(tasks, *t)
		}
	}
	promptText := prompt.BuildCombinePlansPrompt(m.projectRoot, tasks)
	projectRoot := m.projectRoot
	planName := name

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnPlanCmd(m.program, projectRoot, procID, label, promptText)
		innerCmd := cmd
		cmd = func() tea.Msg {
			result := innerCmd()
			if done, ok := result.(claude.ProcessDoneMsg); ok && done.Err == nil {
				store.AddCombinedPlan(projectRoot, planName, taskIDs, done.Output)
			}
			return result
		}
	} else {
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			store.AddCombinedPlan(projectRoot, planName, taskIDs, output)
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd("Combining plans..."))
}

func (m Model) executeFollowUp(taskID, question string) (tea.Model, tea.Cmd) {
	task := store.FindTask(m.store, taskID)
	if task == nil {
		m.mode = model.ModeTaskView
		m.action = actionNone
		return m, nil
	}

	promptText := prompt.BuildPlanFollowUpPrompt(m.projectRoot, task, question)
	label := "Ask: " + task.Title
	procID := fmt.Sprintf("followup-%s-%d", taskID, time.Now().Unix())
	logFile := filepath.Join(m.projectRoot, ".cctask", "logs", procID+".log")
	proc := model.ClaudeProcess{
		ID:      procID,
		Label:   label,
		Status:  model.ProcessRunning,
		Output:  "Waiting for claude...",
		LogFile: logFile,
	}
	m.processes = append(m.processes, proc)

	m.mode = model.ModeTaskView
	m.action = actionNone

	planFile := task.PlanFile
	if planFile == "" {
		planFile = store.PlanFilenameForTask(task)
	}
	projectRoot := m.projectRoot
	hasPlanFile := task.PlanFile != ""

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnPlanCmd(m.program, projectRoot, procID, label, promptText)
		innerCmd := cmd
		cmd = func() tea.Msg {
			result := innerCmd()
			if done, ok := result.(claude.ProcessDoneMsg); ok && done.Err == nil {
				store.SavePlan(projectRoot, planFile, done.Output)
				if !hasPlanFile {
					store.UpdateTask(projectRoot, taskID, map[string]interface{}{"planFile": planFile})
				}
			}
			return result
		}
	} else {
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			store.SavePlan(projectRoot, planFile, output)
			if !hasPlanFile {
				store.UpdateTask(projectRoot, taskID, map[string]interface{}{"planFile": planFile})
			}
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd("Asking Claude..."))
}

func (m Model) handleProcessDone(msg claude.ProcessDoneMsg) (tea.Model, tea.Cmd) {
	for i := range m.processes {
		if m.processes[i].ID == msg.ID {
			if msg.LogFile != "" {
				m.processes[i].LogFile = msg.LogFile
			}
			if msg.Err != nil {
				m.processes[i].Status = model.ProcessError
				m.processes[i].Output = msg.Output + "\nError: " + msg.Err.Error()
			} else {
				m.processes[i].Status = model.ProcessDone
				if msg.Output != "" {
					m.processes[i].Output = msg.Output
				} else {
					m.processes[i].Output = "Done (no output)"
				}
			}
			delete(m.runningLabels, m.processes[i].Label)
			break
		}
	}
	m.reload()

	return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return ProcessAutoRemoveMsg{ID: msg.ID}
	})
}

func (m Model) handleProcessAutoRemove(msg ProcessAutoRemoveMsg) (tea.Model, tea.Cmd) {
	var newProcs []model.ClaudeProcess
	for _, p := range m.processes {
		if p.ID != msg.ID {
			newProcs = append(newProcs, p)
		}
	}
	m.processes = newProcs
	if m.focusPanel == model.FocusProcesses && len(m.processes) == 0 {
		m.focusPanel = model.FocusMain
		if m.mode == model.ModeProcessDetail {
			m.mode = model.ModeList
		}
	}
	if m.processIdx >= len(m.processes) {
		m.processIdx = max(0, len(m.processes)-1)
	}
	return m, nil
}


func runClaudePipe(projectRoot, procID, promptText string) (string, string, error) {
	logsDir := filepath.Join(projectRoot, ".cctask", "logs")
	os.MkdirAll(logsDir, 0o755)
	logFile := filepath.Join(logsDir, procID+".log")

	c := exec.Command("claude", "-p")
	c.Dir = projectRoot
	c.Stdin = strings.NewReader(promptText)
	output, err := c.Output()
	if err != nil {
		return "", logFile, err
	}
	result := strings.TrimSpace(string(output))
	os.WriteFile(logFile, []byte(result), 0o644)
	return result, logFile, nil
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
