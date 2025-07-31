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
	return file.CopyFile(hostFilePath, chrootHostPath, "-f", true)
}

// CopyFileFromChrootToHost copies a file from the chroot environment to the host
func CopyFileFromChrootToHost(hostFilePath, chrootPath string) error {
	chrootHostPath, err := GetChrootEnvHostPath(chrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot host path for %s: %w", chrootPath, err)
	}
	return file.CopyFile(chrootHostPath, hostFilePath, "-f", true)
}

func UpdateChrootLocalRPMRepo(chrootRepoDir string) error {
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
	return nil
}

func UpdateLocalDebRepo(repoPath string) error {
	metaDataPath := filepath.Join(repoPath, "dists/stable/main/binary-amd64", "Packages.gz")
	if _, err := os.Stat(metaDataPath); err == nil {
		if _, err = shell.ExecCmd("rm -f "+metaDataPath, true, "", nil); err != nil {
			return fmt.Errorf("failed to remove existing Packages.gz: %w", err)
		}
	}
	metaDataDir := filepath.Dir(metaDataPath)
	if _, err := os.Stat(metaDataDir); os.IsNotExist(err) {
		if _, err = shell.ExecCmd("mkdir -p "+metaDataDir, true, "", nil); err != nil {
			return fmt.Errorf("failed to create metadata directory %s: %w", metaDataDir, err)
		}
	}

	cmd := fmt.Sprintf("cd %s && sudo dpkg-scanpackages . /dev/null | gzip -9c > %s", repoPath, metaDataPath)
	if _, err := shell.ExecCmd(cmd, false, "", nil); err != nil {
		return fmt.Errorf("failed to create local debian cache repository: %w", err)
	}

	return nil
}

func RefreshLocalCacheRepo() error {
	// From local.repo
	chrootRepoDir := "/cdrom/cache-repo"
	pkgType := GetTaRgetOsPkgType(config.TargetOs)
	if pkgType == "rpm" {
		if err := UpdateChrootLocalRPMRepo(chrootRepoDir); err != nil {
			return fmt.Errorf("failed to update rpm local cache repository %s: %w", chrootRepoDir, err)
		}

		cmd := "tdnf makecache --releasever 3.0"
		if _, err := shell.ExecCmdWithStream(cmd, true, ChrootEnvRoot, nil); err != nil {
			return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
		}
	} else if pkgType == "deb" {
		if err := UpdateLocalDebRepo(ChrootPkgCacheDir); err != nil {
			return fmt.Errorf("failed to update debian local cache repository: %v", err)
		}

		cmd := "apt-get update"
		if _, err := shell.ExecCmdWithStream(cmd, true, ChrootEnvRoot, nil); err != nil {
			return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}
	return nil
}

func initChrootLocalRepo() error {
	// From local.repo
	chrootRepoDir := "/cdrom/cache-repo"

	if err := MountChrootPath(ChrootPkgCacheDir, chrootRepoDir, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory %s to chroot repo directory %s: %w",
			ChrootPkgCacheDir, chrootRepoDir, err)
	}

	if err := RefreshLocalCacheRepo(); err != nil {
		return fmt.Errorf("failed to refresh local cache repository: %w", err)
	}
	return nil
}

func createChrootRepo(targetOs, targetDist string) error {
	var chrootRepoCongfigPath string
	pkgType := GetTaRgetOsPkgType(targetOs)
	chrootConfigDir, err := GetChrootConfigDir(targetOs, targetDist)
	if err != nil {
		return fmt.Errorf("failed to get chroot config directory: %v", err)
	}

	if pkgType == "rpm" {
		chrootRepoCongfigPath = filepath.Join(chrootConfigDir, "local.repo")
		if _, err := os.Stat(chrootRepoCongfigPath); os.IsNotExist(err) {
			return fmt.Errorf("chroot repo config file does not exist: %s", chrootRepoCongfigPath)
		}

		err = CopyFileFromHostToChroot(chrootRepoCongfigPath, "/etc/yum.repos.d/")
		if err != nil {
			return fmt.Errorf("failed to copy local.repo: %w", err)
		}
	} else if pkgType == "deb" {
		chrootRepoCongfigPath, err = GetChrootEnvHostPath("/etc/apt/sources.list.d/*")
		if err != nil {
			return fmt.Errorf("failed to get chroot host path for local repo config: %w", err)
		}
		if _, err := shell.ExecCmd("rm -f "+chrootRepoCongfigPath, true, "", nil); err != nil {
			return fmt.Errorf("failed to remove existing local repo config files: %w", err)
		}

		RepoCongfigPath := filepath.Join(chrootConfigDir, "local.list")
		if _, err := os.Stat(RepoCongfigPath); os.IsNotExist(err) {
			return fmt.Errorf("chroot repo config file does not exist: %s", RepoCongfigPath)
		}

		err = CopyFileFromHostToChroot(RepoCongfigPath, "/etc/apt/sources.list.d/")
		if err != nil {
			return fmt.Errorf("failed to copy local.repo: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
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
	// Init ChrootEnvRoot rootfs
	if err := InitChrootBuildSpace(targetOs, targetDist, targetArch); err != nil {
		return fmt.Errorf("failed to initialize chroot build space: %v", err)
	}

	var chrootRootfsExist bool = true
	if files, _ := os.ReadDir(ChrootEnvRoot); len(files) == 0 {
		chrootRootfsExist = false

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
		// Create chroot repository
		if err = createChrootRepo(targetOs, targetDist); err != nil {
			err = fmt.Errorf("failed to create chroot repository: %w", err)
			goto fail
		}
	}

	if err = initChrootLocalRepo(); err != nil {
		err = fmt.Errorf("failed to initialize chroot local repository: %w", err)
		goto fail
	}

	return nil

fail:
	if err := UmountChrootSysfs("/"); err != nil {
		return fmt.Errorf("failed to unmount sysfs for chroot environment: %w", err)
	}
	return fmt.Errorf("failed to initialize chroot environment: %w", err)
}

func CheckOpenFile(chrootPath string) error {
	log := logger.Logger()
	output, err := shell.ExecCmdSilent("lsof +D "+chrootPath, true, "", nil)
	if err != nil {
		if strings.Contains(output, "WARNING: can't stat()") {
			log.Debugf("Harmless WARNING: The error just means not all filesystems could be checked.")
			log.Debugf("Harmless WARNING: But the result is valid.")
		} else {
			return fmt.Errorf("failed to check open files in chroot environment: %w", err)
		}
	}
	if output != "" {
		for _, line := range strings.Split(output, "\n") {
			log.Debugf("%s", line)
		}
	}
	return nil
}

func CheckUsedMountPoint(chrootPath string) error {
	log := logger.Logger()
	output, err := shell.ExecCmdSilent("fuser -vm "+chrootPath, true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to check used mount points in chroot environment: %w", err)
	}
	if output != "" {
		for _, line := range strings.Split(output, "\n") {
			log.Debugf("%s", line)
		}
	}
	return nil
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
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue // Skip empty lines or lines without a colon
		}
		component := strings.TrimSpace(strings.Split(line, ":")[0])
		log.Debugf("Stopping GPG component: %s", component)
		if _, err := shell.ExecCmd("gpgconf --kill "+component, true, chrootPath, nil); err != nil {
			return fmt.Errorf("failed to stop GPG component %s: %w", component, err)
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

func AptInstallPackage(packageName, installRoot string, repoSrcList []string) error {
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
