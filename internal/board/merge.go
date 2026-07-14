package board

import "time"

// Merge produces a new board combining changes from ours and theirs, using
// base - the last state both sides are known to have agreed on - to tell
// "changed" apart from "unchanged" on each side. This is how SaveMerging
// avoids silently discarding another writer's changes: instead of blindly
// overwriting the file with our in-memory state, we merge it with
// whatever's actually on disk.
//
// Conflict policy:
//   - A card/column added on only one side is kept.
//   - A card/column deleted on one side but edited on the other is kept
//     (edit wins over delete), so an edit is never silently dropped.
//   - A card/column changed differently on both sides is resolved by
//     UpdatedAt, newest wins; a tie prefers ours, since that's the write
//     actually in progress.
//   - Card/column ordering isn't diffed field-by-field: theirs' order is
//     used as the backbone (it reflects the freshest on-disk arrangement)
//     and any ours-only entries (local additions, or cards moved into a
//     column theirs doesn't know about yet) are appended after, in their
//     own relative order. A same-column manual reorder that collides with
//     an external change to that column in the same merge cycle can lose
//     to theirs' arrangement - a known, narrow limitation.
func Merge(base, ours, theirs *Board) *Board {
	merged := &Board{
		ID:            theirs.ID,
		CreatedAt:     theirs.CreatedAt,
		SchemaVersion: theirs.SchemaVersion,
		Columns:       map[string]*Column{},
		Cards:         map[string]*Card{},
	}

	if base.Name != ours.Name {
		merged.Name = ours.Name
	} else {
		merged.Name = theirs.Name
	}

	mergeCards(base, ours, theirs, merged)
	mergeColumns(base, ours, theirs, merged)
	dropOrphanCards(merged)
	rebuildCardOrders(ours, theirs, merged)
	merged.ColumnOrder = mergeOrder(ours.ColumnOrder, theirs.ColumnOrder, func(id string) bool {
		_, ok := merged.Columns[id]
		return ok
	})

	merged.UpdatedAt = time.Now()
	return merged
}

func mergeCards(base, ours, theirs *Board, merged *Board) {
	seen := map[string]bool{}
	consider := func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true

		b, hasBase := base.Cards[id]
		o, hasOurs := ours.Cards[id]
		t, hasTheirs := theirs.Cards[id]

		switch {
		case !hasBase && hasOurs && !hasTheirs:
			merged.Cards[id] = cloneCard(o)
		case !hasBase && !hasOurs && hasTheirs:
			merged.Cards[id] = cloneCard(t)
		case !hasBase && hasOurs && hasTheirs:
			merged.Cards[id] = cloneCard(newerCard(o, t))
		case hasBase && !hasOurs && !hasTheirs:
			// deleted on both sides
		case hasBase && !hasOurs && hasTheirs:
			if cardChanged(b, t) {
				merged.Cards[id] = cloneCard(t) // edit wins over delete
			}
		case hasBase && hasOurs && !hasTheirs:
			if cardChanged(b, o) {
				merged.Cards[id] = cloneCard(o) // edit wins over delete
			}
		default: // present on all three
			oc := cardChanged(b, o)
			tc := cardChanged(b, t)
			switch {
			case oc && !tc:
				merged.Cards[id] = cloneCard(o)
			case !oc && tc:
				merged.Cards[id] = cloneCard(t)
			case oc && tc:
				merged.Cards[id] = cloneCard(newerCard(o, t))
			default:
				merged.Cards[id] = cloneCard(t)
			}
		}
	}

	for id := range base.Cards {
		consider(id)
	}
	for id := range ours.Cards {
		consider(id)
	}
	for id := range theirs.Cards {
		consider(id)
	}
}

func cardChanged(a, b *Card) bool {
	return a.Title != b.Title || a.Body != b.Body || a.Color != b.Color || a.ColumnID != b.ColumnID
}

func newerCard(o, t *Card) *Card {
	if o.UpdatedAt.After(t.UpdatedAt) {
		return o
	}
	return t
}

func cloneCard(c *Card) *Card {
	cc := *c
	return &cc
}

