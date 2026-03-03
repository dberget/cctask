package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidberget/cctask-go/internal/model"
	"github.com/davidberget/cctask-go/internal/store"
)

const mcpToolGuidance = `## Available Tools
You have access to MCP tools via the "cctask" server:
- mcp__cctask__ask_user: Ask the user a clarifying question if you need more information. Use this freely.
- mcp__cctask__get_tasks: Read tasks (filter by group_id or status).
- mcp__cctask__get_task: Read a single task with its plan content.
- mcp__cctask__get_groups: List all groups/projects.
- mcp__cctask__get_plan: Read a plan file by task or group ID.

Do NOT use mcp__cctask__create_task, mcp__cctask__update_task, mcp__cctask__create_group, mcp__cctask__update_plan in this flow — your text output will be saved automatically.
Use the read tools and ask_user as needed, then output the final plan as markdown text.`

const mcpToolGuidanceGroupAction = `## Available Tools
You have access to MCP tools via the "cctask" server:
- mcp__cctask__ask_user: Ask the user a clarifying question if you need more information. Use this freely.
- mcp__cctask__get_tasks: Read tasks (filter by group_id or status).
- mcp__cctask__get_task: Read a single task with its plan content.
- mcp__cctask__get_groups: List all groups/projects.
- mcp__cctask__get_plan: Read a plan file by task or group ID.

Do NOT use mcp__cctask__create_task, mcp__cctask__update_task, mcp__cctask__create_group, mcp__cctask__update_plan in this flow — your JSON output will be parsed and applied automatically.
Use the read tools and ask_user as needed, then output the final JSON result.`

// McpToolGuidanceRun is appended to background run prompts — Claude gets full MCP tool access
// since it's doing actual implementation work.
const McpToolGuidanceRun = `## Available Tools
You have access to MCP tools via the "cctask" server:
- mcp__cctask__ask_user: Ask the user a clarifying question if you need more information. Use this freely.
- mcp__cctask__get_tasks: Read tasks (filter by group_id or status).
- mcp__cctask__get_task: Read a single task with its plan content.
- mcp__cctask__get_groups: List all groups/projects.
- mcp__cctask__get_plan: Read a plan file by task or group ID.
- mcp__cctask__create_task: Create new tasks as needed.
- mcp__cctask__update_task: Update task status, title, description, tags, or group.
- mcp__cctask__create_group: Create new groups/projects.
- mcp__cctask__update_plan: Write or update plan files.

You have full access to all tools. Use them as needed to complete the implementation work.`

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
	if task.WorkDir != "" {
		parts = append(parts, fmt.Sprintf("Working Directory: %s", task.WorkDir))
	}
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
	if group.WorkDir != "" {
		parts = append(parts, fmt.Sprintf("Working Directory: %s", group.WorkDir))
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
		if len(task.Tags) > 0 {
			parts = append(parts, fmt.Sprintf("Tags: %s", strings.Join(task.Tags, ", ")))
		}
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
	parts = append(parts, `      "workDir": "optional/relative/path",`)
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
	parts = append(parts, `      "workDir": "optional/relative/path",`)
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
	lines = append(lines, "")
	lines = append(lines, mcpToolGuidance)
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
		tags := ""
		if len(t.Tags) > 0 {
			tags = " [" + strings.Join(t.Tags, ", ") + "]"
		}
		taskList = append(taskList, fmt.Sprintf("- %s: %s%s%s", t.ID, t.Title, tags, desc))
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
	lines = append(lines, "")
	lines = append(lines, mcpToolGuidance)
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
		tags := ""
		if len(t.Tags) > 0 {
			tags = "Tags: " + strings.Join(t.Tags, ", ") + "\n"
		}
		planSections = append(planSections, fmt.Sprintf("### %s: %s\n%s%s%s", t.ID, t.Title, tags, desc, plan))
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
	parts = append(parts, "")
	parts = append(parts, mcpToolGuidance)
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
	WorkDir     string   `json:"workDir,omitempty"`
}

// groupJSON is a simplified group struct for JSON serialization in group action prompts.
type groupJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentGroup string `json:"parentGroup"`
	WorkDir     string `json:"workDir,omitempty"`
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
			WorkDir:     t.WorkDir,
		})
	}
	tasksData, _ := json.MarshalIndent(tj, "", "  ")

	var gj []groupJSON
	for _, g := range groups {
		gj = append(gj, groupJSON{ID: g.ID, Name: g.Name, Description: g.Description, ParentGroup: g.ParentGroup, WorkDir: g.WorkDir})
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
	lines = append(lines, `    {"id": "t1", "title": "...", "description": "...", "status": "pending", "tags": ["..."], "group": "group-id", "workDir": "optional/relative/path"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "newGroups": [`)
	lines = append(lines, `    {"name": "Group Name", "description": "...", "parentGroup": "parent-group-id-or-empty", "workDir": "optional/relative/path"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "updatedGroups": [`)
	lines = append(lines, `    {"id": "existing-group-id", "parentGroup": "new-parent-id-or-empty"}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "summary": "brief description of changes made"`)
	lines = append(lines, `}`)
	lines = append(lines, "")
	lines = append(lines, "Rules:")
	lines = append(lines, "- Include ONLY tasks that were modified in the tasks array")
	lines = append(lines, "- Each task must have ALL fields: id, title, description, status, tags, group (workDir is optional)")
	lines = append(lines, "- Valid status values: \"pending\", \"planning\", \"in-progress\", \"done\", \"merged\"")
	lines = append(lines, "- Do not modify tasks with status \"merged\"")
	lines = append(lines, "- For group field, use the group ID (lowercase, hyphenated slug of the name)")
	lines = append(lines, "- Groups support hierarchy via the parentGroup field (ID of the parent group, or empty string for top-level)")
	lines = append(lines, "- If creating new groups, add them to newGroups and use their slugified name as the group ID in tasks")
	lines = append(lines, "- To make an existing group a subgroup of another, add it to updatedGroups with the new parentGroup ID")
	lines = append(lines, "- If no new groups are needed, use an empty array for newGroups")
	lines = append(lines, "- If no group hierarchy changes are needed, use an empty array for updatedGroups")
	lines = append(lines, "- Do not remove or add tasks — only modify existing ones")
	lines = append(lines, "")
	lines = append(lines, mcpToolGuidanceGroupAction)
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}

