package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
	claudecode "github.com/severity1/claude-agent-sdk-go"
)

// UserQuestionMsg is sent from the ask_user tool handler to the TUI.
type UserQuestionMsg struct {
	ProcessID string
	Question  string
}

// StoreChangedMsg is sent when a tool modifies the task store, triggering a TUI reload.
type StoreChangedMsg struct{}

// ToolBridge connects MCP tool handlers (running in SDK goroutines) to the Bubbletea TUI.
type ToolBridge struct {
	ProjectRoot string
	Program     *tea.Program

	mu       sync.Mutex
	answerCh chan string
}

// NewToolBridge creates a new ToolBridge for the given project root.
func NewToolBridge(projectRoot string) *ToolBridge {
	return &ToolBridge{
		ProjectRoot: projectRoot,
	}
}

// SendAnswer delivers the user's answer to a pending ask_user tool call.
// Returns false if no question is pending.
func (tb *ToolBridge) SendAnswer(answer string) bool {
	tb.mu.Lock()
	ch := tb.answerCh
	tb.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- answer:
		return true
	default:
		return false
	}
}

// HasPendingQuestion returns true if there is a pending ask_user question.
func (tb *ToolBridge) HasPendingQuestion() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.answerCh != nil
}

// CreateMcpServer builds an MCP server config with all cctask tools for the given process.
func (tb *ToolBridge) CreateMcpServer(procID string) *claudecode.McpSdkServerConfig {
	return claudecode.CreateSDKMcpServer("cctask", "1.0.0",
		tb.askUserTool(procID),
		tb.getTasksTool(),
		tb.getTaskTool(),
		tb.createTaskTool(),
		tb.updateTaskTool(),
		tb.getGroupsTool(),
		tb.createGroupTool(),
		tb.getPlanTool(),
		tb.updatePlanTool(),
	)
}

func (tb *ToolBridge) askUserTool(procID string) *claudecode.McpTool {
	return claudecode.NewTool(
		"ask_user",
		"Ask the user a clarifying question and wait for their answer. Use this when you need more information to proceed.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{
					"type":        "string",
					"description": "The question to ask the user",
				},
			},
			"required": []string{"question"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			question, _ := args["question"].(string)
			if question == "" {
				return errorResult("question is required"), nil
			}

			tb.mu.Lock()
			ch := make(chan string, 1)
			tb.answerCh = ch
			tb.mu.Unlock()

			defer func() {
				tb.mu.Lock()
				tb.answerCh = nil
				tb.mu.Unlock()
			}()

			// Notify the TUI
			if tb.Program != nil {
				tb.Program.Send(UserQuestionMsg{
					ProcessID: procID,
					Question:  question,
				})
			}

			// Block until we get an answer or context is cancelled
			select {
			case answer := <-ch:
				return &claudecode.McpToolResult{
					Content: []claudecode.McpContent{
						{Type: "text", Text: answer},
					},
				}, nil
			case <-ctx.Done():
				return errorResult("question cancelled"), nil
			}
		},
	)
}

