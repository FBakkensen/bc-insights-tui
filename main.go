package main

// Entry point for bc-insights-tui
import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
	"github.com/FBakkensen/bc-insights-tui/tui"
)

func main() {
	// Parse command line flags
	runCmd := flag.String("run", "", "Run a command non-interactively (e.g., 'subs', 'login')")
	flag.Parse()

	// Initialize logging first (allow override via env)
	logLevel := logging.LevelInfo
	if v := strings.TrimSpace(os.Getenv("BC_INSIGHTS_LOG_LEVEL")); v != "" {
		switch strings.ToUpper(v) {
		case logging.LevelDebug:
			logLevel = logging.LevelDebug
		case logging.LevelInfo:
			logLevel = logging.LevelInfo
		case logging.LevelWarn:
			logLevel = logging.LevelWarn
		case logging.LevelError:
			logLevel = logging.LevelError
		}
	}
	if err := logging.InitLogger(logLevel); err != nil {
		fmt.Printf("Warning: Failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	logging.Info("Starting bc-insights-tui application (chat-first rewrite - Step 1)")

	// Load configuration
	cfg := config.LoadConfig()
	logging.Info("Configuration loaded", "logFetchSize", fmt.Sprintf("%d", cfg.LogFetchSize))

	// If run command is specified, execute it non-interactively
	if *runCmd != "" {
		err := runNonInteractiveCommand(*runCmd, cfg)
		if err != nil {
			logging.Error("Non-interactive command failed", "command", *runCmd, "error", err.Error())
			fmt.Printf("Error running command '%s': %v\n", *runCmd, err)
			os.Exit(1)
		}
		return
	}

	// Start chat-first UI (Step 1: Login)
	if err := tui.Run(cfg); err != nil {
		logging.Error("UI exited with error", "error", err.Error())
		fmt.Println("Error:", err)
	}
}

// runNonInteractiveCommand executes a command without starting the TUI
func runNonInteractiveCommand(command string, cfg config.Config) error {
	logging.Info("Running non-interactive command", "command", command)

	switch command {
	case "subs":
		return listSubscriptionsNonInteractive(cfg)
	case "login":
		return loginNonInteractive(cfg)
	case "resources":
		return listInsightsResourcesNonInteractive(cfg)
	case "config":
		return showConfigNonInteractive(cfg)
	case "config-save":
		return saveConfigNonInteractive(cfg)
	case "config-reset":
		return resetConfigNonInteractive(cfg)
	case "config-path":
		return showConfigPathNonInteractive()
	default:
		return fmt.Errorf("unknown command: %s. Available commands: subs, login, resources, config, config-save, config-reset, config-path", command)
	}
}

// listSubscriptionsNonInteractive lists Azure subscriptions without TUI
func listSubscriptionsNonInteractive(cfg config.Config) error {
	logging.Debug("Starting non-interactive subscription listing")

	// Create authenticator
	authenticator := auth.NewAuthenticator(cfg.OAuth2)

	// Check if we have a valid token
	if !authenticator.HasValidToken() {
		return fmt.Errorf("no valid authentication token found. Run with -run=login first")
	}

	// Create Azure client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logging.Debug("Creating Azure client with authenticator")
	client, err := appinsights.NewAzureClientWithAuthenticator(authenticator)
	if err != nil {
		logging.Error("Failed to create Azure client", "error", err.Error())
		return fmt.Errorf("failed to create Azure client: %w", err)
	}

	// List subscriptions
	logging.Debug("Calling ListSubscriptions")
	subs, err := client.ListSubscriptions(ctx)
	if err != nil {
		logging.Error("Failed to list subscriptions", "error", err.Error())
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	// Print results
	fmt.Printf("Found %d subscriptions:\n", len(subs))
	for i, s := range subs {
		fmt.Printf("%d. %s (%s) - State: %s\n", i+1, s.DisplayName, s.ID, s.State)
		logging.Debug("Subscription found", "index", fmt.Sprintf("%d", i), "id", s.ID, "name", s.DisplayName, "state", s.State)
	}

	logging.Info("Successfully listed subscriptions", "count", fmt.Sprintf("%d", len(subs)))
	return nil
}

// loginNonInteractive performs device flow login without TUI
func loginNonInteractive(cfg config.Config) error {
	logging.Debug("Starting non-interactive login")

	// Create authenticator
	authenticator := auth.NewAuthenticator(cfg.OAuth2)

	// Check if already authenticated
	if authenticator.HasValidToken() {
		fmt.Println("Already authenticated with a valid token.")
		logging.Info("Already authenticated")
		return nil
	}

	// Start device flow
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	fmt.Println("Starting Azure device flow authentication...")
	logging.Debug("Initiating device flow")

	resp, err := authenticator.InitiateDeviceFlow(ctx)
	if err != nil {
		logging.Error("Failed to initiate device flow", "error", err.Error())
		return fmt.Errorf("failed to initiate device flow: %w", err)
	}

	// Show user instructions
	if resp.VerificationURI != "" && resp.UserCode != "" {
		fmt.Printf("Open %s and enter code %s\n", resp.VerificationURI, resp.UserCode)
	}
	if resp.VerificationURIComplete != "" {
		fmt.Printf("Or open: %s\n", resp.VerificationURIComplete)
	}
	fmt.Println("Waiting for verification...")

	// Poll for token
	logging.Debug("Polling for token")
	token, err := authenticator.PollForToken(ctx, resp.DeviceCode, resp.Interval)
	if err != nil {
		logging.Error("Failed to poll for token", "error", err.Error())
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save token
	if err := authenticator.SaveTokenSecurely(token); err != nil {
		logging.Error("Failed to save token", "error", err.Error())
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("Authentication successful! Token saved.")
	logging.Info("Authentication completed successfully")
	return nil
}

// listInsightsResourcesNonInteractive lists Application Insights resources without TUI
func listInsightsResourcesNonInteractive(cfg config.Config) error {
	logging.Debug("Starting non-interactive Application Insights resource listing")

	// Check if we have a subscription ID configured
	subscriptionID := cfg.SubscriptionID
	if subscriptionID == "" {
		return fmt.Errorf("no subscription selected. Use 'subs' command to list subscriptions and then set azure.subscriptionId in config")
	}

	// Create authenticator
	authenticator := auth.NewAuthenticator(cfg.OAuth2)

	// Check if we have a valid token
	if !authenticator.HasValidToken() {
		return fmt.Errorf("no valid authentication token found. Run with -run=login first")
	}

	// Create Azure client
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logging.Debug("Creating Azure client with authenticator for Application Insights resources")
	client, err := appinsights.NewAzureClientWithAuthenticator(authenticator)
	if err != nil {
		logging.Error("Failed to create Azure client for insights", "error", err.Error())
		return fmt.Errorf("failed to create Azure client: %w", err)
	}

	// List Application Insights resources for the subscription
	logging.Debug("Calling ListApplicationInsightsResourcesForSubscription", "subscriptionID", subscriptionID)
	resources, err := client.ListApplicationInsightsResourcesForSubscription(ctx, subscriptionID)
	if err != nil {
		logging.Error("Failed to list Application Insights resources", "error", err.Error())
		return fmt.Errorf("failed to list Application Insights resources: %w", err)
	}

	// Print results
	fmt.Printf("Found %d Application Insights resources in subscription %s:\n", len(resources), subscriptionID)
	for i, r := range resources {
		fmt.Printf("%d. %s\n", i+1, r.Name)
		fmt.Printf("   Resource Group: %s\n", r.ResourceGroup)
		fmt.Printf("   Location: %s\n", r.Location)
		fmt.Printf("   Application ID: %s\n", r.ApplicationID)

		// Safely display instrumentation key preview
		var keyPreview string
		if len(r.InstrumentationKey) >= 8 {
			keyPreview = r.InstrumentationKey[:8] + "..."
		} else if len(r.InstrumentationKey) > 0 {
			keyPreview = r.InstrumentationKey + "..."
		} else {
			keyPreview = "(empty)"
		}
		fmt.Printf("   Instrumentation Key: %s\n", keyPreview)

		fmt.Println()
		logging.Debug("Application Insights resource found",
			"index", fmt.Sprintf("%d", i),
			"name", r.Name,
			"resourceGroup", r.ResourceGroup,
			"applicationID", r.ApplicationID)
	}

	logging.Info("Successfully listed Application Insights resources", "count", fmt.Sprintf("%d", len(resources)))
	return nil
}

// showConfigNonInteractive prints the current configuration
func showConfigNonInteractive(cfg config.Config) error {
	logging.Info("Showing configuration settings")

	// Show configuration settings directly
	fmt.Println("Current Configuration:")
	fmt.Printf("  ApplicationInsightsID: %s\n", cfg.ApplicationInsightsID)
	fmt.Printf("  ApplicationInsightsKey: %s\n", cfg.ApplicationInsightsKey)
	fmt.Printf("  Environment: %s\n", cfg.Environment)
	fmt.Printf("  SubscriptionID: %s\n", cfg.SubscriptionID)
	fmt.Printf("  OAuth2 TenantID: %s\n", cfg.OAuth2.TenantID)
	fmt.Printf("  OAuth2 ClientID: %s\n", cfg.OAuth2.ClientID)
	fmt.Printf("  LogFetchSize: %d\n", cfg.LogFetchSize)
	fmt.Printf("  QueryTimeoutSeconds: %d\n", cfg.QueryTimeoutSeconds)

	return nil
}

// saveConfigNonInteractive manually saves the current configuration to file
func saveConfigNonInteractive(cfg config.Config) error {
	logging.Info("Manually saving configuration to file")

	if err := cfg.SaveConfig(); err != nil {
		fmt.Printf("Error: Failed to save configuration: %v\n", err)
		return err
	}

	fmt.Println("Configuration saved successfully.")
	return nil
}

// resetConfigNonInteractive deletes the config file and uses defaults
func resetConfigNonInteractive(cfg config.Config) error {
	logging.Info("Resetting configuration to defaults")

	// Get config file path
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Error: Failed to get user config directory: %v\n", err)
		return err
	}

	configPath := filepath.Join(configDir, "bc-insights-tui", "config.json")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No config file found. Using defaults.")
		return nil
	}

	// Delete the config file
	if err := os.Remove(configPath); err != nil {
		fmt.Printf("Error: Failed to delete config file %s: %v\n", configPath, err)
		return err
	}

	fmt.Printf("Configuration file deleted: %s\n", configPath)
	fmt.Println("Application will use default settings on next run.")
	return nil
}

// showConfigPathNonInteractive shows the config file location
func showConfigPathNonInteractive() error {
	logging.Info("Showing configuration file path")

	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Error: Failed to get user config directory: %v\n", err)
		return err
	}

	configPath := filepath.Join(configDir, "bc-insights-tui", "config.json")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Printf("Config file path: %s (does not exist)\n", configPath)
	} else {
		fmt.Printf("Config file path: %s (exists)\n", configPath)
	}

	return nil
}
