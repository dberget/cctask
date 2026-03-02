package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

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
	return strings.Join(parts, "\n")
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
	return strings.Join(parts, "\n")
}

func BuildPlanGenerationPrompt(task *model.Task) string {
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
	return strings.Join(lines, "\n")
}

func BuildGroupPlanGenerationPrompt(group *model.Group, tasks []model.Task) string {
	return BuildGroupPlanGenerationPromptWithStore(group, tasks, nil)
}

func BuildGroupPlanGenerationPromptWithStore(group *model.Group, tasks []model.Task, s *model.TaskStore) string {
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
	return strings.Join(lines, "\n")
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
	lines = append(lines, "Resolve any conflicts or redundancies between plans.")
	lines = append(lines, "Order steps logically so shared setup comes first.")
	lines = append(lines, "Preserve important details from each plan.")
	lines = append(lines, "Note which original tasks each section addresses.")
	lines = append(lines, "")
	lines = append(lines, "## Individual Plans")
	lines = append(lines, "")
	lines = append(lines, strings.Join(planSections, "\n\n---\n\n"))
	return strings.Join(lines, "\n")
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
	return strings.Join(parts, "\n")
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
}

func BuildGroupActionPrompt(tasks []model.Task, groups []model.Group, scopeLabel string, instruction string) string {
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
		gj = append(gj, groupJSON{ID: g.ID, Name: g.Name, Description: g.Description})
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
	lines = append(lines, `    {"name": "Group Name", "description": "..."}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "summary": "brief description of changes made"`)
	lines = append(lines, `}`)
	lines = append(lines, "")
	lines = append(lines, "Rules:")
	lines = append(lines, "- Include ONLY tasks that were modified in the tasks array")
	lines = append(lines, "- Each task must have ALL fields: id, title, description, status, tags, group")
	lines = append(lines, "- Valid status values: \"pending\", \"in-progress\", \"done\"")
	lines = append(lines, "- For group field, use the group ID (lowercase, hyphenated slug of the name)")
	lines = append(lines, "- If creating new groups, add them to newGroups and use their slugified name as the group ID in tasks")
	lines = append(lines, "- If no new groups are needed, use an empty array for newGroups")
	lines = append(lines, "- Do not remove or add tasks — only modify existing ones")
	return strings.Join(lines, "\n")
}
