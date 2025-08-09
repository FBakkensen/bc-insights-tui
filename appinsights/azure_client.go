package appinsights

// Azure Resource Manager client for discovering Application Insights resources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/applicationinsights/armapplicationinsights"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/logging"
	"golang.org/x/oauth2"
)

// ApplicationInsightsResource represents a discovered Application Insights resource
type ApplicationInsightsResource struct {
	Name               string `json:"name"`
	ResourceGroup      string `json:"resourceGroup"`
	SubscriptionID     string `json:"subscriptionId"`
	Location           string `json:"location"`
	ApplicationID      string `json:"applicationId"`
	InstrumentationKey string `json:"instrumentationKey"`
	ConnectionString   string `json:"connectionString"`
	ResourceID         string `json:"resourceId"`
}

// AzureSubscription represents an Azure subscription
type AzureSubscription struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TenantID    string `json:"tenantId"`
	State       string `json:"state"`
	DisplayName string `json:"displayName"`
}

// FormatSubscriptionForDisplay returns a formatted string for displaying the subscription
func (s AzureSubscription) FormatSubscriptionForDisplay() string {
	return fmt.Sprintf("%s (%s) - %s", s.DisplayName, s.ID, s.State)
}

// DisplayText implements SelectableItem interface for AzureSubscription
func (s AzureSubscription) DisplayText() string {
	return s.FormatSubscriptionForDisplay()
}

// UniqueID implements SelectableItem interface for AzureSubscription
func (s AzureSubscription) UniqueID() string {
	return s.ID
}

// AzureClient provides methods to discover and interact with Azure Application Insights resources
type AzureClient struct {
	credential azcore.TokenCredential
}

// NewAzureClient creates a new Azure Resource Manager client using OAuth2 token
func NewAzureClient(token *oauth2.Token) (*AzureClient, error) {
	if token == nil {
		return nil, fmt.Errorf("no authentication token provided")
	}

	if !token.Valid() {
		return nil, fmt.Errorf("provided authentication token is not valid or expired")
	}

	// Wrap the provided OAuth2 token in an azcore.TokenCredential
	credential := &oauth2TokenCredential{token: token}

	return &AzureClient{credential: credential}, nil
}

// oauth2TokenCredential is a minimal azcore.TokenCredential implementation that
// returns a pre-acquired OAuth2 access token.
type oauth2TokenCredential struct {
	token *oauth2.Token
}

// GetToken implements azcore.TokenCredential.
func (c *oauth2TokenCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if c == nil || c.token == nil {
		return azcore.AccessToken{}, fmt.Errorf("no token available")
	}
	// Ensure token hasn't expired. We don't attempt refresh here; the caller should ensure validity.
	if c.token.Expiry.IsZero() || time.Until(c.token.Expiry) <= 0 {
		return azcore.AccessToken{}, fmt.Errorf("token expired")
	}
	return azcore.AccessToken{
		Token:     c.token.AccessToken,
		ExpiresOn: c.token.Expiry,
	}, nil
}

// authenticatorCredential implements azcore.TokenCredential using our Authenticator, supporting audience-specific scopes.
type authenticatorCredential struct {
	auth *auth.Authenticator
}

