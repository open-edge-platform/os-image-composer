package imageos

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/image-composer/internal/image/imagesign"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func InstallInitrd(template *config.ImageTemplate) (string, string, error) {
	var versionInfo string
	log := logger.Logger()
	log.Infof("Installing initrd for image: %s", template.GetImageName())

	installRoot, err := InitChrootInstallRoot(template)
	if err != nil {
		return installRoot, versionInfo, fmt.Errorf("failed to initialize chroot install root: %w", err)
	}

	pkgType := chroot.GetTaRgetOsPkgType(config.TargetOs)
	if pkgType == "deb" {
		if err := initRootfsForDeb(installRoot); err != nil {
			return installRoot, versionInfo, fmt.Errorf("failed to initialize rootfs for deb: %w", err)
		}
	}

	if err := mountSysfsToRootfs(installRoot); err != nil {
		return installRoot, versionInfo, err
	}

	log.Infof("Image installation pre-processing...")
	err = preImageOsInstall(installRoot, template)
	if err != nil {
		err = fmt.Errorf("pre-install failed: %w", err)
		goto fail
	}

	log.Infof("Image package installation...")
	err = installImagePkgs(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to install image packages: %w", err)
		goto fail
	}

	log.Infof("Image system configuration...")
	err = updateInitrdConfig(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to update image config: %w", err)
		goto fail
	}

	log.Infof("Image installation post-processing...")
	versionInfo, err = postImageOsInstall(installRoot, template)
	if err != nil {
		err = fmt.Errorf("post-install failed: %w", err)
		goto fail
	}

	if err := umountSysfsFromRootfs(installRoot); err != nil {
		return installRoot, versionInfo, err
	}

	return installRoot, versionInfo, nil

fail:
	if umountErr := umountSysfsFromRootfs(installRoot); umountErr != nil {
		log.Errorf("Failed to unmount sysfs from rootfs after error: %v", umountErr)
	}
	return installRoot, versionInfo, fmt.Errorf("initrd installation failed: %w", err)
}

func InstallImageOs(diskPathIdMap map[string]string, template *config.ImageTemplate) (string, error) {
	var err error
	var versionInfo string
	var mountPointInfoList []map[string]string
	log := logger.Logger()
	log.Infof("Installing OS for image: %s", template.GetImageName())

	installRoot, err := InitChrootInstallRoot(template)
	if err != nil {
		return versionInfo, fmt.Errorf("failed to initialize chroot install root: %w", err)
	}

	pkgType := chroot.GetTaRgetOsPkgType(config.TargetOs)
	if pkgType == "deb" {
		if err = mountDiskRootToChroot(installRoot, diskPathIdMap, template); err != nil {
			return versionInfo, fmt.Errorf("failed to mount disk root to chroot: %w", err)
		}

		if err = initRootfsForDeb(installRoot); err != nil {
			err = fmt.Errorf("failed to initialize rootfs for deb: %w", err)
			goto fail
		}
	}

	mountPointInfoList, err = mountDiskToChroot(installRoot, diskPathIdMap, template)
	if err != nil {
		return versionInfo, fmt.Errorf("failed to mount disk to chroot: %w", err)
	}

	log.Infof("Image installation pre-processing...")
	err = preImageOsInstall(installRoot, template)
	if err != nil {
		err = fmt.Errorf("pre-install failed: %w", err)
		goto fail
	}

	log.Infof("Image package installation...")
	err = installImagePkgs(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to install image packages: %w", err)
		goto fail
	}

	log.Infof("Image system configuration...")
	err = updateImageConfig(installRoot, diskPathIdMap, template)
	if err != nil {
		err = fmt.Errorf("failed to update image config: %w", err)
		goto fail
	}

	log.Infof("Installing bootloader...")
	err = imageboot.InstallImageBoot(installRoot, diskPathIdMap, template)
	if err != nil {
		err = fmt.Errorf("failed to install image boot: %w", err)
		goto fail
	}

	err = imagesecure.ConfigImageSecurity(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to configure image security: %w", err)
		goto fail
	}

	log.Infof("Configuring UKI...")
	err = buildImageUKI(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to configure UKI: %w", err)
		goto fail
	}

	err = imagesign.SignImage(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to sign image: %w", err)
		goto fail
	}

	log.Infof("Image installation post-processing...")
	versionInfo, err = postImageOsInstall(installRoot, template)
	if err != nil {
		err = fmt.Errorf("post-install failed: %w", err)
		goto fail
	}

	if err = umountDiskFromChroot(installRoot, mountPointInfoList); err != nil {
		return versionInfo, fmt.Errorf("failed to unmount disk from chroot: %w", err)
	}

	return versionInfo, nil

fail:
	if umountErr := umountDiskFromChroot(installRoot, mountPointInfoList); umountErr != nil {
		log.Errorf("Failed to unmount disk from chroot after error: %v", umountErr)
	}
	return versionInfo, fmt.Errorf("image OS installation failed: %w", err)
}

