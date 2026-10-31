package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ejablonski/gh-terrorizer/internal/config"
	"github.com/ejablonski/gh-terrorizer/internal/github"
)

type mockGitHubClient struct {
	board     *github.ProjectBoard
	movedCard bool
	updatedH  bool
}

func (m *mockGitHubClient) FetchBoard(ctx context.Context, owner string, number int, hoursFieldName string) (*github.ProjectBoard, error) {
	return m.board, nil
}

func (m *mockGitHubClient) MoveCard(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	m.movedCard = true
	return nil
}

func (m *mockGitHubClient) UpdateHours(ctx context.Context, projectID, itemID, fieldID string, totalHours float64) error {
	m.updatedH = true
	return nil
}

type mockTracker struct {
	startedTag string
	stopped    bool
	active     bool
}

func (m *mockTracker) Start(ctx context.Context, tag string, extra ...string) error {
	m.startedTag = tag
	m.active = true
	return nil
}

func (m *mockTracker) Stop(ctx context.Context) (int, error) {
	m.stopped = true
	m.active = false
	return 1800, nil // 30 minutes
}

func (m *mockTracker) IsActive(ctx context.Context) (bool, error) {
	return m.active, nil
}

func (m *mockTracker) GetActiveTag(ctx context.Context) (string, error) {
	if m.active {
		return m.startedTag, nil
	}
	return "", nil
}

func setupTestModel() (Model, *mockGitHubClient, *mockTracker) {
	board := &github.ProjectBoard{
		ID:            "PVT_test",
		Title:         "Test Project Board",
		StatusFieldID: "status_field_id",
		HoursFieldID:  "hours_field_id",
		Columns: []github.Column{
			{ID: "col_todo", Name: "Todo"},
			{ID: "col_prog", Name: "In Progress"},
			{ID: "col_done", Name: "Done"},
		},
		Items: []github.ProjectItem{
			{
				ID:             "item_1",
				Number:         10,
				Title:          "First Task",
				StatusOptionID: "col_todo",
				HoursSpent:     1.0,
			},
			{
				ID:             "item_2",
				Number:         20,
				Title:          "Second Task",
				StatusOptionID: "col_todo",
				HoursSpent:     0.5,
			},
			{
				ID:             "item_3",
				Number:         30,
				Title:          "Done Task",
				StatusOptionID: "col_done",
				HoursSpent:     3.0,
			},
		},
	}

	mockGH := &mockGitHubClient{board: board}
	mockTrk := &mockTracker{}
	cfg := config.DefaultConfig()
	cfg.DefaultOwner = "myorg"
	cfg.ProjectNumber = 1

	model := NewModel(mockGH, mockTrk, cfg)
	// Deliver board fetched message
	updatedModel, _ := model.Update(BoardFetchedMsg{Board: board, Err: nil})
	return updatedModel.(Model), mockGH, mockTrk
}

func TestModel_Navigation(t *testing.T) {
	m, _, _ := setupTestModel()

	if m.activeColumn != 0 {
		t.Errorf("expected initial active column 0, got %d", m.activeColumn)
	}

	// Move right ('l')
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 1 {
		t.Errorf("expected active column 1 after 'l', got %d", m.activeColumn)
	}

	// Move right ('l')
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 2 {
		t.Errorf("expected active column 2 after 'l', got %d", m.activeColumn)
	}

	// Move right ('l') wraps around to 0
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 0 {
		t.Errorf("expected active column 0 after wrap, got %d", m.activeColumn)
	}

	// Down ('j') selects next card
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = m2.(Model)
	if m.activeCard[0] != 1 {
		t.Errorf("expected card index 1, got %d", m.activeCard[0])
	}
	sel := m.SelectedItem()
	if sel == nil || sel.Number != 20 {
		t.Errorf("expected selected item #20, got %+v", sel)
	}

	// Up ('k') selects previous card
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = m2.(Model)
	if m.activeCard[0] != 0 {
		t.Errorf("expected card index 0, got %d", m.activeCard[0])
	}
}

