package isomaker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var (
	ImageBuildDir string
)

func initIsoMakerWorkspace() error {
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

func BuildISOImage(template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Building ISO image for: %s", template.GetImageName())

	if err := initIsoMakerWorkspace(); err != nil {
		return fmt.Errorf("failed to initialize ISO maker workspace: %w", err)
	}

	imageName := template.GetImageName()
	sysConfigName := template.GetSystemConfigName()
	isoFilePath := filepath.Join(ImageBuildDir, sysConfigName, fmt.Sprintf("%s.iso", imageName))
	initrdFilePath := filepath.Join(ImageBuildDir, sysConfigName, "iso-initrd.img")

	log.Infof("Creating ISO Initrd image...")
	initrdRootfsPath, err := buildISOInitrd(initrdFilePath)
	if err != nil {
		return fmt.Errorf("failed to build ISO initrd: %v", err)
	}

	log.Infof("Creating ISO image...")
	if err := createISO(template, initrdRootfsPath, initrdFilePath, isoFilePath); err != nil {
		return fmt.Errorf("failed to create ISO image: %v", err)
	}
	return nil
}

func buildISOInitrd(initrdFilePath string) (string, error) {
	initrdTemplate, err := getInitrdTemplate()
	if err != nil {
		return "", fmt.Errorf("failed to get initrd template: %v", err)
	}
	if err := downloadInitrdPkgs(initrdTemplate); err != nil {
		return "", fmt.Errorf("failed to download initrd packages: %v", err)
	}
	initrdRootfsPath, err := imageos.InstallInitrd(initrdTemplate)
	if err != nil {
		return initrdRootfsPath, fmt.Errorf("failed to install initrd: %v", err)
	}
	if err := createInitrdImg(initrdRootfsPath, initrdFilePath); err != nil {
		return initrdRootfsPath, fmt.Errorf("failed to create initrd image: %v", err)
	}
	return initrdRootfsPath, nil
}

func getInitrdTemplate() (*config.ImageTemplate, error) {
	targetOsConfigDir, err := file.GetTargetOsConfigDir(config.TargetOs, config.TargetDist)
	if err != nil {
		return nil, fmt.Errorf("failed to get target OS config directory: %v", err)
	}
	initrdTemplateFile := filepath.Join(targetOsConfigDir, "imageconfigs", "defaultconfigs",
		"default-iso-initrd-"+config.TargetArch+".yml")
	if _, err := os.Stat(initrdTemplateFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("initrd template file does not exist: %s", initrdTemplateFile)
	}
	initrdTemplate, err := config.LoadTemplate(initrdTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load initrd template: %v", err)
	}
	return initrdTemplate, nil
}

func downloadInitrdPkgs(initrdTemplate *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Downloading packages for: %s", initrdTemplate.GetImageName())

	pkgList := initrdTemplate.GetPackages()
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	pkgCacheDir := filepath.Join(globalCache, "pkgCache", config.ProviderId)
	_, err = rpmutils.DownloadPackages(pkgList, pkgCacheDir, "")
	if err != nil {
		return fmt.Errorf("failed to download initrd packages: %v", err)
	}
	// From local.repo
	chrootRepoDir := filepath.Join("/workspace", "cache-repo")
	if err := chroot.UpdateChrootLocalRPMRepo(chrootRepoDir); err != nil {
		return fmt.Errorf("failed to update chroot local cache repository %s: %w", chrootRepoDir, err)
	}
	return nil
}

func createInitrdImg(initrdRootfsPath string, outputPath string) error {
	cmdStr := fmt.Sprintf("cd %s && sudo find . | sudo cpio -o -H newc | sudo gzip > %s",
		initrdRootfsPath, outputPath)
	if _, err := shell.ExecCmdWithStream(cmdStr, false, "", nil); err != nil {
		return fmt.Errorf("failed to create initrd image: %v", err)
	}
	return nil
}

func createISO(template *config.ImageTemplate, initrdRootfsPath, initrdFilePath, isoFilePath string) error {
	return nil
}
