## Copilot instructions (concise, repo-specific)

Current state
- Chat-first rewrite (Step 1). `main.go` initializes logging, loads config, then either runs a non-interactive command or launches the TUI. Core packages (`config/`, `auth/`, `appinsights/`, `logging/`) are active; `tui/` is being rebuilt. See `docs/overview.md` and `docs/specs/*` for UI intent.

Do-first for agents
- Plan with the Todos tool; keep one active item at a time.
- Quality gate: run `make all` (lint → race tests → build). Expect zero lint warnings and green tests.
- Use official Charm docs when touching Bubble Tea/Bubbles. Research before changing OAuth/logging patterns.
- CRITICAL: don’t run interactive TUI. Use `./bc-insights-tui.exe -run=...` only; ask the user to test interactive flows.

Architecture essentials
- Dynamic telemetry: never hard-code Business Central schemas. `eventId` drives `customDimensions`; build table columns from API response columns.
- Packages
  - `config/`: precedence = defaults → JSON file (`%AppData%/bc-insights-tui/config.json`) → env (BCINSIGHTS_*) → flags; helpers: `ValidateAndUpdateSetting`, `ListAllSettings`.
  - `auth/`: Azure OAuth2 device flow. Refresh tokens are stored in the OS keyring (zalando/go-keyring). APIs: `InitiateDeviceFlow`, `PollForToken`, `SaveTokenSecurely`, `StoredRefreshTokenPresent`, `GetTokenForScopes`, `GetApplicationInsightsToken` (v1).
  - `appinsights/`: `ExecuteQuery(ctx, kql)` → https://api.applicationinsights.io/v1/apps/{appId}/query; dynamic `Columns`/`Rows`; `ValidateQuery` preflight. Uses v1 resource token from `auth`.
  - `logging/`: daily file `logs/bc-insights-tui-YYYY-MM-DD.log`; level via `BC_INSIGHTS_LOG_LEVEL`; optional stdout mirroring `BC_INSIGHTS_LOG_TO_STDOUT=true`.

Build/test workflows
- Make: `make build`, `make test`, `make race`, `make lint`, `make all`. UI tests: `make ui-test`.
- Run (Windows, non-interactive only): `./bc-insights-tui.exe -run=COMMAND`.

Non-interactive commands
- `-run=login` (device flow; persists refresh token)
- `-run=login-status` (diagnose keyring; attempt silent ARM refresh)
- `-run=subs` · `-run=resources` (list subscriptions/resources)
- `-run=config` · `-run=config-save` · `-run=config-reset` · `-run=config-path`
- `-run=keyring-info` · `-run=keyring-test` (keyring diagnostics)
- `-run=logs[:N]` (tail latest log; default 200)

Config/env conventions
- Basics: `LOG_FETCH_SIZE`, `BCINSIGHTS_ENVIRONMENT`, `BCINSIGHTS_APP_INSIGHTS_ID`, `BCINSIGHTS_APP_INSIGHTS_KEY`, subscription via `AZURE_SUBSCRIPTION_ID` (or `BCINSIGHTS_AZURE_SUBSCRIPTION_ID`/`ARM_SUBSCRIPTION_ID`).
- OAuth2: `BCINSIGHTS_OAUTH2_TENANT_ID`, `BCINSIGHTS_OAUTH2_CLIENT_ID`, `BCINSIGHTS_OAUTH2_SCOPES` (comma-separated). Device flow ensures `openid profile offline_access`.
- Keyring overrides (diagnostics/isolation): `BCINSIGHTS_KEYRING_SERVICE`, `BCINSIGHTS_KEYRING_NAMESPACE`.

Project-specific rules
- Errors must be actionable (map common HTTP/Kusto/auth failures to hints). Never log secrets or full KQL; log query sizes/IDs and timings only.
- Log important state changes (auth transitions, config old→new). Use `logging/` helpers consistently.
- KQL/UI are dynamic: parse result columns; keep Bubble Tea MVC (`tui/model.go`, `update.go`, `view.go`); avoid global state.

Key references
- `main.go`, `makefile`, `config/config.go` + `config/loader.go`, `auth/authenticator.go`, `appinsights/client.go`, `logging/logger.go`, docs: `docs/overview.md`, `docs/specs/*`.

Acceptance before PR
- `make all` passes; no lint warnings; tests (incl. race) green. Add/update focused tests when public behavior changes.
