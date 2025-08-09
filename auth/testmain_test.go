package auth

import (
	"os"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

// TestMain sets an isolated keyring namespace for tests so they do not
// modify or delete real user credentials stored by the application.
func TestMain(m *testing.M) {
	// Use in-memory keyring provider for all tests in this package
	keyring.MockInit()
	_ = os.Setenv("BCINSIGHTS_KEYRING_NAMESPACE", "tests")
	code := m.Run()
	os.Exit(code)
}
