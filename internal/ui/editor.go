package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExecuteQueryMsg is sent when the user executes a query with Ctrl+E.
type ExecuteQueryMsg struct {
	SQL string
}

var GhostStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))

type ghostCandidate struct {
	full    string
	suffix  string
	partial int
}

// VimMode represents the current vim editing mode.
type VimMode int

const (
	VimNormal VimMode = iota
	VimInsert
)

// EditorModel wraps a textarea for SQL editing.
type EditorModel struct {
	textarea        textarea.Model
	focused         bool
	width           int
	height          int
	ghost           string
	ghostFull       string
	ghostPartialLen int
	ghostMatches    []ghostCandidate
	ghostIndex      int
	tableNames      []string
	columnNames     []string
	columnCache     map[string][]string // table → columns, loaded once
	visualMode      bool
	visualStartRow  int
	visualStartCol  int
	vimMode         VimMode
	vimPending      string // for multi-key commands like dd, dw, etc.
}

// SetTableNames updates the list of table names used for autocomplete.
func (m *EditorModel) SetTableNames(names []string) {
	m.tableNames = names
}

// SetColumnNames updates the list of column names for autocomplete in SELECT/WHERE contexts.
func (m *EditorModel) SetColumnNames(names []string) {
	m.columnNames = names
}

// SetColumnCache sets the full schema cache (table → column names).
// When set, the editor can resolve column names locally without DB queries.
func (m *EditorModel) SetColumnCache(cache map[string][]string) {
	m.columnCache = cache
}

// LookupColumns returns cached column names for the given table, or nil if not cached.
func (m *EditorModel) LookupColumns(table string) []string {
	if m.columnCache == nil {
		return nil
	}
	return m.columnCache[strings.ToLower(table)]
}

// NewEditorModel creates a new SQL editor.
func NewEditorModel() EditorModel {
	ta := textarea.New()
	ta.Placeholder = "Press i to enter INSERT mode... (Ctrl+J run, Ctrl+E run all)"
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.Prompt = "  "
	ta.SetWidth(40)
	ta.SetHeight(5)
	return EditorModel{
		textarea: ta,
	}
}

// SetFocused sets focus state.
func (m *EditorModel) SetFocused(f bool) {
	m.focused = f
	if f {
		m.textarea.Focus()
	} else {
		m.textarea.Blur()
		m.visualMode = false
	}
}

// Focused returns the focus state.
func (m EditorModel) Focused() bool {
	return m.focused
}

// IsInsertMode returns whether the editor is in vim insert mode.
func (m EditorModel) IsInsertMode() bool {
	return m.vimMode == VimInsert
}

// VimModeString returns a display string for the current vim mode.
func (m EditorModel) VimModeString() string {
	switch m.vimMode {
	case VimInsert:
		return "INSERT"
	default:
		return "NORMAL"
	}
}

// HasGhost returns whether a ghost completion is active.
func (m EditorModel) HasGhost() bool {
	return m.ghost != ""
}

// SetSize sets the editor dimensions.
func (m *EditorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	// Account for border (2) and header (1)
	innerW := w - 2
	innerH := h - 4
	if innerW < 10 {
		innerW = 10
	}
	if innerH < 2 {
		innerH = 2
	}
	m.textarea.SetWidth(innerW)
	m.textarea.SetHeight(innerH)
}

// Init satisfies the tea.Model interface.
func (m EditorModel) Init() tea.Cmd {
	return nil
}

