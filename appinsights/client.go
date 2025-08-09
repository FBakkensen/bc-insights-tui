package appinsights

// Application Insights API client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/FBakkensen/bc-insights-tui/logging"
)

// ApplicationInsightsAuthenticator represents an interface for acquiring Application Insights API tokens
type ApplicationInsightsAuthenticator interface {
	GetApplicationInsightsToken(ctx context.Context) (*oauth2.Token, error)
}

// Client represents an Application Insights API client
type Client struct {
	httpClient *http.Client
	token      *oauth2.Token
	appID      string
	baseURL    string
	auth       ApplicationInsightsAuthenticator
	mu         sync.Mutex // Protects token field from concurrent access
}

// QueryRequest represents a KQL query request
type QueryRequest struct {
	Query string `json:"query"`
}

// QueryResponse represents the response from Application Insights API
type QueryResponse struct {
	Tables []Table `json:"tables"`
}

// Table represents a result table from the API
type Table struct {
	Name    string          `json:"name"`
	Columns []Column        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

// Column represents a column in a result table
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// NewClient creates a new Application Insights client
func NewClient(token *oauth2.Token, appID string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:   token,
		appID:   appID,
		baseURL: "https://api.applicationinsights.io",
	}
}

// NewClientWithAuthenticator creates a new Application Insights client that will automatically
// acquire the proper Application Insights API token using the v1 endpoint
func NewClientWithAuthenticator(authenticator ApplicationInsightsAuthenticator, appID string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:   nil, // Will be acquired on-demand
		appID:   appID,
		baseURL: "https://api.applicationinsights.io",
		auth:    authenticator,
	}
}

// ExecuteQuery executes a KQL query against Application Insights
func (c *Client) ExecuteQuery(ctx context.Context, query string) (*QueryResponse, error) {
	// Get a valid token, either from stored token or via authenticator
	token, err := c.getValidToken(ctx)
	if err != nil {
		logging.Error("KQL token acquisition failed", "error", err.Error())
		return nil, fmt.Errorf("failed to get valid authentication token: %w", err)
	}

	// Prepare the request
	reqBody := QueryRequest{Query: query}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		logging.Error("KQL request marshal failed", "error", err.Error())
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the URL
	queryURL := fmt.Sprintf("%s/v1/apps/%s/query", c.baseURL, c.appID)

	// Log preflight details (redact query text; include hash-length only)
	deadline, hasDeadline := ctx.Deadline()
	logging.Debug("KQL preflight",
		"url", queryURL,
		"appId_len", fmt.Sprintf("%d", len(strings.TrimSpace(c.appID))),
		"body_bytes", fmt.Sprintf("%d", len(bodyJSON)),
		"timeout_set", fmt.Sprintf("%v", hasDeadline),
		"deadline", func() string {
			if hasDeadline {
				return deadline.Format(time.RFC3339)
			}
			return ""
		}(),
	)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", queryURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		logging.Error("KQL request creation failed", "error", err.Error())
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	// Execute request
	start := time.Now()
	logging.Debug("KQL request sending")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Capture whether this is a context deadline / timeout
		sel := ""
		if ctx.Err() != nil {
			sel = ctx.Err().Error()
		}
		logging.Error("KQL request failed", "error", err.Error(), "ctxErr", sel)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.Error("KQL read response failed", "error", err.Error())
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		// Log response metadata (no full body to avoid large logs)
		rid := resp.Header.Get("x-ms-request-id")
		cid := resp.Header.Get("x-ms-correlation-request-id")
		dur := time.Since(start)
		logging.Error("KQL API error",
			"status", fmt.Sprintf("%d", resp.StatusCode),
			"duration_ms", fmt.Sprintf("%d", dur.Milliseconds()),
			"x-ms-request-id", rid,
			"x-ms-correlation-request-id", cid,
			"resp_bytes", fmt.Sprintf("%d", len(body)),
		)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		logging.Error("KQL response parse failed", "error", err.Error(), "resp_bytes", fmt.Sprintf("%d", len(body)))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	dur := time.Since(start)
	rowCount := 0
	colCount := 0
	tableName := ""
	if len(queryResp.Tables) > 0 {
		tableName = queryResp.Tables[0].Name
		colCount = len(queryResp.Tables[0].Columns)
		rowCount = len(queryResp.Tables[0].Rows)
	}
	rid := resp.Header.Get("x-ms-request-id")
	cid := resp.Header.Get("x-ms-correlation-request-id")
	logging.Info("KQL request success",
		"duration_ms", fmt.Sprintf("%d", dur.Milliseconds()),
		"rows", fmt.Sprintf("%d", rowCount),
		"cols", fmt.Sprintf("%d", colCount),
		"table", firstNonEmpty(tableName, "PrimaryResult"),
		"x-ms-request-id", rid,
		"x-ms-correlation-request-id", cid,
	)

	return &queryResp, nil
}

