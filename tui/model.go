package tui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
	util "github.com/FBakkensen/bc-insights-tui/internal/util"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

// string constants used for logging mode names
const (
	logModeChat    = "chat"
	logModeEditor  = "editor"
	logModeUnknown = "unknown"
	logModeTable   = "table"
	logModeListSub = "list_subs"
	logModeListAI  = "list_insights"
)

// model implements the chat-first UI with a top viewport (scrollback) and bottom textarea (input).
// It wires Step 1: Azure OAuth2 Device Flow login.

type model struct {
	width  int
	height int

	vp viewport.Model
	ta textarea.Model

	// top panel alternative components
	list       list.Model
	tbl        table.Model
	mode       uiMode
	returnMode uiMode // stores the authoring mode before opening table view
	cfg        config.Config

	// chat content
	content string

	// when true we auto-scroll to bottom on new content; toggled off when
	// the user scrolls up, and back on when they return to bottom.
	followTail bool

	// auth
	authenticator *auth.Authenticator
	authState     auth.AuthState
	deviceResp    *auth.DeviceCodeResponse
	authCtx       context.Context
	cancelAuth    context.CancelFunc

	quitting bool

	// styling/layout
	maxContentWidth int
	vpStyle         lipgloss.Style
	containerStyle  lipgloss.Style

	// KQL state (Step 5)
	kqlClient    KQLClient
	lastColumns  []appinsights.Column
	lastRows     [][]interface{}
	lastTable    string
	lastDuration time.Duration
	haveResults  bool
	runningKQL   bool

	// editor (Step 6)
	editorDesiredHeight int
	origPrompt          string
}

type uiMode int

const (
	// modeUnknown represents an unset/invalid mode sentinel used for returnMode
	modeUnknown uiMode = -1
)

const (
	modeChat uiMode = iota
	modeKQLEditor
	modeListSubscriptions
	modeListInsightsResources
	modeTableResults
)

// config keys used in TUI (mirror of config.settingAzureSubscriptionID)
const keyAzureSubscriptionID = "azure.subscriptionId"

// UI constants
const (
	titleSelectSubscription = "Select Azure Subscription"
	titleSelectInsights     = "Select Application Insights Resource"
	// Prompts and hints
	promptDefault = "> "
	promptEditor  = "KQL> "
	// Keep Ctrl+Enter in the hint to satisfy existing tests, but also advertise
	// reliable keys (F5/Ctrl+R) that work across terminals on Windows.
	hintEditor = "Ctrl+Enter (Ctrl+M) to run · F5 or Ctrl+R to run · Esc to cancel"
	// Layout minimums
	minViewportHeight = 3
	minEditorHeight   = 3
)

// Minimal interface to App Insights KQL used by the UI (for tests and DI)
type KQLClient interface {
	ValidateQuery(query string) error
	ExecuteQuery(ctx context.Context, query string) (*appinsights.QueryResponse, error)
}

