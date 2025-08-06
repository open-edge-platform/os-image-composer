package chroot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/compression"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var (
	ChrootBuildDir    string // ChrootBuildDir is the directory where the chroot build.
	ChrootPkgCacheDir string // ChrootPkgCacheDir is the directory where chroot environment packages are cached.
)

func InitChrootBuildSpace(targetOs string, targetDist string, targetArch string) error {
	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %v", err)
	}
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	ChrootBuildDir = filepath.Join(globalWorkDir, config.ProviderId, "chrootbuild")
	ChrootPkgCacheDir = filepath.Join(globalCache, "pkgCache", config.ProviderId)

	return nil
}

func getChrootEnvConfig(chrootEnvCongfigPath string) (map[interface{}]interface{}, error) {
	if _, err := os.Stat(chrootEnvCongfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chroot environment config file does not exist: %s", chrootEnvCongfigPath)
	}
	return file.ReadFromYaml(chrootEnvCongfigPath)
}

func GetChrootEnvEssentialPackageList(chrootEnvCongfigPath string) ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := getChrootEnvConfig(chrootEnvCongfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chroot environment config: %v", err)
	}
	if pkgListRaw, ok := chrootEnvConfig["essential"]; ok {
		if pkgListStr, ok := pkgListRaw.([]interface{}); ok {
			for _, pkg := range pkgListStr {
				if pkgStr, ok := pkg.(string); ok {
					pkgList = append(pkgList, pkgStr)
				} else {
					return nil, fmt.Errorf("invalid package format in chroot environment config: %v", pkg)
				}
			}
		} else {
			return nil, fmt.Errorf("essential packages field is not a list in chroot environment config")
		}
	}
	return pkgList, nil
}

func getChrootEnvPackageList(chrootEnvCongfigPath string) ([]string, error) {
	pkgList := []string{}
	chrootEnvConfig, err := getChrootEnvConfig(chrootEnvCongfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chroot environment config: %v", err)
	}
	if pkgListRaw, ok := chrootEnvConfig["packages"]; ok {
		if pkgListStr, ok := pkgListRaw.([]interface{}); ok {
			for _, pkg := range pkgListStr {
				if pkgStr, ok := pkg.(string); ok {
					pkgList = append(pkgList, pkgStr)
				} else {
					return nil, fmt.Errorf("invalid package format in chroot environment config: %v", pkg)
				}
			}
		} else {
			return nil, fmt.Errorf("packages field is not a list in chroot environment config")
		}
	} else {
		return nil, fmt.Errorf("packages field not found in chroot environment config")
	}
	return pkgList, nil
}

func getHostOsInfo() (map[string]string, error) {
	log := logger.Logger()
	var hostOsInfo = map[string]string{
		"name":    "",
		"version": "",
		"arch":    "",
	}

	// Get architecture using uname command
	output, err := shell.ExecCmd("uname -m", false, "", nil)
	if err != nil {
		return hostOsInfo, fmt.Errorf("failed to get host architecture: %v", err)
	} else {
		hostOsInfo["arch"] = strings.TrimSpace(output)
	}

	// Read from /etc/os-release if it exists
	if _, err := os.Stat("/etc/os-release"); err == nil {
		file, err := os.Open("/etc/os-release")
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "NAME=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["name"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				} else if strings.HasPrefix(line, "VERSION_ID=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						hostOsInfo["version"] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
					}
				}
			}

			log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
				hostOsInfo["version"] + " " + hostOsInfo["arch"])

			return hostOsInfo, nil
		}
	}

	output, err = shell.ExecCmd("lsb_release -si", false, "", nil)
	if err != nil {
		return hostOsInfo, fmt.Errorf("failed to get host OS name: %v", err)
	} else {
		if output != "" {
			hostOsInfo["name"] = strings.TrimSpace(output)
			output, err = shell.ExecCmd("lsb_release -sr", false, "", nil)
			if err != nil {
				return hostOsInfo, fmt.Errorf("failed to get host OS version: %v", err)
			} else {
				if output != "" {
					hostOsInfo["version"] = strings.TrimSpace(output)
					log.Infof("Detected OS info: " + hostOsInfo["name"] + " " +
						hostOsInfo["version"] + " " + hostOsInfo["arch"])
					return hostOsInfo, nil
				}
			}
		}
	}

	log.Errorf("Failed to detect host OS info!")
	return hostOsInfo, fmt.Errorf("failed to detect host OS info")
}

