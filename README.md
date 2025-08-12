# BC Insights TUI

A Terminal User Interface for Azure Application Insights, specifically designed for Microsoft Dynamics 365 Business Central developers. Built with Go and the Charm Bracelet TUI ecosystem.

## 🚀 Quick Start

### 1. Clone and Setup
```bash
git clone https://github.com/FBakkensen/bc-insights-tui.git
cd bc-insights-tui

# Install Git hooks (prevents commits to main branch)
./install-hooks.sh          # Unix/Linux/macOS
# OR
.\install-hooks.ps1         # Windows PowerShell
```

### 2. Build and Run
```bash
go build
./bc-insights-tui           # Unix/Linux/macOS
# OR
.\bc-insights-tui.exe       # Windows
```

## 🎯 Project Overview

BC Insights TUI provides a keyboard-driven, command palette-based interface for querying and analyzing Business Central telemetry data stored in Azure Application Insights. The tool is optimized for the dynamic nature of Business Central's telemetry schema, where event structure is determined by the `eventId` field.

## 🚀 Features

### Current (Phase 1 - Complete)
- ✅ Basic TUI skeleton with Bubble Tea foundation
- ✅ Environment-based configuration system
- ✅ Command palette architecture (Ctrl+P)

### Planned Development Phases
- 🚧 **Phase 2**: Azure OAuth2 authentication with device flow
- 🚧 **Phase 3**: Application Insights API integration
- 🚧 **Phase 4**: Advanced features (KQL editor, saved queries, dynamic columns)
- 🚧 **Phase 5**: AI-powered KQL generation

## 🏗️ Architecture

### Core Principles

1. **Dynamic Data Model**: Handles Business Central's flexible telemetry schema where `eventId` determines `customDimensions` structure
2. **Command Palette Pattern**: All interactions through keyboard-driven commands rather than traditional menus
3. **Bubble Tea MVC**: Clean separation with `model.go`, `update.go`, and `view.go` patterns

### Package Structure

```
main.go              # Entry point: config loading → TUI initialization
tui/                 # Bubble Tea UI components
├── model.go         # State and data structures
├── update.go        # Event handling and state transitions
└── view.go          # Rendering logic
auth/                # OAuth2 Device Authorization Flow
appinsights/         # Application Insights API client
ai/                  # AI service integration for KQL generation
config/              # Environment-based configuration
```

## 🔧 Installation & Setup

### Prerequisites

- Go 1.21 or later
- Access to Azure Application Insights with Business Central telemetry
- Windows (primary target platform)

### Build from Source

```powershell
git clone https://github.com/FBakkensen/bc-insights-tui.git
cd bc-insights-tui
go build
```

### Available Make Commands

The project includes a Makefile for standardized build commands that AI coding agents can easily recognize:

```bash
make help    # Show available commands
make build   # Build the application
make test    # Run tests
make race    # Run tests with race detection
make lint    # Run complete quality checks (fmt, vet, golangci-lint)
make clean   # Clean build artifacts
make all     # Run lint, race, and build (default)
```

**For AI Coding Agents**: Use these standardized commands for consistent build/test/lint operations across different development environments.

### Automation compliance (AI agents)

AI agents must not run the interactive TUI. To comply with organizational automation standards and to avoid hanging on interactive input, always use the non-interactive runner with -run=COMMAND.

Example (non-interactive invocations):

```bash
# Authenticate using device flow without launching the TUI
./bc-insights-tui.exe -run=login

# List subscriptions (prints to stdout); do NOT start interactive UI
./bc-insights-tui.exe -run=subs

# After selecting and saving config via non-interactive commands, you can run other commands similarly
# e.g., future: ./bc-insights-tui.exe -run=kql:"traces | take 5"

# Tail latest logs without touching stdout mirroring (useful for debugging)
./bc-insights-tui.exe -run=logs        # last 200 lines
./bc-insights-tui.exe -run=logs:500    # last 500 lines

# Diagnostics for auth/keyring (non-interactive)
./bc-insights-tui.exe -run=login-status   # check refresh token presence and attempt a silent refresh
./bc-insights-tui.exe -run=keyring-info   # show effective keyring service/key and env overrides
./bc-insights-tui.exe -run=keyring-test   # write/read/delete a temporary credential to validate keyring access
```

Never invoke ./bc-insights-tui (without -run) from automation or AI tooling.

## 🧭 Commands quick reference

Non-interactive commands (use with `-run=`):

- `-run=login` – Start device flow sign-in and persist refresh token
- `-run=subs` – List Azure subscriptions
- `-run=resources` – List Application Insights resources for the configured subscription
- `-run=config` – Print current configuration values
- `-run=config-save` – Save current in-memory configuration to file
- `-run=config-reset` – Delete the saved config file (revert to defaults on next run)
- `-run=config-path` – Show the resolved config file path and whether it exists
- `-run=login-status` – Check refresh token presence and attempt a silent ARM token refresh
- `-run=keyring-info` – Show effective keyring service/key and env overrides
- `-run=keyring-test` – Write/read/delete a temporary keyring credential to validate access
- `-run=logs[:N]` – Tail last N lines from the latest log file (default 200)

