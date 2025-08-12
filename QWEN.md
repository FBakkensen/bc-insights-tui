# bc-insights-tui - Project Context for AI Assistants

## Project Overview

**bc-insights-tui** is a Terminal User Interface (TUI) application for Microsoft Dynamics 365 Business Central developers to query and analyze telemetry data stored in Azure Application Insights. It's built with Go and the Charm Bracelet TUI ecosystem, providing a keyboard-driven, command palette-based interface optimized for the dynamic nature of Business Central's telemetry schema.

### Core Purpose
- Provide a fast, efficient, and keyboard-driven interface for viewing Azure Application Insights logs
- Enable Business Central developers to debug applications without context switching
- Feature AI-assisted query building for generating KQL queries from natural language
- Run on Windows as the primary platform with a low memory footprint

### Key Technologies
- **Language**: Go
- **TUI Framework**: Bubble Tea
- **Styling & Components**: Lip Gloss & Bubbles
- **Authentication**: OAuth2 Device Authorization Flow
- **Azure Services**: Application Insights API, Azure Resource Manager API
- **AI Integration**: Azure OpenAI (planned)

## Architecture

### Package Structure
```
main.go              # Entry point: config loading → TUI initialization
tui/                 # Bubble Tea UI components
├── model.go         # State and data structures
├── update.go        # Event handling and state transitions
├── view.go          # Rendering logic
auth/                # OAuth2 Device Authorization Flow
appinsights/         # Application Insights API client
debugdump/           # Raw request/response capture for debugging
internal/
├── telemetry/       # Telemetry data processing helpers
├── util/            # Utility functions
config/              # Environment-based configuration
logging/             # Structured logging
```

### Core Principles
1. **Dynamic Data Model**: Handles Business Central's flexible telemetry schema where `eventId` determines `customDimensions` structure
2. **Command Palette Pattern**: All interactions through keyboard-driven commands rather than traditional menus
3. **Bubble Tea MVC**: Clean separation with `model.go`, `update.go`, and `view.go` patterns

## Development Workflow

### Prerequisites
- Go 1.21 or later
- golangci-lint for code quality checks
- Access to Azure Application Insights (for testing)

### Standard Commands (Makefile)
```bash
make help    # Show available commands
make build   # Build the application
make test    # Run tests
make race    # Run tests with race detection
make lint    # Run complete quality checks (fmt, vet, golangci-lint)
make clean   # Clean build artifacts
make all     # Run lint, race, and build (default)
```

### Code Quality Requirements (MANDATORY)
Before submitting any code, you MUST run the complete linting suite:
```bash
go fmt ./... && go vet ./... && golangci-lint run
```

All submissions must:
- ✅ Pass all linting with ZERO warnings or errors
- ✅ Build successfully with `go build`
- ✅ Pass all tests with `go test ./...`

## Building and Running

### Quick Start
```bash
# Clone and setup
git clone https://github.com/FBakkensen/bc-insights-tui.git
cd bc-insights-tui

# Install Git hooks (prevents commits to main branch)
./install-hooks.sh          # Unix/Linux/macOS
# OR
.\install-hooks.ps1         # Windows PowerShell

# Build and run
go build
./bc-insights-tui           # Unix/Linux/macOS
# OR
.\bc-insights-tui.exe       # Windows
```

### Non-Interactive Commands
For automation and diagnostics, use the `-run` flag:
```bash
# Authenticate using device flow without launching the TUI
./bc-insights-tui.exe -run=login

# List subscriptions
./bc-insights-tui.exe -run=subs

# List Application Insights resources
./bc-insights-tui.exe -run=resources

# Show current configuration
./bc-insights-tui.exe -run=config

# Tail latest logs
./bc-insights-tui.exe -run=logs        # last 200 lines
./bc-insights-tui.exe -run=logs:500    # last 500 lines

# Diagnostics for auth/keyring
./bc-insights-tui.exe -run=login-status
./bc-insights-tui.exe -run=keyring-info
./bc-insights-tui.exe -run=keyring-test
```

## Configuration

### Environment Variables
```bash
# Required (when auth is implemented)
AZURE_CLIENT_ID="your-azure-app-client-id"
AZURE_TENANT_ID="your-azure-tenant-id"
APPINSIGHTS_APP_ID="your-application-insights-app-id"

# Optional
LOG_FETCH_SIZE="100"  # Default: 50

# Keyring overrides (advanced; for testing/diagnostics)
BCINSIGHTS_KEYRING_SERVICE="bc-insights-tui"
BCINSIGHTS_KEYRING_NAMESPACE="dev"

# Debug logging
BC_INSIGHTS_LOG_LEVEL=DEBUG
BC_INSIGHTS_LOG_TO_STDOUT=true
```

### Configuration File
The application uses a JSON configuration file stored in the user's config directory:
```json
{
  "fetchSize": 100,
  "environment": "Production",
  "applicationInsightsKey": "INSTRUMENTATION_KEY_PLACEHOLDER",
  "applicationInsightsAppId": "APP_ID_PLACEHOLDER"
}
```

## Authentication Flow

The application uses OAuth2 Device Authorization Flow for Azure authentication:

