package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ejablonski/gh-terrorizer/internal/config"
	"github.com/ejablonski/gh-terrorizer/internal/github"
	"github.com/ejablonski/gh-terrorizer/internal/timew"
)

// GitHubClientInterface abstracts GitHub operations for testability.
type GitHubClientInterface interface {
	FetchBoard(ctx context.Context, owner string, number int, hoursFieldName string) (*github.ProjectBoard, error)
	MoveCard(ctx context.Context, projectID, itemID, fieldID, optionID string) error
	UpdateHours(ctx context.Context, projectID, itemID, fieldID string, totalHours float64) error
}

// TimeTrackerInterface abstracts Timewarrior tracking for testability.
type TimeTrackerInterface interface {
	Start(ctx context.Context, tag string, extra ...string) error
	Stop(ctx context.Context) (int, error)
	IsActive(ctx context.Context) (bool, error)
	GetActiveTag(ctx context.Context) (string, error)
}

// TickMsg triggers UI timer updates and status bar refreshes.
type TickMsg time.Time

// BoardFetchedMsg delivers a freshly fetched project board.
type BoardFetchedMsg struct {
	Board *github.ProjectBoard
	Err   error
}

// CardMovedMsg indicates result of an asynchronous card movement.
type CardMovedMsg struct {
	ItemID         string
	SourceOptionID string
	TargetOptionID string
	Err            error
}

// HoursUpdatedMsg indicates result of an asynchronous hours update.
type HoursUpdatedMsg struct {
	ItemID string
	Hours  float64
	Err    error
}

// EditorFinishedMsg is returned when external editor process terminates.
type EditorFinishedMsg struct {
	ItemID   string
	FilePath string
	Err      error
}

// ViewMode represents the primary view state of the application.
type ViewMode int

const (
	ViewModeKanban ViewMode = iota
	ViewModeInspector
)

// Model is the root Bubble Tea application model.
type Model struct {
	client  GitHubClientInterface
	tracker TimeTrackerInterface
	cfg     *config.Config
	styles  Styles

	board   *github.ProjectBoard
	loading bool
	spinner spinner.Model

	err   error
	toast string

	activeColumn int
	activeCard   map[int]int // column index -> selected card index
	colOffset    int         // horizontal viewport starting column index
	movingCard   bool

	activeTimerItemID string
	activeTimerTag    string
	activeTimerStart  time.Time

	width  int
	height int

	viewMode          ViewMode
	inspectorViewport viewport.Model
	lastSyncTime      time.Time
	syncInProgress    bool
}

// MinColumnWidth defines the minimum comfortable outer width for a Kanban column.
const MinColumnWidth = 25

// VisibleColumnCount returns the number of columns that can fit side-by-side.
func (m Model) VisibleColumnCount() int {
	if m.board == nil || len(m.board.Columns) == 0 {
		return 1
	}
	numCols := len(m.board.Columns)
	k := m.width / MinColumnWidth
	if k < 1 {
		k = 1
	}
	if k > numCols {
		k = numCols
	}
	return k
}

// adjustViewport ensures colOffset keeps activeColumn visible within the horizontal viewport.
func (m *Model) adjustViewport() {
	if m.board == nil || len(m.board.Columns) == 0 {
		m.colOffset = 0
		return
	}
	numCols := len(m.board.Columns)
	visCols := m.VisibleColumnCount()

	if m.activeColumn < m.colOffset {
		m.colOffset = m.activeColumn
	} else if m.activeColumn >= m.colOffset+visCols {
		m.colOffset = m.activeColumn - visCols + 1
	}

	maxOffset := numCols - visCols
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.colOffset > maxOffset {
		m.colOffset = maxOffset
	}
	if m.colOffset < 0 {
		m.colOffset = 0
	}
}

// NewModel creates an initialized Bubble Tea Model.
func NewModel(client GitHubClientInterface, tracker TimeTrackerInterface, cfg *config.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	styles := NewStyles(cfg.Theme.TokyoNightVariant, cfg.Theme.ShowIcons)
	s.Style = lipgloss.NewStyle().Foreground(styles.Palette.Cyan)

	return Model{
		client:            client,
		tracker:           tracker,
		cfg:               cfg,
		styles:            styles,
		loading:           true,
		spinner:           s,
		activeColumn:      0,
		activeCard:        make(map[int]int),
		colOffset:         0,
		width:             120,
		height:            30,
		viewMode:          ViewModeKanban,
		inspectorViewport: viewport.New(0, 0),
		lastSyncTime:      time.Time{},
		syncInProgress:    false,
	}
}

