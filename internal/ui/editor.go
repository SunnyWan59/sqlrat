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
	visualMode      bool
	visualStartRow  int
	visualStartCol  int
}

// SetTableNames updates the list of table names used for autocomplete.
func (m *EditorModel) SetTableNames(names []string) {
	m.tableNames = names
}

// NewEditorModel creates a new SQL editor.
func NewEditorModel() EditorModel {
	ta := textarea.New()
	ta.Placeholder = "Write SQL here... (Ctrl+J run statement, Ctrl+E run all)"
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
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 && m.visualMode {
			m.visualMode = false
		}
		switch msg.String() {
		case "shift+up", "shift+down", "shift+left", "shift+right", "shift+home", "shift+end":
			m.handleShiftArrow(msg)
			movementKey := shiftToMovementKey(msg)
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(movementKey)
			m.updateGhost()
			return m, cmd
		case "ctrl+c":
			if m.visualMode {
				m.copySelection()
				m.visualMode = false
			}
			return m, nil
		case "esc":
			if m.visualMode {
				m.visualMode = false
				return m, nil
			}
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
			formatted := FormatSQL(sql)
			m.textarea.Reset()
			m.textarea.InsertString(formatted)
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
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.updateGhost()
	return m, cmd
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
	if len(partial) < 2 {
		m.clearGhost()
		return
	}

	pLen := end - start
	var matches []ghostCandidate

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

	partialLower := strings.ToLower(line[start:end])
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

	if len(matches) == 0 {
		m.clearGhost()
		return
	}

	m.ghostMatches = matches
	m.ghostIndex = 0
	m.applyGhostIndex()
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

	titleLeft := HeaderStyle.Render("SQL Editor")
	titleRight := DimText.Render("Shift+Arrows select | Ctrl+C copy sel | Ctrl+Y all | Ctrl+V paste | Ctrl+J run | Ctrl+E all")
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
			result.WriteString(highlightedAfter)

			if m.ghost != "" {
				result.WriteString(GhostStyle.Render(m.ghost))
			}
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
