package main

// Entry point for bc-insights-tui
import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/auth"
	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/logging"
	"github.com/FBakkensen/bc-insights-tui/tui"
	keyring "github.com/zalando/go-keyring"
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

	// Split optional argument (e.g., logs:250)
	name := command
	arg := ""
	if idx := strings.Index(command, ":"); idx != -1 {
		name = command[:idx]
		arg = strings.TrimSpace(command[idx+1:])
	}

	// Special-case: logs[:N]
	if name == "logs" {
		lines := 200
		if arg != "" {
			v, err := strconv.Atoi(arg)
			if err != nil || v <= 0 {
				return fmt.Errorf("invalid logs line count: %q; use a positive integer (e.g., -run=logs:200)", arg)
			}
			lines = v
		}
		return tailLatestLogFileNonInteractive(lines)
	}

	// Command registry to keep complexity low
	handlers := map[string]func() error{
		"subs":         func() error { return listSubscriptionsNonInteractive(cfg) },
		"login":        func() error { return loginNonInteractive(cfg) },
		"keyring-test": func() error { return keyringTestNonInteractive(cfg) },
		"keyring-info": func() error { return keyringInfoNonInteractive() },
		"resources":    func() error { return listInsightsResourcesNonInteractive(cfg) },
		"config":       func() error { return showConfigNonInteractive(cfg) },
		"config-save":  func() error { return saveConfigNonInteractive(cfg) },
		"config-reset": func() error { return resetConfigNonInteractive(cfg) },
		"config-path":  func() error { return showConfigPathNonInteractive() },
		"login-status": func() error { return loginStatusNonInteractive(cfg) },
	}

	if h, ok := handlers[name]; ok {
		return h()
	}
	return fmt.Errorf("unknown command: %s. Available commands: subs, login, login-status, keyring-info, keyring-test, resources, config, config-save, config-reset, config-path, logs[:N]", command)
}

// tailLatestLogFileNonInteractive prints the last N lines of the newest log file in logs/.
func tailLatestLogFileNonInteractive(lines int) error {
	logging.Info("Tailing latest log file", "lines", fmt.Sprintf("%d", lines))

	logsDir := "logs"
	path, err := latestLogFilePath(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No logs directory found. Start the app once to generate logs.")
			return nil
		}
		return err
	}
	if path == "" {
		fmt.Println("No log files found.")
		return nil
	}

	// Print header
	fmt.Printf("==> %s (last %d lines)\n", path, lines)
	printed, err := tailFileLines(path, lines)
	if err != nil {
		return err
	}
	logging.Info("Completed tail of latest log file", "file", path, "linesPrinted", fmt.Sprintf("%d", printed))
	return nil
}

// latestLogFilePath returns the full path of the newest bc-insights-tui-YYYY-MM-DD.log file in the given directory.
func latestLogFilePath(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var latestName string
	var latestTime time.Time
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, "bc-insights-tui-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, infoErr := de.Info()
		if infoErr != nil {
			continue
		}
		if latestName == "" || info.ModTime().After(latestTime) {
			latestName = name
			latestTime = info.ModTime()
		}
	}

	if latestName == "" {
		return "", nil
	}
	return filepath.Join(dir, latestName), nil
}

