package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/editor"
)

// ResultsModel is the interactive results table with CRUD support.
type ResultsModel struct {
	columns      []string
	columnTypes  []string
	rows         [][]string
	cursorRow    int
	cursorCol    int
	focused      bool
	editing      bool
	editValue    string
	changes      *editor.ChangeTracker
	tableName    string
	primaryKeys  []string
	scrollOffset int
	colOffset    int
	width        int
	height       int
	colWidths    []int
	errMsg       string
	infoMsg      string
	bannerMsg    string
	insertedRows int // count of locally inserted rows (at end of rows slice)
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

// IsEditing returns whether we're in edit mode.
func (m ResultsModel) IsEditing() bool {
	return m.editing
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
			return m, nil
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
	}
	return m, nil
}

func (m ResultsModel) updateEditMode(msg tea.KeyMsg) (ResultsModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Confirm edit
		m.editing = false
		if m.isInsertedRow(m.cursorRow) {
			// Update the local row data directly for inserts
			m.rows[m.cursorRow][m.cursorCol] = m.editValue
		} else {
			// Stage an edit for existing rows
			pkVals := m.pkValues(m.cursorRow)
			m.changes.StageEdit(editor.CellEdit{
				TableName:   m.tableName,
				RowPKValues: pkVals,
				ColumnName:  m.columns[m.cursorCol],
				OldValue:    m.rows[m.cursorRow][m.cursorCol],
				NewValue:    m.editValue,
			})
		}
	case "esc":
		m.editing = false
		m.editValue = ""
	case "backspace":
		if len(m.editValue) > 0 {
			m.editValue = m.editValue[:len(m.editValue)-1]
		}
	default:
		// Only add printable characters
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
			if j < len(m.rows[i]) && m.rows[i][j] != "" {
				vals[col] = m.rows[i][j]
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

	if m.errMsg != "" {
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

	return borderStyle.Width(innerW).Height(innerH).Render(content)
}

func (m ResultsModel) renderTable(w, h int) string {
	if len(m.columns) == 0 {
		return ""
	}

	var b strings.Builder

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
			truncVal := truncate(val, colW)

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

			switch {
			case isCursor:
				style = CellSelected
			case isDeleted:
				style = DeletedText
			case isInserted:
				style = NewRowText
			case isModified:
				style = ModifiedText
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
