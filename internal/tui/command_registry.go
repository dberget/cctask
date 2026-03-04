package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Command represents a single command that can be executed from the command bar.
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

// CommandRegistry holds all registered commands and provides lookup and completion.
type CommandRegistry struct {
	commands []Command
	aliases  map[string]string // alias -> canonical name
}

// NewCommandRegistry creates an empty CommandRegistry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		aliases: make(map[string]string),
	}
}

// Register adds a command to the registry and indexes its aliases.
func (r *CommandRegistry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd.Name
	}
}

// Lookup finds a command by name or alias. Returns the command and true if found.
func (r *CommandRegistry) Lookup(name string) (Command, bool) {
	canonical := name
	if mapped, ok := r.aliases[name]; ok {
		canonical = mapped
	}
	for _, cmd := range r.commands {
		if cmd.Name == canonical {
			return cmd, true
		}
	}
	return Command{}, false
}

// Complete returns context-aware completions for the given input string.
// If typing the first word, it prefix-matches command names and aliases.
// If the command is known and typing args, it delegates to the appropriate ArgDef.Completer.
func (r *CommandRegistry) Complete(input string) []string {
	fields := strings.Fields(input)
	trailingSpace := len(input) > 0 && input[len(input)-1] == ' '

	// No input or typing first word (no trailing space and at most one field).
	if len(fields) == 0 || (len(fields) == 1 && !trailingSpace) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}
		return r.completeCommandName(prefix)
	}

	// Command is the first field; determine arg position.
	cmdName := fields[0]
	cmd, ok := r.Lookup(cmdName)
	if !ok {
		return nil
	}

	// Determine which arg index we are completing.
	// fields[0] is the command; fields[1..] are args already typed.
	// If there's a trailing space, we're starting a new arg.
	argIndex := len(fields) - 1 // number of arg fields already present
	partial := ""
	if trailingSpace {
		// Starting a new arg after the last field.
		argIndex = len(fields) - 1
		partial = ""
	} else {
		// Still typing the current arg.
		argIndex = len(fields) - 2
		partial = fields[len(fields)-1]
	}

	if argIndex < 0 || argIndex >= len(cmd.Args) {
		return nil
	}

	argDef := cmd.Args[argIndex]
	if argDef.Completer == nil {
		return nil
	}

	return argDef.Completer(partial)
}

// AllCommands returns all registered commands.
func (r *CommandRegistry) AllCommands() []Command {
	result := make([]Command, len(r.commands))
	copy(result, r.commands)
	return result
}

// completeCommandName returns sorted command names and aliases that match the given prefix.
func (r *CommandRegistry) completeCommandName(prefix string) []string {
	var matches []string
	seen := make(map[string]bool)

	for _, cmd := range r.commands {
		if strings.HasPrefix(cmd.Name, prefix) && !seen[cmd.Name] {
			matches = append(matches, cmd.Name)
			seen[cmd.Name] = true
		}
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(alias, prefix) && !seen[alias] {
				matches = append(matches, alias)
				seen[alias] = true
			}
		}
	}

	sort.Strings(matches)
	return matches
}
