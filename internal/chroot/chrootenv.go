package chroot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/chroot/chrootbuild"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/compression"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"github.com/open-edge-platform/image-composer/internal/utils/system"
)

const (
	ChrootRepoDir       = "/cdrom/cache-repo"
	RPMRepoConfigDir    = "/etc/yum.repos.d/"
	DEBRepoConfigDir    = "/etc/apt/sources.list.d/"
	ResolvConfPath      = "/etc/resolv.conf"
	DefaultArchitecture = "amd64"
)

var log = logger.Logger()

type ChrootEnvInterface interface {
	GetChrootEnvRoot() string
	GetChrootImageBuildDir() string
	GetTargetOsPkgType() string
	GetTargetOsConfigDir() string
	GetTargetOsReleaseVersion() string
	GetChrootPkgCacheDir() string
	GetChrootEnvEssentialPackageList() ([]string, error)
	GetChrootEnvHostPath(chrootPath string) (string, error)
	GetChrootEnvPath(ChrootEnvHostPath string) (string, error)
	MountChrootSysfs(chrootPath string) error
	UmountChrootSysfs(chrootPath string) error
	MountChrootPath(hostFullPath, chrootPath, mountFlags string) error
	UmountChrootPath(chrootPath string) error
	CopyFileFromHostToChroot(hostFilePath, chrootPath string) error
	CopyFileFromChrootToHost(hostFilePath, chrootPath string) error
	RefreshLocalCacheRepo(targetArch string) error
	InitChrootEnv(targetOs, targetDist, targetArch string) error
	CleanupChrootEnv(targetOs, targetDist, targetArch string) error
	TdnfInstallPackage(packageName, installRoot string, repositoryIDList []string) error
	AptInstallPackage(packageName, installRoot string, repoSrcList []string) error
}

type ChrootEnv struct {
	ChrootEnvRoot       string
	ChrootImageBuildDir string
	ChrootBuilder       chrootbuild.ChrootBuilderInterface
}

func NewChrootEnv(targetOs, targetDist, targetArch string) (*ChrootEnv, error) {
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get global work directory: %v", err)
	}
	providerId := system.GetProviderId(targetOs, targetDist, targetArch)
	chrootEnvRoot := filepath.Join(globalWorkDir, providerId, "chrootenv")
	if _, err := os.Stat(chrootEnvRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(chrootEnvRoot, 0700); err != nil {
			return nil, fmt.Errorf("failed to create chroot environment root directory: %w", err)
		}
	}

	chrootBuilder, err := chrootbuild.NewChrootBuilder(targetOs, targetDist, targetArch)
	if err != nil {
		return nil, fmt.Errorf("failed to create chroot builder: %w", err)
	}

	return &ChrootEnv{
		ChrootEnvRoot: chrootEnvRoot,
		ChrootBuilder: chrootBuilder,
	}, nil
}

func (chrootEnv *ChrootEnv) GetChrootEnvRoot() string {
	return chrootEnv.ChrootEnvRoot
}

func (chrootEnv *ChrootEnv) GetChrootImageBuildDir() string {
	return chrootEnv.ChrootImageBuildDir
}

func (chrootEnv *ChrootEnv) GetTargetOsPkgType() string {
	return chrootEnv.ChrootBuilder.GetTargetOsPkgType()
}

func (chrootEnv *ChrootEnv) GetTargetOsConfigDir() string {
	return chrootEnv.ChrootBuilder.GetTargetOsConfigDir()
}

func (chrootEnv *ChrootEnv) GetTargetOsReleaseVersion() string {
	targetOsConfig := chrootEnv.ChrootBuilder.GetTargetOsConfig()
	releaseVersion, ok := targetOsConfig["releaseVersion"]
	if !ok {
		log.Errorf("releaseVersion not found in target OS config")
		return "unknown"
	}
	if s, ok := releaseVersion.(string); ok {
		return s
	}
	log.Errorf("releaseVersion is not a string")
	return "unknown"
}

func (chrootEnv *ChrootEnv) GetChrootPkgCacheDir() string {
	return chrootEnv.ChrootBuilder.GetChrootPkgCacheDir()
}

func (chrootEnv *ChrootEnv) GetChrootEnvEssentialPackageList() ([]string, error) {
	return chrootEnv.ChrootBuilder.GetChrootEnvEssentialPackageList()
}

func (chrootEnv *ChrootEnv) GetChrootEnvHostPath(chrootPath string) (string, error) {
	if strings.Contains(chrootPath, "..") {
		return "", fmt.Errorf("path contains invalid characters: %s", chrootPath)
	}

	if chrootEnv.ChrootEnvRoot == "" {
		log.Errorf("Chroot env is not initialized")
		return "", fmt.Errorf("chroot env is not initialized")
	}
	return filepath.Join(chrootEnv.ChrootEnvRoot, chrootPath), nil
}

