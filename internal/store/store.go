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
	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return SaveStore(projectRoot, s)
}

func AddGroup(projectRoot string, name string, description string) (*model.Group, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}
	id := Slugify(name)
	group := model.Group{
		ID:          id,
		Name:        name,
		Description: description,
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
	for i := range s.Tasks {
		if s.Tasks[i].Group == id {
			s.Tasks[i].Group = ""
		}
	}
	group := s.Groups[idx]
	if group.PlanFile != "" {
		pf := filepath.Join(PlansDir(projectRoot), group.PlanFile)
		os.Remove(pf)
	}
	s.Groups = append(s.Groups[:idx], s.Groups[idx+1:]...)
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

	for gi := range s.Groups {
		group := s.Groups[gi]
		projectTasks := filterGroupTasks(s, group.ID, matchesFilter)
		if filter == "" || len(projectTasks) > 0 {
			items = append(items, model.ListItem{Kind: model.ListItemProject, Project: &group})
			if !collapsed[group.ID] {
				for ti := range projectTasks {
					t := projectTasks[ti]
					items = append(items, model.ListItem{Kind: model.ListItemTask, Task: &t})
				}
			}
		}
	}

	// Unassigned tasks
	for i := range s.Tasks {
		t := s.Tasks[i]
		if t.Group == "" && matchesFilter(t) {
			items = append(items, model.ListItem{Kind: model.ListItemTask, Task: &t})
		}
	}

	return items
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
	var tasks []model.Task
	for _, t := range s.Tasks {
		if t.Group == groupID && matches(t) {
			tasks = append(tasks, t)
		}
	}
	return tasks
}