func TestModel_MoveCardOptimistic(t *testing.T) {
	m, mockGH, _ := setupTestModel()

	// Initial: item_1 is in col_todo (column 0)
	sel := m.SelectedItem()
	if sel.ID != "item_1" {
		t.Fatalf("expected item_1 selected, got %s", sel.ID)
	}

	// Move right ('L')
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m = m2.(Model)

	if m.activeColumn != 1 {
		t.Errorf("expected active column 1, got %d", m.activeColumn)
	}

	// Check optimistic update on item_1
	var foundItem *github.ProjectItem
	for i := range m.board.Items {
		if m.board.Items[i].ID == "item_1" {
			foundItem = &m.board.Items[i]
			break
		}
	}
	if foundItem == nil || foundItem.StatusOptionID != "col_prog" {
		t.Errorf("expected item_1 moved to col_prog, got %+v", foundItem)
	}

	// Execute returned command
	if cmd != nil {
		msg := cmd()
		m3, _ := m.Update(msg)
		m = m3.(Model)
		if !mockGH.movedCard {
			t.Error("expected MoveCard to be called on client")
		}
	}
}

func TestModel_TimerToggle(t *testing.T) {
	m, mockGH, mockTrk := setupTestModel()

	// Press 't' on item_1
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = m2.(Model)

	if m.activeTimerItemID != "item_1" {
		t.Errorf("expected active timer on item_1, got %s", m.activeTimerItemID)
	}
	if cmd != nil {
		_ = cmd()
	}
	if mockTrk.startedTag != "gh-10" {
		t.Errorf("expected timew started with tag gh-10, got %s", mockTrk.startedTag)
	}

	// Press 't' again on item_1 to stop tracking
	m3, cmdStop := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = m3.(Model)

	if m.activeTimerItemID != "" {
		t.Errorf("expected active timer cleared, got %s", m.activeTimerItemID)
	}
	if cmdStop != nil {
		msg := cmdStop()
		m4, _ := m.Update(msg)
		m = m4.(Model)
	}

	if !mockTrk.stopped {
		t.Error("expected tracker.Stop to be called")
	}
	if !mockGH.updatedH {
		t.Error("expected GitHub client.UpdateHours to be called")
	}

	// Hours spent should have incremented from 1.0 by 0.5 (1800s) -> 1.5
	sel := m.SelectedItem()
	if sel.HoursSpent != 1.5 {
		t.Errorf("expected 1.5 hours, got %f", sel.HoursSpent)
	}
}

func TestModel_VisibleColumnCount(t *testing.T) {
	m, _, _ := setupTestModel()

	tests := []struct {
		width    int
		expected int
	}{
		{width: 40, expected: 1},  // ultra narrow
		{width: 50, expected: 2},  // fits 2
		{width: 80, expected: 3},  // fits all 3 columns
		{width: 120, expected: 3}, // fits all 3 columns (capped by board column count)
	}

	for _, tc := range tests {
		m.width = tc.width
		got := m.VisibleColumnCount()
		if got != tc.expected {
			t.Errorf("width %d: expected %d visible columns, got %d", tc.width, tc.expected, got)
		}
	}
}

