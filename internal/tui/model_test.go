package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/ejablonski/gh-terrorizer/internal/config"
	"github.com/ejablonski/gh-terrorizer/internal/github"
	"github.com/mattn/go-runewidth"
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
	if stylesWithIcons.Icon("milestone") != " " {
		t.Errorf("unexpected milestone icon: %s", stylesWithIcons.Icon("milestone"))
	}
	if stylesWithIcons.Icon("type_circle") != "● " {
		t.Errorf("unexpected type_circle icon: %s", stylesWithIcons.Icon("type_circle"))
	}

	stylesNoIcons := NewStyles("night", false)
	if stylesNoIcons.Icon("timer") != "[TIMER]" {
		t.Errorf("expected fallback [TIMER], got %s", stylesNoIcons.Icon("timer"))
	}
	if stylesNoIcons.Icon("milestone") != "[M]" {
		t.Errorf("expected fallback [M], got %s", stylesNoIcons.Icon("milestone"))
	}
	if stylesNoIcons.Icon("type_circle") != "* " {
		t.Errorf("expected fallback '* ', got %s", stylesNoIcons.Icon("type_circle"))
	}
}

func TestRealisticBoardOverflow(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DefaultOwner = "ejablonski"
	cfg.DefaultRepo = "gh-terrorizer"
	cfg.ProjectNumber = 1

	model := NewModel(nil, nil, cfg)

	var items []github.ProjectItem
	for i := 1; i <= 15; i++ {
		items = append(items, github.ProjectItem{
			ID:             fmt.Sprintf("item_%d", i),
			Number:         i,
			Title:          fmt.Sprintf("Task number %d with some relatively long description title", i),
			Body:           "Some long body text describing the task in great detail.\nMultiple lines.\nAnd more.",
			StatusOptionID: "col_todo",
			HoursSpent:     float64(i) * 0.5,
		})
	}
	for i := 16; i <= 20; i++ {
		items = append(items, github.ProjectItem{
			ID:             fmt.Sprintf("item_%d", i),
			Number:         i,
			Title:          fmt.Sprintf("Task in progress %d", i),
			StatusOptionID: "col_prog",
			HoursSpent:     1.0,
		})
	}

	board := &github.ProjectBoard{
		ID:            "PVT_1",
		Title:         "Roadmap & Sprint Board",
		StatusFieldID: "status",
		HoursFieldID:  "hours",
		Columns: []github.Column{
			{ID: "col_todo", Name: "Todo"},
			{ID: "col_prog", Name: "In Progress"},
			{ID: "col_done", Name: "Done"},
		},
		Items: items,
	}

	m2, _ := model.Update(BoardFetchedMsg{Board: board})
	model = m2.(Model)

	for _, dim := range []struct{ w, h int }{
		{80, 24},
		{100, 30},
		{120, 40},
		{140, 45},
		{60, 20},
		{200, 60},
	} {
		mSized, _ := model.Update(tea.WindowSizeMsg{Width: dim.w, Height: dim.h})
		m := mSized.(Model)
		top := m.renderTopBar(dim.w)
		bottom := m.renderBottomBar(dim.w)
		hMain := m.mainHeight()

		view := m.View()
		lines := strings.Split(view, "\n")
		t.Logf("dim: %dx%d, topH: %d, bottomH: %d, hMain: %d -> View lines: %d", dim.w, dim.h, lipgloss.Height(top), lipgloss.Height(bottom), hMain, len(lines))
		if len(lines) != dim.h {
			t.Errorf("dim %dx%d: expected %d lines, got %d", dim.w, dim.h, dim.h, len(lines))
		}
	}
}

func TestWorkstationDynamicGeometry(t *testing.T) {
	m, _, _ := setupTestModel()

	dimensions := []struct {
		width  int
		height int
	}{
		{80, 24},
		{120, 40},
		{160, 50},
		{70, 20},
	}

	for _, d := range dimensions {
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: d.width, Height: d.height})
		model := mUpdated.(Model)
		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) != d.height {
			t.Errorf("dim %dx%d: expected %d lines, got %d", d.width, d.height, d.height, len(lines))
		}

		// Also verify in inspector modal mode
		mInspector, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
		modelInspector := mInspector.(Model)
		if modelInspector.viewMode != ViewModeInspector {
			t.Fatalf("expected ViewModeInspector, got %v", modelInspector.viewMode)
		}
		viewModal := modelInspector.View()
		modalLines := strings.Split(viewModal, "\n")
		if len(modalLines) != d.height {
			t.Errorf("modal dim %dx%d: expected %d lines, got %d", d.width, d.height, d.height, len(modalLines))
		}
	}
}

