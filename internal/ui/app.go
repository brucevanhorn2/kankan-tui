package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/brucevanhorn/kankan-tui/internal/board"
)

type mode int

const (
	modeNormal mode = iota
	modeAddCard
	modeEditCard
	modeAddColumn
	modeRenameColumn
	modeConfirmDeleteCard
	modeConfirmDeleteColumn
	modeHelp
)

type cardField int

const (
	fieldTitle cardField = iota
	fieldBody
)

type saveResultMsg struct {
	gen    int
	board  *board.Board
	merged bool
	hash   string
	err    error
}
type clearStatusMsg struct{ token int }
type externalChangeMsg struct{}
type watchErrMsg struct{ err error }
type reloadedMsg struct {
	b    *board.Board
	hash string
	err  error
}

type Model struct {
	b    *board.Board
	path string

	colIdx  int
	cardIdx int

	mode  mode
	field cardField

	input    textinput.Model
	textarea textarea.Model

	editingCardID  string
	targetColumnID string
	deleteCardID   string
	deleteColumnID string

	watcher      *board.Watcher
	lastHash     string
	pendingBoard *board.Board

	// base is a frozen snapshot of the board as of the last point this
	// process's in-memory state and the on-disk file were known to agree
	// (last load, last successful save, last external reload). It's the
	// common ancestor used to 3-way merge on save so a save never blindly
	// clobbers another writer's change; see board.SaveMerging.
	base *board.Board
	// saveGen counts dispatched saves so a stale saveResultMsg (from a
	// save superseded by a later one before it returned) can be told
	// apart from the latest and ignored instead of regressing the model.
	saveGen int
	// saveInFlight is true while a save goroutine is running. Only one
	// save is ever dispatched at a time: dispatching a second save while
	// the first is still in flight would capture the same stale
	// base/lastHash the first save is about to make obsolete, causing the
	// second save to needlessly (and lossily) merge against its own
	// process's prior write. See dispatchSave.
	saveInFlight bool
	// pendingSave is set when an edit asks for a save while one is
	// already in flight. The in-flight save's completion handler checks
	// this and immediately dispatches the coalesced follow-up against the
	// now-fresh base/lastHash, instead of the edit racing a second save
	// against the first.
	pendingSave bool
	// quitting is set once the user has asked to quit. The program only
	// actually quits once any in-flight/pending save has finished, so a
	// save on the way out is never abandoned mid-flight.
	quitting bool

	dirty     bool
	status    string
	statusTok int
	err       error

	width, height int
}

func NewModel(b *board.Board, path, initialHash string, w *board.Watcher) Model {
	ti := textinput.New()
	ti.CharLimit = 200

	ta := textarea.New()
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false

	return Model{
		b:        b,
		path:     path,
		mode:     modeNormal,
		input:    ti,
		textarea: ta,
		watcher:  w,
		lastHash: initialHash,
		base:     b.Clone(),
	}
}

func (m Model) Init() tea.Cmd {
	if m.watcher != nil {
		return waitForWatch(m.watcher)
	}
	return nil
}

func (m *Model) currentColumn() *board.Column {
	return m.b.ColumnAt(m.colIdx)
}

func (m *Model) clampCardIdx() {
	col := m.currentColumn()
	if col == nil {
		m.cardIdx = 0
		return
	}
	m.cardIdx = clamp(m.cardIdx, 0, len(col.CardOrder)-1)
}

