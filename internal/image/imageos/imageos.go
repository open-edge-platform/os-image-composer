package imageos

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/open-edge-platform/os-image-composer/internal/chroot"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/os-image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/os-image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/os-image-composer/internal/image/imagesign"
	"github.com/open-edge-platform/os-image-composer/internal/utils/file"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/mount"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

type ImageOsInterface interface {
	GetInstallRoot() string
	InstallInitrd() (installRoot, versionInfo string, err error)
	InstallImageOs(diskPathIdMap map[string]string) (versionInfo string, err error)
}

type ImageOs struct {
	installRoot string
	template    *config.ImageTemplate
	chrootEnv   chroot.ChrootEnvInterface
	imageBoot   imageboot.ImageBootInterface
}

var log = logger.Logger()

func NewImageOs(chrootEnv chroot.ChrootEnvInterface, template *config.ImageTemplate) (*ImageOs, error) {
	chrootImageBuildDir := chrootEnv.GetChrootImageBuildDir()
	if _, err := os.Stat(chrootImageBuildDir); os.IsNotExist(err) {
		log.Errorf("Chroot image build directory does not exist: %s", chrootImageBuildDir)
		return nil, fmt.Errorf("chroot image build directory does not exist: %s", chrootImageBuildDir)
	}
	if template == nil {
		return nil, fmt.Errorf("image template cannot be nil")
	}
	sysConfigName := template.GetSystemConfigName()
	installRoot := filepath.Join(chrootImageBuildDir, sysConfigName)
	if _, err := shell.ExecCmd("mkdir -p "+installRoot, true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to create install root directory %s: %v", installRoot, err)
		return nil, fmt.Errorf("failed to create directory %s: %w", installRoot, err)
	}

	imageBoot := imageboot.NewImageBoot()

	return &ImageOs{
		installRoot: installRoot,
		template:    template,
		chrootEnv:   chrootEnv,
		imageBoot:   imageBoot,
	}, nil
}

func (imageOs *ImageOs) GetInstallRoot() string {
	return imageOs.installRoot
}

func (imageOs *ImageOs) InstallInitrd() (installRoot, versionInfo string, err error) {
	installRoot = imageOs.installRoot
	versionInfo = ""
	log.Infof("Installing initrd for image: %s", imageOs.template.GetImageName())

	pkgType := imageOs.chrootEnv.GetTargetOsPkgType()
	if pkgType == "deb" {
		if err = imageOs.initRootfsForDeb(imageOs.installRoot); err != nil {
			err = fmt.Errorf("failed to initialize rootfs for deb: %w", err)
			return
		}
	}

	if err = imageOs.mountSysfsToRootfs(imageOs.installRoot); err != nil {
		return
	}

	defer func() {
		if umountErr := imageOs.umountSysfsFromRootfs(imageOs.installRoot); umountErr != nil {
			if err != nil {
				err = fmt.Errorf("operation failed: %w, cleanup errors: %v", err, umountErr)
			} else {
				err = fmt.Errorf("failed to unmount sysfs from image rootfs: %w", umountErr)
			}
		}
	}()

	log.Infof("Image installation pre-processing...")
	if err = preImageOsInstall(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("pre-install failed: %w", err)
		return
	}

	log.Infof("Image package installation...")
	if err = imageOs.installImagePkgs(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to install image packages: %w", err)
		return
	}

	log.Infof("Image system configuration...")
	if err = updateInitrdConfig(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to update image config: %w", err)
		return
	}

	log.Infof("Image installation post-processing...")
	versionInfo, err = imageOs.postImageOsInstall(imageOs.installRoot, imageOs.template)
	if err != nil {
		err = fmt.Errorf("post-install failed: %w", err)
		return
	}

	return
}

