package model

type ListItemKind int

const (
	ListItemProject ListItemKind = iota
	ListItemTask
	ListItemAllTasks
)

type ListItem struct {
	Kind    ListItemKind
	Task    *Task
	Project *Group
	Depth   int // nesting level (0 = top-level)
}
