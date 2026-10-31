# github

## Overview
Package `github` implements in-process GitHub Projects v2 communication via GraphQL, powered by `github.com/cli/go-gh/v2`. It handles project board fetching, pagination, status field resolution, card status movement mutations, and custom float hour field updates.

## Interface
- `GraphQLExecutor`: Interface abstracting GraphQL execution (`DoWithContext(ctx, query, variables, response)`).
- `ProjectBoard`: Represents a project board containing `ID`, `Title`, `StatusFieldID`, `HoursFieldID`, `Columns`, and `Items`.
- `Column`: Defines a Kanban status column with `ID`, `Name`, and `Color`.
- `ProjectItem`: Represents a board card (`ID`, `Type`, `StatusOptionID`, `HoursSpent`, `Title`, `Body`, `Number`, `State`, `Labels`, `Milestone`).
- `Label`: Issue or PR label badge (`Name`, `Color`).
- `Milestone`: Issue milestone metadata (`Title`, `DueOn`).
- `Client`: In-process GraphQL API client.
- `NewClient(gql GraphQLExecutor) (*Client, error)`: Factory creating a client; defaults to `api.DefaultGraphQLClient()` if `gql` is nil.
- `CheckProjectScope(ctx context.Context) error`: Inspects `gh auth status` output to ensure the `project` or `read:project` OAuth scope is present.
- `(*Client) FetchBoard(ctx context.Context, owner string, number int, hoursFieldName string) (*ProjectBoard, error)`: Queries GitHub Projects v2 across User or Organization owners, paging through cards and extracting status options and hour values.
- `(*Client) MoveCard(ctx context.Context, projectID, itemID, fieldID, optionID string) error`: Mutates item status via `updateProjectV2ItemFieldValue`.
- `(*Client) UpdateHours(ctx context.Context, projectID, itemID, fieldID string, totalHours float64) error`: Updates the custom float field via `updateProjectV2ItemFieldValue`.

## Invariants
- `FetchBoard` locates the Kanban status field by matching `"status"` case-insensitively with options; if absent, it falls back to the first available single-select field with options.
- `hoursFieldName` defaults to `"Hours Spent"` if an empty string is provided.
- `CheckProjectScope` returns an actionable error if GitHub CLI is logged in without the `project` scope.

## Side Effects
- Issues authenticated HTTPS GraphQL network requests to GitHub (`api.github.com/graphql`) via `go-gh/v2`.
- Executes `gh auth status` subprocess to verify CLI authentication state.

## Verification
- Run: `go test ./internal/github`