func (chrootEnv *ChrootEnv) GetChrootEnvPath(ChrootEnvHostPath string) (string, error) {
	if chrootEnv.ChrootEnvRoot == "" {
		log.Errorf("Chroot env is not initialized")
		return "", fmt.Errorf("chroot env is not initialized")
	}
	isSubPath, err := file.IsSubPath(chrootEnv.ChrootEnvRoot, ChrootEnvHostPath)
	if err != nil {
		log.Errorf("Failed to check if path %s is a subpath of chroot env root %s: %v", ChrootEnvHostPath, chrootEnv.ChrootEnvRoot, err)
		return "", fmt.Errorf("failed to check if path %s is a subpath of chroot env root %s: %w",
			ChrootEnvHostPath, chrootEnv.ChrootEnvRoot, err)
	}
	if !isSubPath {
		return "", fmt.Errorf("path %s is not a subpath of chroot env root %s", ChrootEnvHostPath, chrootEnv.ChrootEnvRoot)
	}

	chrootPath := ChrootEnvHostPath[len(chrootEnv.ChrootEnvRoot):]
	if strings.HasPrefix(chrootPath, "/") {
		return chrootPath, nil
	} else {
		return filepath.Join("/", chrootPath), nil
	}
}

func (chrootEnv *ChrootEnv) MountChrootSysfs(chrootPath string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	return mount.MountSysfs(chrootHostPath)
}

func (chrootEnv *ChrootEnv) UmountChrootSysfs(chrootPath string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}

	if err := system.StopGPGComponents(chrootHostPath); err != nil {
		return fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
	}

	if err = mount.UmountSysfs(chrootHostPath); err != nil {
		return fmt.Errorf("failed to unmount sysfs for %s: %w", chrootHostPath, err)
	}
	return nil
}

// MountChrootPath mounts a host path to a chroot path
func (chrootEnv *ChrootEnv) MountChrootPath(hostFullPath, chrootPath, mountFlags string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	if _, err := os.Stat(chrootHostPath); os.IsNotExist(err) {
		if _, err = shell.ExecCmd("mkdir -p "+chrootHostPath, true, "", nil); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", chrootHostPath, err)
		}
	}
	return mount.MountPath(hostFullPath, chrootHostPath, mountFlags)
}

// UmountChrootPath unmounts a chroot path
func (chrootEnv *ChrootEnv) UmountChrootPath(chrootPath string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}

	return mount.UmountPath(chrootHostPath)
}

// CopyFileFromHostToChroot copies a file from the host to the chroot environment
func (chrootEnv *ChrootEnv) CopyFileFromHostToChroot(hostFilePath, chrootPath string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	return file.CopyFile(hostFilePath, chrootHostPath, "-f", true)
}

// CopyFileFromChrootToHost copies a file from the chroot environment to the host
func (chrootEnv *ChrootEnv) CopyFileFromChrootToHost(hostFilePath, chrootPath string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	return file.CopyFile(chrootHostPath, hostFilePath, "-f", true)
}

func (chrootEnv *ChrootEnv) updateChrootLocalRPMRepo(chrootRepoDir string) error {
	chrootHostPath, err := chrootEnv.GetChrootEnvHostPath(chrootRepoDir)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootRepoDir, err)
	}
	if _, err := os.Stat(chrootHostPath); os.IsNotExist(err) {
		return fmt.Errorf("chroot repo directory not existing%s: %w", chrootHostPath, err)
	}
	cmd := fmt.Sprintf("createrepo_c --compatibility --update %s", chrootRepoDir)
	if _, err = shell.ExecCmd(cmd, false, chrootEnv.ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to update chroot local cache repository: %w", err)
	}
	return nil
}

func (chrootEnv *ChrootEnv) updateChrootLocalDebRepo(chrootPkgCacheDir, targetArch string) error {
	return chrootEnv.ChrootBuilder.UpdateLocalDebRepo(chrootPkgCacheDir, targetArch)
}

