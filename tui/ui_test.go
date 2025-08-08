package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
)

// newTestModel creates a minimal model suitable for unit tests without hitting external services.
func newTestModel() model {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Prompt = "> "
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(80)
	ta.KeyMap.InsertNewline.SetEnabled(false) // single-line

	vp := viewport.New(80, 10)
	vp.SetContent("")

	list := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	list.Title = titleSelectSubscription
	list.SetShowStatusBar(false)
	list.SetFilteringEnabled(true)
	list.SetShowHelp(false)

	// Create a properly initialized config
	cfg := config.NewConfig()
	cfg.ApplicationInsightsID = "test-app-id"
	cfg.OAuth2.TenantID = "test-tenant"
	cfg.OAuth2.ClientID = "test-client"
	cfg.OAuth2.Scopes = []string{"https://management.azure.com/.default"}

	m := model{
		vp:              vp,
		ta:              ta,
		list:            list,
		cfg:             cfg,
		authenticator:   nil, // do not call real auth in unit tests
		authState:       auth.AuthStateUnknown,
		maxContentWidth: 120,
		followTail:      true,
	}
	return m
}

func TestUI_PreAuth_LoginFlow_SyntheticMsgs(t *testing.T) {
	m := newTestModel()

	// User types "login" and presses Enter (pre-auth state)
	m.ta.SetValue("login")
	m2Any, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	if m2.authState != auth.AuthStateInProgress {
		t.Fatalf("expected auth state InProgress after 'login', got %v", m2.authState)
	}
	if cmd == nil {
		t.Fatalf("expected a command to start device flow, got nil")
	}
	if !strings.Contains(m2.content, "> login") {
		t.Fatalf("expected input echo appended to content; got: %q", m2.content)
	}

	// Simulate device code message (no network)
	dmsg := deviceCodeMsg{resp: &auth.DeviceCodeResponse{
		DeviceCode:              "dev-code",
		UserCode:                "USER-CODE",
		VerificationURI:         "https://example.test/verify",
		VerificationURIComplete: "https://example.test/verify?user_code=USER-CODE",
		Interval:                1,
	}}
	m3Any, _ := m2.Update(dmsg)
	m3 := m3Any.(model)

	// Verify instructions appended
	if !strings.Contains(m3.content, "Open https://example.test/verify and enter code USER-CODE.") {
		t.Fatalf("expected verification instructions in content; got: %q", m3.content)
	}
	if !strings.Contains(m3.content, "Waiting for verification…") {
		t.Fatalf("expected waiting message in content; got: %q", m3.content)
	}

	// Simulate successful auth completion
	m4Any, _ := m3.Update(authSuccessMsg{})
	m4 := m4Any.(model)
	if m4.authState != auth.AuthStateCompleted {
		t.Fatalf("expected auth state Completed, got %v", m4.authState)
	}
	if !strings.Contains(m4.content, "Authentication successful.") {
		t.Fatalf("expected success message appended; got: %q", m4.content)
	}
}

func TestUI_PostAuth_HelpAndQuit(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// help command
	m.ta.SetValue("help")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)
	if !strings.Contains(m2.content, "Commands: help, subs, resources, config, login, quit") {
		t.Fatalf("expected help text in content; got: %q", m2.content)
	}

	// quit command
	m2.ta.SetValue("quit")
	m3Any, cmd := m2.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := m3Any.(model)
	if !m3.quitting {
		t.Fatalf("expected quitting flag true after quit command")
	}
	if cmd == nil {
		t.Fatalf("expected a quit command, got nil")
	}
	// Execute cmd to verify it returns a QuitMsg
	if msg := cmd(); msg == nil {
		t.Fatalf("expected tea.Quit command to return a message")
	} else {
		if _, ok := (msg).(tea.QuitMsg); !ok {
			t.Fatalf("expected tea.QuitMsg, got %T", msg)
		}
	}
}

