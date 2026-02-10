package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/editor"
)

// EditBlockedMsg is sent when editing is not possible.
type EditBlockedMsg struct {
	Reason string
}

// ResultsModel is the interactive results table with CRUD support.
type ResultsModel struct {
	columns         []string
	columnTypes     []string
	rows            [][]string
	cursorRow       int
	cursorCol       int
	focused         bool
	editing         bool
	editValue       string
	changes         *editor.ChangeTracker
	tableName       string
	primaryKeys     []string
	scrollOffset    int
	colOffset       int
	width           int
	height          int
	colWidths       []int
	errMsg          string
	infoMsg         string
	bannerMsg       string
	insertedRows    int // count of locally inserted rows (at end of rows slice)
	searching       bool
	searchQuery     string
	filteredIndices []int
	searchCursor    int
	previewing      bool
	previewScroll   int
	previewEditing  bool
	previewTextarea textarea.Model
}

// NewResultsModel creates a new results model.
func NewResultsModel(changes *editor.ChangeTracker) ResultsModel {
	return ResultsModel{
		changes: changes,
	}
}

// SetFocused sets focus state.
func (m *ResultsModel) SetFocused(f bool) {
	m.focused = f
}

// Focused returns focus state.
func (m ResultsModel) Focused() bool {
	return m.focused
}

// SetSize sets the results dimensions.
func (m *ResultsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetData populates the results table with query output.
func (m *ResultsModel) SetData(columns []string, columnTypes []string, rows [][]string) {
	m.columns = columns
	m.columnTypes = columnTypes
	m.rows = rows
	m.cursorRow = 0
	m.cursorCol = 0
	m.scrollOffset = 0
	m.colOffset = 0
	m.editing = false
	m.editValue = ""
	m.errMsg = ""
	m.infoMsg = ""
	m.bannerMsg = ""
	m.insertedRows = 0
	m.calcColWidths()
}

// SetTableContext sets the current table name and PKs for CRUD.
func (m *ResultsModel) SetTableContext(tableName string, pks []string) {
	m.tableName = tableName
	m.primaryKeys = pks
}

// SetError shows an error message in the results pane.
func (m *ResultsModel) SetError(msg string) {
	m.errMsg = msg
	m.columns = nil
	m.rows = nil
	m.infoMsg = ""
}

// SetInfo shows an info message.
func (m *ResultsModel) SetInfo(msg string) {
	m.infoMsg = msg
	m.errMsg = ""
}

// SetBanner sets a highlighted banner message above the table.
func (m *ResultsModel) SetBanner(msg string) {
	m.bannerMsg = msg
}

// Clear resets the results pane to an empty state.
func (m *ResultsModel) Clear() {
	m.columns = nil
	m.columnTypes = nil
	m.rows = nil
	m.cursorRow = 0
	m.cursorCol = 0
	m.scrollOffset = 0
	m.colOffset = 0
	m.editing = false
	m.editValue = ""
	m.errMsg = ""
	m.infoMsg = ""
	m.bannerMsg = ""
	m.tableName = ""
	m.primaryKeys = nil
	m.insertedRows = 0
}

// ClearInsertedRows removes all locally inserted rows.
func (m *ResultsModel) ClearInsertedRows() {
	if m.insertedRows > 0 {
		m.rows = m.rows[:len(m.rows)-m.insertedRows]
		m.insertedRows = 0
		if m.cursorRow >= len(m.rows) && len(m.rows) > 0 {
			m.cursorRow = len(m.rows) - 1
		}
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.ensureRowVisible()
	}
}

// IsEditing returns whether we're in edit mode.
func (m ResultsModel) IsEditing() bool {
	return m.editing
}

// IsSearching returns whether we're in search mode.
func (m ResultsModel) IsSearching() bool {
	return m.searching
}

// IsPreviewing returns whether we're in cell preview mode.
func (m ResultsModel) IsPreviewing() bool {
	return m.previewing
}

func (m *ResultsModel) applyRowFilter() {
	if m.searchQuery == "" {
		m.filteredIndices = nil
		m.searchCursor = 0
		return
	}
	m.filteredIndices = nil
	for ri, row := range m.rows {
		for _, cell := range row {
			if FuzzyMatch(cell, m.searchQuery) {
				m.filteredIndices = append(m.filteredIndices, ri)
				break
			}
		}
	}
	m.searchCursor = 0
	if len(m.filteredIndices) > 0 {
		m.cursorRow = m.filteredIndices[0]
		m.ensureRowVisible()
	}
}

// HasPrimaryKey returns whether the current table has a PK.
func (m ResultsModel) HasPrimaryKey() bool {
	return len(m.primaryKeys) > 0
}

func (m *ResultsModel) calcColWidths() {
	if len(m.columns) == 0 {
		m.colWidths = nil
		return
	}
	m.colWidths = make([]int, len(m.columns))
	for i, col := range m.columns {
		w := len(col)
		if w < 10 {
			w = 10
		}
		for _, row := range m.rows {
			if i < len(row) && len(row[i]) > w {
				w = len(row[i])
			}
		}
		if w > 40 {
			w = 40
		}
		m.colWidths[i] = w
	}
}

// Init satisfies tea.Model.
func (m ResultsModel) Init() tea.Cmd {
	return nil
}

// Update handles key events.
func (m ResultsModel) Update(msg tea.Msg) (ResultsModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.previewing {
			return m.updatePreviewMode(msg)
		}
		if m.searching {
			return m.updateSearchMode(msg)
		}
		if m.editing {
			return m.updateEditMode(msg)
		}
		return m.updateNavMode(msg)
	}
	return m, nil
}

