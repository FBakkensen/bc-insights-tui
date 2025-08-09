package auth

// OAuth2 Device Authorization Flow logic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// ErrNoStoredToken indicates no stored token was found
var ErrNoStoredToken = errors.New("no stored token found")

const (
	// tokenKey is not a credential but a key name for storing tokens in the system keyring
	tokenKey = "oauth2-token" // #nosec G101
	// backupTokenKey stores a duplicate refresh token for resilience against sporadic key loss
	backupTokenKey = "oauth2-token-backup" // #nosec G101
)

// baseServiceName is the base service name for keyring storage (may be namespaced via env for tests/dev).
const baseServiceName = "bc-insights-tui"

// keyringServiceName resolves the effective keyring service name, allowing isolation in tests/dev.
// Precedence:
// 1) BCINSIGHTS_KEYRING_SERVICE (full override)
// 2) BCINSIGHTS_KEYRING_NAMESPACE (suffix appended to base)
// 3) baseServiceName
func keyringServiceName() string {
	if v := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_SERVICE")); v != "" {
		return v
	}
	if ns := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_NAMESPACE")); ns != "" {
		return baseServiceName + "-" + ns
	}
	return baseServiceName
}

// KeyringEntryInfo returns the effective keyring service and key used to store the refresh token.
// This is exported for diagnostics only.
func KeyringEntryInfo() (service, key string) {
	return keyringServiceName(), tokenKey
}

// KeyringBackupEntryInfo returns the backup keyring service and key used for redundant storage.
// This is exported for diagnostics only.
func KeyringBackupEntryInfo() (service, key string) {
	return keyringServiceName(), backupTokenKey
}

// AuthState represents the current authentication state
type AuthState int

const (
	AuthStateUnknown AuthState = iota
	AuthStateRequired
	AuthStateInProgress
	AuthStateCompleted
	AuthStateFailed
)

// DeviceCodeResponse represents the response from the device authorization endpoint
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Authenticator manages OAuth2 authentication
type Authenticator struct {
	config       config.OAuth2Config
	oauth2Config *oauth2.Config
	httpClient   *http.Client
}

// NewAuthenticator creates a new authenticator instance
func NewAuthenticator(cfg config.OAuth2Config) *Authenticator {
	endpoint := microsoft.AzureADEndpoint(cfg.TenantID)

	oauth2Config := &oauth2.Config{
		ClientID: cfg.ClientID,
		Scopes:   cfg.Scopes,
		Endpoint: endpoint,
	}

	return &Authenticator{
		config:       cfg,
		oauth2Config: oauth2Config,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// HasValidToken checks if there's a valid stored token
func (a *Authenticator) HasValidToken() bool {
	logging.Debug("Checking for valid stored token")
	token, err := a.getStoredToken()
	if err != nil {
		logging.Debug("No stored token found", "error", err.Error())
		return false
	}
	// If we have a non-expired access token, it's valid.
	if token.Valid() {
		logging.Debug("Token validation result", "valid", strconv.FormatBool(true))
		return true
	}
	// Consider presence of a refresh token as sufficient to defer interactive login.
	if strings.TrimSpace(token.RefreshToken) != "" {
		logging.Debug("Token access expired/missing but refresh token is available; interactive login not required")
		return true
	}
	logging.Debug("No valid access token and no refresh token available")
	return false
}

// InitiateDeviceFlow starts the device authorization flow
func (a *Authenticator) InitiateDeviceFlow(ctx context.Context) (*DeviceCodeResponse, error) {
	logging.Info("Initiating device authorization flow")
	deviceEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", a.config.TenantID)
	logging.Debug("Device endpoint", "url", deviceEndpoint)

	data := url.Values{}
	data.Set("client_id", a.config.ClientID)
	// Ensure essential scopes are requested so we always get a refresh token and ID token
	loginScopes := ensureLoginScopes(a.config.Scopes)
	data.Set("scope", strings.Join(loginScopes, " "))
	logging.Info("Requesting device flow scopes", "scopes", strings.Join(loginScopes, " "))

	req, err := http.NewRequestWithContext(ctx, "POST", deviceEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create device code request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed with status %d", resp.StatusCode)
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, fmt.Errorf("failed to decode device code response: %w", err)
	}

	return &deviceResp, nil
}

// PollForToken polls the token endpoint until authentication is complete
func (a *Authenticator) PollForToken(ctx context.Context, deviceCode string, interval int) (*oauth2.Token, error) {
	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.config.TenantID)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := a.pollOnce(ctx, tokenEndpoint, deviceCode)
			if err != nil {
				// Check for specific OAuth2 errors
				if strings.Contains(err.Error(), "authorization_pending") {
					continue // Keep polling
				}
				return nil, err
			}
			return token, nil
		}
	}
}

