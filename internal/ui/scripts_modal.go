package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/config"
)

type ScriptLoadedMsg struct {
	Name    string
	Content string
}

type ScriptSavedMsg struct {
	Name string
}

type ScriptModalClosedMsg struct{}

type ScriptsModalMode int

const (
	ScriptsModalList ScriptsModalMode = iota
	ScriptsModalCreate
	ScriptsModalSaveAs
)

type ScriptsModalModel struct {
	visible       bool
	mode          ScriptsModalMode
	scripts       []string
	cursor        int
	input         string
	editorContent string
	err           string
	width         int
	height        int
	confirmDelete bool
}

func NewScriptsModalModel() ScriptsModalModel {
	return ScriptsModalModel{}
}

func (m *ScriptsModalModel) Open(editorContent string) {
	scripts, _ := config.ListScripts()
	m.visible = true
	m.mode = ScriptsModalList
	m.scripts = scripts
	m.cursor = 0
	m.input = ""
	m.editorContent = editorContent
	m.err = ""
	m.confirmDelete = false
}

func (m *ScriptsModalModel) Close() {
	m.visible = false
	m.err = ""
	m.confirmDelete = false
}

func (m ScriptsModalModel) Visible() bool {
	return m.visible
}

func (m *ScriptsModalModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ScriptsModalModel) Update(msg tea.Msg) (ScriptsModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				if m.cursor < len(m.scripts) {
					name := m.scripts[m.cursor]
					_ = config.DeleteScript(name)
					scripts, _ := config.ListScripts()
					m.scripts = scripts
					if m.cursor >= len(m.scripts) && m.cursor > 0 {
						m.cursor--
					}
				}
				m.confirmDelete = false
				return m, nil
			default:
				m.confirmDelete = false
				return m, nil
			}
		}

		switch m.mode {
		case ScriptsModalList:
			return m.updateList(msg)
		case ScriptsModalCreate, ScriptsModalSaveAs:
			return m.updateInput(msg)
		}
	}

	return m, nil
}

func (m ScriptsModalModel) updateList(msg tea.KeyMsg) (ScriptsModalModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+o":
		m.Close()
		return m, func() tea.Msg { return ScriptModalClosedMsg{} }
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.scripts)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.scripts) > 0 && m.cursor < len(m.scripts) {
			name := m.scripts[m.cursor]
			content, err := config.LoadScript(name)
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.Close()
			return m, func() tea.Msg {
				return ScriptLoadedMsg{Name: name, Content: content}
			}
		}
	case "n":
		m.mode = ScriptsModalCreate
		m.input = ""
		m.err = ""
	case "s":
		m.mode = ScriptsModalSaveAs
		m.input = ""
		m.err = ""
	case "d", "x":
		if len(m.scripts) > 0 && m.cursor < len(m.scripts) {
			m.confirmDelete = true
		}
	}
	return m, nil
}

func (m ScriptsModalModel) updateInput(msg tea.KeyMsg) (ScriptsModalModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ScriptsModalList
		m.input = ""
		m.err = ""
	case "enter":
		name := strings.TrimSpace(m.input)
		if name == "" {
			m.err = "Name cannot be empty"
			return m, nil
		}
		if !strings.HasSuffix(name, ".sql") {
			name += ".sql"
		}

		if m.mode == ScriptsModalCreate {
			err := config.SaveScript(name, "")
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.Close()
			return m, func() tea.Msg {
				return ScriptLoadedMsg{Name: name, Content: ""}
			}
		}

		err := config.SaveScript(name, m.editorContent)
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		scripts, _ := config.ListScripts()
		m.scripts = scripts
		m.mode = ScriptsModalList
		m.input = ""
		m.err = ""
		return m, func() tea.Msg {
			return ScriptSavedMsg{Name: name}
		}
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case "ctrl+u":
		m.input = ""
	default:
		if len(msg.String()) == 1 || msg.Type == tea.KeySpace {
			m.input += msg.String()
		} else if msg.Type == tea.KeyRunes {
			m.input += string(msg.Runes)
		}
	}
	return m, nil
}

func (m ScriptsModalModel) View() string {
	if !m.visible {
		return ""
	}

	modalW := 50
	if m.width > 0 && modalW > m.width-4 {
		modalW = m.width - 4
	}

	var b strings.Builder

	title := HeaderStyle.Render("SQL Scripts")
	b.WriteString(title)
	b.WriteString("\n")

	if m.confirmDelete && m.cursor < len(m.scripts) {
		b.WriteString("\n")
		b.WriteString(ErrorText.Render(fmt.Sprintf("  Delete %s?", m.scripts[m.cursor])))
		b.WriteString("\n")
		b.WriteString(DimText.Render("  y confirm | any key cancel"))
		b.WriteString("\n")
	} else if m.mode == ScriptsModalCreate || m.mode == ScriptsModalSaveAs {
		label := "New script name"
		if m.mode == ScriptsModalSaveAs {
			label = "Save as"
		}
		b.WriteString("\n")
		b.WriteString(AccentText.Render("  " + label))
		b.WriteString("\n")
		b.WriteString("  " + SearchInput.Render(m.input) + SearchInput.Render("█"))
		b.WriteString("\n")

		if m.err != "" {
			b.WriteString(ErrorText.Render("  " + m.err))
			b.WriteString("\n")
		}

		b.WriteString(DimText.Render("  Enter confirm | Esc back"))
		b.WriteString("\n")
	} else {
		b.WriteString(DimText.Render("  Enter load | n new | s save as | d delete | Esc close"))
		b.WriteString("\n\n")

		if len(m.scripts) == 0 {
			b.WriteString(DimText.Render("  No saved scripts"))
			b.WriteString("\n")
		} else {
			maxShow := 15
			if m.height > 0 {
				maxShow = m.height - 10
				if maxShow < 5 {
					maxShow = 5
				}
			}

			start := 0
			if m.cursor >= maxShow {
				start = m.cursor - maxShow + 1
			}
			end := start + maxShow
			if end > len(m.scripts) {
				end = len(m.scripts)
			}

			for i := start; i < end; i++ {
				name := m.scripts[i]
				if i == m.cursor {
					b.WriteString(SidebarCursorItem.Width(modalW - 4).Render("  " + name))
				} else {
					b.WriteString(SidebarTableItem.Render("  " + name))
				}
				b.WriteString("\n")
			}
		}

		if m.err != "" {
			b.WriteString(ErrorText.Render("  " + m.err))
			b.WriteString("\n")
		}
	}

	content := b.String()

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(modalW)

	rendered := modalStyle.Render(content)

	if m.width > 0 && m.height > 0 {
		renderedLines := strings.Split(rendered, "\n")
		modalH := len(renderedLines)
		topPad := (m.height - modalH) / 2
		if topPad < 0 {
			topPad = 0
		}
		leftPad := (m.width - lipgloss.Width(rendered)) / 2
		if leftPad < 0 {
			leftPad = 0
		}

		var out strings.Builder
		for i := 0; i < topPad; i++ {
			out.WriteString("\n")
		}
		for _, line := range renderedLines {
			out.WriteString(strings.Repeat(" ", leftPad))
			out.WriteString(line)
			out.WriteString("\n")
		}
		return out.String()
	}

	return rendered
}
