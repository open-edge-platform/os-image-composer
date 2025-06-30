package chroot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/compression"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var (
	ChrootEnvRoot       string
	ChrootImageBuildDir string
)

func GetChrootEnvHostPath(chrootPath string) (string, error) {
	if ChrootEnvRoot == "" {
		return "", fmt.Errorf("chroot env may not initialized")
	}
	return filepath.Join(ChrootEnvRoot, chrootPath), nil
}

func GetChrootEnvPath(ChrootEnvHostPath string) (string, error) {
	if ChrootEnvRoot == "" {
		return "", fmt.Errorf("chroot env may not initialized")
	}
	isSubPath, err := file.IsSubPath(ChrootEnvRoot, ChrootEnvHostPath)
	if err != nil {
		return "", fmt.Errorf("failed to check if path %s is a subpath of chroot env root %s: %w",
			ChrootEnvHostPath, ChrootEnvRoot, err)
	}
	if !isSubPath {
		return "", fmt.Errorf("path %s is not a subpath of chroot env root %s", ChrootEnvHostPath, ChrootEnvRoot)
	}

	chrootPath := ChrootEnvHostPath[len(ChrootEnvRoot):]
	if strings.HasPrefix(chrootPath, "/") {
		return chrootPath, nil
	} else {
		return filepath.Join("/", chrootPath), nil
	}
}

func MountChrootSysfs(chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	return mount.MountSysfs(chrootHostPath)
}

func UmountChrootSysfs(chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}

	if err := StopGPGComponents(chrootHostPath); err != nil {
		return fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
	}

	if err = mount.UmountSysfs(chrootHostPath); err != nil {
		return fmt.Errorf("failed to unmount sysfs for %s: %w", chrootHostPath, err)
	}
	return nil
}

// MountChrootPath mounts a host path to a chroot path
func MountChrootPath(hostFullPath, chrootPath, mountFlags string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
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
func UmountChrootPath(chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}

	return mount.UmountPath(chrootHostPath)
}

// CopyFileFromHostToChroot copies a file from the host to the chroot environment
func CopyFileFromHostToChroot(hostFilePath, chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	cmd := "cp -f " + hostFilePath + " " + chrootHostPath
	if _, err = shell.ExecCmd(cmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to copy file from host to chroot: %w", err)
	}
	return nil
}

// CopyFileFromChrootToHost copies a file from the chroot environment to the host
func CopyFileFromChrootToHost(hostFilePath, chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	cmd := "cp -f " + chrootHostPath + " " + hostFilePath
	if _, err = shell.ExecCmd(cmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to copy file from chroot to host: %w", err)
	}
	return nil
}

func updateChrootLocalRPMRepo(chrootRepoDir string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootRepoDir)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootRepoDir, err)
	}
	if _, err := os.Stat(chrootHostPath); os.IsNotExist(err) {
		return fmt.Errorf("chroot repo directory not existing%s: %w", chrootHostPath, err)
	}
	cmd := fmt.Sprintf("createrepo_c --compatibility --update %s", chrootRepoDir)
	if _, err = shell.ExecCmd(cmd, true, ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to update chroot local cache repository: %w", err)
	}
	cmd = "tdnf makecache --releasever 3.0"
	if _, err = shell.ExecCmdWithStream(cmd, true, ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
	}
	return nil
}

func initChrootLocalRPMRepo() error {
	globalCacheDir, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache directory: %v", err)
	}

	pkgCacheDir := filepath.Join(globalCacheDir, "pkgCache", config.ProviderId)
	if _, err := os.Stat(pkgCacheDir); os.IsNotExist(err) {
		return fmt.Errorf("package cache directory does not exist: %s", pkgCacheDir)
	}

	// From local.repo
	chrootRepoDir := filepath.Join("/workspace", "cache-repo")

	if err := MountChrootPath(pkgCacheDir, chrootRepoDir, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory %s to chroot repo directory %s: %w",
			pkgCacheDir, chrootRepoDir, err)
	}
	if err := updateChrootLocalRPMRepo(chrootRepoDir); err != nil {
		return fmt.Errorf("failed to update chroot local cache repository %s: %w", chrootRepoDir, err)
	}
	return nil
}

func createChrootRPMRepo(targetOs, targetDist string) error {
	targetOsConfigDir, err := file.GetTargetOsConfigDir(targetOs, targetDist)
	if err != nil {
		return fmt.Errorf("failed to get target OS config directory: %v", err)
	}
	chrootRepoCongfigPath := filepath.Join(targetOsConfigDir, "chrootenvconfigs", "local.repo")
	if _, err := os.Stat(chrootRepoCongfigPath); os.IsNotExist(err) {
		return fmt.Errorf("chroot repo config file does not exist: %s", chrootRepoCongfigPath)
	}

	err = CopyFileFromHostToChroot(chrootRepoCongfigPath, "/etc/yum.repos.d/")
	if err != nil {
		return fmt.Errorf("failed to copy local.repo: %w", err)
	}

	if err := initChrootLocalRPMRepo(); err != nil {
		return fmt.Errorf("failed to initialize chroot local RPM repository: %w", err)
	}
	return nil
}

func initChrootWorkspace() error {
	chrootWorkspace := filepath.Join(ChrootEnvRoot, "workspace")
	ChrootImageBuildDir = filepath.Join(chrootWorkspace, "imagebuild")
	if _, err := os.Stat(ChrootImageBuildDir); os.IsNotExist(err) {
		if _, err = shell.ExecCmd("mkdir -p "+ChrootImageBuildDir, true, "", nil); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", ChrootImageBuildDir, err)
		}
	}
	return nil
}

