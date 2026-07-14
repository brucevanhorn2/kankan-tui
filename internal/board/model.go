package board

import (
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID        string    `json:"id"`
	ColumnID  string    `json:"columnId"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Color     string    `json:"color,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Column struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CardOrder []string  `json:"cardOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type Board struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
	ColumnOrder   []string           `json:"columnOrder"`
	Columns       map[string]*Column `json:"columns"`
	Cards         map[string]*Card   `json:"cards"`
	SchemaVersion int                `json:"schemaVersion"`
}

// New creates a board with the default Todo / In Progress / Done columns.
func New(name string) *Board {
	now := time.Now()
	todo := &Column{ID: uuid.NewString(), Title: "Todo", CardOrder: []string{}, CreatedAt: now}
	inProgress := &Column{ID: uuid.NewString(), Title: "In Progress", CardOrder: []string{}, CreatedAt: now}
	done := &Column{ID: uuid.NewString(), Title: "Done", CardOrder: []string{}, CreatedAt: now}
	return &Board{
		ID:          uuid.NewString(),
		Name:        name,
		CreatedAt:   now,
		UpdatedAt:   now,
		ColumnOrder: []string{todo.ID, inProgress.ID, done.ID},
		Columns: map[string]*Column{
			todo.ID:       todo,
			inProgress.ID: inProgress,
			done.ID:       done,
		},
		Cards:         map[string]*Card{},
		SchemaVersion: 1,
	}
}

func (b *Board) ColumnAt(idx int) *Column {
	if idx < 0 || idx >= len(b.ColumnOrder) {
		return nil
	}
	return b.Columns[b.ColumnOrder[idx]]
}

func (b *Board) CardAt(columnID string, idx int) *Card {
	col, ok := b.Columns[columnID]
	if !ok || idx < 0 || idx >= len(col.CardOrder) {
		return nil
	}
	return b.Cards[col.CardOrder[idx]]
}

func (b *Board) AddCard(columnID, title string) *Card {
	col, ok := b.Columns[columnID]
	if !ok {
		return nil
	}
	now := time.Now()
	c := &Card{
		ID:        uuid.NewString(),
		ColumnID:  columnID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	b.Cards[c.ID] = c
	col.CardOrder = append(col.CardOrder, c.ID)
	return c
}

func (b *Board) UpdateCard(cardID, title, body string) {
	c, ok := b.Cards[cardID]
	if !ok {
		return
	}
	c.Title = title
	c.Body = body
	c.UpdatedAt = time.Now()
}

func (b *Board) DeleteCard(cardID string) {
	c, ok := b.Cards[cardID]
	if !ok {
		return
	}
	if col, ok := b.Columns[c.ColumnID]; ok {
		col.CardOrder = removeStr(col.CardOrder, cardID)
	}
	delete(b.Cards, cardID)
}

// MoveCardWithinColumn swaps a card's position with an adjacent index (delta -1 or +1).
func (b *Board) MoveCardWithinColumn(columnID string, idx, delta int) int {
	col, ok := b.Columns[columnID]
	if !ok {
		return idx
	}
	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(col.CardOrder) {
		return idx
	}
	col.CardOrder[idx], col.CardOrder[newIdx] = col.CardOrder[newIdx], col.CardOrder[idx]
	return newIdx
}

// MoveCardToColumn moves a card to a different column, appending it to the end.
func (b *Board) MoveCardToColumn(cardID, targetColumnID string) {
	c, ok := b.Cards[cardID]
	if !ok {
		return
	}
	target, ok := b.Columns[targetColumnID]
	if !ok {
		return
	}
	if src, ok := b.Columns[c.ColumnID]; ok {
		src.CardOrder = removeStr(src.CardOrder, cardID)
	}
	target.CardOrder = append(target.CardOrder, cardID)
	c.ColumnID = targetColumnID
	c.UpdatedAt = time.Now()
}

func (b *Board) AddColumn(title string) *Column {
	col := &Column{ID: uuid.NewString(), Title: title, CardOrder: []string{}, CreatedAt: time.Now()}
	b.Columns[col.ID] = col
	b.ColumnOrder = append(b.ColumnOrder, col.ID)
	return col
}

func (b *Board) RenameColumn(columnID, title string) {
	if col, ok := b.Columns[columnID]; ok {
		col.Title = title
	}
}

func (b *Board) DeleteColumn(columnID string) {
	col, ok := b.Columns[columnID]
	if !ok {
		return
	}
	for _, cardID := range col.CardOrder {
		delete(b.Cards, cardID)
	}
	delete(b.Columns, columnID)
	b.ColumnOrder = removeStr(b.ColumnOrder, columnID)
}

// MoveColumn swaps a column's position with an adjacent index (delta -1 or +1).
func (b *Board) MoveColumn(idx, delta int) int {
	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(b.ColumnOrder) {
		return idx
	}
	b.ColumnOrder[idx], b.ColumnOrder[newIdx] = b.ColumnOrder[newIdx], b.ColumnOrder[idx]
	return newIdx
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
