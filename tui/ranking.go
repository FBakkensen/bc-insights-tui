package tui

// Dynamic column ranking (Step 10)
// Implements ranking of customDimensions keys using presence, variability,
// value length, type bias, and keyword boosts. Produces an ordered header slice
// starting with primaries (timestamp, message, optional eventId) followed by ranked keys.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FBakkensen/bc-insights-tui/appinsights"
	"github.com/FBakkensen/bc-insights-tui/config"
	"github.com/FBakkensen/bc-insights-tui/internal/telemetry"
	"github.com/FBakkensen/bc-insights-tui/logging"
)

// rankRule defines a regex boost rule.
type rankRule struct {
	re               *regexp.Regexp
	boost            float64
	presenceWeighted bool
}

// computeRankedHeaders is the public entry used by the model to derive header ordering.
func computeRankedHeaders(columns []appinsights.Column, rows [][]interface{}, cfg config.Config) []string {
	if !cfg.RankEnable {
		return buildHeadersForAllRowsFallback(columns, rows, cfg)
	}
	start := time.Now()
	headers := []string{}
	defer func() {
		if r := recover(); r != nil {
			logging.Error("Ranking panic; falling back", "error", fmt.Sprintf("%v", r))
			headers = buildHeadersForAllRowsFallback(columns, rows, cfg)
		} else {
			logging.Debug("Ranking executed", "took_ms", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
		}
	}()
	headers = rankHeaders(columns, rows, cfg)
	return headers
}

// rankHeaders orchestrates sampling, stat accumulation, scoring and ordering.
func rankHeaders(columns []appinsights.Column, rows [][]interface{}, cfg config.Config) []string {
	if len(rows) == 0 {
		return []string{"timestamp", "message"}
	}
	keys := discoverCanonicalKeys(columns, rows)
	if len(keys) == 0 {
		return []string{"timestamp", "message"}
	}
	sample, sampleSize := selectSample(rows, cfg.RankSampleSize)
	distinctCap, lenCap := normalizeCaps(cfg.RankDistinctCap, cfg.RankLenCap)
	rules, alMinPresence := buildRules(cfg)
	pinnedMap := buildPinnedMap(cfg.RankPinned)
	stats := initKeyStats(keys)
	accumulateStats(stats, keys, columns, sample, distinctCap)
	scoreStats(stats, rules, cfg, distinctCap, lenCap, alMinPresence, pinnedMap)
	orderedStats := sortStats(stats, pinnedMap)
	headers := assembleHeaders(orderedStats)
	logDiagnostics(orderedStats, sampleSize, len(keys))
	return headers
}

func computeBaseMetrics(s *keyStats, distinctCap, lenCap int) {
	s.presenceRate = float64(s.nonEmpty) / float64(s.occurrences)
	distinctCount := len(s.distinctValues)
	s.variability = float64(distinctCount) / float64(distinctCap)
	if s.variability > 1 {
		s.variability = 1
	}
	if s.nonEmpty > 0 {
		s.avgLen = float64(s.totalLen) / float64(s.nonEmpty)
	}
	s.lenRatio = s.avgLen / float64(lenCap)
	if s.lenRatio > 1 {
		s.lenRatio = 1
	}
}

func applyTypeBiases(s *keyStats) {
	distinctCount := len(s.distinctValues)
	if s.booleanLike {
		s.typeBias += 0.2
	}
	if distinctCount > 0 && distinctCount <= 5 && s.avgLen <= 12 {
		s.typeBias += 0.2
	}
	if s.avgLen >= 120 {
		s.typeBias -= 0.2
	}
}

func applyRuleBoosts(s *keyStats, rules []rankRule, alMinPresence float64) {
	for _, rule := range rules {
		if !rule.re.MatchString(s.key) {
			continue
		}
		contrib := rule.boost
		if rule.presenceWeighted { // AL prefix style: already presence gated
			if s.presenceRate >= alMinPresence {
				contrib *= s.presenceRate
			} else {
				contrib = 0
			}
		} else {
			// Mitigation for empty early columns: scale all other keyword boosts by presence rate
			// so extremely sparse keys cannot dominate solely via keyword/rule matches.
			contrib *= s.presenceRate
		}
		if contrib != 0 {
			s.keywordBoost += contrib
			s.keywordMatches++
		}
	}
}

// (removed duplicated legacy block)

type keyStats struct {
	key            string
	occurrences    int
	nonEmpty       int
	distinctValues map[string]struct{}
	totalLen       int
	booleanLike    bool
	presenceRate   float64
	variability    float64
	avgLen         float64
	lenRatio       float64 // avgLen/lenCap (capped 1); weighted (likely negative) to penalize verbosity
	typeBias       float64
	keywordBoost   float64
	keywordMatches int
	score          float64
	pinned         bool
}

// (duplicate computeRankedHeaders removed; single definition kept at top of file)

// buildHeadersForAllRowsFallback builds a deterministic header ordering when ranking is disabled
// or a panic occurs. It preserves key discovery (case-insensitive de-dup, first-seen casing) and
// applies limited priority semantics:
// 1. Primaries: timestamp, message, optional eventId (if present in custom keys)
// 2. Pinned keys (cfg.RankPinned order) excluding primaries
// 3. Remaining keys case-insensitive alphabetical.
func buildHeadersForAllRowsFallback(columns []appinsights.Column, rows [][]interface{}, cfg config.Config) []string {
	if len(rows) == 0 {
		return []string{"timestamp", "message"}
	}
	keys := discoverCanonicalKeys(columns, rows)
	lowerIndex := make(map[string]int, len(keys))
	for i, k := range keys {
		lowerIndex[strings.ToLower(k)] = i
	}
	primaries := []string{"timestamp", "message"}
	eventLower := "eventid"
	if _, ok := lowerIndex[eventLower]; ok {
		primaries = append(primaries, "eventId") // canonical casing
		delete(lowerIndex, eventLower)
	}
	// Build set of remaining keys excluding eventId if promoted
	remaining := make([]string, 0, len(lowerIndex))
	for _, k := range keys {
		lk := strings.ToLower(k)
		if lk == eventLower {
			continue
		}
		remaining = append(remaining, k)
	}
	// Pinned ordering
	pinnedOrder := parsePinnedList(cfg.RankPinned)
	pinnedOut := make([]string, 0, len(pinnedOrder))
	used := map[string]struct{}{}
	for _, p := range pinnedOrder {
		lp := strings.ToLower(p)
		if lp == eventLower { // already a primary
			continue
		}
		for _, k := range remaining {
			if strings.ToLower(k) == lp {
				pinnedOut = append(pinnedOut, k)
				used[lp] = struct{}{}
				break
			}
		}
	}
	// Collect non-pinned
	nonPinned := make([]string, 0, len(remaining))
	for _, k := range remaining {
		lk := strings.ToLower(k)
		if _, ok := used[lk]; ok {
			continue
		}
		nonPinned = append(nonPinned, k)
	}
	sort.Slice(nonPinned, func(i, j int) bool {
		li, lj := strings.ToLower(nonPinned[i]), strings.ToLower(nonPinned[j])
		if li == lj {
			return nonPinned[i] < nonPinned[j]
		}
		return li < lj
	})
	return append(append(primaries, pinnedOut...), nonPinned...)
}

// (duplicate rankHeaders and helpers removed)

// Helper segmentation below keeps rankHeaders readable and testable
func selectSample(rows [][]interface{}, desired int) ([][]interface{}, int) {
	if desired <= 0 {
		desired = 200
	}
	if desired > len(rows) {
		desired = len(rows)
	}
	return rows[:desired], desired
}

func normalizeCaps(distinctCap, lenCap int) (int, int) {
	if distinctCap <= 0 {
		distinctCap = 50
	}
	if lenCap <= 0 {
		lenCap = 200
	}
	return distinctCap, lenCap
}

func buildRules(cfg config.Config) ([]rankRule, float64) {
	rules := defaultRankRules()
	if spec := strings.TrimSpace(cfg.RankRegexSpec); spec != "" {
		if custom, err := parseRankRegexSpec(spec); err == nil {
			rules = append(rules, custom...)
		} else {
			logging.Error("Failed to parse custom rank regex; using defaults", "error", err.Error())
		}
	}
	alBoost := cfg.RankALPrefixBoost
	if alBoost <= 0 {
		alBoost = 3.0
	}
	alMinPresence := cfg.RankALMinPresence
	switch {
	case alMinPresence < 0:
		alMinPresence = 0
	case alMinPresence > 1:
		alMinPresence = 1
	}
	haveAL := false
	for _, r := range rules {
		if r.re.String() == "(?i)^al.*" {
			haveAL = true
			break
		}
	}
	if !haveAL {
		if re, err := regexp.Compile("(?i)^al.*"); err == nil {
			rules = append(rules, rankRule{re: re, boost: alBoost, presenceWeighted: true})
		}
	}
	return rules, alMinPresence
}

func buildPinnedMap(pinned string) map[string]int {
	pinnedList := parsePinnedList(pinned)
	m := make(map[string]int, len(pinnedList))
	for i, p := range pinnedList {
		m[strings.ToLower(p)] = i
	}
	return m
}

func initKeyStats(keys []string) map[string]*keyStats {
	stats := make(map[string]*keyStats, len(keys))
	for _, k := range keys {
		stats[k] = &keyStats{key: k, distinctValues: make(map[string]struct{})}
	}
	return stats
}

// accumulateStats ingests sample rows and fills per-key counters.
func accumulateStats(stats map[string]*keyStats, keys []string, columns []appinsights.Column, sample [][]interface{}, distinctCap int) {
	// Precompute lower-case key list once
	lowerKeys := make([]string, len(keys))
	for i, k := range keys {
		lowerKeys[i] = strings.ToLower(k)
	}
	// Scratch map reused per row; clearing entries cheaper than realloc for typical widths.
	scratch := make(map[string]string, 64)
	for _, r := range sample {
		// Clear scratch map in-place. If it has grown excessively (heuristic >4096 entries) recreate to cap memory.
		if len(scratch) > 4096 {
			scratch = make(map[string]string, 64)
		} else {
			for k := range scratch {
				delete(scratch, k)
			}
		}
		_, _, fields := telemetry.BuildDetails(columns, r)
		for _, f := range fields {
			k := strings.ToLower(strings.TrimSpace(f.Key))
			if k == "" {
				continue
			}
			// last value wins; that's acceptable for stats purposes
			scratch[k] = f.Value
		}
		for i, k := range lowerKeys {
			s := stats[keys[i]]
			v, ok := scratch[k]
			s.occurrences++
			if !ok || len(v) == 0 {
				continue
			}
			s.nonEmpty++
			if len(s.distinctValues) < distinctCap {
				if _, exists := s.distinctValues[v]; !exists {
					s.distinctValues[v] = struct{}{}
				}
			}
			s.totalLen += len(v)
			if !s.booleanLike {
				lv := v
				if lv == "true" || lv == "false" {
					s.booleanLike = true
				}
			}
		}
	}
}

// scoreStats converts raw counters into a final score per key.
// score = weightPresence*presenceRate + weightVariability*variability + keywordBoost + weightLenPenalty*lenRatio + weightType*typeBias
// where lenRatio = avgLen/lenCap (capped 1).
// Side effects: sets s.score and s.pinned.
func scoreStats(stats map[string]*keyStats, rules []rankRule, cfg config.Config, distinctCap, lenCap int, alMinPresence float64, pinnedMap map[string]int) {
	for _, s := range stats {
		if s.occurrences == 0 {
			continue
		}
		computeBaseMetrics(s, distinctCap, lenCap)
		applyTypeBiases(s)
		applyRuleBoosts(s, rules, alMinPresence)
		// Scale variability by presence rate so ultra-sparse unique keys don't overshadow dense informative ones.
		varComponent := cfg.RankWeightVariability * (s.variability * s.presenceRate)
		s.score = cfg.RankWeightPresence*s.presenceRate + varComponent + s.keywordBoost + cfg.RankWeightLenPenalty*s.lenRatio + cfg.RankWeightType*s.typeBias
		if _, ok := pinnedMap[strings.ToLower(s.key)]; ok {
			s.pinned = true
		}
	}
}

func sortStats(stats map[string]*keyStats, pinnedMap map[string]int) []*keyStats {
	arr := make([]*keyStats, 0, len(stats))
	for _, s := range stats {
		arr = append(arr, s)
	}
	sort.SliceStable(arr, func(i, j int) bool {
		a, b := arr[i], arr[j]
		ai, bj := strings.ToLower(a.key), strings.ToLower(b.key)
		aiPinnedIdx, aPinned := pinnedMap[ai]
		bjPinnedIdx, bPinned := pinnedMap[bj]
		switch {
		case aPinned != bPinned:
			return aPinned
		case aPinned && bPinned && aiPinnedIdx != bjPinnedIdx:
			return aiPinnedIdx < bjPinnedIdx
		case a.score != b.score:
			return a.score > b.score
		case a.keywordMatches != b.keywordMatches:
			return a.keywordMatches > b.keywordMatches
		case a.presenceRate != b.presenceRate:
			return a.presenceRate > b.presenceRate
		case a.variability != b.variability:
			return a.variability > b.variability
		case a.avgLen != b.avgLen:
			return a.avgLen < b.avgLen
		case ai != bj: // final deterministic case-insensitive alphabetical ordering
			return ai < bj
		default:
			return false
		}
	})
	return arr
}

func assembleHeaders(arr []*keyStats) []string {
	orderedKeys := make([]string, 0, len(arr))
	hasEventID := false
	for _, s := range arr {
		if strings.EqualFold(s.key, "eventId") { // treat any casing as primary candidate
			hasEventID = true
			continue
		}
		orderedKeys = append(orderedKeys, s.key)
	}
	primaries := []string{"timestamp", "message"}
	if hasEventID {
		// Preserve canonical casing 'eventId'
		primaries = append(primaries, "eventId")
	}
	return append(primaries, orderedKeys...)
}

func logDiagnostics(arr []*keyStats, sampleSize, totalKeys int) {
	top := 10
	if len(arr) < top {
		top = len(arr)
	}
	examples := make([]string, 0, top)
	for i := 0; i < top; i++ {
		s := arr[i]
		examples = append(examples, fmt.Sprintf("%s(pr=%.2f,var=%.2f,len=%.1f,k=%d,score=%.2f)", s.key, s.presenceRate, s.variability, s.avgLen, s.keywordMatches, s.score))
	}
	logging.Info("Ranking complete",
		"sample_size", fmt.Sprintf("%d", sampleSize),
		"total_keys", fmt.Sprintf("%d", totalKeys),
		"top_keys", strings.Join(examples, ";"),
	)
}

func defaultRankRules() []rankRule {
	patterns := map[string]float64{
		"(?i)^(request|operation|correlation|trace|span).*": 3,
		"(?i).*(status|result|outcome).*":                   2,
		"(?i).*(error|exception|severity).*":                3,
		"(?i).*(duration|latency|elapsed).*":                2,
		"(?i).*(user|session|tenant|company|environment).*": 2,
		"(?i).*(id)$": 2,
	}
	rules := make([]rankRule, 0, len(patterns))
	for p, boost := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			rules = append(rules, rankRule{re: re, boost: boost})
		}
	}
	return rules
}

