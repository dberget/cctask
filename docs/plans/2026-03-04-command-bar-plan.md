# Command Bar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a vim-style command bar (`:`) with tab-completion, argument-aware suggestions, and command history for power-user configuration/settings operations.

**Architecture:** New `CommandBarModel` in `internal/tui/commandbar.go` with a `CommandRegistry` pattern. Commands are registered structs with per-argument completers. The command bar replaces the status bar when active, with a floating suggestions popup above it. A new `ModeCommandBar` view mode gates key routing.

**Tech Stack:** Go, Bubbletea, Lipgloss, bubbles/textinput

---

### Task 1: Add ModeCommandBar to ViewMode enum

**Files:**
- Modify: `internal/model/viewmode.go`

**Step 1: Add ModeCommandBar constant**

Add `ModeCommandBar` after `ModeSkillPicker` in the const block:

```go
ModeSkillPicker
ModeCommandBar
```

**Step 2: Add String() case**

```go
case ModeCommandBar:
    return "command-bar"
```

**Step 3: Commit**

```bash
git add internal/model/viewmode.go
git commit -m "feat: add ModeCommandBar view mode"
```

---

### Task 2: Create CommandBarModel core — types, Update, View

**Files:**
- Create: `internal/tui/commandbar.go`

**Step 1: Write the test file**

Create `internal/tui/commandbar_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandBarActivateDeactivate(t *testing.T) {
	cb := NewCommandBar()
	if cb.Active {
		t.Fatal("should start inactive")
	}
	cb = cb.Activate()
	if !cb.Active {
		t.Fatal("should be active after Activate")
	}
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cb.Active {
		t.Fatal("should deactivate on Esc")
	}
}

func TestCommandBarTyping(t *testing.T) {
	cb := NewCommandBar()
	cb = cb.Activate()

	// Type "quit"
	for _, ch := range "quit" {
		cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if cb.Input() != "quit" {
		t.Fatalf("expected 'quit', got %q", cb.Input())
	}
}

func TestCommandBarBackspace(t *testing.T) {
	cb := NewCommandBar()
	cb = cb.Activate()
	for _, ch := range "quit" {
		cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cb.Input() != "qui" {
		t.Fatalf("expected 'qui', got %q", cb.Input())
	}
}

func TestCommandBarSubmit(t *testing.T) {
	cb := NewCommandBar()
	cb = cb.Activate()
	for _, ch := range "quit" {
		cb, _ = cb.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	var cmd tea.Cmd
	cb, cmd = cb.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cb.Active {
		t.Fatal("should deactivate on Enter")
	}
	if cmd == nil {
		t.Fatal("should return a command on Enter")
	}
	msg := cmd()
	submit, ok := msg.(CommandSubmitMsg)
	if !ok {
		t.Fatalf("expected CommandSubmitMsg, got %T", msg)
	}
	if submit.Input != "quit" {
		t.Fatalf("expected 'quit', got %q", submit.Input)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run TestCommandBar -v`
