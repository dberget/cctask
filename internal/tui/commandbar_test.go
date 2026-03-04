package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(t tea.KeyType, runes ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: t, Runes: runes}
}

func runeMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func typeString(cb CommandBarModel, s string) CommandBarModel {
	for _, r := range s {
		cb, _ = cb.Update(runeMsg(r))
	}
	return cb
}

func TestCommandBarActivateDeactivate(t *testing.T) {
	cb := NewCommandBar()
	if cb.Active {
		t.Fatal("expected inactive on creation")
	}

	cb = cb.Activate()
	if !cb.Active {
		t.Fatal("expected active after Activate()")
	}

	cb, cmd := cb.Update(keyMsg(tea.KeyEscape))
	if cb.Active {
		t.Fatal("expected inactive after Esc")
	}
	if cmd == nil {
		t.Fatal("expected a command from Esc")
	}
	msg := cmd()
	if _, ok := msg.(CommandCancelMsg); !ok {
		t.Fatalf("expected CommandCancelMsg, got %T", msg)
	}
}

func TestCommandBarTyping(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "quit")

	if cb.Input() != "quit" {
		t.Fatalf("expected 'quit', got %q", cb.Input())
	}
}

func TestCommandBarBackspace(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "quit")
	cb, _ = cb.Update(keyMsg(tea.KeyBackspace))

	if cb.Input() != "qui" {
		t.Fatalf("expected 'qui', got %q", cb.Input())
	}
}

func TestCommandBarSubmit(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "quit")

	cb, cmd := cb.Update(keyMsg(tea.KeyEnter))
	if cb.Active {
		t.Fatal("expected inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a command from Enter")
	}
	msg := cmd()
	submitMsg, ok := msg.(CommandSubmitMsg)
	if !ok {
		t.Fatalf("expected CommandSubmitMsg, got %T", msg)
	}
	if submitMsg.Input != "quit" {
		t.Fatalf("expected Input='quit', got %q", submitMsg.Input)
	}
}

func TestCommandBarHistory(t *testing.T) {
	cb := NewCommandBar().Activate()

	// Simulate two previous commands by submitting them
	cb = typeString(cb, "first")
	cb, _ = cb.Update(keyMsg(tea.KeyEnter))

	cb = cb.Activate()
	cb = typeString(cb, "second")
	cb, _ = cb.Update(keyMsg(tea.KeyEnter))

	// Now activate again and navigate history
	cb = cb.Activate()

	// Up should go to "second" (most recent)
	cb, _ = cb.Update(keyMsg(tea.KeyUp))
	if cb.Input() != "second" {
		t.Fatalf("expected 'second', got %q", cb.Input())
	}

	// Up again should go to "first"
	cb, _ = cb.Update(keyMsg(tea.KeyUp))
	if cb.Input() != "first" {
		t.Fatalf("expected 'first', got %q", cb.Input())
	}

	// Down should go back to "second"
	cb, _ = cb.Update(keyMsg(tea.KeyDown))
	if cb.Input() != "second" {
		t.Fatalf("expected 'second', got %q", cb.Input())
	}

	// Down again should restore the draft (empty)
	cb, _ = cb.Update(keyMsg(tea.KeyDown))
	if cb.Input() != "" {
		t.Fatalf("expected empty draft, got %q", cb.Input())
	}
}

func TestCommandBarCtrlW(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "theme dracula")

	cb, _ = cb.Update(keyMsg(tea.KeyCtrlW))
	if cb.Input() != "theme " {
		t.Fatalf("expected 'theme ', got %q", cb.Input())
	}
}

func TestCommandBarCtrlA(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "hello")

	if cb.cursor != 5 {
		t.Fatalf("expected cursor at 5, got %d", cb.cursor)
	}

	cb, _ = cb.Update(keyMsg(tea.KeyCtrlA))
	if cb.cursor != 0 {
		t.Fatalf("expected cursor at 0 after Ctrl+A, got %d", cb.cursor)
	}
}

func TestCommandBarCtrlE(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "hello")

	// Move cursor to start, then Ctrl+E to end
	cb, _ = cb.Update(keyMsg(tea.KeyCtrlA))
	cb, _ = cb.Update(keyMsg(tea.KeyCtrlE))
	if cb.cursor != 5 {
		t.Fatalf("expected cursor at 5 after Ctrl+E, got %d", cb.cursor)
	}
}

func TestCommandBarView(t *testing.T) {
	cb := NewCommandBar().Activate()
	cb = typeString(cb, "quit")

	view := cb.View(80)
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestCommandBarEmptySubmit(t *testing.T) {
	cb := NewCommandBar().Activate()

	// Submit with empty input should still deactivate but input is empty
	cb, cmd := cb.Update(keyMsg(tea.KeyEnter))
	if cb.Active {
		t.Fatal("expected inactive after Enter on empty")
	}
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	submitMsg, ok := msg.(CommandSubmitMsg)
	if !ok {
		t.Fatalf("expected CommandSubmitMsg, got %T", msg)
	}
	if submitMsg.Input != "" {
		t.Fatalf("expected empty Input, got %q", submitMsg.Input)
	}
}

func TestCommandBarNilRegistrySafe(t *testing.T) {
	// Ensure no panic when registry is nil
	cb := NewCommandBar().Activate()
	cb.registry = nil
	cb = typeString(cb, "test")
	if cb.Input() != "test" {
		t.Fatalf("expected 'test', got %q", cb.Input())
	}
}
