# Copilot Instructions for bc-insights-tui

## Project Architecture

This is a Go-based Terminal User Interface for Azure Application Insights, specifically designed for Business Central developers. The project uses the Charm Bracelet ecosystem (Bubble Tea, Lip Gloss, Bubbles) for TUI components.

### Core Architecture Principles

1. **Dynamic Data Model**: The application CANNOT use static data models for log entries. Business Central telemetry structure is determined by the `eventId` field, which defines the schema of `customDimensions`. Always design components to dynamically parse and display key-value pairs based on the `eventId`.

2. **Command Palette Pattern**: The UI is built around a keyboard-driven command palette (Ctrl+P) rather than traditional menus. All user actions flow through commands like:
   - `ai: <natural language query>` - AI-powered KQL generation
   - `filter: <text>` - Quick text filtering
   - `set <setting>=<value>` - Configuration changes

3. **Modular Package Structure**:
   ```
   tui/         # Bubble Tea UI components (model.go, update.go, view.go)
   auth/        # OAuth2 Device Authorization Flow
   appinsights/ # Application Insights API client
   ai/          # AI service integration for KQL generation
   config/      # Application configuration
   ```

### Key Implementation Patterns

- **Error Handling**: All authentication and API errors must be user-friendly with actionable guidance (e.g., "Check Azure permissions", "Verify config value")
- **Dynamic Table Columns**: When KQL queries use `project` statements, the log table must dynamically adjust columns to match the selected fields
- **No Local Storage**: All data is fetched on-demand from Application Insights API. No caching or local log storage
- **Pagination**: Implement "load more" functionality for fetching older log batches

### Business Central Telemetry Specifics

- Primary data source: `traces` table in Application Insights
- Critical field: `customDimensions` contains the most valuable log context
- Schema definition: `eventId` determines the structure of `customDimensions`
- Dynamic nature: Event schemas can change with BC releases and custom partner events

### Development Workflow

The project follows a phased development approach:
1. **Phase 1**: Basic TUI skeleton and configuration
2. **Phase 2**: Azure OAuth2 authentication with device flow
3. **Phase 3**: Application Insights integration with dynamic parsing
4. **Phase 4**: Advanced features (KQL editor, saved queries, dynamic columns)
5. **Phase 5**: AI-powered natural language to KQL translation

### Technology Constraints

- **Target Platform**: Windows (primary)
- **Authentication**: OAuth2 Device Authorization Flow only
- **Online-Only**: No offline mode support
- **Memory**: Low memory footprint requirement
- **AI Integration**: Azure OpenAI for KQL query generation

When implementing features, always consider the command palette workflow and ensure dynamic handling of Business Central's flexible telemetry schema.
