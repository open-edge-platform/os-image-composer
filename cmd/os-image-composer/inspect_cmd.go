package main

import (
	"encoding/json"
	"fmt"

	"github.com/open-edge-platform/os-image-composer/internal/image/imageinspect"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// cmd needs only these two methods.
type inspector interface {
	Inspect(imagePath string) (*imageinspect.ImageSummary, error)
}

// Allow tests to inject a fake inspector.
var newInspector = func() inspector {
	return imageinspect.NewDiskfsInspector() // returns *DiskfsInspector which satisfies inspector
}

// Output format command flags
var (
	outputFormat string = "text" // Output format for the inspection results
	prettyJSON   bool   = false  // Pretty-print JSON output
)

// createInspectCommand creates the inspect subcommand
func createInspectCommand() *cobra.Command {
	inspectCmd := &cobra.Command{
		Use:   "inspect [flags] IMAGE_FILE",
		Short: "inspects a RAW image file",
		Long: `Inspect performs a deep inspection of a generated
		RAW image and provides useful details of the image such as
		partition table layout, filesystem type, bootloader type and 
		configuration and overall SBOM details if available.`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			switch outputFormat {
			case "text", "json", "yaml":
				return nil
			default:
				return fmt.Errorf("unsupported --format %q (supported: text, json, yaml)", outputFormat)
			}
		},
		RunE:              executeInspect,
		ValidArgsFunction: templateFileCompletion,
	}

	// Add flags
	inspectCmd.Flags().StringVar(&outputFormat, "format", "text",
		"Specify the output format for the inspection results")

	inspectCmd.Flags().BoolVar(&prettyJSON, "pretty", false,
		"Pretty-print JSON output (only for --format json)")

	return inspectCmd
}

// executeInspect handles the inspect command execution logic
func executeInspect(cmd *cobra.Command, args []string) error {
	log := logger.Logger()
	imageFile := args[0]
	log.Infof("Inspecting image file: %s", imageFile)

	inspector := newInspector()

	inspectionResults, err := inspector.Inspect(imageFile)
	if err != nil {
		return fmt.Errorf("image inspection failed: %v", err)
	}

	if err := writeInspectionResult(cmd, inspectionResults, outputFormat, prettyJSON); err != nil {
		return err
	}

	return nil
}

func writeInspectionResult(cmd *cobra.Command, summary *imageinspect.ImageSummary, format string, pretty bool) error {
	out := cmd.OutOrStdout()

	switch format {
	case "text":
		if err := imageinspect.RenderSummaryText(out, summary, imageinspect.TextOptions{}); err != nil {
			return fmt.Errorf("render text: %w", err)
		}
		return nil

	case "json":
		var (
			b   []byte
			err error
		)
		if pretty {
			b, err = json.MarshalIndent(summary, "", "  ")
		} else {
			b, err = json.Marshal(summary)
		}
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, _ = fmt.Fprintln(out, string(b))
		return nil

	case "yaml":
		b, err := yaml.Marshal(summary)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, _ = fmt.Fprintln(out, string(b))
		return nil

	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
