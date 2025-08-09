package logging

// Logging functionality for bc-insights-tui

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Logger levels
const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// Logger wraps the standard logger with additional functionality
type Logger struct {
	logger   *log.Logger
	logFile  *os.File
	logLevel string
}

// Global logger instance
var globalLogger *Logger

// InitLogger initializes the global logger
func InitLogger(logLevel string) error {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02")
	logFileName := filepath.Join(logsDir, fmt.Sprintf("bc-insights-tui-%s.log", timestamp))

	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create writer that always writes to file; optionally mirror to stdout
	writers := []io.Writer{logFile}

	// Mirror to stdout only when BC_INSIGHTS_LOG_TO_STDOUT is explicitly enabled (opt-in)
	mirrorEnv := strings.TrimSpace(os.Getenv("BC_INSIGHTS_LOG_TO_STDOUT"))
	mirrorToStdout := mirrorEnv != "" && (strings.EqualFold(mirrorEnv, "1") || strings.EqualFold(mirrorEnv, "true") || strings.EqualFold(mirrorEnv, "yes"))
	if mirrorToStdout {
		writers = append(writers, os.Stdout)
	}

	multiWriter := io.MultiWriter(writers...)

	// Use standard flags only; we'll inject caller info manually to get real call site
	logger := log.New(multiWriter, "", log.LstdFlags)

	globalLogger = &Logger{
		logger:   logger,
		logFile:  logFile,
		logLevel: logLevel,
	}

	Info("Logger initialized", "level", logLevel, "file", logFileName, "mirrorStdout", fmt.Sprintf("%t", mirrorToStdout))
	return nil
}

// Close closes the log file
func Close() error {
	if globalLogger != nil && globalLogger.logFile != nil {
		return globalLogger.logFile.Close()
	}
	return nil
}

// shouldLog checks if the message should be logged based on the current log level
func shouldLog(level string) bool {
	if globalLogger == nil {
		return false
	}

	currentLevel := globalLogger.logLevel
	switch currentLevel {
	case LevelDebug:
		return true // Log everything
	case LevelInfo:
		return level != LevelDebug
	case LevelWarn:
		return level == LevelWarn || level == LevelError
	case LevelError:
		return level == LevelError
	default:
		return true
	}
}

// formatMessage formats a log message with key-value pairs
func formatMessage(level, message string, keyValues ...string) string {
	// Determine real caller site: skip 2 to jump over Debug/Info/Warn/Error and formatMessage itself
	file, line := callerSite(2)
	prefix := ""
	if file != "" {
		prefix = fmt.Sprintf("%s:%d ", file, line)
	}
	msg := fmt.Sprintf("[%s] %s%s", level, prefix, message)

	// Add key-value pairs
	for i := 0; i < len(keyValues)-1; i += 2 {
		key := keyValues[i]
		value := keyValues[i+1]
		msg += fmt.Sprintf(" %s=%s", key, value)
	}

	return msg
}

// callerSite returns a short file:line of the true call site.
func callerSite(skip int) (string, int) {
	// skip base accounts for this function, formatMessage, and the exported log helper
	// The provided skip should be 2 from the public helpers; add 1 for this frame
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}

// Debug logs a debug message
func Debug(message string, keyValues ...string) {
	if !shouldLog(LevelDebug) {
		return
	}

	if globalLogger != nil {
		globalLogger.logger.Println(formatMessage(LevelDebug, message, keyValues...))
	}
}

// Info logs an info message
func Info(message string, keyValues ...string) {
	if !shouldLog(LevelInfo) {
		return
	}

	if globalLogger != nil {
		globalLogger.logger.Println(formatMessage(LevelInfo, message, keyValues...))
	}
}

// Warn logs a warning message
func Warn(message string, keyValues ...string) {
	if !shouldLog(LevelWarn) {
		return
	}

	if globalLogger != nil {
		globalLogger.logger.Println(formatMessage(LevelWarn, message, keyValues...))
	}
}

// Error logs an error message
func Error(message string, keyValues ...string) {
	if !shouldLog(LevelError) {
		return
	}

	if globalLogger != nil {
		globalLogger.logger.Println(formatMessage(LevelError, message, keyValues...))
	}
}

// GetLogLevel returns the current log level
func GetLogLevel() string {
	if globalLogger == nil {
		return LevelInfo
	}
	return globalLogger.logLevel
}