func InitChrootInstallRoot(template *config.ImageTemplate) (string, error) {
	if _, err := os.Stat(chroot.ChrootImageBuildDir); os.IsNotExist(err) {
		return "", fmt.Errorf("chroot image build directory does not exist: %s", chroot.ChrootImageBuildDir)
	}
	sysConfigName := template.GetSystemConfigName()
	installRoot := filepath.Join(chroot.ChrootImageBuildDir, sysConfigName)
	if _, err := shell.ExecCmd("mkdir -p "+installRoot, true, "", nil); err != nil {
		return installRoot, fmt.Errorf("failed to create directory %s: %w", installRoot, err)
	}
	return installRoot, nil
}

func initRootfsForDeb(installRoot string) error {
	chrootConfigDir, err := chroot.GetChrootConfigDir(config.TargetOs, config.TargetDist)
	if err != nil {
		return fmt.Errorf("failed to get chroot config directory: %v", err)
	}
	chrootEnvCongfigPath := filepath.Join(chrootConfigDir, "chrootenv_"+config.TargetArch+".yml")
	essentialPkgsList, err := chroot.GetChrootEnvEssentialPackageList(chrootEnvCongfigPath)
	if err != nil {
		return fmt.Errorf("failed to get essential packages list: %v", err)
	}
	pkgListStr := strings.Join(essentialPkgsList, ",")
	localRepoConfigChrootPath := "/etc/apt/sources.list.d/local.list"
	localRepoConfigHostPath, err := chroot.GetChrootEnvHostPath(localRepoConfigChrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment host path for %s: %w", localRepoConfigChrootPath, err)
	}

	if _, err := os.Stat(localRepoConfigHostPath); os.IsNotExist(err) {
		return fmt.Errorf("local repository config file does not exist: %s", localRepoConfigHostPath)
	}

	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path for install root %s: %w", installRoot, err)
	}

	cmd := fmt.Sprintf("mmdebstrap "+
		"--variant=custom "+
		"--format=directory "+
		"--aptopt=APT::Authentication::Trusted=true "+
		"--hook-dir=/usr/share/mmdebstrap/hooks/file-mirror-automount "+
		"--include=%s "+
		"--verbose --debug "+
		"-- bookworm %s %s",
		pkgListStr, chrootInstallRoot, localRepoConfigChrootPath)

	if _, err = shell.ExecCmdWithStream(cmd, true, chroot.ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to install packages into image: %w", err)
	}
	return nil
}

func mountSysfsToRootfs(installRoot string) error {
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	err = chroot.MountChrootSysfs(chrootInstallRoot)
	if err != nil {
		return fmt.Errorf("failed to mount sysfs into image rootfs %s: %w", chrootInstallRoot, err)
	}
	return nil
}

func umountSysfsFromRootfs(installRoot string) error {
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	if err := chroot.UmountChrootSysfs(chrootInstallRoot); err != nil {
		return fmt.Errorf("failed to unmount sysfs for image rootfs: %w", err)
	}
	return nil
}

func mountDiskRootToChroot(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	diskInfo := template.GetDiskConfig()
	partions := diskInfo.Partitions
	for diskId, diskPath := range diskPathIdMap {
		for _, partition := range partions {
			if partition.ID == diskId {
				if partition.MountPoint == "/" {
					mountPoint := filepath.Join(installRoot, partition.MountPoint)
					mountFlags := fmt.Sprintf("-t %s", partition.FsType)
					if err := mount.MountPath(diskPath, mountPoint, mountFlags); err != nil {
						return fmt.Errorf("failed to mount %s to %s: %w", diskPath, mountPoint, err)
					}
					return nil
				}
			}
		}
	}
	return fmt.Errorf("no root partition found in diskPathIdMap")
}

func mountDiskToChroot(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) ([]map[string]string, error) {
	var mountPointInfoList []map[string]string
	diskInfo := template.GetDiskConfig()
	partions := diskInfo.Partitions
	for diskId, diskPath := range diskPathIdMap {
		for _, partition := range partions {
			if partition.ID == diskId {
				mountPointInfo := make(map[string]string)
				mountPointInfo["Id"] = diskId
				mountPointInfo["Path"] = diskPath
				mountPointInfo["MountPoint"] = filepath.Join(installRoot, partition.MountPoint)
				if partition.MountPoint == "/boot/efi" {
					if partition.FsType == "fat32" || partition.FsType == "fat16" {
						mountPointInfo["Flags"] = fmt.Sprintf("-t %s -o umask=0077", "vfat")
					} else {
						mountPointInfo["Flags"] = fmt.Sprintf("-t %s -o umask=0077", partition.FsType)
					}
				} else {
					mountPointInfo["Flags"] = fmt.Sprintf("-t %s", partition.FsType)
				}
				mountPointInfoList = append(mountPointInfoList, mountPointInfo)
			}
		}
	}

	if len(mountPointInfoList) == 0 {
		return nil, fmt.Errorf("no mount points found for the provided diskPathIdMap")
	}

	// sort the mountPointInfoList by the partition.MountPoint
	// mount requires order that the "/" mounted first, then "/boot", "/boot/efi", etc.
	sort.Slice(mountPointInfoList, func(i, j int) bool {
		return mountPointInfoList[i]["MountPoint"] < mountPointInfoList[j]["MountPoint"]
	})

	for _, mountPointInfo := range mountPointInfoList {
		mountPoint := mountPointInfo["MountPoint"]
		path := mountPointInfo["Path"]
		flags := mountPointInfo["Flags"]
		if err := mount.MountPath(path, mountPoint, flags); err != nil {
			return nil, fmt.Errorf("failed to mount %s to %s with flags %s: %w", path, mountPoint, flags, err)
		}
	}

	if err := mountSysfsToRootfs(installRoot); err != nil {
		return nil, err
	}

	return mountPointInfoList, nil
}

