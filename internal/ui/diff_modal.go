package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DiffModalAcceptedMsg struct {
	NewSQL string
}

type DiffModalRejectedMsg struct{}

type DiffModalModel struct {
	visible  bool
	oldSQL   string
	newSQL   string
	diffs    []DiffLine
	width    int
	height   int
	scrollY  int
	maxLines int
}

func NewDiffModalModel() DiffModalModel {
	return DiffModalModel{}
}

func (m *DiffModalModel) Show(oldSQL, newSQL string) {
	m.visible = true
	m.oldSQL = oldSQL
	m.newSQL = newSQL
	m.diffs = ComputeDiff(oldSQL, newSQL)
	m.scrollY = 0
}

func (m *DiffModalModel) Close() {
	m.visible = false
	m.oldSQL = ""
	m.newSQL = ""
	m.diffs = nil
	m.scrollY = 0
}

func (m *DiffModalModel) Visible() bool {
	return m.visible
}

func (m *DiffModalModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.maxLines = h - 15
	if m.maxLines < 5 {
		m.maxLines = 5
	}
}

func (m DiffModalModel) Init() tea.Cmd {
	return nil
}

func (m DiffModalModel) Update(msg tea.Msg) (DiffModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "n":
			m.Close()
			return m, func() tea.Msg { return DiffModalRejectedMsg{} }

		case "enter", "y":
			newSQL := m.newSQL
			m.Close()
			return m, func() tea.Msg { return DiffModalAcceptedMsg{NewSQL: newSQL} }

		case "up", "k":
			if m.scrollY > 0 {
				m.scrollY--
			}

		case "down", "j":
			totalLines := len(m.diffs)
			if m.scrollY < totalLines-m.maxLines {
				m.scrollY++
			}

		case "pgup":
			m.scrollY -= m.maxLines
			if m.scrollY < 0 {
				m.scrollY = 0
			}

		case "pgdown":
			totalLines := len(m.diffs)
			m.scrollY += m.maxLines
			if m.scrollY > totalLines-m.maxLines {
				m.scrollY = totalLines - m.maxLines
				if m.scrollY < 0 {
					m.scrollY = 0
				}
			}

		case "g":
			m.scrollY = 0

		case "G":
			totalLines := len(m.diffs)
			m.scrollY = totalLines - m.maxLines
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		}
	}

	return m, nil
}

func (m DiffModalModel) View() string {
	if !m.visible {
		return ""
	}

	modalW := m.width - 10
	if modalW < 60 {
		modalW = 60
	}
	if modalW > 120 {
		modalW = 120
	}

	modalH := m.height - 6
	if modalH < 20 {
		modalH = 20
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(modalW).
		Height(modalH)

	var content strings.Builder

	title := HeaderStyle.Render("Claude Suggested Changes")
	content.WriteString(title)
	content.WriteString("\n\n")

	visibleDiffs := m.diffs
	if len(m.diffs) > m.maxLines {
		endIdx := m.scrollY + m.maxLines
		if endIdx > len(m.diffs) {
			endIdx = len(m.diffs)
		}
		visibleDiffs = m.diffs[m.scrollY:endIdx]
	}

	diffView := RenderDiff(visibleDiffs, modalW-6)
	content.WriteString(diffView)
	content.WriteString("\n\n")

	totalLines := len(m.diffs)
	if totalLines > m.maxLines {
		scrollInfo := DimText.Render("(↑/↓ to scroll, g/G for top/bottom)")
		content.WriteString(scrollInfo)
		content.WriteString("\n")
	}

	instructions := DimText.Render("y/Enter to accept | n/Esc to reject")
	content.WriteString(instructions)

	modal := modalStyle.Render(content.String())

	overlay := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(""),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)

	return overlay
}
