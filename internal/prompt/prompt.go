package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

func prependContext(projectRoot string, prompt string) string {
	ctx := store.LoadContext(projectRoot)
	if ctx == "" {
		return prompt
	}
	return "# Project Context\n" + ctx + "\n\n---\n\n" + prompt
}

func BuildTaskPrompt(projectRoot string, task *model.Task) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("# Task: %s", task.Title))
	parts = append(parts, fmt.Sprintf("ID: %s", task.ID))
	if task.Description != "" {
		parts = append(parts, fmt.Sprintf("\n## Description\n%s", task.Description))
	}
	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("\nTags: %s", strings.Join(task.Tags, ", ")))
	}
	if task.PlanFile != "" {
		plan, err := store.LoadPlan(projectRoot, task.PlanFile)
		if err == nil && plan != "" {
			parts = append(parts, fmt.Sprintf("\n## Implementation Plan\n%s", plan))
		}
	}
	return prependContext(projectRoot, strings.Join(parts, "\n"))
}

func BuildGroupPrompt(projectRoot string, group *model.Group, s *model.TaskStore) string {
	tasks := store.GetTasksForGroup(s, group.ID)
	children := store.GetChildGroups(s, group.ID)
	var parts []string

	// Show hierarchy context
	if group.ParentGroup != "" {
		path := store.GetGroupPath(s, group.ID)
		var names []string
		for _, g := range path {
			names = append(names, g.Name)
		}
		parts = append(parts, fmt.Sprintf("# Project: %s", strings.Join(names, " > ")))
	} else {
		parts = append(parts, fmt.Sprintf("# Project: %s", group.Name))
	}

	if group.Description != "" {
		parts = append(parts, fmt.Sprintf("\n## Description\n%s", group.Description))
	}
	if group.PlanFile != "" {
		plan, err := store.LoadPlan(projectRoot, group.PlanFile)
		if err == nil && plan != "" {
			parts = append(parts, fmt.Sprintf("\n## Project Plan\n%s", plan))
		}
	}

	// Show subgroups
	if len(children) > 0 {
		parts = append(parts, fmt.Sprintf("\n## Subgroups (%d)", len(children)))
		for _, child := range children {
			childTasks := store.GetTasksForGroup(s, child.ID)
			parts = append(parts, fmt.Sprintf("- %s (%d tasks)", child.Name, len(childTasks)))
		}
	}

	parts = append(parts, fmt.Sprintf("\n## Tasks (%d)", len(tasks)))
	for _, task := range tasks {
		parts = append(parts, fmt.Sprintf("\n### %s: %s", task.ID, task.Title))
		parts = append(parts, fmt.Sprintf("Status: %s", task.Status))
		if task.Description != "" {
			parts = append(parts, task.Description)
		}
		if task.PlanFile != "" {
			plan, err := store.LoadPlan(projectRoot, task.PlanFile)
			if err == nil && plan != "" {
				parts = append(parts, fmt.Sprintf("\n#### Plan\n%s", plan))
			}
		}
	}

	// Task data file instructions
	tasksPath := store.TasksPath(projectRoot)
	plansDir := store.PlansDir(projectRoot)
	parts = append(parts, "\n## Task Data Files")
	parts = append(parts, fmt.Sprintf("You can manage tasks and groups by editing the data files directly."))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("### Task store: %s", tasksPath))
	parts = append(parts, "JSON file with this schema:")
	parts = append(parts, "```json")
	parts = append(parts, `{`)
	parts = append(parts, `  "tasks": [`)
	parts = append(parts, `    {`)
	parts = append(parts, `      "id": "t1",`)
	parts = append(parts, `      "title": "Task title",`)
	parts = append(parts, `      "description": "Details",`)
	parts = append(parts, `      "status": "pending",`)
	parts = append(parts, `      "tags": ["tag1"],`)
	parts = append(parts, fmt.Sprintf(`      "group": "%s",`, group.ID))
	parts = append(parts, `      "planFile": "t1-task-title.md",`)
	parts = append(parts, `      "created": "2025-01-01T00:00:00Z",`)
	parts = append(parts, `      "updated": "2025-01-01T00:00:00Z"`)
	parts = append(parts, `    }`)
	parts = append(parts, `  ],`)
	parts = append(parts, `  "groups": [`)
	parts = append(parts, `    {`)
	parts = append(parts, `      "id": "group-slug",`)
	parts = append(parts, `      "name": "Group Name",`)
	parts = append(parts, `      "description": "Details",`)
	parts = append(parts, `      "parentGroup": "",`)
	parts = append(parts, `      "planFile": "group-slug.md",`)
	parts = append(parts, `      "created": "2025-01-01T00:00:00Z"`)
	parts = append(parts, `    }`)
	parts = append(parts, `  ],`)
	parts = append(parts, `  "combinedPlans": [],`)
	parts = append(parts, `  "nextId": 2`)
	parts = append(parts, `}`)
	parts = append(parts, "```")
	parts = append(parts, "")
	parts = append(parts, "Rules:")
	parts = append(parts, "- Task IDs use the format \"t<N>\" (e.g. t1, t2). Increment nextId when adding tasks.")
	parts = append(parts, "- Valid status values: \"pending\", \"planning\", \"in-progress\", \"done\", \"merged\"")
	parts = append(parts, "- Do not modify tasks with status \"merged\"")
	parts = append(parts, "- Group IDs are lowercase hyphenated slugs of the group name (max 40 chars)")
	parts = append(parts, "- Set the task \"group\" field to the group ID to assign it to a group")
	parts = append(parts, "- Timestamps use RFC3339 format")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("### Plans directory: %s", plansDir))
	parts = append(parts, "- Task plans: `<taskId>-<slugified-title>.md` (e.g. `t1-task-title.md`)")
	parts = append(parts, "- Group plans: `<group-id>.md` (e.g. `my-group.md`)")
	parts = append(parts, "- Set the task/group \"planFile\" field to the filename after creating a plan file")

	return prependContext(projectRoot, strings.Join(parts, "\n"))
}

