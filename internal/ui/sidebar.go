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
	tables         []string
	filteredTables []string
	cursor         int
	selected       string
	focused        bool
	searching      bool
	searchQuery    string
	width          int
	height         int
}

// NewSidebarModel creates a new sidebar with the given table list.
func NewSidebarModel(tables []string) SidebarModel {
	return SidebarModel{
		tables:         tables,
		filteredTables: tables,
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
	m.applyFilter()
}

// IsSearching returns whether the sidebar is in search mode.
func (m SidebarModel) IsSearching() bool {
	return m.searching
}

func (m *SidebarModel) applyFilter() {
	if m.searchQuery == "" {
		m.filteredTables = m.tables
	} else {
		m.filteredTables = nil
		for _, t := range m.tables {
			if FuzzyMatch(t, m.searchQuery) {
				m.filteredTables = append(m.filteredTables, t)
			}
		}
	}
	if m.cursor >= len(m.filteredTables) {
		m.cursor = max(0, len(m.filteredTables)-1)
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
		if m.searching {
			return m.updateSearchMode(msg)
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filteredTables)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.filteredTables) > 0 {
				m.selected = m.filteredTables[m.cursor]
				return m, func() tea.Msg {
					return TableSelectedMsg{Name: m.selected}
				}
			}
		case "/":
			m.searching = true
			m.searchQuery = ""
		}
	}
	return m, nil
}

func (m SidebarModel) updateSearchMode(msg tea.KeyMsg) (SidebarModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchQuery = ""
		m.applyFilter()
	case "enter":
		m.searching = false
		if len(m.filteredTables) > 0 {
			m.selected = m.filteredTables[m.cursor]
			return m, func() tea.Msg {
				return TableSelectedMsg{Name: m.selected}
			}
		}
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.applyFilter()
		}
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down":
		if m.cursor < len(m.filteredTables)-1 {
			m.cursor++
		}
	default:
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			m.searchQuery += msg.String()
			m.applyFilter()
		} else if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.applyFilter()
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

	linesUsed := 1

	if m.searching || m.searchQuery != "" {
		searchDisp := SearchLabel.Render("/") + SearchInput.Render(m.searchQuery)
		if m.searching {
			searchDisp += SearchInput.Render("█")
		}
		b.WriteString(searchDisp)
		b.WriteString("\n")
		linesUsed++
	} else {
		schema := SubHeaderStyle.Render("  public")
		b.WriteString(schema)
		b.WriteString("\n")
		linesUsed++
	}

	tables := m.filteredTables

	if len(tables) == 0 {
		if m.searchQuery != "" {
			b.WriteString(DimText.Render("  No matches"))
		} else {
			b.WriteString(DimText.Render("  No tables found"))
		}
		linesUsed++
	} else {
		for i, t := range tables {
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
			if i < len(tables)-1 {
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

