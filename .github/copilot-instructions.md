## Copilot instructions (concise, repo-specific)

Current state
- Chat-first rewrite in progress (Step 0): `main.go` runs, TUI is being rebuilt. Core packages (`config/`, `auth/`, `appinsights/`, `logging/`) are active; many `tui/` files are placeholders. Read `docs/chat-first-ui-rewrite.md` before UI work.

Do-first for agents
- Always plan with the Todos tool; track each step separately.
- Clean build gate: run `make all` (lint → race tests → build). Zero warnings, all tests green.
- Research first: use Context7 + Perplexity + official Bubble Tea/Bubbles docs (and search charm repos) before changing linters, OAuth, or Bubble Tea patterns.
- **CRITICAL: AI agents CANNOT run interactive TUI mode** - the app will hang waiting for keyboard input. Only use `./bc-insights-tui.exe -run=COMMAND` for testing. If interactive TUI testing is needed, ask the user to test it manually.

Architecture (big picture)
- Go + Charm ecosystem. Entry: `main.go` → init `logging` → load `config` → (future) start chat-first UI.
- Dynamic telemetry: never hard-code Business Central log schemas. `eventId` defines `customDimensions` keys; table columns must follow KQL `project` results.
- Packages:
  - `config/`: precedence = defaults → JSON file (`config.json` or user home) → env (BCINSIGHTS_*) → flags. Provides `ValidateAndUpdateSetting`, `ListAllSettings` for `set/get` commands.
  - `auth/`: Azure OAuth2 Device Flow. Key APIs: `InitiateDeviceFlow`, `PollForToken`, `SaveTokenSecurely` (go-keyring), `GetValidToken`/`RefreshTokenIfNeeded`.
  - `appinsights/`: `ExecuteQuery(ctx, kql)` posts to `https://api.applicationinsights.io/v1/apps/{appId}/query`; returns tables with dynamic `Columns` and `Rows`. `ValidateQuery` does lightweight checks.
  - `logging/`: logs to `logs/bc-insights-tui-YYYY-MM-DD.log`. To mirror logs to stdout, set `BC_INSIGHTS_LOG_TO_STDOUT=true` (opt-in).

Workflows that matter
- Build/test/lint: `make build`, `make test`, `make race`, `make lint`, `make all` (CI-equivalent). UI-specific targets: `make ui-test` (when UI exists).
- Run (Windows): `./bc-insights-tui.exe`. It currently prints a Step 0 placeholder.
- **Non-interactive mode: `./bc-insights-tui.exe -run=COMMAND` for testing/automation without TUI. Commands: `subs` (list subscriptions), `login` (device flow auth). MANDATORY for AI agents - never run interactive mode as it hangs waiting for keyboard input.**
  - Extra: `-run=logs[:N]` prints the last N lines from the latest log file (default N=200) without enabling stdout mirroring.
- Config examples (env): `LOG_FETCH_SIZE`, `BCINSIGHTS_ENVIRONMENT`, `BCINSIGHTS_APP_INSIGHTS_ID`, OAuth: `BCINSIGHTS_OAUTH2_TENANT_ID`, `BCINSIGHTS_OAUTH2_CLIENT_ID`, `BCINSIGHTS_OAUTH2_SCOPES`.

UI conventions (chat-first)
- Replace modal overlays/command palette with: top viewport (scrollback) + bottom textarea (chat). No layered modals; switch focus between components; Esc returns to chat. See `docs/chat-first-ui-rewrite.md` for layout, keys, and steps.
- When reintroducing `tui/`, keep Bubble Tea MVC files: `model.go`, `update.go`, `view.go`. Compose bubbles; avoid global state.

Project-specific rules
- Errors must be actionable (include likely cause and next step, e.g., Azure portal link).
- Online-only; no local caching. Low memory: summarize large results; open interactive tables on demand.
- KQL-driven columns: parse API result columns; don’t rely on static structs.

Integration quick path (example)
1) Acquire token: `auth.NewAuthenticator(cfg.OAuth2)` → device flow → `SaveTokenSecurely`.
2) Query: `appinsights.NewClient(token, cfg.ApplicationInsightsID).ExecuteQuery(ctx, kql)`.
3) Render: build table columns from `QueryResponse.Tables[0].Columns`.

Key files to read first
- `docs/chat-first-ui-rewrite.md`, `makefile`, `config/config.go`, `auth/authenticator.go`, `appinsights/client.go`, `main.go`.

Acceptance before PR
- `make all` passes cleanly; style/lint zero warnings; tests (incl. race) pass. Update or add focused tests when changing public behavior.
