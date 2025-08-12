package tui

import (
	"strings"
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

const testKeyErrorCount = "errorCount"

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
		if h == testKeyErrorCount {
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

// Test sparse boosted key should not outrank dense neutral key after presence scaling adjustments.
func TestRanking_SparseBoostedDoesNotDominate(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	// 100 total rows: denseKey present in 90 with limited variability; sparseKey present in 5 rows but matches boosting patterns ("error" + "id")
	for i := 0; i < 90; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"denseKey": i % 10}))
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"sparseErrorId": i}))
	}
	// Add remaining 5 rows empty for completeness
	for i := 0; i < 5; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxDense, idxSparse := -1, -1
	for i, h := range headers {
		if h == "denseKey" {
			idxDense = i
		}
		if h == "sparseErrorId" {
			idxSparse = i
		}
	}
	if idxDense == -1 || idxSparse == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	if idxDense > idxSparse {
		t.Fatalf("expected denseKey before sparseErrorId after scaling; got %v", headers)
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

// Reuse header constants from ui_headers_test where possible
const (
	testHdrTimestamp = "timestamp"
	testHdrMessage   = "message"
	testKeyAlpha     = "alpha"
)

// Disabled ranking should still promote eventId as a primary when present.
func TestRanking_DisabledFlagFallback_EventIDPrimary(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankEnable = false
	cols := baseCols()
	rows := [][]interface{}{row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"eventId": 42, "z": 1, "a": 1})}
	headers := computeRankedHeaders(cols, rows, cfg)
	if len(headers) < 5 {
		t.Fatalf("expected headers include eventId: %v", headers)
	}
	if headers[0] != testHdrTimestamp || headers[1] != testHdrMessage || headers[2] != "eventId" {
		t.Fatalf("eventId not promoted in fallback: %v", headers[:4])
	}
}

// Disabled ranking should honor pinned ordering after primaries and eventId.
func TestRanking_DisabledFlagFallback_Pinned(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankEnable = false
	cfg.RankPinned = "beta,alpha"
	cols := baseCols()
	rows := [][]interface{}{row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"alpha": 1, "beta": 2, "gamma": 3})}
	headers := computeRankedHeaders(cols, rows, cfg)
	if len(headers) < 5 { // timestamp,message + beta,alpha,gamma
		t.Fatalf("unexpected headers: %v", headers)
	}
	if headers[2] != "beta" || headers[3] != testKeyAlpha {
		t.Fatalf("pinned order not respected in fallback: %v", headers)
	}
}