// Run starts the Bubble Tea program with the chat-first model.
func Run(cfg config.Config) error {
	m := initialModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(cfg config.Config) model {
	ta := textarea.New()
	ta.Placeholder = "Type 'login' to authenticate or 'help'"
	ta.ShowLineNumbers = false
	ta.Focus()
	ta.Prompt = promptDefault
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.CursorEnd()
	ta.KeyMap.InsertNewline.SetEnabled(false) // single-line behavior for chat

	vp := viewport.New(80, 20)
	vp.SetContent("")
	vpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63"))

	a := auth.NewAuthenticator(cfg.OAuth2)

	// list setup (will be sized on WindowSizeMsg)
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = titleSelectSubscription
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	m := model{
		vp:                  vp,
		ta:                  ta,
		list:                l,
		tbl:                 table.New(),
		mode:                modeChat,
		returnMode:          modeUnknown,
		cfg:                 cfg,
		authenticator:       a,
		authState:           auth.AuthStateUnknown,
		maxContentWidth:     120,
		vpStyle:             vpStyle,
		containerStyle:      lipgloss.NewStyle(),
		followTail:          true,
		kqlClient:           nil,
		editorDesiredHeight: 8,
		origPrompt:          promptDefault,
	}
	m.append("Welcome to bc-insights-tui (chat-first).")
	m.append("Step 1: Login using Azure Device Flow.")
	// If a valid token exists, mark completed; else prompt user to 'login'.
	if a.HasValidToken() {
		m.authState = auth.AuthStateCompleted
		m.append("You're already authenticated.")
	} else {
		m.append("Type 'login' and press Enter to authenticate.")
	}
	return m
}

// append adds a line to the scrollback and moves viewport to bottom.
func (m *model) append(line string) {
	if m.content == "" {
		m.content = line
	} else {
		m.content += "\n" + line
	}
	// Update viewport content. Auto-scroll only when following the tail.
	m.vp.SetContent(m.content)
	if m.followTail {
		m.vp.GotoBottom()
	}
}

// showConfig displays the current configuration settings in a nicely formatted way
func (m *model) showConfig() {
	m.append("Current Configuration:")

	settings := m.cfg.ListAllSettings()

	// Group settings by category for better readability
	m.append("  Basic Settings:")
	m.appendSetting(settings, "fetchSize", "Log Fetch Size")
	m.appendSetting(settings, "environment", "Environment")

	m.append("  Application Insights:")
	m.appendSetting(settings, "applicationInsightsAppId", "App ID")
	m.appendSetting(settings, "applicationInsightsKey", "Key")

	m.append("  Azure:")
	m.appendSetting(settings, "azure.subscriptionId", "Subscription ID")

	m.append("  OAuth2:")
	m.appendSetting(settings, "oauth2.tenantId", "Tenant ID")
	m.appendSetting(settings, "oauth2.clientId", "Client ID")
	m.appendSetting(settings, "oauth2.scopes", "Scopes")

	m.append("  Query Settings:")
	m.appendSetting(settings, "queryHistoryMaxEntries", "Max History Entries")
	m.appendSetting(settings, "queryTimeoutSeconds", "Query Timeout (seconds)")
	m.appendSetting(settings, "queryHistoryFile", "History File")
	m.appendSetting(settings, "editorPanelRatio", "Editor Panel Ratio")

	m.append("  Debug / Raw Capture:")
	m.appendSetting(settings, "debug.appInsightsRawEnable", "Enabled")
	m.appendSetting(settings, "debug.appInsightsRawFile", "File")
	m.appendSetting(settings, "debug.appInsightsRawMaxBytes", "Max Bytes")
}

// appendSetting appends a formatted key/value if the key exists.
func (m *model) appendSetting(settings map[string]string, key, label string) {
	if val, ok := settings[key]; ok {
		m.append("    " + label + ": " + val)
	}
}

// showKeys appends a quick reference of active keybindings for major modes
func (m *model) showKeys() {
	m.append("Keybindings:")
	m.append("  Global:")
	// spacing aligned for readability
	m.append("    Esc / Ctrl+C    — Quit (or close panel)")
	m.append("    F6              — Open last results interactively (in Chat/Editor)")
	m.append("  Chat mode:")
	m.append("    Enter            — Submit command (e.g., 'edit', 'subs', 'resources', 'config')")
	m.append("  Editor mode:")
	m.append("    Enter            — Insert newline")
	m.append("    F5 or Ctrl+R     — Run query")
	m.append("    Ctrl+Enter       — Run (may arrive as Ctrl+M in some terminals)")
	m.append("    Esc              — Cancel edit")
	m.append("  List panels (subscriptions/resources):")
	m.append("    Up/Down, PgUp/PgDn — Navigate · Enter — Select · Esc — Close")
	m.append("  Results table:")
	m.append("    Up/Down/Left/Right — Navigate · Home/End — Jump · Esc — Close")
}

// msgs used by the update loop

type (
	deviceCodeMsg struct {
		resp   *auth.DeviceCodeResponse
		ctx    context.Context
		cancel context.CancelFunc
	}
	authSuccessMsg struct{}
	authErrorMsg   struct{ err error }
	subsLoadedMsg  struct {
		items []list.Item
		err   error
	}
	insightsLoadedMsg struct {
		items []list.Item
		err   error
	}
	// KQL messages
	kqlResultMsg struct {
		tableName string
		columns   []appinsights.Column
		rows      [][]interface{}
		duration  time.Duration
		err       error
	}
)

// startAuthCmd begins the device flow.
func (m *model) startAuthCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Debug("Starting device flow")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		resp, err := m.authenticator.InitiateDeviceFlow(ctx)
		if err != nil {
			logging.Error("InitiateDeviceFlow failed", "error", err.Error())
			cancel()
			return authErrorMsg{err: err}
		}
		logging.Info("Device flow initiated", "interval_s", fmt.Sprintf("%d", resp.Interval))
		return deviceCodeMsg{resp: resp, ctx: ctx, cancel: cancel}
	}
}