// Update handles key events.
func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ctrl shortcuts work in both modes
		switch msg.String() {
		case "ctrl+c":
			if m.visualMode {
				m.copySelection()
				m.visualMode = false
			}
			return m, nil
		case "ctrl+y":
			text := m.textarea.Value()
			clipboard.WriteAll(text)
			return m, nil
		case "ctrl+v":
			if clipText, err := clipboard.ReadAll(); err == nil && clipText != "" {
				m.textarea.InsertString(clipText)
				m.updateGhost()
			}
			return m, nil
		case "ctrl+j":
			sql := m.statementAtCursor()
			if sql == "" {
				return m, nil
			}
			m.clearGhost()
			return m, func() tea.Msg {
				return ExecuteQueryMsg{SQL: sql}
			}
		case "ctrl+e":
			sql := strings.TrimSpace(m.textarea.Value())
			if sql == "" {
				return m, nil
			}
			formatted := FormatSQL(sql)
			m.textarea.Reset()
			m.textarea.InsertString(formatted)
			m.clearGhost()
			return m, func() tea.Msg {
				return ExecuteQueryMsg{SQL: sql}
			}
		}

		if m.vimMode == VimNormal {
			return m.updateNormalMode(msg)
		}
		return m.updateInsertMode(msg)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.updateGhost()
	return m, cmd
}

// updateInsertMode handles key events in vim insert mode.
func (m EditorModel) updateInsertMode(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.vimMode = VimNormal
		m.visualMode = false
		m.clearGhost()
		// Move cursor left one (vim behavior on leaving insert)
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyLeft})
		return m, nil
	case "shift+up", "shift+down", "shift+left", "shift+right", "shift+home", "shift+end":
		m.handleShiftArrow(msg)
		movementKey := shiftToMovementKey(msg)
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(movementKey)
		m.updateGhost()
		return m, cmd
	case "tab":
		if m.ghost != "" {
			for i := 0; i < m.ghostPartialLen; i++ {
				m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyBackspace})
			}
			m.textarea.InsertString(m.ghostFull)
			m.clearGhost()
			m.updateGhost()
			return m, nil
		}
		m.textarea.InsertString("  ")
		m.updateGhost()
		return m, nil
	case "up":
		if len(m.ghostMatches) > 1 {
			m.ghostIndex--
			if m.ghostIndex < 0 {
				m.ghostIndex = len(m.ghostMatches) - 1
			}
			m.applyGhostIndex()
			return m, nil
		}
	case "down":
		if len(m.ghostMatches) > 1 {
			m.ghostIndex++
			if m.ghostIndex >= len(m.ghostMatches) {
				m.ghostIndex = 0
			}
			m.applyGhostIndex()
			return m, nil
		}
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 && m.visualMode {
		m.visualMode = false
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.updateGhost()
	return m, cmd
}

// updateNormalMode handles key events in vim normal mode.
func (m EditorModel) updateNormalMode(msg tea.KeyMsg) (EditorModel, tea.Cmd) {
	key := msg.String()

	// Handle pending multi-key commands (dd, dw, etc.)
	if m.vimPending != "" {
		return m.handlePendingVimCmd(key)
	}

	switch key {
	// Enter insert mode
	case "i":
		m.vimMode = VimInsert
		m.textarea.Focus()
		return m, nil
	case "a":
		m.vimMode = VimInsert
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		return m, nil
	case "I":
		m.vimMode = VimInsert
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
		return m, nil
	case "A":
		m.vimMode = VimInsert
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnd})
		return m, nil
	case "o":
		m.vimMode = VimInsert
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnd})
		m.textarea.InsertString("\n")
		return m, nil
	case "O":
		m.vimMode = VimInsert
		m.textarea.Focus()
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
		m.textarea.InsertString("\n")
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyUp})
		return m, nil

	// Movement
	case "h", "left":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyLeft})
		return m, nil
	case "l", "right":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		return m, nil
	case "j", "down":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDown})
		return m, nil
	case "k", "up":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyUp})
		return m, nil
	case "0":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
		return m, nil
	case "$":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnd})
		return m, nil
	case "w":
		m.vimWordForward()
		return m, nil
	case "b":
		m.vimWordBackward()
		return m, nil
	case "g":
		// gg - go to top
		m.vimPending = "g"
		return m, nil
	case "G":
		// G - go to bottom
		lines := strings.Split(m.textarea.Value(), "\n")
		for i := 0; i < len(lines); i++ {
			m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
		return m, nil

	// Editing in normal mode
	case "x":
		// Delete char under cursor
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDelete})
		return m, nil
	case "d":
		m.vimPending = "d"
		return m, nil
	case "p":
		if clipText, err := clipboard.ReadAll(); err == nil && clipText != "" {
			m.textarea.InsertString(clipText)
		}
		return m, nil

	case "u":
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
		return m, nil

	case "esc":
		m.visualMode = false
		m.vimPending = ""
		return m, nil
	}

	return m, nil
}