func TestInspectorModalOpenAndClose(t *testing.T) {
	m, _, _ := setupTestModel()

	// Enter opens modal
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.viewMode != ViewModeInspector {
		t.Fatalf("expected ViewModeInspector on Enter, got %v", model.viewMode)
	}
	if model.SelectedItem() == nil {
		t.Fatal("expected selected item != nil in inspector")
	}

	// Esc closes modal
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m3.(Model)
	if model.viewMode != ViewModeKanban {
		t.Fatalf("expected ViewModeKanban on Esc, got %v", model.viewMode)
	}

	// 'i' key also opens modal
	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	model = m4.(Model)
	if model.viewMode != ViewModeInspector {
		t.Fatalf("expected ViewModeInspector on 'i', got %v", model.viewMode)
	}

	// 'q' in modal closes modal without quitting application
	m5, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = m5.(Model)
	if model.viewMode != ViewModeKanban {
		t.Fatalf("expected ViewModeKanban on 'q' in modal, got %v", model.viewMode)
	}
	if cmd != nil {
		t.Error("expected cmd == nil on 'q' modal close (should not quit)")
	}
}

func TestInspectorEmptyCardSelection(t *testing.T) {
	m, _, _ := setupTestModel()
	// Set board to have no items in column
	m.board.Items = nil

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.viewMode != ViewModeKanban {
		t.Errorf("expected ViewModeKanban when no card selected, got %v", model.viewMode)
	}
	if model.toast != "No card selected" {
		t.Errorf("expected toast 'No card selected', got %q", model.toast)
	}
}

