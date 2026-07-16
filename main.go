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

	b, hash, err := loadOrCreateBoard(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kankan-tui: %v\n", err)
		os.Exit(1)
	}

	w, err := board.NewWatcher(path)
	if err != nil {
		// Not fatal: kankan-tui still works without live-reload, it just
		// won't pick up changes made by other processes until restarted.
		fmt.Fprintf(os.Stderr, "kankan-tui: warning: file watch disabled: %v\n", err)
	} else {
		defer w.Close()
	}

	p := tea.NewProgram(ui.NewModel(b, path, hash, w), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kankan-tui: %v\n", err)
		os.Exit(1)
	}
	if fm, ok := finalModel.(ui.Model); ok {
		if saveErr := fm.Err(); saveErr != nil {
			fmt.Fprintf(os.Stderr, "kankan-tui: warning: last save failed: %v\n", saveErr)
			os.Exit(1)
		}
	}
}

func loadOrCreateBoard(path string) (*board.Board, string, error) {
	if _, err := os.Stat(path); err == nil {
		return board.Load(path)
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}

	b := board.New(deriveBoardName(path))
	hash, err := board.Save(path, b)
	if err != nil {
		return nil, "", err
	}
	return b, hash, nil
}

func deriveBoardName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return "My Board"
	}
	return name
}
