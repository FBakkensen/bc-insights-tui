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

// BuildDetails extracts a timestamp and normalized customDimensions fields from a KQL row.
// nolint:gocyclo // The branching here is intentional for robust type handling and remains contained.
// - columns: the table columns (case-insensitive lookup)
// - row:     the row values aligned with columns
// Returns the timestamp string (empty if not present), message (if present), and an ordered slice of custom fields.
func BuildDetails(columns []appinsights.Column, row []interface{}) (string, string, []DetailField) {
	// Locate relevant columns by name (case-insensitive)
	idxTimestamp := findColumnIndex(columns, "timestamp")
	if idxTimestamp < 0 {
		idxTimestamp = findColumnIndex(columns, "timeGenerated")
	}
	idxCustom := findColumnIndex(columns, "customDimensions")
	idxMessage := findColumnIndex(columns, "message")

	// Extract timestamp string if present
	ts := ""
	if idxTimestamp >= 0 && idxTimestamp < len(row) && row[idxTimestamp] != nil {
		ts = fmt.Sprint(row[idxTimestamp])
	}
	// Extract message if present
	msg := ""
	if idxMessage >= 0 && idxMessage < len(row) && row[idxMessage] != nil {
		msg = fmt.Sprint(row[idxMessage])
	}

	// Extract and normalize customDimensions
	fields := make([]DetailField, 0)
	if idxCustom < 0 || idxCustom >= len(row) || row[idxCustom] == nil {
		// No customDimensions
		return ts, msg, fields
	}

	raw := row[idxCustom]
	// Accept map/object, JSON string, or array. Otherwise stringify.
	var (
		root      interface{}
		parseWarn bool
		flattened = make(map[string]string)
		maxDepth  = 2
	)

	switch v := raw.(type) {
	case map[string]interface{}:
		root = v
	case []interface{}:
		root = v
	case string:
		// Try to parse JSON
		var tmp interface{}
		if err := json.Unmarshal([]byte(v), &tmp); err != nil {
			// Not JSON: show raw string with parse warning
			parseWarn = true
			flattened["raw"] = v
		} else {
			root = tmp
		}
	default:
		// Unsupported type: stringify under raw
		parseWarn = true
		flattened["raw"] = fmt.Sprint(v)
	}

	if root != nil {
		// Flatten up to depth=2 using dot/bracket notation
		flattenInto(flattened, root, "", 0, maxDepth)
	}

	// Convert map to sorted []DetailField
	keys := make([]string, 0, len(flattened))
	for k := range flattened {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		// case-insensitive sort
		li, lj := strings.ToLower(keys[i]), strings.ToLower(keys[j])
		if li == lj {
			return keys[i] < keys[j]
		}
		return li < lj
	})

	// Optionally add a parse warning field at the very top
	if parseWarn {
		fields = append(fields, DetailField{Key: "(parse_warning)", Value: "customDimensions is not valid JSON; showing raw string", Group: GroupCustom, Priority: 0})
	}

	for _, k := range keys {
		fields = append(fields, DetailField{Key: k, Value: flattened[k], Group: GroupCustom, Priority: 10})
	}

	return ts, msg, fields
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
		// Represent null as empty string
		out[nonEmpty(prefix, "")] = ""
		return
	}
	// If we've exceeded maxDepth and v is compound, stringify
	if depth > maxDepth {
		out[nonEmpty(prefix, "")] = jsonCompact(v)
		return
	}
	switch vv := v.(type) {
	case map[string]interface{}:
		if len(vv) == 0 {
			out[nonEmpty(prefix, "")] = "{}"
			return
		}
		// Sort keys for deterministic output
		ks := make([]string, 0, len(vv))
		for k := range vv {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
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
		// Fallback: stringify via JSON if possible, else fmt
		s := jsonCompact(v)
		if s == "" || s == "null" || strings.HasPrefix(s, "<") { // crude detection
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
