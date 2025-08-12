package debugdump

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAIRawFullRotating_PrunesOld(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "appinsights-raw.yaml")
	cap := AIRawCapture{Version: 1}

	// Write 7 captures with keepN=5
	for i := 0; i < 7; i++ {
		if err := WriteAIRawFullRotating(base, 5, cap); err != nil {
			t.Fatalf("rotating write %d: %v", i, err)
		}
	}
	// Count rotated files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	cnt := 0
	for _, e := range entries {
		name := e.Name()
		if name == "appinsights-raw.yaml" {
			continue // main file not created by rotating
		}
		if filepath.Ext(name) == ".yaml" && len(name) > len("appinsights-raw-"+".yaml") && name[:len("appinsights-raw-")] == "appinsights-raw-" {
			cnt++
		}
	}
	if cnt != 5 {
		t.Fatalf("expected 5 rotated files, got %d (entries=%v)", cnt, entries)
	}
}
