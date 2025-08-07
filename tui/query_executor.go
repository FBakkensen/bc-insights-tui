package tui

// Query execution engine for KQL Editor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	tea "github.com/charmbracelet/bubbletea"
)

// QueryExecutor manages the execution of KQL queries
type QueryExecutor struct {
	authenticator *auth.Authenticator
	appID         string
	executing     bool
	lastQuery     string
	lastError     error
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(authenticator *auth.Authenticator, appID string) *QueryExecutor {
	return &QueryExecutor{
		authenticator: authenticator,
		appID:         appID,
		executing:     false,
		lastQuery:     "",
		lastError:     nil,
	}
}

// IsExecuting returns whether a query is currently executing
func (qe *QueryExecutor) IsExecuting() bool {
	return qe.executing
}

// GetLastError returns the last execution error
func (qe *QueryExecutor) GetLastError() error {
	return qe.lastError
}

// ExecuteQuery executes a KQL query asynchronously
func (qe *QueryExecutor) ExecuteQuery(query string, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Get current token
		token, err := qe.authenticator.GetValidToken(ctx)
		if err != nil {
			return QueryResultMsg{
				Query:    query,
				Results:  nil,
				Error:    fmt.Errorf("authentication failed: %w", err),
				Duration: time.Since(startTime),
			}
		}

		// Create client
		client := appinsights.NewClient(token, qe.appID)

		// Validate query first
		if validationErr := client.ValidateQuery(query); validationErr != nil {
			return QueryResultMsg{
				Query:    query,
				Results:  nil,
				Error:    fmt.Errorf("query validation failed: %w", validationErr),
				Duration: time.Since(startTime),
			}
		}

		// Execute query
		response, err := client.ExecuteQuery(ctx, query)
		if err != nil {
			return QueryResultMsg{
				Query:    query,
				Results:  nil,
				Error:    err,
				Duration: time.Since(startTime),
			}
		}

		// Convert response to QueryResults
		results := convertToQueryResults(response, time.Since(startTime))

		return QueryResultMsg{
			Query:    query,
			Results:  results,
			Error:    nil,
			Duration: time.Since(startTime),
		}
	}
}

// convertToQueryResults converts API response to TUI QueryResults
func convertToQueryResults(response *appinsights.QueryResponse, duration time.Duration) *QueryResults {
	if response == nil || len(response.Tables) == 0 {
		return &QueryResults{
			Tables:        nil,
			Columns:       []string{},
			Rows:          [][]interface{}{},
			ExecutionTime: duration,
			RowCount:      0,
			Error:         nil,
		}
	}

	// Use the first table for now (KQL queries typically return one table)
	table := response.Tables[0]

	// Extract column names
	columns := make([]string, len(table.Columns))
	for i, col := range table.Columns {
		columns[i] = col.Name
	}

	// Convert rows
	rows := make([][]interface{}, len(table.Rows))
	copy(rows, table.Rows)

	// Convert tables to interface{} slice
	tables := make([]interface{}, len(response.Tables))
	for i, table := range response.Tables {
		tables[i] = table
	}

	return &QueryResults{
		Tables:        tables,
		Columns:       columns,
		Rows:          rows,
		ExecutionTime: duration,
		RowCount:      len(rows),
		Error:         nil,
	}
}

// QueryResultMsg is sent when query execution completes
type QueryResultMsg struct {
	Query    string
	Results  *QueryResults
	Error    error
	Duration time.Duration
}

// QueryStartedMsg is sent when query execution starts
type QueryStartedMsg struct {
	Query string
}

// formatQueryError formats query errors for user display
func formatQueryError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Provide user-friendly error messages
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401") {
		return "Authentication expired. Please re-authenticate using the 'auth' command."
	}

	if strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "403") {
		return "Access denied. Check your Azure permissions for Application Insights data access."
	}

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "context deadline exceeded") {
		return "Query timed out. Try simplifying your query or reducing the time range."
	}

	if strings.Contains(errStr, "syntax") || strings.Contains(errStr, "parse") {
		return fmt.Sprintf("KQL syntax error: %s", errStr)
	}

	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "404") {
		return "Application Insights resource not found. Check your configuration."
	}

	// Generic error message with the original error
	return fmt.Sprintf("Query execution failed: %s", errStr)
}
