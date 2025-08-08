package main

// Entry point for bc-insights-tui
import (
	"context"
	"flag"
	"fmt"
	"os"
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
	default:
		return fmt.Errorf("unknown command: %s. Available commands: subs, login", command)
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
