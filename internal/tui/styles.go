package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// TokyoNightPalette defines color constants for Tokyo Night themes.
type TokyoNightPalette struct {
	BgDark         lipgloss.Color
	Bg             lipgloss.Color
	BgHighlight    lipgloss.Color
	Fg             lipgloss.Color
	FgDark         lipgloss.Color
	Comment        lipgloss.Color
	Blue           lipgloss.Color
	Cyan           lipgloss.Color
	Green          lipgloss.Color
	Yellow         lipgloss.Color
	Orange         lipgloss.Color
	Red            lipgloss.Color
	Purple         lipgloss.Color
	BorderInactive lipgloss.Color
}

// GetPalette returns the Tokyo Night palette according to variant.
func GetPalette(variant string) TokyoNightPalette {
	switch variant {
	case "night":
		return TokyoNightPalette{
			BgDark:         lipgloss.Color("#16161e"),
			Bg:             lipgloss.Color("#1a1b26"),
			BgHighlight:    lipgloss.Color("#292e42"),
			Fg:             lipgloss.Color("#c0caf5"),
			FgDark:         lipgloss.Color("#a9b1d6"),
			Comment:        lipgloss.Color("#565f89"),
			Blue:           lipgloss.Color("#7aa2f7"),
			Cyan:           lipgloss.Color("#7dcfff"),
			Green:          lipgloss.Color("#73daca"),
			Yellow:         lipgloss.Color("#e0af68"),
			Orange:         lipgloss.Color("#ff9e64"),
			Red:            lipgloss.Color("#f7768e"),
			Purple:         lipgloss.Color("#bb9af7"),
			BorderInactive: lipgloss.Color("#3b4261"),
		}
	default: // "storm" default
		return TokyoNightPalette{
			BgDark:         lipgloss.Color("#1f2335"),
			Bg:             lipgloss.Color("#24283b"),
			BgHighlight:    lipgloss.Color("#292e42"),
			Fg:             lipgloss.Color("#c0caf5"),
			FgDark:         lipgloss.Color("#a9b1d6"),
			Comment:        lipgloss.Color("#565f89"),
			Blue:           lipgloss.Color("#7aa2f7"),
			Cyan:           lipgloss.Color("#7dcfff"),
			Green:          lipgloss.Color("#73daca"),
			Yellow:         lipgloss.Color("#e0af68"),
			Orange:         lipgloss.Color("#ff9e64"),
			Red:            lipgloss.Color("#f7768e"),
			Purple:         lipgloss.Color("#bb9af7"),
			BorderInactive: lipgloss.Color("#3b4261"),
		}
	}
}

// Styles contains all initialized Lip Gloss styles for the TUI.
type Styles struct {
	Palette TokyoNightPalette
	Icons   bool

	AppHeader     lipgloss.Style
	AppTitle      lipgloss.Style
	TabActive     lipgloss.Style
	TabInactive   lipgloss.Style
	ColumnHeader  lipgloss.Style
	ColumnHeaderTitle lipgloss.Style
	ColumnCount   lipgloss.Style
	ColumnBox     lipgloss.Style
	ColumnBoxFocused lipgloss.Style
	
	CardNormal    lipgloss.Style
	CardFocused   lipgloss.Style
	CardMoving    lipgloss.Style
	CardTitle     lipgloss.Style
	CardNumber    lipgloss.Style
	CardHours     lipgloss.Style
	CardHoursActive lipgloss.Style

	BadgeMilestone lipgloss.Style
	BadgeLabel     lipgloss.Style
	BadgeState     lipgloss.Style

	StatusBar     lipgloss.Style
	StatusKey     lipgloss.Style
	StatusDesc    lipgloss.Style
	StatusTimer   lipgloss.Style
	ToastError    lipgloss.Style
	ToastSuccess  lipgloss.Style
	EmptyNotice   lipgloss.Style
}

