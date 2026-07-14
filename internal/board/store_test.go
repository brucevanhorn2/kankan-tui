package board

import (
	"path/filepath"
	"testing"
)

// TestSaveMerging_ConcurrentWriterIsNotClobbered simulates the scenario
// this whole mechanism exists for: an external process (an "agent") writes
// a change to the board file after this process last loaded it, and this
// process then saves its own independent local change. The naive
// overwrite-the-whole-file approach would silently discard whichever side
// saved second; SaveMerging must preserve both.
func TestSaveMerging_ConcurrentWriterIsNotClobbered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board.json")

	b := New("shared board")
	todoID := b.ColumnOrder[0]
	initialHash, err := Save(path, b)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}

	// This process's in-memory state, frozen at load time.
	base := b.Clone()
	ours := b.Clone()
	ourCard := ours.AddCard(todoID, "local edit")

	// An external agent independently loads the same on-disk state,
	// makes its own change, and saves - all before we save ours.
	agentBoard, agentHash, err := Load(path)
	if err != nil {
		t.Fatalf("agent load: %v", err)
	}
	agentCard := agentBoard.AddCard(todoID, "agent edit")
	if _, err := Save(path, agentBoard); err != nil {
		t.Fatalf("agent save: %v", err)
	}
	_ = agentHash

	// Now we save, still holding the pre-agent-write hash as our
	// "lastHash" - exactly the situation that caused silent data loss
	// before SaveMerging existed.
	toWrite, merged, _, err := SaveMerging(path, base, ours, initialHash)
	if err != nil {
		t.Fatalf("SaveMerging: %v", err)
	}
	if !merged {
		t.Fatalf("expected a merge to have been triggered by the agent's concurrent write")
	}

	if _, ok := toWrite.Cards[ourCard.ID]; !ok {
		t.Fatalf("our local edit was lost")
	}
	if _, ok := toWrite.Cards[agentCard.ID]; !ok {
		t.Fatalf("the agent's concurrent edit was lost")
	}

	// And what's actually on disk now must reflect both, not just ours.
	onDisk, _, err := Load(path)
	if err != nil {
		t.Fatalf("final load: %v", err)
	}
	if len(onDisk.Cards) != 2 {
		t.Fatalf("expected 2 cards on disk after merge, got %d", len(onDisk.Cards))
	}
}

func TestSaveMerging_NoConcurrentWriteIsPlainOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "board.json")

	b := New("solo board")
	hash, err := Save(path, b)
	if err != nil {
		t.Fatalf("initial save: %v", err)
	}

	base := b.Clone()
	ours := b.Clone()
	ours.AddColumn("Backlog")

	toWrite, merged, _, err := SaveMerging(path, base, ours, hash)
	if err != nil {
		t.Fatalf("SaveMerging: %v", err)
	}
	if merged {
		t.Fatalf("expected no merge when nobody else wrote to the file")
	}
	if len(toWrite.ColumnOrder) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(toWrite.ColumnOrder))
	}
}