// Init triggers initial board fetching and ticker commands.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchBoardCmd(),
		m.tickCmd(),
	)
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m Model) fetchBoardCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		board, err := m.client.FetchBoard(ctx, m.cfg.DefaultOwner, m.cfg.ProjectNumber, m.cfg.Timewarrior.HoursCustomField)
		return BoardFetchedMsg{Board: board, Err: err}
	}
}

// ItemsInColumn returns all items belonging to the given column option ID.
func (m Model) ItemsInColumn(optionID string) []github.ProjectItem {
	if m.board == nil {
		return nil
	}
	var res []github.ProjectItem
	for _, item := range m.board.Items {
		if item.StatusOptionID == optionID {
			res = append(res, item)
		}
	}
	return res
}

// SelectedItem returns the currently focused project card, if any.
func (m Model) SelectedItem() *github.ProjectItem {
	if m.board == nil || len(m.board.Columns) == 0 {
		return nil
	}
	col := m.board.Columns[m.activeColumn]
	items := m.ItemsInColumn(col.ID)
	if len(items) == 0 {
		return nil
	}
	idx := m.activeCard[m.activeColumn]
	if idx < 0 {
		idx = 0
	}
	if idx >= len(items) {
		idx = len(items) - 1
	}
	return &items[idx]
}

