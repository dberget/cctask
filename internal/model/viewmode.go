package model

type ViewMode int

const (
	ModeList ViewMode = iota
	ModeDetail
	ModePlan
	ModeGroupDetail
	ModeAddTask
	ModeEditTask
	ModeEditDescription
	ModeAddToGroup
	ModeFilter
	ModeEditTags
	ModeConfirmDelete
	ModeTaskForm
	ModeCombineSelect
	ModeCombineName
	ModeProcessDetail
	ModeEditPlan
	ModeTaskView
	ModeTaskViewAsk
	ModeGroupPrompt
	ModeThemePicker
	ModeHelp
)

func (m ViewMode) String() string {
	switch m {
	case ModeList:
		return "list"
	case ModeDetail:
		return "detail"
	case ModePlan:
		return "plan"
	case ModeGroupDetail:
		return "group-detail"
	case ModeAddTask:
		return "add-task"
	case ModeEditTask:
		return "edit-task"
	case ModeEditDescription:
		return "edit-description"
	case ModeAddToGroup:
		return "add-to-group"
	case ModeFilter:
		return "filter"
	case ModeEditTags:
		return "edit-tags"
	case ModeConfirmDelete:
		return "confirm-delete"
	case ModeTaskForm:
		return "task-form"
	case ModeCombineSelect:
		return "combine-select"
	case ModeCombineName:
		return "combine-name"
	case ModeProcessDetail:
		return "process-detail"
	case ModeEditPlan:
		return "edit-plan"
	case ModeTaskView:
		return "task-view"
	case ModeTaskViewAsk:
		return "task-view-ask"
	case ModeGroupPrompt:
		return "group-prompt"
	case ModeThemePicker:
		return "theme-picker"
	case ModeHelp:
		return "help"
	default:
		return "unknown"
	}
}

// IsNavigable returns true if the mode allows j/k/arrow navigation and action keys
func (m ViewMode) IsNavigable() bool {
	switch m {
	case ModeList, ModeDetail, ModePlan, ModeGroupDetail, ModeProcessDetail, ModeTaskView:
		return true
	default:
		return false
	}
}

type FocusPanel int

const (
	FocusMain FocusPanel = iota
	FocusProcesses
)