func umountDiskFromChroot(installRoot string, mountPointInfoList []map[string]string) error {
	if err := umountSysfsFromRootfs(installRoot); err != nil {
		return err
	}

	mountPointInfoListLen := len(mountPointInfoList)
	for i := mountPointInfoListLen - 1; i >= 0; i-- {
		mountPointInfo := mountPointInfoList[i]
		mountPoint := mountPointInfo["MountPoint"]
		err := mount.UmountPath(mountPoint)
		if err != nil {
			return fmt.Errorf("failed to unmount %s: %w", mountPoint, err)
		}
	}
	return nil
}

func getRpmPkgInstallList(template *config.ImageTemplate) []string {
	var head, middle, tail []string
	imagePkgList := template.GetPackages()
	for _, pkg := range imagePkgList {
		if strings.HasPrefix(pkg, "filesystem") {
			head = append(head, pkg)
		} else if strings.HasPrefix(pkg, "initramfs") {
			tail = append(tail, pkg)
		} else {
			middle = append(middle, pkg)
		}
	}
	return append(append(head, middle...), tail...)
}

func getDebPkgInstallList(template *config.ImageTemplate) []string {
	var head, middle, tail []string
	imagePkgList := template.GetPackages()
	for _, pkg := range imagePkgList {
		if strings.HasPrefix(pkg, "base-files") {
			head = append(head, pkg)
		} else if strings.HasPrefix(pkg, "dracut") {
			tail = append(tail, pkg)
		} else if strings.HasPrefix(pkg, "systemd-boot") {
			tail = append(tail, pkg)
		} else {
			middle = append(middle, pkg)
		}
	}
	return append(append(head, middle...), tail...)
}

func initImageRpmDb(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Initializing RPM database in %s", installRoot)
	rpmDbPath := filepath.Join(installRoot, "var", "lib", "rpm")
	if _, err := os.Stat(rpmDbPath); os.IsNotExist(err) {
		if _, err := shell.ExecCmd("mkdir -p "+rpmDbPath, true, "", nil); err != nil {
			return fmt.Errorf("failed to create RPM database directory: %w", err)
		}
	}
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	cmd := fmt.Sprintf("rpm --root %s --initdb", chrootInstallRoot)
	if _, err := shell.ExecCmd(cmd, true, chroot.ChrootEnvRoot, nil); err != nil {
		return fmt.Errorf("failed to initialize RPM database: %w", err)
	}
	return nil
}

func initDebLocalRepoWithinInstallRoot(installRoot string) error {
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path for install root %s: %w", installRoot, err)
	}

	// from local.list
	repoPath := filepath.Join(chrootInstallRoot, "/cdrom/cache-repo")
	if err := chroot.MountChrootPath(chroot.ChrootPkgCacheDir, repoPath, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory %s to chroot repo directory %s: %w",
			chroot.ChrootPkgCacheDir, repoPath, err)
	}

	imageRepoCongfigPath := filepath.Join(installRoot, "/etc/apt/sources.list.d/", "*")
	if _, err := shell.ExecCmd("rm -f "+imageRepoCongfigPath, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove existing local repo config files: %w", err)
	}

	chrootConfigDir, err := chroot.GetChrootConfigDir(config.TargetOs, config.TargetDist)
	if err != nil {
		return fmt.Errorf("failed to get chroot config directory: %v", err)
	}

	repoCongfigPath := filepath.Join(chrootConfigDir, "local.list")
	if _, err := os.Stat(repoCongfigPath); os.IsNotExist(err) {
		return fmt.Errorf("repo config file does not exist: %s", repoCongfigPath)
	}

	targetPath := filepath.Join(chrootInstallRoot, "/etc/apt/sources.list.d/")
	if err := chroot.CopyFileFromHostToChroot(repoCongfigPath, targetPath); err != nil {
		return fmt.Errorf("failed to copy local repository config file to chroot: %w", err)
	}

	cmd := "apt-get update"
	if _, err := shell.ExecCmdWithStream(cmd, true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
	}

	// Create a policy-rc.d file to prevent service startup in chroot
	policyFile := filepath.Join(installRoot, "/usr/sbin/policy-rc.d")
	policyContent := "#!/bin/sh\nexit 101\n"

	if _, err := shell.ExecCmd("mkdir -p "+filepath.Dir(policyFile), true, "", nil); err != nil {
		return fmt.Errorf("failed to create policy-rc.d directory: %w", err)
	}

	if err := file.Write(policyContent, policyFile); err != nil {
		return fmt.Errorf("failed to create policy-rc.d file: %w", err)
	}

	return nil
}

