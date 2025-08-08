package config

import (
	"os"
)

// FileSystem abstracts filesystem operations for testability.
// This allows us to inject different implementations for production and testing.
type FileSystem interface {
	// ReadFile reads the named file and returns its contents
	ReadFile(filename string) ([]byte, error)

	// WriteFile writes data to the named file, creating it if necessary
	WriteFile(filename string, data []byte, perm os.FileMode) error

	// Stat returns a FileInfo describing the named file
	Stat(filename string) (os.FileInfo, error)

	// UserHomeDir returns the current user's home directory
	UserHomeDir() (string, error)

	// UserConfigDir returns the current user's config directory
	UserConfigDir() (string, error)

	// Getwd returns the current working directory
	Getwd() (string, error)

	// MkdirAll creates a directory path and all parents if they don't exist
	MkdirAll(path string, perm os.FileMode) error
}

// OsFileSystem implements FileSystem using the real operating system.
// This is used in production.
type OsFileSystem struct{}

// ReadFile implements FileSystem.ReadFile using os.ReadFile
func (fs *OsFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// WriteFile implements FileSystem.WriteFile using os.WriteFile
func (fs *OsFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// Stat implements FileSystem.Stat using os.Stat
func (fs *OsFileSystem) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(filename)
}

// UserHomeDir implements FileSystem.UserHomeDir using os.UserHomeDir
func (fs *OsFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// UserConfigDir implements FileSystem.UserConfigDir using os.UserConfigDir
func (fs *OsFileSystem) UserConfigDir() (string, error) {
	return os.UserConfigDir()
}

// Getwd implements FileSystem.Getwd using os.Getwd
func (fs *OsFileSystem) Getwd() (string, error) {
	return os.Getwd()
}

// MkdirAll implements FileSystem.MkdirAll using os.MkdirAll
func (fs *OsFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
