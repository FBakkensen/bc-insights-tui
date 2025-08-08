# Chat-First UI Rewrite Plan (Clean Slate)

This document summarizes research and provides a concrete, step-by-step plan to completely replace the current UI with a chat-first interface using Bubble Tea and Bubbles.

Scope: delete the existing TUI implementation and re-implement the UI progressively in the order features are needed. No feature flags or dual paths.

## Rationale

- Current UI relies on modal overlays and a command palette that complicate control flow and state.
- Bubble Tea works best with composable components and mode/focus switches, not layered modals.
- A chat-first pattern is simpler and familiar: bottom input (textarea), scrollback above (viewport), newest at the bottom, minimal overlays.

## Core layout and patterns

- Full-screen app (alt screen). Two main areas:
  - Top: viewport.Model for scrollback (messages: system/info/error/user, plus summaries).
  - Bottom: textarea.Model for chat input (2–3 rows by default). Enter submits; multi-line when in editor mode.
- No modal overlays. When interaction is needed (tables, lists), the top region becomes an active panel for that component (Esc returns to scrollback). Chat remains visible at the bottom.
- On WindowSizeMsg: set viewport height to (terminal height - textarea height - margins). Always scroll to bottom on new content.
 - Use the alternate screen buffer explicitly (tea.WithAltScreen()). On appends or after resize, call viewport.GotoBottom() so newest content remains visible.

## Business Central telemetry (dynamic schema)

- Never use static data models for log entries. Business Central’s `eventId` determines the schema of `customDimensions`.
- Query handling and rendering must adapt dynamically:
  - Build table columns from the KQL `project` statements or infer keys from `customDimensions` for the selected `eventId`.
  - Details views show key-value pairs dynamically; avoid hard-coded fields.
  - Expect schema changes across BC releases and partner events.

## Progress

- [x] Research and write clean-slate plan
- [x] Add deletion-first step to the plan
- [x] Step 0: Remove existing UI and UI tests
- [x] Step 1: Login (device flow)
- [ ] Step 2: Select subscription
- [ ] Step 3: Select Application Insights resource
- [ ] Step 4: Set / Get configuration
- [ ] Step 5: Type and execute KQL

## Implementation order (clean rebuild)

### Step 0 — Remove existing UI and UI tests
- Completely delete all current TUI source files (tui/model.go, update.go, view.go, overlays, selectors, editor UI, tables) and any UI-only helpers.
- Remove UI-related tests under `tui/` and any other UI-focused test files.
- Ensure the repository builds and tests pass without the UI: run `make all` and confirm linting, race tests, and build are green.
- Keep non-UI packages (auth, appinsights, config, logging, query execution) intact.

### Step 1 — Login (Azure OAuth2 Device Flow)
- Goal: Authenticate and surface clear steps and errors in chat.
- UX:
  - On start, show instructions: “Open https://microsoft.com/devicelogin and enter code ABCD-1234.”
  - Show progress messages (waiting, verified, token received) in the scrollback.
  - Errors are actionable (permissions, tenant, network).
- Keys:
  - Enter in chat recognized but blocked until authenticated (or allow `set tenant=<id>` before auth).
  - Esc/Ctrl+C quits gracefully.

### Step 2 — Select subscription
- After login, list azure subscriptions to select a subscription and update config with subscription id.
- if following the intention of bubble tea i would like a list in the viewport to select from. https://github.com/charmbracelet/bubbletea/tree/main/examples/list-simple

### Step 3 — Select Azure Application Insights resource
- Similar to subscriptions: show insights components for the chosen subscription in a list panel.
- On selection, confirm in chat and set as current context.

### Step 4 — Set / Get Configuration
- Commands via chat:
  - `set key=value` (e.g., set tenant, timeRange, fetchSize, theme), persisted via existing config package.
  - `get key` / `config` to dump current settings.
- Echo confirmations and validation errors to scrollback.

### Step 5 — Type and execute query (KQL)
- Default: single-line chat input where `kql: <query>` executes immediately.
- Editor mode for multi-line KQL:
  - Command `edit` toggles textarea to multi-line mode (higher height, InsertNewline enabled). Ctrl+Enter submits; Esc cancels editor mode.
- Execution flow:
  - Append “Running…” message; upon completion append a summary and a results snapshot (rendered using bubbles/table to a string) into scrollback.
  - Provide hint: “Press Enter to open interactively.”
  - When activated, switch top panel to an interactive table (bubbles/table) with dynamic columns from the KQL `project` result. Esc returns to scrollback.

## Command summary (initial set)

- `help` — prints available commands and key bindings.
- `login` — starts device flow (also auto-started on launch if not authenticated).
- `subs` — open subscription selector panel.
- `resources` — open Application Insights selector panel.
- `set <key>=<value>` — set configuration.
- `get <key>` / `config` — print current config.
- `edit` — toggle multi-line KQL editor mode.
- `kql: <query>` — execute a single-line KQL.
 - `ai: <prompt>` — generate KQL from natural language (future step; wires to ai/assistant when available).
 - `filter: <text>` — quick text filtering of recent results/messages (future step).

## Keyboard and focus

- Default focus: chat textarea (Enter submits).
- When in a panel (list/table), focus is in that component; Esc returns to chat.
- Editor mode: Enter inserts newline; Ctrl+Enter submits; Esc exits editor mode.

## Error handling (project requirement)

- Errors must be actionable and friendly, e.g.: “Authentication failed. Check Azure permissions and verify your credentials at https://portal.azure.com”.
- Post errors into scrollback; allow retry via simple commands (`login`, `kql:` again, etc.).

## What to delete (clean slate)

- Remove tui overlays and command palette patterns.
- Replace `tui/model.go`, `tui/update.go`, `tui/view.go`, and associated overlay helpers with the chat-first layout.
- Keep supporting packages (auth, appinsights, config, logging, query execution) and rewire to the new UI.

## Likely additions (missing from the initial list)

- Tenant selection (if multiple tenants): allow `set tenant=<id>` pre-auth or guide user to the right tenant after auth failure.
- Time range defaults and quick changes: `set timerange=last_24h`.
- Query cancellation: `cancel` to attempt canceling an in-flight request.
- Query history: append executed KQL to history; commands `history`, `!n` to rerun.
- Paging/limits: large result sets paged in interactive table; scrollback only shows a compact snapshot.
- Keyboard help: ‘?’ or `help` prints keys for current panel.
- Logging: keep a log file; surface last error id in chat for support.
 - Saved queries: load/save named KQL snippets; integrate into chat via `save <name>`, `load <name>` (future step).

## Acceptance criteria per step

1) Login
- Device flow instructions appear; successful auth is confirmed in chat; failures include guidance and link.

2) Select subscription
- List of subscriptions opens in the top panel; selection persists and is confirmed in chat; Esc returns to chat.

3) Select insights resource
- List opens, selection persists, confirmed in chat; Esc returns to chat.

4) Config
- `set/get/config` work with validation and persistence; confirmations appear in chat.

5) KQL
- `kql:` runs and prints summary + table snapshot to scrollback; “Open interactively” hint works; table panel supports navigation; Esc returns.

## References (patterns used)

- Bubbles textarea + viewport (chat example): bottom input, top scrollback with `GotoBottom()` on updates.
- Bubbles list and table: focused interactive components as top panel, no modals.
- Lip Gloss for consistent borders, paddings, and dynamic width/height.

## Notes

- Windows-first: alt screen, disable deprecated high-perf rendering; enable bracketed paste; keep mouse off by default.
- Keep memory footprint low: summaries in scrollback; open large tables only when requested.
 - Online-only: no offline mode or local log caching.
