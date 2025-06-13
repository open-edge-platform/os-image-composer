package rawmaker

import (
	"fmt"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
)

func BuildRawImage(template *config.ImageTemplate) error {
	filePath := "" // This is a placeholder for the file path where the raw image will be created.
	err := imagedisc.CreateRawImage(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to create raw image: %w", err)
	}
	err = imageos.InstallImageOs(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to install image OS: %w", err)
	}
	err = imageboot.InstallImageBoot(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to install image boot: %w", err)
	}
	err = imageconvert.ConvertImageFile(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to convert image file: %w", err)
	}
	return nil
}
