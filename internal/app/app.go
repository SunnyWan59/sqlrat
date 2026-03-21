package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/SunnyWan59/sqlrat/internal/claude"
	"github.com/SunnyWan59/sqlrat/internal/config"
	"github.com/SunnyWan59/sqlrat/internal/db"
	"github.com/SunnyWan59/sqlrat/internal/editor"
	"github.com/SunnyWan59/sqlrat/internal/env"
	"github.com/SunnyWan59/sqlrat/internal/ui"
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

type claudeResponseMsg struct {
	response string // text explanation from Claude
	diff     *claude.DiffPayload // optional SQL diff
	err      error
}

type claudeStreamTokenMsg struct {
	content string
}

// Model is the root Bubble Tea model.
type Model struct {
	activePane        Pane
	sidebar           ui.SidebarModel
	editor            ui.EditorModel
	results           ui.ResultsModel
	statusbar         ui.StatusBarModel
	scriptsModal      ui.ScriptsModalModel
	claudeModal       ui.ClaudeModalModel
	exportModal       ui.ExportModalModel
	chatPanel         ui.ChatPanelModel
	diffModal         ui.DiffModalModel
	db                *db.DB
	changes           *editor.ChangeTracker
	width             int
	height            int
	lastSQL              string
	lastTable            string
	lastColumnRefreshTbl string
	lastError            string
	pendingDMLMsg     string
	confirmClearEdits bool
	currentScript     string
	claudeClient      *claude.Client
}

