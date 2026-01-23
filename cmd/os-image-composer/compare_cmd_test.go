package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/image/imageinspect"
	"github.com/spf13/cobra"
)

// fakeInspector implements the inspector interface used by executeCompare.
type fakeCompareInspector struct {
	imgByPath map[string]*imageinspect.ImageSummary
	errByPath map[string]error
}

func (f *fakeCompareInspector) Inspect(path string) (*imageinspect.ImageSummary, error) {
	if err, ok := f.errByPath[path]; ok {
		return nil, err
	}
	if img, ok := f.imgByPath[path]; ok {
		return img, nil
	}
	return nil, errors.New("not found")
}

func minimalImage(file string, size int64) *imageinspect.ImageSummary {
	return &imageinspect.ImageSummary{
		File:      file,
		SizeBytes: size,
		PartitionTable: imageinspect.PartitionTableSummary{
			Type:               "gpt",
			LogicalSectorSize:  512,
			PhysicalSectorSize: 4096,
			ProtectiveMBR:      true,
			Partitions:         nil,
		},
	}
}

func runCompareExecute(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := executeCompare(cmd, args)
	return out.String(), err
}

// decodeJSON is tolerant of both “full” compare result and the “diff/summary wrapper” structs.
func decodeJSON(t *testing.T, s string, v any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields() // helps catch shape regressions in these tests
	if err := dec.Decode(v); err != nil {
		t.Fatalf("failed to decode json: %v\njson:\n%s", err, s)
	}
}

// ---- Tests ----

func TestResolveDefaults(t *testing.T) {
	t.Run("json defaults to full when mode empty", func(t *testing.T) {
		format, mode := resolveDefaults("json", "")
		if format != "json" || mode != "full" {
			t.Fatalf("expected (json, full), got (%s, %s)", format, mode)
		}
	})

	t.Run("text defaults to diff when mode empty", func(t *testing.T) {
		format, mode := resolveDefaults("text", "")
		if format != "text" || mode != "diff" {
			t.Fatalf("expected (text, diff), got (%s, %s)", format, mode)
		}
	})

	t.Run("explicit mode is preserved", func(t *testing.T) {
		_, mode := resolveDefaults("text", "summary")
		if mode != "summary" {
			t.Fatalf("expected summary, got %s", mode)
		}
	})
}

func TestCompareCommand_JSONModes_PrettyAndCompact(t *testing.T) {
	// IMPORTANT: these tests assume newInspector is a package-level var func you can override.
	// If it’s a normal function, change code to allow injection or adapt these tests.

	origNewInspector := newInspector
	t.Cleanup(func() { newInspector = origNewInspector })

	fi := &fakeCompareInspector{
		imgByPath: map[string]*imageinspect.ImageSummary{
			"a.raw": minimalImage("a.raw", 10),
			"b.raw": minimalImage("b.raw", 20),
		},
		errByPath: map[string]error{},
	}
	newInspector = func() inspector { return fi }

	// Make a command instance to provide OutOrStdout/flags context (executeCompare uses cmd for output).
	cmd := &cobra.Command{}
	cmd.SetArgs([]string{})

	t.Run("full pretty", func(t *testing.T) {
		outFormat = "json"
		outMode = "full"
		prettyDiffJSON = true

		s, err := runCompareExecute(t, cmd, []string{"a.raw", "b.raw"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(s, "\n  \"") {
			t.Fatalf("expected pretty-printed json with indentation, got:\n%s", s)
		}

		// Validate it looks like ImageCompareResult (at least top-level fields).
		var got struct {
			SchemaVersion string          `json:"schemaVersion"`
			Equal         bool            `json:"equal"`
			From          json.RawMessage `json:"from"`
			To            json.RawMessage `json:"to"`
			Summary       json.RawMessage `json:"summary"`
			Diff          json.RawMessage `json:"diff"`
		}
		decodeJSON(t, s, &got)
		if got.SchemaVersion == "" {
			t.Fatalf("expected schemaVersion to be set")
		}
	})

	t.Run("diff compact", func(t *testing.T) {
		outFormat = "json"
		outMode = "diff"
		prettyDiffJSON = false

		s, err := runCompareExecute(t, cmd, []string{"a.raw", "b.raw"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		// compact JSON: no indentation by default; allow newlines from fmt.Fprintln only
		if strings.Contains(s, "\n  \"") {
			t.Fatalf("expected compact json, got:\n%s", s)
		}

		var got struct {
			Equal bool                   `json:"equal"`
			Diff  imageinspect.ImageDiff `json:"diff"`
		}
		decodeJSON(t, s, &got)
	})

	t.Run("summary pretty", func(t *testing.T) {
		outFormat = "json"
		outMode = "summary"
		prettyDiffJSON = true

		s, err := runCompareExecute(t, cmd, []string{"a.raw", "b.raw"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(s, "\n  \"") {
			t.Fatalf("expected pretty json, got:\n%s", s)
		}

		var got struct {
			Equal   bool                        `json:"equal"`
			Summary imageinspect.CompareSummary `json:"summary"`
		}
		decodeJSON(t, s, &got)
	})
}

func TestCompareCommand_TextOutput(t *testing.T) {
	origNewInspector := newInspector
	t.Cleanup(func() { newInspector = origNewInspector })

	// Make two images that differ in partition table type to force a diff
	img1 := minimalImage("a.raw", 10)
	img2 := minimalImage("b.raw", 10)
	img2.PartitionTable.Type = "mbr"

	fi := &fakeCompareInspector{
		imgByPath: map[string]*imageinspect.ImageSummary{
			"a.raw": img1,
			"b.raw": img2,
		},
	}
	newInspector = func() inspector { return fi }

	cmd := &cobra.Command{}
	outFormat = "text"
	outMode = "" // let resolveDefaults pick "diff"

	s, err := runCompareExecute(t, cmd, []string{"a.raw", "b.raw"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Basic structure checks (don’t overfit exact wording)
	if !strings.Contains(s, "Equal:") {
		t.Fatalf("expected 'Equal:' header, got:\n%s", s)
	}
	if !strings.Contains(s, "Partition table:") {
		t.Fatalf("expected partition table section, got:\n%s", s)
	}
	if !strings.Contains(s, "Type:") {
		t.Fatalf("expected partition table field diff, got:\n%s", s)
	}
}

func TestCompareCommand_InspectorError(t *testing.T) {
	origNewInspector := newInspector
	t.Cleanup(func() { newInspector = origNewInspector })

	fi := &fakeCompareInspector{
		imgByPath: map[string]*imageinspect.ImageSummary{
			"a.raw": minimalImage("a.raw", 10),
		},
		errByPath: map[string]error{
			"b.raw": errors.New("boom"),
		},
	}
	newInspector = func() inspector { return fi }

	cmd := &cobra.Command{}
	outFormat = "json"
	outMode = "summary"

	_, err := runCompareExecute(t, cmd, []string{"a.raw", "b.raw"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "inspection") {
		t.Fatalf("expected inspection error, got: %v", err)
	}
}