func (m ResultsModel) updateNavMode(msg tea.KeyMsg) (ResultsModel, tea.Cmd) {
	if len(m.rows) == 0 && msg.String() != "a" {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.cursorRow > 0 {
			m.cursorRow--
			m.ensureRowVisible()
		}
	case "down", "j":
		if m.cursorRow < len(m.rows)-1 {
			m.cursorRow++
			m.ensureRowVisible()
		}
	case "left", "h":
		if m.cursorCol > 0 {
			m.cursorCol--
			m.ensureColVisible()
		}
	case "right", "l":
		if m.cursorCol < len(m.columns)-1 {
			m.cursorCol++
			m.ensureColVisible()
		}
	case "e":
		if len(m.primaryKeys) == 0 && !m.isInsertedRow(m.cursorRow) {
			if m.tableName == "" {
				return m, func() tea.Msg {
					return EditBlockedMsg{Reason: "Cannot edit free-form query results"}
				}
			}
			return m, func() tea.Msg {
				return EditBlockedMsg{Reason: "Cannot edit: table has no primary key"}
			}
		}
		if len(m.rows) > 0 {
			m.editing = true
			m.editValue = m.displayValue(m.cursorRow, m.cursorCol)
			if m.editValue == "<NULL>" {
				m.editValue = ""
			}
		}
	case "d":
		if len(m.primaryKeys) == 0 {
			return m, nil
		}
		if len(m.rows) > 0 && !m.isInsertedRow(m.cursorRow) {
			pkVals := m.pkValues(m.cursorRow)
			if m.changes.IsRowDeleted(m.tableName, pkVals) {
				m.changes.UnstageDelete(m.tableName, pkVals)
			} else {
				m.changes.StageDelete(editor.RowDelete{
					TableName:   m.tableName,
					RowPKValues: pkVals,
				})
			}
		}
	case "a":
		if len(m.primaryKeys) == 0 && m.tableName != "" {
			return m, nil
		}
		if len(m.columns) > 0 {
			newRow := make([]string, len(m.columns))
			for i := range newRow {
				newRow[i] = ""
			}
			m.rows = append(m.rows, newRow)
			m.insertedRows++
			m.cursorRow = len(m.rows) - 1
			m.cursorCol = 0
			m.ensureRowVisible()
			// Enter edit mode on first cell
			m.editing = true
			m.editValue = ""
		}
	case "ctrl+z":
		m.changes.Undo()
	case "g":
		m.cursorRow = 0
		m.scrollOffset = 0
	case "G":
		if len(m.rows) > 0 {
			m.cursorRow = len(m.rows) - 1
			m.ensureRowVisible()
		}
	case "pgup":
		visibleRows := m.visibleRowCount()
		m.cursorRow -= visibleRows
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.ensureRowVisible()
	case "pgdown":
		visibleRows := m.visibleRowCount()
		m.cursorRow += visibleRows
		if m.cursorRow >= len(m.rows) {
			m.cursorRow = len(m.rows) - 1
		}
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		m.ensureRowVisible()
	case "/":
		m.searching = true
		m.searchQuery = ""
		m.filteredIndices = nil
		m.searchCursor = 0
	case "n":
		if len(m.filteredIndices) > 0 {
			m.searchCursor++
			if m.searchCursor >= len(m.filteredIndices) {
				m.searchCursor = 0
			}
			m.cursorRow = m.filteredIndices[m.searchCursor]
			m.ensureRowVisible()
		}
	case "N":
		if len(m.filteredIndices) > 0 {
			m.searchCursor--
			if m.searchCursor < 0 {
				m.searchCursor = len(m.filteredIndices) - 1
			}
			m.cursorRow = m.filteredIndices[m.searchCursor]
			m.ensureRowVisible()
		}
	case "v":
		if len(m.rows) > 0 && len(m.columns) > 0 {
			val := m.displayValue(m.cursorRow, m.cursorCol)
			if val == "<NULL>" {
				val = ""
			}
			m.previewing = true
			m.previewScroll = 0
			m.previewEditing = false
			ta := textarea.New()
			ta.SetValue(val)
			ta.CharLimit = 0
			ta.ShowLineNumbers = true
			pw := m.width - 6
			if pw < 20 {
				pw = 20
			}
			ph := m.height - 8
			if ph < 4 {
				ph = 4
			}
			ta.SetWidth(pw)
			ta.SetHeight(ph)
			m.previewTextarea = ta
		}
	}
	return m, nil
}