func init() {
	f, err := os.OpenFile("/tmp/sqlrat-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		log.SetOutput(f)
	}
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
	if colCache, err := database.ListAllColumnNames(); err == nil {
		editorModel.SetColumnCache(colCache)
	}

	autosaved, _ := config.LoadAutosave()
	if autosaved != "" {
		editorModel.SetValue(autosaved)
	}

	results := ui.NewResultsModel(changes)
	statusbar := ui.NewStatusBarModel()
	statusbar.SetActivePane(0)
	scriptsModal := ui.NewScriptsModalModel()
	claudeModal := ui.NewClaudeModalModel()
	exportModal := ui.NewExportModalModel()
	chatPanel := ui.NewChatPanelModel()
	diffModal := ui.NewDiffModalModel()

	var claudeClient *claude.Client
	if backendURL := env.Get("SQL_RAT_BACKEND_URL"); backendURL != "" {
		claudeClient = claude.NewClient(backendURL)
	} else {
		claudeClient = claude.NewClient("")
	}

	return Model{
		activePane:   SidebarPane,
		sidebar:      sidebar,
		editor:       editorModel,
		results:      results,
		statusbar:    statusbar,
		scriptsModal: scriptsModal,
		claudeModal:  claudeModal,
		exportModal:  exportModal,
		chatPanel:    chatPanel,
		diffModal:    diffModal,
		db:           database,
		changes:      changes,
		claudeClient: claudeClient,
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
		m.claudeModal.SetSize(msg.Width, msg.Height)
		m.exportModal.SetSize(msg.Width, msg.Height)
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

	case ui.ClaudeModalMsg:
		return m, m.askClaude(msg.Prompt)

	case ui.ClaudeModalClosedMsg:
		return m, nil

	case ui.ChatPanelSendMsg:
		log.Printf("[app] ChatPanelSendMsg received: prompt=%q", msg.Prompt)
		return m, tea.Batch(m.askClaudeChat(msg.Prompt), spinnerTickCmd())

	case tea.KeyMsg:
		if m.chatPanel.Visible() {
			var cmd tea.Cmd
			m.chatPanel, cmd = m.chatPanel.Update(msg)
			return m, cmd
		}

		if m.claudeModal.Visible() {
			var cmd tea.Cmd
			m.claudeModal, cmd = m.claudeModal.Update(msg)
			return m, cmd
		}

		if m.exportModal.Visible() {
			var cmd tea.Cmd
			m.exportModal, cmd = m.exportModal.Update(msg)
			return m, cmd
		}

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
			if m.activePane == EditorPane && m.editor.IsInsertMode() {
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
		case "alt+i":
			m.chatPanel.Toggle()
			m.chatPanel.SetCurrentSQL(m.editor.Value())
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
				m.editor.SetColumnNames(nil)
				m.lastColumnRefreshTbl = ""
				m.refreshSchemaCache()
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
		needsTick := false
		if m.statusbar.IsCopyingDB() {
			m.statusbar.AdvanceSpinner()
			needsTick = true
		}
		if m.chatPanel.Visible() && m.chatPanel.IsWaiting() {
			m.chatPanel.AdvanceSpinner()
			needsTick = true
		}
		if needsTick {
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
			m.editor.SetColumnNames(nil)
			m.lastColumnRefreshTbl = ""
			m.refreshSchemaCache()
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
			m.editor.SetColumnNames(msg.result.Columns)
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

	case ui.OpenExportModalMsg:
		if len(msg.Columns) == 0 || len(msg.Rows) == 0 {
			m.statusbar.SetMessage("No data to export", ui.MsgError)
			return m, nil
		}
		m.exportModal.Open(msg.Columns, msg.Rows)
		m.exportModal.SetSize(m.width, m.height)
		return m, nil

	case ui.ExportFormatSelectedMsg:
		path, err := m.exportResults(msg.Format, msg.Columns, msg.Rows)
		if err != nil {
			m.statusbar.SetMessage("Export failed: "+err.Error(), ui.MsgError)
			return m, nil
		}
		m.statusbar.SetMessage(fmt.Sprintf("Exported %d rows to %s", len(msg.Rows), path), ui.MsgSuccess)
		return m, nil

	case ui.ExportModalClosedMsg:
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
			m.refreshSchemaCache()
			if msg.tableData != nil && msg.tableData.err == nil {
				m.lastTable = msg.tableName
				m.results.SetData(msg.tableData.result.Columns, msg.tableData.result.ColumnTypes, msg.tableData.result.Rows)
				m.results.SetTableContext(msg.tableData.tableName, msg.tableData.pks)
				m.editor.SetColumnNames(msg.tableData.result.Columns)
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
			m.lastError = msg.err.Error()
		} else if msg.result != nil {
			m.results.SetData(msg.result.Columns, msg.result.ColumnTypes, msg.result.Rows)
			m.results.SetTableContext(msg.tableName, msg.pks)
			m.editor.SetColumnNames(msg.result.Columns)
			if msg.tableName != "" {
				m.lastTable = msg.tableName
			}
			m.statusbar.SetQueryInfo(msg.result.ExecTime, msg.result.RowCount)
			m.statusbar.SetMessage(fmt.Sprintf("Query returned %d rows", msg.result.RowCount), ui.MsgSuccess)
			m.lastError = ""
		} else if msg.execRes != nil {
			m.statusbar.SetQueryInfo(msg.execRes.ExecTime, int(msg.execRes.RowsAffected))
			m.statusbar.SetMessage(fmt.Sprintf("%d rows affected", msg.execRes.RowsAffected), ui.MsgSuccess)
			m.lastError = ""

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
			m.lastError = msg.err.Error()
		} else {
			m.statusbar.SetMessage(fmt.Sprintf("Committed %d changes", msg.count), ui.MsgSuccess)
			m.changes.Clear()
			m.lastError = ""
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
			m.refreshSchemaCache()
			m.changes.Clear()
			m.statusbar.SetMessage(fmt.Sprintf("Reconnected (%d tables)", len(msg.tables)), ui.MsgSuccess)
			// Reload active table if one was selected
			if m.lastTable != "" {
				return m, m.loadTable(m.lastTable)
			}
		}
		return m, nil

	case claudeStreamTokenMsg:
		if m.chatPanel.Visible() {
			m.chatPanel.AppendStreamToken(msg.content)
		}
		return m, nil

	case claudeResponseMsg:
		log.Printf("[app] claudeResponseMsg: response=%d chars, hasDiff=%v, err=%v, chatVisible=%v, claudeVisible=%v",
			len(msg.response), msg.diff != nil, msg.err, m.chatPanel.Visible(), m.claudeModal.Visible())
		if msg.diff != nil {
			log.Printf("[app] diff: oldSQL=%d chars, newSQL=%d chars", len(msg.diff.OldSQL), len(msg.diff.NewSQL))
		}
		m.claudeModal.SetWaiting(false)
		m.chatPanel.SetWaiting(false)
		m.chatPanel.FinishStreaming()

		if msg.err != nil {
			if m.claudeModal.Visible() {
				m.claudeModal.Close()
			}
			if m.chatPanel.Visible() {
				m.chatPanel.AddAssistantMessage("Error: " + msg.err.Error())
			}
			m.statusbar.SetMessage("Claude error: "+msg.err.Error(), ui.MsgError)
		} else if m.claudeModal.Visible() {
			m.claudeModal.Close()
			if msg.diff != nil {
				m.diffModal.Show(msg.diff.OldSQL, msg.diff.NewSQL)
			} else {
				oldSQL := m.claudeModal.GetOriginalSQL()
				m.diffModal.Show(oldSQL, msg.response)
			}
			m.statusbar.SetMessage("Review Claude's suggested changes", ui.MsgSuccess)
		} else if m.chatPanel.Visible() {
			if msg.diff != nil {
				// Backend provided an explicit diff — show in chat and editor.
				if msg.response != "" {
					m.chatPanel.AddAssistantMessage(msg.response)
				}
				m.chatPanel.AddDiffMessage(msg.diff.OldSQL, msg.diff.NewSQL)
				m.editor.ShowDiff(msg.diff.OldSQL, msg.diff.NewSQL)
				m.statusbar.SetMessage("Review changes (y to accept, n to reject)", ui.MsgSuccess)
			} else if msg.response != "" {
				// Text-only response (no SQL change suggested).
				m.chatPanel.AddAssistantMessage(msg.response)
				m.statusbar.SetMessage("Claude responded", ui.MsgSuccess)
			}
		}
		return m, nil

	case ui.ChatPanelAcceptDiffMsg:
		newSQL := m.chatPanel.AcceptDiff()
		m.editor.ClearDiff()
		m.editor.SetValue(newSQL)
		m.statusbar.SetMessage("Changes applied", ui.MsgSuccess)
		return m, nil

	case ui.ChatPanelRejectDiffMsg:
		m.editor.ClearDiff()
		m.statusbar.SetMessage("Changes rejected", ui.MsgInfo)
		return m, nil

	case ui.DiffModalAcceptedMsg:
		m.editor.SetValue(msg.NewSQL)
		m.statusbar.SetMessage("Changes applied", ui.MsgSuccess)
		return m, nil

	case ui.DiffModalRejectedMsg:
		m.statusbar.SetMessage("Changes rejected", ui.MsgError)
		return m, nil
	}

	// Handle diff modal first
	var cmd tea.Cmd
	if m.diffModal.Visible() {
		m.diffModal, cmd = m.diffModal.Update(msg)
		return m, cmd
	}

	// Forward to focused pane
	switch m.activePane {
	case SidebarPane:
		m.sidebar, cmd = m.sidebar.Update(msg)
		m.statusbar.SetSearchMode(m.sidebar.IsSearching())
	case EditorPane:
		m.editor, cmd = m.editor.Update(msg)
		m.refreshEditorColumns()
	case ResultsPane:
		m.results, cmd = m.results.Update(msg)
		m.statusbar.SetEditMode(m.results.IsEditing())
		m.statusbar.SetSearchMode(m.results.IsSearching())
		m.statusbar.SetVisualMode(m.results.IsVisualMode())
	}

	return m, cmd
}

// View renders the full layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	topBar := ui.TopBarStyle.Width(m.width - 2).Render(
		fmt.Sprintf(" %s ", m.db.ConnInfo()),
	)

	availH := m.height - 3

	chatPanelW := 0
	if m.chatPanel.Visible() {
		chatPanelW = 50
		if chatPanelW > m.width/3 {
			chatPanelW = m.width / 3
		}
		m.chatPanel.SetSize(chatPanelW, availH)
	}

	sidebarW := 30
	remainingW := m.width - chatPanelW - sidebarW - 2
	if remainingW < 40 {
		remainingW = 40
		if chatPanelW > 0 {
			chatPanelW = m.width - sidebarW - remainingW - 2
			if chatPanelW < 30 {
				chatPanelW = 30
			}
			m.chatPanel.SetSize(chatPanelW, availH)
		}
	}

	if availH < 6 {
		availH = 6
	}

	editorH := availH * 40 / 100
	if editorH < 5 {
		editorH = 5
	}
	resultsH := availH - editorH

	m.sidebar.SetSize(sidebarW, availH)
	m.editor.SetSize(remainingW, editorH)
	m.results.SetSize(remainingW, resultsH)
	m.statusbar.SetWidth(m.width)

	var mainArea string
	if m.chatPanel.Visible() {
		chatView := m.chatPanel.View()
		sidebarView := m.sidebar.View()
		editorView := m.editor.View()
		resultsView := m.results.View()

		rightPane := lipgloss.JoinVertical(lipgloss.Left, editorView, resultsView)
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightPane, chatView)
	} else {
		sidebarView := m.sidebar.View()
		editorView := m.editor.View()
		resultsView := m.results.View()

		rightPane := lipgloss.JoinVertical(lipgloss.Left, editorView, resultsView)
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, rightPane)
	}

	statusView := m.statusbar.View()

	baseView := lipgloss.JoinVertical(lipgloss.Left, topBar, mainArea, statusView)

	if m.diffModal.Visible() {
		m.diffModal.SetSize(m.width, m.height)
		return m.diffModal.View()
	}

	if m.claudeModal.Visible() {
		return m.claudeModal.View()
	}

	if m.scriptsModal.Visible() {
		m.scriptsModal.SetSize(m.width, m.height)
		return m.scriptsModal.View()
	}

	if m.exportModal.Visible() {
		m.exportModal.SetSize(m.width, m.height)
		return m.exportModal.View()
	}

	return baseView
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