// Update handles message dispatch and state transitions.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustViewport()

		if m.viewMode == ViewModeInspector {
			if item := m.SelectedItem(); item != nil {
				hMain := m.mainHeight()
				wModal := max(16, min(m.width-2, int(float64(m.width)*0.85)))
				hModal := max(7, min(hMain-2, int(float64(hMain)*0.85)))
				wViewport := max(10, wModal-4)
				hViewport := max(2, hModal-5)

				m.inspectorViewport.Width = wViewport
				m.inspectorViewport.Height = hViewport
				if strings.TrimSpace(item.Body) == "" {
					m.inspectorViewport.SetContent(m.styles.EmptyNotice.Render("No description provided."))
				} else {
					wrapped := lipgloss.NewStyle().Width(wViewport).Render(item.Body)
					m.inspectorViewport.SetContent(wrapped)
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case TickMsg:
		cmds = append(cmds, m.tickCmd())

	case BoardFetchedMsg:
		var activeID string
		if it := m.SelectedItem(); it != nil {
			activeID = it.ID
		}

		m.loading = false
		m.syncInProgress = false

		if msg.Err != nil {
			m.err = msg.Err
			m.toast = fmt.Sprintf("Error fetching board: %v", msg.Err)
			return m, nil
		}

		m.board = msg.Board
		m.err = nil
		m.toast = ""
		m.lastSyncTime = time.Now()

		// Clamp active column and card
		if m.board != nil && len(m.board.Columns) > 0 {
			if m.activeColumn >= len(m.board.Columns) {
				m.activeColumn = len(m.board.Columns) - 1
			}
			if m.activeColumn < 0 {
				m.activeColumn = 0
			}
			items := m.ItemsInColumn(m.board.Columns[m.activeColumn].ID)
			if m.activeCard[m.activeColumn] >= len(items) {
				if len(items) > 0 {
					m.activeCard[m.activeColumn] = len(items) - 1
				} else {
					m.activeCard[m.activeColumn] = 0
				}
			}
		}
		m.adjustViewport()

		if m.viewMode == ViewModeInspector && activeID != "" {
			found := false
			if m.board != nil {
				for colIdx, col := range m.board.Columns {
					for itemIdx, item := range m.ItemsInColumn(col.ID) {
						if item.ID == activeID {
							m.activeColumn = colIdx
							m.activeCard[colIdx] = itemIdx
							found = true

							hMain := m.mainHeight()
							wModal := max(16, min(m.width-2, int(float64(m.width)*0.85)))
							hModal := max(7, min(hMain-2, int(float64(hMain)*0.85)))
							wViewport := max(10, wModal-4)
							hViewport := max(2, hModal-5)

							m.inspectorViewport.Width = wViewport
							m.inspectorViewport.Height = hViewport
							if strings.TrimSpace(item.Body) == "" {
								m.inspectorViewport.SetContent(m.styles.EmptyNotice.Render("No description provided."))
							} else {
								wrapped := lipgloss.NewStyle().Width(wViewport).Render(item.Body)
								m.inspectorViewport.SetContent(wrapped)
							}
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				m.viewMode = ViewModeKanban
				m.toast = "Issue no longer exists"
			}
		}

	case CardMovedMsg:
		if msg.Err != nil {
			m.toast = fmt.Sprintf("Failed to move card: %v", msg.Err)
			// Roll back optimistic update
			if m.board != nil {
				for i := range m.board.Items {
					if m.board.Items[i].ID == msg.ItemID {
						m.board.Items[i].StatusOptionID = msg.SourceOptionID
						break
					}
				}
			}
		}

	case HoursUpdatedMsg:
		if msg.Err != nil {
			m.toast = fmt.Sprintf("Failed to sync hours: %v", msg.Err)
		} else {
			m.toast = fmt.Sprintf("Hours updated: %.2fh", msg.Hours)
			if m.board != nil {
				for i := range m.board.Items {
					if m.board.Items[i].ID == msg.ItemID {
						m.board.Items[i].HoursSpent = msg.Hours
						break
					}
				}
			}
		}

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.toast = fmt.Sprintf("Editor exited with error: %v", msg.Err)
		} else {
			// Read updated body from file if present
			if msg.FilePath != "" {
				updatedBody, err := os.ReadFile(msg.FilePath)
				_ = os.Remove(msg.FilePath)
				if err == nil && m.board != nil {
					for i := range m.board.Items {
						if m.board.Items[i].ID == msg.ItemID {
							m.board.Items[i].Body = string(updatedBody)
							if m.viewMode == ViewModeInspector {
								hMain := m.mainHeight()
								wModal := max(16, min(m.width-2, int(float64(m.width)*0.85)))
								hModal := max(7, min(hMain-2, int(float64(hMain)*0.85)))
								wViewport := max(10, wModal-4)
								hViewport := max(2, hModal-5)

								m.inspectorViewport.Width = wViewport
								m.inspectorViewport.Height = hViewport
								if strings.TrimSpace(m.board.Items[i].Body) == "" {
									m.inspectorViewport.SetContent(m.styles.EmptyNotice.Render("No description provided."))
								} else {
									wrapped := lipgloss.NewStyle().Width(wViewport).Render(m.board.Items[i].Body)
									m.inspectorViewport.SetContent(wrapped)
								}
							}
							break
						}
					}
				}
			}
			m.toast = "Issue body updated"
		}

	case tea.KeyMsg:
		// Global keys
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		if m.viewMode == ViewModeInspector {
			switch msg.String() {
			case "esc", "q":
				m.viewMode = ViewModeKanban
				return m, nil
			case "e":
				cmd := m.launchEditor()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			case "j", "down", "k", "up", "d", "u", "g", "G", "pgdown", "pgup", "space", "home", "end":
				var cmd tea.Cmd
				m.inspectorViewport, cmd = m.inspectorViewport.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			default:
				return m, nil
			}
		}

		// ViewModeKanban
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "enter", "i":
			if !m.movingCard {
				item := m.SelectedItem()
				if item == nil {
					m.toast = "No card selected"
					return m, nil
				}
				hMain := m.mainHeight()
				wModal := max(16, min(m.width-2, int(float64(m.width)*0.85)))
				hModal := max(7, min(hMain-2, int(float64(hMain)*0.85)))
				wViewport := max(10, wModal-4)
				hViewport := max(2, hModal-5)

				m.inspectorViewport.Width = wViewport
				m.inspectorViewport.Height = hViewport
				if strings.TrimSpace(item.Body) == "" {
					m.inspectorViewport.SetContent(m.styles.EmptyNotice.Render("No description provided."))
				} else {
					wrapped := lipgloss.NewStyle().Width(wViewport).Render(item.Body)
					m.inspectorViewport.SetContent(wrapped)
				}
				m.inspectorViewport.GotoTop()
				m.viewMode = ViewModeInspector
				return m, nil
			}

		case "r":
			m.syncInProgress = true
			m.toast = "Refreshing board..."
			cmds = append(cmds, m.fetchBoardCmd())

		case "h", "left":
			if m.movingCard {
				cmd := m.moveSelectedCardOffset(-1)
				m.movingCard = false
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				m.shiftColumn(-1)
			}

		case "l", "right":
			if m.movingCard {
				cmd := m.moveSelectedCardOffset(1)
				m.movingCard = false
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			} else {
				m.shiftColumn(1)
			}

		case "j", "down":
			m.shiftCard(1)

		case "k", "up":
			m.shiftCard(-1)

		case "g":
			m.activeCard[m.activeColumn] = 0

		case "G":
			if m.board != nil && len(m.board.Columns) > 0 {
				col := m.board.Columns[m.activeColumn]
				items := m.ItemsInColumn(col.ID)
				if len(items) > 0 {
					m.activeCard[m.activeColumn] = len(items) - 1
				}
			}

		case "H":
			cmd := m.moveSelectedCardOffset(-1)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "L":
			cmd := m.moveSelectedCardOffset(1)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "m":
			m.movingCard = !m.movingCard

		case "esc":
			if m.movingCard {
				m.movingCard = false
			} else if m.toast != "" {
				m.toast = ""
			}

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			colIdx := int(msg.String()[0] - '1')
			if m.board != nil && colIdx < len(m.board.Columns) {
				m.activeColumn = colIdx
				m.adjustViewport()
			}

		case "tab":
			m.shiftColumn(1)

		case "shift+tab":
			m.shiftColumn(-1)

		case "t":
			cmd := m.toggleTimer()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case "e":
			cmd := m.launchEditor()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) shiftColumn(delta int) {
	if m.board == nil || len(m.board.Columns) == 0 {
		return
	}
	numCols := len(m.board.Columns)
	m.activeColumn = (m.activeColumn + delta + numCols) % numCols
	m.adjustViewport()
}

func (m *Model) shiftCard(delta int) {
	if m.board == nil || len(m.board.Columns) == 0 {
		return
	}
	col := m.board.Columns[m.activeColumn]
	items := m.ItemsInColumn(col.ID)
	if len(items) == 0 {
		return
	}
	cur := m.activeCard[m.activeColumn]
	next := cur + delta
	if next < 0 {
		next = 0
	}
	if next >= len(items) {
		next = len(items) - 1
	}
	m.activeCard[m.activeColumn] = next
}

func (m *Model) moveSelectedCardOffset(colDelta int) tea.Cmd {
	item := m.SelectedItem()
	if item == nil || m.board == nil || len(m.board.Columns) == 0 {
		return nil
	}

	targetColIdx := m.activeColumn + colDelta
	if targetColIdx < 0 || targetColIdx >= len(m.board.Columns) {
		return nil
	}

	sourceOptionID := item.StatusOptionID
	targetOptionID := m.board.Columns[targetColIdx].ID

	if sourceOptionID == targetOptionID {
		return nil
	}

	// Optimistic UI update
	for i := range m.board.Items {
		if m.board.Items[i].ID == item.ID {
			m.board.Items[i].StatusOptionID = targetOptionID
			break
		}
	}
	m.activeColumn = targetColIdx
	m.adjustViewport()

	projectID := m.board.ID
	statusFieldID := m.board.StatusFieldID
	itemID := item.ID

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := m.client.MoveCard(ctx, projectID, itemID, statusFieldID, targetOptionID)
		return CardMovedMsg{
			ItemID:         itemID,
			SourceOptionID: sourceOptionID,
			TargetOptionID: targetOptionID,
			Err:            err,
		}
	}
}

func (m *Model) toggleTimer() tea.Cmd {
	item := m.SelectedItem()
	if item == nil || !m.cfg.Timewarrior.Enabled {
		return nil
	}

	ctx := context.Background()

	// Case 1: The current item is actively being tracked -> STOP tracking
	if m.activeTimerItemID == item.ID {
		m.activeTimerItemID = ""
		m.activeTimerTag = ""

		return func() tea.Msg {
			elapsedSeconds, err := m.tracker.Stop(ctx)
			if err != nil {
				return HoursUpdatedMsg{ItemID: item.ID, Err: err}
			}

			deltaHours := timew.CalculateHours(elapsedSeconds, m.cfg.Timewarrior.RoundingPrecision)
			newTotal := item.HoursSpent + deltaHours

			err = m.client.UpdateHours(ctx, m.board.ID, item.ID, m.board.HoursFieldID, newTotal)
			return HoursUpdatedMsg{
				ItemID: item.ID,
				Hours:  newTotal,
				Err:    err,
			}
		}
	}

	// Case 2: A different item was being tracked -> STOP previous, START current
	var stopCmd tea.Cmd
	if m.activeTimerItemID != "" {
		prevItemID := m.activeTimerItemID
		var prevItem *github.ProjectItem
		for i := range m.board.Items {
			if m.board.Items[i].ID == prevItemID {
				prevItem = &m.board.Items[i]
				break
			}
		}

		stopCmd = func() tea.Msg {
			elapsedSeconds, err := m.tracker.Stop(ctx)
			if err != nil {
				return HoursUpdatedMsg{ItemID: prevItemID, Err: err}
			}
			if prevItem != nil {
				delta := timew.CalculateHours(elapsedSeconds, m.cfg.Timewarrior.RoundingPrecision)
				newTotal := prevItem.HoursSpent + delta
				prevItem.HoursSpent = newTotal
				_ = m.client.UpdateHours(ctx, m.board.ID, prevItemID, m.board.HoursFieldID, newTotal)
			}
			return nil
		}
	}

	// Formulate tag
	tag := fmt.Sprintf("%s%d", m.cfg.Timewarrior.TagPrefix, item.Number)
	if item.Number == 0 {
		tag = fmt.Sprintf("%s%s", m.cfg.Timewarrior.TagPrefix, item.ID)
	}

	m.activeTimerItemID = item.ID
	m.activeTimerTag = tag
	m.activeTimerStart = time.Now()

	startCmd := func() tea.Msg {
		_ = m.tracker.Start(ctx, tag, ":gh")
		return nil
	}

	if stopCmd != nil {
		return tea.Sequence(stopCmd, startCmd)
	}
	return startCmd
}

func (m *Model) launchEditor() tea.Cmd {
	item := m.SelectedItem()
	if item == nil {
		return nil
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "nano"
	}

	// Create temp file populated with current body
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("gh-terrorizer-%s.md", item.ID))
	_ = os.WriteFile(tmpFile, []byte(item.Body), 0600)

	cmd := exec.Command(editor, tmpFile)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return EditorFinishedMsg{
			ItemID:   item.ID,
			FilePath: tmpFile,
			Err:      err,
		}
	})
}

