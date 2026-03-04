package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

type createTaskRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Group       string   `json:"group"`
}

type taskResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Tags        []string `json:"tags,omitempty"`
	Group       string `json:"group,omitempty"`
	Created     string `json:"created"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	var resp taskResponse
	err := store.LockedModifyStore(s.projectRoot, func(st *model.TaskStore) error {
		id := store.GenerateTaskID(st)
		now := model.Now()
		tags := req.Tags
		if tags == nil {
			tags = []string{}
		}
		task := model.Task{
			ID:          id,
			Title:       req.Title,
			Description: req.Description,
			Status:      model.StatusPending,
			Tags:        tags,
			Group:       req.Group,
			Created:     now,
			Updated:     now,
		}
		st.Tasks = append(st.Tasks, task)
		resp = taskResponse{
			ID:      task.ID,
			Title:   task.Title,
			Description: task.Description,
			Status:  string(task.Status),
			Tags:    task.Tags,
			Group:   task.Group,
			Created: task.Created,
		}
		return nil
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create task: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	st, err := store.LoadStore(s.projectRoot)
	if err != nil {
		http.Error(w, `{"error":"failed to load store: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	groupFilter := r.URL.Query().Get("group")
	statusFilter := r.URL.Query().Get("status")

	var tasks []taskResponse
	for _, t := range st.Tasks {
		if groupFilter != "" && t.Group != groupFilter {
			continue
		}
		if statusFilter != "" && string(t.Status) != statusFilter {
			continue
		}
		tasks = append(tasks, taskResponse{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Status:      string(t.Status),
			Tags:        t.Tags,
			Group:       t.Group,
			Created:     t.Created,
		})
	}
	if tasks == nil {
		tasks = []taskResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	pluginNames := make([]string, len(s.plugins))
	for i, p := range s.plugins {
		pluginNames[i] = p.Name
	}

	uptime := time.Since(s.startTime).Round(time.Second).String()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"plugins": pluginNames,
		"uptime":  uptime,
	})
}
