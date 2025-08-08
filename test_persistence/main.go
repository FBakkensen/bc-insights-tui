package main

import (
	"fmt"
	"log"
	"os"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func main() {
	fmt.Printf("=== Subscription Persistence Test ===\n\n")

	// 1. Load current config
	cfg := config.LoadConfig()
	fmt.Printf("1. Current subscription ID: '%s'\n", cfg.SubscriptionID)

	// 2. Test updating subscription ID to a different one
	newSubscriptionID := "2d1c1604-7ad7-4100-9500-97a64f7d3ee3" // Second subscription from the list
	fmt.Printf("2. Setting subscription ID to: '%s'\n", newSubscriptionID)

	err := cfg.ValidateAndUpdateSetting("azure.subscriptionId", newSubscriptionID)
	if err != nil {
		log.Fatalf("Failed to update subscription: %v", err)
	}

	fmt.Printf("3. Subscription updated successfully in memory: '%s'\n", cfg.SubscriptionID)

	// 3. Reload config to verify persistence
	reloadedCfg := config.LoadConfig()
	fmt.Printf("4. Reloaded subscription ID: '%s'\n", reloadedCfg.SubscriptionID)

	if reloadedCfg.SubscriptionID == newSubscriptionID {
		fmt.Println("✅ SUCCESS: Subscription persistence is working!")
	} else {
		fmt.Printf("❌ FAILURE: Expected '%s', got '%s'\n", newSubscriptionID, reloadedCfg.SubscriptionID)
		return
	}

	// 4. Verify the file was actually updated
	configDir, _ := os.UserConfigDir()
	configPath := fmt.Sprintf("%s/bc-insights-tui/config.json", configDir)
	if data, err := os.ReadFile(configPath); err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
	} else {
		fmt.Printf("5. Config file contents:\n%s\n", string(data))
	}
}
