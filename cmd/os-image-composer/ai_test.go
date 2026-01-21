package main

import (
	"testing"
)

func TestCreateAICommand(t *testing.T) {
	cmd := createAICommand()

	if cmd == nil {
		t.Fatal("createAICommand returned nil")
	}

	if cmd.Use != "ai [query]" {
		t.Errorf("expected Use 'ai [query]', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}

	// Verify flags are registered
	flags := cmd.Flags()

	providerFlag := flags.Lookup("provider")
	if providerFlag == nil {
		t.Error("expected --provider flag to be registered")
	}

	templatesDirFlag := flags.Lookup("templates-dir")
	if templatesDirFlag == nil {
		t.Error("expected --templates-dir flag to be registered")
	}

	clearCacheFlag := flags.Lookup("clear-cache")
	if clearCacheFlag == nil {
		t.Error("expected --clear-cache flag to be registered")
	}

	cacheStatsFlag := flags.Lookup("cache-stats")
	if cacheStatsFlag == nil {
		t.Error("expected --cache-stats flag to be registered")
	}

	searchOnlyFlag := flags.Lookup("search-only")
	if searchOnlyFlag == nil {
		t.Error("expected --search-only flag to be registered")
	}
}

func TestMinFunc(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 1},
		{5, 3, 3},
		{10, 10, 10},
		{-1, 1, -1},
		{0, 5, 0},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