func TestModel_ViewportHorizontalScroll(t *testing.T) {
	m, _, _ := setupTestModel()

	// Add 2 more columns to have 5 columns total
	m.board.Columns = append(m.board.Columns,
		github.Column{ID: "col_review", Name: "Review"},
		github.Column{ID: "col_archived", Name: "Archived"},
	)

	// Set width = 80 so only 3 columns fit side-by-side
	m.width = 80
	m.height = 30
	m.adjustViewport()

	if m.VisibleColumnCount() != 3 {
		t.Fatalf("expected 3 visible columns, got %d", m.VisibleColumnCount())
	}
	if m.colOffset != 0 {
		t.Fatalf("expected initial colOffset 0, got %d", m.colOffset)
	}

	// Move right to column 1
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 1 || m.colOffset != 0 {
		t.Errorf("col 1: expected activeCol=1, colOffset=0, got activeCol=%d, colOffset=%d", m.activeColumn, m.colOffset)
	}

	// Move right to column 2
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 2 || m.colOffset != 0 {
		t.Errorf("col 2: expected activeCol=2, colOffset=0, got activeCol=%d, colOffset=%d", m.activeColumn, m.colOffset)
	}

	// Move right to column 3 -> viewport must slide to colOffset=1
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 3 || m.colOffset != 1 {
		t.Errorf("col 3: expected activeCol=3, colOffset=1, got activeCol=%d, colOffset=%d", m.activeColumn, m.colOffset)
	}

	// Move right to column 4 -> viewport must slide to colOffset=2
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 4 || m.colOffset != 2 {
		t.Errorf("col 4: expected activeCol=4, colOffset=2, got activeCol=%d, colOffset=%d", m.activeColumn, m.colOffset)
	}

	// Render view and verify scroll indicators: left indicator ◀ and right edge
	rendered := m.View()
	if !strings.Contains(rendered, "◀") {
		t.Errorf("expected left scroll indicator '◀' in rendered view when colOffset > 0")
	}

	// Move right -> wraps to column 0 -> colOffset must reset to 0
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m2.(Model)
	if m.activeColumn != 0 || m.colOffset != 0 {
		t.Errorf("col 0 wrap: expected activeCol=0, colOffset=0, got activeCol=%d, colOffset=%d", m.activeColumn, m.colOffset)
	}

	// Render view at colOffset=0 and verify right scroll indicator ▶
	rendered0 := m.View()
	if !strings.Contains(rendered0, "▶") {
		t.Errorf("expected right scroll indicator '▶' in rendered view when more columns exist to the right")
	}
}

func TestModel_ViewRendering(t *testing.T) {
	m, _, _ := setupTestModel()

	// Multi-column view test at width 140, height 35
	m.width = 140
	m.height = 35
	renderedMulti := m.View()
	if len(renderedMulti) == 0 {
		t.Fatal("expected non-empty rendered view at 140")
	}
	if lipgloss.Height(renderedMulti) > 35 {
		t.Errorf("expected view height <= 35, got %d", lipgloss.Height(renderedMulti))
	}

	// Multi-column view test at width 80, height 24 with 10 cards added to column 0
	for i := 4; i <= 15; i++ {
		m.board.Items = append(m.board.Items, github.ProjectItem{
			ID:             "item_extra",
			Number:         i * 10,
			Title:          "Task with a long title that wraps to multiple lines in a column",
			StatusOptionID: "col_todo",
			HoursSpent:     1.0,
		})
	}

	m.width = 80
	m.height = 24
	rendered24 := m.View()
	if len(rendered24) == 0 {
		t.Fatal("expected non-empty rendered view at 80x24")
	}
	// At height 24, view height MUST NOT exceed 24 (preventing top cutoff in alt screen)
	if h := lipgloss.Height(rendered24); h > 24 {
		t.Errorf("view height %d exceeds terminal height 24", h)
	}
	// All 3 columns must be rendered side-by-side
	if !strings.Contains(rendered24, "Todo") || !strings.Contains(rendered24, "In Progress") || !strings.Contains(rendered24, "Done") {
		t.Errorf("expected all 3 columns visible at width 80, view: %s", rendered24)
	}

	// Ultra-narrow viewport (tabbed mode fallback at width < 50)
	m.width = 40
	m.height = 20
	renderedNarrow := m.View()
	if len(renderedNarrow) == 0 {
		t.Fatal("expected non-empty rendered tabbed view at 40")
	}
	if h := lipgloss.Height(renderedNarrow); h > 20 {
		t.Errorf("narrow view height %d exceeds terminal height 20", h)
	}
	if !strings.Contains(renderedNarrow, "[1: Todo") {
		t.Errorf("expected numeric tab header in ultra-narrow mode, view: %s", renderedNarrow)
	}
}


