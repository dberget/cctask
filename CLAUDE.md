# cctask-go

Interactive TUI task manager for Claude Code projects. Built with Go, Bubbletea, and Lipgloss.

## Build & Test

```
go build ./...        # build
go install ./...      # install to PATH
go test ./...         # run tests
```

## Project Structure

```
cmd/                     CLI commands (cobra)
internal/
  model/                 Data types, view modes, task status
  store/                 JSON store, groups, plans (reads/writes .cctask/)
  tui/                   Bubbletea TUI (app.go is the main model + update loop)
  claude/                Spawns `claude -p` background processes
  prompt/                Builds prompts sent to Claude for plans/actions
```

## Key Patterns

- **Bubbletea value receivers**: Model uses value receivers throughout. Mutations happen on copies returned up the chain.
- **View modes**: `model.ViewMode` enum drives rendering and key handling. Fullscreen modes (Plan, TaskView, GroupDetail, ProcessDetail, Help) share scroll logic in `handleNavKey`.
- **Key routing**: `handleKey` dispatches input modes (text, form, select) by mode, then falls through to `handleNavKey` for navigation/actions.
- **Task statuses**: pending → planning → in-progress → done (cycled with `s`).
- **Group hierarchy**: Groups nest via `ParentGroup` field. `store.GetChildGroups`, `GetGroupPath`, `GetAllDescendantGroupIDs` handle traversal.
- **Claude processes**: Background `claude -p` calls stream output via `ProcessOutputMsg` and complete with `ProcessDoneMsg`.
- **Data storage**: All state lives in `.cctask/tasks.json`. Plans are markdown files in `.cctask/plans/`.

## Conventions

- Group/task IDs are slugified lowercase hyphenated strings (max 40 chars)
- Timestamps use RFC3339
- The `renderScrollable` function handles all fullscreen view scrolling — pass the actual available `contentHeight` from `View()`, not a hardcoded estimate