func (imageOs *ImageOs) InstallImageOs(diskPathIdMap map[string]string) (versionInfo string, err error) {
	versionInfo = ""
	var mountPointInfoList []map[string]string
	var mounted bool = false
	log.Infof("Installing OS for image: %s", imageOs.template.GetImageName())

	defer func() {
		if mounted {
			if umountErr := imageOs.umountDiskFromChroot(imageOs.installRoot, mountPointInfoList); umountErr != nil {
				if err != nil {
					err = fmt.Errorf("operation failed: %w, cleanup errors: %v", err, umountErr)
				} else {
					err = fmt.Errorf("failed to unmount disk from chroot: %w", umountErr)
				}
			}
		}
	}()

	pkgType := imageOs.chrootEnv.GetTargetOsPkgType()
	if pkgType == "deb" {
		if err = mountDiskRootToChroot(imageOs.installRoot, diskPathIdMap, imageOs.template); err != nil {
			err = fmt.Errorf("failed to mount disk root to chroot: %w", err)
			return
		}
		mounted = true
		if err = imageOs.initRootfsForDeb(imageOs.installRoot); err != nil {
			err = fmt.Errorf("failed to initialize rootfs for deb: %w", err)
			return
		}
	}

	mountPointInfoList, err = imageOs.mountDiskToChroot(imageOs.installRoot, diskPathIdMap, imageOs.template)
	if err != nil {
		err = fmt.Errorf("failed to mount disk to chroot: %w", err)
		return
	}
	mounted = true

	log.Infof("Image installation pre-processing...")
	if err = preImageOsInstall(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("pre-install failed: %w", err)
		return
	}

	log.Infof("Image package installation...")
	if err = imageOs.installImagePkgs(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to install image packages: %w", err)
		return
	}

	log.Infof("Image system configuration...")
	if err = updateImageConfig(imageOs.installRoot, diskPathIdMap, imageOs.template); err != nil {
		err = fmt.Errorf("failed to update image config: %w", err)
		return
	}

	log.Infof("Installing bootloader...")
	if err = imageOs.imageBoot.InstallImageBoot(imageOs.installRoot, diskPathIdMap, imageOs.template); err != nil {
		err = fmt.Errorf("failed to install image boot: %w", err)
		return
	}

	if err = imagesecure.ConfigImageSecurity(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to configure image security: %w", err)
		return
	}

	log.Infof("Configuring UKI...")
	if err = buildImageUKI(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to configure UKI: %w", err)
		return
	}

	if err = imagesign.SignImage(imageOs.installRoot, imageOs.template); err != nil {
		err = fmt.Errorf("failed to sign image: %w", err)
		return
	}

	log.Infof("Image installation post-processing...")
	versionInfo, err = imageOs.postImageOsInstall(imageOs.installRoot, imageOs.template)
	if err != nil {
		err = fmt.Errorf("post-install failed: %w", err)
		return
	}

	return
}

func (imageOs *ImageOs) initRootfsForDeb(installRoot string) error {
	essentialPkgsList, err := imageOs.chrootEnv.GetChrootEnvEssentialPackageList()
	if err != nil {
		return fmt.Errorf("failed to get essential packages list: %w", err)
	}
	pkgListStr := strings.Join(essentialPkgsList, ",")
	localRepoConfigChrootPath := "/etc/apt/sources.list.d/local.list"
	localRepoConfigHostPath, err := imageOs.chrootEnv.GetChrootEnvHostPath(localRepoConfigChrootPath)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment host path for %s: %w", localRepoConfigChrootPath, err)
	}

	if _, err := os.Stat(localRepoConfigHostPath); os.IsNotExist(err) {
		log.Errorf("Local repository config file does not exist: %s", localRepoConfigHostPath)
		return fmt.Errorf("local repository config file does not exist: %s", localRepoConfigHostPath)
	}

	chrootInstallRoot, err := imageOs.chrootEnv.GetChrootEnvPath(installRoot)
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

	chrootEnvRoot := imageOs.chrootEnv.GetChrootEnvRoot()
	if _, err = shell.ExecCmdWithStream(cmd, true, chrootEnvRoot, nil); err != nil {
		log.Errorf("Failed to install essential packages into image: %v", err)
		return fmt.Errorf("failed to install packages into image: %w", err)
	}
	return nil
}

func (imageOs *ImageOs) mountSysfsToRootfs(installRoot string) error {
	chrootInstallRoot, err := imageOs.chrootEnv.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	if err = imageOs.chrootEnv.MountChrootSysfs(chrootInstallRoot); err != nil {
		return fmt.Errorf("failed to mount sysfs into image rootfs %s: %w", chrootInstallRoot, err)
	}
	return nil
}

func (imageOs *ImageOs) umountSysfsFromRootfs(installRoot string) error {
	chrootInstallRoot, err := imageOs.chrootEnv.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	if err := imageOs.chrootEnv.UmountChrootSysfs(chrootInstallRoot); err != nil {
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
						log.Errorf("Failed to mount %s to %s: %v", diskPath, mountPoint, err)
						return fmt.Errorf("failed to mount %s to %s: %w", diskPath, mountPoint, err)
					}
					return nil
				}
			}
		}
	}
	log.Errorf("No root partition found in diskPathIdMap")
	return fmt.Errorf("no root partition found in diskPathIdMap")
}

func (imageOs *ImageOs) mountDiskToChroot(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) ([]map[string]string, error) {
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
		log.Errorf("No mount points found for the provided diskPathIdMap")
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
			log.Errorf("Failed to mount %s to %s with flags %s: %v", path, mountPoint, flags, err)
			return nil, fmt.Errorf("failed to mount %s to %s with flags %s: %w", path, mountPoint, flags, err)
		}
	}

	if err := imageOs.mountSysfsToRootfs(installRoot); err != nil {
		return nil, err
	}

	return mountPointInfoList, nil
}

