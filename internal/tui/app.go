package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidberget/cctask-go/internal/agent"
	"github.com/davidberget/cctask-go/internal/claude"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/prompt"
	"github.com/davidberget/cctask-go/internal/skill"
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
	actionEditContext
	actionGroupPrompt
	actionAgentPlan
	actionAgentRun
	actionRunMode
	actionRegenPlan
	actionRenameGroup
	actionBulkAdd
)

type Model struct {
	projectRoot string
	store       *model.TaskStore
	mode        model.ViewMode
	focusPanel  model.FocusPanel
	listIndex   int
	processIdx  int
	filter        string
	hideCompleted bool
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
	chatInput   ChatInputModel

	// Action context
	action           actionContext
	actionTaskID     string         // task ID for context-dependent actions
	actionPlanFile   string         // plan file for editor actions
	actionScopeGroup string         // group ID for group prompt actions ("" = unassigned)
	actionParentID   string         // parent group ID for subgroup creation
	actionAgent      *agent.Agent   // stashed agent between agent picker and run mode picker
	returnMode       model.ViewMode // mode to return to on cancel

	// Processes
	processes      []model.ClaudeProcess
	runningLabels  map[string]bool
	processCancels *claude.ProcessCancels
	processInputs  *claude.ProcessInputs

	// Combine flow state
	combineSelectedIDs []string

	// Confirm
	confirmMsg      string
	confirmTaskID   string
	confirmGroupID  string
	confirmIsGroup  bool

	// Collapsed groups in list view
	collapsedGroups map[string]bool

	// Viewport for fullscreen view scrolling
	viewport          viewport.Model
	processAutoScroll bool // pin process detail to bottom while streaming

	// Scroll offset for list panel
	listScrollOffset int

	// Theme picker state
	previousTheme string

	// Vim gg state
	pendingG bool

	// Key bindings and help
	keys      KeyBindings
	helpModel help.Model

	// Group progress bar
	groupProgress progress.Model

	// Table, list, and file picker views
	taskTable  table.Model
	taskList   list.Model
	filePicker filepicker.Model

	// Process panel pagination
	processPaginator paginator.Model

	// Spinner for planning/running tasks
	spinner spinner.Model

	// Program reference for streaming processes
	program *tea.Program

	// Agents loaded from .claude/agents/
	agents []agent.Agent

	// Skills loaded from .claude/skills/
	skills []skill.Skill
}

func NewModel(projectRoot string) Model {
	s, _ := store.LoadStore(projectRoot)

	// Load theme from config
	cfg := store.LoadConfig(projectRoot)
	if cfg.Theme != "" {
		ApplyTheme(cfg.Theme)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    80 * time.Millisecond,
	}
	sp.Style = styleMagenta

	vp := viewport.New(76, 16)
	vp.MouseWheelEnabled = true

	prog := progress.New(progress.WithScaledGradient(string(colorPrimary), string(colorSuccess)))
	prog.Width = 40

	pg := paginator.New()
	pg.Type = paginator.Dots
	pg.ActiveDot = lipgloss.NewStyle().Foreground(colorPrimary).Render("●")
	pg.InactiveDot = lipgloss.NewStyle().Foreground(colorDim).Render("○")
	pg.SetTotalPages(0)
	pg.PerPage = 4

	m := Model{
		projectRoot:     projectRoot,
		store:           s,
		mode:            model.ModeList,
		hideCompleted:   true,
		collapsedGroups: make(map[string]bool),
		runningLabels:   make(map[string]bool),
		processCancels:  &claude.ProcessCancels{},
		processInputs:   &claude.ProcessInputs{},
		width:           80,
		height:          24,
		agents:          agent.LoadAgents(projectRoot),
		skills:          skill.LoadSkills(projectRoot),
		keys:               NewKeyBindings(),
		helpModel:          newHelpModel(),
		groupProgress:      prog,
		viewport:           vp,
		spinner:            sp,
		processPaginator:   pg,
	}
	m.rebuildList()
	return m
}

// processTimeout returns the configured process timeout duration.
func (m Model) processTimeout() time.Duration {
	cfg := store.LoadConfig(m.projectRoot)
	return cfg.Timeout()
}

// skillNames returns the names of available skills for the form picker,
// or nil if the skill picker is disabled via config.
func (m Model) skillNames() []string {
	cfg := store.LoadConfig(m.projectRoot)
	if cfg.DisableSkillPicker {
		return nil
	}
	names := make([]string, len(m.skills))
	for i, s := range m.skills {
		names[i] = s.Name
	}
	return names
}

var programRef *tea.Program