func (tb *ToolBridge) getTasksTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"get_tasks",
		"Get tasks from the project, optionally filtered by group ID or status.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"group_id": map[string]any{
					"type":        "string",
					"description": "Filter tasks by group ID (optional)",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Filter tasks by status: pending, planning, in-progress, done (optional)",
				},
			},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			s, err := store.LoadStore(tb.ProjectRoot)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load store: %v", err)), nil
			}

			groupID, _ := args["group_id"].(string)
			status, _ := args["status"].(string)

			var tasks []model.Task
			for _, t := range s.Tasks {
				if groupID != "" && t.Group != groupID {
					continue
				}
				if status != "" && string(t.Status) != status {
					continue
				}
				tasks = append(tasks, t)
			}

			data, err := json.Marshal(tasks)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to marshal tasks: %v", err)), nil
			}
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) getTaskTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"get_task",
		"Get a single task by ID, including its plan content if available.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "The task ID to look up",
				},
			},
			"required": []string{"task_id"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			taskID, _ := args["task_id"].(string)
			if taskID == "" {
				return errorResult("task_id is required"), nil
			}

			s, err := store.LoadStore(tb.ProjectRoot)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load store: %v", err)), nil
			}

			t := store.FindTask(s, taskID)
			if t == nil {
				return errorResult(fmt.Sprintf("task %s not found", taskID)), nil
			}

			// Build response with optional plan content
			type taskWithPlan struct {
				model.Task
				PlanContent string `json:"planContent,omitempty"`
			}
			resp := taskWithPlan{Task: *t}
			if t.PlanFile != "" {
				if content, err := store.LoadPlan(tb.ProjectRoot, t.PlanFile); err == nil {
					resp.PlanContent = content
				}
			}

			data, err := json.Marshal(resp)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to marshal task: %v", err)), nil
			}
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) createTaskTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"create_task",
		"Create a new task in the project.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Task title",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Task description (optional)",
				},
				"group": map[string]any{
					"type":        "string",
					"description": "Group ID to assign the task to (optional)",
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Tags for the task (optional)",
				},
				"work_dir": map[string]any{
					"type":        "string",
					"description": "Working directory for Claude when running this task (relative to project root or absolute, optional)",
				},
			},
			"required": []string{"title"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			title, _ := args["title"].(string)
			if title == "" {
				return errorResult("title is required"), nil
			}
			description, _ := args["description"].(string)
			group, _ := args["group"].(string)
			workDir, _ := args["work_dir"].(string)

			var tags []string
			if rawTags, ok := args["tags"].([]any); ok {
				for _, rt := range rawTags {
					if s, ok := rt.(string); ok {
						tags = append(tags, s)
					}
				}
			}

			task, err := store.AddTask(tb.ProjectRoot, title, description, tags, group, workDir)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to create task: %v", err)), nil
			}

			// Notify TUI to reload
			if tb.Program != nil {
				tb.Program.Send(StoreChangedMsg{})
			}

			data, err := json.Marshal(task)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to marshal task: %v", err)), nil
			}
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) updateTaskTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"update_task",
		"Update an existing task's title, description, status, tags, or group assignment.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "The task ID to update",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "New title (optional)",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "New description (optional)",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "New status: pending, planning, in-progress, done (optional)",
					"enum":        []string{"pending", "planning", "in-progress", "done"},
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "New tags array — replaces existing tags (optional)",
				},
				"group": map[string]any{
					"type":        "string",
					"description": "Group ID to assign to, or empty string to unassign (optional)",
				},
				"work_dir": map[string]any{
					"type":        "string",
					"description": "Working directory for Claude when running this task (relative to project root or absolute, optional)",
				},
			},
			"required": []string{"task_id"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			taskID, _ := args["task_id"].(string)
			if taskID == "" {
				return errorResult("task_id is required"), nil
			}

			updates := map[string]interface{}{}
			if v, ok := args["title"].(string); ok {
				updates["title"] = v
			}
			if v, ok := args["description"].(string); ok {
				updates["description"] = v
			}
			if v, ok := args["status"].(string); ok {
				updates["status"] = model.TaskStatus(v)
			}
			if rawTags, ok := args["tags"].([]any); ok {
				var tags []string
				for _, rt := range rawTags {
					if s, ok := rt.(string); ok {
						tags = append(tags, s)
					}
				}
				if tags == nil {
					tags = []string{}
				}
				updates["tags"] = tags
			}
			if v, ok := args["group"]; ok {
				if s, ok := v.(string); ok {
					updates["group"] = s
				}
			}
			if v, ok := args["work_dir"]; ok {
				if s, ok := v.(string); ok {
					updates["workDir"] = s
				}
			}

			if len(updates) == 0 {
				return errorResult("no fields to update"), nil
			}

			task, err := store.UpdateTask(tb.ProjectRoot, taskID, updates)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to update task: %v", err)), nil
			}

			if tb.Program != nil {
				tb.Program.Send(StoreChangedMsg{})
			}

			data, _ := json.Marshal(task)
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) getGroupsTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"get_groups",
		"Get all groups/projects in the task store.",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			s, err := store.LoadStore(tb.ProjectRoot)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load store: %v", err)), nil
			}

			data, err := json.Marshal(s.Groups)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to marshal groups: %v", err)), nil
			}
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) createGroupTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"create_group",
		"Create a new group/project. Optionally nest it under a parent group.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Group name",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Group description (optional)",
				},
				"parent_group": map[string]any{
					"type":        "string",
					"description": "Parent group ID to nest under (optional)",
				},
				"work_dir": map[string]any{
					"type":        "string",
					"description": "Working directory for Claude when running tasks in this group (relative to project root or absolute, optional)",
				},
			},
			"required": []string{"name"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return errorResult("name is required"), nil
			}
			description, _ := args["description"].(string)
			parentGroup, _ := args["parent_group"].(string)
			workDir, _ := args["work_dir"].(string)

			group, err := store.AddGroupWithParent(tb.ProjectRoot, name, description, parentGroup, workDir)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to create group: %v", err)), nil
			}

			if tb.Program != nil {
				tb.Program.Send(StoreChangedMsg{})
			}

			data, _ := json.Marshal(group)
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: string(data)},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) getPlanTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"get_plan",
		"Read plan content for a task or group by ID.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID to get plan for (provide task_id or group_id, not both)",
				},
				"group_id": map[string]any{
					"type":        "string",
					"description": "Group ID to get plan for (provide task_id or group_id, not both)",
				},
			},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			taskID, _ := args["task_id"].(string)
			groupID, _ := args["group_id"].(string)

			s, err := store.LoadStore(tb.ProjectRoot)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load store: %v", err)), nil
			}

			var planFile string
			if taskID != "" {
				t := store.FindTask(s, taskID)
				if t == nil {
					return errorResult(fmt.Sprintf("task %s not found", taskID)), nil
				}
				planFile = t.PlanFile
			} else if groupID != "" {
				g := store.FindGroup(s, groupID)
				if g == nil {
					return errorResult(fmt.Sprintf("group %s not found", groupID)), nil
				}
				planFile = g.PlanFile
			} else {
				return errorResult("provide task_id or group_id"), nil
			}

			if planFile == "" {
				return &claudecode.McpToolResult{
					Content: []claudecode.McpContent{
						{Type: "text", Text: "no plan exists"},
					},
				}, nil
			}

			content, err := store.LoadPlan(tb.ProjectRoot, planFile)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load plan: %v", err)), nil
			}
			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: content},
				},
			}, nil
		},
	)
}

