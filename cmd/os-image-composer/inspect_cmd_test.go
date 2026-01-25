package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/image/imageinspect"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// resetInspectFlags resets inspect flags to defaults.
func resetInspectFlags() {
	outputFormat = "text"
	newInspector = func() inspector {
		return imageinspect.NewDiskfsInspector()
	}
}

// fakeInspector is a tiny test double so we can cover output branches without
// needing a real disk image.
type fakeInspector struct {
	summary *imageinspect.ImageSummary
	err     error
}

func (f *fakeInspector) Inspect(imagePath string) (*imageinspect.ImageSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.summary, nil
}
func (f *fakeInspector) DisplaySummary(ioWriter io.Writer, summary *imageinspect.ImageSummary) {
	_, _ = os.Stdout.Write([]byte("FAKE_SUMMARY\n"))
}

// helper: execute a cobra command and capture output.
func execCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestCreateInspectCommand(t *testing.T) {
	defer resetInspectFlags()

	cmd := createInspectCommand()

	t.Run("CommandMetadata", func(t *testing.T) {
		if cmd == nil {
			t.Fatal("createInspectCommand returned nil")
		}
		if cmd.Use != "inspect [flags] IMAGE_FILE" {
			t.Errorf("expected Use='inspect [flags] IMAGE_FILE', got %q", cmd.Use)
		}
		if cmd.Short == "" {
			t.Error("Short description should not be empty")
		}
		if cmd.Long == "" {
			t.Error("Long description should not be empty")
		}
	})

	t.Run("CommandFlags", func(t *testing.T) {
		formatFlag := cmd.Flags().Lookup("format")
		if formatFlag == nil {
			t.Fatal("--format flag should be registered")
		}
		if formatFlag.DefValue != "text" {
			t.Errorf("--format default should be 'text', got %q", formatFlag.DefValue)
		}
		if formatFlag.Usage == "" {
			t.Error("--format should have usage text")
		}
	})

	t.Run("CommandFlags", func(t *testing.T) {
		formatFlag := cmd.Flags().Lookup("pretty")
		if formatFlag == nil {
			t.Fatal("--pretty flag should be registered")
		}
		if formatFlag.DefValue != "false" {
			t.Errorf("--pretty default should be 'false', got %q", formatFlag.DefValue)
		}
		if formatFlag.Usage == "" {
			t.Error("--pretty should have usage text")
		}
	})

	t.Run("ArgsValidation", func(t *testing.T) {
		if cmd.Args == nil {
			t.Fatal("Args validator should be set")
		}

		if err := cmd.Args(cmd, []string{}); err == nil {
			t.Error("should error with no arguments")
		}
		if err := cmd.Args(cmd, []string{"image.raw"}); err != nil {
			t.Errorf("should accept one argument, got error: %v", err)
		}
		if err := cmd.Args(cmd, []string{"image.raw", "extra"}); err == nil {
			t.Error("should error with two arguments")
		}
	})

	t.Run("CompletionFunction", func(t *testing.T) {
		if cmd.ValidArgsFunction == nil {
			t.Error("ValidArgsFunction should be set for shell completion")
		}
	})
}

func TestInspectCommand_HelpOutput(t *testing.T) {
	defer resetInspectFlags()

	cmd := createInspectCommand()

	out, err := execCmd(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help should not error: %v", err)
	}

	expected := []string{
		"inspect",
		"IMAGE_FILE",
		"--format",
	}
	for _, s := range expected {
		if !strings.Contains(out, s) {
			t.Errorf("help output should contain %q", s)
		}
	}
}