func InitChrootEnv(targetOs, targetDist, targetArch string) error {
	var chrootEnvTarPath string

	// Init ChrootEnvRoot path
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %v", err)
	}
	ChrootEnvRoot = filepath.Join(globalWorkDir, config.ProviderId, "chrootenv")
	if _, err := os.Stat(ChrootEnvRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(ChrootEnvRoot, 0755); err != nil {
			return fmt.Errorf("failed to create chroot environment root directory: %w", err)
		}
	}

	var chrootRootfsExist bool = true
	if files, _ := os.ReadDir(ChrootEnvRoot); len(files) == 0 {
		chrootRootfsExist = false
		// Init ChrootEnvRoot rootfs
		if err := InitChrootBuildSpace(targetOs, targetDist, targetArch); err != nil {
			return fmt.Errorf("failed to init chroot build space")
		}

		chrootEnvTarPath = filepath.Join(ChrootBuildDir, "chrootenv.tar.gz")
		if _, err := os.Stat(chrootEnvTarPath); os.IsNotExist(err) {
			// Build chroot environment tarball
			if err = BuildChrootEnv(targetOs, targetDist, targetArch); err != nil {
				return fmt.Errorf("failed to build chroot environment: %w", err)
			}
		}

		// Extract chroot environment tarball
		if err = compression.DecompressFile(chrootEnvTarPath, ChrootEnvRoot, "tar.gz", true); err != nil {
			return fmt.Errorf("failed to extract chroot environment tarball: %w", err)
		}

		// Copy resolv.conf to the chroot environment
		if err = CopyFileFromHostToChroot("/etc/resolv.conf", "/etc/"); err != nil {
			return fmt.Errorf("failed to copy resolv.conf: %w", err)
		}
	}

	// Initialize the chroot workspace
	if err = initChrootWorkspace(); err != nil {
		return fmt.Errorf("failed to initialize chroot workspace: %w", err)
	}

	// Mount sysfs to the chroot environment
	err = MountChrootSysfs("/")
	if err != nil {
		return fmt.Errorf("failed to mount sysfs for chroot environment: %w", err)
	}

	if !chrootRootfsExist {
		// Create chroot RPM repository
		if err = createChrootRPMRepo(targetOs, targetDist); err != nil {
			err = fmt.Errorf("failed to create chroot RPM repository: %w", err)
			goto fail
		}
	} else {
		// If the chroot environment already exists, update the local RPM repository
		if err = initChrootLocalRPMRepo(); err != nil {
			err = fmt.Errorf("failed to initialize chroot local RPM repository: %w", err)
			goto fail
		}
	}

	return nil

fail:
	if err := UmountChrootSysfs("/"); err != nil {
		return fmt.Errorf("failed to unmount sysfs for chroot environment: %w", err)
	}
	return fmt.Errorf("failed to initialize chroot environment: %w", err)
}

func StopGPGComponents(chrootPath string) error {
	log := logger.Logger()
	cmdExist, err := shell.IsCommandExist("gpgconf", chrootPath)
	if err != nil {
		return fmt.Errorf("failed to check if gpgconf command exists in chroot environment: %w", err)
	}
	if !cmdExist {
		log.Debugf("gpgconf command not found in chroot environment, skipping GPG components stop")
		return nil
	}
	output, err := shell.ExecCmd("gpgconf --list-components", true, chrootPath, nil)
	if err != nil {
		return fmt.Errorf("failed to list GPG components in chroot environment: %w", err)
	}
	for _, line := range strings.Split(output, "\n") {
		component := strings.TrimSpace(strings.Split(line, ":")[0])
		if component == "gpg-agent" || component == "keyboxd" {
			log.Debugf("Stopping GPG component: %s", component)
			if _, err := shell.ExecCmd("gpgconf --kill "+component, true, chrootPath, nil); err != nil {
				return fmt.Errorf("failed to stop GPG component %s: %w", component, err)
			}
		}
	}

	return nil
}

func CleanupChrootEnv(targetOs, targetDist, targetArch string) error {
	log := logger.Logger()
	if _, err := os.Stat(ChrootEnvRoot); err == nil {
		if err := StopGPGComponents(ChrootEnvRoot); err != nil {
			return fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
		}
		if err := mount.UmountSubPath(ChrootEnvRoot); err != nil {
			return fmt.Errorf("failed to unmount path for chroot environment: %w", err)
		}
	} else {
		log.Infof("chroot environment root %s does not exist, skipping cleanup", ChrootEnvRoot)
	}
	return nil
}

func TdnfInstallPackage(packageName, installRoot string, repositoryIDList []string) error {
	var installCmd string
	chrootInstallRoot, err := GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path for install root %s: %w", installRoot, err)
	}
	installCmd = fmt.Sprintf("tdnf install %s --releasever 3.0 --setopt reposdir=/etc/yum.repos.d/ --nogpgcheck --assumeyes --installroot %s",
		packageName, chrootInstallRoot)

	if len(repositoryIDList) > 0 {
		installCmd += " --disablerepo=*"
		for _, repoID := range repositoryIDList {
			installCmd += " --enablerepo=" + repoID
		}
	}

	if _, err := shell.ExecCmdWithStream(installCmd, true, ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to install package %s: %w", packageName, err)
	}

	return nil
}
