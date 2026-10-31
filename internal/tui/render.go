package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/ejablonski/gh-terrorizer/internal/github"
	"github.com/ejablonski/gh-terrorizer/internal/timew"
	"github.com/mattn/go-runewidth"
)

// formatRelativeTime formats elapsed duration from a timestamp.
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "󰄬 Initialized"
	}
	elapsed := time.Since(t)
	if elapsed < time.Minute {
		return "󰄬 Synced just now"
	}
	return fmt.Sprintf("󰄬 Synced %dm ago", int(elapsed.Minutes()))
}

// renderTopBar renders the persistent top tier with branding, repository context, Timewarrior status, and sync status.
func (m Model) renderTopBar(width int) string {
	brand := m.styles.TopBarBrand.Render("󰄶 gh-terrorizer")

	var projectInfo string
	if m.board != nil {
		if width < 85 {
			projectInfo = m.board.Title
		} else if m.cfg != nil {
			projectInfo = fmt.Sprintf("%s/%s %s #%d", m.cfg.DefaultOwner, m.cfg.DefaultRepo, m.board.Title, m.cfg.ProjectNumber)
		} else {
			projectInfo = m.board.Title
		}
	}
	projectBadge := m.styles.AppHeader.Render(projectInfo)
	left := lipgloss.JoinHorizontal(lipgloss.Center, brand, " ", projectBadge)

	var timerBadge string
	if !m.activeTimerStart.IsZero() {
		elapsed := time.Since(m.activeTimerStart).Truncate(time.Second).String()
		timerBadge = m.styles.TimerBadgeActive.Render(fmt.Sprintf("󱎫 %s [%s]", m.activeTimerTag, elapsed))
	} else {
		timerBadge = m.styles.TimerBadgeIdle.Render("󱎫 Idle")
	}

	var syncBadge string
	if m.syncInProgress {
		syncBadge = m.styles.TimerBadgeActive.Render("󰑮 Syncing... (r: sync)")
	} else {
		syncBadge = m.styles.TimerBadgeIdle.Render(formatRelativeTime(m.lastSyncTime))
	}

	leftLen := lipgloss.Width(left)
	centerLen := lipgloss.Width(timerBadge)
	rightLen := lipgloss.Width(syncBadge)
	totalLen := leftLen + centerLen + rightLen

	if width <= totalLen+2 {
		if width < 85 {
			left = brand
			leftLen = lipgloss.Width(left)
			totalLen = leftLen + centerLen + rightLen
		}
		if width <= totalLen {
			combined := lipgloss.JoinHorizontal(lipgloss.Center, left, " ", timerBadge, " ", syncBadge)
			return runewidth.Truncate(combined, width, "")
		}
	}

	gap := width - totalLen
	gap1 := gap / 2
	gap2 := gap - gap1

	return left + strings.Repeat(" ", gap1) + timerBadge + strings.Repeat(" ", gap2) + syncBadge
}

// renderBottomBar renders the bottom tier with context-specific keyhints and status toast notifications.
func (m Model) renderBottomBar(width int) string {
	var hints string
	if m.viewMode == ViewModeKanban && m.movingCard {
		hints = "h/l: move card to adjacent column • Esc: cancel move"
	} else if m.viewMode == ViewModeKanban {
		hints = "h/l: cols • j/k: cards • m: move • H/L: shift • t: timer • r: sync • Enter/i: inspect • q: quit"
	} else if m.viewMode == ViewModeInspector {
		if width < 85 {
			hints = "j/k: scroll • e: edit • Esc/q: back"
		} else {
			hints = "j/k: scroll • d/u: half page • g/G: top/bottom • e: edit in $EDITOR • Esc/q: back to board"
		}
	}

	var toast string
	if m.err != nil {
		toast = m.styles.ToastError.Render(m.err.Error())
	} else if m.toast != "" {
		toast = m.styles.ToastSuccess.Render(m.toast)
	}

	hintsStyled := m.styles.StatusBar.Render(hints)
	if toast == "" {
		if lipgloss.Width(hintsStyled) > width {
			return runewidth.Truncate(hintsStyled, width, "")
		}
		return hintsStyled
	}

	toastWidth := lipgloss.Width(toast)
	hintsWidth := lipgloss.Width(hintsStyled)

	if hintsWidth+toastWidth+2 > width {
		availHints := max(0, width-toastWidth-2)
		hints = runewidth.Truncate(hints, availHints, "...")
		hintsStyled = m.styles.StatusBar.Render(hints)
		hintsWidth = lipgloss.Width(hintsStyled)
	}

	gap := max(1, width-hintsWidth-toastWidth)
	return hintsStyled + strings.Repeat(" ", gap) + toast
}

