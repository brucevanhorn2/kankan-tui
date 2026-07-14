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

type saveResultMsg struct{ err error }
type clearStatusMsg struct{ token int }

type Model struct {
	b    *board.Board
	path string

	colIdx  int
	cardIdx int

	mode  mode
	field cardField

	input    textinput.Model
	textarea textarea.Model

	editingCardID string

	dirty     bool
	status    string
	statusTok int
	err       error

	width, height int
}

func NewModel(b *board.Board, path string) Model {
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
	}
}

func (m Model) Init() tea.Cmd {
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
	if m.cardIdx >= len(col.CardOrder) {
		m.cardIdx = len(col.CardOrder) - 1
	}
	if m.cardIdx < 0 {
		m.cardIdx = 0
	}
}

func (m *Model) selectedCard() *board.Card {
	col := m.currentColumn()
	if col == nil {
		return nil
	}
	return m.b.CardAt(col.ID, m.cardIdx)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case saveResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = fmt.Sprintf("save failed: %v", msg.err)
		} else {
			m.dirty = false
			m.status = "saved"
		}
		return m, m.scheduleStatusClear()

	case clearStatusMsg:
		if msg.token == m.statusTok {
			m.status = ""
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) scheduleStatusClear() tea.Cmd {
	m.statusTok++
	tok := m.statusTok
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return clearStatusMsg{token: tok}
	})
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
		return m, m.saveAndQuit()

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
		if m.currentColumn() == nil {
			return m, nil
		}
		m.mode = modeAddCard
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
		if m.selectedCard() == nil {
			return m, nil
		}
		m.mode = modeConfirmDeleteCard
		return m, nil

	case "c":
		c := m.selectedCard()
		if c == nil {
			return m, nil
		}
		c.Color = nextCardColor(c.Color)
		m.dirty = true
		return m, m.autosave()

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
		return m, m.autosave()

	case "J":
		col := m.currentColumn()
		c := m.selectedCard()
		if col == nil || c == nil {
			return m, nil
		}
		m.cardIdx = m.b.MoveCardWithinColumn(col.ID, m.cardIdx, 1)
		m.dirty = true
		return m, m.autosave()

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
		m.input.Placeholder = "Column name"
		m.input.SetValue(col.Title)
		m.input.Focus()
		return m, textinput.Blink

	case "x":
		if m.currentColumn() == nil {
			return m, nil
		}
		m.mode = modeConfirmDeleteColumn
		return m, nil

	case "shift+left":
		m.colIdx = m.b.MoveColumn(m.colIdx, -1)
		m.dirty = true
		return m, m.autosave()

	case "shift+right":
		m.colIdx = m.b.MoveColumn(m.colIdx, 1)
		m.dirty = true
		return m, m.autosave()

	case "s":
		return m, m.saveNow()
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
	return *m, m.autosave()
}

func (m Model) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.input.Blur()
		return m, nil

	case "enter":
		value := m.input.Value()
		switch m.mode {
		case modeAddCard:
			if value != "" {
				col := m.currentColumn()
				m.b.AddCard(col.ID, value)
				m.cardIdx = len(col.CardOrder) - 1
				m.dirty = true
			}
		case modeAddColumn:
			if value != "" {
				m.b.AddColumn(value)
				m.colIdx = len(m.b.ColumnOrder) - 1
				m.cardIdx = 0
			}
			m.dirty = true
		case modeRenameColumn:
			if value != "" {
				m.b.RenameColumn(m.currentColumn().ID, value)
				m.dirty = true
			}
		}
		m.mode = modeNormal
		m.input.Blur()
		if m.dirty {
			return m, m.autosave()
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
		m.mode = modeNormal
		m.input.Blur()
		m.textarea.Blur()
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
		m.b.UpdateCard(m.editingCardID, m.input.Value(), m.textarea.Value())
		m.dirty = true
		m.mode = modeNormal
		m.input.Blur()
		m.textarea.Blur()
		return m, m.autosave()
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
			if c := m.selectedCard(); c != nil {
				m.b.DeleteCard(c.ID)
				m.clampCardIdx()
				m.dirty = true
			}
		case modeConfirmDeleteColumn:
			if col := m.currentColumn(); col != nil {
				m.b.DeleteColumn(col.ID)
				if m.colIdx >= len(m.b.ColumnOrder) {
					m.colIdx = len(m.b.ColumnOrder) - 1
				}
				if m.colIdx < 0 {
					m.colIdx = 0
				}
				m.clampCardIdx()
				m.dirty = true
			}
		}
		m.mode = modeNormal
		return m, m.autosave()
	case "n", "esc":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

func (m Model) autosave() tea.Cmd {
	b := m.b
	path := m.path
	return func() tea.Msg {
		return saveResultMsg{err: board.Save(path, b)}
	}
}

func (m Model) saveNow() tea.Cmd {
	return m.autosave()
}

func (m Model) saveAndQuit() tea.Cmd {
	b := m.b
	path := m.path
	return tea.Sequence(
		func() tea.Msg {
			board.Save(path, b)
			return nil
		},
		tea.Quit,
	)
}

func nextCardColor(current string) string {
	for i, c := range cardColors {
		if c == current {
			return cardColors[(i+1)%len(cardColors)]
		}
	}
	return cardColors[0]
}
