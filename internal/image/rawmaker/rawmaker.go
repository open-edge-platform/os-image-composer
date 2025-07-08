package rawmaker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/config"

	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

var (
	ImageBuildDir string
)

func initRawMakerWorkspace() error {
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %v", err)
	}
	ImageBuildDir = filepath.Join(globalWorkDir, config.ProviderId, "imagebuild")
	if _, err := os.Stat(ImageBuildDir); os.IsNotExist(err) {
		if err = os.MkdirAll(ImageBuildDir, 0755); err != nil {
			return fmt.Errorf("failed to create imagebuild directory: %w", err)
		}
	}
	return nil
}

func BuildRawImage(template *config.ImageTemplate) error {
	var err error
	log := logger.Logger()
	log.Infof("Building raw image for: %s", template.GetImageName())

	if err := initRawMakerWorkspace(); err != nil {
		return fmt.Errorf("failed to initialize raw maker workspace: %w", err)
	}
	imageName := template.GetImageName()
	sysConfigName := template.GetSystemConfigName()
	filePath := filepath.Join(ImageBuildDir, sysConfigName, fmt.Sprintf("%s.raw", imageName))

	log.Infof("Creating raw image disk...")
	loopDevPath, diskPathIdMap, err := imagedisc.CreateRawImage(filePath, template)
	if err != nil {
		err = fmt.Errorf("failed to create raw image: %w", err)
		if loopDevPath != "" {
			goto fail
		} else {
			return err
		}
	}

	err = imageos.InstallImageOs(diskPathIdMap, template)
	if err != nil {
		err = fmt.Errorf("failed to install image OS: %w", err)
		goto fail
	}

	err = imagedisc.LoopSetupDelete(loopDevPath)
	if err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}

	err = imageconvert.ConvertImageFile(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to convert image file: %w", err)
	}
	return nil

fail:
	detachErr := imagedisc.LoopSetupDelete(loopDevPath)
	if detachErr != nil {
		log.Errorf("Failed to detach loopback device after error: %v", detachErr)
	}
	return fmt.Errorf("error during raw image build: %w", err)
}
