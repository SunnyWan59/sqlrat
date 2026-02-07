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
	"cli-sql/internal/config"
	"cli-sql/internal/db"
	"cli-sql/internal/ui"
)

// ---------------------------------------------------------------------------
// pickerModel – choose from saved connections
// ---------------------------------------------------------------------------

type pickerModel struct {
	cfg        *config.Config
	cursor     int
	err        string
	connecting bool
	done       bool
	newConn    bool
	db         *db.DB
	tables     []string
	width      int
	height     int
}

func newPickerModel(cfg *config.Config) pickerModel {
	return pickerModel{cfg: cfg}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.connecting {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.cfg.Connections)-1 {
				m.cursor++
			}
			return m, nil
		case "n":
			m.done = true
			m.newConn = true
			return m, tea.Quit
		case "d", "x":
			if len(m.cfg.Connections) > 0 {
				m.cfg.Delete(m.cursor)
				m.cfg.Save()
				if m.cursor >= len(m.cfg.Connections) && m.cursor > 0 {
					m.cursor--
				}
				if len(m.cfg.Connections) == 0 {
					m.done = true
					m.newConn = true
					return m, tea.Quit
				}
			}
			return m, nil
		case "enter":
			if len(m.cfg.Connections) == 0 {
				return m, nil
			}
			m.connecting = true
			m.err = ""
			return m, m.connectSaved()
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

	return m, nil
}

