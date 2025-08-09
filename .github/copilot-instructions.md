## Copilot instructions (concise, repo-specific)

Current state
- Chat-first rewrite in progress (Step 1). `main.go` initializes logging, loads config, then either runs a non-interactive command or launches the TUI. Core packages (`config/`, `auth/`, `appinsights/`, `logging/`) are active; `tui/` is being rebuilt. See `docs/chat-first-ui-rewrite.md` for layout/keys.

Do-first for agents
- Plan with the Todos tool; keep one active item at a time.
- Quality gate: run `make all` (lint → race tests → build). Zero warnings; tests green.
- Research first using Context7/Perplexity and official Charm docs before changing OAuth, logging, or Bubble Tea patterns.
- CRITICAL: do not run interactive TUI. Use `./bc-insights-tui.exe -run=...` only; ask the user to test interactive flows.

Architecture highlights
- Dynamic telemetry: never hard-code Business Central schemas. `eventId` determines `customDimensions`; build table columns from API response columns.
- Packages
  - `config/`: precedence = defaults → JSON file (`%AppData%/bc-insights-tui/config.json`) → env (BCINSIGHTS_*) → flags; helpers: `ValidateAndUpdateSetting`, `ListAllSettings`.
  - `auth/`: Azure OAuth2 Device Flow; stores refresh token in OS keyring (zalando/go-keyring). APIs: `InitiateDeviceFlow`, `PollForToken`, `SaveTokenSecurely`, `StoredRefreshTokenPresent`, `GetTokenForScopes`.
  - `appinsights/`: `ExecuteQuery(ctx, kql)` → https://api.applicationinsights.io/v1/apps/{appId}/query; dynamic `Columns`/`Rows`; `ValidateQuery` preflight.
  - `logging/`: daily file `logs/bc-insights-tui-YYYY-MM-DD.log`; enable stdout mirroring with `BC_INSIGHTS_LOG_TO_STDOUT=true`. Level via `BC_INSIGHTS_LOG_LEVEL`.

Build/test workflows
- Make targets: `make build`, `make test`, `make race`, `make lint`, `make all`.
- Windows run target: `./bc-insights-tui.exe -run=COMMAND` (mandatory for automation/AI).

Non-interactive commands (examples)
- `-run=login` (device flow, persist refresh token)
- `-run=login-status` (diagnose token presence; silent ARM refresh)
- `-run=subs` (list subscriptions) · `-run=resources` (list App Insights in subscription)
- `-run=config` · `-run=config-save` · `-run=config-reset` · `-run=config-path`
- `-run=keyring-info` · `-run=keyring-test` (diagnose OS keyring)
- `-run=logs[:N]` (tail latest log; default 200)

Config and env conventions
- Basics: `LOG_FETCH_SIZE`, `BCINSIGHTS_ENVIRONMENT`, `BCINSIGHTS_APP_INSIGHTS_ID`, `BCINSIGHTS_APP_INSIGHTS_KEY`, subscription via `AZURE_SUBSCRIPTION_ID` (or `BCINSIGHTS_AZURE_SUBSCRIPTION_ID`/`ARM_SUBSCRIPTION_ID`).
- OAuth2: `BCINSIGHTS_OAUTH2_TENANT_ID`, `BCINSIGHTS_OAUTH2_CLIENT_ID`, `BCINSIGHTS_OAUTH2_SCOPES` (comma-separated). Device flow ensures `openid profile offline_access`.
- Logging: `BC_INSIGHTS_LOG_LEVEL=DEBUG|INFO|WARN|ERROR`, `BC_INSIGHTS_LOG_TO_STDOUT=true` (opt-in).
- Keyring overrides: `BCINSIGHTS_KEYRING_SERVICE`, `BCINSIGHTS_KEYRING_NAMESPACE`.

Project-specific rules
- Errors must be actionable (hint next steps; map HTTP/Kusto errors). Do not log secrets or full KQL; log query hashes and timings.
- Heavily use logging for state transitions and config changes (old → new). Follow `logging/` patterns.
- KQL/UI are dynamic: parse result columns; keep Bubble Tea MVC (`tui/model.go`, `update.go`, `view.go`); avoid global state.

Key references
- `main.go`, `config/config.go`, `auth/authenticator.go`, `appinsights/client.go`, `logging/logger.go`, docs: `docs/authentication.md`, `docs/chat-first-ui-rewrite.md`, `makefile`.

Acceptance before PR
- `make all` passes; no lint warnings; tests (incl. race) green. Update/add focused tests when public behavior changes.