func (m ResultsModel) updateSearchMode(msg tea.KeyMsg) (ResultsModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.searchQuery = ""
		m.filteredIndices = nil
		m.searchCursor = 0
	case "enter":
		m.searching = false
		if len(m.filteredIndices) > 0 {
			m.cursorRow = m.filteredIndices[m.searchCursor]
			m.ensureRowVisible()
		}
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.applyRowFilter()
		}
	default:
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			m.searchQuery += msg.String()
			m.applyRowFilter()
		} else if msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.applyRowFilter()
		}
	}
	return m, nil
}

func (m ResultsModel) updatePreviewMode(msg tea.KeyMsg) (ResultsModel, tea.Cmd) {
	if m.previewEditing {
		switch msg.String() {
		case "esc":
			m.previewEditing = false
			m.previewTextarea.Blur()
			return m, nil
		case "ctrl+s":
			m.editValue = m.previewTextarea.Value()
			m = m.commitCurrentCell()
			m.previewing = false
			m.previewEditing = false
			return m, nil
		}
		var cmd tea.Cmd
		m.previewTextarea, cmd = m.previewTextarea.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc", "v":
		m.previewing = false
		m.previewScroll = 0
	case "e":
		if len(m.primaryKeys) == 0 && !m.isInsertedRow(m.cursorRow) {
			if m.tableName == "" {
				return m, func() tea.Msg {
					return EditBlockedMsg{Reason: "Cannot edit free-form query results"}
				}
			}
			return m, func() tea.Msg {
				return EditBlockedMsg{Reason: "Cannot edit: table has no primary key"}
			}
		}
		m.previewEditing = true
		cmd := m.previewTextarea.Focus()
		return m, cmd
	case "j", "down":
		m.previewScroll++
	case "k", "up":
		if m.previewScroll > 0 {
			m.previewScroll--
		}
	case "G":
		m.previewScroll = 99999
	case "g":
		m.previewScroll = 0
	}
	return m, nil
}

