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
		// Render the card by the ID captured at 'd'-press time, the same
		// ID handleConfirmKey deletes on 'y' - not the cursor-selected
		// card, which can drift out from under the dialog if an external
		// change is applied while it's open.
		title := ""
		if c, ok := m.b.Cards[m.deleteCardID]; ok {
			title = c.Title
		}
		return m.renderModal("Delete card?", dangerStyle.Render(fmt.Sprintf("%q will be deleted.", title)), "y: delete   n/esc: cancel")
	case modeConfirmDeleteColumn:
		title := ""
		if col, ok := m.b.Columns[m.deleteColumnID]; ok {
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

	slots := (m.height - chromeLines) / cardBoxHeight
	if slots < 1 {
		slots = 1
	}

	cols := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		cols = append(cols, m.renderColumn(m.b.ColumnAt(i), i, slots))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m Model) renderColumn(col *board.Column, idx, slots int) string {
	focused := idx == m.colIdx
	headerStyle := columnHeaderBlurredStyle
	boxStyle := columnBoxStyle
	if focused {
		headerStyle = columnHeaderFocusedStyle
		boxStyle = columnBoxFocusedStyle
	}

	header := headerStyle.Render(fmt.Sprintf("%s (%d)", col.Title, len(col.CardOrder)))

	total := len(col.CardOrder)
	start := 0
	if focused {
		if m.cardIdx >= start+slots {
			start = m.cardIdx - slots + 1
		}
		if m.cardIdx < start {
			start = m.cardIdx
		}
	}
	if start+slots > total {
		start = total - slots
	}
	if start < 0 {
		start = 0
	}
	end := start + slots
	if end > total {
		end = total
	}

	posLine := ""
	if total > slots {
		posLine = fmt.Sprintf("%d-%d of %d", start+1, end, total)
	}

	var body string
	if total == 0 {
		body = padLines(emptyColumnStyle.Render("(empty)"), slots*cardBoxHeight)
	} else {
		cards := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			c := m.b.Cards[col.CardOrder[i]]
			cards = append(cards, m.renderCard(c, focused && i == m.cardIdx))
		}
		body = padLines(lipgloss.JoinVertical(lipgloss.Left, cards...), slots*cardBoxHeight)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, scrollIndicatorStyle.Render(posLine), body)
	return boxStyle.Render(content)
}

// padLines pads s with blank lines until it has exactly targetLines lines,
// so every column box renders at the same height regardless of how many
// cards it holds.
func padLines(s string, targetLines int) string {
	got := strings.Count(s, "\n") + 1
	if got >= targetLines {
		return s
	}
	return s + strings.Repeat("\n", targetLines-got)
}

func (m Model) renderCard(c *board.Card, focused bool) string {
	style := cardStyle
	if focused {
		style = cardFocusedStyle
	}
	style = cardColorStyle(c.Color, style)

	title := truncateOneLine(c.Title, cardTextWidth)
	preview := truncateOneLine(c.Body, cardTextWidth)

	content := title + "\n"
	if preview != "" {
		content += cardBodyPreviewStyle.Render(preview)
	}
	return style.Render(content)
}

func truncateOneLine(s string, maxLen int) string {
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
