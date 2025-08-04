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

### Configuration

The application uses environment variables with fallback defaults:

```powershell
# Required (when auth is implemented)
$env:AZURE_CLIENT_ID = "your-azure-app-client-id"
$env:AZURE_TENANT_ID = "your-azure-tenant-id"
$env:APPINSIGHTS_APP_ID = "your-application-insights-app-id"

# Optional
$env:LOG_FETCH_SIZE = "100"  # Default: 50
```

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
- `Ctrl+C` - Exit application

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

## 🛠️ Development

### Development Workflow

**MANDATORY**: Before any code submission, run the complete linting suite:

```powershell
go fmt ./... && go vet ./... && golangci-lint run --fast
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
