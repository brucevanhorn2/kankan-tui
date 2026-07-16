package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/brucevanhorn/kankan-tui/internal/board"
)

// TestRapidLocalEditsCoalesceWithoutLosingData reproduces the scenario
// that broke before save dispatch was made single-flight: two edits fired
// in quick succession (faster than the first save's goroutine can
// complete) used to race each other with the same stale base/lastHash,
// causing the second edit to be merged away against the process's own
// prior (not-yet-synced) save instead of just being saved. Nothing
// external ever touches the file in this test.
func TestRapidLocalEditsCoalesceWithoutLosingData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board.json")

	b := board.New("test")
	todoID := b.ColumnOrder[0]
	cardA := b.AddCard(todoID, "A")
	_ = b.AddCard(todoID, "B")
	_ = b.AddCard(todoID, "C")
	hash, err := board.Save(path, b)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}

	m := NewModel(b, path, hash, nil)
	m.colIdx = 0
	m.cardIdx = 0 // cursor on card A

	pressJ := func(m Model) (Model, tea.Cmd) {
		nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
		return nm.(Model), cmd
	}

	// runSaveCmd executes cmd as the Bubble Tea runtime would, unwrapping
	// a tea.Batch (used to bundle the coalesced follow-up save with the
	// status-clear timer) to pull out just the saveResultMsg, without
	// invoking the timer command and blocking the test on it.
	runSaveCmd := func(cmd tea.Cmd) saveResultMsg {
		t.Helper()
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				if sm, ok := c().(saveResultMsg); ok {
					return sm
				}
			}
			t.Fatalf("expected a saveResultMsg among the batched commands")
		}
		sm, ok := msg.(saveResultMsg)
		if !ok {
			t.Fatalf("expected a saveResultMsg, got %T", msg)
		}
		return sm
	}

	// First J: card A moves down one slot (A,B,C -> B,A,C). This dispatches
	// a save that we deliberately don't run yet, simulating it still being
	// in flight when the next keystroke arrives.
	m, cmd1 := pressJ(m)
	if cmd1 == nil {
		t.Fatalf("expected the first J to dispatch a save")
	}
	if !m.saveInFlight {
		t.Fatalf("expected saveInFlight to be true after the first dispatch")
	}

	// Second J, before the first save's goroutine has run at all: card A
	// (now at index 1) moves down again (B,A,C -> B,C,A).
	m, cmd2 := pressJ(m)
	if cmd2 != nil {
		t.Fatalf("expected the second rapid J to be coalesced (nil cmd) instead of dispatching a second save")
	}
	if !m.pendingSave {
		t.Fatalf("expected pendingSave to be set by the second J")
	}

	// Now let the first save's goroutine actually run and feed its result
	// back through Update, the way the Bubble Tea runtime would.
	msg1 := runSaveCmd(cmd1)
	nm, cmd3 := m.Update(msg1)
	m = nm.(Model)
	if cmd3 == nil {
		t.Fatalf("expected the coalesced follow-up save to be dispatched once the first save completed")
	}

	msg2 := runSaveCmd(cmd3)
	nm, _ = m.Update(msg2)
	m = nm.(Model)

	if m.saveInFlight || m.pendingSave {
		t.Fatalf("expected no save in flight or pending after both completed")
	}

	onDisk, _, err := board.Load(path)
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	col := onDisk.Columns[todoID]
	if len(col.CardOrder) != 3 || col.CardOrder[2] != cardA.ID {
		t.Fatalf("expected card A to have moved down twice (ending last in the order), got %v", col.CardOrder)
	}
}