func deInitDebLocalRepoWithinInstallRoot(installRoot string) error {
	// from local.list
	repoPath := filepath.Join(installRoot, "/cdrom/cache-repo")
	if err := chroot.UmountChrootPath(repoPath); err != nil {
		return fmt.Errorf("failed to unmount chroot repo directory %s: %w", repoPath, err)
	}

	repoconfigPath := filepath.Join(installRoot, "/etc/apt/sources.list.d/local.list")
	if _, err := os.Stat(repoconfigPath); err == nil {
		if _, err := shell.ExecCmd("rm -f "+repoconfigPath, true, "", nil); err != nil {
			return fmt.Errorf("failed to remove local repository config file %s: %w", repoconfigPath, err)
		}
	}

	policyFile := filepath.Join(installRoot, "/usr/sbin/policy-rc.d")
	if _, err := os.Stat(policyFile); err == nil {
		if _, err := shell.ExecCmd("rm -f "+policyFile, true, "", nil); err != nil {
			return fmt.Errorf("failed to remove policy-rc.d file %s: %w", policyFile, err)
		}
	}
	return nil
}

func preImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func installImagePkgs(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	pkgType := chroot.GetTaRgetOsPkgType(config.TargetOs)
	if pkgType == "rpm" {
		err := initImageRpmDb(installRoot, template)
		if err != nil {
			return fmt.Errorf("failed to initialize RPM database: %w", err)
		}
		imagePkgOrderedList := getRpmPkgInstallList(template)
		imagePkgNum := len(imagePkgOrderedList)
		// Force to use the local cache repository
		var repositoryIDList []string = []string{"cache-repo"}
		for i, pkg := range imagePkgOrderedList {
			log.Infof("Installing package %d/%d: %s", i+1, imagePkgNum, pkg)
			if err := chroot.TdnfInstallPackage(pkg, installRoot, repositoryIDList); err != nil {
				return fmt.Errorf("failed to install package %s: %w", pkg, err)
			}
		}
	} else if pkgType == "deb" {
		imagePkgOrderedList := getDebPkgInstallList(template)
		// Prepare local cache repository
		if err := initDebLocalRepoWithinInstallRoot(installRoot); err != nil {
			return fmt.Errorf("failed to initialize local repository within install root: %w", err)
		}
		imagePkgNum := len(imagePkgOrderedList)
		// Force to use the local cache repository
		var repoSrcList []string = []string{"/etc/apt/sources.list.d/local.list"}
		for i, pkg := range imagePkgOrderedList {
			log.Infof("Installing package %d/%d: %s", i+1, imagePkgNum, pkg)
			if pkg == "systemd-boot" {
				// systemd-boot is a special case,
				// 'Failed to write 'LoaderSystemToken' EFI variable: No such file or directory' error is expected.
				installCmd := fmt.Sprintf("apt-get install -y %s", pkg)

				if len(repoSrcList) > 0 {
					for _, repoSrc := range repoSrcList {
						installCmd += fmt.Sprintf(" -o Dir::Etc::sourcelist=%s", repoSrc)
					}
				}
				output, err := shell.ExecCmdWithStream(installCmd, true, installRoot, nil)
				if err != nil {
					if strings.Contains(output, "Failed to write 'LoaderSystemToken' EFI variable") {
						log.Debugf("Expected error: The EFI variable shouldn't be accessed in chroot.")
					} else {
						return fmt.Errorf("failed to install package %s: %w", pkg, err)
					}
				}
			} else {
				if err := chroot.AptInstallPackage(pkg, installRoot, repoSrcList); err != nil {
					return fmt.Errorf("failed to install package %s: %w", pkg, err)
				}
			}
		}
		if err := deInitDebLocalRepoWithinInstallRoot(installRoot); err != nil {
			return fmt.Errorf("failed to de-initialize local repository within install root: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}
	return nil
}

func updateInitrdConfig(installRoot string, template *config.ImageTemplate) error {
	if err := updateImageHostname(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image hostname: %w", err)
	}
	if err := updateImageUsrGroup(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image user/group: %w", err)
	}
	if err := updateImageNetwork(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image network: %w", err)
	}
	if err := addImageAdditionalFiles(installRoot, template); err != nil {
		return fmt.Errorf("failed to add additional files to image: %w", err)
	}
	if err := addImageIDFile(installRoot, template); err != nil {
		return fmt.Errorf("failed to add image ID file: %w", err)
	}
	return nil
}

func updateImageConfig(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	if err := updateImageHostname(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image hostname: %w", err)
	}
	if err := updateImageUsrGroup(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image user/group: %w", err)
	}
	if err := updateImageNetwork(installRoot, template); err != nil {
		return fmt.Errorf("failed to update image network: %w", err)
	}
	if err := addImageAdditionalFiles(installRoot, template); err != nil {
		return fmt.Errorf("failed to add additional files to image: %w", err)
	}
	if err := addImageIDFile(installRoot, template); err != nil {
		return fmt.Errorf("failed to add image ID file: %w", err)
	}
	if err := updateImageFstab(installRoot, diskPathIdMap, template); err != nil {
		return fmt.Errorf("failed to update image fstab: %w", err)
	}
	return nil
}

func getImageVersionInfo(installRoot string, template *config.ImageTemplate) (string, error) {
	var versionInfo string
	var prefix string
	log := logger.Logger()
	log.Infof("Getting image version info for: %s", template.GetImageName())
	imageVersionFilePath := filepath.Join(installRoot, "etc", "os-release")
	if _, err := os.Stat(imageVersionFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("os-release file does not exist: %s", imageVersionFilePath)
	}
	content, err := file.Read(imageVersionFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image version file: %w", err)
	}

	switch config.TargetOs {
	case "azure-linux", "edge-microvisor-toolkit":
		prefix = "VERSION="
	case "wind-river-elxr":
		prefix = "VERSION_ID="
	default:
		prefix = "VERSION_ID="
	}
	// Parse the content to extract version information
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			// Remove prefix, quotes and trim whitespace
			line = strings.TrimPrefix(line, prefix)
			versionInfo = strings.TrimSpace(strings.Trim(line, "\""))
		}
	}
	if versionInfo == "" {
		log.Debugf("Version info not found in %s", imageVersionFilePath)
	} else {
		log.Infof("Extracted version info: %s", versionInfo)
	}
	return versionInfo, nil
}

func postImageOsInstall(installRoot string, template *config.ImageTemplate) (string, error) {
	versionInfo, err := getImageVersionInfo(installRoot, template)
	if err != nil {
		return versionInfo, fmt.Errorf("failed to get image version info: %w", err)
	}
	return versionInfo, nil
}

func updateImageHostname(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageUsrGroup(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Configuring User...")
	err := createUser(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to configuring User: %w", err)
	}
	return nil
}

func updateImageNetwork(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func addImageIDFile(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Adding image ID file for image: %s", template.GetImageName())
	imageIDFilePath := filepath.Join(installRoot, "etc", "image-id")
	// Get the current time in UTC and in format "YYYYMMDDHHMMSS"
	imageBuildDate := time.Now().UTC().Format("20060102150405")
	imageIDContent := fmt.Sprintf("IMAGE_BUILD_DATE=%s\nIMAGE_UUID=%s\n", imageBuildDate, uuid.New().String())
	if err := file.Write(imageIDContent, imageIDFilePath); err != nil {
		return fmt.Errorf("failed to write file %s: %w", imageIDFilePath, err)
	}
	if _, err := shell.ExecCmd("chmod 0444 "+imageIDFilePath, true, "", nil); err != nil {
		return fmt.Errorf("failed to set permissions for image ID file %s: %w", imageIDFilePath, err)
	}
	return nil
}

func addImageAdditionalFiles(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Adding additional files to image: %s", template.GetImageName())
	additionalFiles := template.SystemConfig.AdditionalFiles
	if len(additionalFiles) == 0 {
		log.Debug("No additional files to add to the image")
		return nil
	}
	targetOsConfigDir, err := config.GetTargetOsConfigDir(template.Target.OS, template.Target.Dist)
	if err != nil {
		return fmt.Errorf("failed to get target OS config directory: %w", err)
	}
	additionalFilesPath := filepath.Join(targetOsConfigDir, "imageconfigs", "additionalfiles")
	if _, err := os.Stat(additionalFilesPath); os.IsNotExist(err) {
		return fmt.Errorf("additional files directory does not exist: %s", additionalFilesPath)
	}

	for _, fileInfo := range additionalFiles {
		srcFile := filepath.Join(additionalFilesPath, fileInfo.Local)
		dstFile := filepath.Join(installRoot, fileInfo.Final)
		if err := file.CopyFile(srcFile, dstFile, "-p", true); err != nil {
			return fmt.Errorf("failed to copy additional file %s to image: %w", srcFile, err)
		}
		log.Debugf("Successfully added additional file: %s", dstFile)
	}
	return nil
}

func updateImageFstab(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	const (
		rootfsMountPoint = "/"
		defaultOptions   = "defaults"
		swapFsType       = "swap"
		swapOptions      = "sw"
		defaultDump      = "0"
		disablePass      = "0"
		rootPass         = "1"
		defaultPass      = "2"
	)
	log := logger.Logger()
	log.Infof("Updating fstab for image: %s", template.GetImageName())
	fstabFullPath := filepath.Join(installRoot, "etc", "fstab")
	diskInfo := template.GetDiskConfig()
	partitions := diskInfo.Partitions
	for diskId, diskPath := range diskPathIdMap {
		for _, partition := range partitions {
			if partition.ID == diskId {
				// Get the partition UUID and mount point
				partUUID, err := imagedisc.GetPartUUID(diskPath)
				if err != nil {
					return fmt.Errorf("failed to get partition UUID for %s: %w", diskPath, err)
				}
				mountId := fmt.Sprintf("PARTUUID=%s", partUUID)
				mountPoint := partition.MountPoint

				// Get the filesystem type
				var fsType, options, pass string
				if partition.FsType == "fat16" || partition.FsType == "fat32" {
					fsType = "vfat"
				} else {
					fsType = partition.FsType
				}

				// Get the mount options
				options = defaultOptions
				if partition.MountOptions != "" {
					options = partition.MountOptions
				}

				// Get the default dump and pass values
				pass = defaultPass
				if mountPoint == rootfsMountPoint {
					pass = rootPass
				}

				if fsType == swapFsType {
					// For swap partitions, set the options accordingly
					options = swapOptions
					pass = disablePass // No pass value for swap
				}

				newEntry := fmt.Sprintf("%v %v %v %v %v %v\n",
					mountId, mountPoint, fsType, options, defaultDump, pass)
				log.Debugf("Adding fstab entry: %s", newEntry)
				err = file.Append(newEntry, fstabFullPath)
				if err != nil {
					return fmt.Errorf("failed to append fstab entry for %s: %w", mountPoint, err)
				}
			}
		}
	}
	return nil
}

func buildImageUKI(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	bootloaderConfig := template.GetBootloaderConfig()
	if bootloaderConfig.Provider == "systemd-boot" {
		// 1. Update initramfs
		kernelVersion, err := getKernelVersion(installRoot)
		if err != nil {
			return fmt.Errorf("failed to get kernel version: %w", err)
		}

		log.Debugf("Kernel version:%s", kernelVersion)

		if err := updateInitramfs(installRoot, kernelVersion, template); err != nil {
			return fmt.Errorf("failed to update initramfs: %w", err)
		}

		log.Debug("initrd updated successfully")

		// 2. Build UKI with ukify
		kernelPath := filepath.Join("/boot", "vmlinuz-"+kernelVersion)
		initrdPath := fmt.Sprintf("/boot/initramfs-%s.img", kernelVersion)

		espRoot := installRoot
		espDir, err := prepareESPDir(espRoot)
		if err != nil {
			return fmt.Errorf("failed to prepare ESP directory: %w", err)
		}
		log.Debugf("Succesfully Creating EspPath:", espDir)

		outputPath := filepath.Join(espDir, "EFI", "Linux", "linux.efi")
		log.Debugf("UKI Path:", outputPath)

		cmdlineFile := filepath.Join("/boot", "cmdline.conf")
		if err := buildUKI(installRoot, kernelPath, initrdPath, cmdlineFile, outputPath, template); err != nil {
			return fmt.Errorf("failed to build UKI: %w", err)
		}
		log.Debugf("UKI created successfully on:", outputPath)

		// 3. Copy systemd-bootx64.efi to ESP/EFI/BOOT/BOOTX64.EFI
		srcBootloader := filepath.Join("usr", "lib", "systemd", "boot", "efi", "systemd-bootx64.efi")
		dstBootloader := filepath.Join(espDir, "EFI", "BOOT", "BOOTX64.EFI")
		if err := copyBootloader(installRoot, srcBootloader, dstBootloader); err != nil {
			return fmt.Errorf("failed to copy bootloader: %w", err)
		}
		log.Debugf("bootloader copied successfully on:", dstBootloader)
	} else {
		log.Infof("Skipping UKI build for image: %s, bootloader provider is not systemd-boot", template.GetImageName())
	}

	return nil
}

// Helper to get the current kernel version from the rootfs
func getKernelVersion(installRoot string) (string, error) {
	kernelDir := filepath.Join(installRoot, "boot")
	fileList, err := file.GetFileList(kernelDir)
	if err != nil {
		return "", fmt.Errorf("failed to list kernel directory %s: %w", kernelDir, err)
	}
	for _, f := range fileList {
		if strings.HasPrefix(f, "vmlinuz-") {
			return strings.TrimPrefix(f, "vmlinuz-"), nil
		}
	}
	return "", fmt.Errorf("kernel image not found in %s", kernelDir)
}

// Helper to update initramfs for the given kernel version
func updateInitramfs(installRoot, kernelVersion string, template *config.ImageTemplate) error {
	initrdPath := fmt.Sprintf("/boot/initramfs-%s.img", kernelVersion)
	if template.IsImmutabilityEnabled() {
		cmd := fmt.Sprintf(
			"dracut --force --add systemd-veritysetup --no-hostonly --verbose --kver %s %s",
			kernelVersion,
			initrdPath,
		)
		_, err := shell.ExecCmd(cmd, true, installRoot, nil)
		return err
	}
	// Check if the initrdPath file exists; if not, create it
	fullInitrdPath := filepath.Join(installRoot, initrdPath)
	if _, err := os.Stat(fullInitrdPath); err == nil {
		// initrd file already exists
		log := logger.Logger()
		log.Debugf("Initramfs already exists, skipping update: %s", fullInitrdPath)
		return nil
	}
	cmd := fmt.Sprintf(
		"dracut -f %s %s",
		initrdPath,
		kernelVersion,
	)
	_, err := shell.ExecCmd(cmd, true, installRoot, nil)
	return err
}

// Helper to determine the ESP directory (assumes /boot/efi)
func prepareESPDir(installRoot string) (string, error) {
	espDirs := []string{
		"/boot/efi",
		"/boot/efi/EFI/Linux",
		"/boot/efi/EFI/BOOT",
	}

	// remove previous bootloader
	cleanupDirs := []string{
		"/boot/efi/*",
	}

	// Remove all from efi directories
	for _, dir := range cleanupDirs {
		cmd := fmt.Sprintf("sh -c 'rm -rf %s'", dir)
		if _, err := shell.ExecCmd(cmd, true, installRoot, nil); err != nil {
			return "", err
		}
	}

	// Create required ESP directories
	for _, dir := range espDirs {
		cmd := fmt.Sprintf("mkdir -p %s", dir)
		if _, err := shell.ExecCmd(cmd, true, installRoot, nil); err != nil {
			return "", err
		}
	}

	// Return the ESP root directory
	return espDirs[0], nil
}

func extractRootHashPH(input string) string {
	parts := strings.Fields(input)
	for _, part := range parts {
		if strings.HasPrefix(part, "roothash=") {
			val := strings.TrimPrefix(part, "roothash=")
			val = strings.ReplaceAll(val, "-", " ")
			return val
		}
	}
	return ""
}

func replaceRootHashPH(input, newRootHash string) string {
	parts := strings.Fields(input)
	for i, part := range parts {
		if strings.HasPrefix(part, "roothash=") {
			parts[i] = "roothash=" + newRootHash
			break
		}
	}
	return strings.Join(parts, " ")
}

func prepareVeritySetup(partPair, installRoot string) error {
	// Extract the first part of partPair (before the space)
	parts := strings.Fields(partPair)
	if len(parts) < 1 {
		return fmt.Errorf("invalid partPair: %s", partPair)
	}
	device := parts[0]

	// Remount the device as read-only
	remountCmd := fmt.Sprintf("mount -o remount,ro %s", device)
	log := logger.Logger()
	log.Debugf("Remounting device as read-only: %s", remountCmd)
	if _, err := shell.ExecCmd(remountCmd, true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to remount %s as read-only: %w", device, err)
	}

	// Create and mount /tmp for ukify (Python tempfile needs this)
	tmpDir := filepath.Join(installRoot, "tmp")
	if _, err := shell.ExecCmd("mkdir -p "+tmpDir, true, "", nil); err != nil {
		return fmt.Errorf("failed to create /tmp directory: %w", err)
	}
	if _, err := shell.ExecCmd("mount -t tmpfs tmpfs /tmp", true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to mount tmpfs on /tmp: %w", err)
	}
	if _, err := shell.ExecCmd("chmod 1777 /tmp", true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to chmod 1777 on /tmp: %w", err)
	}

	// Create and mount /boot/efi/tmp for veritysetup
	veritytmpDir := filepath.Join(installRoot, "boot/efi/tmp")
	if _, err := shell.ExecCmd("mkdir -p "+veritytmpDir, true, "", nil); err != nil {
		return fmt.Errorf("failed to create /boot/efi/tmp directory: %w", err)
	}
	if _, err := shell.ExecCmd("mount -t tmpfs tmpfs /boot/efi/tmp", true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to mount tmpfs on /boot/efi/tmp: %w", err)
	}
	if _, err := shell.ExecCmd("chmod 1777 /boot/efi/tmp", true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to chmod 1777 on /boot/efi/tmp: %w", err)
	}
	return nil
}

func removeVerityTmp(installRoot string) {
	log := logger.Logger()

	// Unmount and clean up /tmp
	tmpDir := filepath.Join(installRoot, "tmp")
	if _, err := shell.ExecCmd("umount /tmp", true, installRoot, nil); err != nil {
		log.Warnf("Failed to unmount tmpfs on /tmp: %v", err)
	}
	if _, err := shell.ExecCmd("rm -rf "+tmpDir, true, "", nil); err != nil {
		log.Warnf("Failed to remove tmp directory %s: %v", tmpDir, err)
	}

	// Unmount and clean up /boot/efi/tmp
	veritytmpDir := filepath.Join(installRoot, "boot/efi/tmp")
	if _, err := shell.ExecCmd("umount /boot/efi/tmp", true, installRoot, nil); err != nil {
		log.Warnf("Failed to unmount tmpfs on /boot/efi/tmp: %v", err)
	}

	if _, err := shell.ExecCmd("rm -rf "+veritytmpDir, true, "", nil); err != nil {
		log.Warnf("Failed to remove tmp directory %s: %v", veritytmpDir, err)
	}
}

func getVerityRootHash(partPair, installRoot string) (string, error) {
	log := logger.Logger()
	cmd := fmt.Sprintf(`veritysetup format %s`, partPair)
	log.Debugf("Veritysetup Executing command:", cmd)
	// runs on host
	exists, _ := shell.IsCommandExist("ukify", installRoot)
	if !exists {
		installRoot = ""
	}
	output, err := shell.ExecCmd(cmd, true, installRoot, nil)
	if err != nil {
		return "", fmt.Errorf("failed to run veritysetup format: %w", err)
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Root hash:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2], nil
			}
		}
	}
	return "", fmt.Errorf("root hash not found in veritysetup output")
}

