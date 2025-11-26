package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveRequestedLogLevelPrefersExplicitFlag(t *testing.T) {
	prev := logLevel
	logLevel = "warn"
	t.Cleanup(func() {
		logLevel = prev
	})

	if got := resolveRequestedLogLevel(nil); got != "warn" {
		t.Fatalf("expected explicit log level to win, got %q", got)
	}
}

func TestResolveRequestedLogLevelUsesVerboseFallback(t *testing.T) {
	prev := logLevel
	logLevel = ""
	t.Cleanup(func() {
		logLevel = prev
	})

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("verbose", false, "")
	if err := cmd.Flags().Set("verbose", "true"); err != nil {
		t.Fatalf("set verbose: %v", err)
	}

	if got := resolveRequestedLogLevel(cmd); got != "debug" {
		t.Fatalf("expected verbose flag to set debug level, got %q", got)
	}
}

func TestResolveRequestedLogLevelIgnoresUnsetVerbose(t *testing.T) {
	prev := logLevel
	logLevel = ""
	t.Cleanup(func() {
		logLevel = prev
	})

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("verbose", false, "")

	if got := resolveRequestedLogLevel(cmd); got != "" {
		t.Fatalf("expected empty when verbose not set, got %q", got)
	}
}

func TestAttachLoggingHooksAddsHookToSubcommand(t *testing.T) {
	root := createRootCommand()
	cmd, _, err := root.Find([]string{"build"})
	if err != nil {
		t.Fatalf("find build command: %v", err)
	}
	if cmd == nil {
		t.Fatal("build command not found")
	}
	if cmd.PersistentPreRunE == nil {
		t.Fatal("expected logging hook on build command")
	}
}
