package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
)

const (
	cctaskDirName = ".cctask"
	tasksFileName = "tasks.json"
	plansDirName  = "plans"
	configFileName = "config.json"
	logsDirName   = "logs"
)

func FindProjectRoot(startDir string) string {
	if startDir == "" {
		startDir, _ = os.Getwd()
	}
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, cctaskDirName)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return startDir
}

func CctaskDir(projectRoot string) string {
	return filepath.Join(projectRoot, cctaskDirName)
}

func TasksPath(projectRoot string) string {
	return filepath.Join(CctaskDir(projectRoot), tasksFileName)
}

func PlansDir(projectRoot string) string {
	return filepath.Join(CctaskDir(projectRoot), plansDirName)
}

func LogsDir(projectRoot string) string {
	return filepath.Join(CctaskDir(projectRoot), logsDirName)
}

func ConfigPath(projectRoot string) string {
	return filepath.Join(CctaskDir(projectRoot), configFileName)
}

func IsInitialized(projectRoot string) bool {
	_, err := os.Stat(CctaskDir(projectRoot))
	return err == nil
}

func Init(projectRoot string) error {
	if err := os.MkdirAll(PlansDir(projectRoot), 0o755); err != nil {
		return err
	}
	tp := TasksPath(projectRoot)
	if _, err := os.Stat(tp); os.IsNotExist(err) {
		s := model.TaskStore{
			Tasks:         []model.Task{},
			Groups:        []model.Group{},
			CombinedPlans: []model.CombinedPlan{},
			NextID:        1,
		}
		if err := writeStore(tp, &s); err != nil {
			return err
		}
	}
	cp := ConfigPath(projectRoot)
	if _, err := os.Stat(cp); os.IsNotExist(err) {
		data, _ := json.MarshalIndent(model.Config{}, "", "  ")
		if err := os.WriteFile(cp, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func LoadStore(projectRoot string) (*model.TaskStore, error) {
	fp := TasksPath(projectRoot)
	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyStore(), nil
		}
		return nil, err
	}
	var s model.TaskStore
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Tasks == nil {
		s.Tasks = []model.Task{}
	}
	if s.Groups == nil {
		s.Groups = []model.Group{}
	}
	if s.CombinedPlans == nil {
		s.CombinedPlans = []model.CombinedPlan{}
	}
	return &s, nil
}

func SaveStore(projectRoot string, s *model.TaskStore) error {
	return writeStore(TasksPath(projectRoot), s)
}

func emptyStore() *model.TaskStore {
	return &model.TaskStore{
		Tasks:         []model.Task{},
		Groups:        []model.Group{},
		CombinedPlans: []model.CombinedPlan{},
		NextID:        1,
	}
}

func writeStore(fp string, s *model.TaskStore) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fp, data, 0o644)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(text string) string {
	s := strings.ToLower(text)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func GenerateTaskID(s *model.TaskStore) string {
	id := fmt.Sprintf("t%d", s.NextID)
	s.NextID++
	return id
}

func AddTask(projectRoot string, title, description string, tags []string, group string) (*model.Task, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}
	id := GenerateTaskID(s)
	now := model.Now()
	if tags == nil {
		tags = []string{}
	}
	task := model.Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      model.StatusPending,
		Tags:        tags,
		Group:       group,
		Created:     now,
		Updated:     now,
	}
	s.Tasks = append(s.Tasks, task)
	if err := SaveStore(projectRoot, s); err != nil {
		return nil, err
	}
	return &task, nil
}

func UpdateTask(projectRoot string, id string, updates map[string]interface{}) (*model.Task, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}
	idx := findTaskIndex(s, id)
	if idx == -1 {
		return nil, fmt.Errorf("task %s not found", id)
	}
	task := &s.Tasks[idx]
	for k, v := range updates {
		switch k {
		case "title":
			if val, ok := v.(string); ok {
				task.Title = val
			}
		case "description":
			if val, ok := v.(string); ok {
				task.Description = val
			}
		case "status":
			if val, ok := v.(model.TaskStatus); ok {
				task.Status = val
			}
		case "tags":
			if val, ok := v.([]string); ok {
				task.Tags = val
			}
		case "group":
			if val, ok := v.(string); ok {
				task.Group = val
			}
		case "planFile":
			if val, ok := v.(string); ok {
				task.PlanFile = val
			}
		case "mergedInto":
			if val, ok := v.(string); ok {
				task.MergedInto = val
			}
		}
	}
	task.Updated = model.Now()
	if err := SaveStore(projectRoot, s); err != nil {
		return nil, err
	}
	result := *task
	return &result, nil
}

