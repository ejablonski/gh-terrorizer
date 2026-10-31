package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mockGQL struct {
	doFunc func(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error
}

func (m *mockGQL) DoWithContext(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
	if m.doFunc != nil {
		return m.doFunc(ctx, query, variables, response)
	}
	return errors.New("unimplemented mock doFunc")
}

func TestFetchBoard_Success(t *testing.T) {
	sampleJSON := `{
		"repositoryOwner": {
			"projectV2": {
				"id": "PVT_123",
				"title": "Roadmap Kanban",
				"fields": {
					"nodes": [
						{
							"id": "PVTF_status",
							"name": "Status",
							"dataType": "SINGLE_SELECT",
							"options": [
								{"id": "opt_todo", "name": "Todo", "color": "BLUE"},
								{"id": "opt_done", "name": "Done", "color": "GREEN"}
							]
						},
						{
							"id": "PVTF_hours",
							"name": "Hours Spent",
							"dataType": "NUMBER"
						}
					]
				},
				"items": {
					"pageInfo": {
						"hasNextPage": false,
						"endCursor": ""
					},
					"nodes": [
						{
							"id": "item_1",
							"type": "ISSUE",
							"fieldValues": {
								"nodes": [
									{
										"name": "Todo",
										"optionId": "opt_todo",
										"field": {"id": "PVTF_status", "name": "Status"}
									},
									{
										"number": 1.5,
										"field": {"id": "PVTF_hours", "name": "Hours Spent"}
									}
								]
							},
							"content": {
								"number": 42,
								"title": "Fix Auth Bug",
								"body": "Detailed description",
								"state": "OPEN",
								"milestone": {
									"title": "v1.0",
									"dueOn": "2026-12-31T00:00:00Z"
								},
								"labels": {
									"nodes": [{"name": "bug", "color": "d73a4a"}]
								}
							}
						},
						{
							"id": "item_2",
							"type": "DRAFT_ISSUE",
							"fieldValues": {
								"nodes": [
									{
										"name": "Done",
										"optionId": "opt_done",
										"field": {"id": "PVTF_status", "name": "Status"}
									}
								]
							},
							"content": {
								"title": "Explore Idea",
								"body": "Draft notes"
							}
						}
					]
				}
			}
		}
	}`

	mock := &mockGQL{
		doFunc: func(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
			return json.Unmarshal([]byte(sampleJSON), response)
		},
	}

	client, err := NewClient(mock)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	board, err := client.FetchBoard(context.Background(), "myorg", 1, "Hours Spent")
	if err != nil {
		t.Fatalf("FetchBoard failed: %v", err)
	}

	if board.ID != "PVT_123" {
		t.Errorf("expected ID 'PVT_123', got '%s'", board.ID)
	}
	if board.Title != "Roadmap Kanban" {
		t.Errorf("expected Title 'Roadmap Kanban', got '%s'", board.Title)
	}
	if board.StatusFieldID != "PVTF_status" {
		t.Errorf("expected StatusFieldID 'PVTF_status', got '%s'", board.StatusFieldID)
	}
	if board.HoursFieldID != "PVTF_hours" {
		t.Errorf("expected HoursFieldID 'PVTF_hours', got '%s'", board.HoursFieldID)
	}
	if len(board.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(board.Columns))
	}
	if board.Columns[0].Name != "Todo" || board.Columns[1].Name != "Done" {
		t.Errorf("unexpected column names: %v", board.Columns)
	}
	if len(board.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(board.Items))
	}

	// Verify item 1
	item1 := board.Items[0]
	if item1.ID != "item_1" || item1.Number != 42 || item1.Title != "Fix Auth Bug" {
		t.Errorf("unexpected item1 fields: %+v", item1)
	}
	if item1.StatusOptionID != "opt_todo" {
		t.Errorf("expected status 'opt_todo', got '%s'", item1.StatusOptionID)
	}
	if item1.HoursSpent != 1.5 {
		t.Errorf("expected 1.5 hours, got %f", item1.HoursSpent)
	}
	if item1.Milestone == nil || item1.Milestone.Title != "v1.0" {
		t.Errorf("expected milestone 'v1.0', got %+v", item1.Milestone)
	}
	if len(item1.Labels) != 1 || item1.Labels[0].Name != "bug" {
		t.Errorf("expected label 'bug', got %+v", item1.Labels)
	}

	// Verify draft item 2
	item2 := board.Items[1]
	if item2.ID != "item_2" || item2.Title != "Explore Idea" {
		t.Errorf("unexpected item2 fields: %+v", item2)
	}
	if item2.StatusOptionID != "opt_done" {
		t.Errorf("expected status 'opt_done', got '%s'", item2.StatusOptionID)
	}
}