func TestInspectorCtrlCQuit(t *testing.T) {
	m, _, _ := setupTestModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.viewMode != ViewModeInspector {
		t.Fatalf("expected ViewModeInspector, got %v", model.viewMode)
	}

	// Global ctrl+c quits app
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestInspectorModalKeyDelegation(t *testing.T) {
	m, _, _ := setupTestModel()
	m.board.Items[0].Body = "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8\nLine 9\nLine 10"

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	initialYOffset := model.inspectorViewport.YOffset
	initialCard := model.activeCard[model.activeColumn]

	// Send 'j' down scroll
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = m3.(Model)

	// activeCard cursor should NOT have moved
	if model.activeCard[model.activeColumn] != initialCard {
		t.Errorf("expected activeCard unchanged, was %d now %d", initialCard, model.activeCard[model.activeColumn])
	}
	_ = initialYOffset

	// 'h' and 'l' should be ignored in modal mode
	colBefore := model.activeColumn
	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	model = m4.(Model)
	if model.activeColumn != colBefore {
		t.Errorf("expected activeColumn unchanged on 'l' in modal, got %d", model.activeColumn)
	}
}

func TestInspectorResizeReflow(t *testing.T) {
	m, _, _ := setupTestModel()
	m.board.Items[0].Body = "Long text description that should be wrapped nicely when resized."

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	// Dispatch narrower WindowSizeMsg
	m3, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	model = m3.(Model)

	expectedWModal := max(16, min(58, int(float64(60)*0.85)))
	expectedWViewport := max(10, expectedWModal-4)

	if model.inspectorViewport.Width != expectedWViewport {
		t.Errorf("expected viewport width %d, got %d", expectedWViewport, model.inspectorViewport.Width)
	}
}

func TestInspectorEmptyBodyNotice(t *testing.T) {
	m, _, _ := setupTestModel()
	m.board.Items[0].Body = ""

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	view := model.inspectorViewport.View()
	if !strings.Contains(view, "No description provided.") {
		t.Errorf("expected 'No description provided.' in empty viewport view, got %q", view)
	}
}

func TestInspectorRemoteCardDeleted(t *testing.T) {
	m, _, _ := setupTestModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.viewMode != ViewModeInspector {
		t.Fatalf("expected ViewModeInspector, got %v", model.viewMode)
	}

	// Remote board fetch returns items WITHOUT item_1
	updatedBoard := &github.ProjectBoard{
		ID:            "PVT_test",
		Title:         "Test Project Board",
		StatusFieldID: "status_field_id",
		HoursFieldID:  "hours_field_id",
		Columns:       m.board.Columns,
		Items: []github.ProjectItem{
			{ID: "item_2", StatusOptionID: "col_todo"},
			{ID: "item_3", StatusOptionID: "col_done"},
		},
	}

	m3, _ := model.Update(BoardFetchedMsg{Board: updatedBoard})
	model = m3.(Model)

	if model.viewMode != ViewModeKanban {
		t.Errorf("expected ViewModeKanban when active issue deleted remotely, got %v", model.viewMode)
	}
	if model.toast != "Issue no longer exists" {
		t.Errorf("expected toast 'Issue no longer exists', got %q", model.toast)
	}
}

func TestOnDemandSyncKey(t *testing.T) {
	m, _, _ := setupTestModel()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model := m2.(Model)

	if !model.syncInProgress {
		t.Error("expected syncInProgress == true after 'r'")
	}
	if model.loading {
		t.Error("expected loading == false during ambient 'r' sync")
	}
	if cmd == nil {
		t.Fatal("expected fetchBoardCmd returned")
	}

	// Dispatch BoardFetchedMsg
	m3, _ := model.Update(BoardFetchedMsg{Board: m.board})
	model = m3.(Model)
	if model.syncInProgress {
		t.Error("expected syncInProgress == false after BoardFetchedMsg")
	}
	if model.lastSyncTime.IsZero() {
		t.Error("expected lastSyncTime to be set")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	if formatRelativeTime(time.Time{}) != "󰄬 Initialized" {
		t.Errorf("unexpected zero time: %s", formatRelativeTime(time.Time{}))
	}
	if formatRelativeTime(time.Now()) != "󰄬 Synced just now" {
		t.Errorf("unexpected just now: %s", formatRelativeTime(time.Now()))
	}
	fiveAgo := time.Now().Add(-5 * time.Minute)
	if formatRelativeTime(fiveAgo) != "󰄬 Synced 5m ago" {
		t.Errorf("unexpected 5m ago: %s", formatRelativeTime(fiveAgo))
	}
}

func TestRenderBottomBarVariations(t *testing.T) {
	m, _, _ := setupTestModel()

	// 1. Moving card hints
	m.movingCard = true
	bar := m.renderBottomBar(100)
	if !strings.Contains(bar, "move card to adjacent column") {
		t.Errorf("expected moving card hints, got %q", bar)
	}

	// 2. Inspector hints wide vs compact
	m.movingCard = false
	m.viewMode = ViewModeInspector
	barWide := m.renderBottomBar(100)
	if !strings.Contains(barWide, "half page") {
		t.Errorf("expected wide inspector hints, got %q", barWide)
	}

	barCompact := m.renderBottomBar(80)
	if !strings.Contains(barCompact, "j/k: scroll • e: edit • Esc/q: back") {
		t.Errorf("expected compact inspector hints, got %q", barCompact)
	}

	// 3. Error toast
	m.err = errors.New("sample error")
	barErr := m.renderBottomBar(120)
	if !strings.Contains(barErr, "sample error") {
		t.Errorf("expected error in bottom bar, got %q", barErr)
	}

	// 4. Success toast
	m.err = nil
	m.toast = "Operation succeeded"
	barToast := m.renderBottomBar(120)
	if !strings.Contains(barToast, "Operation succeeded") {
		t.Errorf("expected toast in bottom bar, got %q", barToast)
	}

	// 5. Toast with hints overflow causing truncation
	barTrunc := m.renderBottomBar(40)
	if !strings.Contains(barTrunc, "Operation succeeded") {
		t.Errorf("expected toast preserved during truncation, got %q", barTrunc)
	}

	// 6. No toast, narrow width truncation
	m.toast = ""
	barNoToastNarrow := m.renderBottomBar(20)
	if lipgloss.Width(barNoToastNarrow) > 20 {
		t.Errorf("expected width <= 20, got %d", lipgloss.Width(barNoToastNarrow))
	}
}

func TestRenderTopBarVariations(t *testing.T) {
	m, _, _ := setupTestModel()

	// 1. Full width with idle timer and initialized sync
	top := m.renderTopBar(120)
	if !strings.Contains(top, "gh-terrorizer") || !strings.Contains(top, "Idle") {
		t.Errorf("expected repo and Idle in top bar, got %q", top)
	}

	// 2. Narrow width < 85 truncates owner prefix
	topNarrow := m.renderTopBar(80)
	if strings.Contains(topNarrow, "testowner/") {
		t.Errorf("expected owner prefix omitted in compact top bar, got %q", topNarrow)
	}

	// 3. Sync in progress
	m.syncInProgress = true
	topSync := m.renderTopBar(100)
	if !strings.Contains(topSync, "Syncing...") {
		t.Errorf("expected 'Syncing...' in top bar, got %q", topSync)
	}

	// 4. Active timer
	m.syncInProgress = false
	m.activeTimerStart = time.Now().Add(-10 * time.Second)
	m.activeTimerTag = "gh-10"
	topTimer := m.renderTopBar(100)
	if !strings.Contains(topTimer, "gh-10") {
		t.Errorf("expected 'gh-10' tag in top bar, got %q", topTimer)
	}
}

func TestInspectorRemoteCardRelocated(t *testing.T) {
	m, _, _ := setupTestModel()
	// Open inspector on item_1 (in col_todo, colIdx 0)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.viewMode != ViewModeInspector {
		t.Fatalf("expected ViewModeInspector, got %v", model.viewMode)
	}

	// Remote board fetch returns item_1 now moved to col_done (colIdx 2) with updated body
	updatedBoard := &github.ProjectBoard{
		ID:            "PVT_test",
		Title:         "Test Project Board",
		StatusFieldID: "status_field_id",
		HoursFieldID:  "hours_field_id",
		Columns:       m.board.Columns,
		Items: []github.ProjectItem{
			{
				ID:             "item_1",
				Number:         10,
				Title:          "First Task",
				Body:           "Updated remote description.",
				StatusOptionID: "col_done",
				HoursSpent:     2.0,
			},
			{ID: "item_2", StatusOptionID: "col_todo"},
			{ID: "item_3", StatusOptionID: "col_done"},
		},
	}

	m3, _ := model.Update(BoardFetchedMsg{Board: updatedBoard})
	model = m3.(Model)

	if model.viewMode != ViewModeInspector {
		t.Errorf("expected ViewModeInspector preserved on relocated card, got %v", model.viewMode)
	}
	if model.activeColumn != 2 {
		t.Errorf("expected activeColumn updated to 2, got %d", model.activeColumn)
	}
	if !strings.Contains(model.inspectorViewport.View(), "Updated remote description.") {
		t.Errorf("expected viewport updated with new body, got %q", model.inspectorViewport.View())
	}
}

func TestBoardFetchedMsgError(t *testing.T) {
	m, _, _ := setupTestModel()
	m.syncInProgress = true

	m2, _ := m.Update(BoardFetchedMsg{Err: errors.New("graphql error")})
	model := m2.(Model)

	if model.syncInProgress {
		t.Error("expected syncInProgress == false on error")
	}
	if model.err == nil || !strings.Contains(model.toast, "graphql error") {
		t.Errorf("expected toast error message, got %q", model.toast)
	}
}

func TestEditorFinishedMsg(t *testing.T) {
	m, _, _ := setupTestModel()
	// Open inspector
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	// Case 1: Editor error
	mErr, _ := model.Update(EditorFinishedMsg{Err: errors.New("editor failure")})
	modelErr := mErr.(Model)
	if !strings.Contains(modelErr.toast, "editor failure") {
		t.Errorf("expected editor error toast, got %q", modelErr.toast)
	}

	// Case 2: Successful edit with temp file
	tmpFile, err := os.CreateTemp("", "gh-terrorizer-test-*.md")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	newBody := "Body edited via external editor."
	_, _ = tmpFile.WriteString(newBody)
	_ = tmpFile.Close()

	mOk, _ := model.Update(EditorFinishedMsg{
		ItemID:   "item_1",
		FilePath: tmpPath,
	})
	modelOk := mOk.(Model)

	if modelOk.toast != "Issue body updated" {
		t.Errorf("expected toast 'Issue body updated', got %q", modelOk.toast)
	}
	if modelOk.board.Items[0].Body != newBody {
		t.Errorf("expected board item body updated, got %q", modelOk.board.Items[0].Body)
	}
	if !strings.Contains(modelOk.inspectorViewport.View(), newBody) {
		t.Errorf("expected viewport content updated, got %q", modelOk.inspectorViewport.View())
	}
	// Verify temp file was cleaned up
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("expected temp file %s to be deleted", tmpPath)
	}
}