func (m ResultsModel) renderPreviewOverlay(w, h int) string {
	var b strings.Builder

	colName := ""
	if m.cursorCol < len(m.columns) {
		colName = m.columns[m.cursorCol]
	}

	if m.previewEditing {
		title := HeaderStyle.Render(fmt.Sprintf("Edit: %s", colName))
		hint := DimText.Render("Ctrl+S save | Esc cancel")
		b.WriteString(title + "  " + hint)
		b.WriteString("\n")
		b.WriteString(m.previewTextarea.View())
	} else {
		title := HeaderStyle.Render(fmt.Sprintf("Preview: %s [row %d]", colName, m.cursorRow+1))
		hint := DimText.Render("e edit | j/k scroll | Esc close")
		b.WriteString(title + "  " + hint)
		b.WriteString("\n")
		b.WriteString(DimText.Render(strings.Repeat("─", w)))
		b.WriteString("\n")

		val := m.displayValue(m.cursorRow, m.cursorCol)
		lines := strings.Split(wordWrap(val, w), "\n")

		viewH := h - 4
		if viewH < 1 {
			viewH = 1
		}

		scroll := m.previewScroll
		maxScroll := len(lines) - viewH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if scroll > maxScroll {
			scroll = maxScroll
		}

		endLine := scroll + viewH
		if endLine > len(lines) {
			endLine = len(lines)
		}

		for i := scroll; i < endLine; i++ {
			b.WriteString(lines[i])
			if i < endLine-1 {
				b.WriteString("\n")
			}
		}

		if len(lines) > viewH {
			b.WriteString("\n")
			b.WriteString(DimText.Render(fmt.Sprintf("[lines %d-%d of %d]", scroll+1, endLine, len(lines))))
		}
	}

	return b.String()
}

func wordWrap(s string, width int) string {
	if width <= 0 || len(s) == 0 {
		return s
	}
	var result strings.Builder
	for li, line := range strings.Split(s, "\n") {
		if li > 0 {
			result.WriteString("\n")
		}
		for len(line) > width {
			result.WriteString(line[:width])
			result.WriteString("\n")
			line = line[width:]
		}
		result.WriteString(line)
	}
	return result.String()
}

func (m ResultsModel) commitCurrentCell() ResultsModel {
	newValue := m.editValue
	if newValue == "" {
		newValue = "<NULL>"
	}

	if m.isInsertedRow(m.cursorRow) {
		m.rows[m.cursorRow][m.cursorCol] = newValue
	} else {
		pkVals := m.pkValues(m.cursorRow)
		m.changes.StageEdit(editor.CellEdit{
			TableName:   m.tableName,
			RowPKValues: pkVals,
			ColumnName:  m.columns[m.cursorCol],
			OldValue:    m.rows[m.cursorRow][m.cursorCol],
			NewValue:    newValue,
		})
	}
	return m
}

func (m ResultsModel) moveToEditCell(col int) ResultsModel {
	m.cursorCol = col
	m.ensureColVisible()
	val := m.displayValue(m.cursorRow, m.cursorCol)
	if val == "<NULL>" {
		val = ""
	}
	m.editValue = val
	return m
}