func mergeColumns(base, ours, theirs *Board, merged *Board) {
	seen := map[string]bool{}
	consider := func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true

		b, hasBase := base.Columns[id]
		o, hasOurs := ours.Columns[id]
		t, hasTheirs := theirs.Columns[id]

		switch {
		case !hasBase && hasOurs && !hasTheirs:
			merged.Columns[id] = cloneColumnMeta(o)
		case !hasBase && !hasOurs && hasTheirs:
			merged.Columns[id] = cloneColumnMeta(t)
		case !hasBase && hasOurs && hasTheirs:
			merged.Columns[id] = cloneColumnMeta(newerColumn(o, t))
		case hasBase && !hasOurs && !hasTheirs:
			// deleted on both sides
		case hasBase && !hasOurs && hasTheirs:
			if columnChanged(b, t) {
				merged.Columns[id] = cloneColumnMeta(t)
			}
		case hasBase && hasOurs && !hasTheirs:
			if columnChanged(b, o) {
				merged.Columns[id] = cloneColumnMeta(o)
			}
		default:
			oc := columnChanged(b, o)
			tc := columnChanged(b, t)
			switch {
			case oc && !tc:
				merged.Columns[id] = cloneColumnMeta(o)
			case !oc && tc:
				merged.Columns[id] = cloneColumnMeta(t)
			case oc && tc:
				merged.Columns[id] = cloneColumnMeta(newerColumn(o, t))
			default:
				merged.Columns[id] = cloneColumnMeta(t)
			}
		}
	}

	for id := range base.Columns {
		consider(id)
	}
	for id := range ours.Columns {
		consider(id)
	}
	for id := range theirs.Columns {
		consider(id)
	}
}

func columnChanged(a, b *Column) bool {
	return a.Title != b.Title
}

func newerColumn(o, t *Column) *Column {
	if o.UpdatedAt.After(t.UpdatedAt) {
		return o
	}
	return t
}

// cloneColumnMeta copies a column's identity/title fields but not its
// CardOrder, which is rebuilt separately by rebuildCardOrders once the
// final set of surviving cards and columns is known.
func cloneColumnMeta(c *Column) *Column {
	return &Column{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, CardOrder: []string{}}
}

// dropOrphanCards removes any card whose column didn't survive the merge,
// matching DeleteColumn's cascading-delete semantics.
func dropOrphanCards(merged *Board) {
	for id, c := range merged.Cards {
		if _, ok := merged.Columns[c.ColumnID]; !ok {
			delete(merged.Cards, id)
		}
	}
}

func rebuildCardOrders(ours, theirs *Board, merged *Board) {
	byColumn := map[string][]string{}
	for id, c := range merged.Cards {
		byColumn[c.ColumnID] = append(byColumn[c.ColumnID], id)
	}
	for colID, col := range merged.Columns {
		survivors := byColumn[colID]
		survivorSet := make(map[string]bool, len(survivors))
		for _, id := range survivors {
			survivorSet[id] = true
		}
		var oursOrder, theirsOrder []string
		if oc, ok := ours.Columns[colID]; ok {
			oursOrder = oc.CardOrder
		}
		if tc, ok := theirs.Columns[colID]; ok {
			theirsOrder = tc.CardOrder
		}
		col.CardOrder = mergeOrder(oursOrder, theirsOrder, func(id string) bool {
			return survivorSet[id]
		})
	}
}

// mergeOrder builds an ordering of the IDs for which keep(id) is true,
// using theirsOrder as the backbone and appending any ours-only IDs (in
// their own relative order) that theirs doesn't know about yet.
func mergeOrder(oursOrder, theirsOrder []string, keep func(string) bool) []string {
	out := make([]string, 0, len(theirsOrder)+len(oursOrder))
	placed := map[string]bool{}
	for _, id := range theirsOrder {
		if keep(id) && !placed[id] {
			out = append(out, id)
			placed[id] = true
		}
	}
	for _, id := range oursOrder {
		if keep(id) && !placed[id] {
			out = append(out, id)
			placed[id] = true
		}
	}
	return out
}
