package appinsights

import (
	"os"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

// TestMain ensures this package's tests never touch the real OS keyring.
func TestMain(m *testing.M) {
	keyring.MockInit()                                     // in-memory provider
	_ = os.Setenv("BCINSIGHTS_KEYRING_NAMESPACE", "tests") // extra isolation
	code := m.Run()
	os.Exit(code)
}