func TestModel_AdditionalKeysAndMoveMode(t *testing.T) {
	m, _, _ := setupTestModel()

	// Press 'm' to enter move mode
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = m2.(Model)
	if !m.movingCard {
		t.Error("expected movingCard == true after 'm'")
	}

	// Press 'l' while in move mode -> drops card to next column and exits move mode
	m3, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = m3.(Model)
	if m.movingCard {
		t.Error("expected movingCard == false after drop")
	}
	if m.activeColumn != 1 {
		t.Errorf("expected card moved to col 1, got %d", m.activeColumn)
	}
	if cmd != nil {
		_ = cmd()
	}

	// Enter move mode again and press 'esc' to cancel
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = m4.(Model)
	if !m.movingCard {
		t.Error("expected movingCard == true")
	}
	m5, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = m5.(Model)
	if m.movingCard {
		t.Error("expected movingCard == false after Esc")
	}

	// Jump directly to column 3 ('3')
	m6, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = m6.(Model)
	if m.activeColumn != 2 {
		t.Errorf("expected active column 2 (3rd col), got %d", m.activeColumn)
	}

	// Jump top 'g' and bottom 'G' on column 1 (which now has item_1)
	m7, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = m7.(Model)
	m8, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = m8.(Model)
	col1Items := m.ItemsInColumn(m.board.Columns[1].ID)
	if m.activeCard[1] != len(col1Items)-1 {
		t.Errorf("expected active card %d after 'G', got %d", len(col1Items)-1, m.activeCard[1])
	}
	m9, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = m9.(Model)
	if m.activeCard[1] != 0 {
		t.Errorf("expected active card 0 after 'g', got %d", m.activeCard[1])
	}

	// Test Tab and Shift+Tab (moves from col 1 -> 2 -> 1)
	m10, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = m10.(Model)
	if m.activeColumn != 2 {
		t.Errorf("expected active column 2 after tab, got %d", m.activeColumn)
	}
	m11, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = m11.(Model)
	if m.activeColumn != 1 {
		t.Errorf("expected active column 1 after shift+tab, got %d", m.activeColumn)
	}
}

func TestModel_ErrorRollback(t *testing.T) {
	m, _, _ := setupTestModel()

	// Simulate CardMovedMsg error
	msg := CardMovedMsg{
		ItemID:         "item_1",
		SourceOptionID: "col_todo",
		TargetOptionID: "col_prog",
		Err:            errors.New("network failure"),
	}

	m2, _ := m.Update(msg)
	m = m2.(Model)

	if m.toast == "" {
		t.Error("expected error toast message to be set")
	}

	// Verify rollback restored status
	for _, it := range m.board.Items {
		if it.ID == "item_1" && it.StatusOptionID != "col_todo" {
			t.Errorf("expected item_1 rolled back to col_todo, got %s", it.StatusOptionID)
		}
	}
}

func TestStyles_Icons(t *testing.T) {
	stylesWithIcons := NewStyles("storm", true)
	if stylesWithIcons.Icon("timer") != "󱎫 " {
		t.Errorf("unexpected timer icon: %s", stylesWithIcons.Icon("timer"))
	}
	if stylesWithIcons.Icon("milestone") != " " {
		t.Errorf("unexpected milestone icon: %s", stylesWithIcons.Icon("milestone"))
	}

	stylesNoIcons := NewStyles("night", false)
	if stylesNoIcons.Icon("timer") != "[TIMER]" {
		t.Errorf("expected fallback [TIMER], got %s", stylesNoIcons.Icon("timer"))
	}
	if stylesNoIcons.Icon("milestone") != "[M]" {
		t.Errorf("expected fallback [M], got %s", stylesNoIcons.Icon("milestone"))
	}
}