// handlePendingVimCmd handles the second key of multi-key vim commands.
func (m EditorModel) handlePendingVimCmd(key string) (EditorModel, tea.Cmd) {
	pending := m.vimPending
	m.vimPending = ""

	switch pending {
	case "d":
		switch key {
		case "d":
			// dd - delete current line
			m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
			line := m.currentLine()
			// Select entire line content and delete it
			for i := 0; i < len([]rune(line)); i++ {
				m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDelete})
			}
			// Delete the newline too if possible
			m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDelete})
			return m, nil
		case "w":
			// dw - delete word
			m.vimDeleteWord()
			return m, nil
		}
	case "g":
		switch key {
		case "g":
			// gg - go to top
			for i := 0; i < 10000; i++ {
				line := m.textarea.Line()
				m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyUp})
				if m.textarea.Line() == line {
					break
				}
			}
			m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
			return m, nil
		}
	}

	return m, nil
}

// currentLine returns the text of the current cursor line.
func (m EditorModel) currentLine() string {
	lines := strings.Split(m.textarea.Value(), "\n")
	cur := m.textarea.Line()
	if cur < len(lines) {
		return lines[cur]
	}
	return ""
}

// vimWordForward moves the cursor forward one word.
func (m *EditorModel) vimWordForward() {
	text := m.textarea.Value()
	lines := strings.Split(text, "\n")
	curLine := m.textarea.Line()
	col := m.textarea.LineInfo().ColumnOffset

	if curLine >= len(lines) {
		return
	}
	line := []rune(lines[curLine])

	// Skip current word chars
	for col < len(line) && !unicode.IsSpace(line[col]) {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		col++
	}
	// Skip whitespace
	for col < len(line) && unicode.IsSpace(line[col]) {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyRight})
		col++
	}
	// If at end of line, move to next line
	if col >= len(line) && curLine < len(lines)-1 {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDown})
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyHome})
	}
}

// vimWordBackward moves the cursor backward one word.
func (m *EditorModel) vimWordBackward() {
	col := m.textarea.LineInfo().ColumnOffset

	if col == 0 {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyUp})
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnd})
		return
	}

	text := m.textarea.Value()
	lines := strings.Split(text, "\n")
	curLine := m.textarea.Line()
	if curLine >= len(lines) {
		return
	}
	line := []rune(lines[curLine])

	// Skip whitespace backward
	for col > 0 && col <= len(line) && unicode.IsSpace(line[col-1]) {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyLeft})
		col--
	}
	// Skip word chars backward
	for col > 0 && col <= len(line) && !unicode.IsSpace(line[col-1]) {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyLeft})
		col--
	}
}

// vimDeleteWord deletes from cursor to start of next word.
func (m *EditorModel) vimDeleteWord() {
	text := m.textarea.Value()
	lines := strings.Split(text, "\n")
	curLine := m.textarea.Line()
	col := m.textarea.LineInfo().ColumnOffset

	if curLine >= len(lines) {
		return
	}
	line := []rune(lines[curLine])
	startCol := col

	// Count chars to delete (word + trailing space)
	for col < len(line) && !unicode.IsSpace(line[col]) {
		col++
	}
	for col < len(line) && unicode.IsSpace(line[col]) {
		col++
	}

	count := col - startCol
	for i := 0; i < count; i++ {
		m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyDelete})
	}
}

// Value returns the current editor text.
func (m EditorModel) Value() string {
	return m.textarea.Value()
}

// SetValue replaces the editor content with the given text.
func (m *EditorModel) SetValue(s string) {
	m.textarea.Reset()
	m.textarea.InsertString(s)
	m.clearGhost()
}

