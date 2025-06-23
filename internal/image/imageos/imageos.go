package imageos

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/image-composer/internal/image/imagesign"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func InstallImageOs(diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	var err error
	log := logger.Logger()
	log.Infof("Installing OS for image: %s", template.GetImageName())

	installRoot, err := initChrootInstallRoot(template)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot install root: %w", err)
	}

	mountPointInfoList, err := mountDiskToChroot(installRoot, diskPathIdMap, template)
	if err != nil {
		return fmt.Errorf("failed to mount disk to chroot: %w", err)
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
	err = updateImageConfig(installRoot, template)
	if err != nil {
		err = fmt.Errorf("failed to update image config: %w", err)
		goto fail
	}

	log.Infof("Installing bootloader...")
	err = imageboot.InstallImageBoot(diskPathIdMap, template)
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
	err = postImageOsInstall(installRoot, template)
	if err != nil {
		err = fmt.Errorf("post-install failed: %w", err)
		goto fail
	}

	if err = umountDiskFromChroot(installRoot, mountPointInfoList); err != nil {
		return fmt.Errorf("failed to unmount disk from chroot: %w", err)
	}

	return nil

fail:
	if umountErr := umountDiskFromChroot(installRoot, mountPointInfoList); umountErr != nil {
		log.Errorf("Failed to unmount disk from chroot after error: %v", umountErr)
	}
	return fmt.Errorf("image OS installation failed: %w", err)
}

func initChrootInstallRoot(template *config.ImageTemplate) (string, error) {
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

	// mount sysfs into the image rootfs
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	err = chroot.MountChrootSysfs(chrootInstallRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to mount sysfs into image rootfs %s: %w", chrootInstallRoot, err)
	}

	return mountPointInfoList, nil
}

func umountDiskFromChroot(installRoot string, mountPointInfoList []map[string]string) error {
	chrootInstallRoot, err := chroot.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	if err := chroot.UmountChrootSysfs(chrootInstallRoot); err != nil {
		return fmt.Errorf("failed to unmount sysfs for image rootfs: %w", err)
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

func getImagePkgInstallList(template *config.ImageTemplate) []string {
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

func initImageRpmDb(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Initializing RPM database in %s", installRoot)
	rpmDbPath := filepath.Join(installRoot, "var", "lib", "rpm")
	if _, err := os.Stat(rpmDbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(rpmDbPath, 0755); err != nil {
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

func preImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func installImagePkgs(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	err := initImageRpmDb(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to initialize RPM database: %w", err)
	}
	imagePkgOrderedList := getImagePkgInstallList(template)
	imagePkgNum := len(imagePkgOrderedList)
	// Force to use the local cache repository
	//var repositoryIDList []string = []string{"cache-repo"}

	// workaround for bypass the cache-repo, use remote repo.
	var repositoryIDList []string = []string{}
	for i, pkg := range imagePkgOrderedList {
		log.Infof("Installing package %d/%d: %s", i+1, imagePkgNum, pkg)
		if err := chroot.TdnfInstallPackage(pkg, installRoot, repositoryIDList); err != nil {
			return fmt.Errorf("failed to install package %s: %w", pkg, err)
		}
	}
	return nil
}

func updateImageConfig(installRoot string, template *config.ImageTemplate) error {
	err := updateImageHostname(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image hostname: %w", err)
	}
	err = updateImageUsrGroup(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image user/group: %w", err)
	}
	err = updateImageNetwork(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image network: %w", err)
	}
	err = addImageAdditionalFiles(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to add additional files to image: %w", err)
	}
	return nil
}

func postImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageHostname(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageUsrGroup(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageNetwork(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func addImageAdditionalFiles(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func buildImageUKI(installRoot string, template *config.ImageTemplate) error {
	return nil
}
