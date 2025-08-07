package tui

// Query history management for KQL Editor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// HistoryEntry represents a stored query in history
type HistoryEntry struct {
	Query     string        `json:"query"`
	Timestamp time.Time     `json:"timestamp"`
	Success   bool          `json:"success"`
	RowCount  int           `json:"rowCount"`
	Duration  time.Duration `json:"duration"`
}

// QueryHistory manages the storage and retrieval of query history
type QueryHistory struct {
	queries    []HistoryEntry
	maxEntries int
	filePath   string
}

// NewQueryHistory creates a new QueryHistory instance
func NewQueryHistory(maxEntries int, filePath string) *QueryHistory {
	qh := &QueryHistory{
		queries:    make([]HistoryEntry, 0),
		maxEntries: maxEntries,
		filePath:   filePath,
	}
	qh.loadFromFile()
	return qh
}

// AddEntry adds a new query to history
func (qh *QueryHistory) AddEntry(query string, success bool, rowCount int, duration time.Duration) {
	entry := HistoryEntry{
		Query:     query,
		Timestamp: time.Now(),
		Success:   success,
		RowCount:  rowCount,
		Duration:  duration,
	}

	// Add to beginning of slice (most recent first)
	qh.queries = append([]HistoryEntry{entry}, qh.queries...)

	// Trim to max entries
	if len(qh.queries) > qh.maxEntries {
		qh.queries = qh.queries[:qh.maxEntries]
	}

	// Save to file
	qh.saveToFile()
}

// GetQueries returns all queries in history (most recent first)
func (qh *QueryHistory) GetQueries() []HistoryEntry {
	return qh.queries
}

// GetQuery returns a specific query by index (0 = most recent)
func (qh *QueryHistory) GetQuery(index int) (HistoryEntry, bool) {
	if index < 0 || index >= len(qh.queries) {
		return HistoryEntry{}, false
	}
	return qh.queries[index], true
}

// Clear removes all queries from history
func (qh *QueryHistory) Clear() {
	qh.queries = make([]HistoryEntry, 0)
	qh.saveToFile()
}

// Count returns the number of queries in history
func (qh *QueryHistory) Count() int {
	return len(qh.queries)
}

// loadFromFile loads query history from file
func (qh *QueryHistory) loadFromFile() {
	if qh.filePath == "" {
		return
	}

	// Expand relative path to absolute path
	absPath, err := filepath.Abs(qh.filePath)
	if err != nil {
		return
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		// File doesn't exist or can't be read - not an error for new installations
		return
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// Invalid JSON - start fresh
		return
	}

	qh.queries = entries
	// Ensure we don't exceed max entries when loading
	if len(qh.queries) > qh.maxEntries {
		qh.queries = qh.queries[:qh.maxEntries]
	}
}

// saveToFile saves query history to file
func (qh *QueryHistory) saveToFile() {
	if qh.filePath == "" {
		return
	}

	// Expand relative path to absolute path
	absPath, err := filepath.Abs(qh.filePath)
	if err != nil {
		return
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(absPath)
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return
	}

	data, err := json.MarshalIndent(qh.queries, "", "  ")
	if err != nil {
		return
	}

	// Write to temporary file first, then rename to avoid corruption
	tempFile := absPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0o600); err != nil {
		return
	}

	// Atomic rename
	_ = os.Rename(tempFile, absPath)
}