// bottom-aligning for short content intentionally removed for Option 1.

// pollForTokenCmd runs the blocking PollForToken and signals when done.
func (m *model) pollForTokenCmd() tea.Cmd {
	deviceCode := m.deviceResp.DeviceCode
	interval := m.deviceResp.Interval
	ctx := m.authCtx
	return func() tea.Msg {
		logging.Debug("Polling for token start", "interval_s", fmt.Sprintf("%d", interval))
		token, err := m.authenticator.PollForToken(ctx, deviceCode, interval)
		if err != nil {
			sel := ""
			if ctx.Err() != nil {
				sel = ctx.Err().Error()
			}
			logging.Error("PollForToken failed", "error", err.Error(), "ctxErr", sel)
			return authErrorMsg{err: err}
		}
		if err := m.authenticator.SaveTokenSecurely(token); err != nil {
			logging.Error("SaveTokenSecurely failed", "error", err.Error())
			return authErrorMsg{err: err}
		}
		logging.Info("Token saved successfully")
		return authSuccessMsg{}
	}
}

// waitTickCmd emits ticks while waiting during auth.
// (no waiting ticker needed for now)

// loadSubscriptionsCmd loads Azure subscriptions using the Azure client
func (m *model) loadSubscriptionsCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Debug("Starting subscription loading command")

		// Ensure authenticator is present and can provide ARM-scoped tokens
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		logging.Debug("Creating Azure client with authenticator")
		client, err := appinsights.NewAzureClientWithAuthenticator(m.authenticator)
		if err != nil {
			logging.Error("Failed to create Azure client", "error", err.Error())
			return subsLoadedMsg{err: fmt.Errorf("failed to create Azure client: %w", err)}
		}

		logging.Debug("Calling ListSubscriptions")
		subs, err := client.ListSubscriptions(ctx)
		if err != nil {
			logging.Error("Failed to list subscriptions", "error", err.Error())
			return subsLoadedMsg{err: err}
		}

		logging.Info("Successfully retrieved subscriptions", "count", fmt.Sprintf("%d", len(subs)))
		for i, s := range subs {
			logging.Debug("Subscription found", "index", fmt.Sprintf("%d", i), "id", s.ID, "name", s.DisplayName, "state", s.State)
		}

		items := make([]list.Item, 0, len(subs))
		for _, s := range subs {
			// Wrap into list item adapter
			s := s
			items = append(items, subscriptionItem{s: s})
		}

		logging.Debug("Returning subscription items", "itemCount", fmt.Sprintf("%d", len(items)))
		return subsLoadedMsg{items: items}
	}
}

// subscriptionItem adapts AzureSubscription to list.Item
type subscriptionItem struct{ s appinsights.AzureSubscription }

func (i subscriptionItem) FilterValue() string { return i.s.DisplayName }
func (i subscriptionItem) Title() string       { return i.s.DisplayName }
func (i subscriptionItem) Description() string {
	return fmt.Sprintf("ID: %s | State: %s", i.s.ID, i.s.State)
}

// loadInsightsResourcesCmd loads Application Insights resources for the selected subscription using the Azure client
func (m *model) loadInsightsResourcesCmd() tea.Cmd {
	return func() tea.Msg {
		logging.Debug("Starting Application Insights resource loading command")

		// Check if we have a subscription ID set
		subscriptionID := m.cfg.SubscriptionID
		if subscriptionID == "" {
			logging.Error("No subscription ID set for loading insights resources")
			return insightsLoadedMsg{err: fmt.Errorf("no subscription selected: please select a subscription first using 'subs' command")}
		}

		// Ensure authenticator is present and can provide ARM-scoped tokens
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		logging.Debug("Creating Azure client with authenticator for Application Insights resources")
		client, err := appinsights.NewAzureClientWithAuthenticator(m.authenticator)
		if err != nil {
			logging.Error("Failed to create Azure client for insights", "error", err.Error())
			return insightsLoadedMsg{err: fmt.Errorf("failed to create Azure client: %w", err)}
		}

		logging.Debug("Calling ListApplicationInsightsResourcesForSubscription", "subscriptionID", subscriptionID)
		resources, err := client.ListApplicationInsightsResourcesForSubscription(ctx, subscriptionID)
		if err != nil {
			logging.Error("Failed to list Application Insights resources", "error", err.Error())
			return insightsLoadedMsg{err: err}
		}

		logging.Info("Successfully retrieved Application Insights resources", "count", fmt.Sprintf("%d", len(resources)))
		for i, r := range resources {
			logging.Debug("Application Insights resource found",
				"index", fmt.Sprintf("%d", i),
				"name", r.Name,
				"resourceGroup", r.ResourceGroup,
				"applicationID", r.ApplicationID)
		}

		items := make([]list.Item, 0, len(resources))
		for _, r := range resources {
			// Wrap into list item adapter
			r := r
			items = append(items, insightsResourceItem{r: r})
		}

		logging.Debug("Returning Application Insights resource items", "itemCount", fmt.Sprintf("%d", len(items)))
		return insightsLoadedMsg{items: items}
	}
}

