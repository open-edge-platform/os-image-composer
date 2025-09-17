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
	"github.com/open-edge-platform/image-composer/internal/image/initrdmaker"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/mount"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"github.com/open-edge-platform/image-composer/internal/utils/system"
)

type IsoMakerInterface interface {
	Init(template *config.ImageTemplate) error
	BuildIsoImage(template *config.ImageTemplate) error
}

type IsoMaker struct {
	ImageBuildDir string
	ChrootEnv     chroot.ChrootEnvInterface
	ImageOs       imageos.ImageOsInterface
	InitrdMaker   initrdmaker.InitrdMakerInterface
}

var log = logger.Logger()

func NewIsoMaker(chrootEnv chroot.ChrootEnvInterface) (*IsoMaker, error) {
	return &IsoMaker{
		ChrootEnv: chrootEnv,
	}, nil
}

func (isoMaker *IsoMaker) Init(template *config.ImageTemplate) error {
	imageOs, err := imageos.NewImageOs(isoMaker.ChrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create image OS instance: %w", err)
	}
	isoMaker.ImageOs = imageOs

	initrdMaker, err := initrdmaker.NewInitrdMaker(isoMaker.ChrootEnv)
	if err != nil {
		return fmt.Errorf("failed to create initrd image maker instance: %w", err)
	}
	isoMaker.InitrdMaker = initrdMaker

	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %w", err)
	}

	providerId := system.GetProviderId(template.Target.OS, template.Target.Dist,
		template.Target.Arch)
	isoMaker.ImageBuildDir = filepath.Join(globalWorkDir, providerId, "imagebuild")
	if err := os.MkdirAll(isoMaker.ImageBuildDir, 0700); err != nil {
		log.Errorf("Failed to create imagebuild directory %s: %v", isoMaker.ImageBuildDir, err)
		return fmt.Errorf("failed to create imagebuild directory: %w", err)
	}

	return nil
}

func (isoMaker *IsoMaker) BuildIsoImage(template *config.ImageTemplate) (err error) {

	log.Infof("Building ISO image for: %s", template.GetImageName())

	if err := isoMaker.buildInitrd(template); err != nil {
		return fmt.Errorf("failed to build initrd image: %w", err)
	}
	defer func() {
		if cleanErr := isoMaker.InitrdMaker.CleanInitrdRootfs(); cleanErr != nil {
			err = fmt.Errorf("failed to clean initrd rootfs: %w", cleanErr)
		}
	}()

	sysConfigName := template.GetSystemConfigName()
	versionInfo := isoMaker.InitrdMaker.GetInitrdVersion()
	ImageName := fmt.Sprintf("%s-%s", template.GetImageName(), versionInfo)
	isoFileDir := filepath.Join(isoMaker.ImageBuildDir, sysConfigName)
	if err := os.MkdirAll(isoFileDir, 0700); err != nil {
		log.Errorf("Failed to create ISO image directory %s: %v", isoFileDir, err)
		return fmt.Errorf("failed to create ISO image directory: %w", err)
	}
	isoFilePath := filepath.Join(isoFileDir, fmt.Sprintf("%s.iso", ImageName))

	initrdRootfsPath := isoMaker.InitrdMaker.GetInitrdRootfsPath()
	initrdFilePath := isoMaker.InitrdMaker.GetInitrdFilePath()
	if err := isoMaker.createIso(template, initrdRootfsPath, initrdFilePath, isoFilePath); err != nil {
		return fmt.Errorf("failed to create ISO image: %w", err)
	}

	return nil
}

func (isoMaker *IsoMaker) buildInitrd(template *config.ImageTemplate) error {
	initrdTemplate, err := isoMaker.getInitrdTemplate(template)
	if err != nil {
		return fmt.Errorf("failed to get initrd template: %w", err)
	}

	if err := isoMaker.InitrdMaker.Init(initrdTemplate); err != nil {
		return fmt.Errorf("failed to initialize initrd maker: %w", err)
	}

	if err := isoMaker.InitrdMaker.DownloadInitrdPkgs(initrdTemplate); err != nil {
		return fmt.Errorf("failed to download initrd packages: %w", err)
	}

	if err := isoMaker.InitrdMaker.BuildInitrdImage(initrdTemplate); err != nil {
		return fmt.Errorf("failed to build initrd image: %w", err)
	}

	return nil
}

