# Step 9 — Dynamic customDimensions Details View (Spec-Driven)

This document specifies the details view that renders the Business Central `customDimensions` payload per log row without any per-`eventId` schema. The goal is to provide a simple, complete, and safe presentation of the timestamp and all `customDimensions` fields.

## Purpose and scope

- Provide an interactive details panel for a selected result row that:
  - Shows the row timestamp directly from the `traces` table (from the `timestamp` column when present; fall back to common variants like `timeGenerated` if available).
  - Lists all fields from `customDimensions` dynamically (no static models and no per-`eventId` schema).
  - Avoids additional top-level fields unless explicitly added later; the v1 scope is timestamp + all `customDimensions`.
- Keep the UI responsive and simple; avoid bespoke layout math—prefer Bubbles components with natural wrapping.
- Do not change query behavior; details view operates on the existing result row in memory.

Non-goals (deferred):
- Persisted “details history”.
- Custom per-event layouts (e.g., bespoke cards). The initial version uses generic, heuristic ordering.
- In-row JSON editing or re-querying from details.

## Dependencies and references

- Data: `appinsights.QueryResponse` → `Tables[...].Columns` + `Rows` (existing).
- UI: `tui/` Bubble Tea MVC. Table view already shows query results; details will open from table selection.
- Guidelines: `.github/copilot-instructions.md` (dynamic telemetry, never hard-code schemas), `docs/overview.md` (Business Central context), `.github/instructions/ui.instructions.md` (minimal code, consistent styling), `.github/instructions/logging.instructions.md` (log important user actions; no secrets).

## Requirements (contract)

Functional
- Open details from the interactive results table with Enter; close with Esc.
- Details panel shows:
  - Header: table name, row index, and a short summary (presence of timestamp and count of custom fields).
  - Timestamp line: value from `timestamp` (or `timeGenerated`) when present.
  - “customDimensions” section: dynamic key/value list containing all keys from the row’s `customDimensions` value.
- Dynamic parsing rules for `customDimensions`:
  - Accept underlying types: map/object, JSON string, or null.
  - When a string, attempt JSON parse; if parse fails, render the raw string under a single key (e.g., `raw`).
  - Normalize to a flat map of keys → string values:
    - For scalar values, stringify (numbers, bools, strings, null → empty).
    - For nested objects/arrays, flatten up to depth=2 using dot notation (e.g., `parent.child`), and stringify deeper structures as minified JSON.
- Ordering & grouping:
  - customDimensions: present all keys and sort A→Z (case-insensitive). No per-`eventId` prioritization.
- Rendering:
  - Use a viewport-based text rendering with wrapping. Each field on its own line as `key: value`.
  - Long values (e.g., URLs, JSON) are wrapped; consider truncating lines after a safe width in snapshot, but details should allow scrolling.
- Logging:
  - On open: `details_opened` with `table`, `row_index`, `has_timestamp`, and `custom_count`.
  - On close: `details_closed` with `duration_ms` spent in view.
  - Never log raw `customDimensions` values.

Error handling & edge cases
- If `customDimensions` is missing or null: show “customDimensions: <none>”.
- If `customDimensions` cannot be parsed: show raw string with a parse warning line.
- Handle rows where `customDimensions` was projected away by KQL: show timestamp if present and indicate `<none>` for `customDimensions`.
- Handle unknown or new event IDs transparently with the same generic renderer.

Performance & lifecycle
- Parsing happens only when opening details, against the single selected row.
- No background I/O; all in-memory.
- Memory impact is minimal; flattening is bounded to depth 2 and only runs for one row at a time.

Security & privacy
- Do not log raw bodies, `customDimensions` content, or tokens.
- Redaction: No additional redaction beyond what queries already avoid; details are for local display. If future redaction is needed, add a key-based redaction list.

## Config and UX

- No new settings in v1. Details view is triggered by user action and reads from existing results state.
- Future (deferred): a config to control flatten-depth or maximum value length.

## Data contracts

Input
- Columns: `[]appinsights.Column` (name, type)
- Row: `[]interface{}` (same order/length as Columns; element types may be string, float64, bool, nil, map[string]any, []any)

Output (normalized for view)
- Metadata: `table` (string), `rowIndex` (int)
- Timestamp: `timestamp` (string; empty when not present)
- Custom dims: ordered `[]DetailField`

