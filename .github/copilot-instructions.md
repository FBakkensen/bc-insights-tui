# Copilot Instructions for bc-insights-tui

## Development Workflow & Standards (CRITICAL)

### Todos
It is critical you always use the todos tool to plan and track your work, even if there is only 1 or 2 tasks. This ensures nothing is overlooked and helps maintain focus on priorities.

### ⚠️ MANDATORY: Clean Build & Linting
**BEFORE ANY CODE SUBMISSION OR RESPONSE**, you MUST run the complete linting suite and ensure a clean build:
```bash
make all
```
This command runs the full pipeline: lint, race tests, and build.

**CRITICAL REQUIREMENTS:**
- ✅ **`make all` MUST complete without ANY errors or warnings**
- ✅ All linting must pass with ZERO warnings or errors
- ✅ Code must build successfully with `go build`
- ✅ All tests must pass with `go test -race ./...`

**Linting Configuration**: `.golangci.yml` includes strict rules with exceptions for TUI patterns (disabled `fieldalignment` for TUI models, allows embedding in TUI components).

**Environment Setup**: `.github/workflows/copilot-setup-steps.yml` configures the Copilot coding agent environment with golangci-lint v2.3.1 and Go 1.24 pre-installed.

### ⚠️ MANDATORY: Go Code Standards
Follow the comprehensive Go instructions in `.github/instructions/go.instructions.md`. Key patterns specific to this project:
- **Thread-safe Config**: Use `sync.RWMutex` for configuration changes (see `config/config.go`)
- **OAuth2 Token Security**: Store tokens in OS keyring via `github.com/zalando/go-keyring` (see `auth/authenticator.go`)
- **Bubble Tea Async**: Use proper `tea.Cmd` patterns for long-running operations like auth polling

### ⚠️ MANDATORY: Documentation Research
**BEFORE implementing any feature or fixing configuration issues**, you MUST use Context7 MCP to get up-to-date documentation:
```
1. Use mcp_context7_resolve-library-id to find the correct library
2. Use mcp_context7_get-library-docs to get current documentation
3. Verify configuration/implementation against latest docs
```

**Examples:**
- Before changing `.golangci.yml` → Search for `golangci-lint` documentation
- Before implementing OAuth2 → Search for `azure` or `oauth2` documentation
- Before using Bubble Tea patterns → Search for `bubbletea` or `charm` documentation

### Phase-Based Development
1. ✅ **Phase 1**: Basic TUI skeleton and configuration (COMPLETE)
2. 🚧 **Phase 2**: Azure OAuth2 authentication with device flow (auth/authenticator.go implementation exists)
3. 🚧 **Phase 3**: Application Insights integration (appinsights/client.go placeholder)
4. 🚧 **Phase 4**: Advanced features (KQL editor, saved queries, dynamic columns)
5. 🚧 **Phase 5**: AI-powered KQL generation (ai/assistant.go placeholder)

## Project Overview

This is a Go-based Terminal User Interface for Azure Application Insights, specifically designed for Business Central developers. The project uses the Charm Bracelet ecosystem (Bubble Tea, Lip Gloss, Bubbles) for TUI components.

**Current State**: Phase 1 complete (basic TUI skeleton). The project has a working Bubble Tea foundation with configuration loading and complete OAuth2 device flow implementation ready for integration.

## Architecture Principles

### 1. Dynamic Data Model (CRITICAL)
Business Central telemetry structure is determined by the `eventId` field, which defines the schema of `customDimensions`. **Never use static data models for log entries**. Always design components to dynamically parse and display key-value pairs based on the `eventId`.

### 2. Command Palette Pattern
The UI is built around a keyboard-driven command palette (Ctrl+P) rather than traditional menus. All user actions flow through commands:
- `ai: <natural language query>` - AI-powered KQL generation
- `filter: <text>` - Quick text filtering
- `set <setting>=<value>` - Configuration changes

### 3. Bubble Tea MVC Pattern
Follow the established three-file pattern in `tui/`:
- `model.go` - State and data structures with authentication state
- `update.go` - Event handling with async `tea.Cmd` patterns for auth flow
- `view.go` - Rendering logic with state-aware UI

**Key Implementation Details**:
- `Model` struct includes `Config`, `AuthState`, and `Authenticator` fields
- Authentication commands use context with timeouts (30s for device flow, 15m for polling)
- Command palette uses string input with real-time character handling
- Thread-safe config updates via `Config.ValidateAndUpdateSetting()`

## Package Structure & Responsibilities

```
main.go              # Entry point: config loading → TUI initialization
tui/                 # Bubble Tea UI components (model.go, update.go, view.go)
auth/                # Complete OAuth2 Device Authorization Flow implementation
appinsights/         # Application Insights API client (placeholder)
ai/                  # AI service integration for KQL generation (placeholder)
config/              # Environment-based configuration with thread-safety
```

**Configuration Precedence** (highest to lowest):
1. Command line flags (`--fetch-size`, `--environment`, `--app-insights-key`)
2. Environment variables (`LOG_FETCH_SIZE`, `BCINSIGHTS_*`)
3. Configuration files (`config.json`, `.bc-insights-tui.json`)
4. Default values

## Business Central Telemetry Context

- **Primary data source**: `traces` table in Application Insights
- **Critical field**: `customDimensions` contains the most valuable log context
- **Schema definition**: `eventId` determines the structure of `customDimensions`
- **Dynamic nature**: Event schemas change with BC releases and custom partner events

Example event structure:
```json
{
  "eventId": "RT0019",
  "customDimensions": {
    "alHttpStatus": "404",
    "alUrl": "https://api.example.com/data"
  }
}
```

## Implementation Requirements

### Authentication Flow (Implemented)
The OAuth2 device flow is fully implemented in `auth/authenticator.go`:
- Device code request → User authentication → Token polling → Secure storage
- Tokens stored in OS keyring using `github.com/zalando/go-keyring`
- Automatic token refresh with context timeouts
- State management: `AuthStateUnknown`, `AuthStateRequired`, `AuthStateInProgress`, `AuthStateCompleted`, `AuthStateFailed`

### Error Handling
All authentication and API errors must be user-friendly with actionable guidance:
- ❌ "Authentication failed"
- ✅ "Authentication failed. Check Azure permissions and verify your credentials at https://portal.azure.com"

### UI Constraints
- **Target Platform**: Windows (primary)
- **Online-Only**: No offline mode, no local log caching
- **Memory**: Low memory footprint requirement
- **Dynamic Columns**: KQL `project` statements must dynamically adjust table columns

### Key UI Patterns (from TUI mockup)
- Device auth flow with clear instructions and spinner
- Main log table with navigation (↑↓ keys, Enter for details)
- Modal overlay for log details (preserves background context)
- Command palette input with real-time feedback
- Settings changes via `set` commands with confirmation

**Command Examples**:
- `set fetchSize=100` - Updates log fetch size with validation
- `auth logout` - Clears stored tokens and resets auth state
- `auth` - Shows current authentication status

When implementing new features, prioritize the command palette workflow and ensure dynamic handling of Business Central's flexible telemetry schema.
