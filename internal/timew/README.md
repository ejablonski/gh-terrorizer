# timew

## Overview
Package `timew` manages local time tracking by orchestrating the Timewarrior CLI (`timew`). It provides subroutines for querying session status, starting/stopping intervals, parsing exported interval JSON, and calculating rounded decimal hours for upstream synchronization.

## Interface
- `TimewTimeFormat`: Time layout constant `"20060102T150405Z"` used by `timew export`.
- `ExportInterval`: Represents a timewarrior JSON record (`ID`, `Start`, `End`, `Tags`, `Annotation`).
- `CommandRunner`: Function type for invoking commands (`func(ctx, name, args...) ([]byte, error)`).
- `DefaultCommandRunner`: Executes subprocesses via `os/exec.CommandContext`.
- `Tracker`: Wrapper struct coordinating `timew` CLI operations.
- `NewTracker(runner CommandRunner) *Tracker`: Tracker constructor; defaults to `DefaultCommandRunner`.
- `(*Tracker) IsActive(ctx context.Context) (bool, error)`: Queries `timew get dom.active` to check whether tracking is active.
- `(*Tracker) GetActiveTag(ctx context.Context) (string, error)`: Resolves the active session's tag via DOM query or export fallback.
- `(*Tracker) Start(ctx context.Context, tag string, extra ...string) error`: Executes `timew start <tag> [extra...]`.
- `(*Tracker) Stop(ctx context.Context) (int, error)`: Stops the current interval via `timew stop`, parses `timew export @1`, and returns duration in seconds.
- `(*Tracker) Export(ctx context.Context, filter string) ([]ExportInterval, error)`: Invokes `timew export` and deserializes the JSON array.
- `CalculateHours(elapsedSeconds int, precision int) float64`: Converts elapsed seconds to decimal hours rounded to `precision` places.
- `FormatHours(hours float64) string`: Converts decimal hours to human-readable strings (e.g. `"1h 30m"`, `"45m"`, `"0m"`).

## Invariants
- `CalculateHours` returns `0.0` for any `elapsedSeconds <= 0`.
- Negative `precision` arguments in `CalculateHours` default to `2`.
- `Stop` checks `IsActive` first and returns `(0, nil)` if no tracking session is currently active.

## Side Effects
- Spawns external CLI processes (`timew start`, `timew stop`, `timew export`, `timew get`).
- Mutates local Timewarrior state and database on disk.

## Verification
- Run: `go test ./internal/timew`