Expected: FAIL (types don't exist yet)

**Step 3: Write commandbar.go**

Create `internal/tui/commandbar.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommandSubmitMsg is sent when the user presses Enter in the command bar.
type CommandSubmitMsg struct{ Input string }

// CommandCancelMsg is sent when the user presses Esc in the command bar.
type CommandCancelMsg struct{}

// CommandBarModel is a vim-style command bar with completion and history.
type CommandBarModel struct {
	Active bool

	input  []rune
	cursor int

	// Suggestion state
	suggestions   []string
	suggestionIdx int
	showSuggestions bool

	// History
	history      []string
	historyIdx   int
	historyDraft string // stash current input when browsing history

	// Registry (set externally)
	registry *CommandRegistry
}

func NewCommandBar() CommandBarModel {
	return CommandBarModel{
		registry: NewCommandRegistry(),
	}
}

func (cb CommandBarModel) Activate() CommandBarModel {
	cb.Active = true
	cb.input = nil
	cb.cursor = 0
	cb.suggestions = nil
	cb.suggestionIdx = 0
	cb.showSuggestions = false
	cb.historyIdx = -1
	cb.historyDraft = ""
	return cb
}

func (cb CommandBarModel) Input() string {
	return string(cb.input)
}

func (cb CommandBarModel) Update(msg tea.KeyMsg) (CommandBarModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		cb.Active = false
		cb.suggestions = nil
		return cb, func() tea.Msg { return CommandCancelMsg{} }

	case tea.KeyEnter:
		// If suggestion is selected, accept it first
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			cb = cb.acceptSuggestion()
			return cb, nil
		}
		input := strings.TrimSpace(string(cb.input))
		cb.Active = false
		cb.suggestions = nil
		if input == "" {
			return cb, func() tea.Msg { return CommandCancelMsg{} }
		}
		cb = cb.addHistory(input)
		return cb, func() tea.Msg { return CommandSubmitMsg{Input: input} }

	case tea.KeyTab:
		cb = cb.completeCycle(1)
		return cb, nil

	case tea.KeyShiftTab:
		cb = cb.completeCycle(-1)
		return cb, nil

	case tea.KeyUp:
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			if cb.suggestionIdx > 0 {
				cb.suggestionIdx--
			}
		} else {
			cb = cb.historyPrev()
		}
		return cb, nil

	case tea.KeyDown:
		if cb.showSuggestions && len(cb.suggestions) > 0 {
			if cb.suggestionIdx < len(cb.suggestions)-1 {
				cb.suggestionIdx++
			}
		} else {
			cb = cb.historyNext()
		}
		return cb, nil

	case tea.KeyBackspace:
		if cb.cursor > 0 {
			cb.input = append(cb.input[:cb.cursor-1], cb.input[cb.cursor:]...)
			cb.cursor--
			cb = cb.updateSuggestions()
		}
		return cb, nil

	case tea.KeyCtrlA:
		cb.cursor = 0
		return cb, nil

	case tea.KeyCtrlE:
		cb.cursor = len(cb.input)
		return cb, nil

	case tea.KeyCtrlW:
		// Delete word backwards
		if cb.cursor > 0 {
			i := cb.cursor - 1
			for i > 0 && cb.input[i-1] == ' ' {
				i--
			}
			for i > 0 && cb.input[i-1] != ' ' {
				i--
			}
			cb.input = append(cb.input[:i], cb.input[cb.cursor:]...)
			cb.cursor = i
			cb = cb.updateSuggestions()
		}
		return cb, nil

	case tea.KeyRunes:
		runes := msg.Runes
		newInput := make([]rune, 0, len(cb.input)+len(runes))
		newInput = append(newInput, cb.input[:cb.cursor]...)
		newInput = append(newInput, runes...)
		newInput = append(newInput, cb.input[cb.cursor:]...)
		cb.input = newInput
		cb.cursor += len(runes)
		cb = cb.updateSuggestions()
		return cb, nil
	}

	return cb, nil
}

func (cb CommandBarModel) addHistory(input string) CommandBarModel {
	// Don't add duplicates of last entry
	if len(cb.history) > 0 && cb.history[len(cb.history)-1] == input {
		return cb
	}
	cb.history = append(cb.history, input)
	if len(cb.history) > 100 {
		cb.history = cb.history[len(cb.history)-100:]
	}
	return cb
}

func (cb CommandBarModel) historyPrev() CommandBarModel {
	if len(cb.history) == 0 {
		return cb
	}
	if cb.historyIdx == -1 {
		cb.historyDraft = string(cb.input)
		cb.historyIdx = len(cb.history) - 1
	} else if cb.historyIdx > 0 {
		cb.historyIdx--
	}
	cb.input = []rune(cb.history[cb.historyIdx])
	cb.cursor = len(cb.input)
	cb.suggestions = nil
	cb.showSuggestions = false
	return cb
}

func (cb CommandBarModel) historyNext() CommandBarModel {
	if cb.historyIdx == -1 {
		return cb
	}
	if cb.historyIdx < len(cb.history)-1 {
		cb.historyIdx++
		cb.input = []rune(cb.history[cb.historyIdx])
	} else {
		cb.historyIdx = -1
		cb.input = []rune(cb.historyDraft)
	}
	cb.cursor = len(cb.input)
	cb.suggestions = nil
	cb.showSuggestions = false
	return cb
}

// updateSuggestions recomputes suggestions based on current input.
func (cb CommandBarModel) updateSuggestions() CommandBarModel {
	if cb.registry == nil {
		cb.suggestions = nil
		cb.showSuggestions = false
		return cb
	}
	cb.suggestions = cb.registry.Complete(string(cb.input))
	cb.suggestionIdx = 0
	cb.showSuggestions = len(cb.suggestions) > 0
	return cb
}

// completeCycle applies the selected suggestion or cycles through them.
func (cb CommandBarModel) completeCycle(dir int) CommandBarModel {
	if !cb.showSuggestions || len(cb.suggestions) == 0 {
		cb = cb.updateSuggestions()
		if len(cb.suggestions) == 1 {
			cb = cb.acceptSuggestion()
		}
		return cb
	}
	cb.suggestionIdx += dir
	if cb.suggestionIdx < 0 {
		cb.suggestionIdx = len(cb.suggestions) - 1
	} else if cb.suggestionIdx >= len(cb.suggestions) {
		cb.suggestionIdx = 0
	}
	return cb
}

// acceptSuggestion replaces the current token with the selected suggestion.
func (cb CommandBarModel) acceptSuggestion() CommandBarModel {
	if len(cb.suggestions) == 0 {
		return cb
	}
	suggestion := cb.suggestions[cb.suggestionIdx]
	input := string(cb.input)
	parts := strings.Fields(input)

	if len(parts) == 0 {
		// No input yet, set whole thing
		cb.input = []rune(suggestion + " ")
	} else if strings.HasSuffix(input, " ") {
		// Cursor after space — completing a new argument
		cb.input = []rune(input + suggestion + " ")
	} else {
		// Replace last token
		parts[len(parts)-1] = suggestion
		cb.input = []rune(strings.Join(parts, " ") + " ")
	}
	cb.cursor = len(cb.input)
	cb.suggestions = nil
	cb.showSuggestions = false
	return cb
}

// View renders the command bar (the input line only).
func (cb CommandBarModel) View(width int) string {
	if !cb.Active {
		return ""
	}
	prompt := styleCyanBold.Render(":")
	inputStr := string(cb.input)

	// Render with cursor
	var display string
	if cb.cursor < len(cb.input) {
		before := string(cb.input[:cb.cursor])
		cursorChar := string(cb.input[cb.cursor])
		after := string(cb.input[cb.cursor+1:])
		display = before + styleCursor.Render(cursorChar) + after
	} else {
		display = inputStr + styleCursor.Render(" ")
	}

	return prompt + display
}

// SuggestionsView renders the floating suggestion popup.
func (cb CommandBarModel) SuggestionsView(width int) string {
	if !cb.showSuggestions || len(cb.suggestions) == 0 {
		return ""
	}

	maxShow := 5
	suggestions := cb.suggestions
	if len(suggestions) > maxShow {
		suggestions = suggestions[:maxShow]
	}

	var lines []string
	for i, s := range suggestions {
		if i == cb.suggestionIdx {
			lines = append(lines, styleCyanBold.Render(" > "+s))
		} else {
			lines = append(lines, styleGray.Render("   "+s))
		}
	}

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))

	return box
}
```

**Step 4: Run tests**

Run: `go test ./internal/tui/ -run TestCommandBar -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/commandbar.go internal/tui/commandbar_test.go
git commit -m "feat: add CommandBarModel with input, history, suggestions"
```

---

### Task 3: Create CommandRegistry with completion logic

**Files:**
- Create: `internal/tui/command_registry.go`

**Step 1: Write tests**

Create `internal/tui/command_registry_test.go`:

```go
package tui

import (
	"testing"
)

func TestRegistryCompleteCommandName(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "quit"})
	r.Register(Command{Name: "help"})
	r.Register(Command{Name: "theme"})

	got := r.Complete("q")
	if len(got) != 1 || got[0] != "quit" {
		t.Fatalf("expected [quit], got %v", got)
	}

	got = r.Complete("th")
	if len(got) != 1 || got[0] != "theme" {
		t.Fatalf("expected [theme], got %v", got)
	}

	got = r.Complete("")
	if len(got) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(got))
	}
}

func TestRegistryCompleteArgs(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{
		Name: "theme",
		Args: []ArgDef{{
			Name:      "name",
			Completer: func(partial string) []string {
				all := []string{"default", "dracula", "nord"}
				if partial == "" {
					return all
				}
				var out []string
				for _, a := range all {
					if len(a) >= len(partial) && a[:len(partial)] == partial {
						out = append(out, a)
					}
				}
				return out
			},
		}},
	})

	got := r.Complete("theme d")
	if len(got) != 2 { // default, dracula
		t.Fatalf("expected 2, got %v", got)
	}

	got = r.Complete("theme ")
	if len(got) != 3 {
		t.Fatalf("expected 3 completions, got %v", got)
	}
}

func TestRegistryAliases(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "quit", Aliases: []string{"q"}})

	got := r.Complete("q")
	if len(got) != 1 || got[0] != "q" {
		t.Fatalf("expected [q], got %v", got)
	}

	cmd, ok := r.Lookup("q")
	if !ok || cmd.Name != "quit" {
		t.Fatalf("expected quit command via alias, got %v", cmd)
	}
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./internal/tui/ -run TestRegistry -v`
Expected: FAIL

**Step 3: Write command_registry.go**

Create `internal/tui/command_registry.go`:

```go
package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Command defines a command available in the command bar.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Args        []ArgDef
	Execute     func(args []string) tea.Cmd
}

// ArgDef defines a positional argument for a command.
type ArgDef struct {
	Name      string
	Required  bool
	Completer func(partial string) []string
}

// CommandRegistry holds all registered commands.
type CommandRegistry struct {
	commands []Command
	aliases  map[string]string // alias -> canonical name
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		aliases: make(map[string]string),
	}
}

func (r *CommandRegistry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
	for _, a := range cmd.Aliases {
		r.aliases[a] = cmd.Name
	}
}

// Lookup finds a command by name or alias.
func (r *CommandRegistry) Lookup(name string) (Command, bool) {
	// Check alias first
	if canonical, ok := r.aliases[name]; ok {
		name = canonical
	}
	for _, cmd := range r.commands {
		if cmd.Name == name {
			return cmd, true
		}
		for _, a := range cmd.Aliases {
			if a == name {
				return cmd, true
			}
		}
	}
	return Command{}, false
}

// Complete returns suggestions for the given input.
func (r *CommandRegistry) Complete(input string) []string {
	parts := strings.Fields(input)
	trailingSpace := strings.HasSuffix(input, " ")

	// No input or typing first word — complete command names
	if len(parts) == 0 || (len(parts) == 1 && !trailingSpace) {
		prefix := ""
		if len(parts) == 1 {
			prefix = parts[0]
		}
		return r.completeCommandName(prefix)
	}

	// We have a command — complete its arguments
	cmdName := parts[0]
	cmd, ok := r.Lookup(cmdName)
	if !ok {
		return nil
	}

	// Which argument position are we completing?
	argIdx := len(parts) - 1
	if trailingSpace {
		argIdx = len(parts)
	}
	// Subtract 1 for the command name itself
	argIdx--

	if argIdx < 0 || argIdx >= len(cmd.Args) {
		return nil
	}

	arg := cmd.Args[argIdx]
	if arg.Completer == nil {
		return nil
	}

	partial := ""
	if !trailingSpace && len(parts) > 1 {
		partial = parts[len(parts)-1]
	}

	return arg.Completer(partial)
}

func (r *CommandRegistry) completeCommandName(prefix string) []string {
	var matches []string
	seen := make(map[string]bool)

	for _, cmd := range r.commands {
		if strings.HasPrefix(cmd.Name, prefix) && !seen[cmd.Name] {
			matches = append(matches, cmd.Name)
			seen[cmd.Name] = true
		}
		for _, a := range cmd.Aliases {
			if strings.HasPrefix(a, prefix) && !seen[a] {
				matches = append(matches, a)
				seen[a] = true
			}
		}
	}
	sort.Strings(matches)
	return matches
}

// AllCommands returns all registered commands (for :help).
func (r *CommandRegistry) AllCommands() []Command {
	return r.commands
}
```

**Step 4: Run tests**

Run: `go test ./internal/tui/ -run TestRegistry -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/command_registry.go internal/tui/command_registry_test.go
git commit -m "feat: add CommandRegistry with completion and aliases"
```

---

### Task 4: Register initial commands

**Files:**
- Create: `internal/tui/commands.go`

**Step 1: Write commands.go**

This file registers all commands. Commands that need app state use the `CommandResult` pattern — they return messages that `app.go` handles.

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Command result messages — handled in app.go Update()

type cmdQuitMsg struct{}
type cmdThemeMsg struct{ name string }
type cmdFilterMsg struct{ text string }
type cmdSortMsg struct{ field string }
type cmdModelMsg struct{ name string }
type cmdExportMsg struct{ format string }
type cmdSetMsg struct{ key, value string }
type cmdCdMsg struct{ group string }
type cmdHelpMsg struct{ command string }

// registerCommands sets up all built-in commands on the registry.
// Completers that need live state (groups, etc.) use closures over accessor funcs.
func registerCommands(reg *CommandRegistry, getGroups func() []string) {
	reg.Register(Command{
		Name:        "quit",
		Aliases:     []string{"q"},
		Description: "Quit the application",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg { return cmdQuitMsg{} }
		},
	})

	reg.Register(Command{
		Name:        "help",
		Description: "Show available commands or help for a specific command",
		Args: []ArgDef{{
			Name: "command",
			Completer: func(partial string) []string {
				var names []string
				for _, cmd := range reg.AllCommands() {
					if strings.HasPrefix(cmd.Name, partial) {
						names = append(names, cmd.Name)
					}
				}
				return names
			},
		}},
		Execute: func(args []string) tea.Cmd {
			cmd := ""
			if len(args) > 0 {
				cmd = args[0]
			}
			return func() tea.Msg { return cmdHelpMsg{command: cmd} }
		},
	})

	reg.Register(Command{
		Name:        "theme",
		Description: "Switch color theme",
		Args: []ArgDef{{
			Name:     "name",
			Required: true,
			Completer: func(partial string) []string {
				var out []string
				for _, n := range ThemeNames {
					if strings.HasPrefix(n, partial) {
						out = append(out, n)
					}
				}
				return out
			},
		}},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: theme <name>"} }
			}
			return func() tea.Msg { return cmdThemeMsg{name: args[0]} }
		},
	})

	reg.Register(Command{
		Name:        "filter",
		Description: "Filter tasks by text",
		Args: []ArgDef{{
			Name: "text",
		}},
		Execute: func(args []string) tea.Cmd {
			text := strings.Join(args, " ")
			return func() tea.Msg { return cmdFilterMsg{text: text} }
		},
	})

	reg.Register(Command{
		Name:        "sort",
		Description: "Sort tasks by field",
		Args: []ArgDef{{
			Name:     "field",
			Required: true,
			Completer: func(partial string) []string {
				fields := []string{"name", "status", "created", "updated"}
				var out []string
				for _, f := range fields {
					if strings.HasPrefix(f, partial) {
						out = append(out, f)
					}
				}
				return out
			},
		}},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: sort <field>"} }
			}
			return func() tea.Msg { return cmdSortMsg{field: args[0]} }
		},
	})

	reg.Register(Command{
		Name:        "model",
		Description: "Set Claude model for runs",
		Args: []ArgDef{{
			Name:     "name",
			Required: true,
			Completer: func(partial string) []string {
				models := []string{"sonnet", "opus", "haiku"}
				var out []string
				for _, m := range models {
					if strings.HasPrefix(m, partial) {
						out = append(out, m)
					}
				}
				return out
			},
		}},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: model <name>"} }
			}
			return func() tea.Msg { return cmdModelMsg{name: args[0]} }
		},
	})

	reg.Register(Command{
		Name:        "export",
		Description: "Export tasks to file",
		Args: []ArgDef{{
			Name:     "format",
			Required: true,
			Completer: func(partial string) []string {
				formats := []string{"json", "markdown", "csv"}
				var out []string
				for _, f := range formats {
					if strings.HasPrefix(f, partial) {
						out = append(out, f)
					}
				}
				return out
			},
		}},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: export <format>"} }
			}
			return func() tea.Msg { return cmdExportMsg{format: args[0]} }
		},
	})

	reg.Register(Command{
		Name:        "set",
		Description: "Change a runtime setting",
		Args: []ArgDef{
			{
				Name:     "key",
				Required: true,
				Completer: func(partial string) []string {
					keys := []string{"hideCompleted", "timeout", "budget", "disableSkillPicker"}
					var out []string
					for _, k := range keys {
						if strings.HasPrefix(k, partial) {
							out = append(out, k)
						}
					}
					return out
				},
			},
			{
				Name:     "value",
				Required: true,
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) < 2 {
				return func() tea.Msg { return FlashMsg{Text: "usage: set <key> <value>"} }
			}
			return func() tea.Msg { return cmdSetMsg{key: args[0], value: args[1]} }
		},
	})

	reg.Register(Command{
		Name:        "cd",
		Description: "Navigate to a group",
		Args: []ArgDef{{
			Name:     "group",
			Required: true,
			Completer: func(partial string) []string {
				groups := getGroups()
				if partial == "" {
					return groups
				}
				var out []string
				for _, g := range groups {
					if strings.HasPrefix(g, partial) {
						out = append(out, g)
					}
				}
				return out
			},
		}},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: cd <group>"} }
			}
			return func() tea.Msg { return cmdCdMsg{group: args[0]} }
		},
	})

	_ = fmt // silence unused import if needed
}
```

**Step 2: Commit**

```bash
git add internal/tui/commands.go
git commit -m "feat: register initial command set with completers"
```

---

### Task 5: Wire command bar into app.go — activation, routing, rendering

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/statusbar.go`

