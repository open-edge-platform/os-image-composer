package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/image/imageinspect"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/spf13/cobra"
)

// Output format command flags
var (
	prettyDiffJSON bool   = true // Pretty-print JSON output
	outFormat      string        // "text" | "json"
	outMode        string = ""   // "full" | "diff" | "summary"
)

// createCompareCommand creates the compare subcommand
func createCompareCommand() *cobra.Command {
	compareCmd := &cobra.Command{
		Use:   "compare [flags] IMAGE_FILE1 IMAGE_FILE2",
		Short: "compares two RAW image files",
		Long: `Compare performs a deep comparison of two generated
		RAW images and provides useful details of the differences such as
		partition table layout, filesystem type, bootloader type and 
		configuration and overall SBOM details if available.`,
		Args: cobra.ExactArgs(2),

		RunE:              executeCompare,
		ValidArgsFunction: templateFileCompletion,
	}

	// Add flags
	compareCmd.Flags().BoolVar(&prettyDiffJSON, "pretty", true,
		"Pretty-print JSON output (only for --format json)")
	compareCmd.Flags().StringVar(&outFormat, "format", "text",
		"Output format: text or json")
	compareCmd.Flags().StringVar(&outMode, "mode", "",
		"Output mode: full, diff, or summary (default: diff for text, full for json)")
	return compareCmd
}

func resolveDefaults(format, mode string) (string, string) {
	format = strings.ToLower(format)
	mode = strings.ToLower(mode)

	// Set default mode if not specified
	if mode == "" {
		if format == "json" {
			mode = "full"
		} else {
			mode = "diff"
		}
	}
	return format, mode
}

// executeCompare handles the compare command execution logic
func executeCompare(cmd *cobra.Command, args []string) error {
	log := logger.Logger()
	imageFile1 := args[0]
	imageFile2 := args[1]
	log.Infof("Comparing image files: %s and %s", imageFile1, imageFile2)

	inspector := newInspector()

	image1, err1 := inspector.Inspect(imageFile1)
	if err1 != nil {
		return fmt.Errorf("image inspection failed: %v", err1)
	}
	image2, err2 := inspector.Inspect(imageFile2)
	if err2 != nil {
		return fmt.Errorf("image inspection failed: %v", err2)
	}

	compareResult := imageinspect.CompareImages(image1, image2)

	format, mode := resolveDefaults(outFormat, outMode)

	switch format {
	case "json":
		var payload any
		switch mode {
		case "full":
			payload = &compareResult
		case "diff":
			payload = struct {
				Equal bool                   `json:"equal"`
				Diff  imageinspect.ImageDiff `json:"diff"`
			}{Equal: compareResult.Equal, Diff: compareResult.Diff}
		case "summary":
			payload = struct {
				Equal   bool                        `json:"equal"`
				Summary imageinspect.CompareSummary `json:"summary"`
			}{Equal: compareResult.Equal, Summary: compareResult.Summary}
		default:
			return fmt.Errorf("invalid --mode %q (expected diff|summary|full)", mode)
		}
		return writeCompareResult(cmd, payload, prettyDiffJSON)

	case "text":
	    return imageinspect.RenderCompareText(cmd.OutOrStdout(), &compareResult,
    	    imageinspect.CompareTextOptions{Mode: mode})

	default:
		return fmt.Errorf("invalid --mode %q (expected text|json)", outMode)
	}
}

func writeCompareResult(cmd *cobra.Command, v any, pretty bool) error {
	out := cmd.OutOrStdout()

	var (
		b   []byte
		err error
	)
	if pretty {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	_, _ = fmt.Fprintln(out, string(b))
	return nil
}