func TestExecuteInspect_DirectCall(t *testing.T) {
	defer resetInspectFlags()

	t.Run("NonexistentFile", func(t *testing.T) {
		cmd := createInspectCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := executeInspect(cmd, []string{"/nonexistent/image.raw"})
		if err == nil {
			t.Fatal("expected error for nonexistent image")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "inspection") &&
			!strings.Contains(strings.ToLower(err.Error()), "stat") &&
			!strings.Contains(strings.ToLower(err.Error()), "open") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("UnsupportedFormat", func(t *testing.T) {
		resetInspectFlags()
		t.Cleanup(resetInspectFlags)

		oldNew := newInspector
		t.Cleanup(func() { newInspector = oldNew })
		newInspector = func() inspector {
			return &fakeInspector{
				summary: &imageinspect.ImageSummary{File: "fake.img", SizeBytes: 123},
			}
		}

		defer resetInspectFlags()

		cmd := createInspectCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		if err := cmd.Flags().Set("format", "nope"); err != nil {
			t.Fatalf("failed to set format flag: %v", err)
		}

		err := executeInspect(cmd, []string{"fake.img"})
		if err == nil {
			t.Fatal("expected error for unsupported output format")
		}
		if !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("expected unsupported output format error, got: %v", err)
		}
	})

	t.Run("InspectFailsPropagates", func(t *testing.T) {
		resetInspectFlags()
		t.Cleanup(resetInspectFlags)

		oldNew := newInspector
		t.Cleanup(func() { newInspector = oldNew })
		newInspector = func() inspector {
			return &fakeInspector{err: errors.New("boom")}
		}
		defer resetInspectFlags()

		cmd := createInspectCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		err := executeInspect(cmd, []string{"fake.img"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "image inspection failed") {
			t.Errorf("expected wrapped error, got: %v", err)
		}
	})
}

func TestInspectCommand_OutputFormats_WithFakeInspector(t *testing.T) {
	defer resetInspectFlags()

	// Deterministic summary for output checks
	fake := &imageinspect.ImageSummary{
		File:      "fake.img",
		SizeBytes: 42,
		PartitionTable: imageinspect.PartitionTableSummary{
			Type:              "gpt",
			LogicalSectorSize: 512,
			Partitions: []imageinspect.PartitionSummary{
				{Index: 1, Name: "boot", Type: "C12A7328-F81F-11D2-BA4B-00A0C93EC93B", StartLBA: 2048, EndLBA: 4095, SizeBytes: 2048 * 512},
			},
		},
	}

	newInspector = func() inspector {
		return &fakeInspector{summary: fake}
	}
	defer resetInspectFlags()

	t.Run("Text", func(t *testing.T) {
		outputFormat = "text"
		cmd := createInspectCommand()
		out, err := execCmd(t, cmd, "fake.img")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		_ = out
	})

	t.Run("JSON", func(t *testing.T) {
		resetInspectFlags()

		newInspector = func() inspector {
			return &fakeInspector{
				summary: &imageinspect.ImageSummary{
					File:      "fake.img",
					SizeBytes: 42,
					PartitionTable: imageinspect.PartitionTableSummary{
						Type:              "gpt",
						LogicalSectorSize: 512,
						Partitions: []imageinspect.PartitionSummary{
							{Index: 1, Name: "boot", Type: "C12A7328-F81F-11D2-BA4B-00A0C93EC93B", StartLBA: 2048, EndLBA: 4095, SizeBytes: 1024 * 1024},
						},
					},
				},
			}
		}

		cmd := createInspectCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		if err := cmd.Flags().Set("format", "json"); err != nil {
			t.Fatalf("set flag: %v", err)
		}

		out, err := execCmd(t, cmd, "fake.img")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Stronger: unmarshal instead of substring match
		var got imageinspect.ImageSummary
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid json: %v\nout:\n%s", err, out)
		}
		if got.File != "fake.img" {
			t.Fatalf("expected File=fake.img got %q", got.File)
		}
		if got.PartitionTable.Type != "gpt" {
			t.Fatalf("expected PT type=gpt got %q", got.PartitionTable.Type)
		}
	})

	t.Run("YAML", func(t *testing.T) {
		newInspector = func() inspector {
			return &fakeInspector{
				summary: &imageinspect.ImageSummary{
					File:      "fake.img",
					SizeBytes: 42,
					PartitionTable: imageinspect.PartitionTableSummary{
						Type:              "gpt",
						LogicalSectorSize: 512,
						Partitions: []imageinspect.PartitionSummary{
							{Index: 1, Name: "boot", Type: "C12A7328-F81F-11D2-BA4B-00A0C93EC93B", StartLBA: 2048, EndLBA: 4095, SizeBytes: 1024 * 1024},
						},
					},
				},
			}
		}

		cmd := createInspectCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})

		if err := cmd.Flags().Set("format", "yaml"); err != nil {
			t.Fatalf("set flag: %v", err)
		}

		out, err := execCmd(t, cmd, "fake.img")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var got imageinspect.ImageSummary
		if err := yaml.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("invalid yaml: %v\nout:\n%s", err, out)
		}
		if got.File != "fake.img" {
			t.Fatalf("expected File=fake.img got %q", got.File)
		}
		if got.PartitionTable.Type != "gpt" {
			t.Fatalf("expected PT type=gpt got %q", got.PartitionTable.Type)
		}
	})
}
