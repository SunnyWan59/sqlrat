package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-sql/internal/config"
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

// spinnerTickMsg drives the background-copy spinner animation.
type spinnerTickMsg struct{}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// queryResultMsg carries query results back to the app.
type queryResultMsg struct {
	result    *db.QueryResult
	execRes   *db.ExecResult
	err       error
	lastSQL   string
	tableName string   // extracted table name for enabling edits on free-form SELECTs
	pks       []string // primary keys for the extracted table, if any
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

// reconnectResultMsg carries the result of a reconnect attempt.
type reconnectResultMsg struct {
	tables []string
	err    error
}

// switchDBResultMsg carries the result of a database switch.
type switchDBResultMsg struct {
	tables    []string
	databases []string
	dbName    string
	err       error
}

// copyDBResultMsg carries the result of a database copy.
type copyDBResultMsg struct {
	databases []string
	target    string
	err       error
}

// ddlRefreshMsg carries the result of a DDL-triggered table list refresh.
type ddlRefreshMsg struct {
	tables    []string
	tableName string
	tableData *tableDataMsg
	err       error
}

// dropDBResultMsg carries the result of a database drop.
type dropDBResultMsg struct {
	databases    []string
	dropped      string
	switchedToDB string
	tables       []string
	err          error
}

// Model is the root Bubble Tea model.
type Model struct {
	activePane        Pane
	sidebar           ui.SidebarModel
	editor            ui.EditorModel
	results           ui.ResultsModel
	statusbar         ui.StatusBarModel
	scriptsModal      ui.ScriptsModalModel
	db                *db.DB
	changes           *editor.ChangeTracker
	width             int
	height            int
	lastSQL           string
	lastTable         string
	pendingDMLMsg     string
	confirmClearEdits bool
	currentScript     string
}

// NewModel creates the root app model.
func NewModel(database *db.DB, tables []string, databases []string) Model {
	changes := editor.NewChangeTracker()

	sidebar := ui.NewSidebarModel(tables)
	sidebar.SetFocused(true)
	sidebar.SetDatabases(databases)
	sidebar.SetActiveDatabase(database.Database())

	editorModel := ui.NewEditorModel()
	editorModel.SetTableNames(tables)

	autosaved, _ := config.LoadAutosave()
	if autosaved != "" {
		editorModel.SetValue(autosaved)
	}

	results := ui.NewResultsModel(changes)
	statusbar := ui.NewStatusBarModel()
	statusbar.SetActivePane(0)
	scriptsModal := ui.NewScriptsModalModel()

	return Model{
		activePane:   SidebarPane,
		sidebar:      sidebar,
		editor:       editorModel,
		results:      results,
		statusbar:    statusbar,
		scriptsModal: scriptsModal,
		db:           database,
		changes:      changes,
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
		m.scriptsModal.SetSize(msg.Width, msg.Height)
		return m, nil

	case tickMsg:
		m.statusbar.ClearExpiredMessage()
		m.statusbar.SetPendingChanges(m.changes.PendingCount())
		return m, tickCmd()

	case ui.ScriptLoadedMsg:
		m.editor.SetValue(msg.Content)
		m.currentScript = msg.Name
		m.statusbar.SetMessage(fmt.Sprintf("Loaded %s", msg.Name), ui.MsgSuccess)
		return m, nil

	case ui.ScriptSavedMsg:
		m.currentScript = msg.Name
		m.statusbar.SetMessage(fmt.Sprintf("Saved %s", msg.Name), ui.MsgSuccess)
		return m, nil

	case ui.ScriptModalClosedMsg:
		return m, nil

	case tea.KeyMsg:
		if m.scriptsModal.Visible() {
			var cmd tea.Cmd
			m.scriptsModal, cmd = m.scriptsModal.Update(msg)
			return m, cmd
		}

		if m.confirmClearEdits {
			switch msg.String() {
			case "y", "Y":
				m.changes.Clear()
				m.results.ClearInsertedRows()
				m.confirmClearEdits = false
				m.statusbar.SetMessage("All changes cleared", ui.MsgSuccess)
				return m, nil
			default:
				m.confirmClearEdits = false
				m.statusbar.SetMessage("Cancelled", ui.MsgInfo)
				return m, nil
			}
		}

		// Global shortcuts
		switch msg.String() {
		case "ctrl+c":
			config.SaveAutosave(m.editor.Value())
			return m, tea.Quit
		case "tab":
			if m.activePane == ResultsPane && (m.results.IsEditing() || m.results.IsSearching() || m.results.IsPreviewing()) {
				break
			}
			if m.activePane == SidebarPane && m.sidebar.IsSearching() {
				break
			}
			if m.activePane == EditorPane && m.editor.HasGhost() {
				break
			}
			m.cycleFocus(true)
			return m, nil
		case "shift+tab":
			if m.activePane == ResultsPane && (m.results.IsEditing() || m.results.IsSearching() || m.results.IsPreviewing()) {
				break
			}
			if m.activePane == SidebarPane && m.sidebar.IsSearching() {
				break
			}
			m.cycleFocus(false)
			return m, nil
		case "ctrl+s":
			if m.activePane == ResultsPane && m.results.IsPreviewing() {
				break
			}
			if m.changes.HasChanges() || m.results.GetInsertedRowValues() != nil {
				return m, m.commitChanges()
			}
			return m, nil
		case "ctrl+r":
			m.statusbar.SetMessage("Reconnecting...", ui.MsgInfo)
			return m, m.reconnect()
		case "ctrl+x":
			if m.changes.HasChanges() || m.results.GetInsertedRowValues() != nil {
				m.confirmClearEdits = true
				m.statusbar.SetMessage("Clear all pending changes? (y/n)", ui.MsgInfo)
				return m, nil
			}
		case "ctrl+o":
			m.scriptsModal.Open(m.editor.Value())
			return m, nil
		}

	case ui.EditBlockedMsg:
		m.statusbar.SetMessage(msg.Reason, ui.MsgError)
		return m, nil

	case ui.DeleteDatabaseMsg:
		m.statusbar.SetMessage(fmt.Sprintf("Dropping %s...", msg.Name), ui.MsgInfo)
		return m, m.dropDatabase(msg.Name)

	case dropDBResultMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Drop failed: "+msg.err.Error(), ui.MsgError)
		} else {
			m.sidebar.SetDatabases(msg.databases)
			if msg.switchedToDB != "" {
				m.sidebar.SetActiveDatabase(msg.switchedToDB)
				m.sidebar.SetTables(msg.tables)
				m.editor.SetTableNames(msg.tables)
				m.changes.Clear()
				m.lastTable = ""
				m.results.Clear()
			}
			m.statusbar.SetMessage(fmt.Sprintf("Dropped database %s", msg.dropped), ui.MsgSuccess)
		}
		return m, nil

	case ui.CopyDatabaseMsg:
		m.statusbar.SetCopyingDB(true, msg.Target)
		m.statusbar.SetMessage(fmt.Sprintf("Copying %s → %s…", msg.Source, msg.Target), ui.MsgInfo)
		return m, tea.Batch(m.copyDatabase(msg.Source, msg.Target), spinnerTickCmd())

	case copyDBResultMsg:
		m.statusbar.SetCopyingDB(false, "")
		if msg.err != nil {
			m.statusbar.SetMessage("Copy failed: "+msg.err.Error(), ui.MsgError)
		} else {
			m.sidebar.SetDatabases(msg.databases)
			m.statusbar.SetMessage(fmt.Sprintf("Created database %s", msg.target), ui.MsgSuccess)
		}
		return m, nil

	case spinnerTickMsg:
		if m.statusbar.IsCopyingDB() {
			m.statusbar.AdvanceSpinner()
			return m, spinnerTickCmd()
		}
		return m, nil

	case ui.DatabaseSelectedMsg:
		m.statusbar.SetMessage(fmt.Sprintf("Switching to %s...", msg.Name), ui.MsgInfo)
		return m, m.switchDatabase(msg.Name)

	case switchDBResultMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Switch failed: "+msg.err.Error(), ui.MsgError)
		} else {
			m.sidebar.SetTables(msg.tables)
			m.editor.SetTableNames(msg.tables)
			m.sidebar.SetDatabases(msg.databases)
			m.sidebar.SetActiveDatabase(msg.dbName)
			m.changes.Clear()
			m.lastTable = ""
			m.results.Clear()
			m.statusbar.SetMessage(fmt.Sprintf("Switched to %s (%d tables)", msg.dbName, len(msg.tables)), ui.MsgSuccess)
		}
		return m, nil

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
			if m.pendingDMLMsg != "" {
				m.results.SetBanner(m.pendingDMLMsg)
				m.statusbar.SetMessage(m.pendingDMLMsg, ui.MsgSuccess)
				m.pendingDMLMsg = ""
			} else if len(msg.pks) == 0 {
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

	case ddlRefreshMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("DDL refresh error: "+msg.err.Error(), ui.MsgError)
		} else {
			m.sidebar.SetTables(msg.tables)
			m.editor.SetTableNames(msg.tables)
			if msg.tableData != nil && msg.tableData.err == nil {
				m.lastTable = msg.tableName
				m.results.SetData(msg.tableData.result.Columns, msg.tableData.result.ColumnTypes, msg.tableData.result.Rows)
				m.results.SetTableContext(msg.tableData.tableName, msg.tableData.pks)
				m.statusbar.SetQueryInfo(msg.tableData.result.ExecTime, msg.tableData.result.RowCount)
				m.statusbar.SetMessage(fmt.Sprintf("Created table %s", msg.tableName), ui.MsgSuccess)
			} else {
				m.statusbar.SetMessage(fmt.Sprintf("Tables refreshed (%d tables)", len(msg.tables)), ui.MsgSuccess)
			}
		}
		return m, nil

	case queryResultMsg:
		if msg.err != nil {
			m.results.SetError(msg.err.Error())
			m.statusbar.SetMessage("Query error: "+msg.err.Error(), ui.MsgError)
		} else if msg.result != nil {
			m.results.SetData(msg.result.Columns, msg.result.ColumnTypes, msg.result.Rows)
			// Use extracted table context so free-form SELECTs are still editable
			m.results.SetTableContext(msg.tableName, msg.pks)
			if msg.tableName != "" {
				m.lastTable = msg.tableName
			}
			m.statusbar.SetQueryInfo(msg.result.ExecTime, msg.result.RowCount)
			m.statusbar.SetMessage(fmt.Sprintf("Query returned %d rows", msg.result.RowCount), ui.MsgSuccess)
		} else if msg.execRes != nil {
			m.statusbar.SetQueryInfo(msg.execRes.ExecTime, int(msg.execRes.RowsAffected))
			m.statusbar.SetMessage(fmt.Sprintf("%d rows affected", msg.execRes.RowsAffected), ui.MsgSuccess)

			if ddlTable := extractDDLTableName(msg.lastSQL); ddlTable != "" {
				isCreate := isCreateTable(msg.lastSQL)
				return m, m.refreshAfterDDL(ddlTable, isCreate)
			}

			table := m.lastTable
			if table == "" {
				table = extractTableName(msg.lastSQL)
			}
			if table != "" {
				m.pendingDMLMsg = fmt.Sprintf("✓ %d rows affected", msg.execRes.RowsAffected)
				m.lastTable = table
				return m, m.loadTable(table)
			}
			m.results.SetInfo(fmt.Sprintf("%d rows affected", msg.execRes.RowsAffected))
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

	case reconnectResultMsg:
		if msg.err != nil {
			m.statusbar.SetMessage("Reconnect failed: "+msg.err.Error(), ui.MsgError)
		} else {
			m.sidebar.SetTables(msg.tables)
			m.editor.SetTableNames(msg.tables)
			m.changes.Clear()
			m.statusbar.SetMessage(fmt.Sprintf("Reconnected (%d tables)", len(msg.tables)), ui.MsgSuccess)
			// Reload active table if one was selected
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
		m.statusbar.SetSearchMode(m.sidebar.IsSearching())
	case EditorPane:
		m.editor, cmd = m.editor.Update(msg)
	case ResultsPane:
		m.results, cmd = m.results.Update(msg)
		m.statusbar.SetEditMode(m.results.IsEditing())
		m.statusbar.SetSearchMode(m.results.IsSearching())
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

	if m.scriptsModal.Visible() {
		m.scriptsModal.SetSize(m.width, m.height)
		return m.scriptsModal.View()
	}

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
		queryRes, execRes, err := m.db.ExecuteQuery(sql)
		msg := queryResultMsg{
			result:  queryRes,
			execRes: execRes,
			err:     err,
			lastSQL: sql,
		}
		// For SELECT results, try to extract the table name and look up PKs
		// so that free-form queries like "SELECT * FROM users" are still editable.
		if queryRes != nil && err == nil {
			if table := extractTableName(sql); table != "" {
				msg.tableName = table
				if pks, pkErr := m.db.GetPrimaryKeys(table); pkErr == nil {
					msg.pks = pks
				}
			}
		}
		return msg
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

func (m *Model) reconnect() tea.Cmd {
	return func() tea.Msg {
		if err := m.db.Reconnect(); err != nil {
			return reconnectResultMsg{err: fmt.Errorf("reconnect: %w", err)}
		}
		tables, err := m.db.ListTables()
		if err != nil {
			return reconnectResultMsg{err: fmt.Errorf("list tables: %w", err)}
		}
		return reconnectResultMsg{tables: tables}
	}
}

func (m *Model) dropDatabase(name string) tea.Cmd {
	wasActive := m.db.Database() == name
	return func() tea.Msg {
		if err := m.db.DropDatabase(name); err != nil {
			return dropDBResultMsg{err: fmt.Errorf("drop database: %w", err)}
		}
		databases, err := m.db.ListDatabases()
		if err != nil {
			return dropDBResultMsg{err: fmt.Errorf("list databases: %w", err)}
		}
		result := dropDBResultMsg{databases: databases, dropped: name}
		if wasActive {
			result.switchedToDB = m.db.Database()
			tables, err := m.db.ListTables()
			if err != nil {
				return dropDBResultMsg{err: fmt.Errorf("list tables: %w", err)}
			}
			result.tables = tables
		}
		return result
	}
}

func (m *Model) copyDatabase(source, target string) tea.Cmd {
	return func() tea.Msg {
		if err := m.db.CopyDatabase(source, target); err != nil {
			return copyDBResultMsg{err: fmt.Errorf("copy database: %w", err)}
		}
		databases, err := m.db.ListDatabases()
		if err != nil {
			return copyDBResultMsg{err: fmt.Errorf("list databases: %w", err)}
		}
		return copyDBResultMsg{databases: databases, target: target}
	}
}

func (m *Model) switchDatabase(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.db.SwitchDatabase(name); err != nil {
			return switchDBResultMsg{err: fmt.Errorf("switch database: %w", err)}
		}
		tables, err := m.db.ListTables()
		if err != nil {
			return switchDBResultMsg{err: fmt.Errorf("list tables: %w", err)}
		}
		databases, err := m.db.ListDatabases()
		if err != nil {
			return switchDBResultMsg{err: fmt.Errorf("list databases: %w", err)}
		}
		return switchDBResultMsg{tables: tables, databases: databases, dbName: name}
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

func (m *Model) refreshAfterDDL(tableName string, loadTable bool) tea.Cmd {
	return func() tea.Msg {
		tables, err := m.db.ListTables()
		if err != nil {
			return ddlRefreshMsg{err: fmt.Errorf("list tables: %w", err)}
		}
		result := ddlRefreshMsg{tables: tables, tableName: tableName}
		if loadTable {
			pks, err := m.db.GetPrimaryKeys(tableName)
			if err != nil {
				result.tableData = &tableDataMsg{err: err}
				return result
			}
			sql := fmt.Sprintf(`SELECT * FROM %q LIMIT 100`, tableName)
			qr, _, err := m.db.ExecuteQuery(sql)
			if err != nil {
				result.tableData = &tableDataMsg{err: err}
				return result
			}
			result.tableData = &tableDataMsg{
				result:    qr,
				tableName: tableName,
				pks:       pks,
			}
		}
		return result
	}
}

func isCreateTable(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "CREATE TABLE") || strings.HasPrefix(upper, "CREATE UNLOGGED TABLE") || strings.HasPrefix(upper, "CREATE TEMP TABLE") || strings.HasPrefix(upper, "CREATE TEMPORARY TABLE")
}

func extractDDLTableName(sql string) string {
	tokens := strings.Fields(strings.TrimSpace(sql))
	upper := make([]string, len(tokens))
	for i, t := range tokens {
		upper[i] = strings.ToUpper(t)
	}
	for i, tok := range upper {
		if tok == "TABLE" && i > 0 && (upper[i-1] == "CREATE" || upper[i-1] == "DROP" || upper[i-1] == "ALTER") {
			idx := i + 1
			if idx < len(upper) && (upper[idx] == "IF" || upper[idx] == "NOT") {
				for idx < len(upper) && (upper[idx] == "IF" || upper[idx] == "NOT" || upper[idx] == "EXISTS") {
					idx++
				}
			}
			if idx < len(tokens) {
				name := tokens[idx]
				name = strings.Trim(name, `"'`+"`")
				name = strings.TrimRight(name, "(;,")
				if strings.Contains(name, ".") {
					parts := strings.SplitN(name, ".", 2)
					name = strings.Trim(parts[len(parts)-1], `"'`+"`")
				}
				if name != "" {
					return name
				}
			}
		}
	}
	return ""
}

func extractTableName(sql string) string {
	tokens := strings.Fields(strings.TrimSpace(sql))
	upper := make([]string, len(tokens))
	for i, t := range tokens {
		upper[i] = strings.ToUpper(t)
	}
	for i, tok := range upper {
		if (tok == "INTO" || tok == "FROM" || tok == "UPDATE") && i+1 < len(tokens) {
			name := tokens[i+1]
			name = strings.Trim(name, `"'`)
			name = strings.TrimRight(name, "(;,")
			if name != "" {
				return name
			}
		}
	}
	return ""
}
