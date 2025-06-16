package chroot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
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
	ChrootBuildDir = filepath.Join(globalWorkDir, config.ProviderId, "chrootbuild")
	ChrootPkgCacheDir = filepath.Join(ChrootBuildDir, "packages")
	return nil
}

func getChrootEnvConfig(chrootEnvCongfigPath string) (map[interface{}]interface{}, error) {
	if _, err := os.Stat(chrootEnvCongfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chroot environment config file does not exist: %s", chrootEnvCongfigPath)
	}
	return file.ReadFromYaml(chrootEnvCongfigPath)
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

func downloadChrootEnvPackages(targetOs string, targetDist string, targetArch string) ([]string, error) {
	var allPkgsList []string
	if targetOs == "azure-linux" {
		targetOsConfigDir, err := file.GetTargetOsConfigDir(targetOs, targetDist)
		if err != nil {
			return allPkgsList, fmt.Errorf("failed to get target OS config directory: %v", err)
		}
		chrootEnvCongfigPath := filepath.Join(targetOsConfigDir, "chrootenvconfigs", "chrootenv_"+targetArch+".yml")
		chrootEnvPackageList, err := getChrootEnvPackageList(chrootEnvCongfigPath)
		if err != nil {
			return allPkgsList, fmt.Errorf("failed to get chroot environment package list: %v", err)
		}

		if _, err := os.Stat(ChrootPkgCacheDir); os.IsNotExist(err) {
			if err := os.MkdirAll(ChrootPkgCacheDir, 0755); err != nil {
				return allPkgsList, fmt.Errorf("failed to create chroot package cache directory: %v", err)
			}
		}

		dotFilePath := filepath.Join(ChrootPkgCacheDir, "chrootpkgs.dot")
		allPkgsList, err = rpmutils.DownloadPackages(chrootEnvPackageList, ChrootPkgCacheDir, dotFilePath)
		if err != nil {
			return allPkgsList, fmt.Errorf("failed to download chroot environment packages: %v", err)
		}
		return allPkgsList, nil
	} else {
		return allPkgsList, fmt.Errorf("unsupported OS: %s", targetOs)
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

func BuildChrootEnv(targetOs string, targetDist string, targetArch string) error {
	installComplete := true
	log := logger.Logger()
	err := InitChrootBuildSpace(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot build space: %v", err)
	}
	chrootTarPath := filepath.Join(ChrootBuildDir, "chrootenv.tar.gz")
	chrootEnvPath := filepath.Join(ChrootBuildDir, "chroot")
	chrootRpmDbPath := filepath.Join(chrootEnvPath, "var", "lib", "rpm")
	if _, err := os.Stat(chrootTarPath); err == nil {
		log.Infof("Chroot tarball already exists at %s", chrootTarPath)
		return nil
	}

	if _, err := os.Stat(chrootRpmDbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(chrootRpmDbPath, 0755); err != nil {
			return fmt.Errorf("failed to create chroot environment directory: %v", err)
		}
	}

	allPkgsList, err := downloadChrootEnvPackages(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to download chroot environment packages: %v", err)
	}
	log.Infof("Downloaded %d packages for chroot environment", len(allPkgsList))

	err = mount.MountSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to mount system directories in chroot environment: %v", err)
	}

	for _, pkg := range allPkgsList {
		pkgPath := filepath.Join(ChrootPkgCacheDir, pkg)
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			log.Errorf("package %s does not exist in cache directory", pkg)
			break
		}
		log.Infof("Installing package %s in chroot environment", pkg)
		cmdStr := fmt.Sprintf("rpm -i -v --nodeps --noorder --force --root %s --define '_dbpath /var/lib/rpm' %s",
			chrootEnvPath, pkgPath)
		output, err := shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			installComplete = false
			log.Errorf("Failed to install package %s: %v, output: %s", pkg, err, output)
			break
		}
	}

	if installComplete {
		err = updateRpmDB(chrootEnvPath, allPkgsList)
		if err != nil {
			log.Errorf("failed to update RPM database in chroot environment: %v", err)
		}
		err = importGpgKeys(targetOs, chrootEnvPath)
		if err != nil {
			log.Errorf("failed to import GPG keys in chroot environment: %v", err)
		}
	}

	err = mount.UmountSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to unmount system directories in chroot environment: %v", err)
	}
	err = mount.CleanSysfs(chrootEnvPath)
	if err != nil {
		return fmt.Errorf("failed to clean system directories in chroot environment: %v", err)
	}

	if installComplete {
		if err = compression.CompressFolder(chrootEnvPath, chrootTarPath, "tar.gz", true); err != nil {
			return fmt.Errorf("failed to compress chroot environment: %v", err)
		}
		log.Infof("Chroot environment build completed successfully")
		_, err = shell.ExecCmd("rm -rf "+chrootEnvPath, true, "", nil)
		return err
	} else {
		log.Errorf("Chroot environment build failed, some packages may not have been installed correctly")
		return fmt.Errorf("chroot environment build failed, some packages may not have been installed correctly")
	}
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