func TestMoveCard_Success(t *testing.T) {
	mutationInvoked := false
	mock := &mockGQL{
		doFunc: func(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
			if strings.Contains(query, "updateProjectV2ItemFieldValue") {
				mutationInvoked = true
				if variables["projectId"] != "proj1" || variables["itemId"] != "item1" || variables["optionId"] != "opt2" {
					t.Errorf("unexpected variables: %+v", variables)
				}
				return nil
			}
			return errors.New("unexpected query")
		},
	}

	client, _ := NewClient(mock)
	err := client.MoveCard(context.Background(), "proj1", "item1", "fld1", "opt2")
	if err != nil {
		t.Fatalf("MoveCard returned error: %v", err)
	}
	if !mutationInvoked {
		t.Error("expected mutation to be invoked")
	}
}

func TestUpdateHours_Success(t *testing.T) {
	mutationInvoked := false
	mock := &mockGQL{
		doFunc: func(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
			if strings.Contains(query, "updateProjectV2ItemFieldValue") {
				mutationInvoked = true
				if variables["projectId"] != "proj1" || variables["itemId"] != "item1" || variables["totalHours"] != 2.75 {
					t.Errorf("unexpected variables: %+v", variables)
				}
				return nil
			}
			return errors.New("unexpected query")
		},
	}

	client, _ := NewClient(mock)
	err := client.UpdateHours(context.Background(), "proj1", "item1", "fld1", 2.75)
	if err != nil {
		t.Fatalf("UpdateHours returned error: %v", err)
	}
	if !mutationInvoked {
		t.Error("expected mutation to be invoked")
	}
}

func TestFetchBoard_StatusFieldFallback(t *testing.T) {
	sampleJSON := `{
		"repositoryOwner": {
			"projectV2": {
				"id": "PVT_fallback",
				"title": "Fallback Board",
				"fields": {
					"nodes": [
						{
							"id": "PVTF_stage",
							"name": "stage",
							"dataType": "SINGLE_SELECT",
							"options": [
								{"id": "opt_backlog", "name": "Backlog", "color": "GRAY"},
								{"id": "opt_active", "name": "Active", "color": "BLUE"},
								{"id": "opt_completed", "name": "Completed", "color": "GREEN"}
							]
						}
					]
				},
				"items": {
					"pageInfo": {"hasNextPage": false, "endCursor": ""},
					"nodes": [
						{
							"id": "item_100",
							"type": "ISSUE",
							"fieldValues": {
								"nodes": [
									{
										"name": "Backlog",
										"optionId": "opt_backlog",
										"field": {"id": "PVTF_stage", "name": "stage"}
									}
								]
							},
							"content": {
								"number": 100,
								"title": "Fallback Issue",
								"body": "Testing fallback"
							}
						}
					]
				}
			}
		}
	}`

	mock := &mockGQL{
		doFunc: func(ctx context.Context, query string, variables map[string]interface{}, response interface{}) error {
			return json.Unmarshal([]byte(sampleJSON), response)
		},
	}

	client, err := NewClient(mock)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	board, err := client.FetchBoard(context.Background(), "myorg", 1, "Hours")
	if err != nil {
		t.Fatalf("FetchBoard failed: %v", err)
	}

	if board.StatusFieldID != "PVTF_stage" {
		t.Errorf("expected StatusFieldID 'PVTF_stage', got '%s'", board.StatusFieldID)
	}
	if len(board.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(board.Columns))
	}
	if board.Columns[0].Name != "Backlog" || board.Columns[1].Name != "Active" || board.Columns[2].Name != "Completed" {
		t.Errorf("unexpected columns: %+v", board.Columns)
	}
	if len(board.Items) != 1 || board.Items[0].StatusOptionID != "opt_backlog" {
		t.Errorf("unexpected items: %+v", board.Items)
	}
}
