package telemetry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
)

// Group represents a display group for detail fields.
type Group int

const (
	GroupStandard Group = iota
	GroupCustom
)

// DetailField represents a normalized key/value for display.
type DetailField struct {
	Key      string
	Value    string
	Group    Group
	Priority int // lower renders first
}

// Safety cap to avoid pathological explosion when flattening extremely large or deeply nested customDimensions.
// This prevents UI lockups if telemetry includes huge blobs. Reasonable heuristic; adjust as needed.
const maxFlattenEntries = 200

// BuildDetails extracts a timestamp and normalized customDimensions fields from a KQL row.
// nolint:gocyclo // The branching here is intentional for robust type handling and remains contained.
// - columns: the table columns (case-insensitive lookup)
// - row:     the row values aligned with columns
// Returns the timestamp string (empty if not present), message (if present), and an ordered slice of custom fields.
func BuildDetails(columns []appinsights.Column, row []interface{}) (string, string, []DetailField) {
	tsIdx, msgIdx, customIdx := findCoreIndices(columns)
	ts, msg := extractTimestampAndMessage(row, tsIdx, msgIdx)
	// No custom dimensions column or nil value -> return early
	if customIdx < 0 || customIdx >= len(row) || row[customIdx] == nil {
		return ts, msg, []DetailField{}
	}
	flattened, parseWarn := parseAndFlattenCustom(row[customIdx], 2)
	fields := buildDetailFields(flattened, parseWarn)
	return ts, msg, fields
}

// findCoreIndices returns indices for timestamp/timeGenerated (preferring timestamp), message, and customDimensions.
func findCoreIndices(columns []appinsights.Column) (tsIdx, msgIdx, customIdx int) {
	tsIdx = findColumnIndex(columns, "timestamp")
	if tsIdx < 0 {
		tsIdx = findColumnIndex(columns, "timeGenerated")
	}
	msgIdx = findColumnIndex(columns, "message")
	customIdx = findColumnIndex(columns, "customDimensions")
	return
}

// extractTimestampAndMessage pulls out timestamp and message strings given their indices.
func extractTimestampAndMessage(row []interface{}, tsIdx, msgIdx int) (string, string) {
	ts := ""
	if tsIdx >= 0 && tsIdx < len(row) && row[tsIdx] != nil {
		ts = fmt.Sprint(row[tsIdx])
	}
	msg := ""
	if msgIdx >= 0 && msgIdx < len(row) && row[msgIdx] != nil {
		msg = fmt.Sprint(row[msgIdx])
	}
	return ts, msg
}

// parseAndFlattenCustom normalizes the customDimensions raw value into a flattened map.
// It returns the flattened key/value pairs and whether a parse warning should be emitted.
func parseAndFlattenCustom(raw interface{}, maxDepth int) (map[string]string, bool) {
	flattened := make(map[string]string)
	if raw == nil {
		return flattened, false
	}
	var root interface{}
	parseWarn := false
	switch v := raw.(type) {
	case map[string]interface{}:
		root = v
	case []interface{}:
		root = v
	case string:
		// Attempt to parse JSON, fall back to raw field
		var tmp interface{}
		if err := json.Unmarshal([]byte(v), &tmp); err != nil {
			parseWarn = true
			flattened["raw"] = v
		} else {
			root = tmp
		}
	default:
		parseWarn = true
		flattened["raw"] = fmt.Sprint(v)
	}
	if root != nil {
		flattenInto(flattened, root, "", 0, maxDepth)
	}
	return flattened, parseWarn
}

// buildDetailFields converts a flattened map + parse warning into an ordered []DetailField.
func buildDetailFields(flattened map[string]string, parseWarn bool) []DetailField {
	if len(flattened) == 0 && !parseWarn {
		return []DetailField{}
	}
	keys := make([]string, 0, len(flattened))
	for k := range flattened {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		li, lj := strings.ToLower(keys[i]), strings.ToLower(keys[j])
		if li == lj {
			return keys[i] < keys[j]
		}
		return li < lj
	})
	fields := make([]DetailField, 0, len(keys)+1)
	if parseWarn {
		fields = append(fields, DetailField{Key: "(parse_warning)", Value: "customDimensions is not valid JSON; showing raw string", Group: GroupCustom, Priority: 0})
	}
	for _, k := range keys {
		fields = append(fields, DetailField{Key: k, Value: flattened[k], Group: GroupCustom, Priority: 10})
	}
	return fields
}

func findColumnIndex(cols []appinsights.Column, name string) int {
	for i, c := range cols {
		if strings.EqualFold(strings.TrimSpace(c.Name), name) {
			return i
		}
	}
	return -1
}

// flattenInto flattens v into out with keys composed from prefix using dot/bracket notation.
// Depth is zero-based; when encountering a compound type deeper than maxDepth, stringify as minified JSON.
// nolint:gocyclo // Depth/type handling benefits from explicit branches; kept readable and tested.
func flattenInto(out map[string]string, v interface{}, prefix string, depth, maxDepth int) {
	if v == nil {
		out[nonEmpty(prefix, "")] = ""
		return
	}
	// Depth guard: once exceeded, compact represent and stop.
	if depth > maxDepth {
		out[nonEmpty(prefix, "")] = jsonCompact(v)
		return
	}
	// Entry cap: avoid runaway recursion for massive structures.
	if len(out) >= maxFlattenEntries {
		return
	}
	switch vv := v.(type) {
	case map[string]interface{}:
		if len(vv) == 0 {
			out[nonEmpty(prefix, "")] = "{}"
			return
		}
		ks := make([]string, 0, len(vv))
		for k := range vv {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			if len(out) >= maxFlattenEntries { // re-check inside loop
				return
			}
			childKey := k
			if prefix != "" {
				childKey = prefix + "." + k
			}
			flattenInto(out, vv[k], childKey, depth+1, maxDepth)
		}
	case []interface{}:
		if len(vv) == 0 {
			out[nonEmpty(prefix, "")] = "[]"
			return
		}
		for i, elem := range vv {
			if len(out) >= maxFlattenEntries {
				return
			}
			idxKey := fmt.Sprintf("%s[%d]", prefix, i)
			if prefix == "" {
				idxKey = fmt.Sprintf("[%d]", i)
			}
			flattenInto(out, elem, idxKey, depth+1, maxDepth)
		}
	case string:
		out[nonEmpty(prefix, "")] = vv
	case bool:
		if vv {
			out[nonEmpty(prefix, "")] = "true"
		} else {
			out[nonEmpty(prefix, "")] = "false"
		}
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		out[nonEmpty(prefix, "")] = fmt.Sprint(vv)
	default:
		s := jsonCompact(v)
		if s == "" || s == "null" || strings.HasPrefix(s, "<") {
			s = fmt.Sprint(v)
		}
		out[nonEmpty(prefix, "")] = s
	}
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// jsonCompact returns a compact JSON representation of v, or fmt.Sprint on failure.
func jsonCompact(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
