# Config Package Redesign: Proper Dependency Injection

## Current Problems
1. Direct filesystem coupling via `os` package calls
2. Hard-coded file paths and search logic
3. Impossible to test without affecting real user config files
4. Band-aid solutions with environment variables and path heuristics

## Solution: Filesystem Abstraction + Dependency Injection

### 1. Filesystem Interface
```go
// FileSystem abstracts filesystem operations for testability
type FileSystem interface {
    ReadFile(filename string) ([]byte, error)
    WriteFile(filename string, data []byte, perm os.FileMode) error
    Stat(filename string) (os.FileInfo, error)
    UserHomeDir() (string, error)
    Getwd() (string, error)
}

// OsFileSystem implements FileSystem using the real OS
type OsFileSystem struct{}

// MemFileSystem implements FileSystem using memory (for tests)
type MemFileSystem struct {
    files map[string][]byte
    dirs  map[string]bool
}
```

### 2. Config Loader with Dependency Injection
```go
// ConfigLoader handles configuration loading with injected dependencies
type ConfigLoader struct {
    fs          FileSystem
    searchPaths []string
    flagParser  FlagParser // Also abstracted for testing
}

// NewConfigLoader creates a ConfigLoader for production use
func NewConfigLoader() *ConfigLoader {
    return &ConfigLoader{
        fs: &OsFileSystem{},
        searchPaths: getDefaultSearchPaths(),
        flagParser: &OsFlagParser{},
    }
}

// NewTestConfigLoader creates a ConfigLoader for testing
func NewTestConfigLoader(fs FileSystem, searchPaths []string) *ConfigLoader {
    return &ConfigLoader{
        fs: fs,
        searchPaths: searchPaths,
        flagParser: &MockFlagParser{},
    }
}

// Load loads configuration using the injected filesystem
func (cl *ConfigLoader) Load() Config {
    return cl.LoadWithArgs(os.Args[1:])
}

func (cl *ConfigLoader) LoadWithArgs(args []string) Config {
    cfg := NewConfig()

    // Parse flags using injected parser
    flags := cl.flagParser.Parse(args)

    // Load from file using injected filesystem
    cl.loadFromFile(&cfg, flags.configFile)

    // Apply environment variables
    cl.loadFromEnv(&cfg)

    // Apply command line flags
    cl.applyFlags(&cfg, flags)

    return cfg
}
```

### 3. Clean Testing Interface
```go
func TestConfigLoading(t *testing.T) {
    // Create in-memory filesystem
    fs := NewMemFileSystem()
    fs.WriteFile("/test/config.json", []byte(`{"fetchSize": 100}`), 0644)

    // Create test loader with controlled dependencies
    loader := NewTestConfigLoader(fs, []string{"/test/config.json"})

    // Load config - completely isolated from real filesystem
    cfg := loader.LoadWithArgs([]string{})

    // Assert
    assert.Equal(t, 100, cfg.LogFetchSize)
}
```

### 4. Production Usage (Backward Compatible)
```go
// For existing code, keep the same interface
func LoadConfig() Config {
    loader := NewConfigLoader()
    return loader.Load()
}

func LoadConfigWithArgs(args []string) Config {
    loader := NewConfigLoader()
    return loader.LoadWithArgs(args)
}
```

## Benefits
1. **No environment variables needed** - Tests inject their own filesystem
2. **No path heuristics needed** - Tests control exactly what files exist
3. **Fast and reliable tests** - No real file I/O
4. **Backward compatible** - Existing code doesn't need to change
5. **Follows Go idioms** - Dependency injection via constructor parameters
6. **Composable** - Can easily add caching, validation, etc.

## Implementation Steps
1. Create filesystem abstraction interfaces
2. Implement OS and memory-based filesystems
3. Refactor ConfigLoader to use dependency injection
4. Update all tests to use test filesystem
5. Remove all band-aid environment variable logic
6. Keep public API unchanged for backward compatibility