func (imageOs *ImageOs) umountDiskFromChroot(installRoot string, mountPointInfoList []map[string]string) error {
	if err := imageOs.umountSysfsFromRootfs(installRoot); err != nil {
		return err
	}

	mountPointInfoListLen := len(mountPointInfoList)
	for i := mountPointInfoListLen - 1; i >= 0; i-- {
		mountPointInfo := mountPointInfoList[i]
		mountPoint := mountPointInfo["MountPoint"]
		if err := mount.UmountPath(mountPoint); err != nil {
			log.Errorf("Failed to unmount %s: %v", mountPoint, err)
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
	imagePkgList := append(template.GetKernelPackages(), template.GetPackages()...)
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

func (imageOs *ImageOs) initImageRpmDb(installRoot string, template *config.ImageTemplate) error {
	log.Infof("Initializing RPM database in %s", installRoot)
	rpmDbPath := filepath.Join(installRoot, "var", "lib", "rpm")
	if _, err := os.Stat(rpmDbPath); os.IsNotExist(err) {
		if _, err := shell.ExecCmd("mkdir -p "+rpmDbPath, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to create RPM database directory %s: %v", rpmDbPath, err)
			return fmt.Errorf("failed to create RPM database directory: %w", err)
		}
	}
	chrootInstallRoot, err := imageOs.chrootEnv.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path: %w", err)
	}
	cmd := fmt.Sprintf("rpm --root %s --initdb", chrootInstallRoot)
	chrootEnvRoot := imageOs.chrootEnv.GetChrootEnvRoot()
	if _, err := shell.ExecCmd(cmd, true, chrootEnvRoot, nil); err != nil {
		log.Errorf("Failed to initialize RPM database in %s: %v", chrootInstallRoot, err)
		return fmt.Errorf("failed to initialize RPM database: %w", err)
	}
	return nil
}

func (imageOs *ImageOs) initDebLocalRepoWithinInstallRoot(installRoot string) error {
	chrootInstallRoot, err := imageOs.chrootEnv.GetChrootEnvPath(installRoot)
	if err != nil {
		return fmt.Errorf("failed to get chroot environment path for install root %s: %w", installRoot, err)
	}

	// from local.list
	repoPath := filepath.Join(chrootInstallRoot, "/cdrom/cache-repo")
	chrootPkgCacheDir := imageOs.chrootEnv.GetChrootPkgCacheDir()
	if err := imageOs.chrootEnv.MountChrootPath(chrootPkgCacheDir, repoPath, "--bind"); err != nil {
		return fmt.Errorf("failed to mount package cache directory %s to chroot repo directory %s: %w",
			chrootPkgCacheDir, repoPath, err)
	}

	imageRepoCongfigPath := filepath.Join(installRoot, "/etc/apt/sources.list.d/", "*")
	if _, err := shell.ExecCmd("rm -f "+imageRepoCongfigPath, true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to remove existing local repo config files: %v", err)
		return fmt.Errorf("failed to remove existing local repo config files: %w", err)
	}

	repoCongfigPath := filepath.Join(imageOs.chrootEnv.GetTargetOsConfigDir(), "chrootenvconfigs", "local.list")
	if _, err := os.Stat(repoCongfigPath); os.IsNotExist(err) {
		log.Errorf("Repo config file does not exist: %s", repoCongfigPath)
		return fmt.Errorf("repo config file does not exist: %s", repoCongfigPath)
	}

	targetPath := filepath.Join(chrootInstallRoot, "/etc/apt/sources.list.d/")
	if err := imageOs.chrootEnv.CopyFileFromHostToChroot(repoCongfigPath, targetPath); err != nil {
		return fmt.Errorf("failed to copy local repository config file to chroot: %w", err)
	}

	cmd := "apt-get update"
	if _, err := shell.ExecCmdWithStream(cmd, true, installRoot, nil); err != nil {
		log.Errorf("Failed to refresh cache for chroot repository: %v", err)
		return fmt.Errorf("failed to refresh cache for chroot repository: %w", err)
	}

	// Create a policy-rc.d file to prevent service startup in chroot
	policyFile := filepath.Join(installRoot, "/usr/sbin/policy-rc.d")
	policyContent := "#!/bin/sh\nexit 101\n"

	if _, err := shell.ExecCmd("mkdir -p "+filepath.Dir(policyFile), true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to create policy-rc.d directory: %v", err)
		return fmt.Errorf("failed to create policy-rc.d directory: %w", err)
	}

	if err := file.Write(policyContent, policyFile); err != nil {
		log.Errorf("Failed to create policy-rc.d file %s: %v", policyFile, err)
		return fmt.Errorf("failed to create policy-rc.d file: %w", err)
	}

	return nil
}

func (imageOs *ImageOs) deInitDebLocalRepoWithinInstallRoot(installRoot string) error {
	// from local.list
	repoPath := filepath.Join(installRoot, "/cdrom/cache-repo")
	if err := imageOs.chrootEnv.UmountChrootPath(repoPath); err != nil {
		return fmt.Errorf("failed to unmount chroot repo directory %s: %w", repoPath, err)
	}

	repoconfigPath := filepath.Join(installRoot, "/etc/apt/sources.list.d/local.list")
	if _, err := os.Stat(repoconfigPath); err == nil {
		if _, err := shell.ExecCmd("rm -f "+repoconfigPath, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to remove local repository config file %s: %v", repoconfigPath, err)
			return fmt.Errorf("failed to remove local repository config file %s: %w", repoconfigPath, err)
		}
	}

	policyFile := filepath.Join(installRoot, "/usr/sbin/policy-rc.d")
	if _, err := os.Stat(policyFile); err == nil {
		if _, err := shell.ExecCmd("rm -f "+policyFile, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to remove policy-rc.d file %s: %v", policyFile, err)
			return fmt.Errorf("failed to remove policy-rc.d file %s: %w", policyFile, err)
		}
	}
	return nil
}

func preImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func (imageOs *ImageOs) installImagePkgs(installRoot string, template *config.ImageTemplate) error {
	pkgType := imageOs.chrootEnv.GetTargetOsPkgType()
	if pkgType == "rpm" {
		if err := imageOs.initImageRpmDb(installRoot, template); err != nil {
			return fmt.Errorf("failed to initialize RPM database: %w", err)
		}
		imagePkgOrderedList := getRpmPkgInstallList(template)
		imagePkgNum := len(imagePkgOrderedList)
		// Force to use the local cache repository
		var repositoryIDList []string = []string{"cache-repo"}
		for i, pkg := range imagePkgOrderedList {
			log.Infof("Installing package %d/%d: %s", i+1, imagePkgNum, pkg)
			if err := imageOs.chrootEnv.TdnfInstallPackage(pkg, installRoot, repositoryIDList); err != nil {
				return fmt.Errorf("failed to install package %s: %w", pkg, err)
			}
		}
	} else if pkgType == "deb" {
		imagePkgOrderedList := getDebPkgInstallList(template)
		// Prepare local cache repository
		if err := imageOs.initDebLocalRepoWithinInstallRoot(installRoot); err != nil {
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
						log.Errorf("Failed to install package %s: %v", pkg, err)
						return fmt.Errorf("failed to install package %s: %w", pkg, err)
					}
				}
			} else {
				if err := imageOs.chrootEnv.AptInstallPackage(pkg, installRoot, repoSrcList); err != nil {
					return fmt.Errorf("failed to install package %s: %w", pkg, err)
				}
			}
		}
		if err := imageOs.deInitDebLocalRepoWithinInstallRoot(installRoot); err != nil {
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
	if err := createResolvConfSymlink(installRoot, template); err != nil {
		return fmt.Errorf("failed to create resolv.conf: %w", err)
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
	if err := createResolvConfSymlink(installRoot, template); err != nil {
		return fmt.Errorf("failed to create resolv.conf: %w", err)
	}
	return nil
}

func (imageOs *ImageOs) getImageVersionInfo(installRoot string, template *config.ImageTemplate) (string, error) {
	var versionInfo string
	log.Infof("Getting image version info for: %s", template.GetImageName())

	switch template.Target.OS {
	case "azure-linux", "edge-microvisor-toolkit":
		imageVersionFilePath := filepath.Join(installRoot, "etc", "os-release")
		if _, err := os.Stat(imageVersionFilePath); os.IsNotExist(err) {
			log.Errorf("os-release file does not exist: %s", imageVersionFilePath)
			return "", fmt.Errorf("os-release file does not exist: %s", imageVersionFilePath)
		}
		content, err := file.Read(imageVersionFilePath)
		if err != nil {
			log.Errorf("Failed to read image version file %s: %v", imageVersionFilePath, err)
			return "", fmt.Errorf("failed to read image version file %s: %w", imageVersionFilePath, err)
		}
		// Parse the content to extract version information
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "VERSION=") {
				// Remove prefix, quotes and trim whitespace
				value := strings.TrimPrefix(line, "VERSION=")
				versionInfo = strings.TrimSpace(strings.Trim(value, "\""))
				break
			}
		}
		if versionInfo == "" {
			log.Debugf("Version info not found in %s", imageVersionFilePath)
		}
	default:
		versionInfo = imageOs.chrootEnv.GetTargetOsReleaseVersion()
	}

	log.Infof("Extracted version info: %s", versionInfo)

	return versionInfo, nil
}

