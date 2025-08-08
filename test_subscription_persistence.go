package main

import (
	"fmt"
	"log"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func main() {
	// Load current config
	cfg := config.LoadConfig()
	fmt.Printf("Current subscription ID: %s\n", cfg.SubscriptionID)

	// Test updating subscription ID
	newSubscriptionID := "bf273484-b813-4c92-8527-8fa577aec089" // First subscription from the list
	fmt.Printf("Setting subscription ID to: %s\n", newSubscriptionID)

	err := cfg.ValidateAndUpdateSetting("azure.subscriptionId", newSubscriptionID)
	if err != nil {
		log.Fatalf("Failed to update subscription: %v", err)
	}

	fmt.Println("Subscription updated successfully in memory")

	// Reload config to verify persistence
	reloadedCfg := config.LoadConfig()
	fmt.Printf("Reloaded subscription ID: %s\n", reloadedCfg.SubscriptionID)

	if reloadedCfg.SubscriptionID == newSubscriptionID {
		fmt.Println("✅ SUCCESS: Subscription persistence is working!")
	} else {
		fmt.Printf("❌ FAILURE: Expected %s, got %s\n", newSubscriptionID, reloadedCfg.SubscriptionID)
	}
}