// pollOnce performs a single token request
func (a *Authenticator) pollOnce(ctx context.Context, tokenEndpoint, deviceCode string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("client_id", a.config.ClientID)
	data.Set("device_code", deviceCode)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if errorDesc, ok := result["error"].(string); ok {
			return nil, fmt.Errorf("token request failed: %s", errorDesc)
		}
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	// Parse token response
	token := &oauth2.Token{}
	if accessToken, ok := result["access_token"].(string); ok {
		token.AccessToken = accessToken
	}
	if refreshToken, ok := result["refresh_token"].(string); ok {
		token.RefreshToken = refreshToken
	}
	if expiresIn, ok := result["expires_in"].(float64); ok {
		token.Expiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}

	// Log diagnostic token claims (without secrets) to help debug audience/scope issues
	if token.AccessToken != "" {
		claims := decodeJWTClaims(token.AccessToken)
		if len(claims) > 0 {
			logging.Debug("Device flow token claims",
				"aud", fmt.Sprint(claims["aud"]),
				"scp", fmt.Sprint(claims["scp"]),
				"roles", fmt.Sprint(claims["roles"]),
				"tid", fmt.Sprint(claims["tid"]),
				"appid", fmt.Sprint(claims["appid"]))
		}
		// Also log expiry in RFC3339 for clarity
		logging.Info("Received device flow token", "expires", token.Expiry.Format(time.RFC3339))
	}

	return token, nil
}

// SaveTokenSecurely stores the token in the OS credential manager
func (a *Authenticator) SaveTokenSecurely(token *oauth2.Token) error {
	logging.Info("Saving authentication token securely")
	// To stay within keyring size limits (notably on Windows), only persist the refresh token.
	// Access tokens are short-lived and will be re-acquired on demand via the refresh token.
	if token == nil || strings.TrimSpace(token.RefreshToken) == "" {
		logging.Warn("No refresh token present to store; ensure offline_access scope is requested during login")
		return fmt.Errorf("no refresh token to store; re-run login with offline_access")
	}

	// Store just the refresh token as the secret value (raw string, smallest footprint).
	if err := keyring.Set(keyringServiceName(), tokenKey, token.RefreshToken); err != nil {
		logging.Error("Failed to store refresh token in keyring", "error", err.Error())
		return fmt.Errorf("failed to store refresh token in keyring: %w", err)
	}

	// Best-effort: write a backup copy to mitigate intermittent credential loss on some systems
	if err := keyring.Set(keyringServiceName(), backupTokenKey, token.RefreshToken); err != nil {
		logging.Warn("Failed to store backup refresh token in keyring", "error", err.Error())
		// Non-fatal: continue, primary was saved
	}

	// Optional read-back verification (primary)
	if v, err := keyring.Get(keyringServiceName(), tokenKey); err == nil {
		if strings.TrimSpace(v) == "" {
			logging.Warn("Primary keyring entry read-back is empty")
		}
	} else {
		logging.Warn("Primary keyring entry read-back failed", "error", err.Error())
	}

	logging.Info("Token saved successfully to secure storage (with backup)")
	return nil
}