func GetHostOsPkgManager() (string, error) {
	hostOsInfo, err := getHostOsInfo()
	if err != nil {
		return "", err
	}

	switch hostOsInfo["name"] {
	case "Ubuntu", "Debian", "eLxr":
		return "apt", nil
	case "Fedora", "CentOS", "Red Hat Enterprise Linux":
		return "yum", nil
	case "Microsoft Azure Linux", "Edge Microvisor Toolkit":
		return "tdnf", nil
	default:
		return "", fmt.Errorf("unsupported host OS: %s", hostOsInfo["name"])
	}
}

func GetTaRgetOsPkgType(targetOs string) string {
	switch targetOs {
	case "azure-linux":
		return "rpm"
	case "edge-microvisor-toolkit":
		return "rpm"
	case "wind-river-elxr":
		return "deb"
	default:
		return ""
	}
}

func GetChrootConfigDir(targetOs, targetDist string) (string, error) {
	targetOsConfigDir, err := file.GetTargetOsConfigDir(targetOs, targetDist)
	if err != nil {
		return "", fmt.Errorf("failed to get target OS config directory: %v", err)
	}
	chrootConfigDir := filepath.Join(targetOsConfigDir, "chrootenvconfigs")
	if _, err := os.Stat(chrootConfigDir); os.IsNotExist(err) {
		return "", fmt.Errorf("chroot config path does not exist: %s", chrootConfigDir)
	}
	return chrootConfigDir, nil
}

func downloadChrootEnvPackages(targetOs string, targetDist string, targetArch string) ([]string, []string, error) {
	var pkgsList []string
	var allPkgsList []string

	pkgType := GetTaRgetOsPkgType(targetOs)
	chrootConfigDir, err := GetChrootConfigDir(targetOs, targetDist)
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get chroot config directory: %v", err)
	}
	chrootEnvCongfigPath := filepath.Join(chrootConfigDir, "chrootenv_"+targetArch+".yml")
	essentialPkgsList, err := GetChrootEnvEssentialPackageList(chrootEnvCongfigPath)
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get essential packages list: %v", err)
	}
	pkgsList, err = getChrootEnvPackageList(chrootEnvCongfigPath)
	if err != nil {
		return pkgsList, allPkgsList, fmt.Errorf("failed to get chroot environment package list: %v", err)
	}
	pkgsList = append(essentialPkgsList, pkgsList...)

	if _, err := os.Stat(ChrootPkgCacheDir); os.IsNotExist(err) {
		if err := os.MkdirAll(ChrootPkgCacheDir, 0755); err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to create chroot package cache directory: %v", err)
		}
	}

	dotFilePath := filepath.Join(ChrootPkgCacheDir, "chrootpkgs.dot")

	if pkgType == "rpm" {
		allPkgsList, err = rpmutils.DownloadPackages(pkgsList, ChrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %v", err)
		}
		return pkgsList, allPkgsList, nil
	} else if pkgType == "deb" {
		allPkgsList, err = debutils.DownloadPackages(pkgsList, ChrootPkgCacheDir, dotFilePath)
		if err != nil {
			return pkgsList, allPkgsList, fmt.Errorf("failed to download chroot environment packages: %v", err)
		}
		return pkgsList, allPkgsList, nil
	} else {
		return pkgsList, allPkgsList, fmt.Errorf("unsupported OS: %s", targetOs)
	}
}

