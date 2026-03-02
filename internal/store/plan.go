package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
)

func PlanFilenameForTask(task *model.Task) string {
	return fmt.Sprintf("%s-%s.md", task.ID, Slugify(task.Title))
}

func PlanFilenameForGroup(group *model.Group) string {
	return fmt.Sprintf("%s.md", group.ID)
}

func SavePlan(projectRoot string, filename string, content string) error {
	dir := PlansDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)
}

func LoadPlan(projectRoot string, filename string) (string, error) {
	fp := filepath.Join(PlansDir(projectRoot), filename)
	data, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func PlanExists(projectRoot string, filename string) bool {
	_, err := os.Stat(filepath.Join(PlansDir(projectRoot), filename))
	return err == nil
}

func AddCombinedPlan(projectRoot string, name string, sourceTaskIDs []string, content string) (*model.CombinedPlan, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("cp%d", nowUnixMilli())
	filename := fmt.Sprintf("combined-%s.md", Slugify(name))
	if err := SavePlan(projectRoot, filename, content); err != nil {
		return nil, err
	}
	plan := model.CombinedPlan{
		ID:            id,
		Name:          name,
		SourceTaskIDs: sourceTaskIDs,
		PlanFile:      filename,
		Created:       model.Now(),
	}
	s.CombinedPlans = append(s.CombinedPlans, plan)
	if err := SaveStore(projectRoot, s); err != nil {
		return nil, err
	}
	return &plan, nil
}

func AddCombinedPlanWithTask(projectRoot string, name string, sourceTaskIDs []string, content string) (*model.Task, error) {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return nil, err
	}

	// Save the combined plan file
	id := fmt.Sprintf("cp%d", nowUnixMilli())
	filename := fmt.Sprintf("combined-%s.md", Slugify(name))
	if err := SavePlan(projectRoot, filename, content); err != nil {
		return nil, err
	}

	// Record the combined plan entry
	plan := model.CombinedPlan{
		ID:            id,
		Name:          name,
		SourceTaskIDs: sourceTaskIDs,
		PlanFile:      filename,
		Created:       model.Now(),
	}
	s.CombinedPlans = append(s.CombinedPlans, plan)

	// Determine common group and collect tags from source tasks
	var descLines []string
	tagSet := map[string]bool{}
	commonGroup := ""
	groupSet := false
	for _, tid := range sourceTaskIDs {
		t := FindTask(s, tid)
		if t == nil {
			continue
		}
		descLines = append(descLines, fmt.Sprintf("- %s: %s", t.ID, t.Title))
		for _, tag := range t.Tags {
			tagSet[tag] = true
		}
		if !groupSet {
			commonGroup = t.Group
			groupSet = true
		} else if t.Group != commonGroup {
			commonGroup = ""
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}

	// Create the new task
	now := model.Now()
	taskID := GenerateTaskID(s)
	newTask := model.Task{
		ID:          taskID,
		Title:       name,
		Description: "Combined plan from:\n" + strings.Join(descLines, "\n"),
		Status:      model.StatusPlanning,
		Tags:        tags,
		Group:       commonGroup,
		PlanFile:    filename,
		Created:     now,
		Updated:     now,
	}
	s.Tasks = append(s.Tasks, newTask)

	// Mark source tasks as merged
	for _, tid := range sourceTaskIDs {
		idx := findTaskIndex(s, tid)
		if idx == -1 {
			continue
		}
		s.Tasks[idx].Status = model.StatusMerged
		s.Tasks[idx].MergedInto = taskID
		s.Tasks[idx].Updated = now
	}

	if err := SaveStore(projectRoot, s); err != nil {
		return nil, err
	}
	return &newTask, nil
}

func DeleteCombinedPlan(projectRoot string, id string) error {
	s, err := LoadStore(projectRoot)
	if err != nil {
		return err
	}
	idx := -1
	for i, cp := range s.CombinedPlans {
		if cp.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("combined plan %s not found", id)
	}
	cp := s.CombinedPlans[idx]
	pf := filepath.Join(PlansDir(projectRoot), cp.PlanFile)
	os.Remove(pf)
	s.CombinedPlans = append(s.CombinedPlans[:idx], s.CombinedPlans[idx+1:]...)
	return SaveStore(projectRoot, s)
}

func nowUnixMilli() int64 {
	return model.NowTime().UnixMilli()
}