func (m *Model) selectedCard() *board.Card {
	col := m.currentColumn()
	if col == nil {
		return nil
	}
	return m.b.CardAt(col.ID, m.cardIdx)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// isTextEntryMode reports whether the user currently has typed, unsaved
// input in a modal (as opposed to a confirm prompt, which has nothing to
// lose). Only these modes defer an incoming external board change, so a
// local edit-in-progress is never silently discarded.
func isTextEntryMode(mo mode) bool {
	switch mo {
	case modeAddCard, modeAddColumn, modeRenameColumn, modeEditCard:
		return true
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case saveResultMsg:
		if msg.gen != m.saveGen {
			// Superseded by a later save dispatched before this one
			// returned; that save's snapshot already included whatever
			// this one was trying to persist, so just drop it rather
			// than regress the model with stale state. Shouldn't happen
			// now that saves are single-flight (see dispatchSave), kept
			// as a defensive invariant check.
			return m, nil
		}
		m.saveInFlight = false
		if msg.err != nil {
			m.pendingSave = false
			m.err = msg.err
			m.status = fmt.Sprintf("save failed: %v", msg.err)
			if m.quitting {
				return m, tea.Quit
			}
			m.statusTok++
			return m, clearStatusCmd(m.statusTok)
		}

		m.dirty = false
		m.err = nil
		m.lastHash = msg.hash
		m.base = msg.board.Clone()
		if msg.merged {
			m.b.ReplaceFrom(msg.board)
			m.colIdx = clamp(m.colIdx, 0, len(m.b.ColumnOrder)-1)
			m.clampCardIdx()
			m.status = "merged external changes and saved"
		} else {
			m.status = "saved"
		}

		if m.pendingSave {
			// An edit arrived while this save was in flight; the
			// coalesced follow-up picks it up now against the base/
			// lastHash we just refreshed, instead of racing this save
			// with a second one against stale ancestor state.
			m.pendingSave = false
			cmd := m.dispatchSave()
			m.statusTok++
			return m, tea.Batch(cmd, clearStatusCmd(m.statusTok))
		}
		if m.quitting {
			return m, tea.Quit
		}
		m.statusTok++
		return m, clearStatusCmd(m.statusTok)

	case clearStatusMsg:
		if msg.token == m.statusTok {
			m.status = ""
		}
		return m, nil

	case externalChangeMsg:
		return m, tea.Batch(m.reloadCmd(), waitForWatch(m.watcher))

	case watchErrMsg:
		m.status = fmt.Sprintf("watch error: %v", msg.err)
		m.statusTok++
		return m, tea.Batch(clearStatusCmd(m.statusTok), waitForWatch(m.watcher))

	case reloadedMsg:
		nm := m.handleReloaded(msg)
		nm.statusTok++
		return nm, clearStatusCmd(nm.statusTok)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleReloaded(msg reloadedMsg) Model {
	if msg.err != nil {
		// Likely a transient read (e.g. mid-write by a non-atomic writer).
		// The next fs event will give us another chance.
		return m
	}
	if msg.hash == m.lastHash || board.IsSelfWrite(msg.hash) {
		// Our own save (or an external write with identical content).
		// IsSelfWrite covers the case where the fs watcher's notification
		// of our own atomic rename reaches here before this process's own
		// saveResultMsg has updated m.lastHash.
		return m
	}
	m.lastHash = msg.hash
	if isTextEntryMode(m.mode) {
		m.pendingBoard = msg.b
		m.status = "board changed externally — applying once you finish"
		return m
	}
	m.applyExternalBoard(msg.b)
	return m
}

func (m *Model) applyExternalBoard(nb *board.Board) {
	m.b.ReplaceFrom(nb)
	m.base = nb.Clone()
	m.colIdx = clamp(m.colIdx, 0, len(m.b.ColumnOrder)-1)
	m.clampCardIdx()
	m.dirty = false
	m.status = "board updated externally"
}

// returnToNormal leaves whatever modal is open and, if an external board
// change arrived while the user was typing, applies it now that there's no
// unsaved input left to clobber.
func (m *Model) returnToNormal() {
	m.mode = modeNormal
	if m.pendingBoard != nil {
		nb := m.pendingBoard
		m.pendingBoard = nil
		m.applyExternalBoard(nb)
	}
}

func clearStatusCmd(tok int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return clearStatusMsg{token: tok}
	})
}

func waitForWatch(w *board.Watcher) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-w.Events:
			return externalChangeMsg{}
		case err := <-w.Errors:
			return watchErrMsg{err: err}
		}
	}
}