// insightsResourceItem adapts ApplicationInsightsResource to list.Item
type insightsResourceItem struct {
	r appinsights.ApplicationInsightsResource
}

func (i insightsResourceItem) FilterValue() string { return i.r.Name }
func (i insightsResourceItem) Title() string       { return i.r.Name }
func (i insightsResourceItem) Description() string {
	return fmt.Sprintf("RG: %s | Location: %s | App ID: %s", i.r.ResourceGroup, i.r.Location, i.r.ApplicationID)
}

// runKQLCmd validates inputs, performs timeout, executes, and returns kqlResultMsg
func (m *model) runKQLCmd(query string) tea.Cmd {
	// capture cfg values
	timeoutSec := m.cfg.QueryTimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	appID := m.cfg.ApplicationInsightsID
	fetch := m.cfg.LogFetchSize
	if fetch <= 0 {
		fetch = 50
	}
	// Logging user action without full query text
	hash := sha256.Sum256([]byte(query))
	qhash := hex.EncodeToString(hash[:8])
	firstToken := ""
	parts := strings.Fields(query)
	if len(parts) > 0 {
		firstToken = parts[0]
	}
	logging.Info("Executing KQL",
		"query_hash", qhash,
		"first_token", firstToken,
		"timeout_s", fmt.Sprintf("%d", timeoutSec),
		"fetch_size", fmt.Sprintf("%d", fetch),
	)

	// Perform preflight outside the closure to avoid capturing m
	if err := m.preflightKQL(appID); err != nil {
		logging.Error("KQL preflight failed", "error", err.Error())
		return func() tea.Msg { return kqlResultMsg{err: err} }
	}
	// Construct client once and capture it immutably for the closure
	client := m.getKQLClient(appID)

	return func() tea.Msg {
		deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
		logging.Debug("KQL command starting",
			"appId_len", fmt.Sprintf("%d", len(strings.TrimSpace(appID))),
			"deadline", deadline.Format(time.RFC3339),
		)
		if err := client.ValidateQuery(query); err != nil {
			logging.Error("KQL validation failed", "error", err.Error())
			return kqlResultMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		start := time.Now()
		resp, err := client.ExecuteQuery(ctx, query)
		dur := time.Since(start)
		if err != nil {
			mapped := mapKQLError(err, timeoutSec, ctx.Err())
			logging.Error("KQL execute failed", "status", "n/a", "error", err.Error())
			if ctx.Err() != nil {
				logging.Error("KQL context error", "ctxErr", ctx.Err().Error(), "duration_ms", fmt.Sprintf("%d", dur.Milliseconds()))
			}
			return kqlResultMsg{duration: dur, err: mapped}
		}
		// Parse results; prefer PrimaryResult if available
		tableName := ""
		var cols []appinsights.Column
		var rows [][]interface{}
		if resp != nil && len(resp.Tables) > 0 {
			idx := 0
			for i, t := range resp.Tables {
				if strings.EqualFold(t.Name, "PrimaryResult") {
					idx = i
					break
				}
			}
			tableName = resp.Tables[idx].Name
			cols = resp.Tables[idx].Columns
			rows = resp.Tables[idx].Rows
		}
		logging.Info("KQL execute success",
			"duration_ms", fmt.Sprintf("%d", dur.Milliseconds()),
			"rows", fmt.Sprintf("%d", len(rows)),
			"cols", fmt.Sprintf("%d", len(cols)),
			"table", util.FirstNonEmpty(tableName, "PrimaryResult"),
		)
		return kqlResultMsg{tableName: tableName, columns: cols, rows: rows, duration: dur}
	}
}

// preflightKQL ensures the user is authenticated and App Id is set
func (m *model) preflightKQL(appID string) error {
	if m.authenticator == nil || !m.authenticator.HasValidToken() {
		logging.Error("KQL preflight auth check failed", "hasAuthenticator", fmt.Sprintf("%v", m.authenticator != nil))
		return fmt.Errorf("authentication required. run 'login' to complete device code sign-in. likely cause: expired or missing token. next: verify you can access Application Insights in the Azure portal: https://portal.azure.com")
	}
	if strings.TrimSpace(appID) == "" {
		logging.Error("KQL preflight appId check failed")
		return fmt.Errorf("application insights app id is not set. run: config set applicationInsightsAppId=<id>. next: find the App ID under your Application Insights resource in Azure portal (Overview > Application ID) or browse resources: https://portal.azure.com/#view/HubsExtension/BrowseResource/resourceType/microsoft.insights%%2Fcomponents")
	}
	logging.Debug("KQL preflight ok")
	return nil
}

// getKQLClient returns an existing injected client or constructs one from the authenticator
func (m *model) getKQLClient(appID string) KQLClient {
	if m.kqlClient != nil {
		logging.Debug("Using injected KQL client")
		return m.kqlClient
	}
	logging.Debug("Creating new KQL client from authenticator")
	return appinsights.NewClientWithAuthenticator(m.authenticator, appID)
}

// mapKQLError turns raw errors into actionable messages, avoiding trailing punctuation per lints
func mapKQLError(err error, timeoutSec int, ctxErr error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return fmt.Errorf("unauthorized (401). likely cause: token scope/tenant mismatch or missing access. next: try 'login' again and confirm access to the Application Insights resource in Azure portal: https://portal.azure.com/#view/HubsExtension/BrowseResource/resourceType/microsoft.insights%%2Fcomponents")
	case strings.Contains(lower, "403"):
		return fmt.Errorf("forbidden (403). likely cause: insufficient RBAC. next: ensure you have Reader/Log Analytics Reader on the resource/workspace in Azure portal: https://portal.azure.com")
	case strings.Contains(lower, "400") || strings.Contains(lower, "bad request"):
		return fmt.Errorf("bad kql (400). check table names and syntax. next: validate the query in the portal (Logs) or KQL docs: https://learn.microsoft.com/azure/azure-monitor/logs/get-started-queries")
	case strings.Contains(lower, "429") || strings.Contains(lower, "throttle") || strings.Contains(lower, "too many requests"):
		return fmt.Errorf("throttled (429). retry later or reduce result size (e.g., add '| take N'). consider narrowing the time range")
	default:
		if ctxErr == context.DeadlineExceeded {
			return fmt.Errorf("query timed out after %ds. increase queryTimeoutSeconds or simplify the query", timeoutSec)
		}
		if ctxErr == context.Canceled {
			return fmt.Errorf("query canceled. you can retry or simplify the query")
		}
		return err
	}
}