// getStoredToken retrieves the stored token from the OS credential manager
func (a *Authenticator) getStoredToken() (*oauth2.Token, error) {
	logging.Debug("Retrieving stored token from keyring")
	tokenJSON, err := keyring.Get(keyringServiceName(), tokenKey)
	if err != nil {
		if err == keyring.ErrNotFound {
			logging.Info("No stored token found in primary keyring entry; attempting backup")
			// Try backup entry
			backupJSON, bErr := keyring.Get(keyringServiceName(), backupTokenKey)
			if bErr != nil {
				if bErr == keyring.ErrNotFound {
					logging.Info("No stored token found in backup keyring entry")
					return nil, ErrNoStoredToken
				}
				logging.Error("Failed to retrieve token from backup keyring", "error", bErr.Error())
				return nil, fmt.Errorf("failed to get token from backup keyring: %w", bErr)
			}
			// Self-heal: restore primary from backup (best-effort)
			if setErr := keyring.Set(keyringServiceName(), tokenKey, backupJSON); setErr != nil {
				logging.Warn("Failed to restore primary keyring entry from backup", "error", setErr.Error())
			} else {
				logging.Info("Restored primary keyring entry from backup")
			}
			tokenJSON = backupJSON
		} else {
			logging.Error("Failed to retrieve token from keyring", "error", err.Error())
			return nil, fmt.Errorf("failed to get token from keyring: %w", err)
		}
	}

	// Backward compatibility: previously we stored the full oauth2.Token JSON.
	// New format stores only the refresh token as a raw string.
	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err == nil {
		logging.Debug("Successfully retrieved legacy stored token (JSON)")
		return &token, nil
	}

	// Treat the stored value as a raw refresh token
	rt := strings.TrimSpace(tokenJSON)
	if rt == "" {
		logging.Warn("Stored token entry is empty after trimming")
		return nil, ErrNoStoredToken
	}
	logging.Debug("Successfully retrieved stored refresh token (raw)")
	return &oauth2.Token{RefreshToken: rt}, nil
}

