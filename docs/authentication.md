# Authentication, Token Storage, and Troubleshooting

This app uses Azure AD (Entra ID) OAuth2 Device Authorization Flow. Access tokens are short-lived; a long-lived refresh token is stored securely in the OS keyring so you shouldn't need to sign in frequently.

## How sign-in works (device flow)

- The app requests a device code and shows you a verification URL and short user code.
- You open the URL in a browser, enter the code, and approve access for the app.
- Azure returns an access token and a refresh token. The app saves only the refresh token (smaller, safer footprint) in your OS keyring.

Scopes requested include:
- openid, profile, offline_access (ensures an ID token and a refresh token)
- https://management.azure.com/user_impersonation (ARM)

For Application Insights queries, the app exchanges the refresh token to obtain:
- ARM tokens via v2 endpoint (scopes)
- App Insights tokens via v1 endpoint (resource)

## Where the token is stored

- OS keyring (e.g., Windows Credential Manager)
- Service name: `bc-insights-tui` (can be overridden via env)
- Key: `oauth2-token`
- A best-effort backup entry is also written as `oauth2-token-backup` to help recover from sporadic key loss on some systems.

Env overrides (advanced):
- `BCINSIGHTS_KEYRING_SERVICE` – full override of the service name
- `BCINSIGHTS_KEYRING_NAMESPACE` – suffix appended to the default service name (e.g., `bc-insights-tui-dev`)

## Diagnostics (no TUI required)

Run the app with `-run=` to avoid interactive TUI mode:

- `-run=login` – Start device flow sign-in and persist the refresh token
- `-run=login-status` – Print process/user context, check if a refresh token is present, and attempt a silent refresh for ARM
- `-run=keyring-info` – Show the effective keyring service/key and any env overrides
- `-run=keyring-test` – Create/read/delete a temporary credential to validate OS keyring accessibility
- `-run=logs[:N]` – Print the last N lines from the latest log file (default 200)

## Expected token lifetimes (typical)

- Access token: ~60–90 minutes
- Refresh token: often ~90-day sliding expiry (subject to tenant policy)

You should not be prompted to sign in again frequently; the app uses the refresh token to get new access tokens silently.

## Troubleshooting frequent sign-ins

1) Check whether a refresh token exists
- `./bc-insights-tui.exe -run=login-status`
  - If it says "Stored refresh token: not found", run `-run=login`, then re-run `-run=login-status`.

2) Validate the OS keyring works from this process
- `./bc-insights-tui.exe -run=keyring-test`
  - WRITE/READ/DELETE should be OK. Intermittent failures may indicate OS profile/credential store issues.

3) Inspect effective keyring entry and overrides
- `./bc-insights-tui.exe -run=keyring-info`
  - Verify the service name matches your expectations. If you changed `BCINSIGHTS_KEYRING_*` env vars, ensure they are consistent across runs.

4) After a successful login, ensure persistence across restarts
- Run: `-run=login`
- Then: `-run=login-status` (should show "found")
- Close the terminal, start a new session, run `-run=login-status` again
  - If it flips to "not found", your OS keyring or profile isolation may be clearing entries between sessions.

5) Look at logs
- `./bc-insights-tui.exe -run=logs:300`
  - Search for keyring warnings or refresh errors. The app trims large responses and avoids secrets.

6) Consent or scope issues during refresh
- If you see errors like `consent_required` or `AADSTS65001`, run `-run=login` to grant updated scopes.

If problems persist, open an issue and include the output of `-run=login-status` and `-run=keyring-test` (no secrets included), plus the last 200–300 lines of logs.