func (isoMaker *IsoMaker) getInitrdTemplate(template *config.ImageTemplate) (*config.ImageTemplate, error) {
	initrdTemplateFilePath, err := template.GetInitramfsTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to get initramfs template path: %w", err)
	}

	initrdTemplate, err := config.LoadAndMergeTemplate(initrdTemplateFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load and merge initrd template: %w", err)
	}

	return initrdTemplate, nil
}

func (isoMaker *IsoMaker) createIso(template *config.ImageTemplate, initrdRootfsPath, initrdFilePath, isoFilePath string) error {
	var err error

	log.Infof("Creating ISO image...")

	installRoot := isoMaker.ImageOs.GetInstallRoot()
	imageName := template.GetImageName()
	isoLabel := sanitizeIsoLabel(imageName)

	// Get the config file path to the static ISO root files
	generalConfigDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	staticIsoRootFilesDir := filepath.Join(generalConfigDir, "isolinux", template.Target.Arch)
	if _, err := os.Stat(staticIsoRootFilesDir); os.IsNotExist(err) {
		log.Errorf("Static ISO root files directory does not exist: %s", staticIsoRootFilesDir)
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
		if _, err := shell.ExecCmd("mkdir -p "+dir, true, "", nil); err != nil {
			log.Errorf("Failed to create directory %s: %v", dir, err)
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy ISOLINUX files
	if err := copyStaticFilesToIsolinuxPath(staticIsoRootFilesDir, isoIsolinuxPath); err != nil {
		return fmt.Errorf("failed to copy static files to isolinux path: %w", err)
	}

	// Create ISOLINUX config
	if err := createIsolinuxCfg(isoIsolinuxPath, imageName); err != nil {
		return fmt.Errorf("failed to create isolinux configuration: %w", err)
	}

	// Copy kernel and initrd
	if err := copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy kernel to isolinux path: %w", err)
	}

	if err := copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy initrd to isolinux path: %w", err)
	}

	// Copy EFI bootloader files
	if err := isoMaker.copyEfiBootloaderFiles(initrdRootfsPath, isoEfiPath); err != nil {
		return fmt.Errorf("failed to copy EFI bootloader files: %w", err)
	}

	// Create GRUB config for EFI boot
	if err := createGrubCfg(installRoot, imageName); err != nil {
		return fmt.Errorf("failed to create GRUB configuration: %w", err)
	}

	pkgType := isoMaker.ChrootEnv.GetTargetOsPkgType()
	switch pkgType {
	case "deb":
		// Create standalone grub efi
		if err := createGrubStandAlone(template, initrdRootfsPath, installRoot, isoEfiPath); err != nil {
			return fmt.Errorf("failed to create standalone GRUB: %w", err)
		}
	}

	efiFatImgPath, err := createEfiFatImage(isoEfiPath, isoImagesPath)
	if err != nil {
		return fmt.Errorf("failed to create EFI FAT image: %w", err)
	}
	efiFatImgRelPath := strings.TrimPrefix(efiFatImgPath, installRoot)

	// Check isolinux mbr file for hybrid ISO
	mbrFilePath := filepath.Join(staticIsoRootFilesDir, "isohdpfx.bin")
	if _, err := os.Stat(mbrFilePath); os.IsNotExist(err) {
		log.Errorf("ISOLINUX MBR file does not exist: %s", mbrFilePath)
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
		log.Errorf("Failed to create ISO image: %v", err)
		return fmt.Errorf("failed to create ISO image: %w", err)
	}

	if err := cleanIsoInstallRoot(installRoot); err != nil {
		return fmt.Errorf("failed to clean up ISO install root: %w", err)
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

	log.Infof("Copying static ISO root files...")

	for _, biosFile := range requiredBiosFiles {
		srcFilePath := filepath.Join(staticIsoRootFilesDir, biosFile)
		if _, err := os.Stat(srcFilePath); os.IsNotExist(err) {
			log.Errorf("Required BIOS boot file does not exist: %s", srcFilePath)
			return fmt.Errorf("required BIOS boot file does not exist: %s", srcFilePath)
		}
		destFilePath := filepath.Join(isoIsolinuxPath, biosFile)
		if err := file.CopyFile(srcFilePath, destFilePath, "--preserve=mode", true); err != nil {
			log.Errorf("Failed to copy file %s to %s: %v", srcFilePath, destFilePath, err)
			return fmt.Errorf("failed to copy file %s to %s: %w", srcFilePath, destFilePath, err)
		}
		log.Debugf("Copied %s to %s", srcFilePath, destFilePath)
	}

	return nil
}

func createIsolinuxCfg(isoIsolinuxPath, imageName string) error {
	log.Infof("Creating ISOLINUX configuration...")

	generalConfigDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	isolinuxCfgSrc := filepath.Join(generalConfigDir, "isolinux", "isolinux.cfg")
	if _, err := os.Stat(isolinuxCfgSrc); os.IsNotExist(err) {
		log.Errorf("isolinux.cfg file does not exist: %s", isolinuxCfgSrc)
		return fmt.Errorf("isolinux.cfg file does not exist: %s", isolinuxCfgSrc)
	}

	isolinuxCfgDest := filepath.Join(isoIsolinuxPath, "isolinux.cfg")
	if err := file.CopyFile(isolinuxCfgSrc, isolinuxCfgDest, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy isolinux.cfg to isolinux path: %v", err)
		return fmt.Errorf("failed to copy isolinux.cfg to isolinux path: %w", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.ImageName}}", imageName, isolinuxCfgDest); err != nil {
		log.Errorf("Failed to replace ImageName in isolinux configuration: %v", err)
		return fmt.Errorf("failed to replace ImageName in isolinux configuration: %w", err)
	}

	return nil
}

func copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath string) error {
	// Copy kernel to isolinux path
	var vmlinuzFileList []string
	cmdStr := "ls /boot | grep vmlinuz"
	output, err := shell.ExecCmd(cmdStr, true, initrdRootfsPath, nil)
	if err != nil {
		log.Errorf("Failed to list vmlinuz files in /boot: %v", err)
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
		log.Errorf("No vmlinuz files found in /boot")
		return fmt.Errorf("no vmlinuz files found in /boot")
	}

	kernelPath := filepath.Join(initrdRootfsPath, "boot", vmlinuzFileList[0])
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		log.Errorf("Kernel file does not exist: %s", kernelPath)
		return fmt.Errorf("kernel file does not exist: %s", kernelPath)
	}
	kernelDestPath := filepath.Join(isoImagesPath, "vmlinuz")
	if err := file.CopyFile(kernelPath, kernelDestPath, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy kernel to isolinux path: %v", err)
		return fmt.Errorf("failed to copy kernel to isolinux path: %w", err)
	}
	return nil
}

func copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath string) error {
	// Copy initrd image to isolinux path
	initrdDestPath := filepath.Join(isoImagesPath, "initrd.img")
	if err := file.CopyFile(initrdFilePath, initrdDestPath, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy initrd image to isolinux path: %v", err)
		return fmt.Errorf("failed to copy initrd image to isolinux path: %w", err)
	}
	return nil
}

func (isoMaker *IsoMaker) copyEfiBootloaderFiles(initrdRootfsPath, isoEfiPath string) error {
	log.Infof("Copying EFI bootloader files...")

	// Copy EFI bootloader files
	var efiBootFilesSrc string
	var efiGrubFilesSrc string
	pkgType := isoMaker.ChrootEnv.GetTargetOsPkgType()
	switch pkgType {
	case "rpm":
		efiGrubFilesSrc = filepath.Join(initrdRootfsPath, "/boot/efi/EFI/BOOT/grubx64.efi")
		efiBootFilesSrc = filepath.Join(initrdRootfsPath, "/boot/efi/EFI/BOOT/bootx64.efi")

		if _, err := os.Stat(efiBootFilesSrc); os.IsNotExist(err) {
			log.Errorf("EFI boot file does not exist: %s", efiBootFilesSrc)
			return fmt.Errorf("EFI boot file does not exist: %s", efiBootFilesSrc)
		}
		efiBootFilesDest := filepath.Join(isoEfiPath, "BOOTX64.EFI")
		if err := file.CopyFile(efiBootFilesSrc, efiBootFilesDest, "--preserve=mode", true); err != nil {
			log.Errorf("Failed to copy EFI bootloader files: %v", err)
			return fmt.Errorf("failed to copy EFI bootloader files: %w", err)
		}

	case "deb":
		efiGrubFilesSrc = filepath.Join(initrdRootfsPath, "/usr/lib/grub/x86_64-efi/monolithic/grubx64.efi")
	}

	if _, err := os.Stat(efiGrubFilesSrc); os.IsNotExist(err) {
		log.Errorf("EFI boot file does not exist: %s", efiGrubFilesSrc)
		return fmt.Errorf("EFI boot file does not exist: %s", efiGrubFilesSrc)
	}

	efiGrubFilesDest := filepath.Join(isoEfiPath, "grubx64.efi")
	if err := file.CopyFile(efiGrubFilesSrc, efiGrubFilesDest, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy EFI bootloader files: %v", err)
		return fmt.Errorf("failed to copy EFI bootloader files: %w", err)
	}
	return nil
}

