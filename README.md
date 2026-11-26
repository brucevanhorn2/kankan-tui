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
- `h`/`←`, `l`/`→` — switch column (scrolls horizontally if there are more columns than fit)
- `j`/`↓`, `k`/`↑` — switch card (scrolls vertically within a column if it has more cards than fit; a "3-8 of 12" indicator shows under the column header when scrolled)

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

## Live external edits

kankan-tui watches its board file and picks up changes made by other processes — scripts, agents, another tool editing the JSON directly — without needing a restart. This is the main way something other than the TUI itself is expected to move cards around (e.g. an agent moving a card to "Done" when it finishes a task).

Conflict handling:
- If you're just looking at the board (not typing into a modal), an external change is applied immediately.
- If you're mid-edit (adding a card, renaming a column, editing a card body), the external change is held until you finish, then applied — followed by your edit, so neither side clobbers the other.
- If a card or column you're editing was deleted externally in the meantime, your edit becomes a safe no-op rather than resurrecting it.

External writers should save atomically (write to a temp file in the same directory, then rename over the target) to avoid a race where kankan-tui reads a half-written file — it already does this for its own saves.

### Saving without clobbering other writers

Every save (autosave, `s`, or quit) checks whether the file has changed on disk since kankan-tui last synced with it. If it hasn't, the save is a plain overwrite, same as before. If it has — another process wrote to the file in the meantime, e.g. an agent updating a different card — kankan-tui three-way merges its own state with what's on disk before writing, instead of blindly overwriting the other writer's change. This is what makes it safe for several independent agents (and a human in the TUI) to all be updating the same board file concurrently, which is the main intended use.

Merge policy:
- A card or column added by only one side is kept.
- A card or column deleted on one side but edited on the other keeps the edit, so an edit is never silently dropped by a concurrent delete.
- The same card or column changed differently by both sides is resolved by which change is newer; ties favor the side doing the save.
- Card/column ordering isn't diffed positionally — the freshest on-disk order is used as the backbone, and any new local entries are appended after. A manual reorder within a column that collides with an external change to that same column in the same save cycle can lose to the on-disk arrangement; this is a known, narrow limitation. Moving cards between columns (the primary way agents are expected to update status) isn't affected by this.

Saves from this process itself are serialized against each other (so two rapid local saves can't race), but there's no cross-process locking — the merge is what keeps concurrent writers from stepping on each other, not mutual exclusion.

`q` / `ctrl+c` waits for any save still in flight to finish before exiting, rather than quitting out from under it. If that final save fails, kankan-tui prints a warning to stderr and exits with a non-zero status instead of quitting silently.

## Architecture

```
main.go                    entrypoint: load/create board, start the watcher, run the Bubble Tea program
internal/board/
  model.go                 Board/Column/Card types + mutation methods
  store.go                 JSON load/save (atomic write, content hashing)
  watch.go                 fsnotify-based watcher for external changes
internal/ui/
  app.go                   Model, Update loop, keybindings, modals, external-change reconciliation
  view.go                  rendering (columns, cards, modals, help, per-column scrolling)
  styles.go                lipgloss styles and card color palette
```