func TestInspectorEditorKey(t *testing.T) {
	m, _, _ := setupTestModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	m3, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_ = m3.(Model)
	if cmd == nil {
		t.Error("expected launchEditor command returned on 'e' key")
	}
}

func TestInspectorModalRenderDetails(t *testing.T) {
	m, _, _ := setupTestModel()
	// Nil selection rendering returns empty
	emptyM := m
	emptyM.board = nil
	if emptyM.renderInspectorModal(80, 24) != "" {
		t.Error("expected empty string when no item selected")
	}

	// Item with closed state and milestone
	m.board.Items[0].State = "CLOSED"
	m.board.Items[0].Milestone = &github.Milestone{Title: "v1.2.3"}
	m.board.Items[0].Body = "Line 1\nLine 2\nLine 3"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	rendered := model.renderInspectorModal(80, 24)
	if !strings.Contains(rendered, "CLOSED") {
		t.Errorf("expected 'CLOSED' badge in modal, got %q", rendered)
	}
	if !strings.Contains(rendered, "v1.2.3") {
		t.Errorf("expected milestone in modal, got %q", rendered)
	}
}

func TestModelTickMsg(t *testing.T) {
	m, _, _ := setupTestModel()
	m2, cmd := m.Update(TickMsg(time.Now()))
	_ = m2.(Model)
	if cmd == nil {
		t.Error("expected tickCmd returned on TickMsg")
	}
}

