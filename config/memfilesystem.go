package config

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemFileSystem implements FileSystem using in-memory storage.
// This is used for testing to avoid real filesystem dependencies.
type MemFileSystem struct {
	mu    sync.RWMutex
	files map[string][]byte
	dirs  map[string]bool

	// Mock system directories for testing
	homeDir    string
	configDir  string
	currentDir string
}

// NewMemFileSystem creates a new in-memory filesystem for testing
func NewMemFileSystem() *MemFileSystem {
	return &MemFileSystem{
		files:      make(map[string][]byte),
		dirs:       make(map[string]bool),
		homeDir:    "/home/testuser",
		configDir:  "/home/testuser/.config",
		currentDir: "/",
	}
}

// SetHomeDir sets the mock home directory for testing
func (fs *MemFileSystem) SetHomeDir(dir string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.homeDir = dir
}

// SetConfigDir sets the mock config directory for testing
func (fs *MemFileSystem) SetConfigDir(dir string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.configDir = dir
}

// SetCurrentDir sets the mock current directory for testing
func (fs *MemFileSystem) SetCurrentDir(dir string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.currentDir = dir
}

// ReadFile implements FileSystem.ReadFile using in-memory storage
func (fs *MemFileSystem) ReadFile(filename string) ([]byte, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filename = filepath.Clean(filename)
	if data, exists := fs.files[filename]; exists {
		return data, nil
	}
	return nil, &os.PathError{Op: "open", Path: filename, Err: os.ErrNotExist}
}

// WriteFile implements FileSystem.WriteFile using in-memory storage
func (fs *MemFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	filename = filepath.Clean(filename)

	// Ensure parent directories exist
	dir := filepath.Dir(filename)
	if dir != "." && dir != "/" {
		fs.dirs[dir] = true
	}

	fs.files[filename] = make([]byte, len(data))
	copy(fs.files[filename], data)
	return nil
}

// Stat implements FileSystem.Stat using in-memory storage
func (fs *MemFileSystem) Stat(filename string) (os.FileInfo, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	filename = filepath.Clean(filename)

	// Check if it's a file
	if _, exists := fs.files[filename]; exists {
		return &memFileInfo{
			name:  filepath.Base(filename),
			size:  int64(len(fs.files[filename])),
			isDir: false,
		}, nil
	}

	// Check if it's a directory
	if _, exists := fs.dirs[filename]; exists {
		return &memFileInfo{
			name:  filepath.Base(filename),
			size:  0,
			isDir: true,
		}, nil
	}

	return nil, &os.PathError{Op: "stat", Path: filename, Err: os.ErrNotExist}
}

// UserHomeDir implements FileSystem.UserHomeDir using mock value
func (fs *MemFileSystem) UserHomeDir() (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.homeDir, nil
}

// UserConfigDir implements FileSystem.UserConfigDir using mock value
func (fs *MemFileSystem) UserConfigDir() (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.configDir, nil
}

// Getwd implements FileSystem.Getwd using mock value
func (fs *MemFileSystem) Getwd() (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.currentDir, nil
}

// MkdirAll implements FileSystem.MkdirAll using in-memory storage
func (fs *MemFileSystem) MkdirAll(path string, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path = filepath.Clean(path)
	fs.dirs[path] = true
	return nil
}

// memFileInfo implements os.FileInfo for in-memory files
type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (fi *memFileInfo) Name() string       { return fi.name }
func (fi *memFileInfo) Size() int64        { return fi.size }
func (fi *memFileInfo) Mode() os.FileMode  { return 0o644 }
func (fi *memFileInfo) ModTime() time.Time { return time.Now() }
func (fi *memFileInfo) IsDir() bool        { return fi.isDir }
func (fi *memFileInfo) Sys() interface{}   { return nil }
