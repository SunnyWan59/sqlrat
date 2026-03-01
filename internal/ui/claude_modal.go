package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ClaudeModalMsg struct {
	Prompt string
}

type ClaudeModalClosedMsg struct{}

type ClaudeModalModel struct {
	visible     bool
	textarea    textarea.Model
	width       int
	height      int
	waiting     bool
	response    string
	originalSQL string
}

func NewClaudeModalModel() ClaudeModalModel {
	ta := textarea.New()
	ta.Placeholder = "Ask Claude to help with your SQL..."
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.SetHeight(8)

	return ClaudeModalModel{
		textarea: ta,
	}
}

func (m *ClaudeModalModel) Open(currentSQL string) {
	m.visible = true
	m.textarea.Reset()
	m.textarea.Focus()
	m.waiting = false
	m.response = ""
	m.originalSQL = currentSQL
}

func (m *ClaudeModalModel) Close() {
	m.visible = false
	m.textarea.Blur()
	m.waiting = false
	m.response = ""
}

func (m *ClaudeModalModel) SetWaiting(waiting bool) {
	m.waiting = waiting
}

func (m *ClaudeModalModel) SetResponse(response string) {
	m.response = response
	m.waiting = false
}

func (m *ClaudeModalModel) Visible() bool {
	return m.visible
}

func (m *ClaudeModalModel) GetPrompt() string {
	return m.textarea.Value()
}

func (m *ClaudeModalModel) GetOriginalSQL() string {
	return m.originalSQL
}

func (m *ClaudeModalModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	modalW := w - 20
	if modalW < 60 {
		modalW = 60
	}
	if modalW > 100 {
		modalW = 100
	}
	textareaWidth := modalW - 10
	m.textarea.SetWidth(textareaWidth)
}

func (m ClaudeModalModel) Init() tea.Cmd {
	return nil
}

func (m ClaudeModalModel) Update(msg tea.Msg) (ClaudeModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	if m.waiting {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.Close()
				return m, func() tea.Msg { return ClaudeModalClosedMsg{} }
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.Close()
			return m, func() tea.Msg { return ClaudeModalClosedMsg{} }
		case "enter":
			if !msg.Alt {
				prompt := strings.TrimSpace(m.textarea.Value())
				if prompt != "" {
					m.waiting = true
					return m, func() tea.Msg {
						return ClaudeModalMsg{Prompt: prompt}
					}
				}
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ClaudeModalModel) View() string {
	if !m.visible {
		return ""
	}

	modalW := m.width - 20
	if modalW < 60 {
		modalW = 60
	}
	if modalW > 100 {
		modalW = 100
	}

	modalH := m.height - 10
	if modalH < 15 {
		modalH = 15
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(modalW).
		Height(modalH)

	var content strings.Builder

	title := HeaderStyle.Render("Ask Claude")
	content.WriteString(title)
	content.WriteString("\n\n")

	if m.waiting {
		content.WriteString(AccentText.Render("Waiting for Claude..."))
		content.WriteString("\n\n")
		content.WriteString(DimText.Render("Press Esc to cancel"))
	} else {
		content.WriteString(DimText.Render("Describe what you want to do with your SQL:"))
		content.WriteString("\n\n")
		content.WriteString(m.textarea.View())
		content.WriteString("\n\n")
		content.WriteString(DimText.Render("Enter to send | Alt+Enter for newline | Esc to cancel"))
	}

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
