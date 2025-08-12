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
		return buildHeadersForAllRowsFallback(columns, rows)
	}
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			logging.Error("Ranking panic; falling back", "error", fmt.Sprintf("%v", r))
		} else {
			logging.Debug("Ranking executed", "took_ms", fmt.Sprintf("%d", time.Since(start).Milliseconds()))
		}
	}()
	headers, err := rankHeaders(columns, rows, cfg)
	if err != nil {
		logging.Error("Ranking failed; fallback alphabetical", "error", err.Error())
		return buildHeadersForAllRowsFallback(columns, rows)
	}
	return headers
}

// rankHeaders orchestrates sampling, stat accumulation, scoring and ordering.
func rankHeaders(columns []appinsights.Column, rows [][]interface{}, cfg config.Config) ([]string, error) {
	if len(rows) == 0 {
		return []string{"timestamp", "message"}, nil
	}
	keys := discoverCanonicalKeys(columns, rows)
	if len(keys) == 0 {
		return []string{"timestamp", "message"}, nil
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
	return headers, nil
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
	s.lenPenalty = s.avgLen / float64(lenCap)
	if s.lenPenalty > 1 {
		s.lenPenalty = 1
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
		if rule.presenceWeighted {
			if s.presenceRate >= alMinPresence {
				contrib *= s.presenceRate
			} else {
				contrib = 0
			}
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
	lenPenalty     float64
	typeBias       float64
	keywordBoost   float64
	keywordMatches int
	score          float64
	pinned         bool
}

// (duplicate computeRankedHeaders removed; single definition kept at top of file)

func buildHeadersForAllRowsFallback(columns []appinsights.Column, rows [][]interface{}) []string {
	return buildHeadersForAllRows(columns, rows)
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
	// Scratch map reused per row (capacity grows to max fields)
	scratch := make(map[string]string, 32)
	for _, r := range sample {
		// reset scratch without reallocating: delete existing entries
		for k := range scratch {
			delete(scratch, k)
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
func scoreStats(stats map[string]*keyStats, rules []rankRule, cfg config.Config, distinctCap, lenCap int, alMinPresence float64, pinnedMap map[string]int) {
	for _, s := range stats {
		if s.occurrences == 0 {
			continue
		}
		computeBaseMetrics(s, distinctCap, lenCap)
		applyTypeBiases(s)
		applyRuleBoosts(s, rules, alMinPresence)
		s.score = cfg.RankWeightPresence*s.presenceRate + cfg.RankWeightVariability*s.variability + s.keywordBoost + cfg.RankWeightLenPenalty*s.lenPenalty + cfg.RankWeightType*s.typeBias
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
		case ai != bj:
			return ai < bj
		default:
			return a.key < b.key
		}
	})
	return arr
}

func assembleHeaders(arr []*keyStats) []string {
	orderedKeys := make([]string, 0, len(arr))
	hasEventID := false
	for _, s := range arr {
		if strings.ToLower(s.key) == "eventid" {
			hasEventID = true
			continue
		}
		orderedKeys = append(orderedKeys, s.key)
	}
	primaries := []string{"timestamp", "message"}
	if hasEventID {
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
			return nil, err
		}
		for pattern, boost := range m {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
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
			return nil, fmt.Errorf("invalid rank regex part: %s", part)
		}
		pattern := strings.TrimSpace(kv[0])
		boostStr := strings.TrimSpace(kv[1])
		if pattern == "" || boostStr == "" {
			return nil, fmt.Errorf("invalid rank regex entry: %s", part)
		}
		boost, err := strconv.ParseFloat(boostStr, 64)
		if err != nil {
			return nil, err
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
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
