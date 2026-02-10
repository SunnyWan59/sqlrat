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

// DatabaseSelectedMsg is sent when a database is selected in the sidebar.
type DatabaseSelectedMsg struct {
	Name string
}

// CopyDatabaseMsg is sent when the user confirms copying a database.
type CopyDatabaseMsg struct {
	Source string
	Target string
}

// DeleteDatabaseMsg is sent when the user confirms deleting a database.
type DeleteDatabaseMsg struct {
	Name string
}

// SidebarMode tracks whether the sidebar shows tables or databases.
type SidebarMode int

const (
	SidebarTables SidebarMode = iota
	SidebarDatabases
)

// SidebarModel is the table browser sidebar.
type SidebarModel struct {
	tables            []string
	filteredTables    []string
	databases         []string
	filteredDatabases []string
	activeDatabase    string
	mode              SidebarMode
	cursor            int
	selected          string
	focused           bool
	searching         bool
	searchQuery       string
	scrollOffset      int
	copying           bool
	copySource        string
	copyInput         string
	confirmDelete     bool
	deleteTarget      string
	width             int
	height            int
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

// SetDatabases updates the database list.
func (m *SidebarModel) SetDatabases(databases []string) {
	m.databases = databases
	m.filteredDatabases = databases
}

// SetActiveDatabase sets the currently connected database name.
func (m *SidebarModel) SetActiveDatabase(name string) {
	m.activeDatabase = name
}

// IsSearching returns whether the sidebar is in an input mode (search, copy, or delete confirm).
func (m SidebarModel) IsSearching() bool {
	return m.searching || m.copying || m.confirmDelete
}

func (m *SidebarModel) ensureVisible() {
	innerH := m.height - 2
	if innerH < 1 {
		innerH = 1
	}
	headerLines := 2
	if m.mode == SidebarDatabases {
		if m.copying {
			headerLines = 4
		} else if m.confirmDelete {
			headerLines = 3
		}
	}
	availLines := innerH - headerLines
	listLen := len(m.filteredTables)
	if m.mode == SidebarDatabases {
		listLen = len(m.filteredDatabases)
	}
	if listLen > availLines {
		availLines--
	}
	if availLines < 1 {
		availLines = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+availLines {
		m.scrollOffset = m.cursor - availLines + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m *SidebarModel) applyFilter() {
	if m.mode == SidebarDatabases {
		if m.searchQuery == "" {
			m.filteredDatabases = m.databases
		} else {
			m.filteredDatabases = nil
			for _, d := range m.databases {
				if FuzzyMatch(d, m.searchQuery) {
					m.filteredDatabases = append(m.filteredDatabases, d)
				}
			}
		}
		if m.cursor >= len(m.filteredDatabases) {
			m.cursor = max(0, len(m.filteredDatabases)-1)
		}
		m.ensureVisible()
		return
	}

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
	m.ensureVisible()
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
		if m.confirmDelete {
			return m.updateDeleteConfirm(msg)
		}
		if m.copying {
			return m.updateCopyMode(msg)
		}
		if m.searching {
			return m.updateSearchMode(msg)
		}

		listLen := len(m.filteredTables)
		if m.mode == SidebarDatabases {
			listLen = len(m.filteredDatabases)
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < listLen-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "enter":
			if m.mode == SidebarDatabases {
				if len(m.filteredDatabases) > 0 {
					selected := m.filteredDatabases[m.cursor]
					return m, func() tea.Msg {
						return DatabaseSelectedMsg{Name: selected}
					}
				}
			} else {
				if len(m.filteredTables) > 0 {
					m.selected = m.filteredTables[m.cursor]
					return m, func() tea.Msg {
						return TableSelectedMsg{Name: m.selected}
					}
				}
			}
		case "c":
			if m.mode == SidebarDatabases && len(m.filteredDatabases) > 0 {
				m.copying = true
				m.copySource = m.filteredDatabases[m.cursor]
				m.copyInput = m.copySource + "_copy"
			}
		case "x":
			if m.mode == SidebarDatabases && len(m.filteredDatabases) > 0 {
				m.confirmDelete = true
				m.deleteTarget = m.filteredDatabases[m.cursor]
			}
		case "D":
			m.cursor = 0
			m.scrollOffset = 0
			m.searching = false
			m.searchQuery = ""
			if m.mode == SidebarDatabases {
				m.mode = SidebarTables
				m.applyFilter()
			} else {
				m.mode = SidebarDatabases
				m.applyFilter()
			}
		case "/":
			m.searching = true
			m.searchQuery = ""
		}
	}
	return m, nil
}

func (m SidebarModel) updateCopyMode(msg tea.KeyMsg) (SidebarModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.copying = false
		m.copySource = ""
		m.copyInput = ""
	case "enter":
		name := strings.TrimSpace(m.copyInput)
		if name == "" {
			return m, nil
		}
		source := m.copySource
		m.copying = false
		m.copySource = ""
		m.copyInput = ""
		return m, func() tea.Msg {
			return CopyDatabaseMsg{Source: source, Target: name}
		}
	case "backspace":
		if len(m.copyInput) > 0 {
			m.copyInput = m.copyInput[:len(m.copyInput)-1]
		}
	case "ctrl+u":
		m.copyInput = ""
	default:
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			m.copyInput += msg.String()
		} else if msg.Type == tea.KeyRunes {
			m.copyInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m SidebarModel) updateDeleteConfirm(msg tea.KeyMsg) (SidebarModel, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.deleteTarget
		m.confirmDelete = false
		m.deleteTarget = ""
		return m, func() tea.Msg {
			return DeleteDatabaseMsg{Name: name}
		}
	default:
		m.confirmDelete = false
		m.deleteTarget = ""
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
			m.ensureVisible()
		}
	case "down":
		if m.cursor < len(m.filteredTables)-1 {
			m.cursor++
			m.ensureVisible()
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

func truncateDisplay(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return "..."[:maxWidth]
	}
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "..."
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return "..."
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
	linesUsed := 0

	if m.mode == SidebarDatabases {
		b.WriteString(HeaderStyle.Render("Databases"))
		b.WriteString("\n")
		linesUsed++

		if m.confirmDelete {
			b.WriteString(ErrorText.Render(fmt.Sprintf("  Drop %s?", m.deleteTarget)))
			b.WriteString("\n")
			linesUsed++
			b.WriteString(DimText.Render("  y confirm | any key cancel"))
			b.WriteString("\n")
			linesUsed++
		} else if m.copying {
			b.WriteString(AccentText.Render(fmt.Sprintf("  Copy %s as:", m.copySource)))
			b.WriteString("\n")
			linesUsed++
			copyDisp := "  " + SearchInput.Render(m.copyInput) + SearchInput.Render("█")
			b.WriteString(copyDisp)
			b.WriteString("\n")
			linesUsed++
			b.WriteString(DimText.Render("  Enter confirm | Esc cancel"))
			b.WriteString("\n")
			linesUsed++
		} else if m.searching || m.searchQuery != "" {
			searchDisp := SearchLabel.Render("/") + SearchInput.Render(m.searchQuery)
			if m.searching {
				searchDisp += SearchInput.Render("█")
			}
			b.WriteString(searchDisp)
			b.WriteString("\n")
			linesUsed++
		} else {
			b.WriteString(DimText.Render("  D tables | Enter switch | c copy | x drop"))
			b.WriteString("\n")
			linesUsed++
		}

		dbs := m.filteredDatabases
		if len(dbs) == 0 {
			if m.searchQuery != "" {
				b.WriteString(DimText.Render("  No matches"))
			} else {
				b.WriteString(DimText.Render("  No databases found"))
			}
			linesUsed++
		} else {
			availLines := innerH - linesUsed
			if len(dbs) > availLines {
				availLines--
			}
			if availLines < 1 {
				availLines = 1
			}
			startIdx := m.scrollOffset
			endIdx := startIdx + availLines
			if endIdx > len(dbs) {
				endIdx = len(dbs)
			}
			for i := startIdx; i < endIdx; i++ {
				d := dbs[i]
				label := truncateDisplay(fmt.Sprintf("⛁ %s", d), innerW-1)
				var line string
				if i == m.cursor && m.focused {
					line = SidebarCursorItem.Width(innerW).MaxHeight(1).Render(label)
				} else if d == m.activeDatabase {
					line = SidebarActiveItem.Width(innerW).MaxHeight(1).Render(label)
				} else {
					line = SidebarTableItem.Width(innerW).MaxHeight(1).Render(label)
				}
				b.WriteString(line)
				if i < endIdx-1 {
					b.WriteString("\n")
				}
				linesUsed++
			}
			if len(dbs) > availLines {
				b.WriteString("\n")
				b.WriteString(DimText.Render(fmt.Sprintf(" [%d-%d of %d]", startIdx+1, endIdx, len(dbs))))
				linesUsed++
			}
		}
	} else {
		// Header
		header := HeaderStyle.Render("Tables")
		b.WriteString(header)
		b.WriteString("\n")
		linesUsed++

		if m.searching || m.searchQuery != "" {
			searchDisp := SearchLabel.Render("/") + SearchInput.Render(m.searchQuery)
			if m.searching {
				searchDisp += SearchInput.Render("█")
			}
			b.WriteString(searchDisp)
			b.WriteString("\n")
			linesUsed++
		} else {
			dbName := m.activeDatabase
			if dbName == "" {
				dbName = "public"
			}
			schema := SubHeaderStyle.Render(fmt.Sprintf("  %s | D databases", dbName))
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
			availLines := innerH - linesUsed
			if len(tables) > availLines {
				availLines--
			}
			if availLines < 1 {
				availLines = 1
			}
			startIdx := m.scrollOffset
			endIdx := startIdx + availLines
			if endIdx > len(tables) {
				endIdx = len(tables)
			}
			for i := startIdx; i < endIdx; i++ {
				t := tables[i]
				label := truncateDisplay(fmt.Sprintf("T %s", t), innerW-1)
				var line string
				if i == m.cursor && m.focused {
					line = SidebarCursorItem.Width(innerW).MaxHeight(1).Render(label)
				} else if t == m.selected {
					line = SidebarActiveItem.Width(innerW).MaxHeight(1).Render(label)
				} else {
					line = SidebarTableItem.Width(innerW).MaxHeight(1).Render(label)
				}
				b.WriteString(line)
				if i < endIdx-1 {
					b.WriteString("\n")
				}
				linesUsed++
			}
			if len(tables) > availLines {
				b.WriteString("\n")
				b.WriteString(DimText.Render(fmt.Sprintf(" [%d-%d of %d]", startIdx+1, endIdx, len(tables))))
				linesUsed++
			}
		}
	}

	// Pad remaining lines
	for linesUsed < innerH {
		b.WriteString("\n")
		linesUsed++
	}

	content := lipgloss.NewStyle().Width(innerW).Height(innerH).MaxHeight(innerH).Render(b.String())
	return borderStyle.Width(innerW).Height(innerH).Render(content)
}