func (m Model) reloadCmd() tea.Cmd {
	path := m.path
	return func() tea.Msg {
		b, hash, err := board.Load(path)
		return reloadedMsg{b: b, hash: hash, err: err}
	}
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeNormal:
		return m.handleNormalKey(msg)
	case modeAddCard, modeAddColumn, modeRenameColumn:
		return m.handleTextInputKey(msg)
	case modeEditCard:
		return m.handleEditCardKey(msg)
	case modeConfirmDeleteCard, modeConfirmDeleteColumn:
		return m.handleConfirmKey(msg)
	case modeHelp:
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		if !m.dirty && !m.saveInFlight {
			return m, tea.Quit
		}
		return m, m.dispatchSave()

	case "?":
		m.mode = modeHelp
		return m, nil

	case "left", "h":
		if m.colIdx > 0 {
			m.colIdx--
			m.clampCardIdx()
		}
		return m, nil

	case "right", "l":
		if m.colIdx < len(m.b.ColumnOrder)-1 {
			m.colIdx++
			m.clampCardIdx()
		}
		return m, nil

	case "up", "k":
		if m.cardIdx > 0 {
			m.cardIdx--
		}
		return m, nil

	case "down", "j":
		col := m.currentColumn()
		if col != nil && m.cardIdx < len(col.CardOrder)-1 {
			m.cardIdx++
		}
		return m, nil

	case "a":
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		m.mode = modeAddCard
		m.targetColumnID = col.ID
		m.input.Placeholder = "Card title"
		m.input.SetValue("")
		m.input.Focus()
		return m, textinput.Blink

	case "enter", "e":
		c := m.selectedCard()
		if c == nil {
			return m, nil
		}
		m.mode = modeEditCard
		m.field = fieldTitle
		m.editingCardID = c.ID
		m.input.SetValue(c.Title)
		m.input.Focus()
		m.textarea.SetValue(c.Body)
		m.textarea.Blur()
		return m, textinput.Blink

	case "d":
		c := m.selectedCard()
		if c == nil {
			return m, nil
		}
		m.mode = modeConfirmDeleteCard
		m.deleteCardID = c.ID
		return m, nil

	case "c":
		c := m.selectedCard()
		if c == nil {
			return m, nil
		}
		c.Color = nextCardColor(c.Color)
		c.UpdatedAt = time.Now()
		m.dirty = true
		return m, m.dispatchSave()

	case "H":
		return m.moveCardColumn(-1)

	case "L":
		return m.moveCardColumn(1)

	case "K":
		col := m.currentColumn()
		c := m.selectedCard()
		if col == nil || c == nil {
			return m, nil
		}
		m.cardIdx = m.b.MoveCardWithinColumn(col.ID, m.cardIdx, -1)
		m.dirty = true
		return m, m.dispatchSave()

	case "J":
		col := m.currentColumn()
		c := m.selectedCard()
		if col == nil || c == nil {
			return m, nil
		}
		m.cardIdx = m.b.MoveCardWithinColumn(col.ID, m.cardIdx, 1)
		m.dirty = true
		return m, m.dispatchSave()

	case "n":
		m.mode = modeAddColumn
		m.input.Placeholder = "Column name"
		m.input.SetValue("")
		m.input.Focus()
		return m, textinput.Blink

	case "r":
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		m.mode = modeRenameColumn
		m.targetColumnID = col.ID
		m.input.Placeholder = "Column name"
		m.input.SetValue(col.Title)
		m.input.Focus()
		return m, textinput.Blink

	case "x":
		col := m.currentColumn()
		if col == nil {
			return m, nil
		}
		m.mode = modeConfirmDeleteColumn
		m.deleteColumnID = col.ID
		return m, nil

	case "shift+left":
		m.colIdx = m.b.MoveColumn(m.colIdx, -1)
		m.dirty = true
		return m, m.dispatchSave()

	case "shift+right":
		m.colIdx = m.b.MoveColumn(m.colIdx, 1)
		m.dirty = true
		return m, m.dispatchSave()

	case "s":
		return m, m.dispatchSave()
	}
	return m, nil
}

func (m *Model) moveCardColumn(delta int) (tea.Model, tea.Cmd) {
	col := m.currentColumn()
	c := m.selectedCard()
	if col == nil || c == nil {
		return *m, nil
	}
	targetIdx := m.colIdx + delta
	if targetIdx < 0 || targetIdx >= len(m.b.ColumnOrder) {
		return *m, nil
	}
	target := m.b.ColumnAt(targetIdx)
	m.b.MoveCardToColumn(c.ID, target.ID)
	m.colIdx = targetIdx
	m.cardIdx = len(target.CardOrder) - 1
	m.dirty = true
	return *m, m.dispatchSave()
}