// StoredRefreshTokenPresent returns true if a refresh token is present in secure storage.
// It avoids logging secrets and is intended for diagnostics.
func (a *Authenticator) StoredRefreshTokenPresent() (bool, error) {
	_, err := a.getStoredToken()
	if err != nil {
		if errors.Is(err, ErrNoStoredToken) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RefreshTokenIfNeeded refreshes the token if it's expired
func (a *Authenticator) RefreshTokenIfNeeded(ctx context.Context) (*oauth2.Token, error) {
	logging.Debug("Checking if token refresh is needed")
	token, err := a.getStoredToken()
	if err != nil {
		logging.Debug("No stored token found, refresh not possible")
		return nil, err
	}

	// If token is still valid, return it
	if token.Valid() {
		logging.Debug("Stored token is still valid, no refresh needed")
		return token, nil
	}

	logging.Debug("Token is expired, attempting refresh")

	// If no refresh token, we need to re-authenticate
	if token.RefreshToken == "" {
		logging.Warn("No refresh token available, re-authentication required")
		return nil, fmt.Errorf("no refresh token available, re-authentication required")
	}

	// Refresh the token
	logging.Debug("Refreshing token using refresh token")
	tokenSource := a.oauth2Config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		logging.Error("Failed to refresh token", "error", err.Error())
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the new token
	logging.Debug("Saving refreshed token securely")
	if err := a.SaveTokenSecurely(newToken); err != nil {
		logging.Error("Failed to save refreshed token", "error", err.Error())
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	logging.Info("Token successfully refreshed and saved")
	return newToken, nil
}

// GetValidToken returns a valid token, refreshing if necessary
func (a *Authenticator) GetValidToken(ctx context.Context) (*oauth2.Token, error) {
	logging.Debug("Getting valid token (with refresh if needed)")
	return a.RefreshTokenIfNeeded(ctx)
}

// GetTokenForScopes exchanges the stored refresh token for an access token for the given scopes.
// It does not overwrite the saved token; callers can cache per-scope tokens in-memory.
func (a *Authenticator) GetTokenForScopes(ctx context.Context, scopes []string) (*oauth2.Token, error) {
	logging.Debug("Getting token for scopes", "scopes", strings.Join(scopes, " "))
	stored, err := a.getStoredToken()
	if err != nil {
		return nil, err
	}
	if stored.RefreshToken == "" {
		logging.Warn("No refresh token available when requesting new scopes; user must sign in again")
		return nil, fmt.Errorf("no refresh token available; please run login to re-authenticate with offline_access")
	}
	// Explicitly exchange refresh token for these scopes using AAD v2 token endpoint.
	tok, err := a.requestTokenWithRefresh(ctx, stored.RefreshToken, scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to get token for scopes: %w", err)
	}
	// If the refresh token was rotated, persist the new one.
	if tok.RefreshToken != "" && tok.RefreshToken != stored.RefreshToken {
		logging.Info("Refresh token rotated by authorization server; updating stored token")
		if err := a.SaveTokenSecurely(tok); err != nil {
			logging.Warn("Failed to persist rotated refresh token; future refresh may fail", "error", err.Error())
		}
	}
	// Log claims for diagnostics (audience, scopes, tenant)
	if tok != nil && tok.AccessToken != "" {
		claims := decodeJWTClaims(tok.AccessToken)
		if len(claims) > 0 {
			logging.Info("Acquired scoped access token",
				"requested_scopes", strings.Join(scopes, " "),
				"aud", fmt.Sprint(claims["aud"]),
				"scp", fmt.Sprint(claims["scp"]),
				"roles", fmt.Sprint(claims["roles"]),
				"tid", fmt.Sprint(claims["tid"]),
				"exp", fmt.Sprint(claims["exp"]))
		}
	}
	return tok, nil
}

// requestTokenWithRefresh exchanges a refresh token for a new access token with the requested scopes
// against the Microsoft identity platform v2 endpoint (requires 'scope' parameter).
func (a *Authenticator) requestTokenWithRefresh(ctx context.Context, refreshToken string, scopes []string) (*oauth2.Token, error) {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.config.TenantID)
	// Normalize scopes: collapse any double slashes before .default
	normalized := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.ReplaceAll(s, "https://management.core.windows.net", "https://management.azure.com")
		s = strings.ReplaceAll(s, "//.default", "/.default")
		normalized = append(normalized, strings.TrimSpace(s))
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.config.ClientID)
	form.Set("refresh_token", refreshToken)
	form.Set("scope", strings.Join(normalized, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, a.handleTokenRefreshError(body, resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	t := &oauth2.Token{}
	if at, ok := result["access_token"].(string); ok {
		t.AccessToken = at
	}
	if rt, ok := result["refresh_token"].(string); ok {
		t.RefreshToken = rt
	} else {
		// If service didn't return a new refresh token, keep using the old one
		t.RefreshToken = refreshToken
	}
	if exp, ok := result["expires_in"].(float64); ok {
		t.Expiry = time.Now().Add(time.Duration(exp) * time.Second)
	}
	// If requesting an ARM token, verify the audience to catch wrong-audience issues early
	if err := a.validateARMTokenAudience(t, normalized); err != nil {
		return nil, err
	}
	return t, nil
}

// ClearToken removes the stored token
func (a *Authenticator) ClearToken() error {
	logging.Info("Clearing stored authentication token")
	var firstErr error
	if err := keyring.Delete(keyringServiceName(), tokenKey); err != nil {
		if err == keyring.ErrNotFound {
			logging.Info("Primary keyring entry not found during clear")
		} else {
			logging.Warn("Failed to delete primary token from keyring", "error", err.Error())
			firstErr = err
		}
	} else {
		logging.Info("Primary keyring entry deleted")
	}
	if err := keyring.Delete(keyringServiceName(), backupTokenKey); err != nil {
		if err == keyring.ErrNotFound {
			logging.Info("Backup keyring entry not found during clear")
		} else {
			logging.Warn("Failed to delete backup token from keyring", "error", err.Error())
			if firstErr == nil {
				firstErr = err
			}
		}
	} else {
		logging.Info("Backup keyring entry deleted")
	}
	if firstErr != nil {
		return fmt.Errorf("failed to clear one or more keyring entries: %w", firstErr)
	}
	logging.Info("Token successfully cleared from secure storage")
	return nil
}

// GetApplicationInsightsToken gets a token specifically for Application Insights API using v1 endpoint
// This is needed because Application Insights API doesn't support v2.0 scopes
func (a *Authenticator) GetApplicationInsightsToken(ctx context.Context) (*oauth2.Token, error) {
	stored, err := a.getStoredToken()
	if err != nil {
		return nil, err
	}
	if stored.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available; please run login to re-authenticate")
	}

	// Use v1 endpoint with resource parameter for Application Insights
	tok, err := a.requestTokenWithResourceV1(ctx, stored.RefreshToken, "https://api.applicationinsights.io")
	if err != nil {
		return nil, fmt.Errorf("failed to get Application Insights token: %w", err)
	}

	return tok, nil
}

// requestTokenWithResourceV1 exchanges refresh token for access token using v1 endpoint with resource parameter
func (a *Authenticator) requestTokenWithResourceV1(ctx context.Context, refreshToken, resource string) (*oauth2.Token, error) {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/token", a.config.TenantID)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.config.ClientID)
	form.Set("refresh_token", refreshToken)
	form.Set("resource", resource) // v1 endpoint uses resource parameter

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, a.handleTokenRefreshError(body, resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	t := &oauth2.Token{}
	if at, ok := result["access_token"].(string); ok {
		t.AccessToken = at
	}
	if rt, ok := result["refresh_token"].(string); ok {
		t.RefreshToken = rt
	} else {
		// If service didn't return a new refresh token, keep using the old one
		t.RefreshToken = refreshToken
	}
	if exp, ok := result["expires_in"].(float64); ok {
		t.Expiry = time.Now().Add(time.Duration(exp) * time.Second)
	}

	return t, nil
}

// ensureLoginScopes returns a unique list including required OIDC/refresh scopes for device flow.
func ensureLoginScopes(configured []string) []string {
	base := map[string]bool{
		"openid":         true,
		"profile":        true,
		"offline_access": true,
		"https://management.azure.com/user_impersonation": true,
	}
	for _, s := range configured {
		base[strings.TrimSpace(s)] = true
	}
	out := make([]string, 0, len(base))
	for s := range base {
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	// Keep output stable-ish: put OIDC scopes first, then others (best-effort)
	order := []string{"openid", "profile", "offline_access"}
	result := make([]string, 0, len(out))
	added := map[string]bool{}
	for _, o := range order {
		if base[o] {
			result = append(result, o)
			added[o] = true
		}
	}
	for _, s := range out {
		if !added[s] {
			result = append(result, s)
		}
	}
	return result
}

// decodeJWTClaims decodes the payload of a JWT access token without validation.
// Returns an empty map when decoding fails or token isn't a JWT.
func decodeJWTClaims(token string) map[string]interface{} {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return map[string]interface{}{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return map[string]interface{}{}
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return map[string]interface{}{}
	}
	return claims
}

// handleTokenRefreshError handles error responses from token refresh requests
func (a *Authenticator) handleTokenRefreshError(body []byte, statusCode int) error {
	// Attempt to decode error for clarity
	var errObj map[string]interface{}
	_ = json.Unmarshal(body, &errObj)
	logging.Error("Refresh token exchange failed", "status", fmt.Sprintf("%d", statusCode), "body", string(body))

	// Provide actionable error messages for common issues
	if errDesc, ok := errObj["error_description"].(string); ok {
		if strings.Contains(errDesc, "AADSTS65001") || strings.Contains(errDesc, "consent_required") {
			return fmt.Errorf("user consent required: please run 'login' to re-authorize the application with the required scopes. Visit Azure portal to review app permissions if needed")
		}
		if strings.Contains(errDesc, "AADSTS70008") || strings.Contains(errDesc, "refresh_token_expired") {
			return fmt.Errorf("refresh token expired: please run 'login' to re-authenticate")
		}
	}

	return fmt.Errorf("refresh request failed: status=%d", statusCode)
}

// validateARMTokenAudience validates the token audience for ARM requests
func (a *Authenticator) validateARMTokenAudience(token *oauth2.Token, scopes []string) error {
	if len(scopes) == 1 && (scopes[0] == "https://management.azure.com/.default" || scopes[0] == "https://management.azure.com/user_impersonation") {
		if token.AccessToken != "" {
			claims := decodeJWTClaims(token.AccessToken)
			aud := fmt.Sprint(claims["aud"])
			if !strings.HasPrefix(aud, "https://management.azure.com") && !strings.HasPrefix(aud, "https://management.core.windows.net") {
				logging.Error("Received access token with unexpected audience for ARM scope", "aud", aud)
				return fmt.Errorf("token audience mismatch: expected Azure Resource Manager; please run 'login' to grant ARM permissions")
			}
		}
	}
	return nil
}