**Step 1: Add commandBar field to Model struct**

In `app.go` Model struct (around line 87, after `chatInput`):

```go
commandBar  CommandBarModel
```

**Step 2: Initialize command bar in NewModel**

In `NewModel()`, after the existing sub-model setup, add:

```go
cmdBar := NewCommandBar()
// Register commands — getGroups closure provides live group names
registerCommands(cmdBar.registry, func() []string {
    var names []string
    for _, g := range s.Groups {
        names = append(names, g.ID)
    }
    return names
})
```

Then assign `cmdBar` to `m.commandBar` (where `m` is the Model being built).

**Step 3: Add ModeCommandBar to handleKey dispatch**

In `handleKey()`, add a case before the fallthrough to `handleNavKey`:

```go
case model.ModeCommandBar:
    var cmd tea.Cmd
    m.commandBar, cmd = m.commandBar.Update(msg)
    return m, cmd
```

**Step 4: Add `:` handler in handleNavKey**

In `handleNavKey()`, after the `?` help toggle block and before the Esc handler, add:

```go
if k == ":" && m.mode.IsNavigable() {
    m.commandBar = m.commandBar.Activate()
    m.mode = model.ModeCommandBar
    return m, nil
}
```

**Step 5: Handle command result messages in Update()**

In the main `Update()` `switch msg := msg.(type)` block, add cases for all `cmd*Msg` types:

```go
case CommandSubmitMsg:
    return m.executeCommand(msg.Input)

case CommandCancelMsg:
    m.mode = model.ModeList
    return m, nil

case cmdQuitMsg:
    m.processCancels.CancelAll()
    return m, tea.Quit

case cmdThemeMsg:
    ApplyTheme(msg.name)
    applyThemeToBubbles(&m)
    cfg := store.LoadConfig(m.projectRoot)
    cfg.Theme = msg.name
    store.SaveConfig(m.projectRoot, cfg)
    m.mode = model.ModeList
    return m, flashCmd("Theme: " + msg.name)

case cmdFilterMsg:
    m.filter = msg.text
    m.mode = model.ModeList
    m.refreshListItems()
    return m, flashCmd("Filter: " + msg.text)

case cmdSortMsg:
    // Store sort preference, apply to list
    m.mode = model.ModeList
    return m, flashCmd("Sort: " + msg.field)

case cmdModelMsg:
    cfg := store.LoadConfig(m.projectRoot)
    cfg.Model = msg.name
    store.SaveConfig(m.projectRoot, cfg)
    m.mode = model.ModeList
    return m, flashCmd("Model: " + msg.name)

case cmdExportMsg:
    m.mode = model.ModeList
    return m.handleExport(msg.format)

case cmdSetMsg:
    m.mode = model.ModeList
    return m.handleSet(msg.key, msg.value)

case cmdCdMsg:
    m.mode = model.ModeList
    return m.handleCd(msg.group)

case cmdHelpMsg:
    m.mode = model.ModeList
    return m.handleCommandHelp(msg.command)
```