func (c *authenticatorCredential) GetToken(ctx context.Context, tro policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// tro.Scopes contains requested resource scopes (e.g., https://management.azure.com/.default)
	scopes := tro.Scopes
	if len(scopes) == 0 {
		// Fallback to ARM default scope
		scopes = []string{"https://management.azure.com/.default"}
	}
	// Normalize scopes: prefer management.azure.com and collapse accidental double slashes
	norm := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.ReplaceAll(s, "https://management.core.windows.net", "https://management.azure.com")
		s = strings.ReplaceAll(s, "//.default", "/.default")
		norm = append(norm, s)
	}
	logging.Debug("ARM credential requesting token", "scopes", strings.Join(norm, " "))
	tok, err := c.auth.GetTokenForScopes(ctx, norm)
	if err != nil {
		logging.Error("Failed to get ARM-scoped token", "error", err.Error())
		// Provide more helpful error context for authentication issues
		if strings.Contains(err.Error(), "consent_required") || strings.Contains(err.Error(), "AADSTS65001") {
			return azcore.AccessToken{}, fmt.Errorf("azure Management API access not authorized: please run 'login' command to grant required permissions")
		}
		if strings.Contains(err.Error(), "refresh request failed") {
			return azcore.AccessToken{}, fmt.Errorf("authentication token expired: please run 'login' command to re-authenticate")
		}
		return azcore.AccessToken{}, err
	}
	// Lightweight claims decoding for diagnostics only
	if tok != nil && tok.AccessToken != "" {
		// best-effort decode of JWT payload
		parts := strings.Split(tok.AccessToken, ".")
		if len(parts) >= 2 {
			if payload, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
				var claims map[string]interface{}
				if err := json.Unmarshal(payload, &claims); err == nil {
					logging.Debug("ARM token claims",
						"aud", fmt.Sprint(claims["aud"]),
						"scp", fmt.Sprint(claims["scp"]),
						"tid", fmt.Sprint(claims["tid"]))
				}
			}
		}
		logging.Debug("ARM token expiry", "expires", tok.Expiry.Format(time.RFC3339))
	}
	logging.Debug("ARM credential returning token successfully")
	return azcore.AccessToken{Token: tok.AccessToken, ExpiresOn: tok.Expiry}, nil
}

// NewAzureClientWithAuthenticator builds an AzureClient backed by the authenticator for dynamic scopes
func NewAzureClientWithAuthenticator(a *auth.Authenticator) (*AzureClient, error) {
	if a == nil {
		return nil, fmt.Errorf("authenticator is nil")
	}
	return &AzureClient{credential: &authenticatorCredential{auth: a}}, nil
}

// ListApplicationInsightsResources lists all Application Insights resources accessible to the authenticated user
func (ac *AzureClient) ListApplicationInsightsResources(ctx context.Context) ([]ApplicationInsightsResource, error) {
	if ac.credential == nil {
		return nil, fmt.Errorf("no Azure credential available")
	}

	// First, we need to get all subscriptions the user has access to
	subscriptions, err := ac.getAccessibleSubscriptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible subscriptions: %w", err)
	}

	var allResources []ApplicationInsightsResource

	// For each subscription, list Application Insights resources
	for _, subscriptionID := range subscriptions {
		resources, err := ac.listResourcesInSubscription(ctx, subscriptionID)
		if err != nil {
			// Log error but continue with other subscriptions
			continue
		}
		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// ListApplicationInsightsResourcesForSubscription lists Application Insights resources for a specific subscription
func (ac *AzureClient) ListApplicationInsightsResourcesForSubscription(ctx context.Context, subscriptionID string) ([]ApplicationInsightsResource, error) {
	if ac.credential == nil {
		return nil, fmt.Errorf("no Azure credential available")
	}

	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	logging.Debug("Listing AI resources for subscription", "subscriptionId", subscriptionID)
	return ac.listResourcesInSubscription(ctx, subscriptionID)
}

// getAccessibleSubscriptions gets all subscription IDs accessible to the user
func (ac *AzureClient) getAccessibleSubscriptions(ctx context.Context) ([]string, error) {
	// This is a simplified implementation that would need to be expanded
	// to actually enumerate subscriptions. For now, we'll require the user
	// to provide subscription IDs through configuration or environment variables

	// Check if subscription ID is provided via environment variable
	subscriptionID := getSubscriptionFromEnvironment()
	if subscriptionID == "" {
		return nil, fmt.Errorf("no subscription ID found. Please set AZURE_SUBSCRIPTION_ID environment variable")
	}

	return []string{subscriptionID}, nil
}

// getSubscriptionFromEnvironment gets subscription ID from environment variables
func getSubscriptionFromEnvironment() string {
	// Common environment variable names for Azure subscription ID
	envVars := []string{
		"AZURE_SUBSCRIPTION_ID",
		"BCINSIGHTS_AZURE_SUBSCRIPTION_ID",
		"ARM_SUBSCRIPTION_ID",
	}

	for _, envVar := range envVars {
		if value := getEnvValue(envVar); value != "" {
			return value
		}
	}

	return ""
}

// getEnvValue is a helper to get environment variable values
func getEnvValue(key string) string {
	return os.Getenv(key)
}

// listResourcesInSubscription lists Application Insights resources in a specific subscription
func (ac *AzureClient) listResourcesInSubscription(ctx context.Context, subscriptionID string) ([]ApplicationInsightsResource, error) {
	// Create Application Insights client for this subscription
	logging.Debug("Creating AI client factory", "subscriptionId", subscriptionID)
	clientFactory, err := armapplicationinsights.NewClientFactory(subscriptionID, ac.credential, nil)
	if err != nil {
		logging.Error("Failed to create AI client factory", "subscriptionId", subscriptionID, "error", err.Error())
		// Add actionable guidance with likely causes and next steps
		return nil, fmt.Errorf("failed to create Application Insights client factory: %w. Likely causes: (1) invalid or missing Azure credential (token expired or wrong scopes), (2) incorrect subscription ID, (3) network or proxy restrictions blocking https://management.azure.com. Next steps: re-run 'login', ensure scope includes https://management.azure.com/.default, verify AZURE_SUBSCRIPTION_ID, and check network/proxy settings. See docs: https://learn.microsoft.com/azure/azure-resource-manager/management/managed-identity-apis-use and SDK auth: https://learn.microsoft.com/azure/developer/go/azure-sdk-authentication", err)
	}

	componentsClient := clientFactory.NewComponentsClient()

	// List all Application Insights components in the subscription
	logging.Debug("Starting AI components pager", "subscriptionId", subscriptionID)
	pager := componentsClient.NewListPager(nil)
	var resources []ApplicationInsightsResource

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			logging.Error("Failed to list AI components page", "subscriptionId", subscriptionID, "error", err.Error())
			return nil, fmt.Errorf(
				"failed to list Application Insights components: %w. Likely causes: insufficient RBAC on subscription '%s' or network/proxy blocking ARM. Next steps: verify access in Azure Portal Subscriptions (https://portal.azure.com/#view/Microsoft_Azure_Billing/SubscriptionsBlade), ensure a role like 'Monitoring Reader', and check corporate proxy settings",
				err, subscriptionID,
			)
		}

		logging.Debug("Received AI components page", "subscriptionId", subscriptionID, "count", fmt.Sprintf("%d", len(page.Value)))
		for _, component := range page.Value {
			if component == nil {
				continue
			}

			resource := ac.convertComponentToResource(component, subscriptionID)
			resources = append(resources, resource)
		}
	}

	logging.Info("Completed AI components listing", "subscriptionId", subscriptionID, "total", fmt.Sprintf("%d", len(resources)))
	return resources, nil
}