// updateRpmDB updates the RPM database in the chroot environment
func updateRpmDB(chrootEnvBuildPath string, rpmList []string) error {
	log := logger.Logger()
	cmdStr := "rpm -E '%{_db_backend}'"
	hostRpmDbBackend, err := shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to get host RPM DB backend: %v", err)
	}
	hostRpmDbBackend = strings.TrimSpace(hostRpmDbBackend)
	chrootRpmDbBackend, err := shell.ExecCmd(cmdStr, true, chrootEnvBuildPath, nil)
	if err != nil {
		return fmt.Errorf("failed to get chroot RPM DB backend: %v", err)
	}
	chrootRpmDbBackend = strings.TrimSpace(chrootRpmDbBackend)
	if hostRpmDbBackend == chrootRpmDbBackend {
		log.Debugf("The host RPM DB: " + hostRpmDbBackend + " matches the chroot RPM DB: " + chrootRpmDbBackend)
		log.Debugf("Not rebuilding the chroot RPM database.")
		return nil
	}

	log.Debugf("The host RPM DB: " + hostRpmDbBackend + " differs from the chroot RPM DB: " + chrootRpmDbBackend)
	log.Debugf("Rebuilding the chroot RPM database.")
	if _, err = shell.ExecCmd("rm -rf /var/lib/rpm/*", true, chrootEnvBuildPath, nil); err != nil {
		return fmt.Errorf("failed to remove RPM database: %v", err)
	}
	if _, err = shell.ExecCmd("rpm --initdb", true, chrootEnvBuildPath, nil); err != nil {
		return fmt.Errorf("failed to initialize RPM database: %v", err)
	}

	chrootPkgDir := filepath.Join(chrootEnvBuildPath, "packages")
	if err = mount.MountPath(ChrootPkgCacheDir, chrootPkgDir, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory: %v", err)
	}

	for _, rpm := range rpmList {
		rpmChrootPath := filepath.Join("/packages", rpm)
		cmdStr := "rpm -i -v --nodeps --noorder --force --justdb " + rpmChrootPath
		if _, err := shell.ExecCmdWithStream(cmdStr, true, chrootEnvBuildPath, nil); err != nil {
			return fmt.Errorf("failed to update RPM Database for %s in chroot environment: %v", rpm, err)
		}
	}

	return mount.UmountAndDeletePath(chrootPkgDir)
}

// importGpgKeys imports GPG keys into the chroot environment
func importGpgKeys(targetOs string, chrootEnvBuildPath string) error {
	var cmdStr string
	log := logger.Logger()
	if targetOs == "edge-microvisor-toolkit" {
		cmdStr = "rpm -q -l edge-repos-shared | grep 'rpm-gpg'"
	} else if targetOs == "azure-linux" {
		cmdStr = "rpm -q -l azurelinux-repos-shared | grep 'rpm-gpg'"
	}

	output, err := shell.ExecCmd(cmdStr, true, chrootEnvBuildPath, nil)
	if err != nil {
		return fmt.Errorf("failed to get GPG keys: %v", err)
	}
	if output != "" {
		gpgKeys := strings.Split(output, "\n")
		log.Infof("Importing GPG key: " + gpgKeys[0])
		cmdStr = "rpm --import " + gpgKeys[0]
		_, err = shell.ExecCmd(cmdStr, true, chrootEnvBuildPath, nil)
		if err != nil {
			return fmt.Errorf("failed to import GPG key: %v", err)
		}
	} else {
		return fmt.Errorf("no GPG keys found in the chroot environment")
	}
	return nil
}

