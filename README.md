# gh-terrorizer Blueprint

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

`gh-terrorizer` is a keyboard-driven terminal user interface (TUI) workstation for managing **GitHub Projects v2** Kanban boards and synchronizing task execution time directly into project custom float fields via **Timewarrior** (`timew`).

Built with Go, [Charm's Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss), it executes GitHub GraphQL queries and mutations in-process via [`github.com/cli/go-gh/v2`](https://github.com/cli/go-gh), transparently reusing existing GitHub CLI credentials, hosts, and keychain tokens.

---

## System Overview

`gh-terrorizer` bridges terminal-native developer workflows with cloud-hosted GitHub Projects v2 boards. The application runs in the terminal alternate screen buffer, providing responsive multi-column Kanban card visualization, Vim motions, optimistic card moves, external editor suspension for markdown bodies, and seamless local time tracking synchronized upstream.

### Architecture Overview

```text
gh-terrorizer/
├── main.go                     # Entry point: CLI flags, pre-flight checks, Bubble Tea runner
└── internal/                   # Core application subsystems
    ├── config/                 # TOML configuration parser with defaults & git remote inference
    ├── github/                 # In-process GraphQL client using go-gh/v2
    ├── timew/                  # Subprocess runner for timew start/stop & JSON interval parsing
    └── tui/                    # Bubble Tea model: Kanban board, responsive viewports, keybindings
```

---

## Local Interface & Invariants

### Direct Symbols (`main.go`)
- `version`: Version string constant (`"0.1.0"`).
- `main()`: Root application entry point. Parses CLI flags (`--owner`, `--repo`, `--project`, `--config`, `--version`), executes pre-flight environment checks, initializes adapters, and launches Bubble Tea with `tea.WithAltScreen()`.

### Local Invariants & Pre-flights
- Required parameters `owner` and `project` must be supplied via CLI flags, `config.toml`, or Git remote detection; otherwise `main()` exits with status 1.
- If `timew` is absent from `$PATH`, time tracking is gracefully disabled with a notice instead of exiting.
- If GitHub CLI lacks the `project` OAuth scope, `main()` prints remediation guidance (`gh auth refresh -s project`) and terminates with status 1.

### Local Side Effects
- Spawns pre-flight subprocesses (`exec.LookPath("timew")`, `gh auth status`).
- Allocates terminal alternate screen buffer via `tea.NewProgram(model, tea.WithAltScreen())`.

---

## Subdirectories & System Boundaries

- [internal](./internal/README.md): Core decoupled subsystem packages.
  - [config](./internal/config/README.md): TOML configuration parsing, path expansion, and git remote detection.
  - [github](./internal/github/README.md): In-process GraphQL client for GitHub Projects v2 boards, cards, and custom hour fields.
  - [timew](./internal/timew/README.md): Subprocess controller for Timewarrior tracking, interval export, and decimal hour math.
  - [tui](./internal/tui/README.md): Bubble Tea Kanban TUI model, Vim keybindings, Lip Gloss styles, and adaptive sliding viewports.

---

## Global Invariants

- **Upstream Authoritative State**: GitHub Projects v2 is the authoritative source of truth. No local SQLite or cache database is maintained; all state is fetched on boot and refreshed on demand (`r`).
- **Optimistic Mutation Rollback**: Card movement across Kanban status columns updates local UI state optimistically and rolls back immediately if the asynchronous GraphQL mutation fails.
- **Mutually Exclusive Time Tracking**: At most one issue card can be actively tracked at any time. Starting tracking on a card automatically stops any running interval and synchronizes accumulated decimal hours to the custom field on GitHub.
- **Strict Viewport Containment**: Total rendered TUI output height is dynamically budgeted and clamped to terminal height $H$, ensuring headers, column headers, and status bars never scroll off screen.

---

## Global Side Effects

- **GitHub GraphQL API**: Mutates project item single-select status options (`updateProjectV2ItemFieldValue`) and float numbers (`updateProjectV2ItemFieldValue`).
- **Timewarrior Storage**: Modifies local Timewarrior database on disk via `timew start` and `timew stop`.
- **Filesystem Operations**: Reads `config.toml` from `~/.config/gh-terrorizer/`; creates and unlinks temporary markdown files in `os.TempDir()` during `$EDITOR` editing sessions.

---

## Features

* **Terminal Kanban Workstation**: Inspect project columns, view issue and PR cards, and transition cards across workflow columns.
* **In-Process GitHub GraphQL API**: Connects directly to GitHub via `go-gh/v2` with HTTP keep-alive pools. Zero subprocess spawning for remote operations.
* **Automated Timewarrior Integration**:
  * Press `t` to toggle time tracking for any card on the board.
  * Starting a new card automatically stops the previous active session.
  * On stop, elapsed seconds $\Delta s$ are converted to decimal hours:
    $$\Delta h = \frac{\Delta s}{3600}$$
  * Formatted hours are automatically accumulated into the designated GitHub Projects v2 float custom field (e.g. `1.50h` + `0.50h` = `2.00h`).
* **Tokyo Night Aesthetics**: Beautiful syntax-aware palette (`storm` default, `night` variant) styled with Lip Gloss.
* **Vim Motions Throughout**: Navigate columns and cards effortlessly using `h`, `j`, `k`, `l`, `g`, `G`, `H`, `L`, `t`, `e`, `r`.
* **Responsive Multi-Column Kanban & Sliding Viewport**:
  * Automatically renders as many columns side-by-side as fit your terminal width ($K_{\text{vis}} = \min(N, \lfloor W / W_{\min} \rfloor)$).
  * Horizontally scrollable viewport keeps the active column in focus with visual indicators (`◀`, `▶`).
  * Displays 2 to 3 columns side-by-side even on compact 60–80 column terminals, falling back to tabbed single-column mode only on ultra-narrow windows ($W < 50$).
* **Zero Credential Configuration**: Uses the authenticated session from `gh auth login`. No Personal Access Tokens or API keys to manage.
* **External Editor Support**: Press `e` on any card to suspend Bubble Tea and edit the markdown body directly in your `$EDITOR` or `$VISUAL`.

---

## Prerequisites

1. **Go**: `1.22+` (tested on Go 1.27)
2. **GitHub CLI (`gh`)**: Authenticated with the `project` OAuth scope:
   ```bash
   gh auth refresh -s project
   ```
3. **Timewarrior (`timew`)**: Installed and accessible in `$PATH`:
   ```bash
   # Arch Linux
   sudo pacman -S timew
   # Debian / Ubuntu
   sudo apt install timewarrior
   # macOS (Homebrew)
   brew install timewarrior
   ```
4. **Nerd Font**: (Optional, recommended for icons such as `󱎫`, ``, ``).

---

## Installation & Build

```bash
# Clone the repository
git clone https://github.com/ejablonski/gh-terrorizer.git
cd gh-terrorizer

# Build binary
go build -o bin/gh-terrorizer .

# Run
./bin/gh-terrorizer --owner <owner> --project <project_number>
```

If launched inside a Git repository clone, `gh-terrorizer` automatically inspects `git remote get-url origin` to resolve the default owner and repository.

---

## Configuration

Settings can be customized via `~/.config/gh-terrorizer/config.toml` (or via the `$GH_TERRORIZER_CONFIG` environment variable):

```toml
default_owner = "ejablonski"
default_repo = "gh-terrorizer"
project_number = 1

[timewarrior]
enabled = true
hours_custom_field = "Hours Spent"  # Name of GitHub Projects v2 Number custom field
rounding_precision = 2              # Decimal places (e.g. 0.50)
tag_prefix = "gh-"                  # Tag prefix passed to timew start (e.g. gh-42)

[theme]
variant = "storm"                   # "storm" or "night"
show_icons = true                   # Set false to disable Nerd Font symbols
```

---

## Keybindings

| Key | Action | Description |
| :---: | :--- | :--- |
| `h` / `←` | Move Left | Select adjacent left column |
| `l` / `→` | Move Right | Select adjacent right column |
| `j` / `↓` | Move Down | Select next card in column |
| `k` / `↑` | Move Up | Select previous card in column |
| `g` | Jump Top | Jump to topmost card in column |
| `G` | Jump Bottom | Jump to bottommost card in column |
| `H` / `Shift+←` | Move Card Left | Shift focused card to adjacent left column (optimistic update) |
| `L` / `Shift+→` | Move Card Right | Shift focused card to adjacent right column (optimistic update) |
| `m` | Move Mode | Enter move mode; press `h`/`l` to drop card, `Esc` to cancel |
| `t` | Toggle Timer | Start or stop Timewarrior tracking and sync hours to GitHub |
| `e` | Edit Body | Suspend TUI and edit card body in `$EDITOR` |
| `r` | Refresh | Fetch latest project board state from GitHub |
| `1`–`9` | Select Tab | Direct jump to column $N$ in tabbed mode |
| `Tab` | Next Tab | Cycle forward through columns |
| `Shift+Tab` | Previous Tab | Cycle backward through columns |
| `Esc` | Clear / Dismiss | Dismiss error toast or cancel move mode |
| `q` / `Ctrl+C` | Quit | Exit `gh-terrorizer` |

---

## System Verification

Run all test suites with race detection:

```bash
go test -v -race ./...
```

---

## License

This project is licensed under the [MIT License](LICENSE).