DetailField
- key: string
- value: string (human-friendly; JSON minified when necessary)
- group: enum {Standard, Custom}
- priority: int (lower renders first; used only within Custom group)

Helper function contract (non-UI)
- Build function in a new internal helper package to keep UI thin:
  - `internal/telemetry/details.go`
  - `func BuildDetails(columns []appinsights.Column, row []interface{}) (timestamp string, custom []DetailField)`
  - Behavior per rules above (parse, flatten, order, stringify).
  - No logging from the helper; logging is done at UI open/close points.

## Algorithms

1) Locate columns by name (case-insensitive): indices for `customDimensions`, `timestamp` (fallback: `timeGenerated`).
2) Parse `customDimensions` value from row:
   - If value is `map[string]any`, use it.
   - Else if `string`, try `json.Unmarshal` to `any`. If object/array → use; else treat as raw string.
   - Else if `[]any`, flatten indices as `key[index]` or stringify JSON.
   - Else nil → none.
3) customDimensions fields: flatten depth ≤ 2; sort by key A→Z (case-insensitive); stringify values.
4) Return `(timestamp, custom)` to UI.

Stringify rules
- nil → ""
- bool/number → fmt-based string
- string → as-is
- map/array → minified JSON (compact) unless included via a flattened key at depth ≤ 2

## UI integration

- Trigger: In table results mode, pressing Enter on a selected row opens details.
- Layout: Reuse a `viewport.Model` sized like the results view; a simple header (table, row index, counts) followed by:
  - Timestamp line if present
  - “customDimensions” (key: value for all keys)
- Keys: Esc closes details and returns to table. Future: `/` to filter within details (deferred).
- Keep the design minimal and consistent per UI instructions; rely on Bubbles wrapping.

## Logging

- On open: `logging.Info("details_opened", "table", t, "row_index", i, "has_timestamp", ts != "", "custom_count", len(custom))`
- On close: `logging.Info("details_closed", "duration_ms", dur)`

## Tests

Unit (helper)
- Location: `internal/telemetry/details_test.go`
- Cases:
  - customDimensions as map[string]any → flatten and list all keys.
  - customDimensions as JSON string → parses to map and works as above.
  - No customDimensions column → timestamp-only (if present) and `<none>` for custom.
  - customDimensions parse error (invalid JSON string) → raw shown with parse warning marker.
  - Nested objects/arrays → flattened to depth 2; deeper structures stringified; arrays rendered with indices or JSON.
  - Mixed types (numbers, bools, null) → stringified correctly.

UI (lightweight)
- Location: `tui/` tests (new tests or extend existing patterns):
  - Simulate results present and Enter in table mode → details opens; Esc closes.
  - Snapshot of rendered details contains header and expected key lines.

## Acceptance criteria

- Selecting a row and pressing Enter opens a details panel showing the timestamp (if present) and all `customDimensions` fields parsed dynamically.
- No hard-coded schemas or `eventId`-based prioritization are required; unknown event IDs display cleanly as part of `customDimensions`.
- Logs include `details_opened`/`details_closed` without leaking field values.
- Unit tests cover parsing for both map and string `customDimensions` with nested content.

## Implementation plan (phased)

1) Helper package
- Create `internal/telemetry/details.go` with `DetailField` and `BuildDetails` implementation.
- Add focused unit tests in `internal/telemetry/details_test.go`.

2) TUI integration (minimal)
- In table mode, handle Enter to open a new `modeDetails` (new enum) using a `viewport.Model`.
- Build content string using helper output; header + sections; set viewport with wrapping.
- Esc returns to table mode.

3) Logging
- Add `details_opened` and `details_closed` logs at the UI layer.

4) Polishing (optional, small)
- Pretty JSON minifier and safe truncation for extremely long values (only in UI rendering, not in helper normalization).
- Simple in-view search (`/` to filter) can be deferred.

5) Docs
- Update README usage notes under results table to mention Enter opens details.

Out of scope for this step
- Saved schemas or per-event custom templates.
- Clipboard integration and advanced filtering.
- Config flags for depth/length.

Notes
- BC telemetry evolves; the generic renderer must remain robust to new keys.
- Keep UI lean; most complexity remains in the parsing helper with focused tests.