// getValidToken returns a valid token, acquiring one via authenticator if needed
func (c *Client) getValidToken(ctx context.Context) (*oauth2.Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If we have a valid stored token, use it
	if c.token != nil && c.token.Valid() {
		logging.Debug("Using cached App Insights token", "expires", c.token.Expiry.Format(time.RFC3339))
		return c.token, nil
	}

	// If we have an authenticator, use it to get a new token
	if c.auth != nil {
		logging.Debug("Acquiring new App Insights token via authenticator")
		token, err := c.auth.GetApplicationInsightsToken(ctx)
		if err != nil {
			logging.Error("Failed to acquire App Insights token", "error", err.Error())
			return nil, fmt.Errorf("failed to acquire Application Insights token: %w", err)
		}
		// Cache the token for future use
		c.token = token
		if token != nil {
			logging.Debug("Acquired App Insights token", "expires", token.Expiry.Format(time.RFC3339))
		}
		return token, nil
	}

	// No valid token available
	return nil, fmt.Errorf("no valid authentication token available and no authenticator provided")
}

// ValidateQuery performs basic KQL syntax validation
func (c *Client) ValidateQuery(query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Basic syntax checks
	if !c.containsValidTable(query) {
		return fmt.Errorf("query must start with a valid table name (traces, requests, dependencies, exceptions, etc.)")
	}

	// Check for balanced brackets
	if err := c.checkBalancedBrackets(query); err != nil {
		return err
	}

	// Lightweight validation log
	firstToken := ""
	parts := strings.Fields(query)
	if len(parts) > 0 {
		firstToken = parts[0]
	}
	logging.Debug("KQL validated", "first_token", firstToken, "length", fmt.Sprintf("%d", len(query)))

	return nil
}

// firstNonEmpty returns v if not blank; otherwise fallback.
func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// containsValidTable checks if the query starts with a known table
func (c *Client) containsValidTable(query string) bool {
	lines := strings.Split(query, "\n")
	if len(lines) == 0 {
		return false
	}

	firstLine := strings.TrimSpace(lines[0])
	firstWord := strings.Fields(firstLine)
	if len(firstWord) == 0 {
		return false
	}

	knownTables := []string{
		"traces", "requests", "dependencies", "exceptions",
		"pageviews", "browsertimings", "customevents", "custommetrics",
		"performancecounters", "availabilityresults",
	}

	tableName := strings.ToLower(firstWord[0])
	for _, known := range knownTables {
		if tableName == known {
			return true
		}
	}

	return false
}

// checkBalancedBrackets validates bracket matching
func (c *Client) checkBalancedBrackets(query string) error {
	stack := make([]rune, 0)
	brackets := map[rune]rune{
		')': '(',
		']': '[',
		'}': '{',
	}

	for _, char := range query {
		switch char {
		case '(', '[', '{':
			stack = append(stack, char)
		case ')', ']', '}':
			if len(stack) == 0 {
				return fmt.Errorf("unmatched closing bracket: %c", char)
			}
			expected := brackets[char]
			if stack[len(stack)-1] != expected {
				return fmt.Errorf("mismatched brackets: expected %c but found %c", expected, char)
			}
			stack = stack[:len(stack)-1]
		}
	}

	if len(stack) > 0 {
		return fmt.Errorf("unmatched opening bracket: %c", stack[len(stack)-1])
	}

	return nil
}
