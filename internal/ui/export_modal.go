package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	ExportFormatCSV  = "csv"
	ExportFormatJSON = "json"
	ExportFormatTSV  = "tsv"
)

type ExportFormatSelectedMsg struct {
	Format  string
	Columns []string
	Rows    [][]string
}

type ExportModalClosedMsg struct{}

type ExportModalModel struct {
	visible bool
	cursor  int
	columns []string
	rows    [][]string
	width   int
	height  int
}

var exportFormats = []struct {
	id   string
	name string
	desc string
}{
	{ExportFormatCSV, "CSV", "Comma-separated values"},
	{ExportFormatJSON, "JSON", "JavaScript Object Notation"},
	{ExportFormatTSV, "TSV", "Tab-separated values"},
}

func NewExportModalModel() ExportModalModel {
	return ExportModalModel{}
}

func (m *ExportModalModel) Open(columns []string, rows [][]string) {
	m.visible = true
	m.cursor = 0
	m.columns = columns
	m.rows = rows
}

func (m *ExportModalModel) Close() {
	m.visible = false
	m.columns = nil
	m.rows = nil
}

func (m ExportModalModel) Visible() bool {
	return m.visible
}

func (m *ExportModalModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ExportModalModel) Init() tea.Cmd {
	return nil
}

func (m ExportModalModel) Update(msg tea.Msg) (ExportModalModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Close()
			return m, func() tea.Msg { return ExportModalClosedMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(exportFormats)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor >= 0 && m.cursor < len(exportFormats) {
				f := exportFormats[m.cursor]
				cols := make([]string, len(m.columns))
				copy(cols, m.columns)
				rows := make([][]string, len(m.rows))
				for i, r := range m.rows {
					row := make([]string, len(r))
					copy(row, r)
					rows[i] = row
				}
				m.Close()
				return m, func() tea.Msg {
					return ExportFormatSelectedMsg{Format: f.id, Columns: cols, Rows: rows}
				}
			}
		}
	}

	return m, nil
}

func (m ExportModalModel) View() string {
	if !m.visible {
		return ""
	}

	modalW := 50
	if m.width > 0 && modalW > m.width-4 {
		modalW = m.width - 4
	}

	var b strings.Builder

	title := HeaderStyle.Render("Export Results")
	b.WriteString(title)
	b.WriteString("\n")

	b.WriteString(DimText.Render(fmt.Sprintf("  %d rows × %d columns", len(m.rows), len(m.columns))))
	b.WriteString("\n\n")

	b.WriteString(DimText.Render("  Select format:"))
	b.WriteString("\n\n")

	for i, f := range exportFormats {
		line := "  " + f.name + " – " + f.desc
		if i == m.cursor {
			b.WriteString(SidebarCursorItem.Width(modalW - 4).Render(line))
		} else {
			b.WriteString(SidebarTableItem.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(DimText.Render("  Enter select | Esc cancel"))
	b.WriteString("\n")

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(modalW)

	rendered := modalStyle.Render(b.String())

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
