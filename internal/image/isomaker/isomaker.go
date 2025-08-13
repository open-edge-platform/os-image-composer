package isomaker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/image/imageos"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
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
	initrdFileDir := filepath.Join(ImageBuildDir, sysConfigName)
	if _, err := os.Stat(initrdFileDir); os.IsNotExist(err) {
		if err := os.MkdirAll(initrdFileDir, 0755); err != nil {
			return fmt.Errorf("failed to create initrd file directory: %w", err)
		}
	}
	initrdFilePath := filepath.Join(initrdFileDir, "iso-initrd.img")

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

	if err := addInitScriptsToInitrd(initrdRootfsPath); err != nil {
		return initrdRootfsPath, fmt.Errorf("failed to add init scripts to initrd: %v", err)
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
	// The initrd template does not conform to the full image schema
	initrdTemplate, err := config.LoadTemplate(initrdTemplateFile, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load initrd template: %v", err)
	}
	return initrdTemplate, nil
}

func downloadInitrdPkgs(initrdTemplate *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Downloading packages for: %s", initrdTemplate.GetImageName())

	pkgList := initrdTemplate.GetPackages()
	pkgType := chroot.GetTaRgetOsPkgType(config.TargetOs)
	if pkgType == "deb" {
		_, err := debutils.DownloadPackages(pkgList, chroot.ChrootPkgCacheDir, "")
		if err != nil {
			return fmt.Errorf("failed to download initrd packages: %v", err)
		}
	} else if pkgType == "rpm" {
		_, err := rpmutils.DownloadPackages(pkgList, chroot.ChrootPkgCacheDir, "")
		if err != nil {
			return fmt.Errorf("failed to download initrd packages: %v", err)
		}
	}

	if err := chroot.RefreshLocalCacheRepo(); err != nil {
		return fmt.Errorf("failed to refresh local cache repository: %w", err)
	}
	return nil
}

func addInitScriptsToInitrd(initrdRootfsPath string) error {
	log := logger.Logger()
	log.Infof("Adding init scripts to initrd...")

	generalConfigDir, err := file.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %v", err)
	}
	rcLocalSrc := filepath.Join(generalConfigDir, "isolinux", "rc.local")
	if _, err := os.Stat(rcLocalSrc); os.IsNotExist(err) {
		return fmt.Errorf("rc.local file does not exist: %s", rcLocalSrc)
	}

	rcLocalDest := filepath.Join(initrdRootfsPath, "etc", "rc.d", "rc.local")
	return file.CopyFile(rcLocalSrc, rcLocalDest, "--preserve=mode", true)
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
	installRoot, err := imageos.InitChrootInstallRoot(template)
	if err != nil {
		return fmt.Errorf("failed to initialize chroot install root: %w", err)
	}

	imageName := template.GetImageName()
	isoLabel := sanitizeIsoLabel(imageName)

	// Get the config file path to the static ISO root files
	generalConfigDir, err := file.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %v", err)
	}
	staticIsoRootFilesDir := filepath.Join(generalConfigDir, "isolinux", config.TargetArch)
	if _, err := os.Stat(staticIsoRootFilesDir); os.IsNotExist(err) {
		return fmt.Errorf("static ISO root files directory does not exist: %s", staticIsoRootFilesDir)
	}

	// Create standard ISO directory structure
	isoEfiPath := filepath.Join(installRoot, "EFI", "BOOT")
	isoImagesPath := filepath.Join(installRoot, "images")
	isoIsolinuxPath := filepath.Join(installRoot, "isolinux")

	dirs := []string{
		isoEfiPath,
		isoImagesPath,
		isoIsolinuxPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy ISOLINUX files
	if err := copyStaticFilesToIsolinuxPath(staticIsoRootFilesDir, isoIsolinuxPath); err != nil {
		return fmt.Errorf("failed to copy static files to isolinux path: %v", err)
	}

	// Create ISOLINUX config
	if err := createIsolinuxCfg(isoIsolinuxPath, imageName); err != nil {
		return fmt.Errorf("failed to create isolinux configuration: %v", err)
	}

	// Copy kernel and initrd
	if err := copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy kernel to isolinux path: %v", err)
	}

	if err := copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy initrd to isolinux path: %v", err)
	}

	// Copy EFI bootloader files
	if err := copyEfiBootloaderFiles(initrdRootfsPath, isoEfiPath); err != nil {
		return fmt.Errorf("failed to copy EFI bootloader files: %v", err)
	}

	// Create GRUB config for EFI boot
	if err := createGrubCfg(installRoot, imageName); err != nil {
		return fmt.Errorf("failed to create GRUB configuration: %v", err)
	}

	efiFatImgPath, err := createEfiFatImage(isoEfiPath, isoImagesPath)
	if err != nil {
		return fmt.Errorf("failed to create EFI FAT image: %v", err)
	}
	efiFatImgRelPath := strings.TrimPrefix(efiFatImgPath, installRoot)

	// Check isolinux mbr file for hybrid ISO
	mbrFilePath := filepath.Join(staticIsoRootFilesDir, "isohdpfx.bin")
	if _, err := os.Stat(mbrFilePath); os.IsNotExist(err) {
		return fmt.Errorf("ISOLINUX MBR file does not exist: %s", mbrFilePath)
	}

	// Create ISO image
	xorrisoCmd := fmt.Sprintf("xorriso -as mkisofs -isohybrid-mbr %s", mbrFilePath)
	xorrisoCmd += " -c isolinux/boot.cat -b isolinux/isolinux.bin"
	xorrisoCmd += " -no-emul-boot -boot-load-size 4 -boot-info-table"
	xorrisoCmd += fmt.Sprintf(" -eltorito-alt-boot -e %s", efiFatImgRelPath)
	xorrisoCmd += " -no-emul-boot -isohybrid-gpt-basdat"
	xorrisoCmd += fmt.Sprintf(" -volid \"%s\" -o \"%s\" \"%s\"",
		isoLabel, isoFilePath, installRoot)

	if _, err := shell.ExecCmdWithStream(xorrisoCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to create ISO image: %w", err)
	}

	if err := cleanInitrd(initrdRootfsPath, initrdFilePath); err != nil {
		return fmt.Errorf("failed to clean up initrd rootfs: %v", err)
	}

	if err := cleanIsoInstallRoot(installRoot); err != nil {
		return fmt.Errorf("failed to clean up ISO install root: %v", err)
	}

	return nil
}

