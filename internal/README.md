# internal

## Overview
The `internal` directory contains the core application subsystems for `gh-terrorizer`, separating configuration loading, GitHub API communication, Timewarrior process control, and the terminal user interface into decoupled, unit-tested packages.

## Subdirectories & Boundaries
- [config](./config/README.md): TOML configuration parsing, path expansion, defaults, and Git remote inference.
- [github](./github/README.md): In-process GitHub Projects v2 GraphQL client for boards, columns, cards, and custom hour fields.
- [timew](./timew/README.md): Subprocess controller for Timewarrior CLI tracking, interval export, and decimal hour calculation.
- [tui](./tui/README.md): Bubble Tea Kanban TUI model, Vim keybindings, Lip Gloss styles, and adaptive sliding viewports.