func Run(projectRoot string) {
	m := NewModel(projectRoot)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	programRef = p
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	// Give SDK goroutines time to clean up subprocess after context cancellation.
	if fm, ok := finalModel.(Model); ok && fm.processCancels.HasRunning() {
		time.Sleep(500 * time.Millisecond)
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return programReadyMsg{p: programRef}
		},
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case programReadyMsg:
		m.program = msg.p
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case stopwatch.TickMsg, timer.TickMsg, timer.TimeoutMsg:
		// Reserved for future stopwatch/timer integration
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.form.Width = msg.Width
		// Update viewport dimensions for fullscreen scrolling
		headerHeight := 4 // header + spacing
		statusHeight := 3 // statusbar + spacing
		vpHeight := msg.Height - headerHeight - statusHeight
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width - 4 // account for padding
		m.viewport.Height = vpHeight
		if m.mode == model.ModeEditPlan || m.mode == model.ModeBulkAdd {
			m.editor.VH = msg.Height - 10
			m.editor.VW = msg.Width - 12
		}
		return m, nil

	case tea.MouseMsg:
		switch m.mode {
		case model.ModeEditPlan, model.ModeEditContext, model.ModeBulkAdd:
			var cmd tea.Cmd
			m.editor, cmd = m.editor.UpdateMouse(msg)
			return m, cmd
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

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

	case claude.StreamEventMsg:
		return m.handleStreamEvent(msg)

	case claude.StreamDoneMsg:
		return m.handleStreamDone(msg)

	case claude.StreamWaitingMsg:
		return m.handleStreamWaiting(msg)

	case claude.ChatSubmitMsg:
		return m.handleChatSubmit(msg)

	case chatCancelMsg:
		m.processAutoScroll = true
		m.mode = model.ModeProcessDetail
		return m, nil

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
	case FormSkillPickerMsg:
		if len(m.skills) == 0 {
			return m, flashCmd("No skills available")
		}
		items := make([]CheckItem, len(m.skills))
		for i, s := range m.skills {
			label := s.Name
			if s.Description != "" {
				label += "  " + s.Description
			}
			items[i] = CheckItem{Label: label, Value: s.Name}
		}
		m.multiCheck = NewMultiCheck("Select skills", items)
		// Pre-select already chosen skills
		for _, name := range m.form.skills {
			m.multiCheck.Selected[name] = true
		}
		m.returnMode = m.mode
		m.mode = model.ModeSkillPicker
		return m, nil
	case SelectSubmitMsg:
		return m.handleSelectSubmit(msg.Value)
	case SelectCancelMsg:
		// Restore previous theme if cancelling theme picker
		if m.mode == model.ModeThemePicker && m.previousTheme != "" {
			ApplyTheme(m.previousTheme)
			applyThemeToBubbles(&m)
		}
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil
	case MultiCheckSubmitMsg:
		if m.mode == model.ModeSkillPicker {
			m.form.skills = msg.Selected
			m.mode = model.ModeTaskForm
			return m, nil
		}
		return m.handleMultiCheckSubmit(msg.Selected)
	case MultiCheckCancelMsg:
		if m.mode == model.ModeSkillPicker {
			m.mode = model.ModeTaskForm
			return m, nil
		}
		m.mode = model.ModeList
		m.action = actionNone
		return m, nil
	case EditorSaveMsg:
		return m.handleEditorSave(msg.Content)
	case EditorCancelMsg:
		m.mode = m.returnMode
		m.action = actionNone
		return m, nil

	case filePickerResultMsg:
		content, err := readFileContent(msg.path)
		if err != nil {
			m.mode = model.ModeContextView
			return m, flashCmd(fmt.Sprintf("Error reading file: %s", err))
		}
		// Append imported content to existing context
		existing := store.LoadContext(m.projectRoot)
		separator := ""
		if existing != "" {
			separator = "\n\n---\n\n"
		}
		if err := store.SaveContext(m.projectRoot, existing+separator+content); err != nil {
			m.mode = model.ModeContextView
			return m, flashCmd(fmt.Sprintf("Error saving context: %s", err))
		}
		m.mode = model.ModeContextView
		return m, flashCmd(fmt.Sprintf("Imported: %s", msg.path))

	case dirPickerResultMsg:
		m.form.workDir.SetValue(msg.path)
		m.mode = model.ModeTaskForm
		return m, nil
	}

	// Forward non-key messages to the filepicker so it can receive readDirMsg
	if m.mode == model.ModeFilePicker || m.mode == model.ModeFormDirPicker {
		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd
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
	statusBar := renderStatusBar(m.helpModel, m.keys, m.mode, m.selectedItem, m.message, m.width)
	statusRendered := lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1).Render(statusBar)

	// Measure actual rendered heights to calculate available content space
	headerHeight := len(strings.Split(header, "\n"))
	statusHeight := len(strings.Split(statusRendered, "\n"))
	maxContentLines := m.height - headerHeight - statusHeight
	if maxContentLines < 5 {
		maxContentLines = 5
	}

	content := m.renderContent(maxContentLines)

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

// addProcess appends a process to the list with timing info.
func (m *Model) addProcess(proc *model.ClaudeProcess) {
	proc.StartedAt = time.Now()
	m.processes = append(m.processes, *proc)
	m.processPaginator.SetTotalPages(len(m.processes))
}

// processElapsed returns a human-readable elapsed time for a process.
func processElapsed(proc *model.ClaudeProcess) string {
	if proc.StartedAt.IsZero() {
		return ""
	}
	d := time.Since(proc.StartedAt)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m *Model) reload() {
	s, _ := store.LoadStore(m.projectRoot)
	m.store = s

	// Fix stuck "planning" tasks: if plan exists but no process is running, revert to pending
	for i := range s.Tasks {
		if s.Tasks[i].Status != model.StatusPlanning {
			continue
		}
		hasPlan := s.Tasks[i].PlanFile != "" && store.PlanExists(m.projectRoot, s.Tasks[i].PlanFile)
		running := false
		for _, p := range m.processes {
			if p.CompletionAction == model.CompletionSavePlan &&
				p.CompletionMeta["taskID"] == s.Tasks[i].ID &&
				p.Status == model.ProcessRunning {
				running = true
				break
			}
		}
		if !running && hasPlan {
			store.UpdateTask(m.projectRoot, s.Tasks[i].ID, map[string]interface{}{
				"status": string(model.StatusPending),
			})
			s.Tasks[i].Status = model.StatusPending
		}
	}

	m.rebuildList()
}

func (m *Model) rebuildList() {
	m.listItems = store.BuildListItems(m.store, m.filter, m.collapsedGroups, m.hideCompleted)
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

// activeProcessTaskIDs returns task IDs that have a running process.
func (m *Model) activeProcessTaskIDs() map[string]bool {
	ids := make(map[string]bool)
	for _, p := range m.processes {
		if p.Status == model.ProcessRunning {
			if taskID := p.CompletionMeta["taskID"]; taskID != "" {
				ids[taskID] = true
			}
		}
	}
	return ids
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

func (m Model) renderContent(contentHeight int) string {
	switch m.mode {
	case model.ModeAddTask, model.ModeEditTask, model.ModeFilter,
		model.ModeEditTags, model.ModeEditDescription, model.ModeTaskViewAsk,
		model.ModeGroupPrompt:
		return m.textInput.View()

	case model.ModeTaskForm:
		return m.form.View()

	case model.ModeAddToGroup, model.ModeThemePicker, model.ModeAgentPicker:
		return m.selectInput.View()

	case model.ModeCombineSelect, model.ModeSkillPicker:
		return m.multiCheck.View()

	case model.ModeCombineName:
		info := styleGray.Render("Combining plans from: " + strings.Join(m.combineSelectedIDs, ", "))
		return info + "\n" + m.textInput.View()

	case model.ModeConfirmDelete:
		return styleYellow.Render(m.confirmMsg) + " " + styleGray.Render("(y/n)")

	case model.ModeEditPlan, model.ModeEditContext, model.ModeBulkAdd:
		return m.editor.View()

	case model.ModePlan:
		content := renderPlanView(m.projectRoot, m.selectedTask(), m.selectedGroup(), m.width-8)
		m.viewport.SetContent(content)
		return m.viewport.View()

	case model.ModeGroupDetail:
		if g := m.selectedGroup(); g != nil {
			content := renderGroupView(g, m.store, m.projectRoot, m.groupProgress)
			m.viewport.SetContent(content)
			return m.viewport.View()
		}
		return ""

	case model.ModeTaskView:
		if t := m.selectedTask(); t != nil {
			content := renderTaskView(t, m.projectRoot, m.width-8)
			m.viewport.SetContent(content)
			return m.viewport.View()
		}
		return ""

	case model.ModeProcessDetail:
		if m.processIdx >= 0 && m.processIdx < len(m.processes) {
			content := renderRichProcessDetail(&m.processes[m.processIdx], m.width-8)
			wasAtBottom := m.viewport.AtBottom()
			m.viewport.SetContent(content)
			if m.processAutoScroll || wasAtBottom {
				m.viewport.GotoBottom()
			}
			return m.viewport.View()
		}
		return ""

	case model.ModeProcessChat:
		if m.processIdx >= 0 && m.processIdx < len(m.processes) {
			content := renderRichProcessDetail(&m.processes[m.processIdx], m.width-8)
			wasAtBottom := m.viewport.AtBottom()
			m.viewport.SetContent(content)
			if m.processAutoScroll || wasAtBottom {
				m.viewport.GotoBottom()
			}
			return m.viewport.View() + "\n" + m.chatInput.View()
		}
		return ""

	case model.ModeContextView:
		content := renderContextView(m.projectRoot)
		m.viewport.SetContent(content)
		return m.viewport.View()

	case model.ModeHelp:
		m.viewport.SetContent(renderHelp(m.helpModel, m.keys))
		return m.viewport.View()

	case model.ModeTableView:
		return renderTableView(m.taskTable)

	case model.ModeAllTasksList:
		return m.taskList.View()

	case model.ModeFilePicker:
		return renderFilePickerView(m.filePicker)

	case model.ModeFormDirPicker:
		return renderDirPickerView(m.filePicker)

	default:
		return m.renderListView()
	}
}

func (m Model) renderListView() string {
	hasProcesses := len(m.processes) > 0

	listHeight := m.height - 8
	activeTaskIDs := m.activeProcessTaskIDs()
	listPanel := renderListPanel(m.store, m.projectRoot, m.listItems, m.listIndex,
		m.focusPanel == model.FocusMain, listHeight, m.collapsedGroups, m.listScrollOffset, m.spinner.View(), activeTaskIDs)

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

	detailPanel := renderDetailPanel(m.store, m.projectRoot, m.selectedItem, detailWidth, m.groupProgress)
	sep := lipgloss.NewStyle().PaddingLeft(2).PaddingRight(2).Render(verticalSeparator(listHeight))
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, sep, detailPanel)

	if hasProcesses {
		processPanel := renderProcessPanel(m.processes, m.processIdx, m.focusPanel == model.FocusProcesses, m.processPaginator)
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
		// Ctrl+B on WorkDir field opens directory picker
		if msg.Type == tea.KeyCtrlB && m.form.Active == fieldWorkDir {
			startDir := m.projectRoot
			if v := m.form.workDir.Value(); v != "" {
				startDir = v
			}
			m.filePicker = newDirPicker(startDir)
			m.mode = model.ModeFormDirPicker
			return m, m.filePicker.Init()
		}
		prevActive := m.form.Active
		var cmd tea.Cmd
		m.form, cmd = m.form.Update(msg)
		// Auto-open dir picker when tabbing into WorkDir field
		if m.form.Active == fieldWorkDir && prevActive != fieldWorkDir {
			startDir := m.projectRoot
			if v := m.form.workDir.Value(); v != "" {
				startDir = v
			}
			m.filePicker = newDirPicker(startDir)
			m.mode = model.ModeFormDirPicker
			return m, m.filePicker.Init()
		}
		return m, cmd

	case model.ModeAddToGroup, model.ModeThemePicker, model.ModeAgentPicker:
		var cmd tea.Cmd
		m.selectInput, cmd = m.selectInput.Update(msg)
		// Live preview: apply theme as user navigates
		if m.mode == model.ModeThemePicker && m.selectInput.Index < len(m.selectInput.Items) {
			ApplyTheme(m.selectInput.Items[m.selectInput.Index].Value)
			applyThemeToBubbles(&m)
		}
		return m, cmd

	case model.ModeCombineSelect, model.ModeSkillPicker:
		var cmd tea.Cmd
		m.multiCheck, cmd = m.multiCheck.Update(msg)
		return m, cmd

	case model.ModeConfirmDelete:
		return m.handleConfirm(msg)

	case model.ModeEditPlan, model.ModeEditContext, model.ModeBulkAdd:
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		return m, cmd

	case model.ModeProcessChat:
		var cmd tea.Cmd
		m.chatInput, cmd = m.chatInput.Update(msg)
		return m, cmd

	case model.ModeTableView:
		return m.handleTableKey(msg)

	case model.ModeAllTasksList:
		return m.handleListKey(msg)

	case model.ModeFilePicker:
		return m.handleFilePickerKey(msg)

	case model.ModeFormDirPicker:
		return m.handleDirPickerKey(msg)
	}

	return m.handleNavKey(msg)
}

func (m Model) handleTableKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape || (msg.Type == tea.KeyRunes && string(msg.Runes) == "T"):
		m.mode = model.ModeList
		return m, nil
	case msg.Type == tea.KeyEnter:
		row := m.taskTable.SelectedRow()
		if row != nil && len(row) >= 2 {
			taskID := row[1]
			// Find task and select it
			for i, item := range m.listItems {
				if item.Kind == model.ListItemTask && item.Task != nil && item.Task.ID == taskID {
					m.listIndex = i
					m.updateSelectedItem()
					m.viewport.GotoTop()
					m.mode = model.ModeTaskView
					return m, nil
				}
			}
		}
	}
	var cmd tea.Cmd
	m.taskTable, cmd = m.taskTable.Update(msg)
	return m, cmd
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEscape:
		m.mode = model.ModeList
		return m, nil
	case msg.Type == tea.KeyEnter:
		if item, ok := m.taskList.SelectedItem().(TaskListItem); ok {
			for i, li := range m.listItems {
				if li.Kind == model.ListItemTask && li.Task != nil && li.Task.ID == item.task.ID {
					m.listIndex = i
					m.updateSelectedItem()
					m.viewport.GotoTop()
					m.mode = model.ModeTaskView
					return m, nil
				}
			}
		}
	}
	var cmd tea.Cmd
	m.taskList, cmd = m.taskList.Update(msg)
	return m, cmd
}