func (m ResultsModel) updateEditMode(msg tea.KeyMsg) (ResultsModel, tea.Cmd) {
	switch msg.String() {
	case "enter", "tab":
		m = m.commitCurrentCell()
		if m.cursorCol < len(m.columns)-1 {
			m = m.moveToEditCell(m.cursorCol + 1)
		} else {
			m.editing = false
		}
	case "shift+tab":
		m = m.commitCurrentCell()
		if m.cursorCol > 0 {
			m = m.moveToEditCell(m.cursorCol - 1)
		}
	case "esc":
		m.editing = false
		m.editValue = ""
	case "backspace":
		if len(m.editValue) > 0 {
			m.editValue = m.editValue[:len(m.editValue)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			m.editValue += msg.String()
		} else if msg.Type == tea.KeyRunes {
			m.editValue += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *ResultsModel) ensureRowVisible() {
	visRows := m.visibleRowCount()
	if m.cursorRow < m.scrollOffset {
		m.scrollOffset = m.cursorRow
	} else if m.cursorRow >= m.scrollOffset+visRows {
		m.scrollOffset = m.cursorRow - visRows + 1
	}
}

func (m *ResultsModel) ensureColVisible() {
	// Simple horizontal scrolling: keep cursor column visible
	if m.cursorCol < m.colOffset {
		m.colOffset = m.cursorCol
	}
	// Check if cursor column fits within visible area
	usedWidth := 0
	for i := m.colOffset; i <= m.cursorCol && i < len(m.colWidths); i++ {
		usedWidth += m.colWidths[i] + 3 // +3 for padding/separator
	}
	innerW := m.width - 4 // borders + margin
	for usedWidth > innerW && m.colOffset < m.cursorCol {
		usedWidth -= m.colWidths[m.colOffset] + 3
		m.colOffset++
	}
}

func (m ResultsModel) visibleRowCount() int {
	// Available height minus border (2) + header row (1) + separator (1)
	h := m.height - 6
	if h < 1 {
		h = 1
	}
	return h
}

func (m ResultsModel) isInsertedRow(rowIdx int) bool {
	return rowIdx >= len(m.rows)-m.insertedRows && m.insertedRows > 0
}

func (m ResultsModel) pkValues(rowIdx int) map[string]string {
	vals := make(map[string]string)
	for _, pk := range m.primaryKeys {
		for i, col := range m.columns {
			if col == pk && rowIdx < len(m.rows) && i < len(m.rows[rowIdx]) {
				vals[pk] = m.rows[rowIdx][i]
			}
		}
	}
	return vals
}

func (m ResultsModel) displayValue(rowIdx, colIdx int) string {
	if rowIdx >= len(m.rows) || colIdx >= len(m.rows[rowIdx]) {
		return ""
	}
	// Check for staged edits first
	if !m.isInsertedRow(rowIdx) && len(m.primaryKeys) > 0 {
		pkVals := m.pkValues(rowIdx)
		if newVal, ok := m.changes.GetCellEdit(m.tableName, pkVals, m.columns[colIdx]); ok {
			return newVal
		}
	}
	return m.rows[rowIdx][colIdx]
}

// GetInsertedRowValues returns staged insert values for all locally added rows.
func (m ResultsModel) GetInsertedRowValues() []editor.RowInsert {
	if m.insertedRows == 0 {
		return nil
	}
	var inserts []editor.RowInsert
	startIdx := len(m.rows) - m.insertedRows
	for i := startIdx; i < len(m.rows); i++ {
		vals := make(map[string]string)
		for j, col := range m.columns {
			if j < len(m.rows[i]) {
				val := m.rows[i][j]
				if val == "" {
					val = "<NULL>"
				}
				vals[col] = val
			}
		}
		if len(vals) > 0 {
			inserts = append(inserts, editor.RowInsert{
				TableName: m.tableName,
				Values:    vals,
			})
		}
	}
	return inserts
}

// View renders the results table.
func (m ResultsModel) View() string {
	borderStyle := UnfocusedBorder
	if m.focused {
		borderStyle = FocusedBorder
	}

	innerW := m.width - 2
	if innerW < 10 {
		innerW = 10
	}
	innerH := m.height - 2
	if innerH < 3 {
		innerH = 3
	}

	var content string

	if m.previewing {
		content = m.renderPreviewOverlay(innerW, innerH)
	} else if m.errMsg != "" {
		content = ErrorText.Render(m.errMsg)
	} else if len(m.columns) == 0 {
		msg := "No rows returned"
		if m.infoMsg != "" {
			msg = m.infoMsg
		}
		content = DimText.Render(msg)
	} else {
		content = m.renderTable(innerW, innerH)
	}

	return borderStyle.Width(innerW).Height(innerH).MaxHeight(innerH + 2).Render(content)
}

func (m ResultsModel) isMatchRow(rowIdx int) bool {
	for _, fi := range m.filteredIndices {
		if fi == rowIdx {
			return true
		}
	}
	return false
}

func (m ResultsModel) renderTable(w, h int) string {
	if len(m.columns) == 0 {
		return ""
	}

	var b strings.Builder

	if m.searching || m.searchQuery != "" {
		searchDisp := SearchLabel.Render("/") + SearchInput.Render(m.searchQuery)
		if m.searching {
			searchDisp += SearchInput.Render("█")
		}
		if len(m.filteredIndices) > 0 {
			searchDisp += DimText.Render(fmt.Sprintf(" [%d/%d]", m.searchCursor+1, len(m.filteredIndices)))
		} else if m.searchQuery != "" {
			searchDisp += DimText.Render(" [no matches]")
		}
		b.WriteString(searchDisp)
		b.WriteString("\n")
		h--
	}

	if m.bannerMsg != "" {
		b.WriteString(BannerText.Render(m.bannerMsg))
		b.WriteString("\n")
		h--
	}

	// Determine visible columns
	visibleCols := m.visibleColumns(w)

	// Header
	headerParts := make([]string, 0, len(visibleCols))
	for _, ci := range visibleCols {
		colW := m.colWidths[ci]
		name := m.columns[ci]
		headerParts = append(headerParts, HeaderStyle.Width(colW).Render(truncate(name, colW)))
	}
	b.WriteString(strings.Join(headerParts, " | "))
	b.WriteString("\n")

	// Separator
	sepParts := make([]string, 0, len(visibleCols))
	for _, ci := range visibleCols {
		sepParts = append(sepParts, strings.Repeat("─", m.colWidths[ci]))
	}
	b.WriteString(DimText.Render(strings.Join(sepParts, "─┼─")))
	b.WriteString("\n")

	// Data rows
	visRows := h - 3 // header + sep + padding
	if visRows < 1 {
		visRows = 1
	}

	startRow := m.scrollOffset
	endRow := startRow + visRows
	if endRow > len(m.rows) {
		endRow = len(m.rows)
	}

	for ri := startRow; ri < endRow; ri++ {
		rowParts := make([]string, 0, len(visibleCols))
		for _, ci := range visibleCols {
			val := m.displayValue(ri, ci)
			colW := m.colWidths[ci]
			truncVal := truncate(sanitizeCell(val), colW)

			var style lipgloss.Style

			// Determine cell style
			isCursor := ri == m.cursorRow && ci == m.cursorCol && m.focused

			if m.editing && isCursor {
				// Show edit buffer with cursor
				editDisp := m.editValue + "█"
				truncEdit := truncate(editDisp, colW)
				style = CellEditing
				rowParts = append(rowParts, style.Width(colW).Render(truncEdit))
				continue
			}

			isInserted := m.isInsertedRow(ri)
			isDeleted := false
			isModified := false

			if !isInserted && len(m.primaryKeys) > 0 {
				pkVals := m.pkValues(ri)
				isDeleted = m.changes.IsRowDeleted(m.tableName, pkVals)
				_, isModified = m.changes.GetCellEdit(m.tableName, pkVals, m.columns[ci])
			}

			isMatch := len(m.filteredIndices) > 0 && m.isMatchRow(ri)

			switch {
			case isCursor:
				style = CellSelected
			case isDeleted:
				style = DeletedText
			case isInserted:
				style = NewRowText
			case isModified:
				style = ModifiedText
			case isMatch:
				style = SearchInput
			case val == "<NULL>":
				style = NullText
			default:
				style = CellNormal
			}

			rowParts = append(rowParts, style.Width(colW).Render(truncVal))
		}
		b.WriteString(strings.Join(rowParts, " | "))
		if ri < endRow-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(m.rows) > visRows {
		scrollInfo := fmt.Sprintf(" [%d-%d of %d]", startRow+1, endRow, len(m.rows))
		b.WriteString("\n" + DimText.Render(scrollInfo))
	}

	return b.String()
}

func (m ResultsModel) visibleColumns(availWidth int) []int {
	if len(m.colWidths) == 0 {
		return nil
	}
	var cols []int
	usedWidth := 0
	for i := m.colOffset; i < len(m.colWidths); i++ {
		needed := m.colWidths[i]
		if len(cols) > 0 {
			needed += 3 // " | " separator
		}
		if usedWidth+needed > availWidth && len(cols) > 0 {
			break
		}
		cols = append(cols, i)
		usedWidth += needed
	}
	return cols
}

func sanitizeCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "↵")
	s = strings.ReplaceAll(s, "\n", "↵")
	s = strings.ReplaceAll(s, "\r", "↵")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