// Helper to build UKI using ukify
func buildUKI(installRoot, kernelPath, initrdPath, cmdlineFile, outputPath string, template *config.ImageTemplate) error {
	data, err := file.Read(filepath.Join(installRoot, cmdlineFile))
	if err != nil {
		return fmt.Errorf("failed to read cmdline file: %w", err)
	}
	cmdlineStr := string(data)
	if template.IsImmutabilityEnabled() {
		partData := extractRootHashPH(cmdlineStr)
		err := prepareVeritySetup(partData, installRoot)
		if err != nil {
			return fmt.Errorf("failed to get root hash part: %w", err)
		}
		rootHashR, err := getVerityRootHash(partData, installRoot)
		if err != nil {
			return fmt.Errorf("failed to get verity root hash: %w", err)
		}
		cmdlineStr = replaceRootHashPH(cmdlineStr, rootHashR)
	}

	// runs on host
	var cmd string
	var backInstallRoot = installRoot
	exists, _ := shell.IsCommandExist("ukify", installRoot)
	if !exists {
		kernelPath = filepath.Join(installRoot, kernelPath)
		initrdPath = filepath.Join(installRoot, initrdPath)
		outputPath = filepath.Join(installRoot, outputPath)
		osRelease := filepath.Join(installRoot, "/etc/os-release")
		installRoot = ""

		cmd = fmt.Sprintf(
			"ukify build --linux \"%s\" --initrd \"%s\" --cmdline \"%s\" --os-release @\"%s\" --output \"%s\"",
			kernelPath,
			initrdPath,
			cmdlineStr,
			osRelease,
			outputPath,
		)

	} else {
		cmd = fmt.Sprintf(
			"ukify build --linux \"%s\" --initrd \"%s\" --cmdline \"%s\" --output \"%s\"",
			kernelPath,
			initrdPath,
			cmdlineStr,
			outputPath,
		)
	}

	log := logger.Logger()
	log.Debugf("UKI Executing command:", cmd)
	if template.IsImmutabilityEnabled() {
		// Set TMPDIR environment variable to use the mounted tmpfs
		envVars := []string{"TMPDIR=/tmp"}
		_, err = shell.ExecCmd(cmd, true, installRoot, envVars)
		installRoot = backInstallRoot
		removeVerityTmp(installRoot)
	} else {
		_, err = shell.ExecCmd(cmd, true, installRoot, nil)
	}
	return err
}