func (chrootEnv *ChrootEnv) RefreshLocalCacheRepo(targetArch string) error {
	// From local.repo
	pkgType := chrootEnv.GetTargetOsPkgType()
	if pkgType == "rpm" {
		releaseVersion := chrootEnv.GetTargetOsReleaseVersion()
		if err := chrootEnv.updateChrootLocalRPMRepo(ChrootRepoDir); err != nil {
			return fmt.Errorf("failed to update rpm local cache repository %s: %w", ChrootRepoDir, err)
		}

		cmd := fmt.Sprintf("tdnf makecache --releasever %s", releaseVersion)
		if _, err := shell.ExecCmdWithStream(cmd, true, chrootEnv.ChrootEnvRoot, nil); err != nil {
			return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
		}
	} else if pkgType == "deb" {
		chrootPkgCacheDir := chrootEnv.GetChrootPkgCacheDir()
		if err := chrootEnv.updateChrootLocalDebRepo(chrootPkgCacheDir, targetArch); err != nil {
			return fmt.Errorf("failed to update debian local cache repository: %v", err)
		}

		cmd := "apt clean"
		if _, err := shell.ExecCmdWithStream(cmd, true, chrootEnv.ChrootEnvRoot, nil); err != nil {
			return fmt.Errorf("failed to clean cache for chroot repository: %w", err)
		}

		cmd = "apt update"
		if _, err := shell.ExecCmdWithStream(cmd, true, chrootEnv.ChrootEnvRoot, nil); err != nil {
			return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}
	return nil
}

func (chrootEnv *ChrootEnv) initChrootLocalRepo(targetArch string) error {
	chrootPkgCacheDir := chrootEnv.GetChrootPkgCacheDir()
	if err := chrootEnv.MountChrootPath(chrootPkgCacheDir, ChrootRepoDir, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory %s to chroot repo directory %s: %w",
			chrootPkgCacheDir, ChrootRepoDir, err)
	}

	if err := chrootEnv.RefreshLocalCacheRepo(targetArch); err != nil {
		return fmt.Errorf("failed to refresh local cache repository: %w", err)
	}
	return nil
}