func (imageOs *ImageOs) postImageOsInstall(installRoot string, template *config.ImageTemplate) (string, error) {
	versionInfo, err := imageOs.getImageVersionInfo(installRoot, template)
	if err != nil {
		return versionInfo, fmt.Errorf("failed to get image version info: %w", err)
	}
	return versionInfo, nil
}

func updateImageHostname(installRoot string, template *config.ImageTemplate) error {
	hostname := template.SystemConfig.HostName
	if hostname != "" {
		log.Infof("Configuring Hostname...")
		hostnameFilePath := filepath.Join(installRoot, "etc", "hostname")
		if err := file.Write(hostname+"\n", hostnameFilePath); err != nil {
			return fmt.Errorf("failed to write hostname to %s: %w", hostnameFilePath, err)
		}
		if _, err := shell.ExecCmd("chmod 0644 "+hostnameFilePath, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to set permissions for hostname file %s: %v", hostnameFilePath, err)
			return fmt.Errorf("failed to set permissions for hostname file %s: %w", hostnameFilePath, err)
		}
	}
	return nil
}

func updateImageUsrGroup(installRoot string, template *config.ImageTemplate) error {
	log.Infof("Configuring User...")
	if err := createUser(installRoot, template); err != nil {
		return fmt.Errorf("failed to configuring User: %w", err)
	}
	return nil
}