// Helper to copy the bootloader EFI file
func copyBootloader(installRoot, src, dst string) error {
	// src and dst should be absolute paths inside the chroot
	// (e.g., /usr/lib/systemd/boot/efi/systemd-bootx64.efi
	// and /boot/efi/EFI/BOOT/BOOTX64.EFI)
	cmd := fmt.Sprintf("cp %s %s", src, dst)
	_, err := shell.ExecCmd(cmd, true, installRoot, nil)
	return err
}

func createUser(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	user := "user"
	pwd := "user"

	log.Infof("Creating user: %s", user)

	// Create the user with useradd command
	// -m creates home directory, -s sets shell
	cmd := fmt.Sprintf("useradd -m -s /bin/bash %s", user)
	output, err := shell.ExecCmdSilent(cmd, true, installRoot, nil)
	if err != nil {
		if strings.Contains(output, "already exists") {
			log.Warnf("User %s already exists", user)
		} else {
			return fmt.Errorf("failed to create user %s: output: %s, err: %w", user, output, err)
		}
	}

	// Set password using passwd command with expect-like approach
	passwdInput := fmt.Sprintf("%s\n%s\n", pwd, pwd)
	passwdCmd := fmt.Sprintf("passwd %s", user)
	if _, err := shell.ExecCmdWithInput(passwdInput, passwdCmd, true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to set password for user %s: %w", user, err)
	}

	// Add user to sudo group for sudo permissions
	sudoCmd := fmt.Sprintf("usermod -aG sudo %s", user)
	if _, err := shell.ExecCmd(sudoCmd, true, installRoot, nil); err != nil {
		return fmt.Errorf("failed to add user %s to sudo group: %w", user, err)
	}

	// Verify user creation
	if err := verifyUserCreated(installRoot, user); err != nil {
		return fmt.Errorf("user verification failed: %w", err)
	}

	log.Infof("User %s created successfully with sudo permissions", user)
	return nil
}

