package telemetry

import (
	"fmt"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
)

// TestFlattenEntryCap ensures we don't explode on very large maps.
func TestFlattenEntryCap(t *testing.T) {
	cols := []appinsights.Column{{Name: "customDimensions", Type: "dynamic"}}
	big := make(map[string]interface{})
	for i := 0; i < 250; i++ { // exceed cap intentionally
		big[fmt.Sprintf("k%03d", i)] = i
	}
	row := []interface{}{big}
	_, _, fields := BuildDetails(cols, row)
	if len(fields) > maxFlattenEntries { // cap enforcement
		t.Fatalf("expected <= %d fields, got %d", maxFlattenEntries, len(fields))
	}
}