func (tb *ToolBridge) updatePlanTool() *claudecode.McpTool {
	return claudecode.NewTool(
		"update_plan",
		"Write or update plan content for a task or group. Creates the plan file if it doesn't exist.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID to update plan for (provide task_id or group_id, not both)",
				},
				"group_id": map[string]any{
					"type":        "string",
					"description": "Group ID to update plan for (provide task_id or group_id, not both)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The plan content (markdown)",
				},
			},
			"required": []string{"content"},
		},
		func(ctx context.Context, args map[string]any) (*claudecode.McpToolResult, error) {
			taskID, _ := args["task_id"].(string)
			groupID, _ := args["group_id"].(string)
			content, _ := args["content"].(string)

			if content == "" {
				return errorResult("content is required"), nil
			}

			s, err := store.LoadStore(tb.ProjectRoot)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to load store: %v", err)), nil
			}

			var planFile string
			if taskID != "" {
				t := store.FindTask(s, taskID)
				if t == nil {
					return errorResult(fmt.Sprintf("task %s not found", taskID)), nil
				}
				planFile = t.PlanFile
				if planFile == "" {
					planFile = store.PlanFilenameForTask(t)
					store.UpdateTask(tb.ProjectRoot, taskID, map[string]interface{}{"planFile": planFile})
				}
			} else if groupID != "" {
				g := store.FindGroup(s, groupID)
				if g == nil {
					return errorResult(fmt.Sprintf("group %s not found", groupID)), nil
				}
				planFile = g.PlanFile
				if planFile == "" {
					planFile = store.PlanFilenameForGroup(g)
					g.PlanFile = planFile
					store.SaveStore(tb.ProjectRoot, s)
				}
			} else {
				return errorResult("provide task_id or group_id"), nil
			}

			if err := store.SavePlan(tb.ProjectRoot, planFile, content); err != nil {
				return errorResult(fmt.Sprintf("failed to save plan: %v", err)), nil
			}

			if tb.Program != nil {
				tb.Program.Send(StoreChangedMsg{})
			}

			return &claudecode.McpToolResult{
				Content: []claudecode.McpContent{
					{Type: "text", Text: fmt.Sprintf("plan saved to %s", planFile)},
				},
			}, nil
		},
	)
}

func errorResult(msg string) *claudecode.McpToolResult {
	return &claudecode.McpToolResult{
		Content: []claudecode.McpContent{
			{Type: "text", Text: msg},
		},
		IsError: true,
	}
}
