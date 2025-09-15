package initrdmaker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"github.com/open-edge-platform/image-composer/internal/utils/system"
)

type InitrdMakerInterface interface {
	Init(template *config.ImageTemplate) error
	DownloadInitrdPkgs(template *config.ImageTemplate) error
	BuildInitrdImage(template *config.ImageTemplate) error
	GetInitrdVersion() string
	GetInitrdFilePath() string
	GetInitrdRootfsPath() string
	CleanInitrdRootfs() error
}

type InitrdMaker struct {
	ImageBuildDir    string
	InitrdRootfsPath string
	InitrdFilePath   string
	VersionInfo      string
	ChrootEnv        chroot.ChrootEnvInterface
	ImageOs          imageos.ImageOsInterface
}

var log = logger.Logger()

func NewInitrdMaker(chrootEnv chroot.ChrootEnvInterface) (*InitrdMaker, error) {
	return &InitrdMaker{
		ChrootEnv: chrootEnv,
	}, nil
}

func (initrdMaker *InitrdMaker) Init(template *config.ImageTemplate) error {
	imageOs, err := imageos.NewImageOs(initrdMaker.ChrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create image OS instance: %w", err)
	}
	initrdMaker.ImageOs = imageOs

	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %w", err)
	}

	providerId := system.GetProviderId(template.Target.OS, template.Target.Dist,
		template.Target.Arch)
	initrdMaker.ImageBuildDir = filepath.Join(globalWorkDir, providerId, "imagebuild")
	if err := os.MkdirAll(initrdMaker.ImageBuildDir, 0700); err != nil {
		log.Errorf("Failed to create imagebuild directory %s: %v", initrdMaker.ImageBuildDir, err)
		return fmt.Errorf("failed to create imagebuild directory: %w", err)
	}

	return nil
}

func (initrdMaker *InitrdMaker) GetInitrdVersion() string {
	return initrdMaker.VersionInfo
}

func (initrdMaker *InitrdMaker) GetInitrdFilePath() string {
	return initrdMaker.InitrdFilePath
}

func (initrdMaker *InitrdMaker) GetInitrdRootfsPath() string {
	return initrdMaker.InitrdRootfsPath
}

func (initrdMaker *InitrdMaker) DownloadInitrdPkgs(template *config.ImageTemplate) error {
	log.Infof("Downloading packages for: %s", template.GetImageName())

	pkgList := template.GetPackages()
	pkgType := initrdMaker.ChrootEnv.GetTargetOsPkgType()
	if pkgType == "deb" {
		_, err := debutils.DownloadPackages(pkgList, initrdMaker.ChrootEnv.GetChrootPkgCacheDir(), "")
		if err != nil {
			return fmt.Errorf("failed to download initrd packages: %w", err)
		}
	} else if pkgType == "rpm" {
		_, err := rpmutils.DownloadPackages(pkgList, initrdMaker.ChrootEnv.GetChrootPkgCacheDir(), "")
		if err != nil {
			return fmt.Errorf("failed to download initrd packages: %w", err)
		}
	}

	if err := initrdMaker.ChrootEnv.RefreshLocalCacheRepo(template.Target.Arch); err != nil {
		return fmt.Errorf("failed to refresh local cache repository: %w", err)
	}
	return nil
}

func (initrdMaker *InitrdMaker) BuildInitrdImage(template *config.ImageTemplate) (err error) {
	log.Infof("Building initrd image for: %s", template.GetImageName())

	imageName := template.GetImageName()
	sysConfigName := template.GetSystemConfigName()

	initrdMaker.InitrdRootfsPath, initrdMaker.VersionInfo, err = initrdMaker.ImageOs.InstallInitrd()
	if err != nil {
		return fmt.Errorf("failed to install initrd: %w", err)
	}

	initrdFileDir := filepath.Join(initrdMaker.ImageBuildDir, sysConfigName)
	if err := os.MkdirAll(initrdFileDir, 0700); err != nil {
		return fmt.Errorf("failed to create initrd image directory: %w", err)
	}

	initrdMaker.InitrdFilePath = filepath.Join(initrdFileDir, fmt.Sprintf("%s-%s.img",
		imageName, initrdMaker.VersionInfo))

	if err := addInitScriptsToInitrd(initrdMaker.InitrdRootfsPath); err != nil {
		return fmt.Errorf("failed to add init scripts to initrd: %w", err)
	}

	if err := initrdMaker.createInitrdImg(); err != nil {
		return fmt.Errorf("failed to create initrd image: %w", err)
	}
	return nil
}

func addInitScriptsToInitrd(initrdRootfsPath string) error {
	log.Infof("Adding init scripts to initrd...")

	generalConfigDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	rcLocalSrc := filepath.Join(generalConfigDir, "isolinux", "rc.local")
	if _, err := os.Stat(rcLocalSrc); os.IsNotExist(err) {
		log.Errorf("rc.local file does not exist: %s", rcLocalSrc)
		return fmt.Errorf("rc.local file does not exist: %s", rcLocalSrc)
	}

	rcLocalDest := filepath.Join(initrdRootfsPath, "etc", "rc.d", "rc.local")
	return file.CopyFile(rcLocalSrc, rcLocalDest, "--preserve=mode", true)
}

func (initrdMaker *InitrdMaker) createInitrdImg() error {
	cmdStr := fmt.Sprintf("cd %s && sudo find . | sudo cpio -o -H newc | sudo gzip > %s",
		initrdMaker.InitrdRootfsPath, initrdMaker.InitrdFilePath)
	if _, err := shell.ExecCmdWithStream(cmdStr, false, "", nil); err != nil {
		log.Errorf("Failed to create initrd image: %v", err)
		return fmt.Errorf("failed to create initrd image: %w", err)
	}
	if _, err := os.Stat(initrdMaker.InitrdFilePath); os.IsNotExist(err) {
		log.Errorf("Initrd image file does not exist: %s", initrdMaker.InitrdFilePath)
		return fmt.Errorf("initrd image file does not exist: %s", initrdMaker.InitrdFilePath)
	}
	return nil
}

func (initrdMaker *InitrdMaker) CleanInitrdRootfs() error {
	log.Infof("Cleaning up initrd rootfs: %s", initrdMaker.InitrdRootfsPath)

	if initrdMaker.InitrdRootfsPath == "" {
		log.Debugf("Initrd rootfs path is empty, nothing to clean")
		return nil
	}

	if _, err := os.Stat(initrdMaker.InitrdRootfsPath); os.IsNotExist(err) {
		log.Debugf("Initrd rootfs path does not exist: %s", initrdMaker.InitrdRootfsPath)
		return nil
	}

	if err := mount.UmountPath(initrdMaker.InitrdRootfsPath + "/cdrom/cache-repo"); err != nil {
		log.Errorf("Failed to unmount cache-repo %s: %v",
			initrdMaker.InitrdRootfsPath+"/cdrom/cache-repo", err)
		return fmt.Errorf("failed to unmount cache-repo %s: %w",
			initrdMaker.InitrdRootfsPath+"/cdrom/cache-repo", err)
	}

	// Remove the initrd rootfs directory
	if _, err := shell.ExecCmd("rm -rf "+initrdMaker.InitrdRootfsPath, true, "", nil); err != nil {
		log.Errorf("Failed to remove initrd rootfs directory %s: %v",
			initrdMaker.InitrdRootfsPath, err)
		return fmt.Errorf("failed to remove initrd rootfs directory: %w", err)
	}

	return nil
}
