package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
	IsDiff    bool
	OldSQL    string
	NewSQL    string
}

type ChatPanelSendMsg struct {
	Prompt string
}

type ChatPanelAcceptDiffMsg struct{}

type ChatPanelRejectDiffMsg struct{}

type ChatPanelModel struct {
	visible        bool
	messages       []ChatMessage
	input          textarea.Model
	width          int
	height         int
	scrollOffset   int
	waiting        bool
	currentSQL     string
	pendingDiffSQL string
	showingDiff    bool
}

func NewChatPanelModel() ChatPanelModel {
	ta := textarea.New()
	ta.Placeholder = "Ask Claude about your SQL..."
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.SetHeight(3)

	return ChatPanelModel{
		visible:  false,
		messages: []ChatMessage{},
		input:    ta,
	}
}

func (m *ChatPanelModel) Toggle() {
	m.visible = !m.visible
	if m.visible {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *ChatPanelModel) Show() {
	m.visible = true
	m.input.Focus()
}

func (m *ChatPanelModel) Hide() {
	m.visible = false
	m.input.Blur()
}

func (m *ChatPanelModel) Visible() bool {
	return m.visible
}

func (m *ChatPanelModel) SetCurrentSQL(sql string) {
	m.currentSQL = sql
}

func (m *ChatPanelModel) GetCurrentSQL() string {
	return m.currentSQL
}

func (m *ChatPanelModel) AddUserMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	m.scrollToBottom()
}

func (m *ChatPanelModel) AddAssistantMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	m.scrollToBottom()
}

func (m *ChatPanelModel) GetConversationHistory() []ChatMessage {
	var history []ChatMessage
	for _, msg := range m.messages {
		if !msg.IsDiff {
			history = append(history, msg)
		}
	}
	return history
}

func (m *ChatPanelModel) AddDiffMessage(oldSQL, newSQL string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "assistant",
		Content:   "I've suggested some changes:",
		Timestamp: time.Now(),
		IsDiff:    true,
		OldSQL:    oldSQL,
		NewSQL:    newSQL,
	})
	m.pendingDiffSQL = newSQL
	m.showingDiff = true
	m.scrollToBottom()
}

func (m *ChatPanelModel) AcceptDiff() string {
	sql := m.pendingDiffSQL
	m.pendingDiffSQL = ""
	m.showingDiff = false
	return sql
}

func (m *ChatPanelModel) RejectDiff() {
	m.pendingDiffSQL = ""
	m.showingDiff = false
}

func (m *ChatPanelModel) HasPendingDiff() bool {
	return m.showingDiff
}

func (m *ChatPanelModel) SetWaiting(waiting bool) {
	m.waiting = waiting
}

func (m *ChatPanelModel) ClearMessages() {
	m.messages = []ChatMessage{}
	m.scrollOffset = 0
}

func (m *ChatPanelModel) scrollToBottom() {
	maxScroll := m.maxScrollOffset()
	if maxScroll > 0 {
		m.scrollOffset = maxScroll
	}
}

