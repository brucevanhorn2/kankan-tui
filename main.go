package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/brucevanhorn/kankan-tui/internal/board"
	"github.com/brucevanhorn/kankan-tui/internal/ui"
)

func main() {
	path := "board.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	b, err := loadOrCreateBoard(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kankan-tui: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.NewModel(b, path), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "kankan-tui: %v\n", err)
		os.Exit(1)
	}
}

func loadOrCreateBoard(path string) (*board.Board, error) {
	if _, err := os.Stat(path); err == nil {
		return board.Load(path)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	b := board.New(deriveBoardName(path))
	if err := board.Save(path, b); err != nil {
		return nil, err
	}
	return b, nil
}

func deriveBoardName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return "My Board"
	}
	return name
}
