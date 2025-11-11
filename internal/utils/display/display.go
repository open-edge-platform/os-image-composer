package display

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
)

// PrintImageDirectorySummary displays all image artifacts in a directory
// This is called after image build completes to show all generated files
func PrintImageDirectorySummary(imageBuildDir, imageType string) {
	log := logger.Logger()

	log.Infof("Checking for image artifacts in: %s", imageBuildDir)

	// List all files in the directory (excluding SBOM)
	files, err := os.ReadDir(imageBuildDir)
	if err != nil {
		log.Warnf("Unable to read image build directory %s: %v", imageBuildDir, err)
		return
	}

	log.Infof("Found %d total entries in directory", len(files))

	// Collect all files (including SBOM)
	var imageFiles []string
	for _, file := range files {
		name := file.Name()
		log.Infof("Checking file: %s (isDir=%v)", name, file.IsDir())

		if file.IsDir() {
			continue
		}
		imageFiles = append(imageFiles, name)
	}

	log.Infof("Found %d artifacts after filtering", len(imageFiles))

	if len(imageFiles) == 0 {
		log.Warn("No artifacts found in build directory")
		return
	}

	// Print highlighted box with success message
	log.Info("")
	log.Info("╔════════════════════════════════════════════════════════════════════════════╗")
	log.Info("║                    ✓ IMAGE CREATED SUCCESSFULLY                            ║")
	log.Info("╚════════════════════════════════════════════════════════════════════════════╝")
	log.Info("")

	// Print image type
	log.Infof("  Image Type:   %s", imageType)
	log.Info("")
	log.Info("  Generated Artifacts (including SBOM):")

	// Print each artifact with size
	for _, filename := range imageFiles {
		fullPath := filepath.Join(imageBuildDir, filename)
		fileInfo, err := os.Stat(fullPath)
		var sizeStr string
		if err == nil {
			sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
			if sizeMB > 1024 {
				sizeStr = fmt.Sprintf("%.2f GB", sizeMB/1024)
			} else {
				sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
			}
		} else {
			sizeStr = "unknown"
		}

		log.Infof("    • %s (%s)", filename, sizeStr)
		log.Infof("      %s", fullPath)
		log.Info("")
	}

	log.Info("════════════════════════════════════════════════════════════════════════════")
	log.Info("")
}