func (m Model) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input.Blur()
		m.returnToNormal()
		return m, nil

	case "enter":
		value := m.input.Value()
		mo := m.mode
		targetColumnID := m.targetColumnID
		m.input.Blur()
		m.returnToNormal()

		switch mo {
		case modeAddCard:
			if value != "" {
				if col, ok := m.b.Columns[targetColumnID]; ok {
					m.b.AddCard(col.ID, value)
					for i, id := range m.b.ColumnOrder {
						if id == col.ID {
							m.colIdx = i
						}
					}
					m.cardIdx = len(col.CardOrder) - 1
					m.dirty = true
				}
			}
		case modeAddColumn:
			if value != "" {
				m.b.AddColumn(value)
				m.colIdx = len(m.b.ColumnOrder) - 1
				m.cardIdx = 0
				m.dirty = true
			}
		case modeRenameColumn:
			if value != "" {
				if col, ok := m.b.Columns[targetColumnID]; ok {
					m.b.RenameColumn(col.ID, value)
					m.dirty = true
				}
			}
		}
		if m.dirty {
			return m, m.dispatchSave()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleEditCardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input.Blur()
		m.textarea.Blur()
		m.returnToNormal()
		return m, nil

	case "tab":
		if m.field == fieldTitle {
			m.field = fieldBody
			m.input.Blur()
			m.textarea.Focus()
		} else {
			m.field = fieldTitle
			m.textarea.Blur()
			m.input.Focus()
		}
		return m, nil

	case "ctrl+s":
		cardID := m.editingCardID
		title := m.input.Value()
		body := m.textarea.Value()
		m.input.Blur()
		m.textarea.Blur()
		m.returnToNormal()
		m.b.UpdateCard(cardID, title, body)
		m.dirty = true
		return m, m.dispatchSave()
	}

	var cmd tea.Cmd
	if m.field == fieldTitle {
		m.input, cmd = m.input.Update(msg)
	} else {
		m.textarea, cmd = m.textarea.Update(msg)
	}
	return m, cmd
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		switch m.mode {
		case modeConfirmDeleteCard:
			m.b.DeleteCard(m.deleteCardID)
			m.clampCardIdx()
			m.dirty = true
		case modeConfirmDeleteColumn:
			m.b.DeleteColumn(m.deleteColumnID)
			m.colIdx = clamp(m.colIdx, 0, len(m.b.ColumnOrder)-1)
			m.clampCardIdx()
			m.dirty = true
		}
		m.mode = modeNormal
		return m, m.dispatchSave()
	case "n", "esc":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

// dispatchSave is the single entry point for triggering a save. Only one
// save goroutine ever runs at a time: if one is already in flight, this
// just records that another save is wanted (m.pendingSave) instead of
// dispatching a second one, which would capture the same base/lastHash the
// in-flight save is about to make stale and needlessly (and lossily) merge
// against this process's own prior write. See the saveResultMsg handler in
// Update, which dispatches the coalesced follow-up once the in-flight save
// completes.
func (m *Model) dispatchSave() tea.Cmd {
	if m.saveInFlight {
		m.pendingSave = true
		return nil
	}
	m.saveInFlight = true
	m.saveGen++
	return saveCmd(m.path, m.base, m.b.Clone(), m.lastHash, m.saveGen)
}

func saveCmd(path string, base, ours *board.Board, lastHash string, gen int) tea.Cmd {
	return func() tea.Msg {
		toWrite, merged, hash, err := board.SaveMerging(path, base, ours, lastHash)
		return saveResultMsg{gen: gen, board: toWrite, merged: merged, hash: hash, err: err}
	}
}

// Err returns the most recent save error, if any. Checked by main after
// the program exits (and the terminal is restored) so a save that failed
// on the way out isn't silently swallowed.
func (m Model) Err() error {
	return m.err
}

func nextCardColor(current string) string {
	for i, c := range cardColors {
		if c == current {
			return cardColors[(i+1)%len(cardColors)]
		}
	}
	return cardColors[0]
}