func TestUI_Resize_LayoutDimensions(t *testing.T) {
	m := newTestModel()
	// Send a WindowSizeMsg and verify widths/heights computed
	width, height := 120, 40
	m2, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	got := m2.(model)

	// inner width subtracts 2 for border when capped by maxContentWidth
	expectedContentWidth := width
	if expectedContentWidth > got.maxContentWidth {
		expectedContentWidth = got.maxContentWidth
	}
	expectedInner := expectedContentWidth - 2
	if expectedInner < 10 {
		expectedInner = 10
	}
	if got.ta.Width() <= 0 || got.ta.Width() > expectedInner {
		t.Fatalf("textarea width = %d, want in (0, %d]", got.ta.Width(), expectedInner)
	}
	if got.vp.Width != expectedInner {
		t.Fatalf("viewport width = %d, want %d", got.vp.Width, expectedInner)
	}
	// vpHeight = height - ta.Height() - 1 (spacer) - 2 (border)
	expectedVPHeight := height - got.ta.Height() - 1 - 2
	if expectedVPHeight < 3 {
		expectedVPHeight = 3
	}
	if got.vp.Height != expectedVPHeight {
		t.Fatalf("viewport height = %d, want %d", got.vp.Height, expectedVPHeight)
	}
}

func TestUI_View_QuittingIsEmpty(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	if v := m.View(); v != "" {
		t.Fatalf("expected empty view when quitting, got: %q", v)
	}
}

func TestUI_PostAuth_SubsCommand(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// Type "subs" command and press Enter
	m.ta.SetValue("subs")
	m2Any, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Should have switched to list mode
	if m2.mode != modeListSubscriptions {
		t.Fatalf("expected mode to switch to modeListSubscriptions after subs command, got %v", m2.mode)
	}

	// Should echo the command in content
	if !strings.Contains(m2.content, "> subs") {
		t.Fatalf("expected subs command echoed in content; got: %q", m2.content)
	}

	// Should have a command to load subscriptions
	if cmd == nil {
		t.Fatalf("expected a command to load subscriptions, got nil")
	}

	// Execute the command to verify it returns a message
	msg := cmd()
	if msg == nil {
		t.Fatalf("expected subscription loading command to return a message")
	}

	// Check that it's a subsLoadedMsg (with or without error, depending on authenticator)
	if _, ok := msg.(subsLoadedMsg); !ok {
		t.Fatalf("expected subsLoadedMsg, got %T", msg)
	}
}

func TestUI_SubsLoadedMsg_Success(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Create test subscriptions using the AzureSubscription struct
	subs := []list.Item{
		subscriptionItem{s: appinsights.AzureSubscription{ID: "sub1", DisplayName: "Test Sub 1", State: "Enabled"}},
		subscriptionItem{s: appinsights.AzureSubscription{ID: "sub2", DisplayName: "Test Sub 2", State: "Enabled"}},
	}

	// Send subsLoadedMsg with subscriptions
	msg := subsLoadedMsg{items: subs, err: nil}
	m2Any, _ := m.Update(msg)
	m2 := m2Any.(model)

	// Should remain in list mode with populated subscriptions
	if m2.mode != modeListSubscriptions {
		t.Fatalf("expected mode to remain modeListSubscriptions after loading, got %v", m2.mode)
	}

	// List should be populated with subscriptions
	if m2.list.Index() != 0 {
		t.Fatalf("expected list index to be 0 after loading subscriptions, got %d", m2.list.Index())
	}

	// Check that list has the right number of items
	if len(m2.list.Items()) != 2 {
		t.Fatalf("expected 2 items in list, got %d", len(m2.list.Items()))
	}

	// Check that the items are the expected subscriptions
	items := m2.list.Items()
	if items[0].(subscriptionItem).s.ID != "sub1" {
		t.Fatalf("expected first item ID to be 'sub1', got %q", items[0].(subscriptionItem).s.ID)
	}
	if items[1].(subscriptionItem).s.ID != "sub2" {
		t.Fatalf("expected second item ID to be 'sub2', got %q", items[1].(subscriptionItem).s.ID)
	}
}

