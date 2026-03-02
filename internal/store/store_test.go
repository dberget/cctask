package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidberget/cctask-go/internal/model"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My Task!", "my-task"},
		{"a/b/c", "a-b-c"},
		{"", ""},
		{"UPPERCASE", "uppercase"},
		{"This is a very long title that should be truncated at forty characters max", "this-is-a-very-long-title-that-should-be"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return dir
}

func TestInit(t *testing.T) {
	dir := setupTestDir(t)

	if !IsInitialized(dir) {
		t.Fatal("expected initialized")
	}

	if _, err := os.Stat(filepath.Join(dir, ".cctask", "tasks.json")); err != nil {
		t.Fatalf("tasks.json not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cctask", "plans")); err != nil {
		t.Fatalf("plans dir not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cctask", "config.json")); err != nil {
		t.Fatalf("config.json not found: %v", err)
	}
}

func TestAddAndLoadTask(t *testing.T) {
	dir := setupTestDir(t)

	task, err := AddTask(dir, "Test Task", "A description", []string{"tag1", "tag2"}, "")
	if err != nil {
		t.Fatalf("AddTask failed: %v", err)
	}
	if task.ID != "t1" {
		t.Errorf("expected ID t1, got %s", task.ID)
	}
	if task.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got %s", task.Title)
	}
	if task.Status != "pending" {
		t.Errorf("expected status pending, got %s", task.Status)
	}

	s, err := LoadStore(dir)
	if err != nil {
		t.Fatalf("LoadStore failed: %v", err)
	}
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s.Tasks))
	}
	if s.NextID != 2 {
		t.Errorf("expected nextId 2, got %d", s.NextID)
	}
}

func TestUpdateTask(t *testing.T) {
	dir := setupTestDir(t)
	AddTask(dir, "Original", "", nil, "")

	updated, err := UpdateTask(dir, "t1", map[string]interface{}{
		"title": "Updated",
	})
	if err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %s", updated.Title)
	}
}

func TestDeleteTask(t *testing.T) {
	dir := setupTestDir(t)
	AddTask(dir, "To Delete", "", nil, "")

	if err := DeleteTask(dir, "t1"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	s, _ := LoadStore(dir)
	if len(s.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(s.Tasks))
	}
}

func TestGroupCRUD(t *testing.T) {
	dir := setupTestDir(t)

	g, err := AddGroup(dir, "My Project", "A project")
	if err != nil {
		t.Fatalf("AddGroup failed: %v", err)
	}
	if g.ID != "my-project" {
		t.Errorf("expected ID 'my-project', got %s", g.ID)
	}

	// Add task to group
	AddTask(dir, "Grouped Task", "", nil, g.ID)

	s, _ := LoadStore(dir)
	tasks := GetTasksForGroup(s, g.ID)
	if len(tasks) != 1 {
		t.Errorf("expected 1 task in group, got %d", len(tasks))
	}

	// Delete group should unassign tasks
	if err := DeleteGroup(dir, g.ID); err != nil {
		t.Fatalf("DeleteGroup failed: %v", err)
	}
	s, _ = LoadStore(dir)
	if len(s.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(s.Groups))
	}
	if s.Tasks[0].Group != "" {
		t.Errorf("expected task unassigned, got group %s", s.Tasks[0].Group)
	}
}

func TestPlanOperations(t *testing.T) {
	dir := setupTestDir(t)

	task, _ := AddTask(dir, "Plan Test", "", nil, "")
	filename := PlanFilenameForTask(task)
	if filename != "t1-plan-test.md" {
		t.Errorf("expected filename 't1-plan-test.md', got %s", filename)
	}

	// Save and load plan
	content := "# Plan\n\nStep 1: Do stuff"
	if err := SavePlan(dir, filename, content); err != nil {
		t.Fatalf("SavePlan failed: %v", err)
	}

	if !PlanExists(dir, filename) {
		t.Fatal("expected plan to exist")
	}

	loaded, err := LoadPlan(dir, filename)
	if err != nil {
		t.Fatalf("LoadPlan failed: %v", err)
	}
	if loaded != content {
		t.Errorf("plan content mismatch")
	}
}

func TestBuildListItems(t *testing.T) {
	dir := setupTestDir(t)

	// No groups: flat list
	AddTask(dir, "Task 1", "", nil, "")
	AddTask(dir, "Task 2", "", nil, "")
	s, _ := LoadStore(dir)

	items := BuildListItems(s, "", nil)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// With groups: project headers + tasks
	AddGroup(dir, "Project A", "")
	UpdateTask(dir, "t1", map[string]interface{}{"group": "project-a"})
	s, _ = LoadStore(dir)

	items = BuildListItems(s, "", nil)
	// Should be: All Tasks header, project header, t1 under project, t2 unassigned
	if len(items) != 4 {
		t.Errorf("expected 4 items (All Tasks + 1 project + 2 tasks), got %d", len(items))
	}
	if items[0].Kind != model.ListItemAllTasks {
		t.Error("expected first item to be AllTasks")
	}

	// Filter
	items = BuildListItems(s, "Task 2", nil)
	foundTask2 := false
	for _, item := range items {
		if item.Task != nil && item.Task.Title == "Task 2" {
			foundTask2 = true
		}
	}
	if !foundTask2 {
		t.Error("expected to find Task 2 in filtered results")
	}
}

func TestFindProjectRoot(t *testing.T) {
	dir := setupTestDir(t)
	subdir := filepath.Join(dir, "sub", "deep")
	os.MkdirAll(subdir, 0o755)

	found := FindProjectRoot(subdir)
	if found != dir {
		t.Errorf("expected %s, got %s", dir, found)
	}
}