func updateImageNetwork(installRoot string, template *config.ImageTemplate) error {
	unitFilePath := filepath.Join(installRoot, "lib", "systemd", "system", "systemd-networkd.service")
	if _, err := os.Stat(unitFilePath); os.IsNotExist(err) {
		log.Warnf("systemd-networkd is not installed in %s, skipping enable", installRoot)
		return nil
	}
	cmd := "systemctl enable --root=\"" + installRoot + "\" systemd-networkd"
	if _, err := shell.ExecCmd(cmd, true, shell.HostPath, nil); err != nil {
		return fmt.Errorf("failed to enable systemd-networkd: %w", err)
	}
	return nil
}

func addImageIDFile(installRoot string, template *config.ImageTemplate) error {
	log.Infof("Adding image ID file for image: %s", template.GetImageName())
	imageIDFilePath := filepath.Join(installRoot, "etc", "image-id")
	// Get the current time in UTC and in format "YYYYMMDDHHMMSS"
	imageBuildDate := time.Now().UTC().Format("20060102150405")
	imageIDContent := fmt.Sprintf("IMAGE_BUILD_DATE=%s\nIMAGE_UUID=%s\n", imageBuildDate, uuid.New().String())
	if err := file.Write(imageIDContent, imageIDFilePath); err != nil {
		log.Errorf("Failed to write file %s: %v", imageIDFilePath, err)
		return fmt.Errorf("failed to write file %s: %w", imageIDFilePath, err)
	}
	if _, err := shell.ExecCmd("chmod 0444 "+imageIDFilePath, true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to set permissions for image ID file %s: %v", imageIDFilePath, err)
		return fmt.Errorf("failed to set permissions for image ID file %s: %w", imageIDFilePath, err)
	}
	return nil
}

func addImageAdditionalFiles(installRoot string, template *config.ImageTemplate) error {
	log.Infof("Adding additional files to image: %s", template.GetImageName())
	additionalFiles := template.GetAdditionalFileInfo()
	if len(additionalFiles) == 0 {
		log.Debug("No additional files to add to the image")
		return nil
	}

	for _, fileInfo := range additionalFiles {
		srcFile := fileInfo.Local
		dstFile := filepath.Join(installRoot, fileInfo.Final)
		if err := file.CopyFile(srcFile, dstFile, "-p", true); err != nil {
			log.Errorf("Failed to copy additional file %s to image: %v", srcFile, err)
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
					log.Errorf("Failed to append fstab entry for %s: %v", mountPoint, err)
					return fmt.Errorf("failed to append fstab entry for %s: %w", mountPoint, err)
				}
			}
		}
	}
	return nil
}

func createResolvConfSymlink(installRoot string, template *config.ImageTemplate) error {
	log.Infof("Creating resolv.conf for image: %s", template.GetImageName())
	resolveConfPath := "/etc/resolv.conf"
	resolveConfFullPath := filepath.Join(installRoot, resolveConfPath)
	if _, err := os.Stat(resolveConfFullPath); os.IsNotExist(err) {
		stubResolveConfPath := "/run/systemd/resolve/stub-resolv.conf"
		cmdStr := fmt.Sprintf("ln -sf %s %s", stubResolveConfPath, resolveConfPath)
		if _, err := shell.ExecCmd(cmdStr, true, installRoot, nil); err != nil {
			log.Errorf("Failed to create symlink for resolv.conf: %v", err)
			return fmt.Errorf("failed to create symlink for resolv.conf: %w", err)
		}
		log.Debugf("Created symlink for resolv.conf to %s", stubResolveConfPath)
	} else {
		log.Debug("resolv.conf already exists, skipping creation")
	}
	return nil
}

func buildImageUKI(installRoot string, template *config.ImageTemplate) error {
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

		log.Debug("Initramfs updated successfully")

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
		log.Debugf("Bootloader copied successfully to:", dstBootloader)
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
		log.Errorf("Failed to list kernel directory %s: %v", kernelDir, err)
		return "", fmt.Errorf("failed to list kernel directory %s: %w", kernelDir, err)
	}
	for _, f := range fileList {
		if strings.HasPrefix(f, "vmlinuz-") {
			return strings.TrimPrefix(f, "vmlinuz-"), nil
		}
	}
	log.Errorf("Kernel image not found in %s", kernelDir)
	return "", fmt.Errorf("kernel image not found in %s", kernelDir)
}

