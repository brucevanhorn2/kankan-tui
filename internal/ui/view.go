package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/brucevanhorn/kankan-tui/internal/board"
)

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	switch m.mode {
	case modeAddCard:
		return m.renderModal("Add card", m.input.View(), "enter: add   esc: cancel")
	case modeAddColumn:
		return m.renderModal("Add column", m.input.View(), "enter: add   esc: cancel")
	case modeRenameColumn:
		return m.renderModal("Rename column", m.input.View(), "enter: save   esc: cancel")
	case modeEditCard:
		return m.renderEditCardModal()
	case modeConfirmDeleteCard:
		c := m.selectedCard()
		title := ""
		if c != nil {
			title = c.Title
		}
		return m.renderModal("Delete card?", dangerStyle.Render(fmt.Sprintf("%q will be deleted.", title)), "y: delete   n/esc: cancel")
	case modeConfirmDeleteColumn:
		col := m.currentColumn()
		title := ""
		if col != nil {
			title = col.Title
		}
		return m.renderModal("Delete column?", dangerStyle.Render(fmt.Sprintf("%q and all its cards will be deleted.", title)), "y: delete   n/esc: cancel")
	case modeHelp:
		return m.renderHelp()
	}

	header := boardTitleStyle.Render(m.boardTitleText())
	body := m.renderColumns()
	footer := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) boardTitleText() string {
	dirty := ""
	if m.dirty {
		dirty = " •"
	}
	return fmt.Sprintf("%s%s   %s", m.b.Name, dirty, m.path)
}

func (m Model) renderColumns() string {
	n := len(m.b.ColumnOrder)
	if n == 0 {
		return emptyColumnStyle.Render("No columns. Press 'n' to add one.")
	}

	unit := colWidth + colGap
	visible := m.width / unit
	if visible < 1 {
		visible = 1
	}
	start := 0
	if m.colIdx >= visible {
		start = m.colIdx - visible + 1
	}
	if start+visible > n {
		start = n - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > n {
		end = n
	}

	cols := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		cols = append(cols, m.renderColumn(m.b.ColumnAt(i), i))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) renderColumn(col *board.Column, idx int) string {
	focused := idx == m.colIdx
	headerStyle := columnHeaderBlurredStyle
	boxStyle := columnBoxStyle
	if focused {
		headerStyle = columnHeaderFocusedStyle
		boxStyle = columnBoxFocusedStyle
	}

	header := headerStyle.Render(fmt.Sprintf("%s (%d)", col.Title, len(col.CardOrder)))

	var cardsView string
	if len(col.CardOrder) == 0 {
		cardsView = emptyColumnStyle.Render("(empty)")
	} else {
		cards := make([]string, 0, len(col.CardOrder))
		for i, cardID := range col.CardOrder {
			c := m.b.Cards[cardID]
			cards = append(cards, m.renderCard(c, focused && i == m.cardIdx))
		}
		cardsView = lipgloss.JoinVertical(lipgloss.Left, cards...)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, cardsView)
	return boxStyle.Render(content)
}

func (m Model) renderCard(c *board.Card, focused bool) string {
	style := cardStyle
	if focused {
		style = cardFocusedStyle
	}
	style = cardColorStyle(c.Color, style)

	title := c.Title
	preview := firstLine(c.Body, 34)

	var content string
	if preview != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, title, cardBodyPreviewStyle.Render(preview))
	} else {
		content = title
	}
	return style.Render(content)
}

func firstLine(s string, maxLen int) string {
	line := s
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		line = s[:idx]
	}
	r := []rune(line)
	if len(r) > maxLen {
		return string(r[:maxLen-1]) + "…"
	}
	return line
}

func (m Model) renderStatusBar() string {
	status := m.status
	if status == "" {
		status = "ready"
	}
	left := statusBarStyle.Render(status)
	right := helpStyle.Render("?: help   q: quit")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderModal(title, content, help string) string {
	body := lipgloss.JoinVertical(lipgloss.Left,
		modalTitleStyle.Render(title),
		content,
		modalHelpStyle.Render(help),
	)
	box := modalStyle.Width(50).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderEditCardModal() string {
	titleLabel := "Title"
	bodyLabel := "Body"
	if m.field == fieldTitle {
		titleLabel = "Title ▸"
	} else {
		bodyLabel = "Body ▸"
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		helpStyle.Render(titleLabel),
		m.input.View(),
		"",
		helpStyle.Render(bodyLabel),
		m.textarea.View(),
	)
	return m.renderModal("Edit card", content, "tab: switch field   ctrl+s: save   esc: cancel")
}

func (m Model) renderHelp() string {
	lines := []string{
		"Navigation",
		"  h/←  l/→     switch column",
		"  j/↓  k/↑     switch card",
		"",
		"Cards",
		"  a            add card",
		"  enter / e    edit card",
		"  d            delete card",
		"  c            cycle card color",
		"  H / L        move card to prev/next column",
		"  J / K        move card down/up in column",
		"",
		"Columns",
		"  n            add column",
		"  r            rename column",
		"  x            delete column",
		"  shift+←/→    reorder column",
		"",
		"Other",
		"  s            save now",
		"  ?            toggle this help",
		"  q            save and quit",
	}
	return m.renderModal("kankan-tui help", strings.Join(lines, "\n"), "press any key to close")
}