func BuildPlanGenerationPrompt(projectRoot string, task *model.Task) string {
	var lines []string
	lines = append(lines, "Create a detailed implementation plan for the following task.")
	lines = append(lines, "Output ONLY the plan as markdown, no preamble.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Task: %s", task.Title))
	if task.Description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", task.Description))
	}
	if len(task.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("Tags: %s", strings.Join(task.Tags, ", ")))
	}
	lines = append(lines, "")
	lines = append(lines, "The plan should include:")
	lines = append(lines, "- Step-by-step implementation approach")
	lines = append(lines, "- Key files to create or modify")
	lines = append(lines, "- Important considerations or edge cases")
	lines = append(lines, "- Testing approach")
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}

func BuildGroupPlanGenerationPrompt(projectRoot string, group *model.Group, tasks []model.Task) string {
	return BuildGroupPlanGenerationPromptWithStore(projectRoot, group, tasks, nil)
}

func BuildGroupPlanGenerationPromptWithStore(projectRoot string, group *model.Group, tasks []model.Task, s *model.TaskStore) string {
	var taskList []string
	for _, t := range tasks {
		desc := ""
		if t.Description != "" {
			desc = " — " + t.Description
		}
		taskList = append(taskList, fmt.Sprintf("- %s: %s%s", t.ID, t.Title, desc))
	}

	var lines []string
	lines = append(lines, "Create a detailed implementation plan for the following project.")
	lines = append(lines, "Output ONLY the plan as markdown, no preamble.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Project: %s", group.Name))
	if group.Description != "" {
		lines = append(lines, fmt.Sprintf("Description: %s", group.Description))
	}

	// Include subgroup context if store is available
	if s != nil {
		children := store.GetChildGroups(s, group.ID)
		if len(children) > 0 {
			lines = append(lines, "")
			lines = append(lines, "Subgroups:")
			for _, child := range children {
				childTasks := store.GetTasksForGroup(s, child.ID)
				lines = append(lines, fmt.Sprintf("- %s (%d tasks)", child.Name, len(childTasks)))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Tasks:")
	lines = append(lines, strings.Join(taskList, "\n"))
	lines = append(lines, "")
	lines = append(lines, "The plan should include:")
	lines = append(lines, "- Overall approach and architecture")
	lines = append(lines, "- Implementation order and dependencies between tasks")
	lines = append(lines, "- Key files to create or modify")
	lines = append(lines, "- Important considerations or edge cases")
	lines = append(lines, "- Testing approach")
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}

func BuildCombinePlansPrompt(projectRoot string, tasks []model.Task) string {
	var planSections []string
	for _, t := range tasks {
		if t.PlanFile == "" {
			continue
		}
		plan, err := store.LoadPlan(projectRoot, t.PlanFile)
		if err != nil || plan == "" {
			continue
		}
		desc := ""
		if t.Description != "" {
			desc = t.Description + "\n"
		}
		planSections = append(planSections, fmt.Sprintf("### %s: %s\n%s%s", t.ID, t.Title, desc, plan))
	}

	var lines []string
	lines = append(lines, "You are reviewing multiple implementation plans for related tasks.")
	lines = append(lines, "Combine them into one coherent, unified implementation plan.")
	lines = append(lines, "Output ONLY the combined plan as markdown, no preamble.")
	lines = append(lines, "")
	lines = append(lines, "The combined plan should include:")
	lines = append(lines, "- Overall approach and architecture")
	lines = append(lines, "- Implementation order and dependencies between tasks (which tasks must be completed before others can start)")
	lines = append(lines, "- Key files to create or modify")
	lines = append(lines, "- Important considerations or edge cases")
	lines = append(lines, "- Testing approach")
	lines = append(lines, "")
	lines = append(lines, "Guidelines:")
	lines = append(lines, "- Resolve any conflicts or redundancies between the individual plans")
	lines = append(lines, "- Order steps logically: shared setup and foundational work first, then dependent work")
	lines = append(lines, "- If tasks have natural dependencies (e.g. one task's output is another's input, or one builds on infrastructure from another), call these out explicitly")
	lines = append(lines, "- Preserve important details from each plan")
	lines = append(lines, "- Note which original tasks (by ID) each section addresses")
	lines = append(lines, "")
	lines = append(lines, "## Individual Plans")
	lines = append(lines, "")
	lines = append(lines, strings.Join(planSections, "\n\n---\n\n"))
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}

func BuildPlanFollowUpPrompt(projectRoot string, task *model.Task, question string) string {
	var parts []string
	parts = append(parts, "You are reviewing an implementation plan for a task and answering a follow-up question.")
	parts = append(parts, "Output an updated/revised plan as markdown incorporating your answer, no preamble.")
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("## Task: %s", task.Title))
	if task.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", task.Description))
	}
	if len(task.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("Tags: %s", strings.Join(task.Tags, ", ")))
	}
	if task.PlanFile != "" {
		plan, err := store.LoadPlan(projectRoot, task.PlanFile)
		if err == nil && plan != "" {
			parts = append(parts, "")
			parts = append(parts, "## Current Plan")
			parts = append(parts, plan)
		}
	}
	parts = append(parts, "")
	parts = append(parts, "## Follow-up Question")
	parts = append(parts, question)
	parts = append(parts, "")
	parts = append(parts, "Please provide an updated plan that addresses this question. If the question requires changes to the plan, incorporate them. If it's a clarification, add the answer as a note in the relevant section.")
	return prependContext(projectRoot, strings.Join(parts, "\n"))
}

