package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

const keyEsc = "esc"

// Init starts device flow automatically if no valid token exists.
func (m model) Init() tea.Cmd {
	if m.authenticator != nil && !m.authenticator.HasValidToken() {
		// Start device flow immediately when auth is required
		logging.Debug("No valid token at Init; starting device flow")
		return m.startAuthCmd()
	}
	logging.Debug("Init complete; token present or authenticator nil")
	return nil
}

// Update handles messages and key events.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
	case tea.KeyMsg:
		return m.handleKeyMessage(msg)
	case submitEditorMsg:
		return m.handleEditorSubmit()
	case deviceCodeMsg:
		return m.handleDeviceCode(msg)
	case authSuccessMsg:
		return m.handleAuthSuccess()
	case authErrorMsg:
		return m.handleAuthError(msg)
	case subsLoadedMsg:
		return m.handleSubsLoaded(msg)
	case insightsLoadedMsg:
		return m.handleInsightsLoaded(msg)
	case kqlResultMsg:
		return m.handleKQLResult(msg)
	}
	// Let child components update
	return m.handleComponentUpdate(msg)
}

func (m model) handleKeyMessage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle F6 globally (from both chat and editor modes)
	if msg.String() == "f6" {
		if m.mode == modeChat || m.mode == modeKQLEditor {
			return m.openTableFromLastResults()
		}
	}

	// When in list mode, handle Esc and selection differently
	if m.mode == modeListSubscriptions || m.mode == modeListInsightsResources {
		return m.handleListKey(msg)
	}
	// Editor mode key handling
	if m.mode == modeKQLEditor {
		if m2, cmd, handled := m.handleEditorKey(msg); handled {
			return m2, cmd
		}
	}
	// When in table mode, handle Esc to return to previous mode; delegate nav to table
	if m.mode == modeTableResults {
		return m.handleTableKey(msg)
	}
	// In chat mode: Let textarea consume keys first so typing works
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	// Then handle global keys and Enter submission
	m2, cmd2 := m.handleKey(msg)
	return m2, tea.Batch(cmd, cmd2)
}

// handleTableKey processes key events in table results mode
func (m model) handleTableKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		// Restore to the mode we were in before opening the table
		previousMode := m.returnMode
		if previousMode == modeChat || previousMode == modeKQLEditor {
			m.mode = previousMode
		} else {
			// Fallback to chat if returnMode is not set or invalid
			m.mode = modeChat
		}

		// Log the close action
		logging.Info("Closing interactive table",
			"action", "close_interactive",
			"returnedToMode", func() string {
				switch m.mode {
				case modeChat:
					return "chat"
				case modeKQLEditor:
					return "editor"
				default:
					return "unknown"
				}
			}(),
		)

		// Clear returnMode
		m.returnMode = 0
		m.append("Closed results table.")
		if m.followTail || m.vp.AtBottom() {
			m.vp.GotoBottom()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

func (m model) handleSubsLoaded(msg subsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logging.Error("Failed to load subscriptions", "error", msg.err.Error())
		m.append("Failed to load subscriptions: " + msg.err.Error())
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Received subsLoadedMsg", "itemCount", fmt.Sprintf("%d", len(msg.items)))
	m.list.SetItems(msg.items)
	if len(msg.items) == 0 {
		m.append("No subscriptions found for this account.")
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Set items to list", "listItemCount", fmt.Sprintf("%d", len(m.list.Items())))
	// Stay in list mode and focus list
	return m, nil
}

func (m model) handleInsightsLoaded(msg insightsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		logging.Error("Failed to load Application Insights resources", "error", msg.err.Error())
		m.append("Failed to load Application Insights resources: " + msg.err.Error())
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Received insightsLoadedMsg", "itemCount", fmt.Sprintf("%d", len(msg.items)))
	m.list.SetItems(msg.items)
	if len(msg.items) == 0 {
		m.append("No Application Insights resources found in the selected subscription.")
		m.mode = modeChat
		return m, nil
	}
	logging.Debug("Set Application Insights items to list", "listItemCount", fmt.Sprintf("%d", len(m.list.Items())))
	// Stay in list mode and focus list
	return m, nil
}

func (m model) handleComponentUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	// Track followTail based on whether user is at bottom before update
	wasAtBottom := m.vp.AtBottom()
	m.vp, _ = m.vp.Update(msg)
	// If user scrolled up, stop following; if they reached bottom, resume
	if wasAtBottom && m.vp.AtBottom() {
		m.followTail = true
	} else if !m.vp.AtBottom() {
		m.followTail = false
	}
	return m, cmd
}

// handleEditorKey processes key events in multi-line editor mode.
// Returns (model, cmd, handled) where handled indicates the key was consumed.
func (m model) handleEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		logging.Info("Exiting editor mode (cancel)", "insertNewline", "true=>false", "prompt", promptEditor+"=>"+promptDefault)
		m.mode = modeChat
		m.ta.KeyMap.InsertNewline.SetEnabled(false)
		m.ta.Prompt = m.origPrompt
		// Discard the multi-line draft and restore single-line height
		m.ta.Reset()
		m.ta.SetHeight(3)
		m.append("Canceled edit.")
		// Ensure focus remains on textarea, then recalc layout immediately
		var focusCmd tea.Cmd
		if !m.ta.Focused() {
			m.ta, focusCmd = m.ta.Update(tea.FocusMsg{})
		}
		if focusCmd != nil {
			return m, tea.Batch(focusCmd, tea.WindowSize()), true
		}
		return m, tea.WindowSize(), true
	case tea.KeyEnter:
		// Let textarea handle newline insertion; not handled here
	}
	// Detect common submit chords via string (Windows terminals often map ctrl+enter to ctrl+m)
	// Also accept F5 and Ctrl+R as reliable run keys across terminals.
	if s := msg.String(); s == "ctrl+enter" || s == "ctrl+m" || s == "alt+enter" || s == "f5" || s == "ctrl+r" {
		return m, func() tea.Msg { return submitEditorMsg{} }, true
	}
	// Forward to textarea for normal editing behavior
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd, true
}