// sanitizeIsoLabel ensures the IsoLabel complies with ISO 9660 rules
// 1. Maximum length of 32 characters
// 2. Can only contain uppercase letters A-Z, digits 0-9, and underscore (_)
// 3. No spaces or other special characters are allowed
func sanitizeIsoLabel(isoLabel string) string {
	// Limit to 32 characters
	if len(isoLabel) > 32 {
		isoLabel = isoLabel[:32]
	}

	// Replace invalid characters with underscores
	result := ""
	for _, char := range isoLabel {
		if (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' {
			// Character is already valid
			result += string(char)
		} else if char >= 'a' && char <= 'z' {
			// Convert lowercase to uppercase
			result += string(char - 32)
		} else {
			// Replace invalid character with underscore
			result += "_"
		}
	}

	return result
}

func copyStaticFilesToIsolinuxPath(staticIsoRootFilesDir, isoIsolinuxPath string) error {
	// Copy static ISO root files
	// These ISOLINUX bootloader files are part of the "syslinux" package,
	// which is required for creating bootable ISOs
	// After installlation, the files should be available in the following locations:
	// Locations (depending on distribution):
	//     /usr/lib/syslinux/modules/bios
	//     /usr/lib/ISOLINUX/

	// Required BIOS boot files
	requiredBiosFiles := []string{
		"isolinux.bin", // ISOLINUX bootloader binary
		"ldlinux.c32",
		"libcom32.c32",
		"libutil.c32",
		"vesamenu.c32",
		"menu.c32",
		"linux.c32",   // For booting Linux kernels
		"libmenu.c32", // Required by vesamenu.c32
	}

	log := logger.Logger()
	log.Infof("Copying static ISO root files...")

	for _, biosFile := range requiredBiosFiles {
		srcFilePath := filepath.Join(staticIsoRootFilesDir, biosFile)
		if _, err := os.Stat(srcFilePath); os.IsNotExist(err) {
			return fmt.Errorf("required BIOS boot file does not exist: %s", srcFilePath)
		}
		destFilePath := filepath.Join(isoIsolinuxPath, biosFile)
		if err := file.CopyFile(srcFilePath, destFilePath, "--preserve=mode", true); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %v", srcFilePath, destFilePath, err)
		}
		log.Debugf("Copied %s to %s", srcFilePath, destFilePath)
	}

	return nil
}