// tailFileLines prints the last N lines from the provided file path and returns the number of lines printed.
func tailFileLines(path string, lines int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		logging.Error("Failed to open log file", "file", path, "error", err.Error())
		return 0, fmt.Errorf("failed to open log file %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Increase the scanner buffer to handle long lines
	const maxCapacity = 1024 * 1024 // 1 MiB
	scanner.Buffer(make([]byte, 64*1024), maxCapacity)

	// Ring buffer of last N lines
	buf := make([]string, 0, lines)
	idx := 0
	for scanner.Scan() {
		line := scanner.Text()
		if len(buf) < lines {
			buf = append(buf, line)
		} else {
			buf[idx] = line
			idx = (idx + 1) % lines
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		logging.Warn("Scanner error while reading log file", "file", path, "error", scanErr.Error())
	}

	// Output
	if len(buf) < lines {
		for _, l := range buf {
			fmt.Println(l)
		}
		return len(buf), nil
	}
	for i := 0; i < len(buf); i++ {
		j := (idx + i) % len(buf)
		fmt.Println(buf[j])
	}
	return len(buf), nil
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

// loginStatusNonInteractive prints diagnostics about stored auth state and attempts a silent refresh
func loginStatusNonInteractive(cfg config.Config) error {
	logging.Info("Running login status diagnostics")

	// Basic environment context (no secrets)
	uname := strings.TrimSpace(os.Getenv("USERNAME"))
	udom := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	uprof := strings.TrimSpace(os.Getenv("USERPROFILE"))
	pid := os.Getpid()
	fmt.Println("Process/User Context:")
	fmt.Printf("  PID: %d\n", pid)
	if udom != "" || uname != "" {
		if udom != "" {
			fmt.Printf("  User: %s\\%s\n", udom, uname)
		} else {
			fmt.Printf("  User: %s\n", uname)
		}
	}
	if uprof != "" {
		fmt.Printf("  USERPROFILE: %s\n", uprof)
	}

	// Auth checks
	authenticator := auth.NewAuthenticator(cfg.OAuth2)
	present, err := authenticator.StoredRefreshTokenPresent()
	if err != nil {
		logging.Error("Keyring presence check failed", "error", err.Error())
		fmt.Printf("Stored refresh token: error checking presence: %v\n", err)
	} else if !present {
		fmt.Println("Stored refresh token: not found (interactive login required)")
	} else {
		fmt.Println("Stored refresh token: found")
	}

	// Show effective keyring entry locations (primary and backup)
	svc, key := auth.KeyringEntryInfo()
	bsvc, bkey := auth.KeyringBackupEntryInfo()
	fmt.Println()
	fmt.Println("Keyring entries (effective):")
	fmt.Printf("  Primary: %s / %s\n", svc, key)
	fmt.Printf("  Backup:  %s / %s\n", bsvc, bkey)
	if v := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_SERVICE")); v != "" {
		fmt.Printf("  Env BCINSIGHTS_KEYRING_SERVICE=%s\n", v)
	}
	if v := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_NAMESPACE")); v != "" {
		fmt.Printf("  Env BCINSIGHTS_KEYRING_NAMESPACE=%s\n", v)
	}

	// If present, attempt a silent token acquisition for ARM to validate refresh
	if present {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		scope := "https://management.azure.com/.default"
		fmt.Printf("Attempting silent refresh for scope: %s\n", scope)
		tok, terr := authenticator.GetTokenForScopes(ctx, []string{scope})
		if terr != nil {
			logging.Error("Silent refresh test failed", "error", terr.Error())
			fmt.Printf("Silent refresh: FAILED: %v\n", terr)
		} else {
			exp := tok.Expiry
			if !exp.IsZero() {
				fmt.Printf("Silent refresh: OK (access token exp %s)\n", exp.Format(time.RFC3339))
			} else {
				fmt.Println("Silent refresh: OK (no expiry in response)")
			}
		}
	}

	fmt.Println()
	fmt.Println("Tip: use -run=logs:200 to view recent authentication logs.")
	return nil
}

// keyringTestNonInteractive validates OS keyring accessibility by creating, reading, and deleting a test credential.
func keyringTestNonInteractive(cfg config.Config) error {
	logging.Info("Running keyring self-test diagnostics")

	// Basic environment context (no secrets)
	uname := strings.TrimSpace(os.Getenv("USERNAME"))
	udom := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	uprof := strings.TrimSpace(os.Getenv("USERPROFILE"))
	pid := os.Getpid()
	fmt.Println("Process/User Context:")
	fmt.Printf("  PID: %d\n", pid)
	if udom != "" || uname != "" {
		if udom != "" {
			fmt.Printf("  User: %s\\%s\n", udom, uname)
		} else {
			fmt.Printf("  User: %s\n", uname)
		}
	}
	if uprof != "" {
		fmt.Printf("  USERPROFILE: %s\n", uprof)
	}

	// Report presence of production refresh token (without secrets)
	authenticator := auth.NewAuthenticator(cfg.OAuth2)
	present, err := authenticator.StoredRefreshTokenPresent()
	if err != nil {
		logging.Error("Keyring presence check failed", "error", err.Error())
		fmt.Printf("Stored refresh token: error checking presence: %v\n", err)
	} else if !present {
		fmt.Println("Stored refresh token: not found")
	} else {
		fmt.Println("Stored refresh token: found")
	}

	// Self-test entry lifecycle
	const service = "bc-insights-tui"
	testKey := fmt.Sprintf("keyring-selftest-%d", pid)
	testValue := fmt.Sprintf("ok-%d", time.Now().Unix())

	fmt.Printf("\nSelf-test: writing test credential (%s/%s) ...\n", service, testKey)
	if werr := keyring.Set(service, testKey, testValue); werr != nil {
		logging.Error("Keyring Set failed", "service", service, "key", testKey, "error", werr.Error())
		fmt.Printf("  WRITE: FAIL (%v)\n", werr)
		fmt.Println("Hint: Check Windows Credential Manager availability and that this process runs under your regular user context.")
		return nil
	}
	fmt.Println("  WRITE: OK")

	got, rerr := keyring.Get(service, testKey)
	if rerr != nil {
		logging.Error("Keyring Get failed", "service", service, "key", testKey, "error", rerr.Error())
		fmt.Printf("  READ:  FAIL (%v)\n", rerr)
	} else if strings.TrimSpace(got) != testValue {
		logging.Warn("Keyring Get returned unexpected value", "service", service, "key", testKey)
		fmt.Println("  READ:  WARN (unexpected value)")
	} else {
		fmt.Println("  READ:  OK")
	}

	if derr := keyring.Delete(service, testKey); derr != nil {
		logging.Warn("Keyring Delete failed", "service", service, "key", testKey, "error", derr.Error())
		fmt.Printf("  DELETE: WARN (%v)\n", derr)
	} else {
		fmt.Println("  DELETE: OK")
	}

	fmt.Println()
	fmt.Println("Tip: If WRITE/READ intermittently fail across sessions, it indicates an OS credential store visibility or profile persistence issue.")
	return nil
}

// keyringInfoNonInteractive prints the effective keyring service/key used for the refresh token and any env overrides
func keyringInfoNonInteractive() error {
	svc, key := auth.KeyringEntryInfo()
	bsvc, bkey := auth.KeyringBackupEntryInfo()
	fmt.Println("Keyring entry (refresh token):")
	fmt.Printf("  Service: %s\n", svc)
	fmt.Printf("  Key:     %s\n", key)
	fmt.Println("Backup entry:")
	fmt.Printf("  Service: %s\n", bsvc)
	fmt.Printf("  Key:     %s\n", bkey)
	if v := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_SERVICE")); v != "" {
		fmt.Printf("Env BCINSIGHTS_KEYRING_SERVICE=%s\n", v)
	}
	if v := strings.TrimSpace(os.Getenv("BCINSIGHTS_KEYRING_NAMESPACE")); v != "" {
		fmt.Printf("Env BCINSIGHTS_KEYRING_NAMESPACE=%s\n", v)
	}
	return nil
}