// Test pinned columns precedence: pinned keys should appear immediately after primaries in specified order regardless of scores.
func TestRanking_PinnedPrecedence(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankPinned = "gamma,alpha"
	cols := baseCols()
	rows := [][]interface{}{}
	// Build rows where beta would normally outrank others (e.g., keyword) but should be placed after pinned.
	for i := 0; i < 30; i++ {
		rows = append(rows, row("2025-01-01T00:00:00Z", "m", map[string]interface{}{"betaError": i, "alpha": i, "gamma": i}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	if len(headers) < 5 {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if headers[0] != "timestamp" || headers[1] != "message" {
		t.Fatalf("primaries incorrect: %v", headers[:2])
	}
	if headers[2] != "gamma" || headers[3] != "alpha" {
		t.Fatalf("pinned order not respected: %v", headers)
	}
}

// Test custom regex semicolon spec integration and invalid fragment handling.
func TestRanking_CustomRegexSemicolon(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankRegexSpec = "(?i)^foo=5;badfragment;(?i)end$=2" // 'badfragment' should be ignored
	cols := baseCols()
	rows := [][]interface{}{
		row("2025-01-01T00:00:00Z", "m", map[string]interface{}{"fooValue": 1, "alpha": 1}),
		row("2025-01-01T00:00:01Z", "m", map[string]interface{}{"fooValue": 2, "betaend": 1}),
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxFoo, idxAlpha := -1, -1
	for i, h := range headers {
		if h == "fooValue" {
			idxFoo = i
		}
		if h == "alpha" {
			idxAlpha = i
		}
	}
	if idxFoo == -1 || idxAlpha == -1 {
		t.Fatalf("expected keys missing: %v", headers)
	}
	if idxFoo > idxAlpha {
		t.Fatalf("expected fooValue boosted ahead of alpha: %v", headers)
	}
}

// Test custom regex JSON spec integration with partial invalid entries tolerated.
func TestRanking_CustomRegexJSON(t *testing.T) {
	cfg := config.NewConfig()
	// Provide JSON spec; invalid pattern should be skipped (simulate by unmatched bracket)
	cfg.RankRegexSpec = `{"(?i)^start":4,"(?i)missing[":3,"(?i)ok$":1}`
	cols := baseCols()
	rows := [][]interface{}{
		row("2025-01-01T00:00:00Z", "m", map[string]interface{}{"startKey": 1, "neutral": 1, "veryok": 1}),
		row("2025-01-01T00:00:01Z", "m", map[string]interface{}{"startKey": 2, "neutral": 2, "veryok": 2}),
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxStart, idxNeutral := -1, -1
	for i, h := range headers {
		if h == "startKey" {
			idxStart = i
		}
		if h == "neutral" {
			idxNeutral = i
		}
	}
	if idxStart == -1 || idxNeutral == -1 {
		t.Fatalf("expected keys missing: %v", headers)
	}
	if idxStart > idxNeutral {
		t.Fatalf("expected startKey boosted ahead of neutral: %v", headers)
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

// AL presence threshold boundary tests
// 1) Negative threshold clamps to 0: any presence qualifies for scaled boost.
// 2) Exact threshold: presence == threshold still gets boost.
// 3) Threshold >1 clamps to 1: only 100% presence gets boost (so almost-full presence should NOT get boost).
func TestRanking_ALPresenceThreshold_NegativeClampedZero(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankALMinPresence = -0.25 // should clamp to 0
	cols := baseCols()
	rows := [][]interface{}{}
	// 10 rows; alLow + lowKey each present in exactly 2 rows (20% presence)
	for i := 0; i < 10; i++ {
		dims := map[string]interface{}{}
		if i < 2 { // sparsely include both keys
			dims["alLow"] = i
			dims["lowKey"] = i
		}
		rows = append(rows, row("2025-01-01T00:00:00Z", "m", dims))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxAL, idxLow := -1, -1
	for i, h := range headers {
		if h == "alLow" {
			idxAL = i
		}
		if h == "lowKey" {
			idxLow = i
		}
	}
	if idxAL == -1 || idxLow == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	if idxAL > idxLow {
		t.Fatalf("expected alLow (AL boosted) before lowKey when threshold clamped to 0: %v", headers)
	}
}

func TestRanking_ALPresenceThreshold_Exact(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankALMinPresence = 0.5
	cols := baseCols()
	rows := [][]interface{}{}
	// 10 rows; presence 50% for both alHalf and halfKey
	for i := 0; i < 10; i++ {
		dims := map[string]interface{}{}
		if i%2 == 0 { // 5 rows
			dims["alHalf"] = i
			dims["halfKey"] = i
		}
		rows = append(rows, row("2025-01-01T00:00:00Z", "m", dims))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxAL, idxHalf := -1, -1
	for i, h := range headers {
		if h == "alHalf" {
			idxAL = i
		}
		if h == "halfKey" {
			idxHalf = i
		}
	}
	if idxAL == -1 || idxHalf == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	if idxAL > idxHalf {
		t.Fatalf("expected alHalf (met threshold) before halfKey: %v", headers)
	}
}

func TestRanking_ALPresenceThreshold_ClampAboveOne(t *testing.T) {
	cfg := config.NewConfig()
	cfg.RankALMinPresence = 2.0 // should clamp to 1.0
	cols := baseCols()
	rows := [][]interface{}{}
	// 20 rows; errorCount present in all rows (keyword boost), alAlmost present in 19/20 (presence < 1.0 so no AL boost applied)
	for i := 0; i < 20; i++ {
		dims := map[string]interface{}{"errorCount": i}
		if i != 5 { // miss one to keep presenceRate < 1
			dims["alAlmost"] = i
		}
		rows = append(rows, row("2025-01-01T00:00:00Z", "m", dims))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idxErr, idxAL := -1, -1
	for i, h := range headers {
		if h == "errorCount" {
			idxErr = i
		}
		if h == "alAlmost" {
			idxAL = i
		}
	}
	if idxErr == -1 || idxAL == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	if idxErr > idxAL {
		t.Fatalf("expected errorCount before alAlmost (no AL boost due to clamp to 1): %v", headers)
	}
}

// Test tie alphabetical ordering: ensure deterministic ordering when metrics equal.
const (
	testKeyAAA1 = "aaa1"
	testKeyAAA2 = "aaa2"
	testKeyBBB  = "bbb"
)

func TestRanking_TieAlphabetical(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{}
	// Revised tie alphabetical test using distinct lowercase keys to avoid de-duplication removal.
	for i := 0; i < 20; i++ {
		rows = append(rows, row(time.Now().Format(time.RFC3339), "m", map[string]interface{}{"bbb": 1, "aaa2": 1, "aaa1": 1}))
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	idx1, idx2, idxB := -1, -1, -1
	for i, h := range headers {
		if h == testKeyAAA1 {
			idx1 = i
		}
		if h == testKeyAAA2 {
			idx2 = i
		}
		if h == testKeyBBB {
			idxB = i
		}
	}
	if idx1 == -1 || idx2 == -1 || idxB == -1 {
		t.Fatalf("expected keys present: %v", headers)
	}
	// Lowercase alphabetical order should be: aaa1, aaa2, bbb
	if idx1 >= idx2 || idx2 >= idxB { // simplified De Morgan
		for i, h := range headers {
			t.Logf("%d: %s", i, h)
		}
		panic("alphabetical tie-break ordering invalid")
	}
}

// panicString used to induce panic via fmt.Sprint during ranking.
type panicString struct{}

func (p panicString) String() string { panic("panicString.String boom") }

func TestRanking_PanicRecovery(t *testing.T) {
	cfg := config.NewConfig()
	cols := baseCols()
	rows := [][]interface{}{{panicString{}, "m", map[string]interface{}{"a": 1, "eventId": 123}}}
	// Simulate expected fallback by disabling ranking (uses fallback builder now promoting eventId)
	disabled := cfg
	disabled.RankEnable = false
	expected := computeRankedHeaders(cols, rows, disabled)
	if len(expected) == 0 {
		t.Fatalf("expected baseline headers non-empty")
	}
	headers := computeRankedHeaders(cols, rows, cfg)
	if len(headers) == 0 {
		t.Fatalf("got empty headers: expected %v", expected)
	}
	if strings.Join(headers, ",") != strings.Join(expected, ",") {
		t.Fatalf("fallback mismatch. got %v want %v", headers, expected)
	}
}