func createGrubCfg(installRoot, imageName string) error {
	log.Infof("Creating GRUB configuration for EFI boot...")

	generalConfigDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	grubCfgSrc := filepath.Join(generalConfigDir, "isolinux", "grub.cfg")
	if _, err := os.Stat(grubCfgSrc); os.IsNotExist(err) {
		log.Errorf("grub.cfg file does not exist: %s", grubCfgSrc)
		return fmt.Errorf("grub.cfg file does not exist: %s", grubCfgSrc)
	}

	grubCfgDest := filepath.Join(installRoot, "boot", "grub2", "grub.cfg")
	if err := file.CopyFile(grubCfgSrc, grubCfgDest, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy grub.cfg to install root: %v", err)
		return fmt.Errorf("failed to copy grub.cfg to install root: %w", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.ImageName}}", imageName, grubCfgDest); err != nil {
		log.Errorf("Failed to replace ImageName in grub configuration: %v", err)
		return fmt.Errorf("failed to replace ImageName in grub configuration: %w", err)
	}

	grubCfgSrc = grubCfgDest
	grubCfgDest = filepath.Join(installRoot, "EFI", "BOOT", "grub.cfg")
	if err := file.CopyFile(grubCfgSrc, grubCfgDest, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy grub.cfg to install root: %v", err)
		return fmt.Errorf("failed to copy grub.cfg to install root: %w", err)
	}

	return nil
}

func createGrubStandAlone(template *config.ImageTemplate, initrdRootfsPath, installRoot, isoEfiPath string) error {
	log.Infof("Creating standalone GRUB for EFI boot...")

	target := template.GetTargetInfo()
	arch := target.Arch

	baseDir := filepath.Join(initrdRootfsPath, "boot", "efi", "EFI", "BOOT")
	efiBootFilesDest := filepath.Join(baseDir, "bootx64.efi")
	grubDir := filepath.Join(initrdRootfsPath, "usr", "lib", "grub", "x86_64-efi")
	grubCfgSrc := filepath.Join(installRoot, "EFI", "BOOT", "grub.cfg")
	grubModInfoSrc := filepath.Join(grubDir, "modinfo.sh")

	if _, err := shell.ExecCmd("mkdir -p "+baseDir, true, "", nil); err != nil {
		log.Errorf("Failed to create base dir %s: %v", baseDir, err)
		return fmt.Errorf("failed to create base dir %s: %w", baseDir, err)
	}

	if _, err := os.Stat(grubModInfoSrc); os.IsNotExist(err) {
		log.Errorf("Grub modinfo file does not exist: %s", grubModInfoSrc)
		return fmt.Errorf("grub modinfo file does not exist: %s", grubModInfoSrc)
	}

	if _, err := os.Stat(grubCfgSrc); os.IsNotExist(err) {
		log.Errorf("Grub cfg file does not exist: %s", grubCfgSrc)
		return fmt.Errorf("grub cfg file does not exist: %s", grubCfgSrc)
	}

	grubmkCmd := "grub-mkstandalone"
	grubmkCmd += fmt.Sprintf(" --directory=%s", grubDir)
	formatName, err := archToGrubFormat(arch)
	if err != nil {
		log.Errorf("Unsupported architecture for GRUB: %s", arch)
		return fmt.Errorf("unsupported architecture for GRUB: %s", arch)
	}
	grubmkCmd += fmt.Sprintf(" --format=%s-efi", formatName)
	grubmkCmd += fmt.Sprintf(" --output=%s", efiBootFilesDest)
	grubmkCmd += fmt.Sprintf(" \"boot/grub/grub.cfg=%s\"", grubCfgSrc)
	if _, err := shell.ExecCmd(grubmkCmd, true, "", nil); err != nil {
		log.Errorf("Failed to create standalone efi: %v", err)
		return fmt.Errorf("failed to create standalone efi: %w", err)
	}

	// check output
	if _, err := os.Stat(efiBootFilesDest); os.IsNotExist(err) {
		log.Errorf("EFI boot file does not exist: %s", efiBootFilesDest)
		return fmt.Errorf("EFI boot file does not exist: %s", efiBootFilesDest)
	}

	efiBootFilesFDest := filepath.Join(isoEfiPath, "BOOTX64.EFI")
	if err := file.CopyFile(efiBootFilesDest, efiBootFilesFDest, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy EFI bootloader files: %v", err)
		return fmt.Errorf("failed to copy EFI bootloader files: %w", err)
	}

	return nil
}

