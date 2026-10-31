package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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
		client:       client,
		tracker:      tracker,
		cfg:          cfg,
		styles:       styles,
		loading:      true,
		spinner:      s,
		activeColumn: 0,
		activeCard:   make(map[int]int),
		colOffset:    0,
		width:        120,
		height:       30,
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
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			m.toast = fmt.Sprintf("Error fetching board: %v", msg.Err)
		} else {
			m.board = msg.Board
			m.err = nil
			m.toast = ""
			// Clamp active column
			if m.activeColumn >= len(m.board.Columns) {
				m.activeColumn = 0
			}
			m.adjustViewport()
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
							break
						}
					}
				}
			}
			m.toast = "Issue body updated"
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			m.loading = true
			m.toast = "Refreshing board..."
			cmds = append(cmds, m.spinner.Tick, m.fetchBoardCmd())

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

// View renders the TUI layout.
func (m Model) View() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Fetching GitHub Projects v2 Kanban board...\n", m.spinner.View())
	}

	if m.err != nil && m.board == nil {
		return fmt.Sprintf("\n  %s %s\n\n  Press 'r' to retry, 'q' to quit.\n",
			m.styles.ToastError.Render("ERROR:"),
			m.err.Error(),
		)
	}

	if m.board == nil || len(m.board.Columns) == 0 {
		return m.styles.EmptyNotice.Render("No status columns found on this GitHub Project board.\nPress 'q' to quit.")
	}

	header := m.renderHeader()
	statusBar := m.renderStatusBar()

	// Compute exact column height so total view height == m.height
	colHeight := m.height - lipgloss.Height(header) - lipgloss.Height(statusBar)
	if colHeight < 8 {
		colHeight = 8
	}

	numCols := len(m.board.Columns)
	visCols := m.VisibleColumnCount()

	var content string
	// Render multi-column grid side-by-side whenever 2 or more columns fit,
	// or when the board itself only has 1 column.
	// Only degrade to single-column tabbed view on ultra-narrow terminals (W < 50 with numCols > 1).
	if visCols > 1 || numCols == 1 {
		m.adjustViewport()
		var renderedCols []string

		// Distribute m.width across the visCols columns without gaps or overflow
		wBase := m.width / visCols
		rem := m.width % visCols

		for i := 0; i < visCols; i++ {
			colIdx := m.colOffset + i
			if colIdx >= numCols {
				break
			}
			colOuterWidth := wBase
			if i < rem {
				colOuterWidth++
			}
			renderedCols = append(renderedCols, m.renderColumn(colIdx, m.board.Columns[colIdx], colOuterWidth, colHeight))
		}
		content = lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)
	} else {
		content = m.renderSingleColumnTabbed(colHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar)
}

func (m Model) renderHeader() string {
	title := m.styles.AppTitle.Render("󰄶 gh-terrorizer")
	boardName := ""
	if m.board != nil {
		boardInfo := m.board.Title
		numCols := len(m.board.Columns)
		visCols := m.VisibleColumnCount()
		if numCols > visCols {
			boardInfo += fmt.Sprintf(" [Cols %d-%d of %d]", m.colOffset+1, m.colOffset+visCols, numCols)
		}
		boardName = lipgloss.NewStyle().Foreground(m.styles.Palette.Cyan).Render(boardInfo)
	}

	timerStatus := ""
	if m.activeTimerItemID != "" {
		elapsed := time.Since(m.activeTimerStart).Truncate(time.Second)
		timerStatus = m.styles.StatusTimer.Render(fmt.Sprintf("%s%s [%s]",
			m.styles.Icon("timer"),
			m.activeTimerTag,
			elapsed.String(),
		))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, boardName)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(timerStatus) - 4
	if gap < 0 {
		gap = 0
	}

	return m.styles.AppHeader.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Center,
			left,
			lipgloss.NewStyle().Width(gap).Render(""),
			timerStatus,
		),
	)
}