func (m *EditorModel) clearGhost() {
	m.ghost = ""
	m.ghostFull = ""
	m.ghostPartialLen = 0
	m.ghostMatches = nil
	m.ghostIndex = 0
}

func (m *EditorModel) handleShiftArrow(msg tea.KeyMsg) {
	if !m.visualMode {
		m.visualMode = true
		m.visualStartRow = m.textarea.Line()
		m.visualStartCol = m.textarea.LineInfo().ColumnOffset
	}
}

func shiftToMovementKey(msg tea.KeyMsg) tea.KeyMsg {
	switch msg.String() {
	case "shift+up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "shift+down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "shift+left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "shift+right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "shift+home":
		return tea.KeyMsg{Type: tea.KeyHome}
	case "shift+end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	default:
		return msg
	}
}

func (m *EditorModel) copySelection() {
	lines := strings.Split(m.textarea.Value(), "\n")
	if len(lines) == 0 {
		return
	}
	cursorLine := m.textarea.Line()
	cursorCol := m.textarea.LineInfo().ColumnOffset
	selStartRow := m.visualStartRow
	selStartCol := m.visualStartCol
	selEndRow := cursorLine
	selEndCol := cursorCol
	if selStartRow > selEndRow || (selStartRow == selEndRow && selStartCol > selEndCol) {
		selStartRow, selEndRow = selEndRow, selStartRow
		selStartCol, selEndCol = selEndCol, selStartCol
	}
	var sb strings.Builder
	for row := selStartRow; row <= selEndRow; row++ {
		if row >= len(lines) {
			break
		}
		line := lines[row]
		runes := []rune(line)
		startCol := 0
		endCol := len(runes)
		if row == selStartRow {
			startCol = selStartCol
		}
		if row == selEndRow {
			endCol = selEndCol
		}
		if startCol > len(runes) {
			startCol = len(runes)
		}
		if endCol > len(runes) {
			endCol = len(runes)
		}
		if startCol < endCol {
			sb.WriteString(string(runes[startCol:endCol]))
		}
		if row < selEndRow {
			sb.WriteByte('\n')
		}
	}
	if sb.Len() > 0 {
		clipboard.WriteAll(sb.String())
	}
}

func (m *EditorModel) applyGhostIndex() {
	c := m.ghostMatches[m.ghostIndex]
	m.ghost = c.suffix
	m.ghostFull = c.full
	m.ghostPartialLen = c.partial
}

func (m *EditorModel) isInColumnContext(lines []string, cursorLine int, line string, partialStart int) bool {
	if len(m.columnNames) == 0 {
		return false
	}
	var before strings.Builder
	for i := 0; i < cursorLine && i < len(lines); i++ {
		before.WriteString(lines[i])
		before.WriteByte('\n')
	}
	if partialStart > 0 && partialStart <= len(line) {
		before.WriteString(line[:partialStart])
	}
	trimmed := strings.TrimRight(before.String(), " \t\n")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	trimmed = strings.ReplaceAll(trimmed, "\t", " ")
	for strings.Contains(trimmed, "  ") {
		trimmed = strings.ReplaceAll(trimmed, "  ", " ")
	}
	if len(trimmed) == 0 {
		return false
	}
	if strings.HasSuffix(trimmed, ".") || strings.HasSuffix(strings.TrimRight(trimmed, " "), ",") {
		return true
	}
	trimmed += " "
	beforeUpper := strings.ToUpper(trimmed)
	trims := []string{"SELECT ", "SELECT DISTINCT ", "WHERE ", " AND ", " OR ", " ON ", "ORDER BY ", "GROUP BY ", "HAVING ", "SET ", "RETURNING "}
	for _, t := range trims {
		if strings.HasSuffix(beforeUpper, t) {
			return true
		}
	}
	return false
}

