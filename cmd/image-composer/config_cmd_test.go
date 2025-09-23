package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteConfigInit_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "my-config.yml")

	cmd := createConfigCommand()
	// Run: os-image-composer config init <path>
	cmd.SetArgs([]string{"init", target})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute config init failed: %v", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected config file to be created at %s, got error: %v", target, err)
	}
}