func (m *Model) exportResults(format string, columns []string, rows [][]string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	ext := format
	if ext == "" {
		ext = "csv"
	}
	name := fmt.Sprintf("sqlrat_export_%s.%s", timestamp, ext)
	path, err := filepath.Abs(name)
	if err != nil {
		path = name
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	switch format {
	case ui.ExportFormatJSON:
		recs := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			rec := make(map[string]string)
			for i, col := range columns {
				if i < len(row) {
					rec[col] = row[i]
				} else {
					rec[col] = ""
				}
			}
			recs = append(recs, rec)
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(recs); err != nil {
			return "", err
		}
	case ui.ExportFormatTSV:
		for i, col := range columns {
			if i > 0 {
				_, _ = f.WriteString("\t")
			}
			_, _ = f.WriteString(escapeTSV(col))
		}
		_, _ = f.WriteString("\n")
		for _, row := range rows {
			for i, cell := range row {
				if i > 0 {
					_, _ = f.WriteString("\t")
				}
				_, _ = f.WriteString(escapeTSV(cell))
			}
			_, _ = f.WriteString("\n")
		}
	default:
		w := csv.NewWriter(f)
		if err := w.Write(columns); err != nil {
			return "", err
		}
		for _, row := range rows {
			if err := w.Write(row); err != nil {
				return "", err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return "", err
		}
	}

	return path, nil
}

func escapeTSV(s string) string {
	if strings.ContainsAny(s, "\t\n\"") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
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

// refreshSchemaCache reloads the column cache from the DB.
// Called after DDL or database switches so the in-memory cache stays fresh.
func (m *Model) refreshSchemaCache() {
	if colCache, err := m.db.ListAllColumnNames(); err == nil {
		m.editor.SetColumnCache(colCache)
	}
}

func (m *Model) refreshEditorColumns() {
	stmt := m.editor.GetStatementAtCursor()
	if stmt == "" {
		return
	}
	table := extractTableName(stmt)
	if table == "" {
		return
	}
	tableLower := strings.ToLower(table)
	if tableLower == m.lastColumnRefreshTbl {
		return
	}
	m.lastColumnRefreshTbl = tableLower

	// Use the in-memory schema cache first — no DB query needed.
	if names := m.editor.LookupColumns(tableLower); names != nil {
		m.editor.SetColumnNames(names)
		return
	}

	// Fallback: query DB if table wasn't in cache (e.g. newly created table).
	cols, err := m.db.GetColumns(tableLower)
	if err != nil {
		m.lastColumnRefreshTbl = ""
		return
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	m.editor.SetColumnNames(names)
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

func (m *Model) askClaude(prompt string) tea.Cmd {
	m.claudeModal.SetWaiting(true)
	originalSQL := m.claudeModal.GetOriginalSQL()
	lastError := m.lastError
	tables := m.sidebar.GetTables()

	return func() tea.Msg {
		req := claude.ChatRequest{
			Messages:   []claude.Message{{Role: "user", Content: prompt}},
			CurrentSQL: originalSQL,
			Tables:     tables,
			LastError:  lastError,
		}

		events := m.claudeClient.SendMessageStream(req)
		var response string
		var diff *claude.DiffPayload
		for event := range events {
			switch event.Type {
			case "done":
				if event.Done != nil {
					response = event.Done.FullResponse
					if event.Done.Diff.NewSQL != "" {
						diff = &event.Done.Diff
					}
				}
			case "error":
				return claudeResponseMsg{err: fmt.Errorf("%s", event.Error)}
			}
		}
		return claudeResponseMsg{response: response, diff: diff}
	}
}

func (m *Model) askClaudeChat(prompt string) tea.Cmd {
	m.chatPanel.SetWaiting(true)
	currentSQL := m.chatPanel.GetCurrentSQL()
	lastError := m.lastError
	tables := m.sidebar.GetTables()
	conversationHistory := m.chatPanel.GetConversationHistory()

	return func() tea.Msg {
		var messages []claude.Message

		for _, msg := range conversationHistory {
			messages = append(messages, claude.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}

		messages = append(messages, claude.Message{
			Role:    "user",
			Content: prompt,
		})

		req := claude.ChatRequest{
			Messages:   messages,
			CurrentSQL: currentSQL,
			Tables:     tables,
			LastError:  lastError,
		}

		log.Printf("[askClaudeChat] sending %d messages to backend", len(messages))
		events := m.claudeClient.SendMessageStream(req)
		var response string
		var diff *claude.DiffPayload
		for event := range events {
			log.Printf("[askClaudeChat] event: type=%s", event.Type)
			switch event.Type {
			case "token":
				// Tokens are accumulated; full response is sent at the end.
			case "done":
				if event.Done != nil {
					response = event.Done.FullResponse
					log.Printf("[askClaudeChat] done: fullResponse=%d chars, newSQL=%d chars", len(response), len(event.Done.Diff.NewSQL))
					if event.Done.Diff.NewSQL != "" {
						diff = &event.Done.Diff
					}
				}
			case "error":
				log.Printf("[askClaudeChat] error: %s", event.Error)
				return claudeResponseMsg{err: fmt.Errorf("%s", event.Error)}
			}
		}
		log.Printf("[askClaudeChat] returning response=%d chars, hasDiff=%v", len(response), diff != nil)
		return claudeResponseMsg{response: response, diff: diff}
	}
}
