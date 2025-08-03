package main

import (
	"fmt"
	"os"

	"github.com/FBakkensen/bc-insights-tui/config"
)

func main() {
	// Test default config
	fmt.Println("Testing default config:")
	cfg := config.LoadConfig()
	fmt.Printf("Default LogFetchSize: %d\n", cfg.LogFetchSize)

	// Test with valid environment variable
	os.Setenv("LOG_FETCH_SIZE", "100")
	fmt.Println("\nTesting with LOG_FETCH_SIZE=100:")
	cfg = config.LoadConfig()
	fmt.Printf("LogFetchSize: %d\n", cfg.LogFetchSize)

	// Test with invalid environment variable
	os.Setenv("LOG_FETCH_SIZE", "invalid")
	fmt.Println("\nTesting with LOG_FETCH_SIZE=invalid:")
	cfg = config.LoadConfig()
	fmt.Printf("LogFetchSize (should fallback to default): %d\n", cfg.LogFetchSize)

	// Test with zero value
	os.Setenv("LOG_FETCH_SIZE", "0")
	fmt.Println("\nTesting with LOG_FETCH_SIZE=0:")
	cfg = config.LoadConfig()
	fmt.Printf("LogFetchSize (should fallback to default): %d\n", cfg.LogFetchSize)

	// Clean up
	os.Unsetenv("LOG_FETCH_SIZE")
}
