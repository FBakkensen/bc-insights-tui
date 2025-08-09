# Step 5 — Single-line KQL Execute (Spec-Driven)

This document defines the precise behavior, contracts, and tests for implementing Step 5 of the chat-first UI: executing a single-line KQL via the chat input and presenting a compact, dynamic results snapshot with an option to open an interactive table panel.

## Purpose and scope

- Let the user run a KQL query from the single-line chat input via the prefix: `kql: <query>`.
- Immediately append a “Running…” message to scrollback, then append a summary and a compact, non-interactive table snapshot.
- Offer a simple way to open the results as an interactive table panel; Esc returns to chat.
- Dynamic schema: build columns from the API response (no hard-coded structs).

Non-goals (deferred to later steps):
- Multi-line editor mode (Step 6) and keybindings for that mode.
- AI prompt-to-KQL generation.
- Saved queries and history commands (beyond minimal append-to-history).
- Server-side cancellation.

## Dependencies and references

- appinsights client in this repo: `ExecuteQuery(ctx, kql)` and `ValidateQuery(kql)` with dynamic `Tables[0].Columns/Rows`.
- config for settings: `ApplicationInsightsAppId`, `LogFetchSize`, `QueryTimeoutSeconds`.
- auth: must be authenticated (device flow) before running queries.
- Bubbles components: `viewport` (scrollback), `textarea` (chat), `table` (interactive results panel) — keep component usage simple and idiomatic.
- Reflow: for ANSI-aware wrapping/truncation where needed in snapshots.

Authoritative docs to align with:
- Bubble Tea basics and commands/messages, alt screen, focus: https://github.com/charmbracelet/bubbletea
- Bubbles components: textarea, viewport, table
  - https://github.com/charmbracelet/bubbles/tree/master/textarea
  - https://github.com/charmbracelet/bubbles/tree/master/viewport
  - https://github.com/charmbracelet/bubbles/tree/master/table
- ANSI-aware wrapping/indent/padding: https://github.com/muesli/reflow
- Application Insights/Log Analytics query response model (tables/columns/rows) – see Azure Monitor Log Analytics API response format: https://learn.microsoft.com/azure/azure-monitor/logs/api/response-format

## User flow (happy path)

1) In chat mode (single-line), user types: `kql: <query>` and presses Enter.
2) UI echoes input to scrollback and appends: “Running query…” with a spinner-like ellipsis (no separate bubble).
3) Program validates the query syntactically (lightweight `ValidateQuery`). If invalid, append actionable error (stay in chat).
4) Program executes the query with a timeout from `config.QueryTimeoutSeconds` using the configured Application Insights App Id; on success:
   - Append a succinct summary: e.g., “Query complete in X.XXXs · N rows · table: <table-name>”.
   - Append a compact table snapshot (non-interactive) built from the first min(N, fetchSize) rows using the dynamic column names.
   - Append hint: “Press Enter to open interactively.” (Esc closes the table once open.)
5) If the user presses Enter with an empty chat input (and a “last results” snapshot is available), switch the top panel to an interactive table (Bubbles table) with the same dynamic columns and loaded rows; focus moves to the table. Esc returns to chat.

Notes
- Do not introduce extra modal layers. Reuse the top panel for the table and return to chat with Esc.
- Always scroll to bottom after appending content or closing the table.

## UI states and focus

- Mode: `modeChat` (default) and `modeTableResults`.
- Chat: textarea focused; Enter submits when value starts with `kql:`; Enter with empty value and a pending “open last results” affordance triggers opening the interactive table.
- Table: focus inside table; Esc returns to chat; viewport content remains in background (not visible while table is shown).

## Contracts (inputs/outputs)

Input (command)
- Raw chat string starting with `kql:` (case-insensitive prefix accepted), followed by a non-empty query string.

Pre-flight checks
- Must be authenticated.
- `config.ApplicationInsightsAppId` must be set; otherwise show actionable error with next steps.

Validation
- Use `appinsights.Client.ValidateQuery(query)` for lightweight checks (non-empty, starts with known table, balanced brackets). On failure, append error and return.

Execution
- Build `ctx, cancel := context.WithTimeout(ctx, time.Second*QueryTimeoutSeconds)` and defer cancel.
- Call `client.ExecuteQuery(ctx, query)`.

Output (successful execution)
- Summary line with duration, row count (from `len(Tables[0].Rows)`), and first table name if present.
- Snapshot string rendering of the first `min(rowCount, cfg.LogFetchSize)` rows using the response’s dynamic columns; ANSI-aware wrapping/truncation where needed.
- Store “last results” (columns, rows) for interactive opening.

Output (error execution)
- Append actionable error. Examples:
  - Auth: “Authentication required. Run ‘login’ and try again.”
  - App Id: “Application Insights App Id is not set. Run ‘config set applicationInsightsAppId=<id>’.”
  - 400/Bad KQL: include status and top-level message; suggest checking table names.
  - 401/403: suggest verifying App Insights permissions / tenant.
  - 429: suggest retry later (throttling) and reduce result size.
  - Timeout: “Query timed out after Xs. Increase queryTimeoutSeconds or simplify the query.”

Logging (per logging guidelines)
- Log user action: executing KQL (without full query if sensitive; log hash/length and first table token).
- Log pre-flight state (auth state, appId presence), timeout used, fetch size.
- Log API latency and HTTP status; on error include response body excerpt.

## Rendering details (keep it simple)

