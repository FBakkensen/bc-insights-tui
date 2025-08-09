package auth

import (
	"encoding/json"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
	keyring "github.com/zalando/go-keyring"
)

// Verifies that when only the backup entry contains legacy JSON, getStoredToken()
// restores the primary entry with the raw refresh token (not JSON) and returns it.
func TestBackupLegacyJSONRestoresRawRefreshToken(t *testing.T) {
	cfg := config.OAuth2Config{TenantID: "t", ClientID: "c", Scopes: []string{"s"}}
	a := NewAuthenticator(cfg)

	// Ensure clean state
	_ = a.ClearToken()

	// Prepare legacy JSON in backup only
	legacy := map[string]interface{}{"refresh_token": "r123"}
	raw, _ := json.Marshal(legacy)
	svc, _ := KeyringEntryInfo()
	bsvc, _ := KeyringBackupEntryInfo()
	_ = keyring.Delete(svc, tokenKey)
	if err := keyring.Set(bsvc, backupTokenKey, string(raw)); err != nil {
		t.Fatalf("failed to set backup: %v", err)
	}

	// Trigger retrieval (should fallback to backup and self-heal primary)
	tok, err := a.getStoredToken()
	if err != nil {
		t.Fatalf("getStoredToken failed: %v", err)
	}
	if tok.RefreshToken != "r123" {
		t.Fatalf("expected refresh token 'r123', got %q", tok.RefreshToken)
	}

	// Primary should now contain raw refresh token, not JSON
	v, gerr := keyring.Get(svc, tokenKey)
	if gerr != nil {
		t.Fatalf("primary get after restore failed: %v", gerr)
	}
	if v != "r123" {
		t.Fatalf("expected primary to store raw refresh token 'r123', got %q", v)
	}

	// Cleanup
	_ = a.ClearToken()
}