func createIsolinuxCfg(isoIsolinuxPath, imageName string) error {
	log := logger.Logger()
	log.Infof("Creating ISOLINUX configuration...")

	generalConfigDir, err := file.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %v", err)
	}
	isolinuxCfgSrc := filepath.Join(generalConfigDir, "isolinux", "isolinux.cfg")
	if _, err := os.Stat(isolinuxCfgSrc); os.IsNotExist(err) {
		return fmt.Errorf("isolinux.cfg file does not exist: %s", isolinuxCfgSrc)
	}

	isolinuxCfgDest := filepath.Join(isoIsolinuxPath, "isolinux.cfg")
	if err := file.CopyFile(isolinuxCfgSrc, isolinuxCfgDest, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy isolinux.cfg to isolinux path: %v", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.ImageName}}", imageName, isolinuxCfgDest); err != nil {
		return fmt.Errorf("failed to replace ImageName in grub configuration: %w", err)
	}

	return nil
}

func copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath string) error {
	// Copy kernel to isolinux path
	var vmlinuzFileList []string
	cmdStr := "ls /boot | grep vmlinuz"
	output, err := shell.ExecCmd(cmdStr, true, initrdRootfsPath, nil)
	if err != nil {
		return fmt.Errorf("failed to list vmlinuz files in /boot: %w", err)
	}
	for _, line := range strings.Split(output, "\n") {
		vmlinuzFile := strings.TrimSpace(line)
		if vmlinuzFile == "" {
			continue
		}
		if strings.HasPrefix(vmlinuzFile, "vmlinuz") {
			vmlinuzFileList = append(vmlinuzFileList, vmlinuzFile)
		}
	}

	if len(vmlinuzFileList) == 0 {
		return fmt.Errorf("no vmlinuz files found in /boot")
	}

	kernelPath := filepath.Join(initrdRootfsPath, "boot", vmlinuzFileList[0])
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		return fmt.Errorf("kernel file does not exist: %s", kernelPath)
	}
	kernelDestPath := filepath.Join(isoImagesPath, "vmlinuz")
	if err := file.CopyFile(kernelPath, kernelDestPath, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy kernel to isolinux path: %v", err)
	}
	return nil
}

func copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath string) error {
	// Copy initrd image to isolinux path
	initrdDestPath := filepath.Join(isoImagesPath, "initrd.img")
	if err := file.CopyFile(initrdFilePath, initrdDestPath, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy initrd image to isolinux path: %v", err)
	}
	return nil
}

func copyEfiBootloaderFiles(initrdRootfsPath, isoEfiPath string) error {
	log := logger.Logger()
	log.Infof("Copying EFI bootloader files...")

	// Copy EFI bootloader files
	var efiBootFilesSrc string
	var efiGrubFilesSrc string
	pkgType := chroot.GetTaRgetOsPkgType(config.TargetOs)
	switch pkgType {
	case "rpm":
		efiGrubFilesSrc = filepath.Join(initrdRootfsPath, "/boot/efi/EFI/BOOT/grubx64.efi")
		efiBootFilesSrc = filepath.Join(initrdRootfsPath, "/boot/efi/EFI/BOOT/bootx64.efi")
	case "deb":
		efiBootFilesSrc = filepath.Join(initrdRootfsPath, "/usr/lib/grub/x86_64-efi/monolithic/grubx64.efi")
		efiGrubFilesSrc = filepath.Join(initrdRootfsPath, "/usr/lib/grub/x86_64-efi/monolithic/grubx64.efi")
	}

	if _, err := os.Stat(efiBootFilesSrc); os.IsNotExist(err) {
		return fmt.Errorf("EFI boot file does not exist: %s", efiBootFilesSrc)
	}

	efiBootFilesDest := filepath.Join(isoEfiPath, "BOOTX64.EFI")
	if err := file.CopyFile(efiBootFilesSrc, efiBootFilesDest, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy EFI bootloader files: %v", err)
	}

	if _, err := os.Stat(efiGrubFilesSrc); os.IsNotExist(err) {
		return fmt.Errorf("EFI boot file does not exist: %s", efiGrubFilesSrc)
	}

	efiGrubFilesDest := filepath.Join(isoEfiPath, "grubx64.efi")
	if err := file.CopyFile(efiGrubFilesSrc, efiGrubFilesDest, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy EFI bootloader files: %v", err)
	}
	return nil
}

