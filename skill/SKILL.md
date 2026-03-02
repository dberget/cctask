# cctask Skill

Task management tool for Claude Code projects. Organizes tasks into hierarchical groups (projects) with nested subgroups, generates implementation plans, and launches Claude sessions with full context.

## Data Schema

### tasks.json

```json
{
  "tasks": [
    {
      "id": "t1",
      "title": "Task title",
      "description": "Detailed description",
      "status": "pending | in-progress | done",
      "tags": ["tag1", "tag2"],
      "group": "group-id",
      "planFile": "t1-task-title.md",
      "created": "2024-01-01T00:00:00Z",
      "updated": "2024-01-01T00:00:00Z"
    }
  ],
  "groups": [
    {
      "id": "group-id",
      "name": "Group Name",
      "description": "Group description",
      "parentGroup": "",
      "planFile": "group-id.md",
      "created": "2024-01-01T00:00:00Z"
    }
  ],
  "combinedPlans": [],
  "nextId": 2
}
```

### Group Hierarchy

Groups support nesting via the `parentGroup` field:
- Top-level groups have `parentGroup: ""` (empty string)
- Subgroups reference their parent group's ID
- No depth limit — tree structure handled by TUI indentation
- Deleting a parent cascades to all descendants (subgroups + unassigns tasks)

### File Locations

```
.cctask/
├── tasks.json      # Tasks, groups, and metadata
├── config.json     # Optional: model and budget settings
├── plans/          # Generated plan markdown files
└── logs/           # Process execution logs
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `cctask` | Launch the interactive TUI |
| `cctask init` | Initialize `.cctask/` in current directory |
| `cctask add <title>` | Add a task (`-d` description, `-t` tags, `-g` group) |
| `cctask list` | List all tasks with status and group path |
| `cctask run <id>` | Run a task or project with Claude Code |

## TUI Keys

- `a` — Add task
- `g` — Assign to group (on task) / Create subgroup (on group)
- `p` — Generate/view plan
- `r` — Run with Claude
- `c` — Group prompt (batch instruction)
- `Space` — Collapse/expand group
- `d` — Delete (cascades for groups with subgroups)