**Step 6: Add executeCommand method**

```go
func (m Model) executeCommand(input string) (tea.Model, tea.Cmd) {
    parts := strings.Fields(input)
    if len(parts) == 0 {
        m.mode = model.ModeList
        return m, nil
    }
    cmdName := parts[0]
    args := parts[1:]

    cmd, ok := m.commandBar.registry.Lookup(cmdName)
    if !ok {
        m.mode = model.ModeList
        return m, flashCmd("Unknown command: " + cmdName)
    }
    m.mode = model.ModeList
    return m, cmd.Execute(args)
}
```

**Step 7: Add stub handlers for export, set, cd, commandHelp**

```go
func (m Model) handleExport(format string) (tea.Model, tea.Cmd) {
    // TODO: implement export
    return m, flashCmd("Export " + format + " not yet implemented")
}

func (m Model) handleSet(key, value string) (tea.Model, tea.Cmd) {
    // TODO: implement set
    return m, flashCmd("Set " + key + "=" + value)
}

func (m Model) handleCd(group string) (tea.Model, tea.Cmd) {
    // Navigate to group in list
    for i, item := range m.listItems {
        if item.Group != nil && item.Group.ID == group {
            m.listIndex = i
            m.selectedItem = &m.listItems[i]
            return m, flashCmd("Navigated to " + group)
        }
    }
    return m, flashCmd("Group not found: " + group)
}

func (m Model) handleCommandHelp(command string) (tea.Model, tea.Cmd) {
    if command == "" {
        var lines []string
        for _, cmd := range m.commandBar.registry.AllCommands() {
            lines = append(lines, cmd.Name+" — "+cmd.Description)
        }
        return m, flashCmd(strings.Join(lines, " | "))
    }
    cmd, ok := m.commandBar.registry.Lookup(command)
    if !ok {
        return m, flashCmd("Unknown command: " + command)
    }
    desc := cmd.Name + " — " + cmd.Description
    if len(cmd.Args) > 0 {
        var argNames []string
        for _, a := range cmd.Args {
            argNames = append(argNames, a.Name)
        }
        desc += " (args: " + strings.Join(argNames, ", ") + ")"
    }
    return m, flashCmd(desc)
}
```