func createGrubCfg(installRoot, imageName string) error {
	log := logger.Logger()
	log.Infof("Creating GRUB configuration for EFI boot...")

	generalConfigDir, err := file.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %v", err)
	}
	grubCfgSrc := filepath.Join(generalConfigDir, "isolinux", "grub.cfg")
	if _, err := os.Stat(grubCfgSrc); os.IsNotExist(err) {
		return fmt.Errorf("grub.cfg file does not exist: %s", grubCfgSrc)
	}

	grubCfgDest := filepath.Join(installRoot, "boot", "grub2", "grub.cfg")
	if err := file.CopyFile(grubCfgSrc, grubCfgDest, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy grub.cfg to install root: %v", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.ImageName}}", imageName, grubCfgDest); err != nil {
		return fmt.Errorf("failed to replace ImageName in grub configuration: %w", err)
	}

	grubCfgSrc = grubCfgDest
	grubCfgDest = filepath.Join(installRoot, "EFI", "BOOT", "grub.cfg")
	if err := file.CopyFile(grubCfgSrc, grubCfgDest, "--preserve=mode", true); err != nil {
		return fmt.Errorf("failed to copy grub.cfg to install root: %v", err)
	}

	return nil
}

func createEfiFatImage(isoEfiPath, isoImagesPath string) (string, error) {
	var err error
	log := logger.Logger()
	log.Infof("Creating EFI FAT image for UEFI boot...")
	efiFatImgPath := filepath.Join(isoImagesPath, "efiboot.img")
	if err := imagedisc.CreateRawFile(efiFatImgPath, "9MiB"); err != nil {
		return "", fmt.Errorf("failed to create EFI FAT image: %v", err)
	}

	cmdStr := fmt.Sprintf("mkfs -t vfat %s", efiFatImgPath)
	_, err = shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create FAT filesystem on EFI image: %v", err)
	}

	// Create a temporary directory to mount the FAT image
	tempMountDir := filepath.Join(isoImagesPath, "efi_tmp")
	if err := mount.MountPath(efiFatImgPath, tempMountDir, "-o loop"); err != nil {
		return "", fmt.Errorf("failed to mount EFI FAT image: %v", err)
	}
	defer os.RemoveAll(tempMountDir)

	// Copy the EFI bootloader to the FAT image
	efiBootDir := filepath.Join(tempMountDir, "EFI", "BOOT")
	if err = file.CopyDir(isoEfiPath, efiBootDir, "--preserve=mode", true); err != nil {
		err = fmt.Errorf("failed to copy EFI bootloader to FAT image: %w", err)
		goto fail
	}

	// Sync to ensure all data is written to disk
	if _, err = shell.ExecCmd("sync", true, "", nil); err != nil {
		err = fmt.Errorf("failed to sync temporary mount directory %s: %v", tempMountDir, err)
		goto fail
	}

	// Unmount the FAT image
	if err = mount.UmountPath(tempMountDir); err != nil {
		return "", fmt.Errorf("failed to unmount temporary mount directory %s: %v", tempMountDir, err)
	}

	return efiFatImgPath, nil

fail:
	if umountErr := mount.UmountPath(tempMountDir); umountErr != nil {
		log.Errorf("Failed to unmount temporary mount directory %s: %v", tempMountDir, umountErr)
	}
	if err := os.Remove(efiFatImgPath); err != nil {
		log.Errorf("Failed to remove EFI FAT image %s: %v", efiFatImgPath, err)
	}
	return "", err
}

func cleanInitrd(initrdRootfsPath, initrdFilePath string) error {
	log := logger.Logger()
	log.Infof("Cleaning up initrd rootfs: %s", initrdRootfsPath)

	if err := mount.UmountPath(initrdRootfsPath + "/cdrom/cache-repo"); err != nil {
		return fmt.Errorf("failed to unmount cache-repo %s: %v", initrdRootfsPath+"/cdrom/cache-repo", err)
	}

	// Remove the initrd rootfs directory
	if _, err := shell.ExecCmd("rm -rf "+initrdRootfsPath, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove initrd rootfs directory: %v", err)
	}

	log.Infof("Removing initrd image file: %s", initrdFilePath)
	if _, err := shell.ExecCmd("rm -f "+initrdFilePath, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove initrd image file: %v", err)
	}
	return nil
}

func cleanIsoInstallRoot(installRoot string) error {
	log := logger.Logger()
	log.Infof("Cleaning up ISO workspace: %s", installRoot)

	// Remove the entire image build directory
	if _, err := shell.ExecCmd("rm -rf "+installRoot, true, "", nil); err != nil {
		return fmt.Errorf("failed to remove iso installRoot directory: %v", err)
	}

	return nil
}
