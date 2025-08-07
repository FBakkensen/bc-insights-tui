package appinsights

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestNewClient(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}
	appID := "test-app-id"

	client := NewClient(token, appID)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.token != token {
		t.Error("Expected token to be set correctly")
	}

	if client.appID != appID {
		t.Errorf("Expected appID %q, got %q", appID, client.appID)
	}

	if client.baseURL != "https://api.applicationinsights.io" {
		t.Errorf("Expected base URL to be https://api.applicationinsights.io, got %q", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}

func TestValidateQuery_EmptyQuery(t *testing.T) {
	client := NewClient(nil, "test-app")

	tests := []string{
		"",
		"   ",
		"\n\t  \n",
	}

	for _, query := range tests {
		err := client.ValidateQuery(query)
		if err == nil {
			t.Errorf("Expected error for empty query %q", query)
		}
		if err.Error() != "query cannot be empty" {
			t.Errorf("Expected 'query cannot be empty', got %q", err.Error())
		}
	}
}

func TestValidateQuery_ValidTables(t *testing.T) {
	client := NewClient(nil, "test-app")

	validQueries := []string{
		"traces | limit 10",
		"requests | where timestamp > ago(1h)",
		"dependencies | project name, duration",
		"exceptions | summarize count() by type",
		"customEvents | order by timestamp desc",
	}

	for _, query := range validQueries {
		err := client.ValidateQuery(query)
		if err != nil {
			t.Errorf("Expected no error for valid query %q, got %v", query, err)
		}
	}
}

func TestValidateQuery_InvalidTables(t *testing.T) {
	client := NewClient(nil, "test-app")

	invalidQueries := []string{
		"invalidtable | limit 10",
		"notatable | where something = true",
		"123invalid | project name",
	}

	for _, query := range invalidQueries {
		err := client.ValidateQuery(query)
		if err == nil {
			t.Errorf("Expected error for invalid query %q", query)
		}
	}
}

func TestValidateQuery_BracketMatching(t *testing.T) {
	client := NewClient(nil, "test-app")

	tests := []struct {
		query       string
		shouldError bool
		description string
	}{
		{
			query:       "traces | where (timestamp > ago(1h))",
			shouldError: false,
			description: "valid parentheses",
		},
		{
			query:       "traces | where customDimensions['key'] == 'value'",
			shouldError: false,
			description: "valid square brackets",
		},
		{
			query:       "traces | where (timestamp > ago(1h)",
			shouldError: true,
			description: "unmatched opening parenthesis",
		},
		{
			query:       "traces | where timestamp > ago(1h))",
			shouldError: true,
			description: "unmatched closing parenthesis",
		},
		{
			query:       "traces | where customDimensions['key' == 'value'",
			shouldError: true,
			description: "unmatched square bracket",
		},
		{
			query:       "traces | where (customDimensions['key'] == 'value']",
			shouldError: true,
			description: "mismatched brackets",
		},
	}

	for _, test := range tests {
		err := client.ValidateQuery(test.query)
		if test.shouldError && err == nil {
			t.Errorf("Expected error for %s: %q", test.description, test.query)
		}
		if !test.shouldError && err != nil {
			t.Errorf("Expected no error for %s: %q, got %v", test.description, test.query, err)
		}
	}
}

func TestQueryStructures(t *testing.T) {
	// Test QueryRequest
	req := QueryRequest{Query: "traces | limit 10"}
	if req.Query != "traces | limit 10" {
		t.Errorf("Expected query to be set correctly, got %q", req.Query)
	}

	// Test Table structure
	table := Table{
		Name: "PrimaryResult",
		Columns: []Column{
			{Name: "timestamp", Type: "datetime"},
			{Name: "message", Type: "string"},
		},
		Rows: [][]interface{}{
			{"2023-01-01T10:00:00Z", "test message"},
		},
	}

	if table.Name != "PrimaryResult" {
		t.Errorf("Expected table name 'PrimaryResult', got %q", table.Name)
	}
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
	if len(table.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(table.Rows))
	}

	// Test QueryResponse
	response := QueryResponse{
		Tables: []Table{table},
	}
	if len(response.Tables) != 1 {
		t.Errorf("Expected 1 table in response, got %d", len(response.Tables))
	}
}
