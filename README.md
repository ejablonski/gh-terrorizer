# gh-terrorizer Blueprint

[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

`gh-terrorizer` is a keyboard-driven 3-tier terminal user interface (TUI) workstation for managing **GitHub Projects v2** Kanban boards and synchronizing task execution time directly into project custom float fields via **Timewarrior** (`timew`).

Built with Go, [Charm's Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss), it executes GitHub GraphQL queries and mutations in-process via [`github.com/cli/go-gh/v2`](https://github.com/cli/go-gh), transparently reusing existing GitHub CLI credentials, hosts, and keychain tokens.

---

## System Overview

`gh-terrorizer` bridges terminal-native developer workflows with cloud-hosted GitHub Projects v2 boards. The application runs in the terminal alternate screen buffer, structuring the terminal display into a Lazygit-inspired 3-tier workstation:
1. **Top Bar**: Displays repository name, project title, live in-memory Timewarrior tracking status (`tag` and elapsed time), and on-demand synchronization timestamps.
2. **Main Application Area**: Dynamically renders either the responsive multi-column Kanban board or a centered, scrollable Issue Inspector Modal with word-wrapped descriptions and metadata badges.
3. **Bottom Bar**: Context-sensitive Vim action hints that adapt between board navigation and modal inspection, alongside transient notification toasts.

### Workstation Interface

```text
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ 󰄶 gh-terrorizer │ ejablonski/gh-terrorizer #1 │ 󱎫 gh-42 [01:23:45] │ 󰄬 Synced (r: sync) │  <- Top Bar
├────────────────────────────────────────────────────────────────────────────────────────┤
│ ┏━ Todo (3) ━━━━┓ ┏━ In Progress (2) ━┓ ┏━ Done (5) ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓ │
│ ┃ ●       󱎫 1.0h┃ ┃ ┏━━━━━━━━━━━━━━━┓ ┃ ┃ ●                               󱎫 0.5h     ┃ │  <- Main Area
│ ┃  #10 Task 1  ┃ ┃ ┃ ●       󱎫 0.5h┃ ┃ ┃  #05 Setup Repo                            ┃ │     (Kanban View)
│ ┃   v1.0       ┃ ┃ ┃  #42 Task 3  ┃ ┃ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛ │
│ ┃ ┏━━━━━━━━━━━┓ ┃ ┃ ┃  bug          ┃ ┃                                                ┃ │
│ ┃ ┃ ●         ┃ ┃ ┃ ┗━━━━━━━━━━━━━━━┛ ┃                                                ┃ │
│ ┃ ┃  #20 Task┃ ┃ ┗━━━━━━━━━━━━━━━━━━━┛                                                ┃ │
│ ┃ ┗━━━━━━━━━━━┛ ┃                                                                      ┃ │
│ ┗━━━━━━━━━━━━━━━┛                                                                      ┃ │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ h/l: cols • j/k: cards • m: move • H/L: shift • t: timer • r: sync • Enter/i: inspect • q: quit │  <- Bottom Bar
└────────────────────────────────────────────────────────────────────────────────────────┘
```

When inspecting an issue (`Enter` or `i`), the centered Issue Inspector Modal replaces the Kanban board in the main tier:

```text
┌────────────────────────────────────────────────────────────────────────────────────────┐
│ 󰄶 gh-terrorizer │ ejablonski/gh-terrorizer #1 │ 󱎫 gh-42 [01:23:45] │ 󰄬 Synced (r: sync) │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                   ┏━ Issue #42: Authentication Token Refresh ━━━━━━━━┓                 │
│                   ┃ OPEN   v1.0  󱎫 2.50h                            ┃                 │
│                   ┃ ──────────────────────────────────────────────── ┃                 │
│                   ┃ Description:                                     ┃                 │
│                   ┃ When the access token expires, the client fails  ┃                 │
│                   ┃ to refresh credentials against GitHub OAuth API. ┃                 │
│                   ┃ Steps to reproduce:                              ┃                 │
│                   ┃  1. Invalidate cached token.                     ┃                 │
│                   ┃  2. Run query command.                           ┃                 │
│                   ┃                                            [28%] ┃                 │
│                   ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛                 │
├────────────────────────────────────────────────────────────────────────────────────────┤
│ j/k: scroll • d/u: half page • g/G: top/bottom • e: edit in $EDITOR • Esc/q: back      │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### Architecture Overview

```text
gh-terrorizer/
├── main.go                     # Entry point: CLI flags, pre-flight checks, Bubble Tea runner
└── internal/                   # Core application subsystems
    ├── config/                 # TOML configuration parser with defaults & git remote inference
    ├── github/                 # In-process GraphQL client using go-gh/v2
    ├── timew/                  # Subprocess runner for timew start/stop & JSON interval parsing
    └── tui/                    # Bubble Tea workstation model, render engine, viewports, modal
```

---

## Local Interface & Invariants

### Direct Symbols ([`main.go`](file:///home/ejablonski/dev/gh-terrorizer/main.go))
- `version`: Version string constant (`"0.1.0"`).
- [`main()`](file:///home/ejablonski/dev/gh-terrorizer/main.go): Root application entry point. Parses CLI flags (`--owner`, `--repo`, `--project`, `--config`, `--version`), executes pre-flight environment checks, initializes adapters, and launches Bubble Tea with `tea.WithAltScreen()`.

### Local Invariants & Pre-flights
- Required parameters `owner` and `project` must be supplied via CLI flags, `config.toml`, or Git remote detection; otherwise `main()` exits with status $1$.
- If `timew` is absent from `$PATH`, time tracking is gracefully disabled with a notice instead of exiting.
- If GitHub CLI lacks the `project` OAuth scope, `main()` prints remediation guidance (`gh auth refresh -s project`) and terminates with status $1$.

### Local Side Effects
- Spawns pre-flight subprocesses (`exec.LookPath("timew")`, `gh auth status`).
- Allocates terminal alternate screen buffer via `tea.NewProgram(model, tea.WithAltScreen())`.

---

## Subdirectories & System Boundaries

- [`internal`](file:///home/ejablonski/dev/gh-terrorizer/internal/README.md): Core decoupled subsystem packages.
  - [`config`](file:///home/ejablonski/dev/gh-terrorizer/internal/config/README.md): TOML configuration parsing, path expansion, and git remote detection.
  - [`github`](file:///home/ejablonski/dev/gh-terrorizer/internal/github/README.md): In-process GraphQL client for GitHub Projects v2 boards, cards, and custom hour fields.
  - [`timew`](file:///home/ejablonski/dev/gh-terrorizer/internal/timew/README.md): Subprocess controller for Timewarrior tracking, interval export, and decimal hour math.
  - [`tui`](file:///home/ejablonski/dev/gh-terrorizer/internal/tui/README.md): Bubble Tea workstation TUI model, pure layout renderer, Tokyo Night styles, sliding viewports, and Issue Inspector Modal.

---

## Global Invariants

- **Upstream Authoritative State**: GitHub Projects v2 is the authoritative source of truth. No local SQLite or cache database is maintained; all state is fetched on boot and refreshed on demand (`r`).
- **Optimistic Mutation Rollback**: Card movement across Kanban status columns updates local UI state optimistically and rolls back immediately if the asynchronous GraphQL mutation fails.
- **Mutually Exclusive Time Tracking**: At most one issue card can be actively tracked at any time. Starting tracking on a card automatically stops any running interval and synchronizes accumulated decimal hours to the custom field on GitHub.
- **Strict Viewport Containment**: Total rendered TUI output height is dynamically budgeted and clamped to terminal height $H$ by setting $$h_{\text{main}} = H - \text{lipgloss.Height}(\text{topBar}) - \text{lipgloss.Height}(\text{bottomBar})$$, ensuring headers, column headers, modals, and status bars never scroll off screen.

---

## Global Side Effects

- **GitHub GraphQL API**: Mutates project item single-select status options (`updateProjectV2ItemFieldValue`) and float numbers (`updateProjectV2ItemFieldValue`).
- **Timewarrior Storage**: Modifies local Timewarrior database on disk via `timew start` and `timew stop`.
- **Filesystem Operations**: Reads `config.toml` from `~/.config/gh-terrorizer/`; creates and unlinks temporary markdown files in `os.TempDir()` during `$EDITOR` editing sessions.

---

## Features

* **3-Tier Terminal Workstation**: Structured Lazygit-style workspace separating persistent session awareness, adaptable main work area, and dynamic action strips.
* **Centered Issue Inspector Modal**: Press `Enter` or `i` over any card to view detailed state, milestone badges, tracked hours, and full word-wrapped markdown body text in an embedded scrollable viewport.
* **Ambient Time Tracking & In-Memory Elapsed Timer**: Persistent Top Bar displays the running Timewarrior tag and elapsed tracking duration updated every $1\text{ s}$ without subprocess overhead.
* **On-Demand Board Synchronization**: Press `r` at any time to execute an asynchronous board refresh without resetting cursor coordinates or blocking the interface.
* **In-Process GitHub GraphQL API**: Connects directly to GitHub via `go-gh/v2` with HTTP keep-alive pools. Zero subprocess spawning for remote operations.
* **Automated Timewarrior Integration**:
  * Press `t` to toggle time tracking for any card on the board.
  * Starting a new card automatically stops the previous active session.
  * On stop, elapsed seconds $\Delta s$ are converted to decimal hours:
    $$\Delta h = \frac{\Delta s}{3600}$$
  * Formatted hours are automatically accumulated into the designated GitHub Projects v2 float custom field (e.g. `1.50h` + `0.50h` = `2.00h`).
* **Tokyo Night Aesthetics**: Beautiful syntax-aware palette (`storm` default, `night` variant) styled with Lip Gloss.
* **Vim Motions Throughout**: Navigate columns and cards effortlessly using `h`, `j`, `k`, `l`, `g`, `G`, `H`, `L`, `t`, `e`, `r`, `Enter`, `i`, `Esc`, `q`.
* **Responsive Multi-Column Kanban & Sliding Viewport**:
  * Automatically renders as many columns side-by-side as fit your terminal width ($K_{\text{vis}} = \min(N, \lfloor W / W_{\min} \rfloor)$).
  * Horizontally scrollable viewport keeps the active column in focus with visual indicators (`◀`, `▶`).
  * Displays $2$ to $3$ columns side-by-side even on compact $60\text{--}80$ column terminals, falling back to tabbed single-column mode only on ultra-narrow windows ($W < 50$).
* **Zero Credential Configuration**: Uses the authenticated session from `gh auth login`. No Personal Access Tokens or API keys to manage.
* **External Editor Support**: Press `e` on any card or inside the Inspector Modal to suspend Bubble Tea and edit the markdown body directly in your `$EDITOR` or `$VISUAL`.

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
4. **Nerd Font**: (Optional, recommended for icons such as `󰄶`, `󱎫`, `󰄬`, `󰑮`, ``, ``).

---

## Installation & Build

```bash
# Clone the repository
git clone https://github.com/ejablonski/gh-terrorizer.git
cd gh-terrorizer

# Build binary
go build -o /tmp/gh-terrorizer .

# Run
/tmp/gh-terrorizer --owner <owner> --project <project_number>
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

[theme.issue_type_labels]
"bug" = "#f7768e"                   # Map GitHub label to circle indicator hex color
"feature" = "#7aa2f7"
"task" = "#e0af68"
```

---

## Keybindings

### Kanban Board Mode

| Key | Action | Description |
| :---: | :--- | :--- |
| `h` / `←` | Move Left | Select adjacent left column (or drop card if moving) |
| `l` / `→` | Move Right | Select adjacent right column (or drop card if moving) |
| `j` / `↓` | Move Down | Select next card in column |
| `k` / `↑` | Move Up | Select previous card in column |
| `g` | Jump Top | Jump to topmost card in column |
| `G` | Jump Bottom | Jump to bottommost card in column |
| `H` / `Shift+←` | Shift Left | Shift focused card to adjacent left column (optimistic update) |
| `L` / `Shift+→` | Shift Right | Shift focused card to adjacent right column (optimistic update) |
| `m` | Move Mode | Enter move mode; press `h`/`l` to drop card, `Esc` to cancel |
| `Enter` / `i` | Inspect Card | Open Issue Inspector Modal for focused card |
| `t` | Toggle Timer | Start or stop Timewarrior tracking and sync hours to GitHub |
| `e` | Edit Body | Suspend TUI and edit card body in `$EDITOR` |
| `r` | Refresh | Fetch latest project board state from GitHub |
| `1`–`9` | Select Tab | Direct jump to column $N$ in tabbed mode |
| `Tab` | Next Tab | Cycle forward through columns |
| `Shift+Tab` | Previous Tab | Cycle backward through columns |
| `Esc` | Clear / Dismiss | Dismiss error toast or cancel move mode |
| `q` | Quit | Exit `gh-terrorizer` |
| `Ctrl+C` | Force Quit | Unconditionally terminate application |

### Issue Inspector Modal Mode

| Key | Action | Description |
| :---: | :--- | :--- |
| `j` / `↓` | Scroll Down | Scroll issue body down by $1$ line |
| `k` / `↑` | Scroll Up | Scroll issue body up by $1$ line |
| `d` / `PgDn` / `Space` | Half Page Down | Scroll viewport down by half page |
| `u` / `PgUp` | Half Page Up | Scroll viewport up by half page |
| `g` / `Home` | Top | Jump to top of issue description |
| `G` / `End` | Bottom | Jump to end of issue description |
| `e` | Edit Body | Suspend TUI and edit issue description in `$EDITOR` |
| `Esc` / `q` | Close Modal | Return to Kanban board with card focus preserved |
| `Ctrl+C` | Force Quit | Unconditionally terminate application |

---

## System Verification

Run all test suites with race detection:

```bash
go test -v -race ./...
```

Verify package test coverage:

```bash
go test -coverprofile=/tmp/coverage.out ./internal/tui/...
go tool cover -func=/tmp/coverage.out
```

---

## License

This project is licensed under the [MIT License](LICENSE).
