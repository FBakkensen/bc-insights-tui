# Step 8 — Fetch Size Enforcement at Source (Spec-Driven)

This document specifies how the configured fetch size (fetchSize) is applied to KQL queries sent to the Application Insights API, including contracts, UX, and tests. The current behavior only limits rows in the snapshot view; this step adds an optional server-side row cap by appending a KQL limiter when safe.

## Purpose and scope

- Respect the configured fetch size at the data source by appending a KQL limiter when the user query does not already constrain rows.
- Keep the user fully in control: if the query already limits rows (e.g., `take`, `limit`, `top by`, or `top`), do not add another limiter.
- Preserve dynamic schema parsing and all existing Step 5–7 behavior.

Non-goals (deferred):
- Full pagination (skip/continuation tokens). This spec does not implement paging.
- Rewriting user queries beyond adding a single terminal limiter when safe.
- Inferring limits inside subqueries/let definitions (we only check the top-level pipeline textually).

## Background

- Today, fetchSize is used only to trim the snapshot display. The API still returns all rows matching the query (subject to service limits).
- Enforcing a limiter at source reduces latency, memory, and throttling risk for common scenarios.
- Kusto operators relevant here:
  - `take N` and `limit N` are equivalent (ref: Kusto take operator — Microsoft Learn).
  - `top N by <expr>` semantically equals `sort by <expr> | take N` (ref: Kusto top operator — Microsoft Learn).

Authoritative refs:
- Kusto take operator: https://learn.microsoft.com/azure/data-explorer/kusto/query/take-operator
- Kusto top operator: https://learn.microsoft.com/azure/data-explorer/kusto/query/top-operator
- Azure Monitor Log Analytics API response: https://learn.microsoft.com/azure/azure-monitor/logs/api/response-format

## User stories

- As a developer, when I run an unconstrained query, the tool should cap results at my configured fetchSize automatically, so I get fast, bounded results by default.
- As a power user, when I explicitly specify `take`/`limit`/`top`, the tool must not override my intent.
- As a user, I want transparent logging so I can tell when a limiter was appended and what value was used.

## UX behavior

- No UI change to inputs; this is a transparent enhancement.
- Snapshot rendering continues to cap at fetchSize as before.
- Interactive table will naturally include at most the server-returned rows (likely ≤ fetchSize unless the user specified otherwise).
- When a limiter is appended, add a subtle note to the scrollback after the summary: “Applied server-side row cap: take <N>”. If the user already included a limiter, omit the note.

## Contracts (inputs/outputs)

Inputs
- User-entered KQL string (from chat `kql:` or editor submit) and `cfg.LogFetchSize`.

Pre-flight
- Must be authenticated; App Insights App Id must be set (unchanged from Steps 5–7).

Detection rules (string-level; case-insensitive; whitespace-insensitive where practical)
- If the query contains any of the following tokens at the top level (not within a comment):
  - `| take <int>` or `| limit <int>`
  - `| top <int> by` (with required `by`)
  then consider the query already constrained; do not append another limiter.
- Otherwise, append `| take <fetchSize>` to the end of the query, respecting existing trailing whitespace/newline.

Output
- Query sent to API is either the original (if already constrained) or the original plus `| take <fetchSize>`.
- Logging records whether a limiter was applied and the numeric value.
- Scrollback includes a brief informational note when we add the limiter (non-intrusive).

Errors
- No new error modes. All existing error mapping stays the same.

## Edge cases

- Multi-line queries: simply append on a new line if the last char isn’t a newline; otherwise append on the existing newline.
- Trailing comments: appending after trailing comments is acceptable; we don’t parse comments.
- Queries that use `top` without `by`: do not treat as row-limiting unless it matches `top <int> by` per Kusto semantics.
- Embedded `take/limit/top` inside subqueries: out of scope for v1; we only check the textual presence in the whole query. This is a pragmatic compromise; advanced users can stay explicit.
- fetchSize ≤ 0: fall back to default (50) and log the normalization.

## Logging

- On execution attempt, log: `fetch_size` and `limiter_applied` (true/false). Avoid logging the full query. Continue logging query hash/first token.
- When fetchSize is normalized (≤0 → default), log the old/new values.
- When limiter is appended, log once per execution and add the subtle user-facing note.

## Tests (required)

Location: `tui/ui_kql_test.go` or new file `tui/ui_kql_fetchsize_test.go`.

Happy path
- Unconstrained query: outgoing query passed to the KQL client includes `| take <fetchSize>`; user sees the informational note after summary.
- Already constrained queries (no change):
  - `... | take 10` → no extra limiter.
  - `... | limit 10` → no extra limiter.
  - `... | top 5 by timestamp` → no extra limiter.

Edge cases
- Multi-line query without limiter → appends `| take <N>` on its own line.
- fetchSize set to 0 or negative → normalized to 50 before append.
- Query containing the word “take” or “limit” in a string literal or field name: still considered constrained in this v1 textual approach (documented limitation); test ensures we don’t panic and behavior is stable.

Non-functional
- Ensure logs include `limiter_applied` and the numeric `fetch_size`.

## Implementation notes (small, surgical)

- Introduce a tiny helper in the UI layer where we dispatch ExecuteQuery:
  - Input: `query string`, `fetch int`
  - Output: `finalQuery string`, `applied bool`, `effectiveFetch int`
  - Behavior: normalize fetch; detect limiter via case-insensitive substring search for `| take`, `| limit`, or `| top <num> by` (regex acceptable); if none found, append `| take <effectiveFetch>`.
- Call this helper in `runKQLCmd` before `ValidateQuery` and `ExecuteQuery`.
- Append the user-facing note after success only if `applied==true`.
- Keep the appinsights client unchanged; enforcement is a presentation-layer concern.

## Rollout and compatibility

- Backward compatible; explicit user limits continue to win.
- No config or flag changes; feature is on by default because it’s conservative and transparent.
- If field feedback suggests surprises, consider a config toggle in a future step (e.g., `enforceFetchSizeAtSource` true/false).

## Acceptance criteria

- For unconstrained queries, the tool appends `| take <fetchSize>` server-side and surfaces a small note about the applied cap.
- For constrained queries, the tool does not inject another limiter.
- Logging includes whether the limiter was applied and the effective `fetch_size`.
- All existing tests continue to pass; new tests added per above.

---

Appendix: Examples

- Unconstrained
```
traces
| where timestamp > ago(1h)
```
becomes
```
traces
| where timestamp > ago(1h)
| take 50   // if fetchSize=50
```

- Already constrained
```
traces | where severityLevel >= 3 | take 100
```
(no change)

- Top by (constrained)
```
requests | top 10 by duration desc
```
(no change)
