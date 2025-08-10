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

	cfgpkg "github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/debugdump"
	util "github.com/FBakkensen/bc-insights-tui/internal/util"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

// Track last-seen raw capture settings to log changes once per process
var (
	rawCfgMu        sync.Mutex
	rawCfgInit      bool
	lastRawEnabled  bool
	lastRawPath     string
	lastRawMaxBytes int
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
	timeoutSet, deadlineStr := computeDeadlineFields(ctx)
	logging.Debug("KQL preflight",
		"url", queryURL,
		"appId_len", fmt.Sprintf("%d", len(strings.TrimSpace(c.appID))),
		"body_bytes", fmt.Sprintf("%d", len(bodyJSON)),
		"timeout_set", timeoutSet,
		"deadline", deadlineStr,
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

	// Prepare optional raw debug capture
	rawEnabled, resolvedPath, maxBytes := getRawCaptureConfig()
	checkAndLogRawConfigChange(rawEnabled, resolvedPath, maxBytes)

	// If enabled, write request-only capture first
	if rawEnabled {
		writeRawRequestCapture(req, bodyJSON, maxBytes, resolvedPath)
	}

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
		// Write error-only capture if enabled
		if rawEnabled {
			writeRawTransportError(req, bodyJSON, maxBytes, resolvedPath, err, time.Since(start))
		}
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
		// Write full capture if enabled
		if rawEnabled {
			writeRawHTTPResult(req, bodyJSON, resp, body, maxBytes, resolvedPath, dur, fmt.Sprintf("API request failed with status %d", resp.StatusCode))
		}
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
		"table", util.FirstNonEmpty(tableName, "PrimaryResult"),
		"x-ms-request-id", rid,
		"x-ms-correlation-request-id", cid,
	)

	// Write success capture if enabled
	if rawEnabled {
		writeRawHTTPResult(req, bodyJSON, resp, body, maxBytes, resolvedPath, dur, "")
	}

	return &queryResp, nil
}

// computeDeadlineFields returns (timeout_set, deadline) strings for logging.
func computeDeadlineFields(ctx context.Context) (string, string) {
	if d, ok := ctx.Deadline(); ok {
		return "true", d.Format(time.RFC3339)
	}
	return "false", ""
}

// getRawCaptureConfig reads config/env to determine raw capture enablement and resolves path.
func getRawCaptureConfig() (bool, string, int) {
	cfg := cfgpkg.LoadConfigWithArgs(nil)
	if !cfg.DebugAppInsightsRawEnable {
		return false, "", cfg.DebugAppInsightsRawMaxBytes
	}
	resolvedPath, rerr := debugdump.ResolvePath(cfg.DebugAppInsightsRawFile)
	if rerr != nil {
		logging.Warn("AI raw path resolution failed", "error", rerr.Error())
		return false, "", cfg.DebugAppInsightsRawMaxBytes
	}
	return true, resolvedPath, cfg.DebugAppInsightsRawMaxBytes
}

// checkAndLogRawConfigChange logs when the raw capture settings change (old -> new) once per process run.
func checkAndLogRawConfigChange(enabled bool, path string, maxBytes int) {
	rawCfgMu.Lock()
	defer rawCfgMu.Unlock()
	if !rawCfgInit {
		// Initialize without logging
		rawCfgInit = true
		lastRawEnabled = enabled
		lastRawPath = path
		lastRawMaxBytes = maxBytes
		return
	}
	changed := false
	enabledOld := lastRawEnabled
	pathOld := lastRawPath
	maxBytesOld := lastRawMaxBytes
	if enabled != lastRawEnabled {
		changed = true
		lastRawEnabled = enabled
	}
	if strings.TrimSpace(path) != strings.TrimSpace(lastRawPath) {
		changed = true
		lastRawPath = path
	}
	if maxBytes != lastRawMaxBytes {
		changed = true
		lastRawMaxBytes = maxBytes
	}
	if changed {
		logging.Info("ai_raw_dump_settings_changed",
			"enabled_old", fmt.Sprintf("%t", enabledOld),
			"enabled_new", fmt.Sprintf("%t", enabled),
			"path_old", pathOld,
			"path_new", path,
			"max_bytes_old", fmt.Sprintf("%d", maxBytesOld),
			"max_bytes_new", fmt.Sprintf("%d", maxBytes),
		)
	}
}