func TestCardMovedAndHoursUpdatedMsgs(t *testing.T) {
	m, _, _ := setupTestModel()

	// CardMovedMsg error rollback
	m.board.Items[0].StatusOptionID = "col_done"
	mErr, _ := m.Update(CardMovedMsg{
		Err:            errors.New("move failed"),
		ItemID:         "item_1",
		SourceOptionID: "col_todo",
	})
	modelErr := mErr.(Model)
	if modelErr.board.Items[0].StatusOptionID != "col_todo" {
		t.Errorf("expected rollback to col_todo, got %s", modelErr.board.Items[0].StatusOptionID)
	}
	if !strings.Contains(modelErr.toast, "Failed to move card") {
		t.Errorf("expected toast error, got %q", modelErr.toast)
	}

	// HoursUpdatedMsg success and error
	mHoursOk, _ := m.Update(HoursUpdatedMsg{ItemID: "item_1", Hours: 4.5})
	modelHoursOk := mHoursOk.(Model)
	if modelHoursOk.board.Items[0].HoursSpent != 4.5 {
		t.Errorf("expected hours 4.5, got %f", modelHoursOk.board.Items[0].HoursSpent)
	}

	mHoursErr, _ := m.Update(HoursUpdatedMsg{Err: errors.New("timew error")})
	modelHoursErr := mHoursErr.(Model)
	if !strings.Contains(modelHoursErr.toast, "Failed to sync hours") {
		t.Errorf("expected hours error toast, got %q", modelHoursErr.toast)
	}
}

