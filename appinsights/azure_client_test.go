package appinsights

import (
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
)

func TestNewAzureClientWithAuthenticator(t *testing.T) {
	cfg := config.OAuth2Config{
		TenantID: "test-tenant-id",
		ClientID: "test-client-id",
		Scopes:   []string{"https://management.azure.com/.default"},
	}
	authenticator := auth.NewAuthenticator(cfg)

	client, err := NewAzureClientWithAuthenticator(authenticator)
	if err != nil {
		t.Fatalf("Expected no error creating Azure client, got %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil Azure client")
	}

	// Check that the client has a credential
	if client.credential == nil {
		t.Error("Expected credential to be set")
	}
}

func TestNewAzureClientWithAuthenticator_NilAuth(t *testing.T) {
	_, err := NewAzureClientWithAuthenticator(nil)
	if err == nil {
		t.Error("Expected error when creating Azure client with nil authenticator")
	}
	if err.Error() != "authenticator is nil" {
		t.Errorf("Expected 'authenticator is nil', got %q", err.Error())
	}
}

func TestAzureSubscription_Structure(t *testing.T) {
	sub := AzureSubscription{
		ID:          "test-subscription-id",
		DisplayName: "Test Subscription",
		State:       "Enabled",
	}

	if sub.ID != "test-subscription-id" {
		t.Errorf("Expected ID 'test-subscription-id', got %q", sub.ID)
	}
	if sub.DisplayName != "Test Subscription" {
		t.Errorf("Expected DisplayName 'Test Subscription', got %q", sub.DisplayName)
	}
	if sub.State != "Enabled" {
		t.Errorf("Expected State 'Enabled', got %q", sub.State)
	}
}

func TestAzureSubscription_FormatSubscriptionForDisplay(t *testing.T) {
	sub := AzureSubscription{
		ID:          "bf273484-b813-4c92-8527-8fa577aec089",
		DisplayName: "Production Subscription",
		State:       "Enabled",
	}

	expected := "Production Subscription (bf273484-b813-4c92-8527-8fa577aec089) - Enabled"
	result := sub.FormatSubscriptionForDisplay()

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestAzureSubscription_DisplayText(t *testing.T) {
	sub := AzureSubscription{
		ID:          "test-id",
		DisplayName: "Test Display Name",
		State:       "Active",
	}

	result := sub.DisplayText()
	expected := sub.FormatSubscriptionForDisplay()

	if result != expected {
		t.Errorf("Expected DisplayText to match FormatSubscriptionForDisplay: %q vs %q", result, expected)
	}
}

func TestAzureSubscription_UniqueID(t *testing.T) {
	sub := AzureSubscription{
		ID:          "unique-subscription-id",
		DisplayName: "Test Subscription",
		State:       "Enabled",
	}

	result := sub.UniqueID()
	if result != sub.ID {
		t.Errorf("Expected UniqueID to return subscription ID %q, got %q", sub.ID, result)
	}
}

// Integration test that requires real Azure credentials - should be skipped in CI
func TestListSubscriptions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires real Azure credentials and should only run in integration test mode
	cfg := config.OAuth2Config{
		TenantID: "test-tenant", // Would need real values for integration test
	}

	// Skip test if no real config is available
	if cfg.TenantID == "test-tenant" {
		t.Skip("Skipping integration test - no real Azure config provided")
	}

	// This would require a real authenticator with valid tokens
	t.Log("Integration test for ListSubscriptions would run here with real credentials")
}

func TestSubscriptionDataValidation(t *testing.T) {
	tests := []struct {
		name        string
		sub         AzureSubscription
		expectValid bool
	}{
		{
			name: "valid subscription",
			sub: AzureSubscription{
				ID:          "bf273484-b813-4c92-8527-8fa577aec089",
				DisplayName: "Test Subscription",
				State:       "Enabled",
			},
			expectValid: true,
		},
		{
			name: "empty ID",
			sub: AzureSubscription{
				ID:          "",
				DisplayName: "Test Subscription",
				State:       "Enabled",
			},
			expectValid: false,
		},
		{
			name: "empty display name",
			sub: AzureSubscription{
				ID:          "bf273484-b813-4c92-8527-8fa577aec089",
				DisplayName: "",
				State:       "Enabled",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasID := tt.sub.ID != ""
			hasName := tt.sub.DisplayName != ""
			isValid := hasID && hasName

			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v for %+v", tt.expectValid, isValid, tt.sub)
			}
		})
	}
}

// Test the authenticatorCredential internal implementation (limited testing without real Azure calls)
func TestNewAzureClient(t *testing.T) {
	validToken := &oauth2.Token{
		AccessToken: "test-token",
		Expiry:      time.Now().Add(time.Hour),
	}

	tests := []struct {
		name        string
		token       *oauth2.Token
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid token",
			token:       validToken,
			expectError: false,
		},
		{
			name:        "nil token",
			token:       nil,
			expectError: true,
			errorMsg:    "no authentication token provided",
		},
		{
			name: "expired token",
			token: &oauth2.Token{
				AccessToken: "expired-token",
				Expiry:      time.Now().Add(-time.Hour),
			},
			expectError: true,
			errorMsg:    "provided authentication token is not valid or expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAzureClient(tt.token)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error %q, got %q", tt.errorMsg, err.Error())
				}
				if client != nil {
					t.Errorf("Expected nil client on error, got %v", client)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				if client == nil {
					t.Error("Expected non-nil client")
					return // Prevent nil pointer dereference
				}
				if client.credential == nil {
					t.Error("Expected credential to be set")
				}
			}
		})
	}
}
