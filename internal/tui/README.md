# tui

## Overview
Package `tui` contains the interactive Bubble Tea terminal user interface and Lip Gloss styling for `gh-terrorizer`. It implements responsive Kanban board rendering, horizontal sliding viewport navigation, Vim motions, optimistic card moves, external `$EDITOR` suspension, and Tokyo Night aesthetic palettes.

## Interface
- `GitHubClientInterface`: Abstracts GitHub board queries, status moves, and hour mutations for testability.
- `TimeTrackerInterface`: Abstracts Timewarrior CLI actions (`Start`, `Stop`, `IsActive`, `GetActiveTag`).
- `Model`: The root Bubble Tea application state model implementing `tea.Model`.
- `NewModel(client GitHubClientInterface, tracker TimeTrackerInterface, cfg *config.Config) Model`: Initializes the TUI model.
- `(Model) Init() tea.Cmd`: Dispatches initial board fetch and tick commands.
- `(Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)`: Handles input events, Vim navigation keys, card moves, and timer updates.
- `(Model) View() string`: Renders the adaptive Kanban board with zero vertical line overflow.
- `(Model) VisibleColumnCount() int`: Computes the number of side-by-side columns that fit within current terminal width.
- `(Model) ItemsInColumn(optionID string) []github.ProjectItem`: Filters cards belonging to a specific status option ID.
- `(Model) SelectedItem() *github.ProjectItem`: Returns the currently focused card.
- `MinColumnWidth`: Constant (`25`) defining minimum column character width.
- `TokyoNightPalette`: Struct defining color hex codes for the Tokyo Night color system.
- `GetPalette(variant string) TokyoNightPalette`: Returns palette colors for `"storm"` or `"night"`.
- `Styles`: Collection of Lip Gloss styles used across all UI components.
- `NewStyles(variant string, icons bool) Styles`: Initializes styles and Nerd Font icon preferences.
- `(*Styles) Icon(iconName string) string`: Resolves Nerd Font glyphs with ASCII fallbacks.

## Invariants
- Total rendered view height never exceeds `m.height`, anchoring header and status bars cleanly against terminal bounds.
- Horizontal column scrolling ensures `activeColumn` is always within the visible column range $[colOffset, colOffset + VisibleColumnCount() - 1]$.
- Optimistic card movement rolls back to `SourceOptionID` if the upstream `CardMovedMsg` returns an error.
- Toggling time tracking on a new item stops any previously tracked session and schedules additive hour synchronization.

## Side Effects
- Creates and removes temporary markdown files in `os.TempDir()` (`gh-terrorizer-<item_id>.md`).
- Suspends the Bubble Tea event loop to spawn external text editor processes (`$EDITOR`, `$VISUAL`, or `nano`) via `tea.ExecProcess`.
- Dispatches asynchronous network operations to GitHub and subprocess calls to `timew`.

## Verification
- Run: `go test ./internal/tui`
