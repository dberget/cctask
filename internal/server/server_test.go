package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := store.Init(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newTestServer(t *testing.T, token string) (*Server, string) {
	t.Helper()
	root := setupTestProject(t)
	cfg := model.ServerConfig{
		Port:      0,
		AuthToken: token,
	}
	srv := New(root, cfg)
	return srv, root
}

// --- Handler Tests ---

func TestHealthEndpoint(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestCreateTaskEndpoint(t *testing.T) {
	srv, root := newTestServer(t, "")

	payload := `{"title":"Test task","description":"A test","tags":["webhook"]}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleCreateTask(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp taskResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Title != "Test task" {
		t.Fatalf("expected title 'Test task', got %q", resp.Title)
	}
	if resp.Status != "pending" {
		t.Fatalf("expected status 'pending', got %q", resp.Status)
	}

	// Verify task persisted in store
	s, err := store.LoadStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task in store, got %d", len(s.Tasks))
	}
	if s.Tasks[0].Title != "Test task" {
		t.Fatalf("persisted task title mismatch: %q", s.Tasks[0].Title)
	}
}

func TestCreateTaskMissingTitle(t *testing.T) {
	srv, _ := newTestServer(t, "")

	payload := `{"description":"no title"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()
	srv.handleCreateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateTaskInvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t, "")

	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	srv.handleCreateTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListTasksEndpoint(t *testing.T) {
	srv, root := newTestServer(t, "")

	// Create some tasks
	store.AddTask(root, "Task A", "", []string{"a"}, "")
	store.AddTask(root, "Task B", "", []string{"b"}, "mygroup")

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()
	srv.handleListTasks(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string][]taskResponse
	json.NewDecoder(w.Body).Decode(&body)
	if len(body["tasks"]) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(body["tasks"]))
	}
}

func TestListTasksFilterByGroup(t *testing.T) {
	srv, root := newTestServer(t, "")

	store.AddTask(root, "Task A", "", nil, "")
	store.AddTask(root, "Task B", "", nil, "backend")

	req := httptest.NewRequest("GET", "/api/tasks?group=backend", nil)
	w := httptest.NewRecorder()
	srv.handleListTasks(w, req)

	var body map[string][]taskResponse
	json.NewDecoder(w.Body).Decode(&body)
	if len(body["tasks"]) != 1 {
		t.Fatalf("expected 1 task for group=backend, got %d", len(body["tasks"]))
	}
	if body["tasks"][0].Title != "Task B" {
		t.Fatalf("expected 'Task B', got %q", body["tasks"][0].Title)
	}
}

func TestListTasksFilterByStatus(t *testing.T) {
	srv, root := newTestServer(t, "")

	store.AddTask(root, "Pending", "", nil, "")
	task, _ := store.AddTask(root, "Done", "", nil, "")
	store.UpdateTask(root, task.ID, map[string]interface{}{"status": model.StatusDone})

	req := httptest.NewRequest("GET", "/api/tasks?status=done", nil)
	w := httptest.NewRecorder()
	srv.handleListTasks(w, req)

	var body map[string][]taskResponse
	json.NewDecoder(w.Body).Decode(&body)
	if len(body["tasks"]) != 1 {
		t.Fatalf("expected 1 done task, got %d", len(body["tasks"]))
	}
}

// --- Auth Middleware Tests ---

func TestAuthMiddlewareValidToken(t *testing.T) {
	srv, _ := newTestServer(t, "mysecret")

	called := false
	handler := srv.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer mysecret")
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Fatal("handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	srv, _ := newTestServer(t, "mysecret")

	handler := srv.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	srv, _ := newTestServer(t, "mysecret")

	handler := srv.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareNoTokenConfigured(t *testing.T) {
	srv, _ := newTestServer(t, "")

	called := false
	handler := srv.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Fatal("handler should have been called when no token configured")
	}
}

// --- Server Lifecycle Tests ---

func TestServerStartStop(t *testing.T) {
	srv, _ := newTestServer(t, "")
	srv.port = 0 // will pick a random port

	// Use a known free port
	srv.port = 18923

	if err := srv.StartBackground(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	if !srv.Running() {
		t.Fatal("server should be running")
	}

	// Wait briefly for server to be ready
	time.Sleep(50 * time.Millisecond)

	// Make a health check request
	resp, err := http.Get("http://127.0.0.1:18923/api/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from health, got %d", resp.StatusCode)
	}

	srv.Stop(context.Background())
	if srv.Running() {
		t.Fatal("server should not be running after stop")
	}
}

// --- Integration Test ---

func TestIntegrationCreateAndListTasks(t *testing.T) {
	srv, _ := newTestServer(t, "tok123")
	srv.port = 18924

	if err := srv.StartBackground(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())
	time.Sleep(50 * time.Millisecond)

	client := &http.Client{}
	base := "http://127.0.0.1:18924"

	// Create a task via API
	body := `{"title":"Integration test task","tags":["test"]}`
	req, _ := http.NewRequest("POST", base+"/api/tasks", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer tok123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// List tasks via API
	req, _ = http.NewRequest("GET", base+"/api/tasks", nil)
	req.Header.Set("Authorization", "Bearer tok123")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}

	var listResp map[string][]taskResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	if len(listResp["tasks"]) != 1 {
		t.Fatalf("expected 1 task, got %d", len(listResp["tasks"]))
	}
	if listResp["tasks"][0].Title != "Integration test task" {
		t.Fatalf("task title mismatch: %q", listResp["tasks"][0].Title)
	}
}

// --- File Locking Test ---

func TestLockedModifyStore(t *testing.T) {
	root := setupTestProject(t)

	err := store.LockedModifyStore(root, func(s *model.TaskStore) error {
		id := store.GenerateTaskID(s)
		s.Tasks = append(s.Tasks, model.Task{
			ID:      id,
			Title:   "Locked task",
			Status:  model.StatusPending,
			Tags:    []string{},
			Created: model.Now(),
			Updated: model.Now(),
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	s, _ := store.LoadStore(root)
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(s.Tasks))
	}

	// Verify lock file was created
	lockPath := filepath.Join(root, ".cctask", ".lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should exist: %v", err)
	}
}
