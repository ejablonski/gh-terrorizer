# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
