package main

import (
	"encoding/json"
	"fmt"
	"io"
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


func printPartitionLine(w io.Writer, prefix string, p imageinspect.PartitionSummary) {
	fs := ""
	if p.Filesystem != nil {
		fs = fmt.Sprintf(" fs=%s uuid=%s label=%s", p.Filesystem.Type, p.Filesystem.UUID, p.Filesystem.Label)
	}
	fmt.Fprintf(w, "%s idx=%d name=%q type=%q lba=%d-%d size=%d flags=%q%s\n",
		prefix, p.Index, p.Name, p.Type, p.StartLBA, p.EndLBA, p.SizeBytes, p.Flags, fs)
}

func printFilesystemChange(w io.Writer, c *imageinspect.FilesystemChange) {
	if c.Added != nil {
		fmt.Fprintf(w, "      FS: added type=%s uuid=%s label=%s\n", c.Added.Type, c.Added.UUID, c.Added.Label)
		return
	}
	if c.Removed != nil {
		fmt.Fprintf(w, "      FS: removed type=%s uuid=%s label=%s\n", c.Removed.Type, c.Removed.UUID, c.Removed.Label)
		return
	}
	if c.Modified != nil {
		fmt.Fprintf(w, "      FS: modified %s(%s) -> %s(%s)\n",
			c.Modified.From.Type, c.Modified.From.UUID, c.Modified.To.Type, c.Modified.To.UUID)
		for _, ch := range c.Modified.Changes {
			fmt.Fprintf(w, "        %s: %v -> %v\n", ch.Field, ch.From, ch.To)
		}
	}
}

func printEFIDiff(w io.Writer, d *imageinspect.EFIBinaryDiff, header string) {
	fmt.Fprintln(w, header)

	if len(d.Added) > 0 {
		fmt.Fprintln(w, "        Added:")
		for _, e := range d.Added {
			fmt.Fprintf(w, "          + %s kind=%s arch=%s signed=%v sha=%s\n", e.Path, e.Kind, e.Arch, e.Signed, e.SHA256)
		}
	}
	if len(d.Removed) > 0 {
		fmt.Fprintln(w, "        Removed:")
		for _, e := range d.Removed {
			fmt.Fprintf(w, "          - %s kind=%s arch=%s signed=%v sha=%s\n", e.Path, e.Kind, e.Arch, e.Signed, e.SHA256)
		}
	}
	if len(d.Modified) > 0 {
		fmt.Fprintln(w, "        Modified:")
		for _, m := range d.Modified {
			fmt.Fprintf(w, "          ~ %s\n", m.Key)
			if m.From.Kind != m.To.Kind {
				fmt.Fprintf(w, "            kind: %s -> %s\n", m.From.Kind, m.To.Kind)
			}
			if m.From.SHA256 != m.To.SHA256 {
				fmt.Fprintf(w, "            sha256: %s -> %s\n", m.From.SHA256, m.To.SHA256)
			}
			if m.From.Signed != m.To.Signed {
				fmt.Fprintf(w, "            signed: %v -> %v\n", m.From.Signed, m.To.Signed)
			}

			if m.UKI != nil && m.UKI.Changed {
				fmt.Fprintln(w, "            UKI payload:")
				if m.UKI.KernelSHA256 != nil {
					fmt.Fprintf(w, "              kernel:  %s -> %s\n", m.UKI.KernelSHA256.From, m.UKI.KernelSHA256.To)
				}
				if m.UKI.InitrdSHA256 != nil {
					fmt.Fprintf(w, "              initrd:  %s -> %s\n", m.UKI.InitrdSHA256.From, m.UKI.InitrdSHA256.To)
				}
				if m.UKI.CmdlineSHA256 != nil {
					fmt.Fprintf(w, "              cmdline: %s -> %s\n", m.UKI.CmdlineSHA256.From, m.UKI.CmdlineSHA256.To)
				}
				if m.UKI.OSRelSHA256 != nil {
					fmt.Fprintf(w, "              osrel:   %s -> %s\n", m.UKI.OSRelSHA256.From, m.UKI.OSRelSHA256.To)
				}
				if m.UKI.UnameSHA256 != nil {
					fmt.Fprintf(w, "              uname:   %s -> %s\n", m.UKI.UnameSHA256.From, m.UKI.UnameSHA256.To)
				}
			}
		}
	}
}