func TestUI_SubsLoadedMsg_Error(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Send subsLoadedMsg with error
	testErr := fmt.Errorf("failed to load subscriptions")
	msg := subsLoadedMsg{items: nil, err: testErr}
	m2Any, _ := m.Update(msg)
	m2 := m2Any.(model)

	// Should append error message to content (exact message from implementation)
	if !strings.Contains(m2.content, "Failed to load subscriptions: failed to load subscriptions") {
		t.Fatalf("expected error message in content; got: %q", m2.content)
	}

	// Should switch back to chat mode on error
	if m2.mode != modeChat {
		t.Fatalf("expected mode to switch to modeChat after error, got %v", m2.mode)
	}

	// List should remain empty
	if len(m2.list.Items()) != 0 {
		t.Fatalf("expected empty list after error, got %d items", len(m2.list.Items()))
	}
}

func TestUI_SubsLoadedMsg_NoSubscriptions(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Send subsLoadedMsg with empty list (no error)
	msg := subsLoadedMsg{items: []list.Item{}, err: nil}
	m2Any, _ := m.Update(msg)
	m2 := m2Any.(model)

	// Should append no subscriptions message to content
	if !strings.Contains(m2.content, "No subscriptions found for this account.") {
		t.Fatalf("expected no subscriptions message in content; got: %q", m2.content)
	}

	// Should switch back to chat mode when no subscriptions
	if m2.mode != modeChat {
		t.Fatalf("expected mode to switch to modeChat when no subscriptions, got %v", m2.mode)
	}

	// List should remain empty
	if len(m2.list.Items()) != 0 {
		t.Fatalf("expected empty list when no subscriptions, got %d items", len(m2.list.Items()))
	}
}

func TestUI_ListMode_EscapeToChat(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Press Escape key
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := m2Any.(model)

	// Should switch back to chat mode
	if m2.mode != modeChat {
		t.Fatalf("expected mode to switch to modeChat after Escape, got %v", m2.mode)
	}

	// Should append return message to content (exact message from implementation)
	if !strings.Contains(m2.content, "Closed subscriptions panel.") {
		t.Fatalf("expected return message in content; got: %q", m2.content)
	}
}

func TestUI_ListMode_SelectSubscription(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Set up list with test subscriptions
	subs := []list.Item{
		subscriptionItem{s: appinsights.AzureSubscription{ID: "sub1", DisplayName: "Test Sub 1", State: "Enabled"}},
		subscriptionItem{s: appinsights.AzureSubscription{ID: "sub2", DisplayName: "Test Sub 2", State: "Disabled"}},
	}
	m.list.SetItems(subs)

	// Press Enter to select first subscription
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Should switch back to chat mode
	if m2.mode != modeChat {
		t.Fatalf("expected mode to switch to modeChat after selection, got %v", m2.mode)
	}

	// Should append selection message to content (exact message from implementation)
	if !strings.Contains(m2.content, "Subscription selected: Test Sub 1 (sub1)") {
		t.Fatalf("expected selection message in content; got: %q", m2.content)
	}
}

func TestUI_ListMode_NoSelection(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted
	m.mode = modeListSubscriptions

	// Don't add any items to list (empty list)

	// Press Enter when no items are in the list
	m2Any, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Should remain in list mode
	if m2.mode != modeListSubscriptions {
		t.Fatalf("expected mode to remain modeListSubscriptions when no selection, got %v", m2.mode)
	}

	// Should not append any selection message
	contentBefore := m.content
	if m2.content != contentBefore {
		t.Fatalf("expected content to remain unchanged when no selection")
	}
}

func TestSubscriptionItem_Title(t *testing.T) {
	sub := subscriptionItem{
		s: appinsights.AzureSubscription{
			ID:          "bf273484-b813-4c92-8527-8fa577aec089",
			DisplayName: "Production Subscription",
			State:       "Enabled",
		},
	}

	result := sub.Title()
	expected := "Production Subscription"

	if result != expected {
		t.Errorf("Expected Title() to return %q, got %q", expected, result)
	}
}