func (chrootEnv *ChrootEnv) createChrootRepo(targetOs, targetDist string) error {
	targetOsConfigDir := chrootEnv.GetTargetOsConfigDir()
	pkgType := chrootEnv.GetTargetOsPkgType()
	if pkgType == "rpm" {
		chrootRepoCongfigPath := filepath.Join(targetOsConfigDir, "chrootenvconfigs", "local.repo")
		if _, err := os.Stat(chrootRepoCongfigPath); os.IsNotExist(err) {
			return fmt.Errorf("chroot repo config file does not exist: %s", chrootRepoCongfigPath)
		}

		if err := chrootEnv.CopyFileFromHostToChroot(chrootRepoCongfigPath, RPMRepoConfigDir); err != nil {
			return fmt.Errorf("failed to copy local.repo: %w", err)
		}
	} else if pkgType == "deb" {
		chrootRepoCongfigPath, err := chrootEnv.GetChrootEnvHostPath(DEBRepoConfigDir)
		if err != nil {
			return fmt.Errorf("failed to get chroot host path for local repo config: %w", err)
		}
		if _, err := shell.ExecCmd("rm -f "+chrootRepoCongfigPath+"/*", true, "", nil); err != nil {
			return fmt.Errorf("failed to remove existing local repo config files: %w", err)
		}

		RepoCongfigPath := filepath.Join(targetOsConfigDir, "chrootenvconfigs", "local.list")
		if _, err := os.Stat(RepoCongfigPath); os.IsNotExist(err) {
			return fmt.Errorf("chroot repo config file does not exist: %s", RepoCongfigPath)
		}

		if err := chrootEnv.CopyFileFromHostToChroot(RepoCongfigPath, DEBRepoConfigDir); err != nil {
			return fmt.Errorf("failed to copy local.repo: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}

	return nil
}

func (chrootEnv *ChrootEnv) initChrootWorkspace() error {
	chrootWorkspace := filepath.Join(chrootEnv.ChrootEnvRoot, "workspace")
	chrootEnv.ChrootImageBuildDir = filepath.Join(chrootWorkspace, "imagebuild")
	if _, err := os.Stat(chrootEnv.ChrootImageBuildDir); os.IsNotExist(err) {
		if _, err = shell.ExecCmd("mkdir -p "+chrootEnv.ChrootImageBuildDir, true, "", nil); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", chrootEnv.ChrootImageBuildDir, err)
		}
	}
	return nil
}

func (chrootEnv *ChrootEnv) InitChrootEnv(targetOs, targetDist, targetArch string) (err error) {
	var chrootRootfsExist bool = true

	if files, _ := os.ReadDir(chrootEnv.ChrootEnvRoot); len(files) == 0 {
		chrootRootfsExist = false
		chrootBuildDir := chrootEnv.ChrootBuilder.GetChrootBuildDir()
		chrootEnvTarPath := filepath.Join(chrootBuildDir, "chrootenv.tar.gz")
		if _, err := os.Stat(chrootEnvTarPath); os.IsNotExist(err) {
			// Build chroot environment tarball
			if err = chrootEnv.ChrootBuilder.BuildChrootEnv(targetOs, targetDist, targetArch); err != nil {
				return fmt.Errorf("failed to build chroot environment: %w", err)
			}
		}

		// Extract chroot environment tarball
		if err = compression.DecompressFile(chrootEnvTarPath, chrootEnv.ChrootEnvRoot, "tar.gz", true); err != nil {
			return fmt.Errorf("failed to extract chroot environment tarball: %w", err)
		}

		// Copy resolv.conf to the chroot environment
		if err = chrootEnv.CopyFileFromHostToChroot(ResolvConfPath, "/etc/"); err != nil {
			return fmt.Errorf("failed to copy resolv.conf: %w", err)
		}
	}

	// Initialize the chroot workspace
	if err = chrootEnv.initChrootWorkspace(); err != nil {
		return fmt.Errorf("failed to initialize chroot workspace: %w", err)
	}

	// Mount sysfs to the chroot environment
	err = chrootEnv.MountChrootSysfs("/")
	if err != nil {
		return fmt.Errorf("failed to mount sysfs for chroot environment: %w", err)
	}

	defer func() {
		if err != nil {
			if umountErr := chrootEnv.UmountChrootSysfs("/"); umountErr != nil {
				log.Errorf("Failed to unmount sysfs for chroot environment: %v", umountErr)
				err = fmt.Errorf("operation failed: %w, cleanup errors: %v", err, umountErr)
			}
		}
	}()

	if !chrootRootfsExist {
		// Create chroot repository
		if err = chrootEnv.createChrootRepo(targetOs, targetDist); err != nil {
			return fmt.Errorf("failed to create chroot repository: %w", err)
		}
	}

	if err = chrootEnv.initChrootLocalRepo(targetArch); err != nil {
		return fmt.Errorf("failed to initialize chroot local repository: %w", err)
	}

	return nil
}

func (chrootEnv *ChrootEnv) CleanupChrootEnv(targetOs, targetDist, targetArch string) error {
	log := logger.Logger()
	if _, err := os.Stat(chrootEnv.ChrootEnvRoot); err == nil {
		if err := system.StopGPGComponents(chrootEnv.ChrootEnvRoot); err != nil {
			return fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
		}
		if err := mount.UmountSubPath(chrootEnv.ChrootEnvRoot); err != nil {
			return fmt.Errorf("failed to unmount path for chroot environment: %w", err)
		}
	} else {
		log.Infof("Chroot environment root %s does not exist, skipping cleanup", chrootEnv.ChrootEnvRoot)
	}
	return nil
}

func (chrootEnv *ChrootEnv) TdnfInstallPackage(packageName, installRoot string, repositoryIDList []string) error {
	var installCmd string
	chrootInstallRoot, err := chrootEnv.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path for install root %s: %w", installRoot, err)
	}
	releaseVersion := chrootEnv.GetTargetOsReleaseVersion()
	installCmd = fmt.Sprintf("tdnf install %s --releasever %s --setopt reposdir=%s --nogpgcheck --assumeyes --installroot %s",
		packageName, releaseVersion, RPMRepoConfigDir, chrootInstallRoot)

	if len(repositoryIDList) > 0 {
		installCmd += " --disablerepo=*"
		for _, repoID := range repositoryIDList {
			installCmd += " --enablerepo=" + repoID
		}
	}

	if _, err := shell.ExecCmdWithStream(installCmd, true, chrootEnv.ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to install package %s: %w", packageName, err)
	}

	return nil
}

func CleanDebName(packageName string) string {
	packageName = strings.Replace(packageName, "_", "=", 1)
	if idx := strings.LastIndex(packageName, "_"); idx != -1 {
		archTag := packageName[idx+1:]
		switch archTag {
		case "amd64", "arm64", "all":
			packageName = packageName[:idx]
		}
	}
	return packageName
}

func (chrootEnv *ChrootEnv) AptInstallPackage(packageName, installRoot string, repoSrcList []string) error {
	packageName = CleanDebName(packageName)
	installCmd := fmt.Sprintf("apt-get install -y %s", packageName)

	if len(repoSrcList) > 0 {
		for _, repoSrc := range repoSrcList {
			installCmd += fmt.Sprintf(" -o Dir::Etc::sourcelist=%s", repoSrc)
		}
	}

	if _, err := shell.ExecCmdWithStream(installCmd, true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to install package %s: %w", packageName, err)
	}

	return nil
}
