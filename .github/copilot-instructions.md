# Copilot Instructions for bc-insights-tui

## Development Workflow & Standards (CRITICAL)

### ⚠️ MANDATORY: Clean Build & Linting
**BEFORE ANY CODE SUBMISSION OR RESPONSE**, you MUST run the complete linting suite and ensure a clean build:
```powershell
go fmt ./...; go vet ./...; golangci-lint run --fast
```

**CRITICAL REQUIREMENTS:**
- ✅ All linting must pass with ZERO warnings or errors
- ✅ Code must build successfully with `go build`
- ✅ All tests must pass with `go test ./...`

**Linting Configuration**: `.golangci.yml` includes strict rules with exceptions for TUI patterns (disabled `fieldalignment` for TUI models, allows embedding in TUI components).

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
2. 🚧 **Phase 2**: Azure OAuth2 authentication with device flow (auth/authenticator.go placeholder)
3. 🚧 **Phase 3**: Application Insights integration (appinsights/client.go placeholder)
4. 🚧 **Phase 4**: Advanced features (KQL editor, saved queries, dynamic columns)
5. 🚧 **Phase 5**: AI-powered KQL generation (ai/assistant.go placeholder)

## Project Overview

This is a Go-based Terminal User Interface for Azure Application Insights, specifically designed for Business Central developers. The project uses the Charm Bracelet ecosystem (Bubble Tea, Lip Gloss, Bubbles) for TUI components.

**Current State**: Phase 1 complete (basic TUI skeleton). The project has a working Bubble Tea foundation with configuration loading, but auth, API client, and AI integration are placeholder files.

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
- `model.go` - State and data structures
- `update.go` - Event handling and state transitions
- `view.go` - Rendering logic

Current `Model` struct includes `Config` field for accessing settings.

## Package Structure & Responsibilities

```
main.go              # Entry point: config loading → TUI initialization
tui/                 # Bubble Tea UI components (model.go, update.go, view.go)
auth/                # OAuth2 Device Authorization Flow (placeholder)
appinsights/         # Application Insights API client (placeholder)
ai/                  # AI service integration for KQL generation (placeholder)
config/              # Environment-based configuration
```

**Configuration Pattern**: Environment variables with fallback defaults (see `config.LoadConfig()`). Example: `LOG_FETCH_SIZE` env var with default of 50.

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

When implementing new features, prioritize the command palette workflow and ensure dynamic handling of Business Central's flexible telemetry schema.