func parseRankRegexSpec(spec string) ([]rankRule, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	rules := []rankRule{}
	if strings.HasPrefix(spec, "{") {
		var m map[string]float64
		if err := json.Unmarshal([]byte(spec), &m); err != nil {
			// Attempt lenient recovery: try to salvage simple pattern=boost pairs inside braces by naive splitting
			// Fall through to best-effort semicolon style after trimming braces.
			inner := strings.Trim(spec, "{}")
			semSpec := strings.ReplaceAll(inner, ",", ";")
			return parseRankRegexSpec(semSpec)
		}
		for pattern, boost := range m {
			re, err := regexp.Compile(pattern)
			if err != nil {
				logging.Warn("Invalid custom regex skipped", "pattern", pattern, "error", err.Error())
				continue
			}
			rules = append(rules, rankRule{re: re, boost: boost})
		}
		return rules, nil
	}
	parts := strings.Split(spec, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			logging.Warn("Invalid custom regex fragment skipped", "fragment", part)
			continue
		}
		pattern := strings.TrimSpace(kv[0])
		boostStr := strings.TrimSpace(kv[1])
		if pattern == "" || boostStr == "" {
			logging.Warn("Empty pattern or boost skipped", "fragment", part)
			continue
		}
		boost, err := strconv.ParseFloat(boostStr, 64)
		if err != nil {
			logging.Warn("Invalid boost skipped", "fragment", part, "error", err.Error())
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			logging.Warn("Invalid regex skipped", "pattern", pattern, "error", err.Error())
			continue
		}
		rules = append(rules, rankRule{re: re, boost: boost})
	}
	return rules, nil
}

func parsePinnedList(spec string) []string {
	if strings.TrimSpace(spec) == "" {
		return nil
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		lk := strings.ToLower(v)
		if _, ok := seen[lk]; ok {
			continue
		}
		seen[lk] = struct{}{}
		out = append(out, v)
	}
	return out
}