func (m *ChatPanelModel) maxScrollOffset() int {
	totalLines := 0
	for _, msg := range m.messages {
		lines := strings.Count(msg.Content, "\n") + 3
		totalLines += lines
	}

	availHeight := m.height - 8
	if availHeight < 10 {
		availHeight = 10
	}

	maxScroll := totalLines - availHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

func (m *ChatPanelModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	inputWidth := w - 6
	if inputWidth < 20 {
		inputWidth = 20
	}
	m.input.SetWidth(inputWidth)
}

func (m ChatPanelModel) Init() tea.Cmd {
	return nil
}

func (m ChatPanelModel) Update(msg tea.Msg) (ChatPanelModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	if m.waiting {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "y":
			if m.showingDiff {
				return m, func() tea.Msg {
					return ChatPanelAcceptDiffMsg{}
				}
			}

		case "n":
			if m.showingDiff {
				m.RejectDiff()
				return m, nil
			}

		case "esc":
			if m.showingDiff {
				m.RejectDiff()
				return m, nil
			}
			if m.input.Value() == "" {
				m.Hide()
				return m, nil
			}
			m.input.Reset()
			return m, nil

		case "enter":
			if m.showingDiff {
				return m, func() tea.Msg {
					return ChatPanelAcceptDiffMsg{}
				}
			}
			if !msg.Alt {
				prompt := strings.TrimSpace(m.input.Value())
				if prompt != "" {
					m.AddUserMessage(prompt)
					m.input.Reset()
					m.waiting = true
					return m, func() tea.Msg {
						return ChatPanelSendMsg{Prompt: prompt}
					}
				}
				return m, nil
			}

		case "ctrl+l":
			m.ClearMessages()
			return m, nil

		case "pgup":
			m.scrollOffset -= 5
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil

		case "pgdown":
			m.scrollOffset += 5
			maxScroll := m.maxScrollOffset()
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
			return m, nil

		case "up":
			if m.input.Value() == "" {
				m.scrollOffset--
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
				return m, nil
			}

		case "down":
			if m.input.Value() == "" {
				m.scrollOffset++
				maxScroll := m.maxScrollOffset()
				if m.scrollOffset > maxScroll {
					m.scrollOffset = maxScroll
				}
				return m, nil
			}
		}

		if !m.showingDiff {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m ChatPanelModel) View() string {
	if !m.visible {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1)

	header := headerStyle.Render("Claude Chat")

	var messagesView strings.Builder

	visibleMessages := m.messages
	if len(m.messages) > 0 {
		startIdx := 0
		currentLine := 0
		for i, msg := range m.messages {
			msgLines := strings.Count(msg.Content, "\n") + 3
			if currentLine >= m.scrollOffset {
				startIdx = i
				break
			}
			currentLine += msgLines
		}
		if startIdx < len(m.messages) {
			visibleMessages = m.messages[startIdx:]
		}
	}

	for _, msg := range visibleMessages {
		var msgStyle lipgloss.Style
		var prefix string

		if msg.Role == "user" {
			msgStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Padding(0, 1)
			prefix = "You:"
		} else {
			msgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#cccccc")).
				Padding(0, 1)
			prefix = "Claude:"
		}

		timestamp := msg.Timestamp.Format("15:04")
		headerLine := DimText.Render(prefix + " " + timestamp)
		messagesView.WriteString(headerLine + "\n")

		if msg.IsDiff {
			content := strings.TrimSpace(msg.Content)
			messagesView.WriteString(msgStyle.Render(content) + "\n\n")

			diffs := ComputeDiff(msg.OldSQL, msg.NewSQL)
			diffContent := RenderDiff(diffs, m.width-8)
			messagesView.WriteString(diffContent + "\n\n")
		} else {
			content := strings.TrimSpace(msg.Content)
			wrapped := wrapText(content, m.width-4)
			messagesView.WriteString(msgStyle.Render(wrapped) + "\n\n")
		}
	}

	if m.waiting {
		messagesView.WriteString(DimText.Render("Claude is typing...") + "\n\n")
	}

	scrollHint := ""
	if len(m.messages) > 0 {
		scrollHint = DimText.Render("↑/↓ scroll | PgUp/PgDn | Ctrl+L clear")
	}

	inputLabel := DimText.Render("Message:")
	inputHelp := DimText.Render("Enter to send | Alt+Enter for newline | Esc to close")

	if m.showingDiff {
		inputHelp = DimText.Render("y/Enter to accept | n/Esc to reject")
	}

	contentHeight := m.height - 12
	if contentHeight < 5 {
		contentHeight = 5
	}

	messagesBox := lipgloss.NewStyle().
		Height(contentHeight).
		Width(m.width - 4).
		Render(messagesView.String())

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		messagesBox,
		scrollHint,
		"",
		inputLabel,
		m.input.View(),
		inputHelp,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Width(m.width).
		Padding(0, 1).
		Render(content)
}

func wrapText(text string, width int) string {
	if width < 10 {
		width = 10
	}

	lines := strings.Split(text, "\n")
	var wrapped []string

	for _, line := range lines {
		if len(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			wrapped = append(wrapped, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				wrapped = append(wrapped, currentLine)
				currentLine = word
			}
		}
		if currentLine != "" {
			wrapped = append(wrapped, currentLine)
		}
	}

	return strings.Join(wrapped, "\n")
}
