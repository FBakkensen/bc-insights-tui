package main

import (
	"os"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/config"
	keyring "github.com/zalando/go-keyring"
)

// Ensure tests use in-memory keyring and isolated namespace
func TestMain(m *testing.M) {
	keyring.MockInit()
	_ = os.Setenv("BCINSIGHTS_KEYRING_NAMESPACE", "tests")
	os.Exit(m.Run())
}

// Test that keyringTestNonInteractive succeeds with the mock keyring available.
func TestKeyringSelfTestOK(t *testing.T) {
	cfg := config.Config{}
	if err := keyringTestNonInteractive(cfg); err != nil {
		t.Fatalf("keyring self-test failed: %v", err)
	}
}
