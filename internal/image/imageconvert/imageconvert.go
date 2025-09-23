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

func convertImageFile(filePath, imageType string) (string, error) {
	var cmdStr string

	fileDir := filepath.Dir(filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Errorf("Image file does not exist: %s", filePath)
		return "", fmt.Errorf("image file does not exist: %s", filePath)
	}

	log.Infof("Converting image file %s to type %s", filePath, imageType)

	fileName := filepath.Base(filePath)
	fileNameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	outputFilePath := filepath.Join(fileDir, fileNameWithoutExt+"."+imageType)

	switch imageType {
	case "vhd":
		cmdStr = fmt.Sprintf("qemu-img convert -O vpc %s %s", filePath, outputFilePath)
	case "vhdx":
		cmdStr = fmt.Sprintf("qemu-img convert -O vhdx %s %s", filePath, outputFilePath)
	case "qcow2":
		cmdStr = fmt.Sprintf("qemu-img convert -O qcow2 %s %s", filePath, outputFilePath)
	case "vmdk":
		cmdStr = fmt.Sprintf("qemu-img convert -O vmdk %s %s", filePath, outputFilePath)
	case "vdi":
		cmdStr = fmt.Sprintf("qemu-img convert -O vdi %s %s", filePath, outputFilePath)
	default:
		log.Errorf("Unsupported image type: %s", imageType)
		return outputFilePath, fmt.Errorf("unsupported image type: %s", imageType)
	}

	_, err := shell.ExecCmd(cmdStr, false, "", nil)
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