1. User runs `login` command
2. Application initiates device flow with Azure AD
3. User opens verification URI and enters user code
4. Application polls for token until authentication completes
5. Refresh token is securely stored in OS keyring
6. Access tokens are acquired on-demand using the refresh token

## Application Insights Integration

### Key Features
- Execute KQL queries against Application Insights
- Dynamic schema handling for Business Central telemetry
- Automatic fetch size limiting (`| take N`)
- Column ranking for optimal display ordering
- Raw request/response capture for debugging

### Data Model
- **Primary source**: `traces` table in Application Insights
- **Key field**: `customDimensions` contains the most valuable context
- **Dynamic schema**: Event structure varies based on `eventId`
- **No static models**: UI adapts to whatever schema is returned

### KQL Execution
1. Validate query syntax (lightweight checks)
2. Apply fetch size limit if not explicitly set by user
3. Acquire Application Insights API token
4. Execute query with configured timeout
5. Parse response and build dynamic table
6. Display results with ranked columns

## UI Components

### Chat-First Interface
- Single-line input for commands
- Scrollback viewport for output
- Command palette pattern (Ctrl+P)

### Key Commands
- `login` - Start device flow authentication
- `subs` - List Azure subscriptions
- `resources` - List Application Insights resources
- `config` - Show configuration
- `kql: <query>` - Execute single-line KQL query
- `edit` - Open multi-line KQL editor
- `help` - Show help text

### Modes
1. **Chat Mode**: Default mode with single-line input
2. **Editor Mode**: Multi-line KQL editor (F6/Ctrl+Enter to run)
3. **List Mode**: Subscription/resource selection lists
4. **Table Mode**: Interactive results table (Esc to close)
5. **Details Mode**: Detailed view of a log entry

## Testing

### Running Tests
```bash
go test ./...                    # Run all tests
go test -race ./...             # Run with race detection
make test                       # Via Makefile
```

### Test Structure
- Unit tests for individual functions
- UI tests using Bubble Tea's testing facilities
- Mock external dependencies (Azure APIs)
- Test both success and error scenarios

## Logging

The application uses structured logging with multiple levels:
- DEBUG: Detailed diagnostic information
- INFO: General information about application progress
- WARN: Warning conditions
- ERROR: Error events that might still allow the application to continue

Logs are written to daily files in the `logs/` directory:
- `logs/bc-insights-tui-YYYY-MM-DD.log`

Enable debug logging:
```bash
BC_INSIGHTS_LOG_LEVEL=DEBUG ./bc-insights-tui
```

Mirror logs to stdout:
```bash
BC_INSIGHTS_LOG_TO_STDOUT=true ./bc-insights-tui
```

## Debugging Features

### Raw Capture
Optional capture of the last KQL HTTP request/response to a YAML file:
```bash
BCINSIGHTS_AI_RAW_ENABLE=true
BCINSIGHTS_AI_RAW_FILE="logs/appinsights-raw.yaml"
BCINSIGHTS_AI_RAW_MAX_BYTES=1048576
```

### Keyring Diagnostics
```bash
-run=keyring-info   # Show effective keyring service/key and env overrides
-run=keyring-test   # Write/read/delete a temporary credential to validate keyring access
```

## Business Central Telemetry Context

### Data Structure
```json
{
  "eventId": "RT0019",
  "customDimensions": {
    "alHttpStatus": "404",
    "alUrl": "https://api.example.com/data",
    "alObjectType": "Page",
    "alObjectId": "50001"
  }
}
```

### Key Characteristics
- **Primary source**: `traces` table in Application Insights
- **Dynamic schema**: Event structure varies based on `eventId`
- **Rich context**: Most valuable information in `customDimensions`
- **No hard-coded schemas**: Tool adapts to whatever keys are present

## Development Guidelines

### Code Standards
- Follow the established Bubble Tea MVC pattern
- Design for dynamic data handling (no static Business Central models)
- Prioritize command palette workflow
- Ensure user-friendly error messages with actionable guidance
- Handle errors gracefully with clear logging

### UI Guidelines
- Target Windows as primary platform
- Design for keyboard-first interaction
- Ensure dynamic column handling for KQL results
- Provide clear user feedback for long operations
- Follow the command palette workflow pattern

### Error Handling
- All errors must be user-friendly and actionable
- Include likely causes and next steps in error messages
- Log technical details for diagnostics
- Never expose secrets in logs or UI

## Project Roadmap

1. ✅ **Phase 1**: Basic TUI skeleton (Complete)
2. 🚧 **Phase 2**: Azure OAuth2 authentication (Complete)
3. 🚧 **Phase 3**: Application Insights integration (In progress)
4. 🚧 **Phase 4**: Advanced features (KQL editor, saved queries)
5. 🚧 **Phase 5**: AI-powered KQL generation

## Contributing

### Pull Request Process
1. Create a feature branch: `git checkout -b feature/your-feature-name`
2. Make your changes following the coding standards
3. Add tests for new functionality
4. Run the full linting suite
5. Update documentation if needed
6. Submit a pull request with a clear description

### Git Hooks
The repository includes shared Git hooks to maintain code quality:
- Pre-commit hook prevents direct commits to the `main` branch
- Installation: Run `./install-hooks.sh` (Unix) or `.\install-hooks.ps1` (Windows)