// mainHeight calculates the dynamic height allocated to the middle tier.
func (m Model) mainHeight() int {
	topH := lipgloss.Height(m.renderTopBar(m.width))
	bottomH := lipgloss.Height(m.renderBottomBar(m.width))
	h := m.height - topH - bottomH
	if h < 8 {
		return 8
	}
	return h
}

// View renders the TUI layout.
func (m Model) View() string {
	if m.loading && m.board == nil {
		return fmt.Sprintf("\n  %s Fetching GitHub Projects v2 Kanban board...\n", m.spinner.View())
	}

	if m.err != nil && m.board == nil {
		return fmt.Sprintf("\n  %s %s\n\n  Press 'r' to retry, 'q' to quit.\n",
			m.styles.ToastError.Render("ERROR:"),
			m.err.Error(),
		)
	}

	topBar := m.renderTopBar(m.width)
	bottomBar := m.renderBottomBar(m.width)
	hMain := m.mainHeight()

	var content string
	if m.viewMode == ViewModeInspector {
		content = m.renderInspectorModal(m.width, hMain)
	} else {
		content = m.renderKanbanBoard(m.width, hMain)
	}

	view := lipgloss.JoinVertical(lipgloss.Left, topBar, content, bottomBar)
	lines := strings.Split(view, "\n")
	if len(lines) > m.height && m.height > 0 {
		topH := lipgloss.Height(topBar)
		bottomH := lipgloss.Height(bottomBar)
		availMiddle := m.height - topH - bottomH
		if availMiddle >= 0 {
			var clampedLines []string
			clampedLines = append(clampedLines, lines[:topH]...)
			middleStart := topH
			middleEnd := middleStart + availMiddle
			if middleEnd > len(lines)-bottomH {
				middleEnd = len(lines) - bottomH
			}
			clampedLines = append(clampedLines, lines[middleStart:middleEnd]...)
			clampedLines = append(clampedLines, lines[len(lines)-bottomH:]...)
			return strings.Join(clampedLines, "\n")
		}
		return strings.Join(lines[:m.height], "\n")
	}
	return view
}