func (m Model) isFullscreenMode() bool {
	switch m.mode {
	case model.ModePlan, model.ModeTaskView, model.ModeGroupDetail, model.ModeProcessDetail, model.ModeContextView, model.ModeHelp:
		return true
	}
	return false
}

// syncViewportContent sets the viewport content for the current fullscreen mode
// so that scroll operations in Update have accurate line counts.
func (m *Model) syncViewportContent() {
	switch m.mode {
	case model.ModePlan:
		m.viewport.SetContent(renderPlanView(m.projectRoot, m.selectedTask(), m.selectedGroup(), m.width-8))
	case model.ModeTaskView:
		if t := m.selectedTask(); t != nil {
			m.viewport.SetContent(renderTaskView(t, m.projectRoot, m.width-8))
		}
	case model.ModeGroupDetail:
		if g := m.selectedGroup(); g != nil {
			m.viewport.SetContent(renderGroupView(g, m.store, m.projectRoot, m.groupProgress))
		}
	case model.ModeProcessDetail, model.ModeProcessChat:
		if m.processIdx < len(m.processes) {
			m.viewport.SetContent(renderRichProcessDetail(&m.processes[m.processIdx], m.width-8))
		}
	case model.ModeContextView:
		m.viewport.SetContent(renderContextView(m.projectRoot))
	case model.ModeHelp:
		m.viewport.SetContent(renderHelp(m.helpModel, m.keys))
	}
}