// NewStyles initializes styles with the selected palette and icon preference.
func NewStyles(variant string, icons bool) Styles {
	p := GetPalette(variant)

	s := Styles{
		Palette: p,
		Icons:   icons,

		AppHeader: lipgloss.NewStyle().
			Background(p.BgDark).
			Foreground(p.Fg).
			Padding(0, 1),

		AppTitle: lipgloss.NewStyle().
			Foreground(p.Blue).
			Bold(true).
			MarginRight(2),

		TabActive: lipgloss.NewStyle().
			Background(p.Blue).
			Foreground(p.BgDark).
			Bold(true).
			Padding(0, 1).
			MarginRight(1),

		TabInactive: lipgloss.NewStyle().
			Background(p.BgHighlight).
			Foreground(p.Comment).
			Padding(0, 1).
			MarginRight(1),

		ColumnHeader: lipgloss.NewStyle().
			Background(p.BgHighlight).
			Foreground(p.Fg).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1),

		ColumnHeaderTitle: lipgloss.NewStyle().
			Foreground(p.Cyan).
			Bold(true),

		ColumnCount: lipgloss.NewStyle().
			Foreground(p.Comment).
			MarginLeft(1),

		ColumnBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderInactive).
			Padding(0, 1),

		ColumnBoxFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Blue).
			Padding(0, 1),

		CardNormal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderInactive).
			Padding(0, 1).
			MarginBottom(1),

		CardFocused: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(p.Blue).
			Background(p.BgHighlight).
			Padding(0, 1).
			MarginBottom(1),

		CardMoving: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(p.Purple).
			Background(p.BgHighlight).
			Padding(0, 1).
			MarginBottom(1),

		CardTitle: lipgloss.NewStyle().
			Foreground(p.Fg).
			Bold(true),

		CardNumber: lipgloss.NewStyle().
			Foreground(p.Comment).
			MarginRight(1),

		CardHours: lipgloss.NewStyle().
			Foreground(p.Comment).
			MarginLeft(1),

		CardHoursActive: lipgloss.NewStyle().
			Background(p.Orange).
			Foreground(p.BgDark).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1),

		BadgeMilestone: lipgloss.NewStyle().
			Background(p.BgHighlight).
			Foreground(p.Yellow).
			Padding(0, 1).
			MarginRight(1),

		BadgeLabel: lipgloss.NewStyle().
			Background(p.BgHighlight).
			Foreground(p.Cyan).
			Padding(0, 1).
			MarginRight(1),

		BadgeState: lipgloss.NewStyle().
			Foreground(p.Green).
			MarginRight(1),

		StatusBar: lipgloss.NewStyle().
			Background(p.BgDark).
			Foreground(p.Fg).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(p.Blue).
			Bold(true),

		StatusDesc: lipgloss.NewStyle().
			Foreground(p.Comment).
			MarginRight(2),

		StatusTimer: lipgloss.NewStyle().
			Background(p.Orange).
			Foreground(p.BgDark).
			Bold(true).
			Padding(0, 1),

		ToastError: lipgloss.NewStyle().
			Background(p.Red).
			Foreground(p.Fg).
			Bold(true).
			Padding(0, 1),

		ToastSuccess: lipgloss.NewStyle().
			Background(p.Green).
			Foreground(p.BgDark).
			Bold(true).
			Padding(0, 1),

		EmptyNotice: lipgloss.NewStyle().
			Foreground(p.Comment).
			Italic(true).
			Padding(2, 4),
	}

	return s
}

// Icon returns the Nerd Font icon if enabled, otherwise a textual fallback.
func (s *Styles) Icon(iconName string) string {
	if !s.Icons {
		switch iconName {
		case "timer":
			return "[TIMER]"
		case "milestone":
			return "[M]"
		case "issue":
			return "#"
		case "pr":
			return "PR"
		case "draft":
			return "[D]"
		case "check":
			return "[X]"
		default:
			return ""
		}
	}

	switch iconName {
	case "timer":
		return "󱎫 "
	case "milestone":
		return " "
	case "issue":
		return " "
	case "pr":
		return " "
	case "draft":
		return " "
	case "check":
		return " "
	default:
		return ""
	}
}