**Step 8: Update View() to render command bar**

In `View()`, modify the status bar section. When `m.mode == model.ModeCommandBar`, render the command bar instead of the normal status bar. The suggestions popup renders above the input line:

Replace the `statusBar` / `statusRendered` block with:

```go
var statusRendered string
if m.mode == model.ModeCommandBar {
    barView := m.commandBar.View(m.width)
    sugView := m.commandBar.SuggestionsView(m.width)
    if sugView != "" {
        statusRendered = lipgloss.NewStyle().PaddingLeft(2).Render(sugView) + "\n" +
            lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1).Render(barView)
    } else {
        statusRendered = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1).Render(barView)
    }
} else {
    statusBar := renderStatusBar(m.helpModel, m.keys, m.mode, m.selectedItem, m.message, m.width)
    statusRendered = lipgloss.NewStyle().PaddingLeft(2).PaddingBottom(1).Render(statusBar)
}
```

**Step 9: Update statusbar.go to show `:` hint in navigable modes**

In `modeShortHelp()`, for `ModeList` case (or the default navigable modes), add a hint for `:`:

Add to the returned bindings for list mode:
```go
key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
```

**Step 10: Update renderHelp to document command bar**

In `renderHelp()`, add a section:

```go
lines = append(lines, hdr("Command Bar"))
lines = append(lines, k(":", "Open command bar"))
lines = append(lines, k("Tab", "Complete command/arg"))
lines = append(lines, k("Up/Down", "History / suggestions"))
lines = append(lines, k("Enter", "Execute"))
lines = append(lines, k("Esc", "Cancel"))
lines = append(lines, "")
```

