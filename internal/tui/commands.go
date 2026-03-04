package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Command messages — each command's Execute returns a tea.Cmd that emits one of these.

type cmdQuitMsg struct{}
type cmdThemeMsg struct{ name string }
type cmdFilterMsg struct{ text string }
type cmdSortMsg struct{ field string }
type cmdModelMsg struct{ name string }
type cmdExportMsg struct{ format string }
type cmdSetMsg struct{ key, value string }
type cmdCdMsg struct{ group string }
type cmdHelpMsg struct{ command string }

// prefixCompleter returns a completer function that filters options by prefix.
func prefixCompleter(options []string) func(partial string) []string {
	return func(partial string) []string {
		if partial == "" {
			return options
		}
		var matches []string
		for _, o := range options {
			if strings.HasPrefix(o, partial) {
				matches = append(matches, o)
			}
		}
		return matches
	}
}

// registerCommands registers all built-in commands with the given registry.
// getGroups is a closure that returns live group IDs for :cd completion.
func registerCommands(reg *CommandRegistry, getGroups func() []string) {
	// quit (alias: q)
	reg.Register(Command{
		Name:        "quit",
		Aliases:     []string{"q"},
		Description: "Quit the application",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg { return cmdQuitMsg{} }
		},
	})

	// help [command]
	reg.Register(Command{
		Name:        "help",
		Description: "Show help for a command",
		Args: []ArgDef{
			{
				Name:     "command",
				Required: false,
				Completer: func(partial string) []string {
					var names []string
					for _, cmd := range reg.AllCommands() {
						names = append(names, cmd.Name)
					}
					return prefixCompleter(names)(partial)
				},
			},
		},
		Execute: func(args []string) tea.Cmd {
			command := ""
			if len(args) > 0 {
				command = args[0]
			}
			return func() tea.Msg { return cmdHelpMsg{command: command} }
		},
	})

	// theme <name>
	reg.Register(Command{
		Name:        "theme",
		Description: "Switch color theme",
		Args: []ArgDef{
			{
				Name:      "name",
				Required:  true,
				Completer: prefixCompleter(ThemeNames),
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: theme <name>"} }
			}
			name := args[0]
			return func() tea.Msg { return cmdThemeMsg{name: name} }
		},
	})

	// filter [text...]
	reg.Register(Command{
		Name:        "filter",
		Description: "Filter tasks by text",
		Args: []ArgDef{
			{Name: "text", Required: false},
		},
		Execute: func(args []string) tea.Cmd {
			text := strings.Join(args, " ")
			return func() tea.Msg { return cmdFilterMsg{text: text} }
		},
	})

	// sort <field>
	sortFields := []string{"name", "status", "created", "updated"}
	reg.Register(Command{
		Name:        "sort",
		Description: "Sort tasks by field",
		Args: []ArgDef{
			{
				Name:      "field",
				Required:  true,
				Completer: prefixCompleter(sortFields),
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: sort <field>"} }
			}
			field := args[0]
			return func() tea.Msg { return cmdSortMsg{field: field} }
		},
	})

	// model <name>
	modelNames := []string{"sonnet", "opus", "haiku"}
	reg.Register(Command{
		Name:        "model",
		Description: "Switch Claude model",
		Args: []ArgDef{
			{
				Name:      "name",
				Required:  true,
				Completer: prefixCompleter(modelNames),
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: model <name>"} }
			}
			name := args[0]
			return func() tea.Msg { return cmdModelMsg{name: name} }
		},
	})

	// export <format>
	exportFormats := []string{"json", "markdown", "csv"}
	reg.Register(Command{
		Name:        "export",
		Description: "Export tasks to file",
		Args: []ArgDef{
			{
				Name:      "format",
				Required:  true,
				Completer: prefixCompleter(exportFormats),
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: export <format>"} }
			}
			format := args[0]
			return func() tea.Msg { return cmdExportMsg{format: format} }
		},
	})

	// set <key> <value>
	setKeys := []string{"hideCompleted", "timeout", "budget", "disableSkillPicker"}
	reg.Register(Command{
		Name:        "set",
		Description: "Set a configuration value",
		Args: []ArgDef{
			{
				Name:      "key",
				Required:  true,
				Completer: prefixCompleter(setKeys),
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

	// cd <group>
	reg.Register(Command{
		Name:        "cd",
		Description: "Navigate to a group",
		Args: []ArgDef{
			{
				Name:     "group",
				Required: true,
				Completer: func(partial string) []string {
					return prefixCompleter(getGroups())(partial)
				},
			},
		},
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return func() tea.Msg { return FlashMsg{Text: "usage: cd <group>"} }
			}
			group := args[0]
			return func() tea.Msg { return cmdCdMsg{group: group} }
		},
	})
}