func (m model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	// Desired content width: cap at maxContentWidth; center horizontally via container padding
	contentWidth := m.width
	if m.maxContentWidth > 0 && contentWidth > m.maxContentWidth {
		contentWidth = m.maxContentWidth
	}
	// Borders take 2 cols; textarea/view should subtract borders from inner widths
	innerWidth := contentWidth - 2
	if innerWidth < 10 {
		innerWidth = 10
	}
	m.ta.SetWidth(innerWidth)
	// Compute textarea height depending on mode
	taHeight := m.ta.Height()
	if m.mode == modeKQLEditor {
		// Aim for desired height but cap to available space (leave border + spacer + min vp)
		maxTA := m.height - 2 - 1 - minViewportHeight // borders + spacer + min vp
		if maxTA < minEditorHeight {
			maxTA = minEditorHeight
		}
		h := m.editorDesiredHeight
		if h > maxTA {
			h = maxTA
		}
		if h < minEditorHeight {
			h = minEditorHeight
		}
		m.ta.SetHeight(h)
		taHeight = h
	}
	// Leave one line spacer between viewport and textarea
	vpHeight := m.height - taHeight - 1 - 2 // -2 for viewport border
	if vpHeight < minViewportHeight {
		vpHeight = minViewportHeight
	}
	m.vp.Width = innerWidth
	m.vp.Height = vpHeight
	// size list to same as viewport when active
	m.list.SetSize(innerWidth, vpHeight)
	// size table similarly
	m.tbl.SetWidth(innerWidth)
	m.tbl.SetHeight(vpHeight)
	if m.followTail || m.vp.AtBottom() {
		m.vp.GotoBottom()
		m.followTail = true
	}

	// Center container horizontally
	sidePad := (m.width - (innerWidth + 2)) / 2 // +2 borders
	if sidePad < 0 {
		sidePad = 0
	}
	m.containerStyle = lipgloss.NewStyle().PaddingLeft(sidePad).PaddingRight(sidePad)
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		// Only submit when we're in single-line mode (InsertNewline disabled)
		if m.ta.KeyMap.InsertNewline.Enabled() {
			return m, nil
		}
		input := strings.TrimSpace(m.ta.Value())
		m.ta.Reset()
		if input == "" {
			// If we have results, open interactive table on empty Enter
			if m.haveResults {
				return m.openTableFromLastResults()
			}
			return m, nil
		}
		m.append("> " + input)
		if m.authState != auth.AuthStateCompleted {
			return m.handlePreAuthCommand(input)
		}
		return m.handlePostAuthCommand(input)
	}
	return m, nil
}

func (m model) handlePreAuthCommand(input string) (tea.Model, tea.Cmd) {
	if input == "login" {
		if m.authState != auth.AuthStateInProgress {
			m.authState = auth.AuthStateInProgress
			return m, m.startAuthCmd()
		}
		m.append("Login already in progress…")
		return m, nil
	}
	m.append("Authentication required. Type 'login' to start device flow.")
	return m, nil
}