func (m Model) renderColumn(colIdx int, col github.Column, outerWidth int, colHeight int) string {
	items := m.ItemsInColumn(col.ID)
	isFocusedCol := (colIdx == m.activeColumn)

	boxWidth := outerWidth - 2
	if boxWidth < 10 {
		boxWidth = 10
	}
	innerContentWidth := boxWidth - 2
	if innerContentWidth < 8 {
		innerContentWidth = 8
	}

	hBox := colHeight
	if hBox < 8 {
		hBox = 8
	}
	innerContentHeight := hBox - 2
	if innerContentHeight < 6 {
		innerContentHeight = 6
	}

	// Column Title with scroll indicators
	colTitle := col.Name
	visCols := m.VisibleColumnCount()
	numCols := len(m.board.Columns)

	if colIdx == m.colOffset && m.colOffset > 0 {
		colTitle = "◀ " + colTitle
	}
	if colIdx == m.colOffset+visCols-1 && m.colOffset+visCols < numCols {
		colTitle = colTitle + " ▶"
	}

	countBadge := m.styles.ColumnCount.Render(fmt.Sprintf("(%d)", len(items)))
	headerTitle := m.styles.ColumnHeaderTitle.Render(colTitle)
	header := m.styles.ColumnHeader.Width(innerContentWidth).Render(headerTitle + countBadge)
	hHeader := lipgloss.Height(header)

	availCardsHeight := innerContentHeight - hHeader
	if availCardsHeight < 4 {
		availCardsHeight = 4
	}

	var cards []string
	if len(items) == 0 {
		cards = append(cards, m.styles.EmptyNotice.Render("Empty"))
	} else {
		selectedIdx := m.activeCard[colIdx]
		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= len(items) {
			selectedIdx = len(items) - 1
		}

		cardWidth := innerContentWidth - 2
		if cardWidth < 6 {
			cardWidth = 6
		}

		renderedCards := make([]string, len(items))
		cardHeights := make([]int, len(items))
		for i, item := range items {
			renderedCards[i] = m.renderCard(item, isFocusedCol && (i == selectedIdx), cardWidth)
			cardHeights[i] = lipgloss.Height(renderedCards[i])
		}

		start := selectedIdx
		end := selectedIdx
		usedHeight := cardHeights[selectedIdx]

		for {
			expanded := false
			if end+1 < len(items) {
				extraIndicator := 0
				if start > 0 {
					extraIndicator++
				}
				if end+1 < len(items)-1 {
					extraIndicator++
				}
				if usedHeight+cardHeights[end+1]+extraIndicator <= availCardsHeight {
					end++
					usedHeight += cardHeights[end]
					expanded = true
				}
			}

			if start-1 >= 0 {
				extraIndicator := 0
				if start-1 > 0 {
					extraIndicator++
				}
				if end < len(items)-1 {
					extraIndicator++
				}
				if usedHeight+cardHeights[start-1]+extraIndicator <= availCardsHeight {
					start--
					usedHeight += cardHeights[start]
					expanded = true
				}
			}

			if !expanded {
				break
			}
		}

		if start > 0 {
			cards = append(cards, m.styles.EmptyNotice.Render(fmt.Sprintf("↑ %d more", start)))
		}
		for i := start; i <= end; i++ {
			cards = append(cards, renderedCards[i])
		}
		if end < len(items)-1 {
			cards = append(cards, m.styles.EmptyNotice.Render(fmt.Sprintf("↓ %d more", len(items)-1-end)))
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, cards...)

	boxStyle := m.styles.ColumnBox.
		Width(boxWidth).
		Height(hBox).
		MaxHeight(hBox)
	if isFocusedCol {
		boxStyle = m.styles.ColumnBoxFocused.
			Width(boxWidth).
			Height(hBox).
			MaxHeight(hBox)
	}

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderSingleColumnTabbed(colHeight int) string {
	var tabs []string
	for idx, col := range m.board.Columns {
		label := fmt.Sprintf("[%d: %s (%d)]", idx+1, col.Name, len(m.ItemsInColumn(col.ID)))
		if idx == m.activeColumn {
			tabs = append(tabs, m.styles.TabActive.Render(label))
		} else {
			tabs = append(tabs, m.styles.TabInactive.Render(label))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	hTabs := lipgloss.Height(tabBar)

	activeCol := m.board.Columns[m.activeColumn]
	activeItems := m.ItemsInColumn(activeCol.ID)

	availCardsHeight := colHeight - hTabs
	if availCardsHeight < 4 {
		availCardsHeight = 4
	}

	var cards []string
	if len(activeItems) == 0 {
		cards = append(cards, m.styles.EmptyNotice.Render("Empty column"))
	} else {
		selectedIdx := m.activeCard[m.activeColumn]
		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= len(activeItems) {
			selectedIdx = len(activeItems) - 1
		}

		cardWidth := m.width - 6
		if cardWidth < 10 {
			cardWidth = 10
		}

		renderedCards := make([]string, len(activeItems))
		cardHeights := make([]int, len(activeItems))
		for i, item := range activeItems {
			renderedCards[i] = m.renderCard(item, i == selectedIdx, cardWidth)
			cardHeights[i] = lipgloss.Height(renderedCards[i])
		}

		start := selectedIdx
		end := selectedIdx
		usedHeight := cardHeights[selectedIdx]

		for {
			expanded := false
			if end+1 < len(activeItems) {
				extraIndicator := 0
				if start > 0 {
					extraIndicator++
				}
				if end+1 < len(activeItems)-1 {
					extraIndicator++
				}
				if usedHeight+cardHeights[end+1]+extraIndicator <= availCardsHeight {
					end++
					usedHeight += cardHeights[end]
					expanded = true
				}
			}

			if start-1 >= 0 {
				extraIndicator := 0
				if start-1 > 0 {
					extraIndicator++
				}
				if end < len(activeItems)-1 {
					extraIndicator++
				}
				if usedHeight+cardHeights[start-1]+extraIndicator <= availCardsHeight {
					start--
					usedHeight += cardHeights[start]
					expanded = true
				}
			}

			if !expanded {
				break
			}
		}

		if start > 0 {
			cards = append(cards, m.styles.EmptyNotice.Render(fmt.Sprintf("↑ %d more", start)))
		}
		for i := start; i <= end; i++ {
			cards = append(cards, renderedCards[i])
		}
		if end < len(activeItems)-1 {
			cards = append(cards, m.styles.EmptyNotice.Render(fmt.Sprintf("↓ %d more", len(activeItems)-1-end)))
		}
	}

	colBody := lipgloss.JoinVertical(lipgloss.Left, cards...)
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, colBody)
}

func (m Model) renderCard(item github.ProjectItem, isFocused bool, width int) string {
	// Card style selection
	cardStyle := m.styles.CardNormal.Width(width)
	if isFocused {
		if m.movingCard {
			cardStyle = m.styles.CardMoving.Width(width)
		} else {
			cardStyle = m.styles.CardFocused.Width(width)
		}
	}

	// Badges & title
	prefix := m.styles.Icon("issue")
	if item.Type == "PULL_REQUEST" {
		prefix = m.styles.Icon("pr")
	} else if item.Type == "DRAFT_ISSUE" {
		prefix = m.styles.Icon("draft")
	}

	numberStr := ""
	if item.Number > 0 {
		numberStr = fmt.Sprintf("#%d", item.Number)
	}

	titleLine := fmt.Sprintf("%s%s %s",
		prefix,
		m.styles.CardNumber.Render(numberStr),
		m.styles.CardTitle.Render(item.Title),
	)

	// Hours badge
	hoursStr := timew.FormatHours(item.HoursSpent)
	hoursBadge := m.styles.CardHours.Render(fmt.Sprintf("%s%s", m.styles.Icon("timer"), hoursStr))
	if m.activeTimerItemID == item.ID {
		hoursBadge = m.styles.CardHoursActive.Render(fmt.Sprintf("%s%s [TRACKING]", m.styles.Icon("timer"), hoursStr))
	}

	// Second line: Milestone + labels
	var metaBadges []string
	if item.Milestone != nil {
		metaBadges = append(metaBadges, m.styles.BadgeMilestone.Render(
			fmt.Sprintf("%s%s", m.styles.Icon("milestone"), item.Milestone.Title),
		))
	}
	for _, l := range item.Labels {
		metaBadges = append(metaBadges, m.styles.BadgeLabel.Render(l.Name))
	}
	metaBadges = append(metaBadges, hoursBadge)
	metaLine := lipgloss.JoinHorizontal(lipgloss.Center, metaBadges...)

	return cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, titleLine, metaLine))
}

func (m Model) renderStatusBar() string {
	keys := []struct {
		key  string
		desc string
	}{
		{"h/l", "cols"},
		{"j/k", "cards"},
		{"H/L", "move"},
		{"t", "timer"},
		{"e", "edit"},
		{"r", "refresh"},
		{"q", "quit"},
	}

	if m.width < 85 {
		keys = []struct {
			key  string
			desc string
		}{
			{"h/l", "cols"},
			{"j/k", "cards"},
			{"H/L", "move"},
			{"t", "timer"},
			{"q", "quit"},
		}
	}

	var items []string
	for _, k := range keys {
		items = append(items, m.styles.StatusKey.Render(k.key), m.styles.StatusDesc.Render(k.desc))
	}

	if m.board != nil && len(m.board.Columns) > 0 {
		colIndicator := fmt.Sprintf("[%d/%d: %s]", m.activeColumn+1, len(m.board.Columns), m.board.Columns[m.activeColumn].Name)
		items = append(items, m.styles.BadgeState.Render(colIndicator))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, items...)

	right := ""
	if m.toast != "" {
		right = m.styles.ToastError.Render(m.toast)
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 0 {
		gap = 0
	}

	return m.styles.StatusBar.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Center,
			left,
			lipgloss.NewStyle().Width(gap).Render(""),
			right,
		),
	)
}
