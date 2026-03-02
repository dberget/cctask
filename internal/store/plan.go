package store

import (
	"fmt"
	"os"
	"path/filepath"

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
