package appinsights

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

const testAppID = "test-app-id"

// mockAuthenticator implements ApplicationInsightsAuthenticator for testing
type mockAuthenticator struct {
	token *oauth2.Token
	err   error
}

func (m *mockAuthenticator) GetApplicationInsightsToken(ctx context.Context) (*oauth2.Token, error) {
	return m.token, m.err
}

func TestNewClient(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}

	client := NewClient(token, testAppID)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.token != token {
		t.Error("Expected token to be set correctly")
	}

	if client.appID != testAppID {
		t.Errorf("Expected appID %q, got %q", testAppID, client.appID)
	}

	if client.baseURL != "https://api.applicationinsights.io" {
		t.Errorf("Expected base URL to be https://api.applicationinsights.io, got %q", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if client.auth != nil {
		t.Error("Expected auth to be nil for regular client")
	}
}

func TestNewClientWithAuthenticator(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}
	auth := &mockAuthenticator{token: token}

	client := NewClientWithAuthenticator(auth, testAppID)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.token != nil {
		t.Error("Expected token to be nil initially for authenticator client")
	}

	if client.appID != testAppID {
		t.Errorf("Expected appID %q, got %q", testAppID, client.appID)
	}

	if client.baseURL != "https://api.applicationinsights.io" {
		t.Errorf("Expected base URL to be https://api.applicationinsights.io, got %q", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if client.auth != auth {
		t.Error("Expected auth to be set correctly")
	}
}

func TestGetValidToken(t *testing.T) {
	t.Run("valid stored token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken: "test-token",
			Expiry:      time.Now().Add(time.Hour),
		}
		client := NewClient(token, testAppID)

		validToken, err := client.getValidToken(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if validToken != token {
			t.Error("Expected to get the stored token")
		}
	})

	t.Run("expired token with authenticator", func(t *testing.T) {
		newToken := &oauth2.Token{
			AccessToken: "new-token",
			Expiry:      time.Now().Add(time.Hour),
		}
		auth := &mockAuthenticator{token: newToken}
		client := NewClientWithAuthenticator(auth, testAppID)

		// Set an expired token
		client.token = &oauth2.Token{
			AccessToken: "expired-token",
			Expiry:      time.Now().Add(-time.Hour),
		}

		validToken, err := client.getValidToken(context.Background())
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if validToken != newToken {
			t.Error("Expected to get the new token from authenticator")
		}
		if client.token != newToken {
			t.Error("Expected token to be cached in client")
		}
	})

	t.Run("no token and no authenticator", func(t *testing.T) {
		client := NewClient(nil, testAppID)

		_, err := client.getValidToken(context.Background())
		if err == nil {
			t.Error("Expected error when no token and no authenticator")
		}
	})
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

func TestApplyFetchLimitIfNeeded(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		fetch   int
		applied bool
		want    string
		reason  string
	}{
		{name: "simple traces adds take", query: "traces", fetch: 50, applied: true, want: "traces | take 50", reason: "applied_table_no_user_limit"},
		{name: "simple requests adds take", query: "requests | where timestamp > ago(1h)", fetch: 25, applied: true, want: "requests | where timestamp > ago(1h) | take 25", reason: "applied_table_no_user_limit"},
		{name: "existing take not overridden", query: "traces | take 10", fetch: 100, applied: false, want: "traces | take 10", reason: "user_explicit"},
		{name: "existing limit not overridden", query: "traces | limit 5", fetch: 100, applied: false, want: "traces | limit 5", reason: "user_explicit"},
		{name: "case insensitive TAKE not overridden", query: "traces | TAKE 7", fetch: 9, applied: false, want: "traces | TAKE 7", reason: "user_explicit"},
		{name: "semicolon preserves remainder", query: "traces | where something; requests", fetch: 15, applied: true, want: "traces | where something | take 15; requests", reason: "applied_table_no_user_limit"},
		{name: "multiline table first", query: "traces\n| where timestamp > ago(1h)", fetch: 30, applied: true, want: "traces\n| where timestamp > ago(1h) | take 30", reason: "applied_table_no_user_limit"},
		{name: "not a table first token", query: "let x = traces; x | take 5", fetch: 20, applied: false, want: "let x = traces; x | take 5", reason: "not_table_first"},
		{name: "empty first statement", query: "  ; traces", fetch: 20, applied: false, want: "  ; traces", reason: "empty_stmt"},
		{name: "fetch zero no-op", query: "traces", fetch: 0, applied: false, want: "traces", reason: "fetch_zero"},
		{name: "whitespace preserved", query: "  traces  \t", fetch: 3, applied: true, want: "  traces  \t | take 3", reason: "applied_table_no_user_limit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, applied, reason := applyFetchLimitIfNeeded(tc.query, tc.fetch)
			if applied != tc.applied {
				t.Fatalf("applied mismatch: got %v want %v (reason=%s)", applied, tc.applied, reason)
			}
			if strings.TrimSpace(got) != strings.TrimSpace(tc.want) {
				t.Fatalf("query mutated wrong.\n got: %q\nwant: %q", got, tc.want)
			}
			if reason != tc.reason {
				t.Fatalf("reason mismatch: got %q want %q", reason, tc.reason)
			}
		})
	}
}
