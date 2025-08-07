package tui

import (
	"testing"
	"time"
)

func TestQueryHistory_AddEntry(t *testing.T) {
	history := NewQueryHistory(3, "") // No file persistence for test

	// Add some test entries
	history.AddEntry("traces | limit 10", true, 10, time.Second)
	history.AddEntry("requests | limit 5", true, 5, 500*time.Millisecond)
	history.AddEntry("exceptions | limit 1", false, 0, 100*time.Millisecond)

	if history.Count() != 3 {
		t.Errorf("Expected 3 entries, got %d", history.Count())
	}

	// Check that entries are in reverse chronological order (most recent first)
	entries := history.GetQueries()
	if entries[0].Query != "exceptions | limit 1" {
		t.Errorf("Expected most recent query first, got %q", entries[0].Query)
	}
	if entries[2].Query != "traces | limit 10" {
		t.Errorf("Expected oldest query last, got %q", entries[2].Query)
	}
}

func TestQueryHistory_MaxEntries(t *testing.T) {
	history := NewQueryHistory(2, "") // Max 2 entries

	// Add 3 entries
	history.AddEntry("query1", true, 1, time.Second)
	history.AddEntry("query2", true, 2, time.Second)
	history.AddEntry("query3", true, 3, time.Second)

	// Should only keep the 2 most recent
	if history.Count() != 2 {
		t.Errorf("Expected 2 entries (max), got %d", history.Count())
	}

	entries := history.GetQueries()
	if entries[0].Query != "query3" {
		t.Errorf("Expected most recent query, got %q", entries[0].Query)
	}
	if entries[1].Query != "query2" {
		t.Errorf("Expected second most recent query, got %q", entries[1].Query)
	}
}

func TestQueryHistory_GetQuery(t *testing.T) {
	history := NewQueryHistory(5, "")
	history.AddEntry("test query", true, 10, time.Second)

	// Valid index
	entry, ok := history.GetQuery(0)
	if !ok {
		t.Error("Expected to get query at index 0")
	}
	if entry.Query != "test query" {
		t.Errorf("Expected 'test query', got %q", entry.Query)
	}

	// Invalid index
	_, ok = history.GetQuery(1)
	if ok {
		t.Error("Expected GetQuery to return false for invalid index")
	}

	_, ok = history.GetQuery(-1)
	if ok {
		t.Error("Expected GetQuery to return false for negative index")
	}
}

func TestQueryHistory_Clear(t *testing.T) {
	history := NewQueryHistory(5, "")
	history.AddEntry("query1", true, 1, time.Second)
	history.AddEntry("query2", true, 2, time.Second)

	if history.Count() != 2 {
		t.Errorf("Expected 2 entries before clear, got %d", history.Count())
	}

	history.Clear()

	if history.Count() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", history.Count())
	}

	entries := history.GetQueries()
	if len(entries) != 0 {
		t.Errorf("Expected empty slice after clear, got %d entries", len(entries))
	}
}