// convertComponentToResource converts an ARM Application Insights component to our resource type
func (ac *AzureClient) convertComponentToResource(component *armapplicationinsights.Component, subscriptionID string) ApplicationInsightsResource {
	resource := ApplicationInsightsResource{
		SubscriptionID: subscriptionID,
	}

	if component.Name != nil {
		resource.Name = *component.Name
	}

	if component.ID != nil {
		resource.ResourceID = *component.ID
		// Extract resource group from resource ID
		// Format: /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.Insights/components/{componentName}
		parts := strings.Split(*component.ID, "/")
		if len(parts) >= 5 {
			resource.ResourceGroup = parts[4]
		}
	}

	if component.Location != nil {
		resource.Location = *component.Location
	}

	if component.Properties != nil {
		if component.Properties.AppID != nil {
			resource.ApplicationID = *component.Properties.AppID
		}
		if component.Properties.InstrumentationKey != nil {
			resource.InstrumentationKey = *component.Properties.InstrumentationKey
		}
		if component.Properties.ConnectionString != nil {
			resource.ConnectionString = *component.Properties.ConnectionString
		}
	}

	return resource
}

// GetResourceDetails gets detailed information for a specific Application Insights resource
func (ac *AzureClient) GetResourceDetails(ctx context.Context, subscriptionID, resourceGroupName, componentName string) (*ApplicationInsightsResource, error) {
	clientFactory, err := armapplicationinsights.NewClientFactory(subscriptionID, ac.credential, nil)
	if err != nil {
		// Provide actionable context to help resolve
		return nil, fmt.Errorf("failed to create Application Insights client factory: %w. This typically indicates an auth or configuration issue. Verify that your token grants access to Azure Resource Manager (scope https://management.azure.com/.default), and that the subscription ID '%s' is correct and accessible. If behind a corporate proxy, configure proxy environment variables. Docs: ARM overview https://learn.microsoft.com/azure/azure-resource-manager/management/overview and Go SDK auth https://learn.microsoft.com/azure/developer/go/azure-sdk-authentication", err, subscriptionID)
	}

	componentsClient := clientFactory.NewComponentsClient()

	// Get the specific component
	resp, err := componentsClient.Get(ctx, resourceGroupName, componentName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Application Insights component: %w", err)
	}

	resource := ac.convertComponentToResource(&resp.Component, subscriptionID)
	return &resource, nil
}

