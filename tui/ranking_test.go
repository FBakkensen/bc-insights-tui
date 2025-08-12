package tui

import (
	"testing"
	"time"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/config"
)

// helper to build rows with customDimensions map
func row(ts, msg string, dims map[string]interface{}) []interface{} {
	return []interface{}{ts, msg, dims}
}

func baseCols() []appinsights.Column {
	return []appinsights.Column{{Name: "timestamp"}, {Name: "message"}, {Name: "customDimensions"}}
}

// Test presence dominance: key present in most rows outranks sparse key absent of keywords.
func TestRanking_PresenceDominance(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	// 90% rows have aDense, 10% have aSparse
	for i := 0; i < 90; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"aDense": i}))
	}
	for i := 0; i < 10; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"aSparse": i}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxDense, idxSparse := -1, -1
	for i, h := range headers {
		if h == "aDense" {
			idxDense = i
		}
		if h == "aSparse" {
			idxSparse = i
		}
	}
	if idxDense == -1 || idxSparse == -1 {
		t.Fatalf("expected both keys present: %v", headers)
	}
	if idxDense > idxSparse {
		t.Fatalf("expected aDense before aSparse: %v", headers)
	}
}

// Test keyword influence: errorCount should outrank neutralKey given similar presence.
func TestRanking_KeywordInfluence(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	for i := 0; i < 50; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"errorCount": i, "neutralKey": i}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxErr, idxNeutral := -1, -1
	for i, h := range headers {
		if h == "errorCount" {
			idxErr = i
		}
		if h == "neutralKey" {
			idxNeutral = i
		}
	}
	if idxErr == -1 || idxNeutral == -1 {
		t.Fatalf("missing keys in headers: %v", headers)
	}
	if idxErr > idxNeutral {
		t.Fatalf("expected errorCount before neutralKey due to keyword boost: %v", headers)
	}
}

// Test length penalty: longText should rank below shortID when both present equally.
func TestRanking_LengthPenalty(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	long := ""
	for i := 0; i < 300; i++ {
		long += "x"
	}
	for i := 0; i < 40; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"shortId": i, "longText": long}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxShort, idxLong := -1, -1
	for i, h := range headers {
		if h == "shortId" {
			idxShort = i
		}
		if h == "longText" {
			idxLong = i
		}
	}
	if idxShort == -1 || idxLong == -1 {
		t.Fatalf("missing keys: %v", headers)
	}
	if idxShort > idxLong {
		t.Fatalf("expected shortId before longText: %v", headers)
	}
}

// Test variability boost: key with multiple distinct values outranks constantKey.
func TestRanking_VariabilityBoost(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	for i := 0; i < 60; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"varyKey": i % 10, "constantKey": 1}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxVary, idxConst := -1, -1
	for i, h := range headers {
		if h == "varyKey" {
			idxVary = i
		}
		if h == "constantKey" {
			idxConst = i
		}
	}
	if idxVary == -1 || idxConst == -1 {
		t.Fatalf("missing keys: %v", headers)
	}
	if idxVary > idxConst {
		t.Fatalf("expected varyKey before constantKey: %v", headers)
	}
}

// Test AL prefix scaling: high presence AL should outrank similar non-keyword; sparse AL should not.
func TestRanking_ALPrefixScaling(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	for i := 0; i < 50; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"alObjectId": i, "refKey": i}))
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"alRareField": i}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxAL, idxRef, idxRare := -1, -1, -1
	for i, h := range headers {
		if h == "alObjectId" {
			idxAL = i
		}
		if h == "refKey" {
			idxRef = i
		}
		if h == "alRareField" {
			idxRare = i
		}
	}
	if idxAL == -1 || idxRef == -1 || idxRare == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	if idxAL > idxRef {
		t.Fatalf("expected alObjectId before refKey due to presence-weighted boost: %v", headers)
	}
	if idxRare < idxRef {
		t.Fatalf("expected alRareField after refKey: %v", headers)
	}
}

// Determinism test: repeated calls yield same ordering
func TestRanking_Determinism(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	for i := 0; i < 25; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"kA": i % 3, "kB": i % 5}))
	}
	h1 := computeRankedHeaders(cols, rows, cfg)
	h2 := computeRankedHeaders(cols, rows, cfg)
	if len(h1) != len(h2) {
		t.Fatalf("header length mismatch")
	}
	for i := range h1 {
		if h1[i] != h2[i] {
			t.Fatalf("ordering not deterministic: %v vs %v", h1, h2)
		}
	}
}

// Feature flag disabled -> fallback alphabetical (k1 before k2 by alpha)
func TestRanking_DisabledFlagFallback(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankEnable = false
	cols := baseCols()
	rows := [][]interface{}{row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"kB": 1, "kA": 1})}
	headers := computeRankedHeaders(cols, rows, cfg)
	if len(headers) < 4 {
		t.Fatalf("expected at least 4 headers: %v", headers)
	}
	if headers[2] != "kA" || headers[3] != "kB" {
		t.Fatalf("expected alphabetical order kA,kB after primaries; got %v", headers)
	}
}

// Performance smoke: ensure ranking executes quickly on typical dataset (not strict micro benchmark)
func TestRanking_PerformanceTypical(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	// Reduced dataset size to keep test stable under race detector while still exercising logic.
	rows := make([][]interface{}, 0, 120)
	for i := 0; i < 120; i++ {
		dims := map[string]interface{}{}
		for k := 0; k < 60; k++ { // fewer distinct keys per row
			dims["k"+string(rune('A'+(k%26)))+string(rune('a'+(k%26)))+string(rune('0'+(k%10)))] = i * k
		}
		dims["errorCount"] = i
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", dims))
	}
	start := time.Now()
	_ = computeRankedHeaders(cols, rows, cfg)
	dur := time.Since(start)
	// Fixed generous threshold chosen based on empirical race runs (<130ms after reduction) with headroom.
	const maxMs = 400
	if dur > maxMs*time.Millisecond {
		t.Fatalf("ranking too slow: %v (threshold %dms)", dur, maxMs)
	}
}
