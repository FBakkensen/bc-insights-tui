.PHONY: build test lint clean fmt vet race all help ui-test ui-verify

# Default target
all: lint race build

# Build the application
build:
	@echo "🔨 Building bc-insights-tui..."
	go build -o bc-insights-tui.exe

# Run tests
test:
	@echo "🧪 Running tests..."
	go test ./...

# Run tests with race detection (as per your project requirements)
race:
	@echo "🧪 Running tests with race detection..."
	go test -race ./...

# Run complete linting suite (your mandatory requirement)
lint:
	@echo "🔍 Running quality checks..."
	go fmt ./... && go vet ./... && golangci-lint run

# Format code
fmt:
	@echo "📝 Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "🔬 Running go vet..."
	go vet ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f bc-insights-tui.exe bc-insights-tui.test.exe
	rm -rf ui_test_output ui_verification_reports custom_ui_output custom_reports

# Run UI tests specifically
ui-test:
	@echo "🎨 Running UI visual tests..."
	go test -v ./tui/ -run "TestUI.*"

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  test       - Run tests"
	@echo "  race       - Run tests with race detection"
	@echo "  lint       - Run complete quality checks (fmt, vet, golangci-lint)"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  clean      - Clean build artifacts and UI test outputs"
	@echo "  ui-test    - Run UI visual tests"
	@echo "  all        - Run lint, test, and build (default)"
	@echo "  help       - Show this help"