func DeleteTask(projectRoot string, id string) error {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return err
	}
	idx := findTaskIndex(s, id)
	if idx == -1 {
		return fmt.Errorf("task %s not found", id)
	}
	task := s.Tasks[idx]
	if task.PlanFile != "" {
		pf := filepath.Join(PlansDir(projectRoot), task.PlanFile)
		os.Remove(pf)
	}

	// Revert any tasks that were merged into this one
	now := model.Now()
	for i := range s.Tasks {
		if s.Tasks[i].MergedInto == id {
			s.Tasks[i].Status = model.StatusPending
			s.Tasks[i].MergedInto = ""
			s.Tasks[i].Updated = now
		}
	}

	// Clean up associated combined plan entry
	for i, cp := range s.CombinedPlans {
		if cp.PlanFile == task.PlanFile {
			s.CombinedPlans = append(s.CombinedPlans[:i], s.CombinedPlans[i+1:]...)
			break
		}
	}

	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return SaveStore(projectRoot, s)
}

func AddGroup(projectRoot string, name string, description string) (*model.Group, error) {
	return AddGroupWithParent(projectRoot, name, description, "")
}

func AddGroupWithParent(projectRoot string, name string, description string, parentGroup string) (*model.Group, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}
	id := Slugify(name)
	// If parent is set and a sibling with same slug exists, disambiguate
	if parentGroup != "" && FindGroup(s, id) != nil {
		id = Slugify(parentGroup + "-" + name)
	}
	group := model.Group{
		ID:          id,
		Name:        name,
		Description: description,
		ParentGroup: parentGroup,
		Created:     model.Now(),
	}
	s.Groups = append(s.Groups, group)
	if err := SaveStore(projectRoot, s); err != nil {
		return nil, err
	}
	return &group, nil
}

func DeleteGroup(projectRoot string, id string) error {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return err
	}
	idx := findGroupIndex(s, id)
	if idx == -1 {
		return fmt.Errorf("group %s not found", id)
	}

	// Collect all descendant group IDs (recursive)
	allIDs := getAllDescendantGroupIDs(s, id)
	allIDs = append(allIDs, id)

	// Unassign tasks from all deleted groups
	deleteSet := make(map[string]bool, len(allIDs))
	for _, did := range allIDs {
		deleteSet[did] = true
	}
	for i := range s.Tasks {
		if deleteSet[s.Tasks[i].Group] {
			s.Tasks[i].Group = ""
		}
	}

	// Remove plan files and groups
	var remaining []model.Group
	for _, g := range s.Groups {
		if deleteSet[g.ID] {
			if g.PlanFile != "" {
				pf := filepath.Join(PlansDir(projectRoot), g.PlanFile)
				os.Remove(pf)
			}
			continue
		}
		remaining = append(remaining, g)
	}
	s.Groups = remaining

	return SaveStore(projectRoot, s)
}