func (m *EditorModel) updateGhost() {
	text := m.textarea.Value()
	lines := strings.Split(text, "\n")
	cursorLine := m.textarea.Line()

	if cursorLine >= len(lines) {
		m.clearGhost()
		return
	}

	line := lines[cursorLine]
	li := m.textarea.LineInfo()
	col := li.ColumnOffset

	if col == 0 || col > len(line) {
		m.clearGhost()
		return
	}

	end := col
	start := end
	for start > 0 && start <= len(line) {
		ch := rune(line[start-1])
		if unicode.IsLetter(ch) || ch == '_' {
			start--
		} else {
			break
		}
	}

	if start == end {
		m.clearGhost()
		return
	}

	if end < len(line) {
		next := rune(line[end])
		if unicode.IsLetter(next) || next == '_' {
			m.clearGhost()
			return
		}
	}

	partial := strings.ToUpper(line[start:end])
	partialLower := strings.ToLower(line[start:end])
	pLen := end - start

	inColumnContext := m.isInColumnContext(lines, cursorLine, line, start)
	minLen := 2
	if inColumnContext {
		minLen = 1
	}
	if len(partial) < minLen {
		m.clearGhost()
		return
	}

	var matches []ghostCandidate

	if inColumnContext {
		for _, cn := range m.columnNames {
			lower := strings.ToLower(cn)
			if strings.HasPrefix(lower, partialLower) && lower != partialLower {
				matches = append(matches, ghostCandidate{
					full:    cn,
					suffix:  cn[len(partialLower):],
					partial: pLen,
				})
			}
		}
	}

	if len(matches) == 0 {
		allKeywords := append(sqlKeywords, sqlFunctions...)
		for _, kw := range allKeywords {
			if strings.HasPrefix(kw, partial) && kw != partial {
				matches = append(matches, ghostCandidate{
					full:    kw,
					suffix:  kw[len(partial):],
					partial: pLen,
				})
			}
		}

		for _, tn := range m.tableNames {
			lower := strings.ToLower(tn)
			if strings.HasPrefix(lower, partialLower) && lower != partialLower {
				matches = append(matches, ghostCandidate{
					full:    tn,
					suffix:  tn[len(partialLower):],
					partial: pLen,
				})
			}
		}
	}

	if len(matches) == 0 {
		m.clearGhost()
		return
	}

	m.ghostMatches = matches
	m.ghostIndex = 0
	m.applyGhostIndex()
}

// GetStatementAtCursor returns the SQL statement containing the cursor.
func (m EditorModel) GetStatementAtCursor() string {
	return m.statementAtCursor()
}