// writeRawRequestCapture writes the initial request-only capture and logs a start event.
func writeRawRequestCapture(req *http.Request, bodyJSON []byte, maxBytes int, path string) {
	hdrs := map[string]string{
		"content-type":  req.Header.Get("Content-Type"),
		"authorization": req.Header.Get("Authorization"),
	}
	red := debugdump.RedactHeaders(hdrs)
	// Pretty print request body JSON
	bodyStr, bodyLen, truncated := debugdump.FormatBodyPrettyJSON(bodyJSON, maxBytes)
	cap := debugdump.AIRawCapture{
		Version:    1,
		CapturedAt: debugdump.Now(),
		Request: debugdump.AIRawRequest{
			StartedAt: debugdump.Now(),
			Method:    req.Method,
			URL:       req.URL.String(),
			Headers:   red,
			Body:      bodyStr,
			BodyBytes: bodyLen,
			Truncated: truncated,
		},
		Response: nil,
		Error:    nil,
	}
	if werr := debugdump.WriteAIRawRequest(path, cap); werr != nil {
		logging.Warn("AI raw dump request write failed", "error", werr.Error())
		return
	}
	logging.Info("ai_raw_dump_started", "path", path, "body_bytes", fmt.Sprintf("%d", bodyLen))
}

// writeRawTransportError writes a full capture with request details and error message.
func writeRawTransportError(req *http.Request, bodyJSON []byte, maxBytes int, path string, err error, dur time.Duration) {
	reqHdr := debugdump.RedactHeaders(map[string]string{
		"content-type":  req.Header.Get("Content-Type"),
		"authorization": req.Header.Get("Authorization"),
	})
	reqBodyStr, reqBodyLen, reqTrunc := debugdump.FormatBodyPrettyJSON(bodyJSON, maxBytes)
	cap := debugdump.AIRawCapture{
		Version:    1,
		CapturedAt: debugdump.Now(),
		Request: debugdump.AIRawRequest{
			StartedAt: "",
			Method:    req.Method,
			URL:       req.URL.String(),
			Headers:   reqHdr,
			Body:      reqBodyStr,
			BodyBytes: reqBodyLen,
			Truncated: reqTrunc,
		},
		Response: nil,
		Error:    &debugdump.AIRawError{Message: err.Error()},
	}
	if werr := debugdump.WriteAIRawFull(path, cap); werr != nil {
		logging.Warn("AI raw dump error write failed", "error", werr.Error())
		return
	}
	logging.Info("ai_raw_dump_written", "path", path, "status", "n/a", "duration_ms", fmt.Sprintf("%d", dur.Milliseconds()), "resp_bytes", "0")
}

// writeRawHTTPResult writes a full capture for HTTP responses, optionally including an error message.
func writeRawHTTPResult(req *http.Request, bodyJSON []byte, resp *http.Response, respBody []byte, maxBytes int, path string, dur time.Duration, errMsg string) {
	rh := map[string]string{
		"x-ms-request-id":             resp.Header.Get("x-ms-request-id"),
		"x-ms-correlation-request-id": resp.Header.Get("x-ms-correlation-request-id"),
		"content-type":                resp.Header.Get("Content-Type"),
	}
	redh := debugdump.RedactHeaders(rh)
	// Pretty print response body JSON when possible
	bodyStr, bodyLen, truncated := debugdump.FormatBodyPrettyJSON(respBody, maxBytes)
	full := debugdump.AIRawFullCapture{
		Version:    1,
		CapturedAt: debugdump.Now(),
		Request: debugdump.AIRawRequest{
			StartedAt: "",
			Method:    req.Method,
			URL:       req.URL.String(),
			Headers:   debugdump.RedactHeaders(map[string]string{"content-type": req.Header.Get("Content-Type"), "authorization": req.Header.Get("Authorization")}),
			Body:      func() string { s, _, _ := debugdump.FormatBodyPrettyJSON(bodyJSON, 0); return s }(),
			BodyBytes: len(bodyJSON),
			Truncated: false,
		},
		Response: &debugdump.AIRawResponse{
			CompletedAt: debugdump.Now(),
			Status:      resp.StatusCode,
			DurationMs:  dur.Milliseconds(),
			Headers:     redh,
			Body:        bodyStr,
			BodyBytes:   bodyLen,
			Truncated:   truncated,
		},
	}
	if strings.TrimSpace(errMsg) != "" {
		full.Error = &debugdump.AIRawError{Message: errMsg}
	}
	if werr := debugdump.WriteAIRawFull(path, full); werr != nil {
		logging.Warn("AI raw dump full write failed", "error", werr.Error())
		return
	}
	logging.Info("ai_raw_dump_written", "path", path, "status", fmt.Sprintf("%d", resp.StatusCode), "duration_ms", fmt.Sprintf("%d", dur.Milliseconds()), "resp_bytes", fmt.Sprintf("%d", bodyLen))
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
// firstNonEmpty was moved to internal/util. Use util.FirstNonEmpty instead.

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
