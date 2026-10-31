# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **3-Tier Terminal Workstation Layout**: Structured Lazygit-inspired workstation partitioning the viewport into a persistent Top Bar (branding, repository context, live Timewarrior status with in-memory $1\text{ s}$ elapsed timer, and on-demand synchronization timestamps), an adaptable Main Area (Kanban board or Issue Inspector Modal), and a contextual Bottom Bar with dynamic Vim action hints and notification toasts.
- **Focused Issue Inspector Modal**: Centered modal view (`Enter`/`i` on focused card) replacing the board view in the main tier with metadata badges (state, milestone, tracked hours), horizontal divider, and embedded scrollable viewport (`bubbles/viewport`) with Lip Gloss word wrapping.
- **Modal Navigation Motions**: Viewport vertical scrolling (`j`/`k`, `d`/`u`, `g`/`G`, arrow keys, page up/down), external `$EDITOR` launching (`e`), and non-destructive modal dismissal returning focus to the previous card (`Esc`/`q`).
- **On-Demand GitHub Board Synchronization**: Ambient board refresh key (`r`) triggering asynchronous GraphQL queries without blocking the UI (`loading = false`) or resetting active cursor positions.
- **Pure Layout Rendering Architecture**: Decoupled visual rendering into deterministic, unit-testable pure functions in `internal/tui/render.go`.
- **Design Documentation**: Technical design specification for the 3-tier workstation in `docs/designs/workstation_modal.md` and centralized sitemap in `docs/designs/SITEMAP.md`.

### Changed
- Reorganized documentation structure by moving legacy documents from `docs/1/` to `docs/designs/` and merging design sitemaps.

### Fixed
- **Terminal Vertical Height Overflow**: Replaced `EmptyNotice` (`Padding(2, 4)`) with dedicated zero-padding `ScrollIndicator` for card scroll indicators (`↑ %d more`, `↓ %d more`), fixed expansion budgeting, and enforced strict boundary clamping in `renderColumn`, `renderSingleColumnTabbed`, and `View()`, completely preventing the Top Bar and column headers from scrolling off the top of the terminal screen across all terminal dimensions $H \times W$.

## [0.1.0] - 2026-10-31

### Added
- **Terminal Kanban Workstation**: Bubble Tea and Lip Gloss powered interactive Kanban board for GitHub Projects v2.
- **Adaptive Multi-Column Layout**: Responsive side-by-side column grid with horizontal sliding viewport (`colOffset`), boundary scroll indicators (`◀`, `▶`), and compact single-column fallback for narrow windows ($W < 50$).
- **Vim Navigation Keybindings**: Support for column and card movement (`h`, `j`, `k`, `l`, `g`, `G`), optimistic horizontal card moves (`H`, `L`, `m`), board refresh (`r`), and direct tab selection (`1`–`9`, `Tab`, `Shift+Tab`).
- **Timewarrior Synchronization**: Subprocess wrapper for the Timewarrior CLI (`timew`) supporting single-card tracking toggles (`t`), session tracking detection, decimal hour calculation ($h = s / 3600$), and additive float updates to GitHub custom fields.
- **In-Process GitHub GraphQL Client**: Communicates directly with GitHub via `github.com/cli/go-gh/v2` without shell subprocess overhead, reusing local CLI credentials.
- **External Editor Markdown Editing**: Suspend Bubble Tea via `tea.ExecProcess` (`e`) to modify card descriptions in `$EDITOR` or `$VISUAL`.
- **Tokyo Night Styling**: Syntax-aware theming (`storm` default, `night` variant) with Nerd Font glyphs and ASCII fallbacks.
- **TOML Configuration System**: Configuration management with production defaults, path expansion (`~`), and automatic owner/repository detection via `git remote get-url origin`.
- **Pre-flight Health Checks**: Startup checks for `timew` binary availability and GitHub CLI `project` OAuth scope.
- **EGM Documentation Contracts**: Elephant-and-Goldfish Model architectural contracts across all packages (`README.md`, `internal/README.md`, `internal/*/README.md`).
- **Comprehensive Test Suite**: Unit tests with race detection (`go test -v -race ./...`) across all packages.

### Fixed
- Fixed single-column layout collapse where boards with $\ge 5$ columns or terminal width $< 120$ prematurely degraded to single-column tabbed mode.
- Fixed terminal height overflow where cards exceeded screen height and caused column headers to scroll off top in alternate screen mode.
- Fixed case-sensitive status field resolution by implementing case-insensitive matching and single-select option fallbacks.

[Unreleased]: https://github.com/ejablonski/gh-terrorizer/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ejablonski/gh-terrorizer/releases/tag/v0.1.0
