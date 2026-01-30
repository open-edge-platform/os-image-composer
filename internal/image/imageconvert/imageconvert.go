package imageconvert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/utils/compression"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

var log = logger.Logger()

type ImageConvertInterface interface {
	ConvertImageFile(filePath string, template *config.ImageTemplate) error
}

type ImageConvert struct{}

func NewImageConvert() *ImageConvert {
	return &ImageConvert{}
}

func (imageConvert *ImageConvert) ConvertImageFile(filePath string, template *config.ImageTemplate) error {
	var keepRawImage bool
	var rawImageCompressionType string

	if template == nil {
		return fmt.Errorf("image template is nil")
	}

	diskConfig := template.GetDiskConfig()
	if diskConfig.Artifacts != nil {
		if len(diskConfig.Artifacts) > 0 {
			for _, artifact := range diskConfig.Artifacts {
				if artifact.Type != "raw" {
					outputFilePath, err := convertImageFile(filePath, artifact.Type)
					if err != nil {
						return fmt.Errorf("failed to convert image file: %w", err)
					}
					if artifact.Compression != "" {
						if err = compressImageFile(outputFilePath, artifact.Compression); err != nil {
							return fmt.Errorf("failed to compress image file: %w", err)
						}
					}
				} else {
					keepRawImage = true
					if artifact.Compression != "" {
						rawImageCompressionType = artifact.Compression
					}
				}
			}

			if !keepRawImage {
				if err := os.Remove(filePath); err != nil {
					log.Warnf("Failed to remove raw image file: %v", err)
				}
			} else {
				if rawImageCompressionType != "" {
					if err := compressImageFile(filePath, rawImageCompressionType); err != nil {
						return fmt.Errorf("failed to compress raw image file: %w", err)
					}
				}
			}
		}
	}

	return nil
}

// DetectImageFormat detects the format of an image file using qemu-img
func DetectImageFormat(filePath string) (string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("image file does not exist: %s", filePath)
	}

	cmdStr := fmt.Sprintf("qemu-img info --output=json %s", filePath)
	output, err := shell.ExecCmd(cmdStr, false, shell.HostPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to detect image format: %w", err)
	}

	// Parse JSON output to extract format
	// Simple string search for format field
	outputStr := strings.TrimSpace(output)
	if strings.Contains(outputStr, `"format"`) {
		// Extract format value
		for _, line := range strings.Split(outputStr, "\n") {
			if strings.Contains(line, `"format"`) {
				// Parse: "format": "raw",
				parts := strings.Split(line, `"`)
				if len(parts) >= 4 {
					format := strings.TrimSpace(parts[3])
					log.Debugf("Detected image format: %s", format)
					return format, nil
				}
			}
		}
	}

	return "", fmt.Errorf("failed to parse image format from qemu-img output")
}

// ConvertImageToRaw converts any qemu-img supported format to RAW format
// This is useful for normalizing images before comparison or inspection
func ConvertImageToRaw(filePath, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = filepath.Dir(filePath)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Detect source format
	sourceFormat, err := DetectImageFormat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to detect source format: %w", err)
	}

	// If already raw, just return the path
	if sourceFormat == "raw" {
		log.Debugf("Image is already in raw format: %s", filePath)
		return filePath, nil
	}

	log.Infof("Converting %s image to raw format: %s", sourceFormat, filePath)

	fileName := filepath.Base(filePath)
	fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	outputFilePath := filepath.Join(outputDir, fileNameWithoutExt+".raw")

	// Convert to raw using qemu-img
	cmdStr := fmt.Sprintf("qemu-img convert -O raw %s %s", filePath, outputFilePath)
	_, err = shell.ExecCmd(cmdStr, false, shell.HostPath, nil)
	if err != nil {
		log.Errorf("Failed to convert %s to raw: %v", sourceFormat, err)
		return "", fmt.Errorf("failed to convert %s to raw: %w", sourceFormat, err)
	}

	log.Infof("Successfully converted to raw: %s", outputFilePath)
	return outputFilePath, nil
}