func installRpmPkg(targetOs, chrootEnvPath string, allPkgsList []string) error {
	log := logger.Logger()
	chrootRpmDbPath := filepath.Join(chrootEnvPath, "var", "lib", "rpm")
	if _, err := os.Stat(chrootRpmDbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(chrootRpmDbPath, 0755); err != nil {
			return fmt.Errorf("failed to create chroot environment directory: %v", err)
		}
	}

	err := mount.MountSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to mount system directories in chroot environment: %v", err)
	}

	for _, pkg := range allPkgsList {
		pkgPath := filepath.Join(ChrootPkgCacheDir, pkg)
		if _, err = os.Stat(pkgPath); os.IsNotExist(err) {
			err = fmt.Errorf("package %s does not exist in cache directory: %v", pkg, err)
			goto fail
		}
		log.Infof("Installing package %s in chroot environment", pkg)
		cmdStr := fmt.Sprintf("rpm -i -v --nodeps --noorder --force --root %s --define '_dbpath /var/lib/rpm' %s",
			chrootEnvPath, pkgPath)
		var output string
		output, err = shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			err = fmt.Errorf("failed to install package %s: %v, output: %s", pkg, err, output)
			goto fail
		}
	}

	err = updateRpmDB(chrootEnvPath, allPkgsList)
	if err != nil {
		err = fmt.Errorf("failed to update RPM database in chroot environment: %v", err)
		goto fail
	}
	err = importGpgKeys(targetOs, chrootEnvPath)
	if err != nil {
		err = fmt.Errorf("failed to import GPG keys in chroot environment: %v", err)
		goto fail
	}

	err = StopGPGComponents(chrootEnvPath)
	if err != nil {
		err = fmt.Errorf("failed to stop GPG components in chroot environment: %w", err)
		goto fail
	}

	err = mount.UmountSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to unmount system directories in chroot environment: %v", err)
	}
	err = mount.CleanSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to clean system directories in chroot environment: %v", err)
	}

	return nil

fail:
	if err := mount.UmountSysfs(chrootEnvPath); err != nil {
		log.Errorf("failed to unmount system directories in chroot environment: %v", err)
	} else {
		log.Infof("Unmounted system directories in chroot environment: %s", chrootEnvPath)
	}
	if err := mount.CleanSysfs(chrootEnvPath); err != nil {
		log.Errorf("failed to clean system directories in chroot environment: %v", err)
	} else {
		log.Infof("Cleaned system directories in chroot environment: %s", chrootEnvPath)
	}
	if _, err := shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		log.Errorf("failed to remove chroot environment build path: %v", err)
	} else {
		log.Infof("Removed chroot environment build path: %s", chrootEnvPath)
	}
	return err
}

func mountDebLocalRepo(mountPoint string) error {
	return mount.MountPath(ChrootPkgCacheDir, mountPoint, "--bind")
}

func umountDebLocalRepo(mountPoint string) error {
	if err := mount.UmountPath(mountPoint); err != nil {
		return fmt.Errorf("failed to unmount debian local repository: %w", err)
	}
	return nil
}

func installDebPkg(targetOs, targetDist, chrootEnvPath string, pkgsList []string) error {
	var err error
	var cmd string

	// from local.list
	repoPath := "/cdrom/cache-repo"
	pkgListStr := strings.Join(pkgsList, ",")

	chrootConfigDir, err := GetChrootConfigDir(targetOs, targetDist)
	if err != nil {
		return fmt.Errorf("failed to get chroot config directory: %v", err)
	}

	localRepoConfigPath := filepath.Join(chrootConfigDir, "local.list")
	if _, err := os.Stat(localRepoConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("local repository config file does not exist: %s", localRepoConfigPath)
	}

	if err := mountDebLocalRepo(repoPath); err != nil {
		return fmt.Errorf("failed to mount debian local repository: %v", err)
	}

	if _, err := os.Stat(chrootEnvPath); os.IsNotExist(err) {
		if err := os.MkdirAll(chrootEnvPath, 0755); err != nil {
			return fmt.Errorf("failed to create chroot environment directory: %v", err)
		}
	}

	cmd = fmt.Sprintf("mmdebstrap "+
		"--variant=custom "+
		"--format=directory "+
		"--aptopt=APT::Authentication::Trusted=true "+
		"--hook-dir=/usr/share/mmdebstrap/hooks/file-mirror-automount "+
		"--include=%s "+
		"--verbose --debug "+
		"-- bookworm %s %s",
		pkgListStr, chrootEnvPath, localRepoConfigPath)

	if _, err = shell.ExecCmdWithStream(cmd, true, "", nil); err != nil {
		goto fail
	}

	if err := umountDebLocalRepo(repoPath); err != nil {
		return fmt.Errorf("failed to unmount debian local repository: %v", err)
	}

	return nil

fail:
	if err := umountDebLocalRepo(repoPath); err != nil {
		logger.Logger().Errorf("failed to unmount debian local repository: %v", err)
	}

	if _, err := shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove chroot environment build path: %v", err)
	}
	return fmt.Errorf("failed to install debian packages in chroot environment: %v", err)
}

