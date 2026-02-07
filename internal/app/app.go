package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/db"
	"cli-sql/internal/editor"
	"cli-sql/internal/ui"
)

// Pane represents which pane is focused.
type Pane int

const (
	SidebarPane Pane = iota
	EditorPane
	ResultsPane
)

// tickMsg is sent to clear expired status messages.
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// queryResultMsg carries query results back to the app.
type queryResultMsg struct {
	result   *db.QueryResult
	execRes  *db.ExecResult
	err      error
	lastSQL  string
}

// tableDataMsg carries table data after selecting a table.
type tableDataMsg struct {
	result    *db.QueryResult
	tableName string
	pks       []string
	err       error
}

// commitResultMsg carries commit result.
type commitResultMsg struct {
	err   error
	count int
}

// Model is the root Bubble Tea model.
type Model struct {
	activePane Pane
	sidebar    ui.SidebarModel
	editor     ui.EditorModel
	results    ui.ResultsModel
	statusbar  ui.StatusBarModel
	db         *db.DB
	changes    *editor.ChangeTracker
	width      int
	height     int
	lastSQL    string
	lastTable  string
}

// NewModel creates the root app model.
func NewModel(database *db.DB, tables []string) Model {
	changes := editor.NewChangeTracker()

	sidebar := ui.NewSidebarModel(tables)
	sidebar.SetFocused(true)

	editorModel := ui.NewEditorModel()
	results := ui.NewResultsModel(changes)
	statusbar := ui.NewStatusBarModel()
	statusbar.SetActivePane(0)

	return Model{
		activePane: SidebarPane,
		sidebar:    sidebar,
		editor:     editorModel,
		results:    results,
		statusbar:  statusbar,
		db:         database,
		changes:    changes,
	}
}

// Init starts the app.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	case tickMsg:
		m.statusbar.ClearExpiredMessage()
		m.statusbar.SetPendingChanges(m.changes.PendingCount())
		return m, tickCmd()

	case tea.KeyMsg:
		// Global shortcuts
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			// Don't cycle focus when editing a cell
			if m.activePane == ResultsPane && m.results.IsEditing() {
				break
			}
			m.cycleFocus(true)
			return m, nil
		case "shift+tab":
			if m.activePane == ResultsPane && m.results.IsEditing() {
				break
			}
			m.cycleFocus(false)
			return m, nil
		case "ctrl+s":
			if m.changes.HasChanges() || m.results.GetInsertedRowValues() != nil {
				return m, m.commitChanges()
			}
			return m, nil
		}

	case ui.TableSelectedMsg:
		m.lastTable = msg.Name
		return m, m.loadTable(msg.Name)

	case tableDataMsg:
		if msg.err != nil {
			m.results.SetError(msg.err.Error())
			m.statusbar.SetMessage("Error: "+msg.err.Error(), ui.MsgError)
		} else {
			m.results.SetData(msg.result.Columns, msg.result.ColumnTypes, msg.result.Rows)
			m.results.SetTableContext(msg.tableName, msg.pks)
			if len(msg.pks) == 0 {
				m.statusbar.SetMessage("Read-only: table has no primary key", ui.MsgInfo)
			} else {
				m.statusbar.SetMessage(fmt.Sprintf("Loaded %d rows from %s", msg.result.RowCount, msg.tableName), ui.MsgSuccess)
			}
			m.statusbar.SetQueryInfo(msg.result.ExecTime, msg.result.RowCount)
		}
		return m, nil

	case ui.ExecuteQueryMsg:
		m.lastSQL = msg.SQL
		return m, m.executeQuery(msg.SQL)

	case queryResultMsg:
		if msg.err != nil {
			m.results.SetError(msg.err.Error())
			m.statusbar.SetMessage("Query error: "+msg.err.Error(), ui.MsgError)
		} else if msg.result != nil {
			m.results.SetData(msg.result.Columns, msg.result.ColumnTypes, msg.result.Rows)
			m.results.SetTableContext("", nil) // free-form query, no CRUD context
			m.statusbar.SetQueryInfo(msg.result.ExecTime, msg.result.RowCount)
			m.statusbar.SetMessage(fmt.Sprintf("Query returned %d rows", msg.result.RowCount), ui.MsgSuccess)
		} else if msg.execRes != nil {
			m.results.SetInfo(fmt.Sprintf("%d rows affected", msg.execRes.RowsAffected))
			m.statusbar.SetQueryInfo(msg.execRes.ExecTime, int(msg.execRes.RowsAffected))
			m.statusbar.SetMessage(fmt.Sprintf("%d rows affected", msg.execRes.RowsAffected), ui.MsgSuccess)
		}
		return m, nil

	case commitResultMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Commit failed: "+msg.err.Error(), ui.MsgError)
		} else {
			m.statusbar.SetMessage(fmt.Sprintf("Committed %d changes", msg.count), ui.MsgSuccess)
			m.changes.Clear()
			// Refresh the current table if we were browsing one
			if m.lastTable != "" {
				return m, m.loadTable(m.lastTable)
			}
		}
		return m, nil
	}

	// Forward to focused pane
	var cmd tea.Cmd
	switch m.activePane {
	case SidebarPane:
		m.sidebar, cmd = m.sidebar.Update(msg)
	case EditorPane:
		m.editor, cmd = m.editor.Update(msg)
	case ResultsPane:
		m.results, cmd = m.results.Update(msg)
		m.statusbar.SetEditMode(m.results.IsEditing())
	}

	return m, cmd
}

