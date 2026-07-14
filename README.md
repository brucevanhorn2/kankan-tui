# kankan-tui

A terminal kanban board. Same idea as [kankan](../kankan), rebuilt for the terminal with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Usage

```bash
go run . [path/to/board.json]
```

If the file doesn't exist, a new board is created there with default `Todo` / `In Progress` / `Done` columns. Defaults to `board.json` in the current directory if no path is given.

Build a binary:

```bash
go build -o kankan-tui .
./kankan-tui myboard.json
```

## Keybindings

**Navigation**
- `h`/`←`, `l`/`→` — switch column
- `j`/`↓`, `k`/`↑` — switch card

**Cards**
- `a` — add card
- `enter` / `e` — edit card (title + body)
- `d` — delete card
- `c` — cycle card color
- `H` / `L` — move card to previous/next column
- `J` / `K` — move card down/up within column

**Columns**
- `n` — add column
- `r` — rename column
- `x` — delete column (and its cards)
- `shift+←`/`shift+→` — reorder column

**Other**
- `s` — save now
- `?` — toggle help
- `q` / `ctrl+c` — save and quit

## Data format

Boards are plain JSON, autosaved after every change (temp-file-then-rename, so a crash mid-write can't corrupt the file). The schema is its own format — not the same file layout as the Electron `kankan` app — so boards aren't interchangeable between the two yet.

## Architecture

```
main.go                    entrypoint: load/create board, run the Bubble Tea program
internal/board/
  model.go                 Board/Column/Card types + mutation methods
  store.go                 JSON load/save (atomic write)
internal/ui/
  app.go                   Model, Update loop, keybindings, modals
  view.go                  rendering (columns, cards, modals, help)
  styles.go                lipgloss styles and card color palette
```