func (m pickerModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(ui.ColorAccent).
		Bold(true).
		MarginBottom(1)

	var b strings.Builder

	b.WriteString(titleStyle.Render("CLI-SQL - Saved Connections"))
	b.WriteString("\n\n")

	for i, conn := range m.cfg.Connections {
		display := conn.Name
		if conn.URI != "" {
			display += ui.DimText.Render("  " + conn.URI)
		} else {
			display += ui.DimText.Render(fmt.Sprintf("  %s@%s:%s/%s", conn.User, conn.Host, conn.Port, conn.Database))
		}

		if i == m.cursor {
			b.WriteString(ui.AccentText.Bold(true).Render("  ▸ " + display))
		} else {
			b.WriteString("    " + display)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(ui.ErrorText.Render(fmt.Sprintf("  Connection failed: %s", m.err)))
		b.WriteString("\n\n")
	}

	if m.connecting {
		b.WriteString(ui.DimText.Render("  Connecting..."))
	} else {
		b.WriteString(ui.DimText.Render("  Enter to connect | n new connection | d delete | Ctrl+C quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m pickerModel) connectSaved() tea.Cmd {
	conn := m.cfg.Connections[m.cursor]
	return func() tea.Msg {
		if conn.URI != "" {
			d, err := db.ConnectURI(conn.URI)
			if err != nil {
				return connectResultMsg{err: err}
			}
			tables, err := d.ListTables()
			if err != nil {
				d.Close()
				return connectResultMsg{err: fmt.Errorf("failed to list tables: %w", err)}
			}
			return connectResultMsg{db: d, tables: tables}
		}

		d, err := db.Connect(conn.Host, conn.Port, conn.User, conn.Password, conn.Database)
		if err != nil {
			return connectResultMsg{err: err}
		}
		tables, err := d.ListTables()
		if err != nil {
			d.Close()
			return connectResultMsg{err: fmt.Errorf("failed to list tables: %w", err)}
		}
		return connectResultMsg{db: d, tables: tables}
	}
}

// ---------------------------------------------------------------------------
// connectionModel – new connection form (URI or individual fields)
// ---------------------------------------------------------------------------

// connMode represents whether the user is entering a URI or individual fields.
type connMode int

const (
	modeURI connMode = iota
	modeFields
)

// connPhase tracks whether we're filling in fields or naming the connection.
type connPhase int

const (
	phaseConnect connPhase = iota
	phaseName
)

// connectionModel handles the connection form on startup.
type connectionModel struct {
	inputs     []textinput.Model
	uriInput   textinput.Model
	nameInput  textinput.Model
	mode       connMode
	phase      connPhase
	cursor     int
	err        string
	connecting bool
	done       bool
	db         *db.DB
	tables     []string
	savedConn  config.SavedConnection
	cfg        *config.Config
	width      int
	height     int
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

func newConnectionModel(cfg *config.Config) connectionModel {
	inputs := make([]textinput.Model, 5)

	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 256
		t.Width = 40

		switch i {
		case fieldHost:
			t.Placeholder = "localhost"
			t.SetValue("localhost")
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

	uriInput := textinput.New()
	uriInput.Placeholder = "postgres://user:password@host:5432/dbname"
	uriInput.CharLimit = 512
	uriInput.Width = 60
	uriInput.Focus()

	nameInput := textinput.New()
	nameInput.Placeholder = "my-connection"
	nameInput.CharLimit = 128
	nameInput.Width = 40

	return connectionModel{
		inputs:    inputs,
		uriInput:  uriInput,
		nameInput: nameInput,
		mode:      modeURI,
		phase:     phaseConnect,
		cursor:    0,
		cfg:       cfg,
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
		if m.phase == phaseName {
			return m.updateNamePhase(msg)
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+u":
			// Toggle between URI and fields mode
			if m.connecting {
				return m, nil
			}
			m.err = ""
			if m.mode == modeURI {
				m.mode = modeFields
				m.uriInput.Blur()
				m.cursor = 0
				m.inputs[0].Focus()
			} else {
				m.mode = modeURI
				for i := range m.inputs {
					m.inputs[i].Blur()
				}
				m.uriInput.Focus()
			}
			return m, textinput.Blink
		case "enter":
			if m.connecting {
				return m, nil
			}
			if m.mode == modeURI {
				return m, m.tryConnectURI()
			}
			// Fields mode
			if m.cursor < len(m.inputs)-1 {
				m.inputs[m.cursor].Blur()
				m.cursor++
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
			return m, m.tryConnect()
		case "shift+tab":
			if m.mode == modeURI {
				return m, nil
			}
			if m.cursor > 0 {
				m.inputs[m.cursor].Blur()
				m.cursor--
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
		case "tab", "down":
			if m.mode == modeURI {
				return m, nil
			}
			if m.cursor < len(m.inputs)-1 {
				m.inputs[m.cursor].Blur()
				m.cursor++
				m.inputs[m.cursor].Focus()
				return m, textinput.Blink
			}
		case "up":
			if m.mode == modeURI {
				return m, nil
			}
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
		m.db = msg.db
		m.tables = msg.tables
		m.phase = phaseName
		if m.mode == modeURI {
			m.uriInput.Blur()
		} else {
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
		}
		m.nameInput.Focus()
		return m, textinput.Blink
	}

	// Update the active input
	var cmd tea.Cmd
	if m.mode == modeURI {
		m.uriInput, cmd = m.uriInput.Update(msg)
	} else {
		m.inputs[m.cursor], cmd = m.inputs[m.cursor].Update(msg)
	}
	return m, cmd
}

func (m connectionModel) updateNamePhase(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.err = "Connection name cannot be empty"
			return m, nil
		}
		m.savedConn.Name = name
		if m.mode == modeURI {
			m.savedConn.URI = strings.TrimSpace(m.uriInput.Value())
		} else {
			m.savedConn.Host = m.inputs[fieldHost].Value()
			m.savedConn.Port = m.inputs[fieldPort].Value()
			m.savedConn.User = m.inputs[fieldUser].Value()
			m.savedConn.Password = m.inputs[fieldPassword].Value()
			m.savedConn.Database = m.inputs[fieldDatabase].Value()
		}
		m.cfg.Add(m.savedConn)
		m.cfg.Save()
		m.done = true
		return m, tea.Quit
	case "esc":
		m.done = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
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

	if m.phase == phaseName {
		b.WriteString(ui.AccentText.Render("  Connected successfully!"))
		b.WriteString("\n\n")
		b.WriteString(ui.AccentText.Render("  Save as"))
		b.WriteString("\n")
		b.WriteString("  " + m.nameInput.View())
		b.WriteString("\n\n")

		if m.err != "" {
			b.WriteString(ui.ErrorText.Render(fmt.Sprintf("  %s", m.err)))
			b.WriteString("\n\n")
		}

		b.WriteString(ui.DimText.Render("  Enter to save | Esc to skip"))
		b.WriteString("\n")
		return b.String()
	}

	// Mode tabs
	uriTab := "  Connection URI  "
	fieldsTab := "  Individual Fields  "
	if m.mode == modeURI {
		uriTab = ui.AccentText.Bold(true).Render("  Connection URI  ")
		fieldsTab = ui.DimText.Render("  Individual Fields  ")
	} else {
		uriTab = ui.DimText.Render("  Connection URI  ")
		fieldsTab = ui.AccentText.Bold(true).Render("  Individual Fields  ")
	}
	b.WriteString("  " + uriTab + " | " + fieldsTab)
	b.WriteString("\n")
	b.WriteString(ui.DimText.Render("  Ctrl+U to switch mode"))
	b.WriteString("\n\n")

	if m.mode == modeURI {
		b.WriteString(ui.AccentText.Render("  Connection URI"))
		b.WriteString("\n")
		b.WriteString("  " + m.uriInput.View())
		b.WriteString("\n\n")
	} else {
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
	}

	if m.err != "" {
		b.WriteString(ui.ErrorText.Render(fmt.Sprintf("  Connection failed: %s", m.err)))
		b.WriteString("\n\n")
	}

	if m.connecting {
		b.WriteString(ui.DimText.Render("  Connecting..."))
	} else if m.mode == modeURI {
		b.WriteString(ui.DimText.Render("  Press Enter to connect | Ctrl+U for individual fields | Ctrl+C to quit"))
	} else {
		b.WriteString(ui.DimText.Render("  Press Enter to connect | Tab between fields | Ctrl+U for URI mode | Ctrl+C to quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m connectionModel) tryConnectURI() tea.Cmd {
	uri := strings.TrimSpace(m.uriInput.Value())
	if uri == "" {
		return func() tea.Msg {
			return connectResultMsg{err: fmt.Errorf("URI cannot be empty")}
		}
	}

	return func() tea.Msg {
		conn, err := db.ConnectURI(uri)
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

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	cfg, _ := config.Load()

	var database *db.DB
	var tables []string

	if len(cfg.Connections) > 0 {
		picker := newPickerModel(cfg)
		p := tea.NewProgram(picker, tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		pm, ok := result.(pickerModel)
		if !ok {
			return
		}

		if !pm.done {
			return
		}

		if !pm.newConn {
			database = pm.db
			tables = pm.tables
		}
	}

	if database == nil {
		connModel := newConnectionModel(cfg)
		p := tea.NewProgram(connModel, tea.WithAltScreen())
		result, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cm, ok := result.(connectionModel)
		if !ok || !cm.done {
			return
		}

		database = cm.db
		tables = cm.tables
	}

	if database == nil {
		return
	}

	defer database.Close()

	// Phase 2: Main TUI
	appModel := app.NewModel(database, tables)
	appProgram := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := appProgram.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