func (m EditorModel) statementAtCursor() string {
	text := m.textarea.Value()
	if strings.TrimSpace(text) == "" {
		return ""
	}

	cursorLine := m.textarea.Line()
	lines := strings.Split(text, "\n")

	offset := 0
	for i := 0; i < cursorLine && i < len(lines); i++ {
		offset += len(lines[i]) + 1
	}

	segments := strings.Split(text, ";")
	pos := 0
	for _, seg := range segments {
		segEnd := pos + len(seg)
		if offset <= segEnd {
			trimmed := strings.TrimSpace(seg)
			if trimmed != "" {
				return trimmed
			}
		}
		pos = segEnd + 1
	}

	for i := len(segments) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(segments[i])
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// View renders the editor pane.
func (m EditorModel) View() string {
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

	modeStr := m.VimModeString()
	modeStyle := lipgloss.NewStyle().Bold(true)
	if m.vimMode == VimInsert {
		modeStyle = modeStyle.Foreground(lipgloss.Color("#00FF00"))
	} else {
		modeStyle = modeStyle.Foreground(lipgloss.Color("#FFAA00"))
	}
	titleLeft := HeaderStyle.Render("SQL Editor") + " " + modeStyle.Render("-- "+modeStr+" --")
	var helpText string
	if m.vimMode == VimInsert {
		helpText = "Esc normal | Tab indent/complete | Ctrl+J run | Ctrl+E all"
	} else {
		helpText = "i insert | hjkl move | dd del line | dw del word | w/b word | Ctrl+J run"
	}
	titleRight := DimText.Render(helpText)
	gap := innerW - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 1 {
		gap = 1
	}
	header := titleLeft + strings.Repeat(" ", gap) + titleRight

	editorContent := m.renderHighlightedText()
	content := header + "\n" + editorContent
	return borderStyle.Width(innerW).Height(innerH).Render(content)
}

func (m EditorModel) renderHighlightedText() string {
	text := m.textarea.Value()
	if strings.TrimSpace(text) == "" && m.textarea.Placeholder != "" {
		return m.textarea.View()
	}

	lines := strings.Split(text, "\n")
	cursorLine := m.textarea.Line()
	li := m.textarea.LineInfo()
	cursorCol := li.ColumnOffset

	selStartRow := m.visualStartRow
	selStartCol := m.visualStartCol
	selEndRow := cursorLine
	selEndCol := cursorCol
	if m.visualMode {
		if selStartRow > selEndRow || (selStartRow == selEndRow && selStartCol > selEndCol) {
			selStartRow, selEndRow = selEndRow, selStartRow
			selStartCol, selEndCol = selEndCol, selStartCol
		}
	}

	var result strings.Builder
	lineNumWidth := 4

	startLine := 0
	displayLines := m.height - 4
	if displayLines < 1 {
		displayLines = 1
	}

	if len(lines) > displayLines {
		if cursorLine >= startLine+displayLines {
			startLine = cursorLine - displayLines + 1
		}
		if startLine < 0 {
			startLine = 0
		}
	}

	endLine := startLine + displayLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	for i := startLine; i < endLine; i++ {
		lineNum := fmt.Sprintf("%*d", lineNumWidth, i+1)
		lineNumStyled := DimText.Render(lineNum)

		line := ""
		if i < len(lines) {
			line = lines[i]
		}

		runes := []rune(line)
		lineSelStart := -1
		lineSelEnd := -1
		if m.visualMode && selStartRow <= i && i <= selEndRow {
			if i == selStartRow {
				lineSelStart = selStartCol
			} else {
				lineSelStart = 0
			}
			if i == selEndRow {
				lineSelEnd = selEndCol
			} else {
				lineSelEnd = len(runes)
			}
			if lineSelStart > len(runes) {
				lineSelStart = len(runes)
			}
			if lineSelEnd > len(runes) {
				lineSelEnd = len(runes)
			}
		}

		if i == cursorLine && m.focused {
			cursorChar := " "
			if cursorCol < len(runes) {
				cursorChar = string(runes[cursorCol])
			}

			highlightedBefore := m.renderLineSegment(runes, 0, cursorCol, lineSelStart, lineSelEnd)
			cursorStyled := lipgloss.NewStyle().Reverse(true).Render(cursorChar)
			highlightedAfter := m.renderLineSegment(runes, cursorCol+1, len(runes), lineSelStart, lineSelEnd)

			result.WriteString(lineNumStyled)
			result.WriteString("  ")
			result.WriteString(highlightedBefore)
			result.WriteString(cursorStyled)
			if m.ghost != "" {
				result.WriteString(GhostStyle.Render(m.ghost))
			}
			result.WriteString(highlightedAfter)
		} else {
			rendered := m.renderLineSegment(runes, 0, len(runes), lineSelStart, lineSelEnd)
			result.WriteString(lineNumStyled)
			result.WriteString("  ")
			result.WriteString(rendered)
		}

		if i < endLine-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m EditorModel) renderLineSegment(runes []rune, from, to int, selStart, selEnd int) string {
	if from >= to {
		return ""
	}
	var sb strings.Builder
	hasSel := selStart >= 0 && selEnd >= 0 && selStart < selEnd
	if !hasSel {
		return HighlightSQL(string(runes[from:to]))
	}
	segStart := from
	for segStart < to {
		if segStart < selStart && selStart < to {
			sb.WriteString(HighlightSQL(string(runes[segStart:selStart])))
			segStart = selStart
		} else if segStart >= selStart && segStart < selEnd {
			end := selEnd
			if end > to {
				end = to
			}
			sb.WriteString(lipgloss.NewStyle().Reverse(true).Render(string(runes[segStart:end])))
			segStart = end
		} else {
			sb.WriteString(HighlightSQL(string(runes[segStart:to])))
			segStart = to
		}
	}
	return sb.String()
}