func BuildChrootEnv(targetOs string, targetDist string, targetArch string) error {
	log := logger.Logger()
	pkgType := GetTaRgetOsPkgType(targetOs)
	err := InitChrootBuildSpace(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot build space: %v", err)
	}
	chrootTarPath := filepath.Join(ChrootBuildDir, "chrootenv.tar.gz")
	if _, err := os.Stat(chrootTarPath); err == nil {
		log.Infof("Chroot tarball already exists at %s", chrootTarPath)
		return nil
	}

	chrootEnvPath := filepath.Join(ChrootBuildDir, "chroot")

	pkgsList, allPkgsList, err := downloadChrootEnvPackages(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to download chroot environment packages: %v", err)
	}
	log.Infof("Downloaded %d packages for chroot environment", len(allPkgsList))

	if pkgType == "rpm" {
		if err := installRpmPkg(targetOs, chrootEnvPath, allPkgsList); err != nil {
			return fmt.Errorf("failed to install packages in chroot environment: %v", err)
		}
	} else if pkgType == "deb" {
		if err = UpdateLocalDebRepo(ChrootPkgCacheDir); err != nil {
			return fmt.Errorf("failed to create debian local repository: %v", err)
		}

		if err := installDebPkg(targetOs, targetDist, chrootEnvPath, pkgsList); err != nil {
			return fmt.Errorf("failed to install packages in chroot environment: %v", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}

	if err = compression.CompressFolder(chrootEnvPath, chrootTarPath, "tar.gz", true); err != nil {
		return fmt.Errorf("failed to compress chroot environment: %v", err)
	}

	log.Infof("Chroot environment build completed successfully")

	if _, err = shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove chroot environment build path: %v", err)
	}

	return nil
}

func CleanChrootBuild(targetOs string, targetDist string, targetArch string) error {
	log := logger.Logger()
	err := InitChrootBuildSpace(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot build space: %v", err)
	}

	files, err := os.ReadDir(ChrootBuildDir)
	if err != nil {
		return fmt.Errorf("failed to read chroot build path: %v", err)
	}

	for _, file := range files {
		if file.IsDir() && file.Name() == "chroot" {
			chrootEnvPath := filepath.Join(ChrootBuildDir, file.Name())
			err := mount.UmountSysfs(chrootEnvPath)
			if err != nil {
				return fmt.Errorf("failed to unmount sysfs path: %v", err)
			}
			err = mount.CleanSysfs(chrootEnvPath)
			if err != nil {
				return fmt.Errorf("failed to clean sysfs path: %v", err)
			}

			_, err = shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil)
			if err != nil {
				return fmt.Errorf("failed to remove chroot env build path: %v", err)
			} else {
				log.Infof("Removed chroot env build path: %s", chrootEnvPath)
			}
		} else if file.IsDir() && file.Name() == "packages" {
			chrootPkgCachePath := filepath.Join(ChrootBuildDir, file.Name())
			_, err = shell.ExecCmd("rm -rf "+chrootPkgCachePath, true, "", nil)
			if err != nil {
				return fmt.Errorf("failed to remove chroot package cache path: %v", err)
			} else {
				log.Infof("Removed chroot package cache path: %s", chrootPkgCachePath)
			}
		}
	}
	if _, err := os.Stat(ChrootBuildDir); !os.IsNotExist(err) {
		_, err = shell.ExecCmd("rm -rf "+ChrootBuildDir, true, "", nil)
		if err != nil {
			return fmt.Errorf("failed to remove chroot build directory: %v", err)
		} else {
			log.Infof("Removed chroot build directory: %s", ChrootBuildDir)
		}
	}

	return nil
}