// View renders the full layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Top bar
	topBar := ui.TopBarStyle.Width(m.width - 2).Render(
		fmt.Sprintf(" %s ", m.db.ConnInfo()),
	)

	// Layout: sidebar on left, editor+results stacked on right
	sidebarW := 30
	rightW := m.width - sidebarW - 1

	availH := m.height - 3 // top bar + status bar + spacing
	if availH < 6 {
		availH = 6
	}

	editorH := availH * 40 / 100
	if editorH < 5 {
		editorH = 5
	}
	resultsH := availH - editorH

	m.sidebar.SetSize(sidebarW, availH)
	m.editor.SetSize(rightW, editorH)
	m.results.SetSize(rightW, resultsH)
	m.statusbar.SetWidth(m.width)

	sidebarView := m.sidebar.View()
	editorView := m.editor.View()
	resultsView := m.results.View()

	rightPane := lipgloss.JoinVertical(lipgloss.Left, editorView, resultsView)
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightPane)

	statusView := m.statusbar.View()

	return lipgloss.JoinVertical(lipgloss.Left, topBar, mainArea, statusView)
}

func (m *Model) cycleFocus(forward bool) {
	m.sidebar.SetFocused(false)
	m.editor.SetFocused(false)
	m.results.SetFocused(false)

	if forward {
		switch m.activePane {
		case SidebarPane:
			m.activePane = EditorPane
		case EditorPane:
			m.activePane = ResultsPane
		case ResultsPane:
			m.activePane = SidebarPane
		}
	} else {
		switch m.activePane {
		case SidebarPane:
			m.activePane = ResultsPane
		case EditorPane:
			m.activePane = SidebarPane
		case ResultsPane:
			m.activePane = EditorPane
		}
	}

	switch m.activePane {
	case SidebarPane:
		m.sidebar.SetFocused(true)
		m.statusbar.SetActivePane(0)
	case EditorPane:
		m.editor.SetFocused(true)
		m.statusbar.SetActivePane(1)
	case ResultsPane:
		m.results.SetFocused(true)
		m.statusbar.SetActivePane(2)
	}
	m.statusbar.SetEditMode(false)
}

func (m *Model) recalcLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}
	sidebarW := 30
	rightW := m.width - sidebarW - 1
	availH := m.height - 3
	if availH < 6 {
		availH = 6
	}
	editorH := availH * 40 / 100
	if editorH < 5 {
		editorH = 5
	}
	resultsH := availH - editorH

	m.sidebar.SetSize(sidebarW, availH)
	m.editor.SetSize(rightW, editorH)
	m.results.SetSize(rightW, resultsH)
	m.statusbar.SetWidth(m.width)
}

func (m *Model) executeQuery(sql string) tea.Cmd {
	return func() tea.Msg {
		qr, er, err := m.db.ExecuteQuery(sql)
		return queryResultMsg{
			result:  qr,
			execRes: er,
			err:     err,
			lastSQL: sql,
		}
	}
}

func (m *Model) loadTable(tableName string) tea.Cmd {
	return func() tea.Msg {
		pks, err := m.db.GetPrimaryKeys(tableName)
		if err != nil {
			return tableDataMsg{err: err}
		}
		sql := fmt.Sprintf(`SELECT * FROM %q LIMIT 100`, tableName)
		qr, _, err := m.db.ExecuteQuery(sql)
		if err != nil {
			return tableDataMsg{err: err}
		}
		return tableDataMsg{
			result:    qr,
			tableName: tableName,
			pks:       pks,
		}
	}
}

func (m *Model) commitChanges() tea.Cmd {
	return func() tea.Msg {
		// Stage any inserted rows from the results model
		inserts := m.results.GetInsertedRowValues()
		for _, ins := range inserts {
			m.changes.StageInsert(ins)
		}

		queries, allArgs := m.changes.GenerateSQL()
		if len(queries) == 0 {
			return commitResultMsg{count: 0}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tx, err := m.db.Conn.Begin(ctx)
		if err != nil {
			return commitResultMsg{err: fmt.Errorf("begin transaction: %w", err)}
		}

		for i, q := range queries {
			var args []interface{}
			if i < len(allArgs) {
				args = allArgs[i]
			}
			_, err := tx.Exec(ctx, q, args...)
			if err != nil {
				tx.Rollback(ctx)
				return commitResultMsg{err: fmt.Errorf("exec: %w", err)}
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return commitResultMsg{err: fmt.Errorf("commit: %w", err)}
		}

		return commitResultMsg{count: len(queries)}
	}
}
