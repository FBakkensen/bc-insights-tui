# Step 10 — Dynamic Column Ranking (SDD)

Status: Draft
Owner: TUI Team
Last updated: 2025-08-12

## Summary

Rank dynamic columns (customDimensions) for list views so that important fields appear first. Keep timestamp and message and eventId as primaries, then order remaining keys using frequency and keyword signals. Maintain one canonical header set (no duplicates) shared by snapshot and interactive views, with consistent ellipsis (+N) semantics.

## Goals

- Fixed primaries first: `timestamp`, `message`. `eventId`.
- Consolidated dynamic headers: case-insensitive union of customDimensions keys; no duplicates.
- Order custom keys by data-driven signals: frequency, keywords, with stable tie breakers.
- Deterministic and cheap to compute on a bounded sample (first N rows).
- Identical ordering across snapshot and interactive table views.
- Clear instrumentation for diagnostics; no secrets logged.

## Non-Goals

- Query-aware boosts (e.g., from KQL project/extend/rename) — future step.
- Full user pinning UI — may be introduced later; basic hooks included here.

## User Story

As a user, when I run a query that returns many dynamic fields, the most useful columns (IDs, error/status, durations) should appear first so I can scan results without horizontal scrolling. The same order should apply in both the snapshot and interactive views.

## Terminology

- Canonical headers: `timestamp`, `message`, `eventId`, plus custom keys discovered across all rows (case-insensitive de-dup, first-seen casing), sorted by ranking; used by both list views.
- Sample window: first N rows (default 200) used to compute ranking metrics.

## Ranking Model

Score components (tunable weights):

* presenceRate (strong): fraction of sampled rows where key is non-empty. Weight: 5.0
* keywordBoost (moderate): sum of matched regex boosts (after AL scaling). Weight: intrinsic per-rule boost.
* variability (moderate): normalized distinct value count in sample. Weight: 2.0
* lenRatio (moderate penalty): avgLen / lenCap (clamped 0..1). Weighted by -1.0 (so shorter values help the score by subtracting a smaller number).
* typeBias (light): + for boolean / compact categorical fields; - for long blobs. Weight: 0.5

Implemented score formula:

score = (W_presence * presenceRate) + (W_variability * variability) + keywordBoost + (W_len * lenRatio) + (W_type * typeBias)

with defaults: W_presence=5.0, W_variability=2.0, W_len=-1.0, W_type=0.5.

Tie-breakers (in order, after primary score desc):
1. Pinned (explicitly listed) before non-pinned
2. Higher raw keyword match count (not total boost value)
3. Higher presenceRate
4. Higher variability
5. Shorter avgLen
6. Case-insensitive alphabetical (final deterministic ordering) then original casing

### Primaries and Known-Good Keys

- Always first: `timestamp`, `message`, `eventId`.
- Known-good keys (optional small set): `requestId`, `operationId`, `correlationId`, `traceId`, `spanId`, `userId`, `sessionId`, `severity`, `category`, `environment`, `tenantId`, `companyId`, `status`, `resultCode`, `durationMs`.
- Placement: Known-good keys are boosted by keyword rules and/or a small hard offset so they rise naturally without absolute pinning.

### Keyword Rules (default)

Regex patterns (case-insensitive), with additive boosts:

- `(?i)^(request|operation|correlation|trace|span).*` → +3
- `(?i).*(status|result|outcome).*` → +2
- `(?i).*(error|exception|severity).*` → +3
- `(?i).*(duration|latency|elapsed).*` → +2
- `(?i).*(user|session|tenant|company|environment).*` → +2
- `(?i).*(id)$` → +2 (generic ID suffix)
- `(?i)^al.*` → +3 (Business Central AL telemetry prefix; presence‑weighted, see below)

#### Business Central AL Prefix Rationale

Business Central emits many customDimensions originating from AL code units / objects and these often begin with the prefix `al` (e.g. `alObjectId`, `alMethod`, `alStackFrame`). These fields are typically highly actionable when diagnosing functional issues and should surface early, but only when they are actually present with reasonable coverage. To avoid noise from a single sparse `alSomething` key, the `^al` prefix boost is applied proportionally to its presence rate (presence-weighted keyword boost). Concretely:

`alKeywordContribution = baseBoost * presenceRate` (linear scaling), where `baseBoost` defaults to 3 (configurable). A minimal presence threshold (default 0.05) is enforced; below this, the boost is suppressed (contribution = 0) to prevent rare AL keys from bubbling up. The AL rule still contributes 1 to the keyword match count only when the threshold is met and contributes >0 boost (so extremely sparse AL keys neither boost score nor help tie-breaking).

This keeps dense AL telemetry prominent while ignoring sporadic / experimental fields.

### Sampling and Metrics

Compute metrics over first N rows (configurable, default 200):

- presenceRate: non-empty count / sample size
- variability: min(1, distinctCount / distinctCap), with distinctCap default 50
- avgLen: sum(len(value)) / occurrences, normalized by a cap (e.g., 200 chars)
- typeBias: +0.2 for boolean, +0.2 for short-enum-like fields (<= 5 distinct values and avgLen <= 12); −0.2 for very long strings (avgLen >= 120)

