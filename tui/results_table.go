package tui

// Dynamic results table for KQL query results

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// QueryResults represents the result of a KQL query execution
type QueryResults struct {
	Tables        []interface{}   // Raw API response tables
	Columns       []string        // Dynamic column names
	Rows          [][]interface{} // Dynamic row data
	ExecutionTime time.Duration   // Query performance metrics
	RowCount      int             // Total rows returned
	Error         error           // Execution error if any
}

// ResultsTable manages the display of query results
type ResultsTable struct {
	table   table.Model
	results *QueryResults
	focused bool
	width   int
	height  int
}

// NewResultsTable creates a new results table
func NewResultsTable(width, height int) *ResultsTable {
	// Create table with default columns
	columns := []table.Column{
		{Title: "No Data", Width: width - 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(height-2),
	)

	// Set table styles
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &ResultsTable{
		table:   t,
		results: nil,
		focused: false,
		width:   width,
		height:  height,
	}
}

// Focus sets the table as focused
func (rt *ResultsTable) Focus() {
	rt.focused = true
	rt.table.Focus()
}

// Blur removes focus from the table
func (rt *ResultsTable) Blur() {
	rt.focused = false
	rt.table.Blur()
}

// IsFocused returns whether the table is focused
func (rt *ResultsTable) IsFocused() bool {
	return rt.focused
}

// SetSize updates the table dimensions
func (rt *ResultsTable) SetSize(width, height int) {
	rt.width = width
	rt.height = height
	rt.table.SetHeight(height - 2)
	rt.updateColumns()
}

// SetResults updates the table with new query results
func (rt *ResultsTable) SetResults(results *QueryResults) {
	rt.results = results
	rt.updateTable()
}

// ClearResults clears the table
func (rt *ResultsTable) ClearResults() {
	rt.results = nil
	rt.updateTable()
}

// HasResults returns whether the table has results to display
func (rt *ResultsTable) HasResults() bool {
	return rt.results != nil && rt.results.RowCount > 0
}

// updateTable rebuilds the table based on current results
func (rt *ResultsTable) updateTable() {
	if rt.results == nil || rt.results.Error != nil {
		// Show error state or empty state
		rt.showEmptyState()
		return
	}

	if rt.results.RowCount == 0 {
		rt.showNoResults()
		return
	}

	// Build columns based on results
	rt.updateColumns()

	// Build rows
	rows := make([]table.Row, 0, len(rt.results.Rows))
	for _, rowData := range rt.results.Rows {
		row := make(table.Row, len(rt.results.Columns))
		for i, cellData := range rowData {
			if i < len(row) {
				row[i] = rt.formatCellValue(cellData)
			}
		}
		rows = append(rows, row)
	}

	rt.table.SetRows(rows)
}

// updateColumns creates dynamic columns based on query results
func (rt *ResultsTable) updateColumns() {
	if rt.results == nil || len(rt.results.Columns) == 0 {
		return
	}

	// Calculate column widths dynamically
	availableWidth := rt.width - 4 // Account for borders
	numColumns := len(rt.results.Columns)

	if numColumns == 0 {
		return
	}

	// Minimum column width
	minColWidth := 10
	maxColWidth := 40

	// Calculate equal distribution first
	equalWidth := availableWidth / numColumns
	if equalWidth < minColWidth {
		equalWidth = minColWidth
	} else if equalWidth > maxColWidth {
		equalWidth = maxColWidth
	}

	columns := make([]table.Column, numColumns)
	for i, colName := range rt.results.Columns {
		width := equalWidth

		// Adjust width based on column name length
		if len(colName) > width-2 {
			width = len(colName) + 2
		}

		// Clamp to min/max
		if width < minColWidth {
			width = minColWidth
		} else if width > maxColWidth {
			width = maxColWidth
		}

		columns[i] = table.Column{
			Title: colName,
			Width: width,
		}
	}

	rt.table.SetColumns(columns)
}

// showEmptyState displays when no results are available
func (rt *ResultsTable) showEmptyState() {
	columns := []table.Column{
		{Title: "Status", Width: rt.width - 10},
	}

	message := "No query executed yet"
	if rt.results != nil && rt.results.Error != nil {
		message = fmt.Sprintf("Query failed: %v", rt.results.Error)
	}

	rows := []table.Row{
		{message},
	}

	rt.table.SetColumns(columns)
	rt.table.SetRows(rows)
}

// showNoResults displays when query returned no data
func (rt *ResultsTable) showNoResults() {
	columns := []table.Column{
		{Title: "Results", Width: rt.width - 10},
	}

	rows := []table.Row{
		{"Query executed successfully but returned no results"},
	}

	rt.table.SetColumns(columns)
	rt.table.SetRows(rows)
}

// formatCellValue formats a cell value for display
func (rt *ResultsTable) formatCellValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		// Truncate long strings
		if len(v) > 50 {
			return v[:47] + "..."
		}
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		// Convert to string for other types
		str := fmt.Sprintf("%v", v)
		if len(str) > 50 {
			return str[:47] + "..."
		}
		return str
	}
}

// Update handles bubble tea messages for the table
func (rt *ResultsTable) Update(msg tea.Msg) (*ResultsTable, tea.Cmd) {
	if !rt.focused {
		return rt, nil
	}

	var cmd tea.Cmd
	rt.table, cmd = rt.table.Update(msg)
	return rt, cmd
}

// View renders the results table
func (rt *ResultsTable) View() string {
	var (
		titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Bold(true)

		borderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("24"))

		statusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))
	)

	var content strings.Builder

	// Title and status
	title := "Query Results"
	if rt.results != nil {
		if rt.results.Error != nil {
			title += " (Error)"
		} else {
			statusInfo := fmt.Sprintf("(%d rows", rt.results.RowCount)
			if rt.results.ExecutionTime > 0 {
				statusInfo += fmt.Sprintf(", %v", rt.results.ExecutionTime.Round(time.Millisecond))
			}
			statusInfo += ")"
			title += " " + statusStyle.Render(statusInfo)
		}
	}

	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n")

	// Table
	content.WriteString(rt.table.View())

	return borderStyle.Render(content.String())
}