func BuildProofInstructions(projectRoot, taskID, preChangeCommit string) string {
	screenshotsDir := store.ScreenshotsDir(projectRoot)
	beforeFile := taskID + "-before.png"
	afterFile := taskID + "-after.png"

	return fmt.Sprintf(`
## PROOF Screenshot Instructions

This task is tagged with PROOF. After completing all code changes, you MUST capture before/after screenshots using the Chrome browser integration. Do NOT use screencapture or any macOS desktop capture tools.

### Steps:
1. **Complete all code changes first** — finish the implementation fully before proceeding.
2. **Determine the target URL** — find the URL to screenshot from the task plan/description (e.g. localhost dev server page).
3. **Screenshot the "after" state** using Chrome MCP:
   - Start the dev server if needed
   - Call mcp__claude-in-chrome__tabs_context_mcp to get available tabs
   - Create a new tab with mcp__claude-in-chrome__tabs_create_mcp
   - Navigate to the target URL with mcp__claude-in-chrome__navigate
   - Wait briefly for the page to render (mcp__claude-in-chrome__computer action: wait, duration: 2)
   - Take a screenshot with mcp__claude-in-chrome__computer action: screenshot
   - Save the screenshot to: %s/%s
   - To save the screenshot, use mcp__claude-in-chrome__javascript_tool to run:
     document.querySelector('canvas')?.toDataURL() or capture the page via the returned image data
   - Alternatively, use Bash to download/save the screenshot image once captured
4. **Screenshot the "before" state** using git worktree + Chrome:
   - Run: git worktree add /tmp/proof-worktree-%s %s
   - Start a dev server in the worktree on a DIFFERENT port
   - Navigate Chrome to the worktree dev server URL
   - Take a screenshot with mcp__claude-in-chrome__computer action: screenshot
   - Save to: %s/%s
   - Stop the worktree dev server
   - Clean up: git worktree remove /tmp/proof-worktree-%s
5. **Ensure both files exist** at the paths above.

### Screenshot directory: %s
Create the directory if it doesn't exist: mkdir -p %s

### Important:
- Use png format for screenshots
- Do NOT use screencapture or any desktop screen capture — use Chrome MCP exclusively
- The Chrome MCP captures through the browser extension API without taking over the desktop
- If Chrome MCP is unavailable, save placeholder text explaining what to screenshot manually
- Always clean up the git worktree when done
`, screenshotsDir, afterFile,
		taskID, preChangeCommit,
		screenshotsDir, beforeFile,
		taskID,
		screenshotsDir, screenshotsDir)
}

func BuildBulkAddPrompt(projectRoot string, bulkText string, groups []model.Group, groupID string) string {
	var lines []string
	lines = append(lines, "You are a task management assistant. Parse the following freeform text into structured tasks.")
	lines = append(lines, "The text may contain bullet points, numbered lists, plain lines, or any other format.")
	lines = append(lines, "")
	lines = append(lines, "## Input Text")
	lines = append(lines, bulkText)
	lines = append(lines, "")

	if groupID != "" {
		for _, g := range groups {
			if g.ID == groupID {
				lines = append(lines, fmt.Sprintf("## Target Project: %s (ID: %s)", g.Name, g.ID))
				lines = append(lines, "Assign all tasks to this project.")
				lines = append(lines, "")
				break
			}
		}
	}

	lines = append(lines, "## Output Format")
	lines = append(lines, "Output ONLY valid JSON (no markdown code blocks, no preamble, no explanation):")
	lines = append(lines, `{`)
	lines = append(lines, `  "tasks": [`)
	lines = append(lines, `    {"title": "...", "description": "...", "tags": ["..."]}`)
	lines = append(lines, `  ],`)
	lines = append(lines, `  "summary": "brief description of what was parsed"`)
	lines = append(lines, `}`)
	lines = append(lines, "")
	lines = append(lines, "Rules:")
	lines = append(lines, "- Each item in the input should become one task")
	lines = append(lines, "- Title should be concise and actionable")
	lines = append(lines, "- Description can include additional detail from the input, or be empty if the title captures everything")
	lines = append(lines, "- Tags should be inferred from content when appropriate (e.g. \"bug\", \"feature\", \"docs\"), or an empty array")
	lines = append(lines, "- Preserve the user's intent — do not add, remove, or merge items unless they are clearly duplicates")
	return prependContext(projectRoot, strings.Join(lines, "\n"))
}
