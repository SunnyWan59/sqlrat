package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExecuteQueryMsg is sent when the user executes a query with Ctrl+E.
type ExecuteQueryMsg struct {
	SQL string
}

// EditorModel wraps a textarea for SQL editing.
type EditorModel struct {
	textarea textarea.Model
	focused  bool
	width    int
	height   int
}

// NewEditorModel creates a new SQL editor.
func NewEditorModel() EditorModel {
	ta := textarea.New()
	ta.Placeholder = "Write SQL here... (Ctrl+E to execute)"
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
		case "ctrl+e":
			sql := strings.TrimSpace(m.textarea.Value())
			if sql == "" {
				return m, nil
			}
			return m, func() tea.Msg {
				return ExecuteQueryMsg{SQL: sql}
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// Value returns the current editor text.
func (m EditorModel) Value() string {
	return m.textarea.Value()
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

	// Header bar
	titleLeft := HeaderStyle.Render("SQL Editor")
	titleRight := DimText.Render("Ctrl+E to execute")
	gap := innerW - lipgloss.Width(titleLeft) - lipgloss.Width(titleRight)
	if gap < 1 {
		gap = 1
	}
	header := titleLeft + strings.Repeat(" ", gap) + titleRight

	content := header + "\n" + m.textarea.View()
	return borderStyle.Width(innerW).Height(innerH).Render(content)
}