// openTableFromLastResults opens the interactive table view from last results
// if available, while preserving the current authoring mode for return.
func (m *model) openTableFromLastResults() (model, tea.Cmd) {
	if !m.haveResults {
		// No-op with optional hint
		m.append("No results to open.")
		// Log the attempted open for diagnostics (string values only)
		logging.Debug("Interactive open skipped: no results", "action", "open_interactive", "fromMode", func() string {
			switch m.mode {
			case modeChat:
				return logModeChat
			case modeKQLEditor:
				return logModeEditor
			case modeListSubscriptions:
				return logModeListSub
			case modeListInsightsResources:
				return logModeListAI
			case modeTableResults:
				return logModeTable
			default:
				return logModeUnknown
			}
		}())
		return *m, nil
	}

	// Store the current authoring mode for restoration on close
	if m.mode == modeChat || m.mode == modeKQLEditor {
		m.returnMode = m.mode
	}

	// Log the action with metadata
	// Note: logging expects string key-values; stringify counts explicitly
	logging.Info("Opening interactive table from last results",
		"action", "open_interactive",
		"rowCount", fmt.Sprintf("%d", len(m.lastRows)),
		"columnCount", fmt.Sprintf("%d", len(m.lastColumns)),
		"table", util.FirstNonEmpty(m.lastTable, "PrimaryResult"),
		"fromMode", func() string {
			switch m.returnMode {
			case modeChat:
				return logModeChat
			case modeKQLEditor:
				return logModeEditor
			default:
				return logModeUnknown
			}
		}(),
	)

	// Initialize interactive table and switch mode
	m.initInteractiveTable()
	m.mode = modeTableResults
	m.append("Opened results table.")

	return *m, nil
}