// archToGrubFormat maps a CPU architecture to its GRUB platform name
func archToGrubFormat(arch string) (string, error) {
	switch arch {
	case "x86_64":
		return "x86_64", nil
	case "i386":
		return "i386", nil
	case "arm64", "aarch64":
		return "arm64", nil
	case "arm":
		return "arm", nil
	case "riscv64":
		return "riscv64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

func createEfiFatImage(isoEfiPath, isoImagesPath string) (efiFatImgPath string, err error) {
	log.Infof("Creating EFI FAT image for UEFI boot...")
	efiFatImgPath = filepath.Join(isoImagesPath, "efiboot.img")
	// For the EFI FAT image, create it with sudo as the image rootfs path is owned by root.
	if err = imagedisc.CreateRawFile(efiFatImgPath, "18MiB", true); err != nil {
		return // Bare return - returns efiFatImgPath and err
	}

	cmdStr := fmt.Sprintf("mkfs -t vfat %s", efiFatImgPath)
	if _, err = shell.ExecCmd(cmdStr, true, "", nil); err != nil {
		log.Errorf("Failed to create FAT filesystem on EFI image: %v", err)
		return // Bare return - returns efiFatImgPath and err
	}

	// Create a temporary directory to mount the FAT image
	tempMountDir := filepath.Join(isoImagesPath, "efi_tmp")
	if err = mount.MountPath(efiFatImgPath, tempMountDir, "-o loop"); err != nil {
		log.Errorf("Failed to mount EFI FAT image: %v", err)
		return
	}

	defer func() {
		if umountErr := mount.UmountPath(tempMountDir); umountErr != nil {
			log.Errorf("Failed to unmount temporary mount directory %s: %v", tempMountDir, umountErr)
			if err == nil {
				err = fmt.Errorf("failed to unmount temporary mount directory %s: %w", tempMountDir, umountErr)
			} else {
				err = fmt.Errorf("operation failed: %w, cleanup errors: %v", err, umountErr)
			}
			return
		}
		if _, rmErr := shell.ExecCmd("rm -rf "+tempMountDir, true, "", nil); rmErr != nil {
			log.Errorf("Failed to remove temporary mount directory %s: %v", tempMountDir, rmErr)
			if err == nil {
				err = fmt.Errorf("failed to remove temporary mount directory %s: %w", tempMountDir, rmErr)
			} else {
				err = fmt.Errorf("operation failed: %w, cleanup errors: %v", err, rmErr)
			}
			return
		}
	}()

	// Copy the EFI bootloader to the FAT image
	efiBootDir := filepath.Join(tempMountDir, "EFI", "BOOT")
	if err = file.CopyDir(isoEfiPath, efiBootDir, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy EFI bootloader to FAT image: %v", err)
		return
	}

	// Sync to ensure all data is written to disk
	if _, err = shell.ExecCmd("sync", true, "", nil); err != nil {
		log.Errorf("Failed to sync temporary mount directory %s: %v", tempMountDir, err)
		return
	}

	return
}

func cleanIsoInstallRoot(installRoot string) error {
	log.Infof("Cleaning up ISO workspace: %s", installRoot)

	// Remove the entire image build directory
	if _, err := shell.ExecCmd("rm -rf "+installRoot, true, "", nil); err != nil {
		log.Errorf("Failed to remove iso installRoot directory %s: %v", installRoot, err)
		return fmt.Errorf("failed to remove iso installRoot directory %s: %w", installRoot, err)
	}

	return nil
}