func TestSubscriptionItem_Description(t *testing.T) {
	sub := subscriptionItem{
		s: appinsights.AzureSubscription{
			ID:          "bf273484-b813-4c92-8527-8fa577aec089",
			DisplayName: "Production Subscription",
			State:       "Enabled",
		},
	}

	result := sub.Description()
	expected := "ID: bf273484-b813-4c92-8527-8fa577aec089 | State: Enabled"

	if result != expected {
		t.Errorf("Expected Description() to return %q, got %q", expected, result)
	}
}

func TestSubscriptionItem_FilterValue(t *testing.T) {
	sub := subscriptionItem{
		s: appinsights.AzureSubscription{
			ID:          "test-id",
			DisplayName: "Test Subscription",
			State:       "Enabled",
		},
	}

	result := sub.FilterValue()
	expected := "Test Subscription"

	if result != expected {
		t.Errorf("Expected FilterValue() to return %q, got %q", expected, result)
	}
}

func TestSubscriptionItem_ListItemInterface(t *testing.T) {
	// Test that subscriptionItem satisfies the list.Item interface
	var _ list.Item = subscriptionItem{}

	sub := subscriptionItem{
		s: appinsights.AzureSubscription{
			ID:          "test-id",
			DisplayName: "Test Subscription",
			State:       "Enabled",
		},
	}

	// Test that we can use it as a list.Item
	items := []list.Item{sub}
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	// Test that we can cast back
	if item, ok := items[0].(subscriptionItem); !ok {
		t.Fatalf("Expected to be able to cast back to subscriptionItem")
	} else if item.s.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %q", item.s.ID)
	}
}

func TestSubscriptionItem_VariousStates(t *testing.T) {
	tests := []struct {
		name         string
		subscription appinsights.AzureSubscription
		wantTitle    string
		wantDesc     string
		wantFilter   string
	}{
		{
			name: "enabled subscription",
			subscription: appinsights.AzureSubscription{
				ID:          "12345678-1234-1234-1234-123456789012",
				DisplayName: "Production Environment",
				State:       "Enabled",
			},
			wantTitle:  "Production Environment",
			wantDesc:   "ID: 12345678-1234-1234-1234-123456789012 | State: Enabled",
			wantFilter: "Production Environment",
		},
		{
			name: "disabled subscription",
			subscription: appinsights.AzureSubscription{
				ID:          "87654321-4321-4321-4321-210987654321",
				DisplayName: "Development Environment",
				State:       "Disabled",
			},
			wantTitle:  "Development Environment",
			wantDesc:   "ID: 87654321-4321-4321-4321-210987654321 | State: Disabled",
			wantFilter: "Development Environment",
		},
		{
			name: "long subscription name",
			subscription: appinsights.AzureSubscription{
				ID:          "short-id",
				DisplayName: "Very Long Subscription Name That Might Cause Layout Issues",
				State:       "Active",
			},
			wantTitle:  "Very Long Subscription Name That Might Cause Layout Issues",
			wantDesc:   "ID: short-id | State: Active",
			wantFilter: "Very Long Subscription Name That Might Cause Layout Issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := subscriptionItem{s: tt.subscription}

			if got := sub.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}

			if got := sub.Description(); got != tt.wantDesc {
				t.Errorf("Description() = %q, want %q", got, tt.wantDesc)
			}

			if got := sub.FilterValue(); got != tt.wantFilter {
				t.Errorf("FilterValue() = %q, want %q", got, tt.wantFilter)
			}
		})
	}
}

func TestUI_ConfigCommand(t *testing.T) {
	m := newTestModel()
	m.authState = auth.AuthStateCompleted

	// config command
	m.ta.SetValue("config")
	m2Any, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := m2Any.(model)

	// Verify config output appears in content
	if !strings.Contains(m2.content, "Current Configuration:") {
		t.Fatalf("expected config output header in content; got: %q", m2.content)
	}
	if !strings.Contains(m2.content, "Basic Settings:") {
		t.Fatalf("expected basic settings section in content; got: %q", m2.content)
	}
	if !strings.Contains(m2.content, "OAuth2:") {
		t.Fatalf("expected OAuth2 section in content; got: %q", m2.content)
	}
}