func (m model) handlePostAuthCommand(input string) (tea.Model, tea.Cmd) {
	logging.Debug("handlePostAuthCommand", "input_prefix", func() string {
		if len(input) > 16 {
			return input[:16]
		}
		return input
	}())
	switch input {
	case "help", "?":
		m.append("Commands: help, keys, subs, resources, config, config get <key>, config set <key>=<value>, kql: <query>, login, quit")
		m.append("Tip: type 'keys' for keybindings.")
	case "quit", "exit":
		m.quitting = true
		return m, tea.Quit
	case "edit":
		// Enter multi-line editor mode
		logging.Info("Entering editor mode", "insertNewline", "false=>true", "prompt", m.ta.Prompt+"=>"+promptEditor)
		m.mode = modeKQLEditor
		m.ta.KeyMap.InsertNewline.SetEnabled(true)
		m.origPrompt = m.ta.Prompt
		m.ta.Prompt = promptEditor
		// Resize textarea height on next WindowSize or compute now using current height
		// Ensure focus remains on textarea
		var focusCmd tea.Cmd
		if !m.ta.Focused() {
			m.ta, focusCmd = m.ta.Update(tea.FocusMsg{})
		}
		// Show helper hint and keybindings
		m.append(hintEditor)
		m.showKeys()
		// Trigger a resize recompute to adjust heights
		if focusCmd != nil {
			return m, tea.Batch(focusCmd, tea.WindowSize())
		}
		return m, tea.WindowSize()
	case "subs":
		logging.Debug("Entering list subscriptions mode")
		// Open subscriptions list panel and load items
		m.mode = modeListSubscriptions
		m.list.SetItems(nil)
		m.list.Title = titleSelectSubscription
		m.append("Loading subscriptions…")
		return m, m.loadSubscriptionsCmd()
	case "resources":
		logging.Debug("Entering list insights resources mode")
		// Open Application Insights resources list panel and load items
		m.mode = modeListInsightsResources
		m.list.SetItems(nil)
		m.list.Title = titleSelectInsights
		m.append("Loading Application Insights resources…")
		return m, m.loadInsightsResourcesCmd()
	case "config":
		// Show current configuration
		m.showConfig()
		return m, nil
	case "keys":
		m.showKeys()
		return m, nil
	default:
		// Step 5: single-line KQL prefix
		lower := strings.ToLower(input)
		if strings.HasPrefix(lower, "kql:") {
			logging.Debug("Detected KQL command")
			q := strings.TrimSpace(input[len("kql:"):])
			if q == "" {
				m.append("Query cannot be empty.")
				return m, nil
			}
			// Start running
			m.append("Running query…")
			m.runningKQL = true
			return m, m.runKQLCmd(q)
		}
		// Handle extended config commands
		if strings.HasPrefix(input, "config ") {
			sub := strings.TrimSpace(strings.TrimPrefix(input, "config "))
			return m.handleConfigSubcommand(sub)
		}
		m.append("Unknown command. Type 'help'.")
	}
	return m, nil
}

// internal message and handler for editor submission
type submitEditorMsg struct{}

func (m model) handleEditorSubmit() (tea.Model, tea.Cmd) {
	// Normalize line endings and trim
	raw := m.ta.Value()
	// Don't mutate textarea content in place for now; we will reset after submission/cancel
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		m.append("Query cannot be empty.")
		return m, nil
	}
	// Log safe details and exit editor mode before running
	logging.Info("Submitting editor query")
	// Echo first line + ellipsis
	firstLine := trimmed
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx] + " …"
	}
	m.append("> " + firstLine)
	m.append("Running…")
	// Stay in editor mode while the query runs; keep multi-line editing active
	// Dispatch KQL pipeline
	return m, m.runKQLCmd(trimmed)
}