Tip: for auth flow and troubleshooting, see `docs/overview.md` and the non-interactive diagnostics commands (`-run=login-status`, `-run=keyring-info`, `-run=keyring-test`).

### Configuration

The application uses environment variables with fallback defaults:

```powershell
# Required (when auth is implemented)
$env:AZURE_CLIENT_ID = "your-azure-app-client-id"
$env:AZURE_TENANT_ID = "your-azure-tenant-id"
$env:APPINSIGHTS_APP_ID = "your-application-insights-app-id"

# Optional
$env:LOG_FETCH_SIZE = "100"  # Default: 50

# Keyring overrides (advanced; for testing/diagnostics)
# BCINSIGHTS_KEYRING_SERVICE fully overrides the credential service name used in the OS keyring
# BCINSIGHTS_KEYRING_NAMESPACE appends a suffix to the default service name (bc-insights-tui-<namespace>)
$env:BCINSIGHTS_KEYRING_SERVICE = "bc-insights-tui"
$env:BCINSIGHTS_KEYRING_NAMESPACE = "dev"
```

See `docs/overview.md` for the current architecture and non-interactive commands for OAuth2 login, token storage checks, and troubleshooting frequent sign-ins.

## 🎮 Usage

### Command Palette

Press `Ctrl+P` to open the command palette and use these commands:

- `ai: <natural language query>` - AI-powered KQL generation
- `filter: <text>` - Quick text filtering
- `set <setting>=<value>` - Configuration changes

### Navigation

- `↑↓` - Navigate log entries
- `Enter` - View detailed log entry
- `Esc` - Close modals/return to main view
- `Ctrl+Q` - Exit application
- `Ctrl+C` - Force exit application

### Single-line KQL (Step 5)

You can run Application Insights Kusto queries directly from the chat input:

- Type: `kql: <your KQL>` and press Enter.
- The top panel shows a snapshot table with up to your configured fetch size and a summary line.
- Press F6 to open the results in an interactive table (use arrow keys to navigate, Esc to return).

Requirements:
- Be authenticated (`login`).
- Set an Application Insights App ID (`config set applicationInsightsAppId=<id>` or pick from resources).

Errors are mapped to actionable hints (401/403/400/429, timeouts) and logs include a query hash, not the full text.

### Multi-line KQL editor (Step 6)

When you want to compose multi-line Kusto queries interactively:

- Type `edit` in the chat input and press Enter to open the editor.
- The prompt changes to `KQL> ` and the input becomes multi-line.
- Keys while editing:
  - Enter inserts a newline
  - Ctrl+Enter submits the query (Ctrl+M in some terminals)
  - Esc cancels and returns to chat
- On submit, the first line is echoed with an ellipsis and the query runs.
- After results complete, you’ll see a summary, a compact table snapshot, and a hint: “Press F6 to open interactively.”
- Resize dynamically adjusts the editor and output panel heights.

Tip: For quick one-liners, keep using `kql: ...`. For anything longer or pipelined, use `edit`.

### Debug logs

To enable detailed debug logging while writing to a daily file under `logs/`:

- Set environment variable `BC_INSIGHTS_LOG_LEVEL=DEBUG` before starting the app.
- Logs are written to `logs/bc-insights-tui-YYYY-MM-DD.log`.
- Logs are not printed to stdout by default. To mirror logs to stdout, explicitly set `BC_INSIGHTS_LOG_TO_STDOUT=true` (opt-in).
- KQL execution logs preflight, request timing, HTTP status codes, and response metadata (request IDs). Secrets and the full query text are not logged.

### App Insights raw capture (advanced)

You can optionally capture the last KQL HTTP request/response to a YAML file for deep diagnostics. It’s disabled by default.

- Enable: set `BCINSIGHTS_AI_RAW_ENABLE=true`
- Path: override with `BCINSIGHTS_AI_RAW_FILE` (default `logs/appinsights-raw.yaml`)
- Size cap: set `BCINSIGHTS_AI_RAW_MAX_BYTES` per body (default 1048576; 0 = unlimited)

Notes
- The file is atomically overwritten for each request and may include your KQL text. Treat it as sensitive.
- Daily logs record when the feature toggles or path changes, and when a capture is written.

## 🔍 Business Central Telemetry Context

This tool is specifically designed for Business Central telemetry data structure:

- **Primary source**: `traces` table in Application Insights
- **Key field**: `customDimensions` contains the most valuable context
- **Dynamic schema**: Event structure varies based on `eventId`