// Helper to update initramfs for the given kernel version
func updateInitramfs(installRoot, kernelVersion string, template *config.ImageTemplate) error {
	initrdPath := fmt.Sprintf("/boot/initramfs-%s.img", kernelVersion)

	// Build dracut command with all required options
	var cmdParts []string
	cmdParts = append(cmdParts, "dracut")
	cmdParts = append(cmdParts, "--force")
	cmdParts = append(cmdParts, "--no-hostonly")
	cmdParts = append(cmdParts, "--verbose")

	// Add systemd-veritysetup module if immutability is enabled
	if template.IsImmutabilityEnabled() {
		cmdParts = append(cmdParts, "--add", "systemd-veritysetup")
	}

	// Always add USB drivers
	extraModules := strings.TrimSpace(template.SystemConfig.Kernel.EnableExtraModules)
	if extraModules != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--add-drivers '%s'", extraModules))
	}

	// Add kernel version and output path
	cmdParts = append(cmdParts, "--kver", kernelVersion)
	cmdParts = append(cmdParts, initrdPath)

	// Execute single dracut command
	cmd := strings.Join(cmdParts, " ")
	_, err := shell.ExecCmd(cmd, true, installRoot, nil)
	if err != nil {
		if template.IsImmutabilityEnabled() {
			log.Errorf("Failed to update initramfs with veritysetup and USB drivers: %v", err)
			return fmt.Errorf("failed to update initramfs with veritysetup and USB drivers: %w", err)
		} else {
			log.Errorf("Failed to update initramfs with USB drivers: %v", err)
			return fmt.Errorf("failed to update initramfs with USB drivers: %w", err)
		}
	}

	if template.IsImmutabilityEnabled() {
		log.Debugf("Initramfs updated successfully with veritysetup and USB drivers")
	} else {
		log.Debugf("Initramfs updated successfully with USB drivers")
	}

	return nil
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
			log.Errorf("Failed to clean up ESP directory %s: %v", dir, err)
			return "", fmt.Errorf("failed to clean up ESP directory %s: %w", dir, err)
		}
	}

	// Create required ESP directories
	for _, dir := range espDirs {
		cmd := fmt.Sprintf("mkdir -p %s", dir)
		if _, err := shell.ExecCmd(cmd, true, installRoot, nil); err != nil {
			log.Errorf("Failed to create ESP directory %s: %v", dir, err)
			return "", fmt.Errorf("failed to create ESP directory %s: %w", dir, err)
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
		log.Errorf("Invalid partPair format: %s", partPair)
		return fmt.Errorf("invalid partPair: %s", partPair)
	}
	device := parts[0]

	// Remount the device as read-only
	remountCmd := fmt.Sprintf("mount -o remount,ro %s", device)
	log.Debugf("Remounting device as read-only: %s", remountCmd)
	if _, err := shell.ExecCmd(remountCmd, true, installRoot, nil); err != nil {
		log.Errorf("Failed to remount %s as read-only: %v", device, err)
		return fmt.Errorf("failed to remount %s as read-only: %w", device, err)
	}

	// Create and mount /tmp for ukify (Python tempfile needs this)
	tmpDir := filepath.Join(installRoot, "tmp")
	if _, err := shell.ExecCmd("mkdir -p "+tmpDir, true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to create /tmp directory: %v", err)
		return fmt.Errorf("failed to create /tmp directory: %w", err)
	}
	if _, err := shell.ExecCmd("mount -t tmpfs tmpfs /tmp", true, installRoot, nil); err != nil {
		log.Errorf("Failed to mount tmpfs on /tmp: %v", err)
		return fmt.Errorf("failed to mount tmpfs on /tmp: %w", err)
	}
	if _, err := shell.ExecCmd("chmod 1777 /tmp", true, installRoot, nil); err != nil {
		log.Errorf("Failed to chmod 1777 on /tmp: %v", err)
		return fmt.Errorf("failed to chmod 1777 on /tmp: %w", err)
	}

	// Create and mount /boot/efi/tmp for veritysetup
	veritytmpDir := filepath.Join(installRoot, "boot/efi/tmp")
	if _, err := shell.ExecCmd("mkdir -p "+veritytmpDir, true, shell.HostPath, nil); err != nil {
		log.Errorf("Failed to create /boot/efi/tmp directory: %v", err)
		return fmt.Errorf("failed to create /boot/efi/tmp directory: %w", err)
	}
	if _, err := shell.ExecCmd("mount -t tmpfs tmpfs /boot/efi/tmp", true, installRoot, nil); err != nil {
		log.Errorf("Failed to mount tmpfs on /boot/efi/tmp: %v", err)
		return fmt.Errorf("failed to mount tmpfs on /boot/efi/tmp: %w", err)
	}
	if _, err := shell.ExecCmd("chmod 1777 /boot/efi/tmp", true, installRoot, nil); err != nil {
		log.Errorf("Failed to chmod 1777 on /boot/efi/tmp: %v", err)
		return fmt.Errorf("failed to chmod 1777 on /boot/efi/tmp: %w", err)
	}
	return nil
}

func removeVerityTmp(installRoot string) {

	// Unmount and clean up /tmp
	tmpDir := filepath.Join(installRoot, "tmp")
	if _, err := shell.ExecCmd("umount /tmp", true, installRoot, nil); err != nil {
		log.Warnf("Failed to unmount tmpfs on /tmp: %v", err)
	}
	if _, err := shell.ExecCmd("rm -rf "+tmpDir, true, shell.HostPath, nil); err != nil {
		log.Warnf("Failed to remove tmp directory %s: %v", tmpDir, err)
	}

	// Unmount and clean up /boot/efi/tmp
	veritytmpDir := filepath.Join(installRoot, "boot/efi/tmp")
	if _, err := shell.ExecCmd("umount /boot/efi/tmp", true, installRoot, nil); err != nil {
		log.Warnf("Failed to unmount tmpfs on /boot/efi/tmp: %v", err)
	}

	if _, err := shell.ExecCmd("rm -rf "+veritytmpDir, true, shell.HostPath, nil); err != nil {
		log.Warnf("Failed to remove tmp directory %s: %v", veritytmpDir, err)
	}
}