func TestViewHeightClamping(t *testing.T) {
	m, _, _ := setupTestModel()

	// Terminal height smaller than minimum hMain
	mSmall, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 6})
	modelSmall := mSmall.(Model)
	vSmall := modelSmall.View()
	linesSmall := strings.Split(vSmall, "\n")
	if len(linesSmall) != 6 {
		t.Errorf("expected 6 lines on clamped height, got %d", len(linesSmall))
	}

	// Extreme small terminal height
	mTiny, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 1})
	modelTiny := mTiny.(Model)
	vTiny := modelTiny.View()
	linesTiny := strings.Split(vTiny, "\n")
	if len(linesTiny) != 1 {
		t.Errorf("expected 1 line on extreme small height, got %d", len(linesTiny))
	}
}

func TestCard_CollapsibleRow3(t *testing.T) {
	m, _, _ := setupTestModel()

	// Card without milestone and without labels -> 4 lines outer (collapsed)
	cleanItem := github.ProjectItem{
		ID:     "item_clean",
		Number: 101,
		Title:  "Clean task with no metadata",
	}
	renderedClean := m.renderCard(cleanItem, false, 30)
	linesClean := strings.Split(renderedClean, "\n")
	if len(linesClean) != 4 {
		t.Errorf("expected 4 lines for collapsed card, got %d: %q", len(linesClean), renderedClean)
	}

	// Card with milestone -> 5 lines outer (expanded)
	milestoneItem := github.ProjectItem{
		ID:        "item_ms",
		Number:    102,
		Title:     "Task with milestone",
		Milestone: &github.Milestone{Title: "v1.0"},
	}
	renderedMS := m.renderCard(milestoneItem, false, 30)
	linesMS := strings.Split(renderedMS, "\n")
	if len(linesMS) != 5 {
		t.Errorf("expected 5 lines for card with milestone, got %d: %q", len(linesMS), renderedMS)
	}

	// Card with labels -> 5 lines outer (expanded)
	labeledItem := github.ProjectItem{
		ID:     "item_label",
		Number: 103,
		Title:  "Task with label",
		Labels: []github.Label{{Name: "bug", Color: "f7768e"}},
	}
	renderedLabeled := m.renderCard(labeledItem, false, 30)
	linesLabeled := strings.Split(renderedLabeled, "\n")
	if len(linesLabeled) != 5 {
		t.Errorf("expected 5 lines for card with labels, got %d: %q", len(linesLabeled), renderedLabeled)
	}
}

func TestCard_LabelColorResolution(t *testing.T) {
	// Nil config safety
	mNil := Model{styles: NewStyles("storm", true)}
	testItem := github.ProjectItem{
		Labels: []github.Label{{Name: "feature"}},
	}
	color := mNil.resolveLabelColor(testItem)
	if color != string(mNil.styles.Palette.Comment) {
		t.Errorf("expected fallback comment color on nil config, got %s", color)
	}

	// First match wins & case-insensitivity
	cfg := config.DefaultConfig()
	cfg.Theme.IssueTypeLabels = map[string]string{
		"bug":     "#f7768e",
		"feature": "#7aa2f7",
		"task":    "#e0af68",
	}
	m := Model{cfg: cfg, styles: NewStyles("storm", true)}

	multiItem := github.ProjectItem{
		Labels: []github.Label{
			{Name: "FEATURE"},
			{Name: "bug"},
		},
	}
	colorMulti := m.resolveLabelColor(multiItem)
	if colorMulti != "#7aa2f7" {
		t.Errorf("expected first match #7aa2f7 (feature), got %s", colorMulti)
	}

	// Fallback when unmatched
	unmatchedItem := github.ProjectItem{
		Labels: []github.Label{{Name: "documentation"}},
	}
	colorUnmatched := m.resolveLabelColor(unmatchedItem)
	if colorUnmatched != string(m.styles.Palette.Comment) {
		t.Errorf("expected fallback comment color for unmatched label, got %s", colorUnmatched)
	}
}

