package telemetry

import (
	"encoding/json"
	"testing"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
)

func col(name string) appinsights.Column { return appinsights.Column{Name: name, Type: "string"} }

func TestBuildDetails_MapObject(t *testing.T) {
	cols := []appinsights.Column{col("timestamp"), col("message"), col("customDimensions")}
	row := []interface{}{"2025-01-01T00:00:00Z", "hello", map[string]interface{}{
		"a":    1,
		"b":    true,
		"c":    "str",
		"nest": map[string]interface{}{"x": 2, "y": []interface{}{1, 2}},
	}}
	ts, msg, fields := BuildDetails(cols, row)
	if ts == "" {
		t.Fatalf("expected timestamp")
	}
	if msg == "" {
		t.Fatalf("expected message present when message column exists")
	}
	// Ensure keys exist after flatten
	wantKeys := []string{"a", "b", "c", "nest.x", "nest.y[0]", "nest.y[1]"}
	got := map[string]bool{}
	for _, f := range fields {
		got[f.Key] = true
	}
	for _, k := range wantKeys {
		if !got[k] {
			t.Fatalf("missing key %s", k)
		}
	}
}

func TestBuildDetails_JSONString(t *testing.T) {
	cols := []appinsights.Column{col("timeGenerated"), col("customDimensions")}
	m := map[string]interface{}{"x": 1, "y": []interface{}{"a", 2}}
	b, _ := json.Marshal(m)
	row := []interface{}{"2025-02-02T00:00:00Z", string(b)}
	_, _, fields := BuildDetails(cols, row)
	keys := map[string]bool{}
	for _, f := range fields {
		keys[f.Key] = true
	}
	if !keys["x"] || !keys["y[0]"] || !keys["y[1]"] {
		t.Fatalf("expected flattened keys from JSON string")
	}
}

func TestBuildDetails_InvalidJSONString_ShowsRaw(t *testing.T) {
	cols := []appinsights.Column{col("timestamp"), col("customDimensions")}
	row := []interface{}{"t", "not-json"}
	_, _, fields := BuildDetails(cols, row)
	if len(fields) == 0 {
		t.Fatalf("expected fields including raw")
	}
	if fields[0].Key != "(parse_warning)" {
		t.Fatalf("expected parse warning at first field")
	}
	hasRaw := false
	for _, f := range fields {
		if f.Key == "raw" {
			hasRaw = true
		}
	}
	if !hasRaw {
		t.Fatalf("expected raw field present")
	}
}

func TestBuildDetails_NoCustomColumn(t *testing.T) {
	cols := []appinsights.Column{col("timestamp"), col("message")}
	row := []interface{}{"t", "m"}
	_, _, fields := BuildDetails(cols, row)
	if len(fields) != 0 {
		t.Fatalf("expected no custom fields when column missing")
	}
}
