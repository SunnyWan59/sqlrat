package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExecuteQueryMsg is sent when the user executes a query with Ctrl+E.
type ExecuteQueryMsg struct {
	SQL string
}

var GhostStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))

// EditorModel wraps a textarea for SQL editing.
type EditorModel struct {
	textarea        textarea.Model
	focused         bool
	width           int
	height          int
	ghost           string
	ghostFull       string
	ghostPartialLen int
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
		switch msg.String() {
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
				m.ghost = ""
				m.ghostFull = ""
				m.ghostPartialLen = 0
				m.updateGhost()
				return m, nil
			}
			m.textarea.InsertString("  ")
			m.updateGhost()
			return m, nil
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

func (m *EditorModel) clearGhost() {
	m.ghost = ""
	m.ghostFull = ""
	m.ghostPartialLen = 0
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

	allKeywords := append(sqlKeywords, sqlFunctions...)
	for _, kw := range allKeywords {
		if strings.HasPrefix(kw, partial) && kw != partial {
			m.ghost = kw[len(partial):]
			m.ghostFull = kw
			m.ghostPartialLen = end - start
			return
		}
	}

	m.clearGhost()
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
	titleRight := DimText.Render("Ctrl+J line | Ctrl+E all")
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

		if i == cursorLine && m.focused {
			before := ""
			cursorChar := " "
			after := ""

			runes := []rune(line)
			if cursorCol < len(runes) {
				before = string(runes[:cursorCol])
				cursorChar = string(runes[cursorCol])
				if cursorCol+1 < len(runes) {
					after = string(runes[cursorCol+1:])
				}
			} else {
				before = line
			}

			highlightedBefore := HighlightSQL(before)
			cursorStyled := lipgloss.NewStyle().Reverse(true).Render(cursorChar)
			highlightedAfter := HighlightSQL(after)

			result.WriteString(lineNumStyled)
			result.WriteString("  ")
			result.WriteString(highlightedBefore)
			result.WriteString(cursorStyled)
			result.WriteString(highlightedAfter)

			if m.ghost != "" {
				result.WriteString(GhostStyle.Render(m.ghost))
			}
		} else {
			result.WriteString(lineNumStyled)
			result.WriteString("  ")
			result.WriteString(HighlightSQL(line))
		}

		if i < endLine-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
