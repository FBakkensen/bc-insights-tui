package auth

import (
	"context"
	"testing"
	"time"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func TestNewAuthenticator(t *testing.T) {
	cfg := config.OAuth2Config{
		TenantID: "test-tenant",
		ClientID: "test-client",
		Scopes:   []string{"test-scope"},
	}

	auth := NewAuthenticator(cfg)

	if auth == nil {
		t.Fatal("Expected authenticator to be created, got nil")
	}

	if auth.config.TenantID != cfg.TenantID {
		t.Errorf("Expected TenantID %s, got %s", cfg.TenantID, auth.config.TenantID)
	}

	if auth.config.ClientID != cfg.ClientID {
		t.Errorf("Expected ClientID %s, got %s", cfg.ClientID, auth.config.ClientID)
	}

	if len(auth.config.Scopes) != len(cfg.Scopes) {
		t.Errorf("Expected %d scopes, got %d", len(cfg.Scopes), len(auth.config.Scopes))
	}
}

func TestHasValidToken_NoToken(t *testing.T) {
	cfg := config.OAuth2Config{
		TenantID: "test-tenant",
		ClientID: "test-client",
		Scopes:   []string{"test-scope"},
	}

	auth := NewAuthenticator(cfg)

	// Should return false when no token is stored
	if auth.HasValidToken() {
		t.Error("Expected HasValidToken to return false when no token is stored")
	}
}

func TestInitiateDeviceFlow_InvalidTenant(t *testing.T) {
	cfg := config.OAuth2Config{
		TenantID: "invalid-tenant",
		ClientID: "test-client",
		Scopes:   []string{"test-scope"},
	}

	auth := NewAuthenticator(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should fail due to network/DNS issues in test environment
	_, err := auth.InitiateDeviceFlow(ctx)
	if err == nil {
		t.Error("Expected InitiateDeviceFlow to fail with invalid tenant, but it succeeded")
	}

	// The error should be network-related (expected in test environment)
	if err != nil {
		t.Logf("Expected network error received: %v", err)
	}
}

func TestClearToken_NoToken(t *testing.T) {
	cfg := config.OAuth2Config{
		TenantID: "test-tenant",
		ClientID: "test-client",
		Scopes:   []string{"test-scope"},
	}

	auth := NewAuthenticator(cfg)

	// Should not panic when clearing non-existent token
	err := auth.ClearToken()
	// Error is expected when no token exists to clear
	if err == nil {
		t.Log("ClearToken returned no error (token may not have existed)")
	} else {
		t.Logf("ClearToken returned expected error: %v", err)
	}
}

func TestAuthState_Constants(t *testing.T) {
	// Test that auth state constants are defined correctly
	states := []AuthState{
		AuthStateUnknown,
		AuthStateRequired,
		AuthStateInProgress,
		AuthStateCompleted,
		AuthStateFailed,
	}

	if len(states) != 5 {
		t.Errorf("Expected 5 auth states, got %d", len(states))
	}

	// Ensure they have different values
	stateValues := make(map[AuthState]bool)
	for _, state := range states {
		if stateValues[state] {
			t.Errorf("Duplicate auth state value found: %d", state)
		}
		stateValues[state] = true
	}
}