// FormatResourceForDisplay formats a resource for display in the TUI
func (r *ApplicationInsightsResource) FormatResourceForDisplay() string {
	if r.Name == "" {
		return "Unknown Resource"
	}

	// Format: "ResourceName (ResourceGroup/Location)"
	location := r.Location
	if location == "" {
		location = "unknown"
	}

	resourceGroup := r.ResourceGroup
	if resourceGroup == "" {
		resourceGroup = "unknown"
	}

	return fmt.Sprintf("%s (%s/%s)", r.Name, resourceGroup, location)
}

// ListSubscriptions lists all Azure subscriptions accessible to the authenticated user
func (c *AzureClient) ListSubscriptions(ctx context.Context) ([]AzureSubscription, error) {
	logging.Debug("ListSubscriptions called")

	client, err := armsubscriptions.NewClient(c.credential, nil)
	if err != nil {
		logging.Error("Failed to create subscriptions client", "error", err.Error())
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	logging.Debug("Created ARM subscriptions client, starting to list subscriptions")
	var subscriptions []AzureSubscription
	pager := client.NewListPager(nil)

	pageCount := 0
	for pager.More() {
		pageCount++
		logging.Debug("Fetching subscription page", "pageNumber", fmt.Sprintf("%d", pageCount))

		page, err := pager.NextPage(ctx)
		if err != nil {
			logging.Error("Failed to get subscriptions page", "pageNumber", fmt.Sprintf("%d", pageCount), "error", err.Error())
			return nil, fmt.Errorf("failed to get subscriptions page: %w", err)
		}

		logging.Debug("Received subscription page", "pageNumber", fmt.Sprintf("%d", pageCount), "subscriptionsInPage", fmt.Sprintf("%d", len(page.Value)))

		for i, sub := range page.Value {
			logging.Debug("Processing subscription from page", "pageNumber", fmt.Sprintf("%d", pageCount), "subscriptionIndex", fmt.Sprintf("%d", i),
				"hasSubscriptionID", fmt.Sprintf("%v", sub.SubscriptionID != nil),
				"hasDisplayName", fmt.Sprintf("%v", sub.DisplayName != nil),
				"hasState", fmt.Sprintf("%v", sub.State != nil))

			if sub.SubscriptionID == nil || sub.DisplayName == nil || sub.State == nil {
				logging.Warn("Skipping incomplete subscription data", "pageNumber", fmt.Sprintf("%d", pageCount), "subscriptionIndex", fmt.Sprintf("%d", i))
				continue // Skip incomplete subscription data
			}

			subscription := AzureSubscription{
				ID:          *sub.SubscriptionID,
				DisplayName: *sub.DisplayName,
				State:       string(*sub.State),
			}

			// Set tenant ID if available
			if sub.TenantID != nil {
				subscription.TenantID = *sub.TenantID
			}

			// Set name (use DisplayName as fallback)
			subscription.Name = subscription.DisplayName

			logging.Debug("Added subscription", "id", subscription.ID, "displayName", subscription.DisplayName, "state", subscription.State)
			subscriptions = append(subscriptions, subscription)
		}
	}

	logging.Info("ListSubscriptions completed", "totalSubscriptions", fmt.Sprintf("%d", len(subscriptions)), "pagesProcessed", fmt.Sprintf("%d", pageCount))
	return subscriptions, nil
}

// IsConfigured returns true if the resource has both Application ID and connection string
func (r *ApplicationInsightsResource) IsConfigured() bool {
	return r.ApplicationID != "" && r.ConnectionString != ""
}
