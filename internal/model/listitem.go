package model

type ListItemKind int

const (
	ListItemProject ListItemKind = iota
	ListItemTask
)

type ListItem struct {
	Kind    ListItemKind
	Task    *Task
	Project *Group
}