Example telemetry structure:
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

  ## 📊 Dynamic Column Ranking (Step 10)

  When displaying tabular query results the tool dynamically ranks custom dimension keys so the most useful columns appear first without manual configuration. This addresses the highly variable Business Central telemetry schema.

  Scoring dimensions (normalized per key on sampled rows):
  1. Presence rate (non-empty occurrences / sampled rows)
  2. Variability (distinct value count up to a cap)
  3. Length penalty (shorter average values favored; very large average length penalized)
  4. Type / heuristic bias (boolean-like and small controlled vocabularies boosted)
  5. Keyword / regex boosts (semantic patterns like request*, *status*, *error*, *duration*, user/session, IDs)
  6. Optional AL* prefix rule with presence-weighted boost (highlights AL-specific fields)

  Pinned columns (exact names, case-insensitive) are forced to the front (after primaries timestamp, message, eventId) in the specified order before remaining ranked keys.

  ### Configuration (file or JSON)
  Fields in `config.Config` (defaults in parentheses):
  - rank.enable (true)
  - rank.sampleSize (200) – max rows sampled for statistics
  - rank.distinctCap (50) – cap for variability normalization
  - rank.lenCap (200) – cap for average length normalization
  - rank.weightPresence (5.0)
  - rank.weightVariability (2.0)
  - rank.weightLenPenalty (-1.0) – negative reduces score as avg length grows
  - rank.weightType (0.5)
  - rank.regex ("") – custom regex spec appended to defaults
  - rank.pinned ("") – comma list (e.g. "companyName,environment,alObjectId")
  - rank.alPrefixBoost (3.0) – boost applied to ^al.* keys when present rule qualifies
  - rank.alMinPresence (0.05) – minimum presence rate before AL boost applies

  ### Environment Variables
  All optional; omit to use defaults.
  ```
  BCINSIGHTS_RANK_ENABLE=true|false
  BCINSIGHTS_RANK_SAMPLE_SIZE=200
  BCINSIGHTS_RANK_DISTINCT_CAP=50
  BCINSIGHTS_RANK_LEN_CAP=200
  BCINSIGHTS_RANK_WEIGHT_PRESENCE=5.0
  BCINSIGHTS_RANK_WEIGHT_VARIABILITY=2.0
  BCINSIGHTS_RANK_WEIGHT_LEN_PENALTY=-1.0
  BCINSIGHTS_RANK_WEIGHT_TYPE=0.5
  BCINSIGHTS_RANK_REGEX="(?i)alTenant=4;(?i)^cust.*=2"   # format: pattern=boost;pattern=boost OR JSON {"pattern":boost,...}
  BCINSIGHTS_RANK_PINNED="companyName,environment"
  BCINSIGHTS_RANK_AL_PREFIX_BOOST=3.0
  BCINSIGHTS_RANK_AL_MIN_PRESENCE=0.05
  ```

  Regex spec formats:
  - Delimited: `pattern=boost;pattern2=boost` (floats allowed)
  - JSON object: `{"(?i)^foo":2,"(?i)bar$":1.5}`
  Invalid fragments are ignored with a log entry.

  ### Fallback & Safety
  If ranking is disabled or errors occur (including a panic) the system falls back to a deterministic alphabetical ordering of discovered keys. Sampling keeps performance predictable on large result sets.

  ### Diagnostics
  Logs (INFO) include: sample size, total keys, and the top scored keys with component metrics. Enable debug logging for timing details.

  ### Typical Use
  Leave defaults; optionally pin high-priority business identifiers or add custom regex boosts for domain-specific fields.


## 🛠️ Development

### Development Workflow

**MANDATORY**: Before any code submission, run the complete linting suite:

```powershell
go fmt ./... && go vet ./... && golangci-lint run
```

**Requirements**:
- ✅ All linting must pass with ZERO warnings or errors
- ✅ Code must build successfully with `go build`
- ✅ All tests must pass with `go test ./...`

### Code Standards

- Follow the established Bubble Tea MVC pattern
- Design for dynamic data handling (no static Business Central models)
- Prioritize command palette workflow
- Ensure user-friendly error messages with actionable guidance

### Linting Configuration

The project uses `.golangci.yml` with strict rules and specific exceptions for TUI patterns (disabled `fieldalignment` for TUI models, allows embedding in TUI components).

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Follow the development workflow and ensure all linting passes
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙋 Support

For Business Central telemetry questions or tool usage:
- Create an issue on GitHub
- Check the [docs/](docs/) folder for additional documentation

## 🏷️ Project Status

**Current Phase**: 1 (Complete - Basic TUI skeleton)
**Next Milestone**: Phase 2 - Azure OAuth2 Authentication

This is an active development project specifically tailored for Business Central developers working with Azure Application Insights telemetry data.