// handleConfigSubcommand parses and executes `config get` and `config set` operations
func (m model) handleConfigSubcommand(sub string) (tea.Model, tea.Cmd) {
	// Patterns:
	// - get <key>
	// - set <key>=<value>
	lower := strings.ToLower(sub)
	if strings.HasPrefix(lower, "get ") {
		key := strings.TrimSpace(sub[4:])
		if key == "" {
			m.append("Usage: config get <key>")
			return m, nil
		}
		val, err := m.cfg.GetSettingValue(key)
		if err != nil {
			m.append("Unknown setting: " + key)
			m.append("Tip: type 'config' to see available settings.")
			return m, nil
		}
		m.append(key + " = " + val)
		return m, nil
	}

	if strings.HasPrefix(lower, "set ") {
		rest := strings.TrimSpace(sub[4:])
		if rest == "" {
			m.append("Usage: config set <key>=<value>")
			return m, nil
		}
		// Split on first '=' only to allow spaces in value
		parts := strings.SplitN(rest, "=", 2)
		if len(parts) != 2 {
			m.append("Usage: config set <key>=<value>")
			return m, nil
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Capture old value (masked where applicable)
		oldVal, _ := m.cfg.GetSettingValue(key)

		if err := m.cfg.ValidateAndUpdateSetting(key, value); err != nil {
			// Check if this is a persistence error vs validation error
			if strings.Contains(err.Error(), "setting updated in memory but failed to save to file") {
				// Get the updated value since it was changed in memory
				newVal, _ := m.cfg.GetSettingValue(key)
				logging.Error("Failed to persist config", "key", key, "error", err.Error())
				m.append("Updated " + key + " (old: " + oldVal + ", new: " + newVal + "), but failed to save: " + err.Error() + ". Check file permissions and disk space.")
			} else {
				// Validation error
				m.append("Failed to update setting: " + err.Error())
				m.append("Tip: type 'config' to see available settings or 'config get " + key + "'.")
			}
			return m, nil
		}

		// Get masked new value for display/logging
		newVal, _ := m.cfg.GetSettingValue(key)
		m.append("Updated " + key + " (old: " + oldVal + ", new: " + newVal + ").")
		logging.Info("Config updated", "key", key, "old", oldVal, "new", newVal)
		return m, nil
	}

	m.append("Unknown config command. Usage: config | config get <key> | config set <key>=<value>")
	return m, nil
}

// handleListKey processes keys while in list modes (subscriptions or insights resources)
func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		switch m.mode {
		case modeListSubscriptions:
			m.append("Closed subscriptions panel.")
		case modeListInsightsResources:
			m.append("Closed Application Insights resources panel.")
		}
		m.mode = modeChat
		return m, nil
	case "enter":
		switch m.mode {
		case modeListSubscriptions:
			if sel, ok := m.list.SelectedItem().(subscriptionItem); ok {
				logging.Info("Subscription selected", "id", sel.s.ID, "name", sel.s.DisplayName)
				if err := m.cfg.ValidateAndUpdateSetting(keyAzureSubscriptionID, sel.s.ID); err != nil {
					m.append("Failed to set subscription: " + err.Error())
				} else {
					m.append("Subscription selected: " + sel.s.DisplayName + " (" + sel.s.ID + ")")
				}
				m.mode = modeChat
			}
		case modeListInsightsResources:
			if sel, ok := m.list.SelectedItem().(insightsResourceItem); ok {
				logging.Info("Insights resource selected", "name", sel.r.Name, "appId", sel.r.ApplicationID)
				if err := m.cfg.ValidateAndUpdateSetting("applicationInsightsAppId", sel.r.ApplicationID); err != nil {
					m.append("Failed to set Application Insights resource: " + err.Error())
				} else {
					m.append("Application Insights resource selected: " + sel.r.Name + " (App ID: " + sel.r.ApplicationID + ")")
				}
				m.mode = modeChat
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) handleDeviceCode(msg deviceCodeMsg) (tea.Model, tea.Cmd) {
	// store context and device response
	m.deviceResp = msg.resp
	m.authCtx = msg.ctx
	m.cancelAuth = msg.cancel
	m.authState = auth.AuthStateInProgress
	// Show instructions
	if m.deviceResp.VerificationURI != "" && m.deviceResp.UserCode != "" {
		m.append("Open " + m.deviceResp.VerificationURI + " and enter code " + m.deviceResp.UserCode + ".")
	}
	if m.deviceResp.VerificationURIComplete != "" {
		m.append("Or open: " + m.deviceResp.VerificationURIComplete)
	}
	m.append("Waiting for verification…")
	return m, m.pollForTokenCmd()
}

func (m model) handleAuthSuccess() (tea.Model, tea.Cmd) {
	if m.cancelAuth != nil {
		m.cancelAuth()
		m.cancelAuth = nil
	}
	m.authState = auth.AuthStateCompleted
	m.append("Authentication successful. Token received and saved.")
	return m, nil
}

func (m model) handleAuthError(msg authErrorMsg) (tea.Model, tea.Cmd) {
	if m.cancelAuth != nil {
		m.cancelAuth()
		m.cancelAuth = nil
	}
	m.authState = auth.AuthStateFailed
	m.append("Authentication failed: " + msg.err.Error())
	m.append("Tips: verify Tenant and Client ID, network access, and permissions in Azure Portal: https://portal.azure.com")
	m.append("Type 'login' to try again or 'help'.")
	return m, nil
}

// handleKQLResult processes the outcome of a KQL execution
func (m model) handleKQLResult(res kqlResultMsg) (tea.Model, tea.Cmd) {
	m.runningKQL = false
	if res.err != nil {
		logging.Error("KQL result error", "error", res.err.Error())
		m.append(res.err.Error())
		m.haveResults = false
		return m, nil
	}
	// Zero tables case
	if len(res.columns) == 0 && len(res.rows) == 0 {
		m.append(fmt.Sprintf("Query complete in %.3fs · 0 rows", res.duration.Seconds()))
		m.append("No results.")
		m.haveResults = false
		return m, nil
	}
	// Build snapshot
	summary := fmt.Sprintf("Query complete in %.3fs · %d rows · table: %s", res.duration.Seconds(), len(res.rows), firstNonEmpty(res.tableName, "PrimaryResult"))
	m.append(summary)
	snapshot := m.renderSnapshot(res.columns, res.rows)
	if snapshot != "" {
		m.append(snapshot)
	}
	if m.mode == modeKQLEditor {
		m.append("Press Esc to exit editor, then Enter to open interactively.")
	} else {
		m.append("Press Enter to open interactively.")
	}
	// Store for interactive
	m.lastColumns = res.columns
	m.lastRows = res.rows
	m.lastTable = res.tableName
	m.lastDuration = res.duration
	m.haveResults = true
	// Scroll to bottom for visibility
	if m.followTail || m.vp.AtBottom() {
		m.vp.GotoBottom()
	}
	return m, nil
}

// renderSnapshot builds a Bubbles table string with dynamic columns, limited rows
func (m *model) renderSnapshot(columns []appinsights.Column, rows [][]interface{}) string {
	// Limit rows to fetch size
	maxRows := m.cfg.LogFetchSize
	if maxRows <= 0 {
		maxRows = 50
	}
	if len(rows) < maxRows {
		maxRows = len(rows)
	}
	// Build columns definitions with equal widths
	colCount := len(columns)
	if colCount == 0 {
		return ""
	}
	width := m.vp.Width
	if width <= 0 {
		width = 80
	}
	// Reserve minimal padding for borders already handled by container; just split equally
	per := width / colCount
	if per < 5 {
		per = 5
	}
	cols := make([]table.Column, 0, colCount)
	for _, c := range columns {
		cols = append(cols, table.Column{Title: c.Name, Width: per})
	}
	// Build rows as []table.Row (slice of string)
	trows := make([]table.Row, 0, maxRows)
	for i := 0; i < maxRows; i++ {
		r := rows[i]
		tr := make([]string, 0, colCount)
		for j := 0; j < colCount && j < len(r); j++ {
			v := r[j]
			if v == nil {
				tr = append(tr, "")
				continue
			}
			// Format primitives via fmt.Sprint
			tr = append(tr, sprintAny(v))
		}
		// ensure length matches columns
		for len(tr) < colCount {
			tr = append(tr, "")
		}
		trows = append(trows, tr)
	}

	tm := table.New(
		table.WithColumns(cols),
		table.WithRows(trows),
		table.WithWidth(width),
		table.WithHeight(min(10, m.vp.Height)),
	)
	// Not focused in snapshot mode
	tm.Blur()
	return tm.View()
}

// initInteractiveTable builds a focused table model with all rows and columns
func (m *model) initInteractiveTable() {
	columns := m.lastColumns
	rows := m.lastRows
	colCount := len(columns)
	if colCount == 0 {
		m.tbl = table.New()
		return
	}
	width := m.vp.Width
	if width <= 0 {
		width = 80
	}
	per := width / colCount
	if per < 5 {
		per = 5
	}
	cols := make([]table.Column, 0, colCount)
	for _, c := range columns {
		cols = append(cols, table.Column{Title: c.Name, Width: per})
	}
	trows := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		tr := make([]string, 0, colCount)
		for j := 0; j < colCount && j < len(r); j++ {
			v := r[j]
			if v == nil {
				tr = append(tr, "")
			} else {
				tr = append(tr, sprintAny(v))
			}
		}
		for len(tr) < colCount {
			tr = append(tr, "")
		}
		trows = append(trows, tr)
	}
	m.tbl = table.New(
		table.WithColumns(cols),
		table.WithRows(trows),
		table.WithWidth(width),
		table.WithHeight(m.vp.Height),
		table.WithFocused(true),
	)
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func sprintAny(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		// avoid scientific notation for whole numbers if common in KQL
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		if t == float32(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", t)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