// taskJSON is a simplified task struct for JSON serialization in group action prompts.
type taskJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Group       string   `json:"group"`
}

// groupJSON is a simplified group struct for JSON serialization in group action prompts.
type groupJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentGroup string `json:"parentGroup"`
}

func BuildGroupActionPrompt(projectRoot string, tasks []model.Task, groups []model.Group, scopeLabel string, instruction string) string {
	var tj []taskJSON
	for _, t := range tasks {
		tags := t.Tags
		if tags == nil {
			tags = []string{}
		}
		tj = append(tj, taskJSON{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
			Status:      string(t.Status),
			Tags:        tags,
			Group:       t.Group,
		})
	}
	tasksData, _ := json.MarshalIndent(tj, "", "  ")

	var gj []groupJSON
	for _, g := range groups {
		gj = append(gj, groupJSON{ID: g.ID, Name: g.Name, Description: g.Description, ParentGroup: g.ParentGroup})
	}
	groupsData, _ := json.MarshalIndent(gj, "", "  ")

	var lines []string
	lines = append(lines, "You are a task management assistant. Process the tasks below according to the user's instruction.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("## Scope: %s", scopeLabel))
	lines = append(lines, "")
	lines = append(lines, "## Available Groups")
	lines = append(lines, string(groupsData))
	lines = append(lines, "")
	lines = append(lines, "## Tasks")
	lines = append(lines, string(tasksData))
	lines = append(lines, "")
	lines = append(lines, "## Instruction")
	lines = append(lines, instruction)
	lines = append(lines, "")
	lines = append(lines, "## Output Format")
	lines = append(lines, "Output ONLY valid JSON (no markdown code blocks, no preamble, no explanation):")
	lines = append(lines, `{`)
	lines = append(lines, `  "tasks": [`)
	lines = append(lines, `    {"id": "t1", "title": "...", "description": "...", "status": "pending", "tags": ["..."], "group": "group-id"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "newGroups": [`)
	lines = append(lines, `    {"name": "Group Name", "description": "...", "parentGroup": "parent-group-id-or-empty"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "updatedGroups": [`)
	lines = append(lines, `    {"id": "existing-group-id", "parentGroup": "new-parent-id-or-empty"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "summary": "brief description of changes made"`)
	lines = append(lines, `}`)
	lines = append(lines, "")
	lines = append(lines, "Rules:")
	lines = append(lines, "- Include ONLY tasks that were modified in the tasks array")
	lines = append(lines, "- Each task must have ALL fields: id, title, description, status, tags, group")
	lines = append(lines, "- Valid status values: \"pending\", \"planning\", \"in-progress\", \"done\", \"merged\"")
	lines = append(lines, "- Do not modify tasks with status \"merged\"")
	lines = append(lines, "- For group field, use the group ID (lowercase, hyphenated slug of the name)")
	lines = append(lines, "- Groups support hierarchy via the parentGroup field (ID of the parent group, or empty string for top-level)")
	lines = append(lines, "- If creating new groups, add them to newGroups and use their slugified name as the group ID in tasks")
	lines = append(lines, "- To make an existing group a subgroup of another, add it to updatedGroups with the new parentGroup ID")
	lines = append(lines, "- If no new groups are needed, use an empty array for newGroups")
	lines = append(lines, "- If no group hierarchy changes are needed, use an empty array for updatedGroups")
	lines = append(lines, "- Do not remove or add tasks — only modify existing ones")
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}