// Verify that the user was created correctly
func verifyUserCreated(installRoot, username string) error {
	log := logger.Logger()

	// Check if user exists in passwd file
	passwdCmd := fmt.Sprintf("grep '^%s:' /etc/passwd", username)
	output, err := shell.ExecCmd(passwdCmd, true, installRoot, nil)
	if err != nil {
		return fmt.Errorf("user %s not found in passwd file: %w", username, err)
	}
	log.Debugf("User in passwd: %s", strings.TrimSpace(output))

	// Check if user has password in shadow file
	shadowCmd := fmt.Sprintf("grep '^%s:' /etc/shadow", username)
	output, err = shell.ExecCmd(shadowCmd, true, installRoot, nil)
	if err != nil {
		return fmt.Errorf("user %s not found in shadow file: %w", username, err)
	}
	log.Debugf("User in shadow: %s", strings.TrimSpace(output))

	// Check if account is locked (password field starts with ! or *)
	shadowFields := strings.Split(strings.TrimSpace(output), ":")
	if len(shadowFields) >= 2 {
		passwordField := shadowFields[1]
		if strings.HasPrefix(passwordField, "!") || strings.HasPrefix(passwordField, "*") {
			return fmt.Errorf("user %s account is locked (password field: %s)", username, passwordField)
		}
		if passwordField == "" {
			return fmt.Errorf("user %s has no password set", username)
		}
	}

	// Check sudo group membership
	groupCmd := fmt.Sprintf("groups %s", username)
	output, err = shell.ExecCmd(groupCmd, true, installRoot, nil)
	if err != nil {
		return fmt.Errorf("failed to check groups for user %s: %w", username, err)
	}
	log.Debugf("User groups: %s", strings.TrimSpace(output))

	if !strings.Contains(output, "sudo") {
		return fmt.Errorf("user %s is not in sudo group", username)
	}

	return nil
}