func TestCard_DomainTypeIcons(t *testing.T) {
	m, _, _ := setupTestModel()

	prItem := github.ProjectItem{
		ID:     "pr_1",
		Number: 10,
		Title:  "Fix memory leak",
		Type:   "PULL_REQUEST",
	}
	renderedPR := m.renderCard(prItem, false, 40)
	if !strings.Contains(renderedPR, m.styles.Icon("pr")) {
		t.Errorf("expected PR card to contain PR icon %q, got: %q", m.styles.Icon("pr"), renderedPR)
	}

	draftItem := github.ProjectItem{
		ID:    "draft_1",
		Title: "Draft idea",
		Type:  "DRAFT_ISSUE",
	}
	renderedDraft := m.renderCard(draftItem, false, 40)
	if !strings.Contains(renderedDraft, m.styles.Icon("draft")) {
		t.Errorf("expected Draft card to contain draft icon %q, got: %q", m.styles.Icon("draft"), renderedDraft)
	}

	issueItem := github.ProjectItem{
		ID:     "issue_1",
		Number: 20,
		Title:  "Standard bug report",
		Type:   "ISSUE",
	}
	renderedIssue := m.renderCard(issueItem, false, 40)
	if !strings.Contains(renderedIssue, m.styles.Icon("issue")) {
		t.Errorf("expected Issue card to contain issue icon %q, got: %q", m.styles.Icon("issue"), renderedIssue)
	}
}

func TestCard_RowClamping(t *testing.T) {
	m, _, _ := setupTestModel()

	// Extremely long labels and title on a narrow card
	overflowItem := github.ProjectItem{
		ID:     "item_overflow",
		Number: 999,
		Title:  "This is a very very very long issue title that should be safely truncated without wrapping onto multiple lines",
		Milestone: &github.Milestone{
			Title: "Extremely long milestone title that exceeds column width easily",
		},
		Labels: []github.Label{
			{Name: "label-alpha-one-very-long"},
			{Name: "label-beta-two-also-long"},
			{Name: "label-gamma-three-very-long"},
		},
		HoursSpent: 123.45,
	}

	// Narrow width: 20 columns
	rendered := m.renderCard(overflowItem, false, 20)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 5 {
		t.Errorf("expected strictly 5 lines for expanded card under narrow width, got %d: %q", len(lines), rendered)
	}

	// Even narrower width: 10 columns
	renderedNarrow := m.renderCard(overflowItem, false, 10)
	linesNarrow := strings.Split(renderedNarrow, "\n")
	if len(linesNarrow) != 5 {
		t.Errorf("expected strictly 5 lines under narrow width, got %d: %q", len(linesNarrow), renderedNarrow)
	}
}

func TestModal_ThickBorderAlignment(t *testing.T) {
	m, _, _ := setupTestModel()
	m.viewMode = ViewModeInspector
	m.board.Items[0].Title = "A very long issue title that tests whether modal top border is truncated properly"

	rendered := m.renderInspectorModal(80, 24)

	var modalLines []string
	inModal := false
	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(ansi.Strip(line))
		if strings.HasPrefix(trimmed, "┏") {
			inModal = true
		}
		if inModal {
			modalLines = append(modalLines, trimmed)
			if strings.HasPrefix(trimmed, "┗") {
				inModal = false
			}
		}
	}

	if len(modalLines) == 0 {
		t.Fatal("expected modal box lines, found none")
	}

	wModal := max(16, min(80-2, int(float64(80)*0.85)))

	// Check top border
	top := modalLines[0]
	if !strings.HasPrefix(top, "┏") || !strings.HasSuffix(top, "┓") {
		t.Errorf("expected top line to start with ┏ and end with ┓, got %q", top)
	}

	// Check bottom border
	bottom := modalLines[len(modalLines)-1]
	if !strings.HasPrefix(bottom, "┗") || !strings.HasSuffix(bottom, "┛") {
		t.Errorf("expected bottom line to start with ┗ and end with ┛, got %q", bottom)
	}

	// Check all modal lines have exact width wModal and consistent vertical borders
	for i, line := range modalLines {
		cellWidth := runewidth.StringWidth(line)
		if cellWidth != wModal {
			t.Errorf("line %d has width %d (expected %d): %q", i, cellWidth, wModal, line)
		}
		if i > 0 && i < len(modalLines)-1 {
			if !strings.HasPrefix(line, "┃") || !strings.HasSuffix(line, "┃") {
				t.Errorf("line %d expected vertical borders ┃...┃, got %q", i, line)
			}
		}
	}
}
