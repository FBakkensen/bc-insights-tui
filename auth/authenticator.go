package auth

// OAuth2 Device Authorization Flow logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	// Service name for keyring storage
	serviceName = "bc-insights-tui"
	// tokenKey is not a credential but a key name for storing tokens in the system keyring
	tokenKey = "oauth2-token" // #nosec G101
)

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

	isValid := token.Valid()
	logging.Debug("Token validation result", "valid", strconv.FormatBool(isValid))
	return isValid
}

// InitiateDeviceFlow starts the device authorization flow
func (a *Authenticator) InitiateDeviceFlow(ctx context.Context) (*DeviceCodeResponse, error) {
	logging.Info("Initiating device authorization flow")
	deviceEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", a.config.TenantID)
	logging.Debug("Device endpoint", "url", deviceEndpoint)

	data := url.Values{}
	data.Set("client_id", a.config.ClientID)
	data.Set("scope", strings.Join(a.config.Scopes, " "))

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

	return token, nil
}

// SaveTokenSecurely stores the token in the OS credential manager
func (a *Authenticator) SaveTokenSecurely(token *oauth2.Token) error {
	logging.Info("Saving authentication token securely")
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		logging.Error("Failed to marshal token", "error", err.Error())
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := keyring.Set(serviceName, tokenKey, string(tokenJSON)); err != nil {
		logging.Error("Failed to store token in keyring", "error", err.Error())
		return fmt.Errorf("failed to store token in keyring: %w", err)
	}

	logging.Info("Token saved successfully to secure storage")
	return nil
}

// getStoredToken retrieves the stored token from the OS credential manager
func (a *Authenticator) getStoredToken() (*oauth2.Token, error) {
	logging.Debug("Retrieving stored token from keyring")
	tokenJSON, err := keyring.Get(serviceName, tokenKey)
	if err != nil {
		if err == keyring.ErrNotFound {
			logging.Info("No stored token found in keyring")
			return nil, ErrNoStoredToken
		}
		logging.Error("Failed to retrieve token from keyring", "error", err.Error())
		return nil, fmt.Errorf("failed to get token from keyring: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		logging.Error("Failed to unmarshal stored token", "error", err.Error())
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	logging.Debug("Successfully retrieved and parsed stored token")
	return &token, nil
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

// ClearToken removes the stored token
func (a *Authenticator) ClearToken() error {
	logging.Info("Clearing stored authentication token")
	if err := keyring.Delete(serviceName, tokenKey); err != nil {
		logging.Error("Failed to delete token from keyring", "error", err.Error())
		return fmt.Errorf("failed to delete token from keyring: %w", err)
	}
	logging.Info("Token successfully cleared from secure storage")
	return nil
}
