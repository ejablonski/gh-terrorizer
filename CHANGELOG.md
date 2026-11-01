# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Look & Feel Visual Overhaul**: Adopted sharp `lipgloss.ThickBorder()` container frames across Kanban columns, cards, and modal frames, establishing a unified, high-contrast terminal workstation aesthetic.
- **Encapsulated Bordered Frame Helper**: Implemented `renderBorderedFrameWithTitle` to dynamically derive inner dimensions, border glyphs, and colors directly from Lip Gloss style definitions. Seamlessly injects top titles into thick or rounded containers while mathematically guaranteeing flush rectangular alignment ($w_{\text{line}} = \text{outerWidth}$) across arbitrary terminal dimensions, border styles, and padding settings.
- **Adaptive 3-Row Card Layout with Collapsible Metadata**: Cards dynamically collapse when neither milestone nor labels are assigned ($h_{\text{inner}} = 2$, $h_{\text{outer}} = 4$ lines vs expanded $h_{\text{inner}} = 3$, $h_{\text{outer}} = 5$ lines), increasing visible card density by up to $33\%$ on standard terminals ($H \le 24$).
- **Configurable Label-to-Color Type Indicators**: Introduced `[theme.issue_type_labels]` in `config.toml` mapping issue labels to circle indicator colors (`●`) with first-match resolution and comment fallback, keeping configuration cleanly decoupled from domain models.
- **Domain Entity Type Preservation**: Explicit item type glyphs (` ` for Pull Requests, ` ` for Draft Issues, ` ` for Standard Issues) displayed on Row 2 to maintain entity differentiation on cards.
- **Unambiguous Iconography**: Replaced ambiguous milestone icon with signpost flag ` ` (`[M]`), added type circle `● ` (`* `), and replaced `[TRACKING]` textual badge with active green clock icon (`󱎫`) via `CardHoursActive`.

### Changed
- Removed inter-card vertical bottom margins (`MarginBottom(1)`), achieving zero-gap card packing across Kanban columns.
- Reorganized documentation structure by moving legacy documents from `docs/1/` to `docs/designs/` and merging design sitemaps.

### Fixed
- **Modal Frame Corner Alignment**: Fixed outer width coordinate calculation by configuring `ModalFrame.Width(wModal - 2)`, ensuring top border title injection (`┏━ ... ━┓`), vertical side borders (`┃`), and bottom border (`┗━ ... ━┛`) align strictly flush across all terminal dimensions without horizontal offset or line wrapping.
- **ANSI Escape Sequence Slicing**: Pre-truncated unstyled titles on Row 2 before applying Lip Gloss styles, and applied ANSI-aware truncation (`ansi.Truncate`) on Row 3, eliminating escape code corruption and color leakage across lines.
- **Runtime Nil Pointer Safety**: Added explicit `m.cfg == nil` guard in `resolveLabelColor`, preventing runtime panics ($SIGSEGV$) during headless testing or default initialization.
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