func (m Model) handleNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	// Clear pending g on any non-g key
	if k != "g" {
		m.pendingG = false
	}

	if k == "ctrl+c" || (key.Matches(msg, m.keys.Quit) && m.mode == model.ModeList) {
		m.processCancels.CancelAll()
		return m, tea.Quit
	}

	if key.Matches(msg, m.keys.Help) {
		if m.mode == model.ModeHelp {
			m.mode = model.ModeList
		} else {
			m.mode = model.ModeHelp
			m.viewport.GotoTop()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Back) && m.mode != model.ModeList {
		m.mode = model.ModeList
		m.viewport.GotoTop()
		return m, nil
	}

	// Scroll in fullscreen views (including help) via viewport
	if m.isFullscreenMode() {
		m.syncViewportContent()
		isProcess := m.mode == model.ModeProcessDetail || m.mode == model.ModeProcessChat
		halfPage := max(1, m.viewport.Height/2)
		switch {
		case key.Matches(msg, m.keys.ScrollDown):
			m.viewport.LineDown(1)
			if isProcess && !m.viewport.AtBottom() {
				m.processAutoScroll = false
			}
			return m, nil
		case key.Matches(msg, m.keys.ScrollUp):
			m.viewport.LineUp(1)
			if isProcess {
				m.processAutoScroll = false
			}
			return m, nil
		case key.Matches(msg, m.keys.HalfPageDown):
			m.viewport.LineDown(halfPage)
			if isProcess && !m.viewport.AtBottom() {
				m.processAutoScroll = false
			}
			return m, nil
		case key.Matches(msg, m.keys.HalfPageUp):
			m.viewport.LineUp(halfPage)
			if isProcess {
				m.processAutoScroll = false
			}
			return m, nil
		case key.Matches(msg, m.keys.GotoBottom):
			m.viewport.GotoBottom()
			if isProcess {
				m.processAutoScroll = true
			}
			return m, nil
		case key.Matches(msg, m.keys.GotoTop):
			if m.pendingG {
				m.pendingG = false
				m.viewport.GotoTop()
				if isProcess {
					m.processAutoScroll = false
				}
			} else {
				m.pendingG = true
			}
			return m, nil
		}
		// Let viewport handle mouse wheel
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}

	// Help mode only responds to scroll, ?, Esc, Ctrl+C (handled above)
	if m.mode == model.ModeHelp {
		return m, nil
	}

	// Process panel pagination
	if key.Matches(msg, m.keys.PrevPage) && m.focusPanel == model.FocusProcesses {
		m.processPaginator.PrevPage()
		return m, nil
	}
	if key.Matches(msg, m.keys.NextPage) && m.focusPanel == model.FocusProcesses {
		m.processPaginator.NextPage()
		return m, nil
	}

	if key.Matches(msg, m.keys.Up) {
		if m.focusPanel == model.FocusMain && len(m.listItems) > 0 {
			m.listIndex = max(0, m.listIndex-1)
			m.updateSelectedItem()
		} else if m.focusPanel == model.FocusProcesses && len(m.processes) > 0 {
			m.processIdx = max(0, m.processIdx-1)
		}
		return m, nil
	}
	if key.Matches(msg, m.keys.Down) {
		if m.focusPanel == model.FocusMain && len(m.listItems) > 0 {
			m.listIndex = min(len(m.listItems)-1, m.listIndex+1)
			m.updateSelectedItem()
		} else if m.focusPanel == model.FocusProcesses && len(m.processes) > 0 {
			m.processIdx = min(len(m.processes)-1, m.processIdx+1)
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.Tab) {
		if m.focusPanel == model.FocusMain && len(m.processes) > 0 {
			m.focusPanel = model.FocusProcesses
		} else {
			m.focusPanel = model.FocusMain
		}
		m.updateSelectedItem()
		return m, nil
	}

	if key.Matches(msg, m.keys.Collapse) && m.mode == model.ModeList && m.focusPanel == model.FocusMain {
		if g := m.selectedGroup(); g != nil {
			m.collapsedGroups[g.ID] = !m.collapsedGroups[g.ID]
			m.rebuildList()
			return m, nil
		}
	}

	if key.Matches(msg, m.keys.Enter) && (m.mode == model.ModeList || m.mode == model.ModeDetail) {
		if m.focusPanel == model.FocusMain {
			if m.selectedTask() != nil {
				m.viewport.GotoTop()
				m.mode = model.ModeTaskView
				return m, nil
			} else if m.selectedGroup() != nil {
				m.viewport.GotoTop()
				m.mode = model.ModeGroupDetail
			}
		} else if m.focusPanel == model.FocusProcesses && m.processIdx < len(m.processes) {
			m.processAutoScroll = true
			m.mode = model.ModeProcessDetail
			m.viewport.GotoBottom()
		}
		return m, nil
	}

	if key.Matches(msg, m.keys.View) && (m.mode == model.ModeList || m.mode == model.ModeDetail) {
		if m.selectedTask() != nil {
			m.viewport.GotoTop()
			m.mode = model.ModeTaskView
			return m, nil
		}
		if g := m.selectedGroup(); g != nil && g.PlanFile != "" {
			m.viewport.GotoTop()
			m.mode = model.ModePlan
			return m, nil
		}
	}

	if key.Matches(msg, m.keys.Prompt) && m.mode == model.ModeTaskView && m.selectedTask() != nil {
		m.action = actionFollowUp
		m.actionTaskID = m.selectedTask().ID
		m.textInput = NewTextInput("Question for Claude", "")
		m.mode = model.ModeTaskViewAsk
		return m, nil
	}

	if key.Matches(msg, m.keys.OpenProof) && (m.mode == model.ModeTaskView || m.mode == model.ModeList || m.mode == model.ModeDetail) {
		if t := m.selectedTask(); t != nil && t.IsProof() {
			if store.ProofExists(m.projectRoot, t) {
				if err := openProofInBrowser(m.projectRoot, t); err != nil {
					return m, flashCmd("Error opening proof: " + err.Error())
				}
				return m, flashCmd("Opening proof comparison...")
			}
			return m, flashCmd("No proof screenshots yet")
		}
	}

	if key.Matches(msg, m.keys.Cancel) && m.processIdx < len(m.processes) {
		if m.focusPanel == model.FocusProcesses || m.mode == model.ModeProcessDetail {
			proc := &m.processes[m.processIdx]
			if proc.Status == model.ProcessRunning {
				if m.processCancels.Cancel(proc.ID) {
					return m, flashCmd("Interrupting " + proc.Label + "...")
				}
				return m, flashCmd("Could not cancel process")
			}
			// For waiting processes with a live subprocess, end the conversation
			if proc.Status == model.ProcessWaiting && m.processInputs.Has(proc.ID) {
				m.processInputs.Close(proc.ID)
				m.processCancels.Cancel(proc.ID)
				return m, flashCmd("Ending conversation...")
			}
			// Remove non-running processes from the list
			m.processes = append(m.processes[:m.processIdx], m.processes[m.processIdx+1:]...)
			if m.processIdx >= len(m.processes) {
				m.processIdx = max(0, len(m.processes)-1)
			}
			if len(m.processes) == 0 {
				m.focusPanel = model.FocusMain
				m.mode = model.ModeList
			} else if m.mode == model.ModeProcessDetail {
				m.mode = model.ModeList
			}
			return m, flashCmd("Removed process")
		}
	}

	if key.Matches(msg, m.keys.OpenFull) && m.processIdx < len(m.processes) {
		if m.focusPanel == model.FocusProcesses || m.mode == model.ModeProcessDetail {
			proc := &m.processes[m.processIdx]
			if proc.Status == model.ProcessRunning {
				return m, flashCmd("Still running — press Enter to view output")
			}
			return m, claude.ExecContinue(m.projectRoot, proc.SessionID)
		}
	}

	if key.Matches(msg, m.keys.Chat) && m.mode == model.ModeProcessDetail && m.processIdx < len(m.processes) {
		proc := &m.processes[m.processIdx]
		if proc.Status == model.ProcessRunning {
			// Allow queuing a message while running — it will auto-send when the turn completes
			m.chatInput = NewChatInput(proc.ID, proc.SessionID)
			m.chatInput.inner.Placeholder = "Queue message (sent when turn completes)..."
			m.mode = model.ModeProcessChat
			return m, nil
		}
		if proc.SessionID == "" {
			return m, flashCmd("No session — can't follow up")
		}
		m.chatInput = NewChatInput(proc.ID, proc.SessionID)
		m.mode = model.ModeProcessChat
		return m, nil
	}

	if key.Matches(msg, m.keys.Add) && m.mode == model.ModeList {
		m.action = actionAddTask
		m.form = NewForm("New Task", nil, m.width, m.skillNames())
		m.mode = model.ModeTaskForm
		return m, nil
	}

	if key.Matches(msg, m.keys.BulkAdd) && m.mode == model.ModeList {
		m.action = actionBulkAdd
		m.returnMode = model.ModeList
		m.editor = NewEditor("Bulk Add Tasks", "", m.height-10, m.width-12)
		m.mode = model.ModeBulkAdd
		return m, nil
	}

	if key.Matches(msg, m.keys.Edit) {
		return m.handleEdit()
	}

	if key.Matches(msg, m.keys.Delete) && (m.mode == model.ModeList || m.mode == model.ModeDetail) {
		return m.handleDelete()
	}

	if key.Matches(msg, m.keys.CycleStatus) && m.selectedTask() != nil {
		return m.cycleStatus()
	}

	if key.Matches(msg, m.keys.AssignGroup) && m.mode == model.ModeList {
		return m.handleAssignGroup()
	}

	if key.Matches(msg, m.keys.Run) {
		return m.handleRun()
	}

	if key.Matches(msg, m.keys.Plan) && (m.mode == model.ModeList || m.mode == model.ModeDetail || m.mode == model.ModeTaskView || m.mode == model.ModePlan) {
		return m.handlePlan()
	}

	if key.Matches(msg, m.keys.Filter) && m.mode == model.ModeList {
		m.action = actionFilter
		m.textInput = NewTextInput("Filter", m.filter)
		m.mode = model.ModeFilter
		return m, nil
	}

	if key.Matches(msg, m.keys.HideDone) && m.mode == model.ModeList {
		m.hideCompleted = !m.hideCompleted
		m.rebuildList()
		if m.hideCompleted {
			return m, flashCmd("Hiding completed tasks")
		}
		return m, flashCmd("Showing all tasks")
	}

	if key.Matches(msg, m.keys.Merge) && m.mode == model.ModeList {
		return m.handleCombine()
	}

	if key.Matches(msg, m.keys.Prompt) && m.mode == model.ModeList && m.focusPanel == model.FocusMain {
		return m.handleGroupPrompt()
	}

	if key.Matches(msg, m.keys.Context) && m.mode == model.ModeList {
		m.viewport.GotoTop()
		m.mode = model.ModeContextView
		return m, nil
	}

	if key.Matches(msg, m.keys.FilePick) && m.mode == model.ModeContextView {
		m.filePicker = newFilePicker(m.projectRoot)
		m.mode = model.ModeFilePicker
		return m, m.filePicker.Init()
	}

	if key.Matches(msg, m.keys.Theme) && m.mode == model.ModeList {
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

	if key.Matches(msg, m.keys.TableView) && m.mode == model.ModeList {
		tableHeight := m.height - 10
		if tableHeight < 5 {
			tableHeight = 5
		}
		m.taskTable = newTaskTable(m.store.Tasks, m.width-8, tableHeight)
		m.mode = model.ModeTableView
		return m, nil
	}

	if key.Matches(msg, m.keys.ListView) && m.mode == model.ModeList {
		m.taskList = newTaskList(m.store.Tasks, m.width-8, m.height-8)
		m.mode = model.ModeAllTasksList
		return m, nil
	}

	return m, nil
}

func (m Model) handleConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "y" || key == "enter" {
		if m.action == actionRegenPlan {
			m.action = actionNone
			if m.confirmIsGroup {
				g := store.FindGroup(m.store, m.confirmGroupID)
				if g == nil {
					m.mode = model.ModeList
					return m, nil
				}
				if len(m.agents) > 0 {
					return m.showAgentPicker(actionAgentPlan)
				}
				return m.spawnGroupPlanGeneration(g, nil)
			}
			t := store.FindTask(m.store, m.confirmTaskID)
			if t == nil {
				m.mode = model.ModeList
				return m, nil
			}
			if len(m.agents) > 0 {
				return m.showAgentPicker(actionAgentPlan)
			}
			return m.spawnPlanGeneration(t, nil)
		}
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
		if m.action == actionRegenPlan {
			m.action = actionNone
			m.mode = model.ModePlan
		} else {
			m.mode = model.ModeList
		}
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

	case actionRenameGroup:
		store.RenameGroup(m.projectRoot, m.actionTaskID, value)
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd(fmt.Sprintf("Renamed to \"%s\"", value))
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
		t, _ := store.AddTask(m.projectRoot, data.Title, data.Description, tags, "", data.WorkDir)
		if t != nil && len(data.Skills) > 0 {
			store.UpdateTask(m.projectRoot, t.ID, map[string]interface{}{"skills": data.Skills})
		}
		m.reload()
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Task added")

	case actionEditTask:
		store.UpdateTask(m.projectRoot, m.actionTaskID, map[string]interface{}{
			"title":       data.Title,
			"description": data.Description,
			"tags":        tags,
			"workDir":     data.WorkDir,
			"skills":      data.Skills,
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
	if m.mode == model.ModeAgentPicker {
		switch m.action {
		case actionAgentPlan:
			var a *agent.Agent
			if value != "__default__" {
				a = m.agentByName(value)
			}
			m.mode = model.ModeList
			m.action = actionNone
			if t := m.selectedTask(); t != nil {
				return m.spawnPlanGeneration(t, a)
			}
			if g := m.selectedGroup(); g != nil {
				return m.spawnGroupPlanGeneration(g, a)
			}
		case actionAgentRun:
			var a *agent.Agent
			if value != "__default__" {
				a = m.agentByName(value)
			}
			m.actionAgent = a
			return m.showRunModePicker()
		case actionRunMode:
			a := m.actionAgent
			m.actionAgent = nil
			m.action = actionNone
			m.mode = model.ModeList
			switch value {
			case "terminal":
				return m.executeRun(a)
			case "background":
				return m.spawnBackgroundRun(a)
			}
		}
		m.action = actionNone
		m.mode = model.ModeList
		return m, nil
	}

	if m.mode == model.ModeThemePicker {
		ApplyTheme(value)
		applyThemeToBubbles(&m)
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
	m.textInput.SetPlaceholder("e.g. auth-full-plan")
	m.mode = model.ModeCombineName
	return m, nil
}

func (m Model) handleEditorSave(content string) (tea.Model, tea.Cmd) {
	if m.action == actionEditContext {
		store.SaveContext(m.projectRoot, content)
		m.mode = m.returnMode
		m.action = actionNone
		return m, flashCmd("Context saved")
	}
	if m.action == actionBulkAdd {
		return m.spawnBulkAdd(content)
	}
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

// --- Bulk add ---

func (m Model) spawnBulkAdd(bulkText string) (tea.Model, tea.Cmd) {
	bulkText = strings.TrimSpace(bulkText)
	if bulkText == "" {
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("No text to process")
	}

	// Determine group context: if a group is selected, assign tasks to it
	groupID := ""
	if g := m.selectedGroup(); g != nil {
		groupID = g.ID
	}

	promptText := prompt.BuildBulkAddPrompt(m.projectRoot, bulkText, m.store.Groups, groupID)

	label := "Bulk Add Tasks"
	if m.runningLabels[label] {
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Already running...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("bulk-add-%d", time.Now().Unix())
	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Parsing tasks...",
		CompletionAction: model.CompletionApplyBulkAdd,
		CompletionMeta:   map[string]string{"groupID": groupID},
	}
	m.addProcess(&proc)

	m.mode = model.ModeList
	m.action = actionNone

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnStreamCmd(m.program, m.projectRoot, procID, promptText, m.processCancels, m.processTimeout(), claude.CLIOptions{PermissionMode: "plan"}, nil)
	} else {
		projectRoot := m.projectRoot
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			applyBulkAddResult(projectRoot, output, groupID)
			return claude.ProcessDoneMsg{ID: procID, Output: output, LogFile: logFile}
		}
	}

	return m, tea.Batch(cmd, flashCmd("Processing bulk add..."))
}

type bulkAddResult struct {
	Tasks []struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	} `json:"tasks"`
	Summary string `json:"summary"`
}

func applyBulkAddResult(projectRoot string, output string, groupID string) {
	cleaned := extractJSON(output)
	var result bulkAddResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return
	}
	for _, t := range result.Tasks {
		if t.Title == "" {
			continue
		}
		store.AddTask(projectRoot, t.Title, t.Description, t.Tags, groupID)
	}
}

// --- Action initiators ---

func (m Model) handleEdit() (tea.Model, tea.Cmd) {
	if m.mode == model.ModeContextView {
		content := store.LoadContext(m.projectRoot)
		m.action = actionEditContext
		m.returnMode = model.ModeContextView
		m.editor = NewEditor("Edit Project Context", content, m.height-10, m.width-12)
		m.mode = model.ModeEditContext
		return m, nil
	}

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
		if m.mode == model.ModeTaskView && t.PlanFile != "" {
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
				WorkDir:     t.WorkDir,
				Skills:      t.Skills,
			}
			m.form = NewForm(fmt.Sprintf("Edit Task — %s", t.ID), initial, m.width, m.skillNames())
			m.mode = model.ModeTaskForm
		}
		return m, nil
	}

	if g := m.selectedGroup(); g != nil && (m.mode == model.ModeList || m.mode == model.ModeGroupDetail) {
		m.action = actionRenameGroup
		m.actionTaskID = g.ID // reuse for group ID
		m.textInput = NewTextInput("Rename Project", g.Name)
		m.mode = model.ModeAddTask // reuse text input mode
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
	if t.Status == model.StatusMerged {
		return m, flashCmd("Cannot change status of merged task")
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
	if m.selectedTask() == nil && m.selectedGroup() == nil {
		return m, nil
	}
	if len(m.agents) > 0 {
		return m.showAgentPicker(actionAgentRun)
	}
	m.actionAgent = nil
	return m.showRunModePicker()
}

func (m Model) showRunModePicker() (tea.Model, tea.Cmd) {
	items := []SelectItem{
		{Label: "New terminal", Value: "terminal"},
		{Label: "Background process", Value: "background"},
	}
	m.selectInput = NewSelectInput("Run mode", items)
	m.action = actionRunMode
	m.mode = model.ModeAgentPicker
	return m, nil
}

func (m Model) executeRun(a *agent.Agent) (tea.Model, tea.Cmd) {
	var systemPrompt string
	var workDir string
	if t := m.selectedTask(); t != nil {
		systemPrompt = prompt.BuildTaskPrompt(m.projectRoot, t)
		workDir = store.ResolveWorkDir(m.projectRoot, m.store, t)
		store.UpdateTask(m.projectRoot, t.ID, map[string]interface{}{"status": string(model.StatusInProgress)})
		m.reload()
	} else if g := m.selectedGroup(); g != nil {
		systemPrompt = prompt.BuildGroupPrompt(m.projectRoot, g, m.store)
		workDir = store.ResolveGroupWorkDir(m.projectRoot, m.store, g)
	} else {
		return m, nil
	}
	if a != nil && a.SystemPrompt != "" {
		systemPrompt = a.SystemPrompt + "\n\n---\n\n" + systemPrompt
	}
	if t := m.selectedTask(); t != nil && len(t.Skills) > 0 {
		for _, name := range t.Skills {
			if s := m.skillByName(name); s != nil && s.SystemPrompt != "" {
				systemPrompt = systemPrompt + "\n\n---\n\n## Skill: " + s.Name + "\n\n" + s.SystemPrompt
			}
		}
	}
	return m, tea.Batch(
		claude.SpawnInTerminal(workDir, systemPrompt),
		flashCmd("Opening claude in new terminal..."),
	)
}

func captureHeadCommit(workDir string) string {
	out, err := exec.Command("git", "-C", workDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (m Model) spawnBackgroundRun(a *agent.Agent) (tea.Model, tea.Cmd) {
	var title, id, promptText string
	var workDir string
	isTask := false
	isProof := false

	if t := m.selectedTask(); t != nil {
		title = t.Title
		id = t.ID
		isTask = true
		isProof = t.IsProof()
		promptText = prompt.BuildTaskPrompt(m.projectRoot, t)
		workDir = store.ResolveWorkDir(m.projectRoot, m.store, t)
	} else if g := m.selectedGroup(); g != nil {
		title = g.Name
		id = g.ID
		promptText = prompt.BuildGroupPrompt(m.projectRoot, g, m.store)
		workDir = store.ResolveGroupWorkDir(m.projectRoot, m.store, g)
	} else {
		return m, nil
	}

	label := "Run: " + title
	if m.runningLabels[label] {
		return m, flashCmd("Already running...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("run-%s-%d", id, time.Now().Unix())
	proc := model.ClaudeProcess{
		ID:            procID,
		Label:         label,
		Status:        model.ProcessRunning,
		Output:        "Waiting for claude...",
		IsInteractive: true,
	}
	if isTask {
		proc.CompletionAction = model.CompletionRunTask
		proc.CompletionMeta = map[string]string{"taskID": id}

		if isProof {
			preChangeCommit := captureHeadCommit(workDir)
			proc.CompletionMeta["isProof"] = "true"
			proc.CompletionMeta["preChangeCommit"] = preChangeCommit
			promptText += "\n\n" + prompt.BuildProofInstructions(m.projectRoot, id, preChangeCommit)
		}

		store.UpdateTask(m.projectRoot, id, map[string]interface{}{"status": string(model.StatusInProgress)})
		m.reload()
	}
	m.addProcess(&proc)

	opts := buildCLIOptsFromAgent(a)
	opts.PermissionMode = "bypassPermissions"
	if isTask {
		if t := m.selectedTask(); t != nil && len(t.Skills) > 0 {
			appendSkillPrompts(&opts, m.skills, t.Skills)
		}
	}
	if isProof {
		opts.Model = "sonnet"
	}

	var cmd tea.Cmd
	if m.program != nil {
		inputCh := make(chan string, 1)
		m.processInputs.Register(procID, inputCh)
		cmd = claude.SpawnStreamCmd(m.program, workDir, procID, promptText, m.processCancels, m.processTimeout(), opts, inputCh)
	} else {
		return m, flashCmd("Streaming not available (no program ref)")
	}

	return m, tea.Batch(cmd, flashCmd(fmt.Sprintf("Running %s in background...", title)))
}

func (m Model) handlePlan() (tea.Model, tea.Cmd) {
	alreadyViewing := m.mode == model.ModePlan

	if t := m.selectedTask(); t != nil {
		if t.PlanFile != "" && !alreadyViewing {
			m.viewport.GotoTop()
			m.mode = model.ModePlan
			return m, nil
		}
		if t.PlanFile != "" && alreadyViewing {
			// Confirm before regenerating existing plan
			m.confirmMsg = fmt.Sprintf("Regenerate plan for \"%s\"?", t.Title)
			m.confirmTaskID = t.ID
			m.confirmIsGroup = false
			m.action = actionRegenPlan
			m.mode = model.ModeConfirmDelete
			return m, nil
		}
		if len(m.agents) > 0 {
			return m.showAgentPicker(actionAgentPlan)
		}
		return m.spawnPlanGeneration(t, nil)
	}
	if g := m.selectedGroup(); g != nil {
		if g.PlanFile != "" && !alreadyViewing {
			m.viewport.GotoTop()
			m.mode = model.ModePlan
			return m, nil
		}
		if g.PlanFile != "" && alreadyViewing {
			m.confirmMsg = fmt.Sprintf("Regenerate plan for \"%s\"?", g.Name)
			m.confirmGroupID = g.ID
			m.confirmIsGroup = true
			m.action = actionRegenPlan
			m.mode = model.ModeConfirmDelete
			return m, nil
		}
		if len(m.agents) > 0 {
			return m.showAgentPicker(actionAgentPlan)
		}
		return m.spawnGroupPlanGeneration(g, nil)
	}
	return m, nil
}

func (m Model) showAgentPicker(action actionContext) (tea.Model, tea.Cmd) {
	items := []SelectItem{{Label: "Default (no agent)", Value: "__default__"}}
	for _, a := range m.agents {
		label := a.Name
		if a.Description != "" {
			label += "  " + a.Description
		}
		items = append(items, SelectItem{Label: label, Value: a.Name})
	}
	m.selectInput = NewSelectInput("Select agent", items)
	m.action = action
	m.mode = model.ModeAgentPicker
	return m, nil
}

func (m Model) agentByName(name string) *agent.Agent {
	for i := range m.agents {
		if m.agents[i].Name == name {
			return &m.agents[i]
		}
	}
	return nil
}

func buildCLIOptsFromAgent(a *agent.Agent) claude.CLIOptions {
	var opts claude.CLIOptions
	if a == nil {
		return opts
	}
	if a.SystemPrompt != "" {
		opts.AppendSystemPrompt = a.SystemPrompt
	}
	if a.Model != "" && a.Model != "inherit" {
		opts.Model = a.Model
	}
	return opts
}

func appendSkillPrompts(opts *claude.CLIOptions, skills []skill.Skill, taskSkillNames []string) {
	if len(taskSkillNames) == 0 {
		return
	}
	byName := make(map[string]*skill.Skill, len(skills))
	for i := range skills {
		byName[skills[i].Name] = &skills[i]
	}
	var prompts []string
	for _, name := range taskSkillNames {
		if s, ok := byName[name]; ok && s.SystemPrompt != "" {
			prompts = append(prompts, fmt.Sprintf("## Skill: %s\n\n%s", s.Name, s.SystemPrompt))
		}
	}
	if len(prompts) == 0 {
		return
	}
	combined := strings.Join(prompts, "\n\n---\n\n")
	if opts.AppendSystemPrompt != "" {
		opts.AppendSystemPrompt += "\n\n---\n\n" + combined
	} else {
		opts.AppendSystemPrompt = combined
	}
}

func (m Model) skillByName(name string) *skill.Skill {
	for i := range m.skills {
		if m.skills[i].Name == name {
			return &m.skills[i]
		}
	}
	return nil
}

func (m Model) handleCombine() (tea.Model, tea.Cmd) {
	// Determine scope: if cursor is on a group or task within a group, scope to that group
	var scopeGroupID string
	var scopeLabel string
	if m.selectedAllTasks() {
		scopeGroupID = ""
		scopeLabel = ""
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
		}
	}

	// Build set of group IDs in scope (the group + all descendants)
	scopeGroups := map[string]bool{}
	if scopeGroupID != "" {
		scopeGroups[scopeGroupID] = true
		for _, id := range store.GetAllDescendantGroupIDs(m.store, scopeGroupID) {
			scopeGroups[id] = true
		}
	}

	inScope := func(t model.Task) bool {
		if t.Status == model.StatusMerged {
			return false
		}
		if scopeGroupID == "" {
			return true
		}
		return scopeGroups[t.Group]
	}

	withPlans := store.GetTasksWithPlans(m.projectRoot, m.store)

	planCount := 0
	for _, wp := range withPlans {
		if inScope(wp) {
			planCount++
		}
	}
	if planCount < 2 {
		return m, flashCmd("Need at least 2 tasks with plans to combine")
	}

	var items []CheckItem
	for _, t := range m.store.Tasks {
		if !inScope(t) {
			continue
		}
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

	title := "Select tasks to combine plans:"
	if scopeLabel != "" {
		title = fmt.Sprintf("Select tasks to combine plans (%s):", scopeLabel)
	}
	m.multiCheck = NewMultiCheck(title, items)
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
	m.textInput.SetPlaceholder("e.g. fill out tags, regroup tasks, review descriptions...")
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
	WorkDir     string   `json:"workDir,omitempty"`
}

type groupActionGroup struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentGroup string `json:"parentGroup"`
	WorkDir     string `json:"workDir,omitempty"`
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

	promptText := prompt.BuildGroupActionPrompt(m.projectRoot, scopeTasks, m.store.Groups, scopeLabel, instruction)

	// Resolve workDir: for "all tasks" scope use projectRoot, otherwise resolve from the group
	workDir := m.projectRoot
	if scopeGroupID != "" && scopeGroupID != "__all__" {
		if g := store.FindGroup(m.store, scopeGroupID); g != nil {
			workDir = store.ResolveGroupWorkDir(m.projectRoot, m.store, g)
		}
	}

	label := "Prompt: " + scopeLabel
	if m.runningLabels[label] {
		m.mode = model.ModeList
		m.action = actionNone
		return m, flashCmd("Already running...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("group-prompt-%d", time.Now().Unix())
	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Processing tasks...",
		CompletionAction: model.CompletionApplyGroupAction,
	}
	m.addProcess(&proc)

	m.mode = model.ModeList
	m.action = actionNone

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnStreamCmd(m.program, workDir, procID, promptText, m.processCancels, m.processTimeout(), claude.CLIOptions{PermissionMode: "plan"}, nil)
	} else {
		projectRoot := m.projectRoot
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
			WorkDir:     ng.WorkDir,
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
		t.WorkDir = ut.WorkDir
		t.Updated = model.Now()
	}

	store.SaveStore(projectRoot, s)
}

// --- Process management ---

func (m Model) spawnPlanGeneration(task *model.Task, a *agent.Agent) (tea.Model, tea.Cmd) {
	label := "Plan: " + task.Title
	if m.runningLabels[label] {
		return m, flashCmd("Already generating...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("plan-%s-%d", task.ID, time.Now().Unix())
	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Waiting for claude...",
		IsInteractive:    true,
		CompletionAction: model.CompletionSavePlan,
		CompletionMeta: map[string]string{
			"taskID":    task.ID,
			"taskTitle": task.Title,
		},
	}
	m.addProcess(&proc)

	// Set task status to planning
	store.UpdateTask(m.projectRoot, task.ID, map[string]interface{}{"status": string(model.StatusPlanning)})
	m.reload()

	workDir := store.ResolveWorkDir(m.projectRoot, m.store, task)
	promptText := prompt.BuildPlanGenerationPrompt(m.projectRoot, task)

	opts := buildCLIOptsFromAgent(a)
	opts.PermissionMode = "plan"
	if task != nil && len(task.Skills) > 0 {
		appendSkillPrompts(&opts, m.skills, task.Skills)
	}

	var cmd tea.Cmd
	if m.program != nil {
		inputCh := make(chan string, 1)
		m.processInputs.Register(procID, inputCh)
		cmd = claude.SpawnStreamCmd(m.program, workDir, procID, promptText, m.processCancels, m.processTimeout(), opts, inputCh)
	} else {
		projectRoot := m.projectRoot
		taskID := task.ID
		taskTitle := task.Title
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

	return m, tea.Batch(cmd, flashCmd(fmt.Sprintf("Generating plan for %s...", task.ID)))
}

func (m Model) spawnGroupPlanGeneration(group *model.Group, a *agent.Agent) (tea.Model, tea.Cmd) {
	label := "Plan: " + group.Name
	if m.runningLabels[label] {
		return m, flashCmd("Already generating...")
	}
	m.runningLabels[label] = true

	procID := fmt.Sprintf("plan-%s-%d", group.ID, time.Now().Unix())
	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Waiting for claude...",
		IsInteractive:    true,
		CompletionAction: model.CompletionSaveGroupPlan,
		CompletionMeta:   map[string]string{"groupID": group.ID},
	}
	m.addProcess(&proc)

	tasks := store.GetTasksForGroup(m.store, group.ID)
	workDir := store.ResolveGroupWorkDir(m.projectRoot, m.store, group)
	promptText := prompt.BuildGroupPlanGenerationPrompt(m.projectRoot, group, tasks)

	var cmd tea.Cmd
	if m.program != nil {
		opts := buildCLIOptsFromAgent(a)
		opts.PermissionMode = "plan"
		inputCh := make(chan string, 1)
		m.processInputs.Register(procID, inputCh)
		cmd = claude.SpawnStreamCmd(m.program, workDir, procID, promptText, m.processCancels, m.processTimeout(), opts, inputCh)
	} else {
		projectRoot := m.projectRoot
		groupID := group.ID
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
	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Waiting for claude...",
		CompletionAction: model.CompletionCombinePlans,
		CompletionMeta: map[string]string{
			"planName": name,
			"taskIDs":  strings.Join(taskIDs, ","),
		},
	}
	m.addProcess(&proc)

	m.mode = model.ModeList
	m.action = actionNone

	var tasks []model.Task
	for _, id := range taskIDs {
		if t := store.FindTask(m.store, id); t != nil {
			tasks = append(tasks, *t)
		}
	}
	promptText := prompt.BuildCombinePlansPrompt(m.projectRoot, tasks)

	var cmd tea.Cmd
	if m.program != nil {
		cmd = claude.SpawnStreamCmd(m.program, m.projectRoot, procID, promptText, m.processCancels, m.processTimeout(), claude.CLIOptions{PermissionMode: "plan"}, nil)
	} else {
		projectRoot := m.projectRoot
		planName := name
		cmd = func() tea.Msg {
			output, logFile, err := runClaudePipe(projectRoot, procID, promptText)
			if err != nil {
				return claude.ProcessDoneMsg{ID: procID, LogFile: logFile, Err: err}
			}
			store.AddCombinedPlanWithTask(projectRoot, planName, taskIDs, output)
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

	workDir := store.ResolveWorkDir(m.projectRoot, m.store, task)
	promptText := prompt.BuildPlanFollowUpPrompt(m.projectRoot, task, question)
	label := "Ask: " + task.Title
	procID := fmt.Sprintf("followup-%s-%d", taskID, time.Now().Unix())

	planFile := task.PlanFile
	if planFile == "" {
		planFile = store.PlanFilenameForTask(task)
	}

	proc := model.ClaudeProcess{
		ID:               procID,
		Label:            label,
		Status:           model.ProcessRunning,
		Output:           "Waiting for claude...",
		IsInteractive:    true,
		CompletionAction: model.CompletionSaveFollowUp,
		CompletionMeta: map[string]string{
			"taskID":      taskID,
			"planFile":    planFile,
			"hasPlanFile": fmt.Sprintf("%v", task.PlanFile != ""),
		},
	}
	m.addProcess(&proc)

	m.mode = model.ModeTaskView
	m.action = actionNone

	var cmd tea.Cmd
	if m.program != nil {
		inputCh := make(chan string, 1)
		m.processInputs.Register(procID, inputCh)
		cmd = claude.SpawnStreamCmd(m.program, workDir, procID, promptText, m.processCancels, m.processTimeout(), claude.CLIOptions{PermissionMode: "plan"}, inputCh)
	} else {
		projectRoot := m.projectRoot
		hasPlanFile := task.PlanFile != ""
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

// --- Streaming handlers ---

func (m Model) handleStreamEvent(msg claude.StreamEventMsg) (tea.Model, tea.Cmd) {
	for i := range m.processes {
		if m.processes[i].ID == msg.ProcessID {
			m.processes[i].Events = append(m.processes[i].Events, msg.Event)
			if msg.SessionID != "" {
				m.processes[i].SessionID = msg.SessionID
			}
			// Update legacy Output for sidebar preview
			switch msg.Event.Kind {
			case model.EventText:
				// Clear initial placeholder on first text
				if m.processes[i].Output == "Waiting for claude..." || m.processes[i].Output == "Processing tasks..." {
					m.processes[i].Output = ""
				}
				m.processes[i].Output += msg.Event.Text
			case model.EventToolUse:
				m.processes[i].Output = "Running " + msg.Event.ToolName + "..."
			}
			break
		}
	}
	return m, nil
}

func (m Model) handleStreamWaiting(msg claude.StreamWaitingMsg) (tea.Model, tea.Cmd) {
	for i := range m.processes {
		if m.processes[i].ID != msg.ProcessID {
			continue
		}
		proc := &m.processes[i]
		if msg.SessionID != "" {
			proc.SessionID = msg.SessionID
		}
		proc.TurnCount = msg.TurnCount
		proc.CostUSD = msg.CostUSD

		// Run completion action on first turn (e.g., save plan)
		if proc.CompletionAction != model.CompletionNone {
			m.runCompletionAction(proc, msg.FinalText)
			proc.CompletionAction = model.CompletionNone
		}

		// If there's a queued message, auto-send it via the live channel
		if proc.QueuedMessage != "" {
			queuedMsg := proc.QueuedMessage
			proc.QueuedMessage = ""
			proc.Status = model.ProcessRunning
			if m.processInputs.Send(proc.ID, queuedMsg) {
				break
			}
		}

		proc.Status = model.ProcessWaiting
		proc.Events = append(proc.Events, model.StreamEvent{
			Kind: model.EventSystem,
			Text: "Waiting for your input — press c to respond",
		})
		break
	}
	m.reload()
	return m, nil
}

func (m Model) handleStreamDone(msg claude.StreamDoneMsg) (tea.Model, tea.Cmd) {
	// Clean up input channel (already closed by goroutine exit)
	m.processInputs.Remove(msg.ProcessID)

	for i := range m.processes {
		if m.processes[i].ID != msg.ProcessID {
			continue
		}
		proc := &m.processes[i]
		if msg.SessionID != "" {
			proc.SessionID = msg.SessionID
		}
		proc.TurnCount = msg.TurnCount
		proc.CostUSD = msg.CostUSD
		delete(m.runningLabels, proc.Label)

		if msg.Err != nil {
			// If cancelled but has a session ID, treat as interruptible — go to waiting
			if proc.SessionID != "" && (errors.Is(msg.Err, context.Canceled) || errors.Is(msg.Err, context.DeadlineExceeded)) {
				proc.Status = model.ProcessWaiting
				proc.Events = append(proc.Events, model.StreamEvent{
					Kind: model.EventSystem,
					Text: "Interrupted — press c to resume with guidance",
				})
				break
			}
			proc.Status = model.ProcessError
			proc.Output = "Error: " + msg.Err.Error()
			proc.Events = append(proc.Events, model.StreamEvent{
				Kind:    model.EventSystem,
				Text:    "Error: " + msg.Err.Error(),
				IsError: true,
			})
			// Revert planning status on error
			if proc.CompletionAction == model.CompletionSavePlan {
				if taskID := proc.CompletionMeta["taskID"]; taskID != "" {
					store.UpdateTask(m.projectRoot, taskID, map[string]interface{}{
						"status": string(model.StatusPending),
					})
				}
			}
			break
		}

		// Run completion action (for non-keep-alive processes;
		// keep-alive processes handle this in handleStreamWaiting)
		if proc.CompletionAction != model.CompletionNone {
			m.runCompletionAction(proc, msg.FinalText)
		}

		// If there's a queued message, auto-send it as a follow-up
		if proc.QueuedMessage != "" && msg.SessionID != "" {
			queuedMsg := proc.QueuedMessage
			proc.QueuedMessage = ""
			proc.Events = append(proc.Events, model.StreamEvent{
				Kind: model.EventUserMsg,
				Text: queuedMsg,
			})
			proc.Status = model.ProcessRunning
			m.reload()
			return m, claude.SpawnStreamResumeCmd(m.program, m.projectRoot, proc.ID, msg.SessionID, queuedMsg, m.processCancels, m.processTimeout())
		}

		// Set status: interactive processes wait for follow-up, others are done
		if proc.IsInteractive {
			proc.Status = model.ProcessWaiting
		} else {
			proc.Status = model.ProcessDone
		}
		break
	}
	m.reload()

	// Auto-remove non-interactive done/errored processes after 5s
	for _, proc := range m.processes {
		if proc.ID == msg.ProcessID && !proc.IsInteractive &&
			(proc.Status == model.ProcessDone || proc.Status == model.ProcessError) {
			return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return ProcessAutoRemoveMsg{ID: msg.ProcessID}
			})
		}
	}
	return m, nil
}

// extractPlanContent checks if Claude wrote a markdown file during the session.
// If so, reads that file's content. Otherwise falls back to finalText.
func extractPlanContent(events []model.StreamEvent, finalText string) string {
	// Scan for the last Write tool use targeting a .md file
	var lastMdPath string
	for _, ev := range events {
		if ev.Kind == model.EventToolUse && ev.ToolName == "Write" && strings.HasSuffix(ev.ToolInput, ".md") {
			lastMdPath = ev.ToolInput
		}
	}
	if lastMdPath != "" {
		if data, err := os.ReadFile(lastMdPath); err == nil && len(data) > 0 {
			return string(data)
		}
	}
	return finalText
}

func (m *Model) runCompletionAction(proc *model.ClaudeProcess, finalText string) {
	projectRoot := m.projectRoot
	meta := proc.CompletionMeta
	if meta == nil {
		meta = map[string]string{}
	}

	switch proc.CompletionAction {
	case model.CompletionSavePlan:
		taskID := meta["taskID"]
		taskTitle := meta["taskTitle"]
		filename := store.PlanFilenameForTask(&model.Task{ID: taskID, Title: taskTitle})
		planContent := extractPlanContent(proc.Events, finalText)
		store.SavePlan(projectRoot, filename, planContent)
		store.UpdateTask(projectRoot, taskID, map[string]interface{}{
			"planFile": filename,
			"status":   string(model.StatusPending),
		})

	case model.CompletionSaveGroupPlan:
		groupID := meta["groupID"]
		filename := store.PlanFilenameForGroup(&model.Group{ID: groupID})
		planContent := extractPlanContent(proc.Events, finalText)
		store.SavePlan(projectRoot, filename, planContent)
		s, _ := store.LoadStore(projectRoot)
		if g := store.FindGroup(s, groupID); g != nil {
			g.PlanFile = filename
			store.SaveStore(projectRoot, s)
		}

	case model.CompletionApplyGroupAction:
		applyGroupActionResult(projectRoot, finalText)

	case model.CompletionCombinePlans:
		planName := meta["planName"]
		var taskIDs []string
		for _, id := range strings.Split(meta["taskIDs"], ",") {
			if id != "" {
				taskIDs = append(taskIDs, id)
			}
		}
		store.AddCombinedPlanWithTask(projectRoot, planName, taskIDs, finalText)

	case model.CompletionSaveFollowUp:
		taskID := meta["taskID"]
		planFile := meta["planFile"]
		store.SavePlan(projectRoot, planFile, finalText)
		if meta["hasPlanFile"] != "true" {
			store.UpdateTask(projectRoot, taskID, map[string]interface{}{"planFile": planFile})
		}

	case model.CompletionRunTask:
		taskID := meta["taskID"]
		if taskID != "" {
			updates := map[string]interface{}{"status": string(model.StatusDone)}

			if meta["isProof"] == "true" {
				beforeFile := taskID + "-before.png"
				afterFile := taskID + "-after.png"
				dir := store.ScreenshotsDir(projectRoot)
				if _, err := os.Stat(filepath.Join(dir, beforeFile)); err == nil {
					updates["proofBefore"] = beforeFile
				}
				if _, err := os.Stat(filepath.Join(dir, afterFile)); err == nil {
					updates["proofAfter"] = afterFile
				}
			}

			store.UpdateTask(projectRoot, taskID, updates)
		}

	case model.CompletionApplyBulkAdd:
		applyBulkAddResult(projectRoot, finalText, meta["groupID"])
	}
}

func (m Model) handleChatSubmit(msg claude.ChatSubmitMsg) (tea.Model, tea.Cmd) {
	// If process is still running, queue the message for when the turn completes
	for i := range m.processes {
		if m.processes[i].ID == msg.ProcessID && m.processes[i].Status == model.ProcessRunning {
			m.processes[i].QueuedMessage = msg.Message
			m.processes[i].Events = append(m.processes[i].Events, model.StreamEvent{
				Kind: model.EventSystem,
				Text: "Message queued: " + msg.Message,
			})
			m.processAutoScroll = true
			m.mode = model.ModeProcessDetail
			m.viewport.GotoBottom()
			return m, flashCmd("Message queued — will send when turn completes")
		}
	}

	// Try keep-alive path: send via live input channel
	if m.processInputs.Has(msg.ProcessID) {
		for i := range m.processes {
			if m.processes[i].ID == msg.ProcessID {
				m.processes[i].Status = model.ProcessRunning
				break
			}
		}
		m.processInputs.Send(msg.ProcessID, msg.Message)
		m.processAutoScroll = true
		m.mode = model.ModeProcessDetail
		m.viewport.GotoBottom()
		return m, nil
	}

	// Fallback: resume via new subprocess (process was interrupted/cancelled)
	for i := range m.processes {
		if m.processes[i].ID == msg.ProcessID {
			m.processes[i].Events = append(m.processes[i].Events, model.StreamEvent{
				Kind: model.EventUserMsg,
				Text: msg.Message,
			})
			m.processes[i].Status = model.ProcessRunning
			break
		}
	}

	m.processAutoScroll = true
	m.mode = model.ModeProcessDetail
	m.viewport.GotoBottom()

	return m, claude.SpawnStreamResumeCmd(m.program, m.projectRoot, msg.ProcessID, msg.SessionID, msg.Message, m.processCancels, m.processTimeout())
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
			// Revert planning status when process completes (success or error)
			if m.processes[i].CompletionAction == model.CompletionSavePlan {
				if taskID := m.processes[i].CompletionMeta["taskID"]; taskID != "" {
					store.UpdateTask(m.projectRoot, taskID, map[string]interface{}{
						"status": string(model.StatusPending),
					})
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
	// Remember the ID of the currently selected process so we can
	// re-locate it after the slice changes.
	var selectedID string
	if m.processIdx < len(m.processes) {
		selectedID = m.processes[m.processIdx].ID
	}

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

	// Re-locate the previously selected process by ID.
	found := false
	for i, p := range m.processes {
		if p.ID == selectedID {
			m.processIdx = i
			found = true
			break
		}
	}
	if !found {
		// Selected process was the one removed — clamp index.
		if m.processIdx >= len(m.processes) {
			m.processIdx = max(0, len(m.processes)-1)
		}
	}
	m.processPaginator.SetTotalPages(len(m.processes))
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
