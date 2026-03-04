# Command Bar Design

Vim-style command bar activated with `:` for power-user operations — configuration, settings, integrations, and deeper operations that don't warrant dedicated keybindings.

## Command Bar Model

New `internal/tui/commandbar.go` with `CommandBarModel`:

- **State**: input buffer, cursor position, active bool, suggestions list, selected suggestion index, history slice, history index
- **Rendering**: Replaces status bar hint line with `:` + input + cursor when active. Floating suggestions popup (max 5 items) renders above as overlay.
- **Activation**: `:` in any navigable mode
- **Deactivation**: `Esc` cancels, `Enter` executes

## Command Registry

```go
type Command struct {
    Name        string
    Description string
    Args        []ArgDef
    Execute     func(args []string) tea.Cmd
}

type ArgDef struct {
    Name      string
    Required  bool
    Completer func(partial string) []string
}
```

Commands registered at app init. Completers are per-argument for context-aware tab completion.

## Initial Commands

| Command | Args | Description |
|---|---|---|
| `:quit` / `:q` | — | Quit app |
| `:help` | `[command]` | List commands or show command help |
| `:theme` | `<name>` | Switch theme |
| `:set` | `<key> <value>` | Change runtime setting |
| `:export` | `<format>` | Export tasks (json, markdown, csv) |
| `:import` | `<file>` | Import tasks from file |
| `:model` | `<name>` | Set Claude model |
| `:sort` | `<field>` | Sort by field |
| `:filter` | `<text>` | Filter tasks |
| `:cd` | `<group>` | Navigate to group |

## Completion & History

- **Tab**: Prefix-match command names, or call arg Completer for arguments
- **Up/Down** (no suggestions): Cycle command history
- **Up/Down** (suggestions visible): Navigate suggestion list
- **History**: Persisted in `.cctask/command_history` (last 100 entries)

## App Integration

- New `ModeCommandBar` view mode
- `handleKey`: route to `commandbar.Update(msg)` when active
- `handleNavKey`: `:` activates command bar
- `View()`: render command bar in place of status bar when active, suggestions overlay above
- Commands receive app state via closures at registration time
