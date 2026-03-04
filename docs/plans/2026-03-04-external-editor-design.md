# External $EDITOR Support

## Summary

Add a `V` keybinding to open editable content in the user's `$EDITOR`. The TUI suspends, the user edits in their preferred editor, and changes are picked up on return.

## Contexts

| Context | Content | File strategy |
|---|---|---|
| `ModePlan` | Plan markdown | Direct: `.cctask/plans/{file}.md` |
| `ModeEditPlan` | Plan markdown | Direct: `.cctask/plans/{file}.md` |
| `ModeContextView` | Project context | Direct: `.cctask/context.md` |
| `ModeEditContext` | Project context | Direct: `.cctask/context.md` |
| `ModeTaskView` | Task description | Temp file round-trip |
| `ModeBulkAdd` | Bulk input text | Temp file round-trip |

## Mechanics

- **Editor resolution**: `$EDITOR` -> `$VISUAL` -> `vim`
- **TUI suspension**: `tea.ExecProcess(exec.Command(editor, filepath))`
- **File-backed content**: Open actual file, reload on return
- **Non-file content**: Write to `/tmp/cctask-*.md`, read back on return, clean up
- **Feedback**: Flash message on return ("Changes saved" / "No changes")