// renderInspectorModal renders the centered Issue Inspector Modal with metadata and embedded viewport.
func (m Model) renderInspectorModal(width, height int) string {
	item := m.SelectedItem()
	if item == nil {
		return ""
	}

	wModal := max(16, min(width-2, int(float64(width)*0.85)))
	hModal := max(7, min(height-2, int(float64(height)*0.85)))
	wViewport := max(10, wModal-4)
	hViewport := max(2, hModal-5)

	m.inspectorViewport.Width = wViewport
	m.inspectorViewport.Height = hViewport

	availWidth := max(0, wModal-6)
	headerTitle := runewidth.Truncate(fmt.Sprintf("#%d %s", item.Number, item.Title), availWidth, "...")

	// Metadata strip
	var stateBadge string
	if strings.ToUpper(item.State) == "CLOSED" {
		stateBadge = m.styles.BadgeStateClosed.Render("CLOSED")
	} else {
		stateBadge = m.styles.BadgeStateOpen.Render("OPEN")
	}

	var metaParts []string
	metaParts = append(metaParts, stateBadge)

	if item.Milestone != nil {
		metaParts = append(metaParts, m.styles.BadgeMilestone.Render(m.styles.Icon("milestone")+item.Milestone.Title))
	}
	for _, l := range item.Labels {
		metaParts = append(metaParts, m.styles.BadgeLabel.Render(l.Name))
	}
	timeBadge := m.styles.TimerBadgeActive.Render(fmt.Sprintf("󱎫 %.2fh", item.HoursSpent))
	metaParts = append(metaParts, timeBadge)

	metaStrip := lipgloss.JoinHorizontal(lipgloss.Center, metaParts...)
	if lipgloss.Width(metaStrip) > wViewport {
		metaStrip = runewidth.Truncate(metaStrip, wViewport, "...")
	}

	// Divider
	divider := m.styles.TimerBadgeIdle.Render(strings.Repeat("─", wViewport))

	// Viewport content
	viewportView := m.inspectorViewport.View()

	// Footer
	percent := m.inspectorViewport.ScrollPercent() * 100
	footer := m.styles.ViewportFooter.Width(wViewport).Render(fmt.Sprintf("[%3.0f%%]", percent))

	// Combine inner lines
	content := lipgloss.JoinVertical(lipgloss.Left, metaStrip, divider, viewportView, footer)

	// Apply ModalFrame
	modalBox := m.styles.ModalFrame.Width(wViewport).Render(content)

	// Inject title into top border line if space permits
	lines := strings.Split(modalBox, "\n")
	if len(lines) > 0 && availWidth >= 6 && headerTitle != "" {
		remainingDashes := max(0, wModal-runewidth.StringWidth(headerTitle)-5)
		topBorder := "╭─ " + headerTitle + " " + strings.Repeat("─", remainingDashes) + "╮"
		lines[0] = lipgloss.NewStyle().Foreground(m.styles.Palette.Blue).Render(topBorder)
		modalBox = strings.Join(lines, "\n")
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modalBox)
}