**Step 11: Run full test suite**

Run: `go test ./... -v`
Expected: PASS

**Step 12: Commit**

```bash
git add internal/tui/app.go internal/tui/statusbar.go
git commit -m "feat: wire command bar into app — activation, routing, rendering"
```

---

### Task 6: Add command history persistence

**Files:**
- Modify: `internal/tui/commandbar.go`
- Modify: `internal/tui/app.go`

**Step 1: Add history load/save functions**

In `commandbar.go`, add:

```go
func LoadHistory(projectRoot string) []string {
    data, err := os.ReadFile(filepath.Join(projectRoot, ".cctask", "command_history"))
    if err != nil {
        return nil
    }
    lines := strings.Split(strings.TrimSpace(string(data)), "\n")
    if len(lines) == 1 && lines[0] == "" {
        return nil
    }
    return lines
}

func SaveHistory(projectRoot string, history []string) {
    dir := filepath.Join(projectRoot, ".cctask")
    os.MkdirAll(dir, 0o755)
    data := strings.Join(history, "\n") + "\n"
    os.WriteFile(filepath.Join(dir, "command_history"), []byte(data), 0o644)
}
```

**Step 2: Load history in NewModel**

After creating the command bar in `NewModel`, load history:

```go
cmdBar.history = LoadHistory(projectRoot)
```

**Step 3: Save history on command submit**

In the `CommandSubmitMsg` handler in `Update()`, after `executeCommand`, save:

```go
SaveHistory(m.projectRoot, m.commandBar.history)
```

**Step 4: Commit**

```bash
git add internal/tui/commandbar.go internal/tui/app.go
git commit -m "feat: persist command bar history to .cctask/command_history"
```

---

### Task 7: Build and manual test

**Step 1: Build**

```bash
go build ./...
```

**Step 2: Install and copy**

```bash
go install ./... && cp ~/go/bin/cctask-go /opt/homebrew/bin/cctask
```

**Step 3: Manual test checklist**

- Press `:` — command bar appears at bottom
- Type `th` + Tab — completes to `theme `
- Type `dra` + Tab — completes to `dracula`
- Press Enter — theme switches
- Press `:` + Up arrow — cycles through history
- `:q` + Enter — quits
- `:help` — shows command list
- `:model opus` — sets model
- Esc — cancels command bar

**Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: command bar polish from manual testing"
```