func convertImageFile(filePath, imageType string) (string, error) {
	var cmdStr string

	fileDir := filepath.Dir(filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Errorf("Image file does not exist: %s", filePath)
		return "", fmt.Errorf("image file does not exist: %s", filePath)
	}

	log.Infof("Converting image file %s to type %s", filePath, imageType)

	// Detect source format for better conversion handling
	sourceFormat, err := DetectImageFormat(filePath)
	if err != nil {
		log.Warnf("Failed to detect source format, assuming raw: %v", err)
		sourceFormat = "raw"
	}

	// If source is already the target format, skip conversion
	if sourceFormat == imageType {
		log.Infof("Image is already in %s format, skipping conversion", imageType)
		return filePath, nil
	}

	// Skip trimming for now to avoid file locking conflicts
	// The -S 4k flag in qemu-img convert will handle sparse optimization
	log.Debugf("Skipping pre-conversion trimming to avoid file lock conflicts")

	fileName := filepath.Base(filePath)
	fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	outputFilePath := filepath.Join(fileDir, fileNameWithoutExt+"."+imageType)

	switch imageType {
	case "raw":
		cmdStr = fmt.Sprintf("qemu-img convert -O raw %s %s", filePath, outputFilePath)
	case "vhd":
		cmdStr = fmt.Sprintf("qemu-img convert -O vpc %s %s", filePath, outputFilePath)
	case "vhdx":
		cmdStr = fmt.Sprintf("qemu-img convert -O vhdx %s %s", filePath, outputFilePath)
	case "qcow2":
		cmdStr = fmt.Sprintf("qemu-img convert -O qcow2 -c -S 4k -p -o cluster_size=2M,lazy_refcounts=on %s %s", filePath, outputFilePath)
	case "vmdk":
		cmdStr = fmt.Sprintf("qemu-img convert -O vmdk %s %s", filePath, outputFilePath)
	case "vdi":
		cmdStr = fmt.Sprintf("qemu-img convert -O vdi %s %s", filePath, outputFilePath)
	default:
		log.Errorf("Unsupported image type: %s", imageType)
		return outputFilePath, fmt.Errorf("unsupported image type: %s", imageType)
	}

	_, err = shell.ExecCmd(cmdStr, false, shell.HostPath, nil)
	if err != nil {
		log.Errorf("Failed to convert image file to %s: %v", imageType, err)
		return outputFilePath, fmt.Errorf("failed to convert image file to %s: %w", imageType, err)
	}

	return outputFilePath, nil
}

func compressImageFile(filePath, compressionType string) error {
	log.Infof("Compressing image file %s with %s", filePath, compressionType)

	if err := compression.CompressFile(filePath, filePath+"."+compressionType, compressionType, false); err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}
	if err := os.Remove(filePath); err != nil {
		log.Warnf("Failed to remove uncompressed image file: %v", err)
	}
	return nil
}

// trimUnusedSpace attempts to reduce image size by zeroing unused space
func trimUnusedSpace(filePath string) error {
	log.Infof("Attempting to trim unused space in image file: %s", filePath)

	// Method 1: Try virt-sparsify if available (most effective)
	if _, err := shell.ExecCmd("which virt-sparsify", false, shell.HostPath, nil); err == nil {
		tempFile := filePath + ".sparse"
		sparsifyCmd := fmt.Sprintf("virt-sparsify --in-place %s", filePath)
		if _, err := shell.ExecCmd(sparsifyCmd, true, shell.HostPath, nil); err == nil {
			log.Infof("Successfully sparsified image using virt-sparsify")
			return nil
		}
		log.Warnf("virt-sparsify failed, trying alternative methods: %v", err)
		os.Remove(tempFile) // Clean up on failure
	}

	// Method 2: Use qemu-img convert with sparse detection (fallback)
	return sparsifyWithQemuImg(filePath)
}

// sparsifyWithQemuImg uses qemu-img to create a sparse version of the image
func sparsifyWithQemuImg(filePath string) error {
	// Check file size first - skip sparse processing for very small files (< 1MB)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Skip sparsification for small test files to avoid qemu-img issues
	if fileInfo.Size() < 1024*1024 {
		log.Debugf("Skipping sparsification for small file (%d bytes): %s", fileInfo.Size(), filePath)
		return nil
	}

	tempFile := filePath + ".tmp"

	// Convert image to itself without -S flag to avoid size parameter issues
	// qemu-img automatically detects and optimizes sparse regions
	convertCmd := fmt.Sprintf("qemu-img convert -O raw %s %s", filePath, tempFile)
	if _, err := shell.ExecCmd(convertCmd, true, shell.HostPath, nil); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to sparsify image: %w", err)
	}

	// Replace original file with sparsified version
	if err := os.Rename(tempFile, filePath); err != nil {
		os.Remove(tempFile) // Clean up on error
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	log.Infof("Successfully sparsified image using qemu-img: %s", filePath)
	return nil
}
