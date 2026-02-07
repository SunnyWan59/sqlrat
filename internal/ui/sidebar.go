package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableSelectedMsg is sent when a table is selected in the sidebar.
type TableSelectedMsg struct {
	Name string
}

// SidebarModel is the table browser sidebar.
type SidebarModel struct {
	tables   []string
	cursor   int
	selected string
	focused  bool
	width    int
	height   int
}

// NewSidebarModel creates a new sidebar with the given table list.
func NewSidebarModel(tables []string) SidebarModel {
	return SidebarModel{
		tables: tables,
	}
}

// SetFocused sets the focus state.
func (m *SidebarModel) SetFocused(f bool) {
	m.focused = f
}

// Focused returns the focus state.
func (m SidebarModel) Focused() bool {
	return m.focused
}

// SetSize sets the sidebar dimensions.
func (m *SidebarModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetTables updates the table list.
func (m *SidebarModel) SetTables(tables []string) {
	m.tables = tables
	if m.cursor >= len(tables) {
		m.cursor = max(0, len(tables)-1)
	}
}

// Selected returns the currently selected table name.
func (m SidebarModel) Selected() string {
	return m.selected
}

// Init satisfies the tea.Model interface.
func (m SidebarModel) Init() tea.Cmd {
	return nil
}

// Update handles key events.
func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tables)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.tables) > 0 {
				m.selected = m.tables[m.cursor]
				return m, func() tea.Msg {
					return TableSelectedMsg{Name: m.selected}
				}
			}
		}
	}
	return m, nil
}

// View renders the sidebar.
func (m SidebarModel) View() string {
	borderStyle := UnfocusedBorder
	if m.focused {
		borderStyle = FocusedBorder
	}

	// Inner width accounts for border
	innerW := m.width - 2
	if innerW < 5 {
		innerW = 5
	}
	innerH := m.height - 2
	if innerH < 1 {
		innerH = 1
	}

	var b strings.Builder

	// Header
	header := HeaderStyle.Render("Tables")
	b.WriteString(header)
	b.WriteString("\n")
	schema := SubHeaderStyle.Render("  public")
	b.WriteString(schema)
	b.WriteString("\n")

	linesUsed := 2

	if len(m.tables) == 0 {
		b.WriteString(DimText.Render("  No tables found"))
		linesUsed++
	} else {
		for i, t := range m.tables {
			if linesUsed >= innerH {
				break
			}
			label := fmt.Sprintf("T %s", t)
			if len(label) > innerW {
				label = label[:innerW-3] + "..."
			}
			var line string
			if i == m.cursor && m.focused {
				line = SidebarCursorItem.Width(innerW).Render(label)
			} else if t == m.selected {
				line = SidebarActiveItem.Width(innerW).Render(label)
			} else {
				line = SidebarTableItem.Width(innerW).Render(label)
			}
			b.WriteString(line)
			if i < len(m.tables)-1 {
				b.WriteString("\n")
			}
			linesUsed++
		}
	}

	// Pad remaining lines
	for linesUsed < innerH {
		b.WriteString("\n")
		linesUsed++
	}

	content := lipgloss.NewStyle().Width(innerW).Height(innerH).Render(b.String())
	return borderStyle.Width(innerW).Height(innerH).Render(content)
}

