package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	ColorAccent    = lipgloss.Color("#4ecca3")
	ColorDanger    = lipgloss.Color("#e94560")
	ColorModified  = lipgloss.Color("#f0a500")
	ColorDim       = lipgloss.Color("#555555")
	ColorSuccess   = lipgloss.Color("#4ecca3")
	ColorError     = lipgloss.Color("#e94560")
	ColorNewRow    = lipgloss.Color("#4ecca3")
	ColorDeleteRow = lipgloss.Color("#e94560")
)

// Border styles
var (
	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent)

	UnfocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDim)
)

// Text styles
var (
	AccentText = lipgloss.NewStyle().Foreground(ColorAccent)
	DimText    = lipgloss.NewStyle().Foreground(ColorDim)
	ErrorText  = lipgloss.NewStyle().Foreground(ColorError)
	SuccessText = lipgloss.NewStyle().Foreground(ColorSuccess)
	ModifiedText = lipgloss.NewStyle().Foreground(ColorModified)
	DeletedText  = lipgloss.NewStyle().Foreground(ColorDeleteRow).Faint(true)
	NewRowText   = lipgloss.NewStyle().Foreground(ColorNewRow)
	NullText     = lipgloss.NewStyle().Foreground(ColorDim).Italic(true)
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(ColorDim)
)

// Table cell styles
var (
	CellNormal   = lipgloss.NewStyle()
	CellSelected = lipgloss.NewStyle().Reverse(true)
	CellEditing  = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorAccent)
)

// Status bar
var (
	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#cccccc")).
			Padding(0, 1)

	StatusErrorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#333333")).
				Foreground(ColorError).
				Padding(0, 1)

	StatusSuccessStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#333333")).
				Foreground(ColorSuccess).
				Padding(0, 1)
)

// Sidebar styles
var (
	SidebarTableItem = lipgloss.NewStyle().PaddingLeft(1)
	SidebarActiveItem = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(ColorAccent).
				Bold(true)
	SidebarCursorItem = lipgloss.NewStyle().
				PaddingLeft(1).
				Reverse(true)
)

// Top bar style
var TopBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#333333")).
	Foreground(lipgloss.Color("#cccccc")).
	Padding(0, 1)
