package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// MessageType represents the type of status message.
type MessageType int

const (
	MsgInfo MessageType = iota
	MsgSuccess
	MsgError
)

// StatusBarModel is the context-aware status bar at the bottom.
type StatusBarModel struct {
	message        string
	messageType    MessageType
	messageTime    time.Time
	pendingChanges int
	activePane     int
	editMode       bool
	queryTime      time.Duration
	rowCount       int
	width          int
}

// NewStatusBarModel creates a new status bar.
func NewStatusBarModel() StatusBarModel {
	return StatusBarModel{}
}

// SetWidth sets the status bar width.
func (m *StatusBarModel) SetWidth(w int) {
	m.width = w
}

// SetMessage sets a status message.
func (m *StatusBarModel) SetMessage(msg string, t MessageType) {
	m.message = msg
	m.messageType = t
	m.messageTime = time.Now()
}

// SetPendingChanges updates the pending changes count.
func (m *StatusBarModel) SetPendingChanges(count int) {
	m.pendingChanges = count
}

// SetActivePane sets which pane is focused (0=sidebar, 1=editor, 2=results).
func (m *StatusBarModel) SetActivePane(pane int) {
	m.activePane = pane
}

// SetEditMode sets whether the results table is in edit mode.
func (m *StatusBarModel) SetEditMode(editing bool) {
	m.editMode = editing
}

// SetQueryInfo updates the last query stats.
func (m *StatusBarModel) SetQueryInfo(elapsed time.Duration, rowCount int) {
	m.queryTime = elapsed
	m.rowCount = rowCount
}

// ClearExpiredMessage clears success messages after 3 seconds.
func (m *StatusBarModel) ClearExpiredMessage() {
	if m.messageType == MsgSuccess && time.Since(m.messageTime) > 3*time.Second {
		m.message = ""
	}
}

// View renders the status bar.
func (m StatusBarModel) View() string {
	// Left side: keybinding hints
	hints := m.contextHints()

	// Right side: pending changes + query info
	var rightParts []string
	if m.pendingChanges > 0 {
		rightParts = append(rightParts, fmt.Sprintf("Pending: %d | Ctrl+S to commit", m.pendingChanges))
	}
	if m.queryTime > 0 {
		rightParts = append(rightParts, fmt.Sprintf("%d rows in %s", m.rowCount, m.queryTime.Round(time.Millisecond)))
	}
	right := strings.Join(rightParts, " | ")

	// Message overlay
	if m.message != "" {
		var msgStyle lipgloss.Style
		switch m.messageType {
		case MsgError:
			msgStyle = StatusErrorStyle
		case MsgSuccess:
			msgStyle = StatusSuccessStyle
		default:
			msgStyle = StatusBarStyle
		}
		hints = msgStyle.Render(m.message)
	}

	// Combine left and right
	w := m.width
	if w < 20 {
		w = 20
	}
	gap := w - lipgloss.Width(hints) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	line := hints + strings.Repeat(" ", gap) + right
	return StatusBarStyle.Width(w).Render(line)
}

func (m StatusBarModel) contextHints() string {
	if m.editMode {
		return "Type to edit | Tab/Enter Next col | Shift+Tab Prev col | Esc Cancel"
	}

	switch m.activePane {
	case 0: // sidebar
		return "j/k Navigate | Enter Select table | Tab Switch pane"
	case 1: // editor
		return "Ctrl+E Execute | Tab Switch pane"
	case 2: // results
		return "Arrow keys Navigate | e Edit | d Delete | a Add row | Tab Switch pane"
	default:
		return "Tab Switch pane | Ctrl+C Quit"
	}
}