func GetTasksForGroup(s *model.TaskStore, groupID string) []model.Task {
	var tasks []model.Task
	for _, t := range s.Tasks {
		if t.Group == groupID {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func GetTasksWithPlans(projectRoot string, s *model.TaskStore) []model.Task {
	var tasks []model.Task
	for _, t := range s.Tasks {
		if t.PlanFile != "" && PlanExists(projectRoot, t.PlanFile) {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

func FindTask(s *model.TaskStore, id string) *model.Task {
	idx := findTaskIndex(s, id)
	if idx == -1 {
		return nil
	}
	return &s.Tasks[idx]
}

func FindGroup(s *model.TaskStore, id string) *model.Group {
	idx := findGroupIndex(s, id)
	if idx == -1 {
		return nil
	}
	return &s.Groups[idx]
}

// GetChildGroups returns direct child groups of the given parent.
func GetChildGroups(s *model.TaskStore, parentID string) []model.Group {
	var children []model.Group
	for _, g := range s.Groups {
		if g.ParentGroup == parentID {
			children = append(children, g)
		}
	}
	return children
}

// getAllDescendantGroupIDs returns all nested group IDs recursively.
func getAllDescendantGroupIDs(s *model.TaskStore, groupID string) []string {
	var ids []string
	for _, g := range s.Groups {
		if g.ParentGroup == groupID {
			ids = append(ids, g.ID)
			ids = append(ids, getAllDescendantGroupIDs(s, g.ID)...)
		}
	}
	return ids
}

// GetAllDescendantGroupIDs returns all nested group IDs recursively (exported).
func GetAllDescendantGroupIDs(s *model.TaskStore, groupID string) []string {
	return getAllDescendantGroupIDs(s, groupID)
}

// GetGroupPath returns the ancestor chain from root to the given group (inclusive).
func GetGroupPath(s *model.TaskStore, groupID string) []model.Group {
	var path []model.Group
	current := groupID
	for current != "" {
		g := FindGroup(s, current)
		if g == nil {
			break
		}
		path = append([]model.Group{*g}, path...)
		current = g.ParentGroup
	}
	return path
}

func BuildListItems(s *model.TaskStore, filter string, collapsed map[string]bool) []model.ListItem {
	var items []model.ListItem
	matchesFilter := func(t model.Task) bool {
		if filter == "" {
			return true
		}
		f := strings.ToLower(filter)
		return strings.Contains(strings.ToLower(t.Title), f) ||
			strings.Contains(strings.ToLower(t.Description), f) ||
			strings.Contains(strings.ToLower(t.ID), f) ||
			tagsContain(t.Tags, f)
	}

	if len(s.Groups) == 0 {
		for i := range s.Tasks {
			if matchesFilter(s.Tasks[i]) {
				t := s.Tasks[i]
				items = append(items, model.ListItem{Kind: model.ListItemTask, Task: &t})
			}
		}
		return items
	}

	// Add "All Tasks" virtual group at the top
	items = append(items, model.ListItem{Kind: model.ListItemAllTasks})

	// Build tree: start with top-level groups (no parent)
	for gi := range s.Groups {
		group := s.Groups[gi]
		if group.ParentGroup != "" {
			continue // skip non-root groups, they'll be added recursively
		}
		items = appendGroupItems(items, s, group.ID, 0, filter, collapsed, matchesFilter)
	}

	// Unassigned tasks (merged ones last)
	var unassignedMerged []model.ListItem
	for i := range s.Tasks {
		t := s.Tasks[i]
		if t.Group == "" && matchesFilter(t) {
			item := model.ListItem{Kind: model.ListItemTask, Task: &t}
			if t.Status == model.StatusMerged {
				unassignedMerged = append(unassignedMerged, item)
			} else {
				items = append(items, item)
			}
		}
	}
	items = append(items, unassignedMerged...)

	return items
}

// appendGroupItems recursively adds a group, its tasks, and child groups to the list.
func appendGroupItems(items []model.ListItem, s *model.TaskStore, groupID string, depth int, filter string, collapsed map[string]bool, matchesFilter func(model.Task) bool) []model.ListItem {
	g := FindGroup(s, groupID)
	if g == nil {
		return items
	}

	groupTasks := filterGroupTasks(s, groupID, matchesFilter)
	children := GetChildGroups(s, groupID)

	// Check if this group or any descendant has matching tasks
	hasContent := len(groupTasks) > 0
	if !hasContent {
		for _, child := range children {
			if groupSubtreeHasMatch(s, child.ID, matchesFilter) {
				hasContent = true
				break
			}
		}
	}

	if filter != "" && !hasContent {
		return items
	}

	group := *g
	items = append(items, model.ListItem{Kind: model.ListItemProject, Project: &group, Depth: depth})

	if collapsed[groupID] {
		return items
	}

	// Add child groups first (recursively)
	for _, child := range children {
		items = appendGroupItems(items, s, child.ID, depth+1, filter, collapsed, matchesFilter)
	}

	// Add direct tasks for this group
	for ti := range groupTasks {
		t := groupTasks[ti]
		items = append(items, model.ListItem{Kind: model.ListItemTask, Task: &t, Depth: depth + 1})
	}

	return items
}

// groupSubtreeHasMatch returns true if a group or any descendant has matching tasks.
func groupSubtreeHasMatch(s *model.TaskStore, groupID string, matchesFilter func(model.Task) bool) bool {
	tasks := filterGroupTasks(s, groupID, matchesFilter)
	if len(tasks) > 0 {
		return true
	}
	for _, child := range GetChildGroups(s, groupID) {
		if groupSubtreeHasMatch(s, child.ID, matchesFilter) {
			return true
		}
	}
	return false
}

func findTaskIndex(s *model.TaskStore, id string) int {
	for i, t := range s.Tasks {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func findGroupIndex(s *model.TaskStore, id string) int {
	for i, g := range s.Groups {
		if g.ID == id {
			return i
		}
	}
	return -1
}

func tagsContain(tags []string, query string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

func filterGroupTasks(s *model.TaskStore, groupID string, matches func(model.Task) bool) []model.Task {
	var active, merged []model.Task
	for _, t := range s.Tasks {
		if t.Group == groupID && matches(t) {
			if t.Status == model.StatusMerged {
				merged = append(merged, t)
			} else {
				active = append(active, t)
			}
		}
	}
	return append(active, merged...)
}