Normalization caps are configuration knobs to keep calculations bounded. Distinct value tracking is capped by a configurable distinctCap; once exceeded we mark variability as saturated to avoid unbounded memory. Average length is accumulated with an upper clamp per value to prevent a single massive blob from dominating.

## Algorithm

1. Discover canonical headers: `timestamp`, `message`, `eventId`, then case-insensitive union of custom keys across all rows; de-duplicate; preserve first-seen casing.
2. Take sample window of up to N rows.
3. For each custom key, compute metrics and score.
4. Sort custom keys by score desc; apply tie-breakers.
5. Final header order: primaries → pinned custom keys (score ordering inside pinned set) → remaining ranked custom keys.
6. Use the same headers in both snapshot and interactive; ellipsis (+N) continues to reflect hidden header count.

## Configuration

Environment variables and config settings (optional, with defaults):

- BCINSIGHTS_RANK_SAMPLE_SIZE (int, default 200)
- BCINSIGHTS_RANK_DISTINCT_CAP (int, default 50)
- BCINSIGHTS_RANK_LEN_CAP (int, default 200)
- BCINSIGHTS_RANK_WEIGHT_PRESENCE (float, default 5.0)
- BCINSIGHTS_RANK_WEIGHT_VARIABILITY (float, default 2.0)
- BCINSIGHTS_RANK_WEIGHT_KEYWORD (float, implicit via regex boosts)
- BCINSIGHTS_RANK_WEIGHT_LEN_PENALTY (float, default -1.0) (applied to lenRatio)
- BCINSIGHTS_RANK_WEIGHT_TYPE (float, default 0.5)
- BCINSIGHTS_RANK_REGEX (string, optional; JSON or semicolon spec for custom regex→boost rules)
- BCINSIGHTS_RANK_PINNED (string, optional; comma-separated keys to pin first after primaries)
- BCINSIGHTS_RANK_AL_PREFIX_BOOST (float, default 3.0) — base boost applied for `^al` prefix before presence weighting.
- BCINSIGHTS_RANK_AL_MIN_PRESENCE (float, default 0.05) — minimum presenceRate required for any `^al` boost.

Implementation detail: Implemented as a special-case after generic regex matching so presence scaling and threshold logic are explicit; only contributes to keyword match count when its scaled boost > 0.

Defaults should be hard-coded with config overrides via existing precedence (defaults → file → env → flags).

## Telemetry & Logging

- On result processing, log:
  - canonical header count and a few example keys.
  - sample size used, total keys, and top 10 ranked keys with components (presence, variability, avgLen, keyword matches, final score).
- Never log values or queries; only counts and key names.

## Acceptance Criteria

- Timestamp and message are always first.
- Consolidated headers are a case-insensitive superset of details (no duplicates).
- With ranking enabled, keys with higher presence and relevant keywords appear before sparse/long blob-like keys.
- Snapshot and interactive list show the same header order and ellipsis count.
- Ranking computation runs within 30ms on typical datasets (<= 200 rows, <= 200 keys); measured locally.
- Logging provides enough context to debug ranking without exposing sensitive data.

## Test Plan

Unit tests (ranker):
- Presence dominance: a key present in 90% rows ranks above one in 10%.
- Keyword influence: `errorCount` outranks a similarly dense non-keyword key.
- Length penalty: a long text field demoted below a compact ID.
- Variability boost: categorical fields with multiple distinct values outrank constants.
- Case-insensitive de-dup: `Foo`, `foo`, `FOO` merge to one header; values map regardless of case.
- Determinism: same inputs → same order; stable sort under ties & explicit alphabetical fallback.
- AL prefix weighting: an `alObjectId` key with high presence (>=50%) outranks a similar non-keyword key; an `alRareField` appearing in <5% of sampled rows receives no boost and does not jump ahead of denser diagnostic keys.
- AL presence scaling: scaling of `^al` boost is linear with presence; verify half presence gives roughly half the keyword contribution (within floating precision tolerance) and suppression below threshold.

UI tests:
- Header order matches ranker output (verify first few columns).
- Ellipsis (+N) computed over the full canonical set; hidden columns are the lowest-ranked.

## Rollout & Feature Flags

- Introduce a config flag `BCINSIGHTS_RANK_ENABLE` (default true). Allow disabling for troubleshooting.
- Guard complex regex parsing behind defensive error handling; fallback to frequency-only ranking if regex parsing fails.

## Risks & Mitigations

- Wide schemas may be slow: cap sample and use O(1) capped structures (distinct sets, length caps). Scratch maps are reused per row to limit allocations; a size heuristic resets them if they grow too large.
- Overfitting to keyword rules: keep weights moderate; allow configuration.
- User confusion: document that ranking is automatic; allow pinning and alternative sort modes in future iterations.

## Future Work

- Query-aware ranking: boost keys projected/extended in the KQL.
- User pin/unpin and sort mode cycling from the UI.
- Persist learned weights per appId/query hash to personalize ordering over time.
