package rawmaker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"

	"github.com/open-edge-platform/image-composer/internal/image/imageconvert"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"github.com/open-edge-platform/image-composer/internal/utils/system"
)

type RawMakerInterface interface {
	Init(template *config.ImageTemplate) error
	BuildRawImage(template *config.ImageTemplate) error
}

type RawMaker struct {
	ImageBuildDir string
	ChrootEnv     chroot.ChrootEnvInterface
	LoopDev       imagedisc.LoopDevInterface
	ImageOs       imageos.ImageOsInterface
	ImageConvert  imageconvert.ImageConvertInterface
}

var log = logger.Logger()

func NewRawMaker(chrootEnv chroot.ChrootEnvInterface) (*RawMaker, error) {
	return &RawMaker{
		ChrootEnv: chrootEnv,
		LoopDev:   imagedisc.NewLoopDev(),
	}, nil
}

func (rawMaker *RawMaker) Init(template *config.ImageTemplate) error {
	imageOs, err := imageos.NewImageOs(rawMaker.ChrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create image OS instance: %w", err)
	}
	rawMaker.ImageOs = imageOs

	imageConvert := imageconvert.NewImageConvert()
	rawMaker.ImageConvert = imageConvert

	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %w", err)
	}

	providerId := system.GetProviderId(template.Target.OS, template.Target.Dist,
		template.Target.Arch)
	rawMaker.ImageBuildDir = filepath.Join(globalWorkDir, providerId, "imagebuild")
	if err := os.MkdirAll(rawMaker.ImageBuildDir, 0700); err != nil {
		log.Errorf("Failed to create imagebuild directory %s: %v", rawMaker.ImageBuildDir, err)
		return fmt.Errorf("failed to create imagebuild directory: %w", err)
	}

	return nil
}

func (rawMaker *RawMaker) cleanupOnSuccess(loopDevPath string, err *error) {
	if loopDevPath != "" {
		if detachErr := rawMaker.LoopDev.LoopSetupDelete(loopDevPath); detachErr != nil {
			log.Errorf("Failed to detach loopback device: %v", detachErr)
			*err = fmt.Errorf("failed to detach loopback device: %w", detachErr)
		}
	}
}

func (rawMaker *RawMaker) cleanupOnError(loopDevPath, imagePath string, err *error) {
	if loopDevPath != "" {
		detachErr := rawMaker.LoopDev.LoopSetupDelete(loopDevPath)
		if detachErr != nil {
			log.Errorf("Failed to detach loopback device after error: %v", detachErr)
			*err = fmt.Errorf("operation failed: %w, cleanup errors: %v", *err, detachErr)
			return
		}
	}
	if _, statErr := os.Stat(imagePath); statErr == nil {
		if _, rmErr := shell.ExecCmd(fmt.Sprintf("rm -f %s", imagePath), true, "", nil); rmErr != nil {
			log.Errorf("Failed to remove raw image file %s after error: %v", imagePath, rmErr)
			*err = fmt.Errorf("operation failed: %w, cleanup errors: %v", *err, rmErr)
		}
	}
}

func (rawMaker *RawMaker) BuildRawImage(template *config.ImageTemplate) (err error) {
	var versionInfo string
	var newFilePath string

	log.Infof("Building raw image for: %s", template.GetImageName())
	imageName := template.GetImageName()
	sysConfigName := template.GetSystemConfigName()
	filePath := filepath.Join(rawMaker.ImageBuildDir, sysConfigName, fmt.Sprintf("%s.raw", imageName))

	log.Infof("Creating raw image disk...")
	loopDevPath, diskPathIdMap, err := rawMaker.LoopDev.CreateRawImageLoopDev(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to create raw image: %w", err)
	}

	defer func() {
		if err != nil {
			rawMaker.cleanupOnError(loopDevPath, filePath, &err)
		} else {
			rawMaker.cleanupOnSuccess(loopDevPath, &err)
		}
	}()

	versionInfo, err = rawMaker.ImageOs.InstallImageOs(diskPathIdMap)
	if err != nil {
		return fmt.Errorf("failed to install image OS: %w", err)
	}

	err = rawMaker.LoopDev.LoopSetupDelete(loopDevPath)
	loopDevPath = ""
	if err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}

	newFilePath = filepath.Join(rawMaker.ImageBuildDir, sysConfigName, fmt.Sprintf("%s-%s.raw", imageName, versionInfo))
	if _, err := shell.ExecCmd(fmt.Sprintf("mv %s %s", filePath, newFilePath), true, "", nil); err != nil {
		log.Errorf("Failed to rename raw image file: %v", err)
		return fmt.Errorf("failed to rename raw image file: %w", err)
	}
	filePath = newFilePath

	err = rawMaker.ImageConvert.ConvertImageFile(filePath, template)
	if err != nil {
		return fmt.Errorf("failed to convert image file: %w", err)
	}
	return nil
}