func getVerityRootHash(partPair, installRoot string) (string, error) {
	cmd := fmt.Sprintf(`veritysetup format %s`, partPair)
	log.Debugf("Veritysetup Executing command:", cmd)
	// runs on host
	exists, _ := shell.IsCommandExist("ukify", installRoot)
	if !exists {
		log.Debugf("Ukify not found, running veritysetup on host")
		installRoot = shell.HostPath
	}
	output, err := shell.ExecCmd(cmd, true, installRoot, nil)
	if err != nil {
		log.Errorf("Failed to run veritysetup format: %v", err)
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
	log.Errorf("Root hash not found in veritysetup output")
	return "", fmt.Errorf("root hash not found in veritysetup output")
}

// Helper to build UKI using ukify
func buildUKI(installRoot, kernelPath, initrdPath, cmdlineFile, outputPath string, template *config.ImageTemplate) error {
	data, err := file.Read(filepath.Join(installRoot, cmdlineFile))
	if err != nil {
		log.Errorf("Failed to read cmdline file %s: %v", cmdlineFile, err)
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
		log.Debugf("Ukify not found, running ukify on host")
		kernelPath = filepath.Join(installRoot, kernelPath)
		initrdPath = filepath.Join(installRoot, initrdPath)
		outputPath = filepath.Join(installRoot, outputPath)
		osRelease := filepath.Join(installRoot, "/etc/os-release")
		installRoot = shell.HostPath

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

	log.Debugf("UKI Executing command:", cmd)
	if template.IsImmutabilityEnabled() {
		// Set TMPDIR environment variable to use the mounted tmpfs
		envVars := []string{"TMPDIR=/tmp"}
		_, err = shell.ExecCmd(cmd, true, installRoot, envVars)
		if err != nil {
			log.Errorf("Failed to build UKI with veritysetup: %v", err)
			err = fmt.Errorf("failed to build UKI with veritysetup: %w", err)
		}
		installRoot = backInstallRoot
		removeVerityTmp(installRoot)
	} else {
		_, err = shell.ExecCmd(cmd, true, installRoot, nil)
		if err != nil {
			log.Errorf("Failed to build UKI: %v", err)
			err = fmt.Errorf("failed to build UKI: %w", err)
		}
	}
	return err
}

// Helper to copy the bootloader EFI file
func copyBootloader(installRoot, src, dst string) error {
	// src and dst should be absolute paths inside the chroot
	// (e.g., /usr/lib/systemd/boot/efi/systemd-bootx64.efi
	// and /boot/efi/EFI/BOOT/BOOTX64.EFI)
	cmd := fmt.Sprintf("cp %s %s", src, dst)
	if _, err := shell.ExecCmd(cmd, true, installRoot, nil); err != nil {
		log.Errorf("Failed to copy bootloader from %s to %s: %v", src, dst, err)
		return fmt.Errorf("failed to copy bootloader from %s to %s: %w", src, dst, err)
	}
	return nil
}

// Verify that the user was created correctly
func verifyUserCreated(installRoot, username string) error {

	// Check if user exists in passwd file
	passwdCmd := fmt.Sprintf("grep '^%s:' /etc/passwd", username)
	output, err := shell.ExecCmd(passwdCmd, true, installRoot, nil)
	if err != nil {
		log.Errorf("User %s not found in passwd file: %v", username, err)
		return fmt.Errorf("user %s not found in passwd file: %w", username, err)
	}
	log.Debugf("User in passwd: %s", strings.TrimSpace(output))

	// Check if user has password in shadow file
	shadowCmd := fmt.Sprintf("grep '^%s:' /etc/shadow", username)
	output, err = shell.ExecCmd(shadowCmd, true, installRoot, nil)
	if err != nil {
		log.Errorf("User %s not found in shadow file: %v", username, err)
		return fmt.Errorf("user %s not found in shadow file: %w", username, err)
	}
	log.Debugf("User in shadow: %s", strings.TrimSpace(output))

	return nil
}

func createUser(installRoot string, template *config.ImageTemplate) error {
	// Check if there are any users to create
	if len(template.SystemConfig.Users) == 0 {
		log.Debug("No users defined in template, skipping user creation")
		return nil
	}

	// Loop through each user in the template configuration
	for _, user := range template.SystemConfig.Users {
		log.Infof("Creating user: %s", user.Name)

		// Create the user with useradd command
		// -m creates home directory, -s sets shell
		cmd := fmt.Sprintf("useradd -m -s /bin/bash %s", user.Name)
		output, err := shell.ExecCmdSilent(cmd, true, installRoot, nil)
		if err != nil {
			if strings.Contains(output, "already exists") {
				log.Warnf("User %s already exists", user.Name)
			} else {
				log.Errorf("Failed to create user %s: output: %s, err: %v", user.Name, output, err)
				return fmt.Errorf("failed to create user %s: output: %s, err: %w", user.Name, output, err)
			}
		}

		// Set password if provided
		if user.Password != "" {
			if err := setUserPassword(installRoot, user); err != nil {
				return fmt.Errorf("failed to set password for user %s: %w", user.Name, err)
			}
		} else {
			cmd := fmt.Sprintf("passwd -d %s", user.Name)
			if _, err := shell.ExecCmd(cmd, true, installRoot, nil); err != nil {
				log.Errorf("Failed to delete password for user %s: %v", user.Name, err)
				return fmt.Errorf("failed to delete password for user %s: %w", user.Name, err)
			}
			log.Debugf("Deleted password for user %s (no password set)", user.Name)
		}

		// Add user to groups if specified, filtering out template placeholders
		if len(user.Groups) > 0 {
			for _, group := range user.Groups {
				// Skip template placeholders and invalid group names
				if strings.HasPrefix(group, "<") && strings.HasSuffix(group, ">") {
					log.Debugf("Skipping template placeholder group: %s for user %s", group, user.Name)
					continue
				}
				if strings.TrimSpace(group) == "" {
					log.Debugf("Skipping empty group for user %s", user.Name)
					continue
				}

				groupCmd := fmt.Sprintf("usermod -aG %s %s", group, user.Name)
				if _, err := shell.ExecCmd(groupCmd, true, installRoot, nil); err != nil {
					log.Errorf("Failed to add user %s to group %s: %v", user.Name, group, err)
					return fmt.Errorf("failed to add user %s to group %s: %w", user.Name, group, err)
				}
				log.Debugf("Added user %s to group %s", user.Name, group)
			}
		}

		// Verify user creation
		if err := verifyUserCreated(installRoot, user.Name); err != nil {
			return fmt.Errorf("user verification failed for %s: %w", user.Name, err)
		}

		log.Infof("User %s created successfully", user.Name)
	}

	return nil
}

// Helper function to set user password based on hash algorithm
func setUserPassword(installRoot string, user config.UserConfig) error {
	// Check if password is already hashed or needs hashing
	if user.HashAlgo != "" {
		log.Debugf("Setting password with hash algorithm %s for user %s", user.HashAlgo, user.Name)

		// Check if password is already in hashed format (starts with $)
		if strings.HasPrefix(user.Password, "$") {
			// Password is already hashed, use usermod to set it directly
			usermodCmd := fmt.Sprintf("usermod -p '%s' %s", user.Password, user.Name)
			if _, err := shell.ExecCmd(usermodCmd, true, installRoot, nil); err != nil {
				log.Errorf("Failed to set hashed password for user %s: %v", user.Name, err)
				return fmt.Errorf("failed to set hashed password for user %s: %w", user.Name, err)
			}
		} else {
			// Password is plaintext, need to hash it first
			hashedPassword, err := hashPassword(user.Password, user.HashAlgo, installRoot)
			if err != nil {
				return fmt.Errorf("failed to hash password for user %s: %w", user.Name, err)
			}

			usermodCmd := fmt.Sprintf("usermod -p '%s' %s", hashedPassword, user.Name)
			if _, err := shell.ExecCmd(usermodCmd, true, installRoot, nil); err != nil {
				log.Errorf("Failed to set hashed password for user %s: %v", user.Name, err)
				return fmt.Errorf("failed to set hashed password for user %s: %w", user.Name, err)
			}
		}
	} else {
		// No hash algorithm specified, use interactive passwd command (legacy behavior)
		passwdInput := fmt.Sprintf("%s\n%s\n", user.Password, user.Password)
		passwdCmd := fmt.Sprintf("passwd %s", user.Name)
		if _, err := shell.ExecCmdWithInput(passwdInput, passwdCmd, true, installRoot, nil); err != nil {
			log.Errorf("Failed to set password for user %s: %v", user.Name, err)
			return fmt.Errorf("failed to set password for user %s: %w", user.Name, err)
		}
	}

	return nil
}

// Helper function to hash password using specified algorithm
func hashPassword(password, hashAlgo, installRoot string) (string, error) {
	var cmd string

	switch strings.ToLower(hashAlgo) {
	case "sha512":
		// Use openssl to generate SHA-512 hash
		cmd = fmt.Sprintf("openssl passwd -6 '%s'", password)
	case "sha256":
		// Use openssl to generate SHA-256 hash
		cmd = fmt.Sprintf("openssl passwd -5 '%s'", password)
	case "md5":
		// Use openssl to generate MD5 hash (not recommended for production)
		cmd = fmt.Sprintf("openssl passwd -1 '%s'", password)
	case "bcrypt":
		// Use python3 to generate bcrypt hash
		pythonScript := fmt.Sprintf("import bcrypt; print(bcrypt.hashpw(b'%s', bcrypt.gensalt()).decode())", password)
		cmd = fmt.Sprintf("python3 -c \"%s\"", pythonScript)
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", hashAlgo)
	}

	log.Debugf("Hashing password with algorithm %s", hashAlgo)
	output, err := shell.ExecCmd(cmd, true, installRoot, nil)
	if err != nil {
		log.Errorf("Failed to hash password with algorithm %s: %v", hashAlgo, err)
		return "", fmt.Errorf("failed to hash password with algorithm %s: %w", hashAlgo, err)
	}

	hashedPassword := strings.TrimSpace(output)
	log.Debugf("Password hashed successfully with algorithm %s", hashAlgo)

	return hashedPassword, nil
}
