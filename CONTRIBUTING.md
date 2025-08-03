# Contributing to bc-insights-tui

Thank you for your interest in contributing to bc-insights-tui! This project aims to provide a powerful Terminal User Interface for Azure Application Insights, specifically tailored for Business Central developers.

## Development Setup

### Prerequisites
- Go 1.21 or later
- golangci-lint for code quality checks
- Access to Azure Application Insights (for testing)

### Getting Started
1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/bc-insights-tui.git`
3. Install dependencies: `go mod download`
4. Run the application: `go run main.go`

## Development Workflow

### Code Quality Requirements (MANDATORY)
Before submitting any code, you **MUST** run the complete linting suite:

```powershell
go fmt ./...; go vet ./...; golangci-lint run --fast
```

**All submissions must:**
- ✅ Pass all linting with ZERO warnings or errors
- ✅ Build successfully with `go build`
- ✅ Pass all tests with `go test ./...`

### Project Phases
The project follows a phase-based development approach:
1. ✅ **Phase 1**: Basic TUI skeleton (Complete)
2. 🚧 **Phase 2**: Azure OAuth2 authentication
3. 🚧 **Phase 3**: Application Insights integration
4. 🚧 **Phase 4**: Advanced features (KQL editor, saved queries)
5. 🚧 **Phase 5**: AI-powered KQL generation

## Architecture Guidelines

### Key Principles
- **Dynamic Data Model**: Business Central telemetry varies by `eventId` - never use static models
- **Command Palette Pattern**: All user actions flow through keyboard commands (Ctrl+P)
- **Bubble Tea MVC**: Follow the three-file pattern (`model.go`, `update.go`, `view.go`)

### Package Structure
- `main.go` - Entry point and configuration loading
- `tui/` - Bubble Tea UI components
- `auth/` - OAuth2 Device Authorization Flow
- `appinsights/` - Application Insights API client
- `ai/` - AI service integration
- `config/` - Environment-based configuration

## Contribution Types

### Bug Reports
When reporting bugs, please include:
- Go version and OS
- Steps to reproduce
- Expected vs actual behavior
- Log output (if applicable)

### Feature Requests
For new features:
- Describe the use case
- Consider which development phase it fits into
- Provide mockups or examples if UI-related

### Code Contributions

#### Pull Request Process
1. Create a feature branch: `git checkout -b feature/your-feature-name`
2. Make your changes following the coding standards
3. Add tests for new functionality
4. Run the full linting suite (see requirements above)
5. Update documentation if needed
6. Submit a pull request with a clear description

#### Coding Standards
- Follow standard Go conventions
- Use meaningful variable and function names
- Add comments for complex logic
- Handle errors gracefully with user-friendly messages
- Maintain the established package structure

#### UI Guidelines
- Target Windows as primary platform
- Design for keyboard-first interaction
- Ensure dynamic column handling for KQL results
- Provide clear user feedback for long operations
- Follow the command palette workflow pattern

## Business Central Context

When working with telemetry data:
- `eventId` determines the schema of `customDimensions`
- Always handle dynamic key-value pairs
- Consider that schemas change with BC releases
- Focus on the `traces` table in Application Insights

## Testing

### Running Tests
```bash
go test ./...
```

### Test Guidelines
- Write unit tests for new functions
- Mock external dependencies (Azure APIs)
- Test both success and error scenarios
- Ensure tests run quickly (< 1 second each)

## Getting Help

- Check existing issues for similar problems
- Review the documentation in `docs/`
- Look at the TUI mockup for UI guidance
- Ask questions in issue discussions

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help newcomers get started
- Maintain a professional tone in all interactions

Thank you for contributing to bc-insights-tui!
