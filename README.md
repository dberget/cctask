# cctask

A terminal UI task manager built for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Organize tasks into projects, generate implementation plans with Claude, and launch Claude sessions with full task context.

```
cctask ~/my-project  [4 tasks, 2 projects]
────────────────────────────────────────────────────────

▾ Authentication  (3)          │  t1 · Login flow
    t1  Login flow       ◉     │
    t2  MFA setup        ●     │  Status:    in-progress ◉
    t3  Session mgmt     ●     │  Tags:      auth, backend
                               │  Project:   Authentication
▸ Infrastructure  (2)          │  Plan:      ✓ saved
                               │
  Unassigned                   │  ── Description ─────────
    t5  Fix CI pipeline  ●     │  Implement login flow...
```

## Install

Requires [Go 1.25+](https://go.dev/dl/) and [Claude Code](https://docs.anthropic.com/en/docs/claude-code).

```bash
# Clone and install
git clone https://github.com/davidberget/cctask-go.git
cd cctask-go
make install
```

This builds the binary and copies it to `/opt/homebrew/bin/cctask`. To install elsewhere, run `go build -o cctask .` and move the binary to your PATH.

## Quick start

```bash
cd your-project

# Launch the TUI (auto-initializes .cctask/ on first run)
cctask

# Or use CLI commands directly
cctask add "Implement auth" -d "Add JWT authentication" -t "backend,security"
cctask add "Write tests" -g "authentication"
cctask list
cctask run t1    # opens Claude with task context
```

## CLI commands

| Command | Description |
|---------|-------------|
| `cctask` | Launch the interactive TUI |
| `cctask init` | Initialize `.cctask/` in current directory |
| `cctask add <title>` | Add a task (`-d` description, `-t` tags, `-g` group) |
| `cctask list` | List all tasks with status indicators |
| `cctask run <id>` | Run a task or project with Claude Code interactively |

## TUI keybindings

### Main list view

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate up/down |
| `a` | Add new task |
| `e` | Edit task (or edit plan if one exists) |
| `d` | Delete task or project |
| `s` | Cycle status: pending → in-progress → done |
| `g` | Assign task to a project (or create subgroup when on a group) |
| `r` | Run with Claude Code in a new terminal |
| `p` | Generate or view implementation plan |
| `c` | Group prompt — send an instruction to Claude about all tasks in the selected group |
| `v` | Full-screen task view |
| `m` | Merge/combine plans from multiple tasks |
| `/` | Filter tasks by title, description, ID, or tags |
| `Space` | Collapse/expand a project group |
| `Tab` | Switch focus between task list and processes panel |
| `Enter` | Open detail view for task or project |
| `Esc` | Back to list |
| `Ctrl+C` | Quit |

### Full-screen views (plan, task, project)

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll line by line |
| `d` / `u` | Scroll half-page |
| `G` / `g` | Jump to end / beginning |
| `e` | Edit plan content |
| `r` | Run with Claude |
| `Esc` | Back |

### Plan editor

| Key | Action |
|-----|--------|
| `i` | Enter insert mode |
| `Ctrl+S` | Save |
| `q` / `Esc` | Cancel |

## Features

### Projects and nested groups

Organize tasks into projects. Press `g` on a task to assign it to an existing project, create a new one, or remove the assignment. Press `g` on a project header to create a subgroup underneath it. Groups nest to arbitrary depth — the list indents to show hierarchy, and collapsed parent groups hide all their children and tasks.

Collapse groups with `Space` to keep the list clean. Collapsed groups still show their task count (and subgroup count if any) so you know what's inside.

### Implementation plans

Press `p` on a task or project to generate an implementation plan with Claude. Plans run in the background — you'll see progress in the processes panel on the right. Once generated, view plans with `p` or edit them with `e`.

Combine plans from multiple tasks with `m` to create a unified implementation strategy.

### Group prompts

Press `c` on any project (or on a task to target its group) to send a batch instruction to Claude about all tasks in that scope. Claude processes the tasks and can:

- Fill in missing tags across all tasks
- Regroup tasks into new or different projects
- Review and improve descriptions
- Anything else you describe in the prompt

Results are applied automatically when the background process completes.

### Claude integration

- **`r` (run)**: Opens Claude Code in a new terminal window with full task/project context — description, tags, plan, and related tasks are all included in the system prompt.
- **`c` (ask) in task view**: Ask Claude a follow-up question about a task. The current plan is included as context, and Claude's answer updates the plan.
- **`o` (continue)**: On a completed process, open Claude Code to continue the conversation interactively.

### Background processes

Plan generation, plan combining, and group prompts all run as background processes. The processes panel (right side, toggle with `Tab`) shows live status. Completed processes auto-remove after 5 seconds.

## Project structure

All data lives in `.cctask/` at your project root:

```
.cctask/
├── tasks.json      # Tasks, groups, and metadata
├── config.json     # Optional: model and budget settings
├── plans/          # Generated plan markdown files
└── logs/           # Process execution logs
```

The `tasks.json` file is plain JSON — you can edit it by hand or commit it to version control.

### Configuration

`.cctask/config.json` supports:

```json
{
  "model": "claude-sonnet-4-20250514",
  "budget": 0
}
```

Both fields are optional. `model` sets the default Claude model for background operations. `budget` sets a token limit (0 = unlimited).

## Tips and tricks

- **Start with the TUI.** Just run `cctask` in your project directory. It auto-initializes and you can do everything from the keyboard.
- **Use groups early.** Even with a few tasks, grouping helps Claude give better plans since it sees related tasks together.
- **Collapse what you're not working on.** `Space` on a group folds it away. Groups show task counts so you still know what's inside.
- **Generate plans before running.** Press `p` to get a plan, review it, then press `r` to start Claude with the plan as context. This gives Claude a clear roadmap instead of starting from scratch.
- **Edit plans freely.** Press `e` on a plan view to tweak it. Plans are just markdown files in `.cctask/plans/`.
- **Use group prompts for housekeeping.** Select a project, press `c`, and type things like "add relevant tags to all tasks" or "break the large tasks into smaller subtasks". Claude will batch-update everything.
- **Combine plans for big features.** If you have several related tasks with individual plans, press `m` to merge them into one coherent implementation plan.
- **Filter to focus.** Press `/` and type to filter by title, description, ID, or tags. Useful when the list gets long.
- **Run from CLI for quick adds.** `cctask add "Fix bug" -t "urgent"` is faster than opening the TUI for a single task.
- **Commit `.cctask/` to git.** The JSON files and plans are text-based and diff well. Share task context with your team.

## Requirements

- Go 1.25+ (build only)
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI (`claude` command must be on PATH)
- macOS (terminal spawning uses AppleScript; the TUI itself works cross-platform)

## License

MIT
