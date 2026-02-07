package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/app"
	"cli-sql/internal/db"
	"cli-sql/internal/ui"
)

// connectionModel handles the connection form on startup.
type connectionModel struct {
	inputs  []textinput.Model
	cursor  int
	err     string
	connecting bool
	done    bool
	db      *db.DB
	tables  []string
	width   int
	height  int
}

type connectResultMsg struct {
	db     *db.DB
	tables []string
	err    error
}

const (
	fieldHost = iota
	fieldPort
	fieldUser
	fieldPassword
	fieldDatabase
)

var fieldLabels = []string{"Host", "Port", "Username", "Password", "Database"}

func newConnectionModel() connectionModel {
	inputs := make([]textinput.Model, 5)

	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 256
		t.Width = 40

		switch i {
		case fieldHost:
			t.Placeholder = "localhost"
			t.SetValue("localhost")
			t.Focus()
		case fieldPort:
			t.Placeholder = "5432"
			t.SetValue("5432")
		case fieldUser:
			t.Placeholder = "postgres"
		case fieldPassword:
			t.Placeholder = ""
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '*'
		case fieldDatabase:
			t.Placeholder = "mydb"
		}
		inputs[i] = t
	}

	return connectionModel{
		inputs: inputs,
		cursor: 0,
	}
}

func (m connectionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m connectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.connecting {
				return m, nil
			}
			if m.cursor < len(m.inputs)-1 {
				// Move to next input
				m.inputs[m.cursor].Blur()
				m.cursor++
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
			// On the last field, attempt connection
			return m, m.tryConnect()
		case "shift+tab":
			if m.cursor > 0 {
				m.inputs[m.cursor].Blur()
				m.cursor--
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
		case "tab", "down":
			if m.cursor < len(m.inputs)-1 {
				m.inputs[m.cursor].Blur()
				m.cursor++
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
		case "up":
			if m.cursor > 0 {
				m.inputs[m.cursor].Blur()
				m.cursor--
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
		}

	case connectResultMsg:
		m.connecting = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.done = true
		m.db = msg.db
		m.tables = msg.tables
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(msg)
	return m, cmd
}

func (m connectionModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(ui.ColorAccent).
		Bold(true).
		MarginBottom(1)

	var b strings.Builder

	b.WriteString(titleStyle.Render("CLI-SQL - PostgreSQL Client"))
	b.WriteString("\n\n")

	for i, input := range m.inputs {
		label := fieldLabels[i]
		if i == m.cursor {
			b.WriteString(ui.AccentText.Render(fmt.Sprintf("  %s", label)))
		} else {
			b.WriteString(fmt.Sprintf("  %s", label))
		}
		b.WriteString("\n")
		b.WriteString("  " + input.View())
		b.WriteString("\n\n")
	}

	if m.err != "" {
		b.WriteString(ui.ErrorText.Render(fmt.Sprintf("  Connection failed: %s", m.err)))
		b.WriteString("\n\n")
	}

	if m.connecting {
		b.WriteString(ui.DimText.Render("  Connecting..."))
	} else {
		b.WriteString(ui.DimText.Render("  Press Enter to connect | Tab to move between fields | Ctrl+C to quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m connectionModel) tryConnect() tea.Cmd {
	m.connecting = true
	m.err = ""

	host := m.inputs[fieldHost].Value()
	port := m.inputs[fieldPort].Value()
	user := m.inputs[fieldUser].Value()
	password := m.inputs[fieldPassword].Value()
	database := m.inputs[fieldDatabase].Value()

	// Defaults
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}

	return func() tea.Msg {
		// Validate port
		if _, err := strconv.Atoi(port); err != nil {
			return connectResultMsg{err: fmt.Errorf("invalid port number")}
		}

		conn, err := db.Connect(host, port, user, password, database)
		if err != nil {
			return connectResultMsg{err: err}
		}

		tables, err := conn.ListTables()
		if err != nil {
			conn.Close()
			return connectResultMsg{err: fmt.Errorf("failed to list tables: %w", err)}
		}

		return connectResultMsg{db: conn, tables: tables}
	}
}

func main() {
	// Phase 1: Connection form
	connModel := newConnectionModel()
	p := tea.NewProgram(connModel, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cm, ok := result.(connectionModel)
	if !ok || !cm.done {
		// User quit during connection
		return
	}

	defer cm.db.Close()

	// Phase 2: Main TUI
	appModel := app.NewModel(cm.db, cm.tables)
	appProgram := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := appProgram.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
