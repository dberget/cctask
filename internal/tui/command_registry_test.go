package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRegistryCompleteCommandName(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "quit", Description: "Quit the app"})
	r.Register(Command{Name: "help", Description: "Show help"})
	r.Register(Command{Name: "theme", Description: "Change theme"})

	// Prefix "q" should match only quit.
	got := r.Complete("q")
	if len(got) != 1 || got[0] != "quit" {
		t.Errorf("Complete(\"q\") = %v, want [quit]", got)
	}

	// Empty string should return all three, sorted.
	got = r.Complete("")
	want := []string{"help", "quit", "theme"}
	if len(got) != len(want) {
		t.Fatalf("Complete(\"\") = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Complete(\"\")[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestRegistryCompleteArgs(t *testing.T) {
	themes := []string{"default", "dracula", "nord"}
	completer := func(partial string) []string {
		var matches []string
		for _, th := range themes {
			if strings.HasPrefix(th, partial) {
				matches = append(matches, th)
			}
		}
		return matches
	}

	r := NewCommandRegistry()
	r.Register(Command{
		Name:        "theme",
		Description: "Change theme",
		Args: []ArgDef{
			{Name: "name", Required: true, Completer: completer},
		},
	})

	// "theme d" — typing arg starting with "d".
	got := r.Complete("theme d")
	want := []string{"default", "dracula"}
	if len(got) != len(want) {
		t.Fatalf("Complete(\"theme d\") = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Complete(\"theme d\")[%d] = %q, want %q", i, got[i], w)
		}
	}

	// "theme " — trailing space, show all themes.
	got = r.Complete("theme ")
	if len(got) != 3 {
		t.Fatalf("Complete(\"theme \") = %v, want all 3 themes", got)
	}
	for i, w := range themes {
		if got[i] != w {
			t.Errorf("Complete(\"theme \")[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestRegistryAliases(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{
		Name:    "quit",
		Aliases: []string{"q"},
		Execute: func(args []string) tea.Cmd { return tea.Quit },
	})

	cmd, ok := r.Lookup("q")
	if !ok {
		t.Fatal("Lookup(\"q\") returned false, want true")
	}
	if cmd.Name != "quit" {
		t.Errorf("Lookup(\"q\").Name = %q, want \"quit\"", cmd.Name)
	}
}

func TestRegistryLookupNotFound(t *testing.T) {
	r := NewCommandRegistry()
	r.Register(Command{Name: "help"})

	_, ok := r.Lookup("nonexistent")
	if ok {
		t.Error("Lookup(\"nonexistent\") returned true, want false")
	}
}
