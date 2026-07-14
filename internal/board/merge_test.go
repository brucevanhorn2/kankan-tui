package board

import (
	"testing"
	"time"
)

func newTestBoard() *Board {
	b := New("test")
	return b
}

func TestMerge_DisjointCardAdditionsBothSurvive(t *testing.T) {
	base := newTestBoard()
	todoID := base.ColumnOrder[0]

	ours := base.Clone()
	ourCard := ours.AddCard(todoID, "ours card")

	theirs := base.Clone()
	theirCard := theirs.AddCard(todoID, "theirs card")

	merged := Merge(base, ours, theirs)

	if _, ok := merged.Cards[ourCard.ID]; !ok {
		t.Fatalf("expected our card to survive the merge")
	}
	if _, ok := merged.Cards[theirCard.ID]; !ok {
		t.Fatalf("expected their card to survive the merge")
	}
	col := merged.Columns[todoID]
	if len(col.CardOrder) != 2 {
		t.Fatalf("expected 2 cards in merged column order, got %d: %v", len(col.CardOrder), col.CardOrder)
	}
}

func TestMerge_EditWinsOverDelete(t *testing.T) {
	base := newTestBoard()
	todoID := base.ColumnOrder[0]
	card := base.AddCard(todoID, "original")

	// ours deletes the card
	ours := base.Clone()
	ours.DeleteCard(card.ID)

	// theirs edits the card
	theirs := base.Clone()
	theirs.UpdateCard(card.ID, "edited title", "edited body")

	merged := Merge(base, ours, theirs)

	mc, ok := merged.Cards[card.ID]
	if !ok {
		t.Fatalf("expected the externally-edited card to survive the local delete")
	}
	if mc.Title != "edited title" {
		t.Fatalf("expected edited title to win, got %q", mc.Title)
	}
}

func TestMerge_ConflictingEditsNewerWins(t *testing.T) {
	base := newTestBoard()
	todoID := base.ColumnOrder[0]
	card := base.AddCard(todoID, "original")

	ours := base.Clone()
	ours.Cards[card.ID].Title = "ours title"
	ours.Cards[card.ID].UpdatedAt = time.Now().Add(-1 * time.Hour) // older

	theirs := base.Clone()
	theirs.Cards[card.ID].Title = "theirs title"
	theirs.Cards[card.ID].UpdatedAt = time.Now() // newer

	merged := Merge(base, ours, theirs)

	if got := merged.Cards[card.ID].Title; got != "theirs title" {
		t.Fatalf("expected newer edit (theirs) to win, got %q", got)
	}
}

func TestMerge_CardMovedToDifferentColumnsConflict(t *testing.T) {
	base := newTestBoard()
	todoID := base.ColumnOrder[0]
	inProgressID := base.ColumnOrder[1]
	doneID := base.ColumnOrder[2]
	card := base.AddCard(todoID, "task")

	ours := base.Clone()
	ours.MoveCardToColumn(card.ID, inProgressID)
	ours.Cards[card.ID].UpdatedAt = time.Now() // newer

	theirs := base.Clone()
	theirs.MoveCardToColumn(card.ID, doneID)
	theirs.Cards[card.ID].UpdatedAt = time.Now().Add(-1 * time.Hour) // older

	merged := Merge(base, ours, theirs)

	mc := merged.Cards[card.ID]
	if mc.ColumnID != inProgressID {
		t.Fatalf("expected newer move (ours, to in-progress) to win, got column %q", mc.ColumnID)
	}
	for _, colID := range []string{todoID, doneID} {
		for _, id := range merged.Columns[colID].CardOrder {
			if id == card.ID {
				t.Fatalf("card should not remain listed in column %q after losing the move conflict", colID)
			}
		}
	}
	found := false
	for _, id := range merged.Columns[inProgressID].CardOrder {
		if id == card.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected card to be listed in the winning column's CardOrder")
	}
}

func TestMerge_ColumnDeletedExternallyDropsOrphanCard(t *testing.T) {
	base := newTestBoard()
	todoID := base.ColumnOrder[0]

	// theirs deletes the whole column
	theirs := base.Clone()
	theirs.DeleteColumn(todoID)

	// ours (unaware) adds a card to that same column
	ours := base.Clone()
	ours.AddCard(todoID, "new task")

	merged := Merge(base, ours, theirs)

	if _, ok := merged.Columns[todoID]; ok {
		t.Fatalf("expected column deletion to be honored")
	}
	if len(merged.Cards) != 0 {
		t.Fatalf("expected the orphaned card to be dropped along with its column, got %d cards", len(merged.Cards))
	}
}

func TestMerge_NoConflictIsIdempotent(t *testing.T) {
	base := newTestBoard()
	same := base.Clone()

	merged := Merge(base, same, same)

	if len(merged.Cards) != len(base.Cards) || len(merged.Columns) != len(base.Columns) {
		t.Fatalf("expected merge of identical boards to be a no-op")
	}
}
