---
applyTo: '**'
---

# Repo Memory

- Project: bc-insights-tui (Go, Charm Bubble Tea ecosystem)
- Current focus: frequent re-login investigation; added non-interactive diagnostics.
- Auth flow: Azure AD (Entra) OAuth2 device code flow; tokens stored in OS keyring (go-keyring). Refresh via v2 scopes; AI via v1 resource.
- New commands: -run=login-status (diagnose token presence and silent refresh); -run=logs[:N].
- Likely root cause of frequent logins: missing/evicted refresh token in keyring (Windows Credential Manager), not CA policy.
- Next steps: build/test login-status, validate persistence across restarts, document troubleshooting.

## Conversation summary (essentials)
- Ask: Research why the app requires frequent logins (<1h) and implement diagnostics; user confirms no CA policies enforcing sign-in frequency and consistent terminal usage.
- Research outcome: Access tokens typically 60–90 min; refresh tokens ~90-day sliding expiration. Frequent prompts likely due to missing/cleared refresh token or audience/scope mismatch, not policy.
- Code review: auth/authenticator.go handles device flow, persists token via keyring. Added ensureLoginScopes to always include offline_access/openid/profile and ARM scope; added StoredRefreshTokenPresent(); added token-for-scopes exchange and AI v1 token path.
- CLI additions: main.go non-interactive command 'login-status' prints user/process context, checks keyring presence, attempts silent refresh for ARM scope, and hints to view logs; 'logs[:N]' tails latest log.
- Build/test: make all passes clean. Running -run=login-status works but config flag parser prints "flag provided but not defined: -run" banner (from config OsFlagParser). Diagnostics show "Stored refresh token: not found" on test system.
- Hypothesis: Keyring entry not being created/persisted (Windows Credential Manager issue), causing app to re-prompt. Need to verify after a successful 'login' that refresh token is stored and survives restarts.
- Follow-ups: (1) Silence unknown flags in config OsFlagParser (avoid noisy banner), (2) Ask user to run: -run=login then -run=login-status; then restart app and re-run login-status. (3) Document troubleshooting and expected lifetimes.