- Snapshot: build a minimal Bubbles table model with dynamic headers and rows, set width to current viewport inner width, and call `table.View()` to produce a string for the scrollback. Avoid manual padding; let the component style manage widths. Limit rows by `cfg.LogFetchSize`.
- Interactive table: create the same table model, but as a focused top panel; Esc closes it. Don’t add custom keymaps beyond default up/down/pgup/pgdn at this step.
- Wrapping/truncation: allow defaults from the table component; optionally use `reflow/wordwrap` or `reflow/wrap` in snapshot-only if a column is extremely wide. Prefer not to hand-calculate widths.

## Edge cases and handling

- Empty or whitespace after `kql:` → validation error: “Query cannot be empty.”
- Not authenticated → guidance to run `login`.
- Missing App Id → guidance to `config set applicationInsightsAppId=<id>`.
- No tables in response → show “No results.”
- Columns present but zero rows → show “0 rows” and an empty-table header snapshot.
- Null values in rows → render as empty string.
- Very wide content → rely on table’s default truncation/wrapping; snapshot limited to viewport width.
- Very large result set → snapshot shows up to `fetchSize`; interactive table loads full results set already in memory.
- Slow query → show ‘Running…’; timeout ends with clear message.

## Messages/events (internal)

- User enters `kql:` in chat → internal `runKQLMsg{ query }` cmd.
- Command runs async; on completion → `kqlResultMsg{ columns, rows, summary, duration, err }`.
- When user “opens interactively” → switch mode to `modeTableResults` and initialize table model.
- Escape in table mode → switch back to `modeChat` and append “Closed results table.”

## Acceptance criteria

- Typing `kql: <query>` in chat runs the query and appends a summary plus a compact table snapshot; “Open interactively” hint works; table panel supports navigation; Esc returns.
- Dynamic columns are built from the response; no hard-coded fields.
- Errors are actionable and logged; timeouts respect `queryTimeoutSeconds`.
- Snapshot respects `fetchSize` and viewport width; no manual padding math.

## Test plan (required tests)

Location: `tui/ui_kql_test.go` unless noted.

Unit-level (model/update)
- Parse and dispatch
  - `kql:` with valid query dispatches a command (returns non-nil tea.Cmd) and echoes input; appends “Running…”.
  - `kql:` with empty/whitespace → validation error message in scrollback, no command.
- Validation and friendly errors
  - Invalid: not starting with known table → shows validation error.
  - Not authenticated → shows guidance to run `login`.
  - Missing App Id → shows guidance to `config set applicationInsightsAppId=<id>`.
- Results rendering (dynamic schema)
  - Zero tables → “No results.”
  - Columns but zero rows → “0 rows” summary and header-only snapshot.
  - Small N rows → snapshot includes N or `fetchSize` rows, whichever smaller; headers match `Columns` from response.
  - Values with nulls/long text → nulls rendered empty; long text is truncated/wrapped within viewport width (don’t test exact wrapping, just assert non-empty and printable).
- “Open interactively” flow
  - After a successful snapshot, pressing Enter with empty chat input switches to `modeTableResults`.
  - In table mode, Esc switches back to `modeChat` and scrollback gets “Closed results table.” (or similar message).
- Timeout and API error pathways (using fakes)
  - Timeout: fake client sleeps beyond timeout → timeout message.
  - 400/401/403/429: fake client returns error strings → message includes status hint.

Integration-style (isolated, with fake client)
- Provide a test double implementing the minimal interface used by the UI for KQL (`ValidateQuery`, `ExecuteQuery`) that returns controlled `QueryResponse` or errors. Ensure no network calls.
- Verify that `cfg.LogFetchSize` and `cfg.QueryTimeoutSeconds` affect behavior as expected (e.g., set tiny timeout in test to exercise timeout branch).

Support tests (small helpers)
- Dynamic header builder returns names in order from `Columns`.
- Row rendering converts `nil` to "" and formats primitives via fmt.Sprint.

Notes
- Tests must not require interactive TUI run. Use synthetic messages and direct `Update` calls (see existing tests in `tui/ui_test.go`).
- Avoid asserting exact ANSI or full table strings; assert presence of headers, row counts, and mode transitions.

## Implementation checklist (dev tasks)

1) Command parsing: recognize `kql:` prefix (case-insensitive) in chat mode.
2) Pre-flight: ensure auth done and App Id present; otherwise append guidance.
3) Validate query with `appinsights.Client.ValidateQuery`.
4) Append echo and “Running…”; trigger async cmd with timeout.
5) On success: compute duration, build snapshot string via Bubbles table, append summary + snapshot + hint; store last results for interactive open.
6) On error: append actionable error.
7) Enter-to-open: if empty chat input and last results exist, open table panel; Esc returns to chat.
8) Logging: instrument user action, config changes, errors per `.github/instructions/logging.instructions.md`.
9) Tests: add `tui/ui_kql_test.go` with cases above; keep non-interactive.

## Windows/terminal notes

- Continue using the alt screen and `GotoBottom()` for scrollback; bracketed paste enabled by default.
- Keep mouse interactions off; rely on keyboard navigation.
- Low memory: snapshot only; interactive loads existing in-memory rows.

---

Appendix: quick component references
- Bubbles table/textarea/viewport: see component folders in https://github.com/charmbracelet/bubbles
- Reflow ANSI-aware wrapping: https://github.com/muesli/reflow