// renderKanbanBoard renders all visible Kanban columns horizontally.
func (m Model) renderKanbanBoard(width, height int) string {
	if m.board == nil || len(m.board.Columns) == 0 {
		notice := m.styles.EmptyNotice.Render("No status columns found on this GitHub Project board.\nPress 'q' to quit.")
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, notice)
	}

	colHeight := height
	if colHeight < 8 {
		colHeight = 8
	}

	numCols := len(m.board.Columns)
	visCols := m.VisibleColumnCount()

	if visCols > 1 || numCols == 1 {
		m.adjustViewport()
		var renderedCols []string

		wBase := width / visCols
		rem := width % visCols

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
		return lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)
	}

	return m.renderSingleColumnTabbed(colHeight)
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

	// colHeight is the total outer height allocated for the column box.
	// ColumnBox borders take 2 lines (top 1 + bottom 1).
	// In Lip Gloss, .Height(h) sets the inner content height.
	// Therefore, innerContentHeight must be colHeight - 2 so the outer height is exactly colHeight.
	innerContentHeight := colHeight - 2
	if innerContentHeight < 4 {
		innerContentHeight = 4
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
		cards = append(cards, m.styles.EmptyNotice.Render("Empty column"))
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

		calcTotalHeight := func(s, e int) int {
			h := 0
			for i := s; i <= e; i++ {
				h += cardHeights[i]
			}
			if s > 0 {
				h++
			}
			if e < len(items)-1 {
				h++
			}
			return h
		}

		start := selectedIdx
		end := selectedIdx

		for {
			expanded := false
			if end+1 < len(items) && calcTotalHeight(start, end+1) <= availCardsHeight {
				end++
				expanded = true
			}
			if start-1 >= 0 && calcTotalHeight(start-1, end) <= availCardsHeight {
				start--
				expanded = true
			}
			if !expanded {
				break
			}
		}

		if start > 0 {
			cards = append(cards, m.styles.ScrollIndicator.Render(fmt.Sprintf("↑ %d more", start)))
		}
		for i := start; i <= end; i++ {
			cards = append(cards, renderedCards[i])
		}
		if end < len(items)-1 {
			cards = append(cards, m.styles.ScrollIndicator.Render(fmt.Sprintf("↓ %d more", len(items)-1-end)))
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, cards...)
	if lipgloss.Height(body) > availCardsHeight {
		bLines := strings.Split(body, "\n")
		if len(bLines) > availCardsHeight {
			body = strings.Join(bLines[:availCardsHeight], "\n")
		}
	}

	boxStyle := m.styles.ColumnBox.
		Width(boxWidth).
		Height(innerContentHeight)
	if isFocusedCol {
		boxStyle = m.styles.ColumnBoxFocused.
			Width(boxWidth).
			Height(innerContentHeight)
	}

	renderedBox := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
	boxLines := strings.Split(renderedBox, "\n")
	if len(boxLines) > colHeight && colHeight > 2 {
		clamped := make([]string, 0, colHeight)
		clamped = append(clamped, boxLines[:colHeight-1]...)
		clamped = append(clamped, boxLines[len(boxLines)-1])
		renderedBox = strings.Join(clamped, "\n")
	}
	return renderedBox
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

		cardWidth := m.width - 4
		if cardWidth < 10 {
			cardWidth = 10
		}

		renderedCards := make([]string, len(activeItems))
		cardHeights := make([]int, len(activeItems))
		for i, item := range activeItems {
			renderedCards[i] = m.renderCard(item, i == selectedIdx, cardWidth)
			cardHeights[i] = lipgloss.Height(renderedCards[i])
		}

		calcTotalHeight := func(s, e int) int {
			h := 0
			for i := s; i <= e; i++ {
				h += cardHeights[i]
			}
			if s > 0 {
				h++
			}
			if e < len(activeItems)-1 {
				h++
			}
			return h
		}

		start := selectedIdx
		end := selectedIdx

		for {
			expanded := false
			if end+1 < len(activeItems) && calcTotalHeight(start, end+1) <= availCardsHeight {
				end++
				expanded = true
			}
			if start-1 >= 0 && calcTotalHeight(start-1, end) <= availCardsHeight {
				start--
				expanded = true
			}
			if !expanded {
				break
			}
		}

		if start > 0 {
			cards = append(cards, m.styles.ScrollIndicator.Render(fmt.Sprintf("↑ %d more", start)))
		}
		for i := start; i <= end; i++ {
			cards = append(cards, renderedCards[i])
		}
		if end < len(activeItems)-1 {
			cards = append(cards, m.styles.ScrollIndicator.Render(fmt.Sprintf("↓ %d more", len(activeItems)-1-end)))
		}
	}

	body := lipgloss.JoinVertical(lipgloss.Left, cards...)
	if lipgloss.Height(body) > availCardsHeight {
		bLines := strings.Split(body, "\n")
		if len(bLines) > availCardsHeight {
			body = strings.Join(bLines[:availCardsHeight], "\n")
		}
	}
	tabbed := lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
	tabLines := strings.Split(tabbed, "\n")
	if len(tabLines) > colHeight {
		tabbed = strings.Join(tabLines[:colHeight], "\n")
	} else if len(tabLines) < colHeight {
		tabbed += strings.Repeat("\n", colHeight-len(tabLines))
	}
	return tabbed
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
