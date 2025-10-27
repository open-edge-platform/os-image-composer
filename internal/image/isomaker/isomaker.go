package isomaker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/chroot"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/imageos"
	"github.com/open-edge-platform/os-image-composer/internal/image/initrdmaker"
	"github.com/open-edge-platform/os-image-composer/internal/utils/file"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
	"github.com/open-edge-platform/os-image-composer/internal/utils/system"
)

type IsoMakerInterface interface {
	Init() error          // Initialize with stored template
	BuildIsoImage() error // Build ISO image using stored template
}

type IsoMaker struct {
	template      *config.ImageTemplate
	ImageBuildDir string
	ChrootEnv     chroot.ChrootEnvInterface
	ImageOs       imageos.ImageOsInterface
	InitrdMaker   initrdmaker.InitrdMakerInterface
}

var log = logger.Logger()

func NewIsoMaker(chrootEnv chroot.ChrootEnvInterface, template *config.ImageTemplate) (*IsoMaker, error) {
	// nil checking is done one in constructor only to avoid repetitive checks
	// in every method and schema check is done during template load making
	// sure internal structure is valid
	if template == nil {
		return nil, fmt.Errorf("image template cannot be nil")
	}
	if chrootEnv == nil {
		return nil, fmt.Errorf("chroot environment cannot be nil")
	}

	// Create ImageOs with template
	imageOs, err := imageos.NewImageOs(chrootEnv, template)
	if err != nil {
		return nil, fmt.Errorf("failed to create image OS: %w", err)
	}

	return &IsoMaker{
		template:  template,
		ChrootEnv: chrootEnv,
		ImageOs:   imageOs,
	}, nil
}

func (isoMaker *IsoMaker) Init() error {

	globalWorkDir, err := config.WorkDir()
	if err != nil {
		return fmt.Errorf("failed to get global work directory: %w", err)
	}

	providerId := system.GetProviderId(
		isoMaker.template.Target.OS,
		isoMaker.template.Target.Dist,
		isoMaker.template.Target.Arch,
	)

	isoMaker.ImageBuildDir = filepath.Join(globalWorkDir,
		providerId,
		"imagebuild",
		isoMaker.template.GetSystemConfigName(),
	)

	return os.MkdirAll(isoMaker.ImageBuildDir, 0700)
}

func (isoMaker *IsoMaker) BuildIsoImage() (err error) {

	log.Infof("Building ISO image for: %s", isoMaker.template.GetImageName())

	if err := isoMaker.buildInitrd(isoMaker.template); err != nil {
		return fmt.Errorf("failed to build initrd image: %w", err)
	}
	defer func() {
		if cleanErr := isoMaker.InitrdMaker.CleanInitrdRootfs(); cleanErr != nil {
			err = fmt.Errorf("failed to clean initrd rootfs: %w", cleanErr)
		}
	}()

	versionInfo := isoMaker.InitrdMaker.GetInitrdVersion()
	ImageName := fmt.Sprintf("%s-%s", isoMaker.template.GetImageName(), versionInfo)
	isoFilePath := filepath.Join(isoMaker.ImageBuildDir, fmt.Sprintf("%s.iso", ImageName))

	initrdRootfsPath := isoMaker.InitrdMaker.GetInitrdRootfsPath()
	initrdFilePath := isoMaker.InitrdMaker.GetInitrdFilePath()
	if err := isoMaker.createIso(isoMaker.template, initrdRootfsPath, initrdFilePath, isoFilePath); err != nil {
		return fmt.Errorf("failed to create ISO image: %w", err)
	}

	return nil
}

func (isoMaker *IsoMaker) buildInitrd(template *config.ImageTemplate) error {
	initrdTemplate, err := isoMaker.getInitrdTemplate(template)
	if err != nil {
		return fmt.Errorf("failed to get initrd template: %w", err)
	}

	isoMaker.InitrdMaker, err = initrdmaker.NewInitrdMaker(isoMaker.ChrootEnv, initrdTemplate)
	if err != nil {
		return fmt.Errorf("failed to create initrd maker: %w", err)
	}

	if err := isoMaker.InitrdMaker.Init(); err != nil {
		return fmt.Errorf("failed to initialize initrd maker: %w", err)
	}

	if err := isoMaker.InitrdMaker.DownloadInitrdPkgs(); err != nil {
		return fmt.Errorf("failed to download initrd packages: %w", err)
	}

	if err := isoMaker.InitrdMaker.BuildInitrdImage(); err != nil {
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

	log.Infof("Creating ISO image: %s", isoFilePath)

	// Create standard ISO directory structure
	isoBootPath := filepath.Join(installRoot, "boot")
	isoEfiPath := filepath.Join(installRoot, "EFI", "BOOT")
	isoImagesPath := filepath.Join(installRoot, "images")

	dirs := []string{
		isoBootPath,
		isoEfiPath,
		isoImagesPath,
	}

	log.Infof("Creating ISO directory structure...")
	for _, dir := range dirs {
		if _, err := shell.ExecCmd("mkdir -p "+dir, true, "", nil); err != nil {
			log.Errorf("Failed to create directory %s: %v", dir, err)
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Copy kernel and initrd
	log.Infof("Copying kernel and initrd files...")
	if err := copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy kernel to iso image path: %w", err)
	}

	if err := copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath); err != nil {
		return fmt.Errorf("failed to copy initrd to iso image path: %w", err)
	}

	// Create GRUB config for EFI boot
	if err := createGrubCfg(installRoot, imageName); err != nil {
		return fmt.Errorf("failed to create GRUB configuration: %w", err)
	}

	// Copy GRUB files to ISO boot path
	log.Infof("Copying GRUB files to ISO boot path...")
	if err := copyGrubFilesToGrubPath(initrdRootfsPath, installRoot); err != nil {
		return fmt.Errorf("failed to copy GRUB files to ISO boot path: %w", err)
	}

	log.Infof("Creating EFI FAT image...")
	efiFatImgPath, err := createEfiFatImage(template, initrdRootfsPath, installRoot)
	if err != nil {
		return fmt.Errorf("failed to create EFI FAT image: %w", err)
	}
	efiFatImgRelPath := strings.TrimPrefix(efiFatImgPath, installRoot)

	log.Infof("Creating image for Bios boot...")
	biosImgRelPath, err := createBiosImage(template, initrdRootfsPath, installRoot)
	if err != nil {
		return fmt.Errorf("failed to create BIOS image: %w", err)
	}

	// Create ISO image with xorriso
	log.Infof("Creating ISO image with xorriso...")
	var xorrisoCmd string
	if biosImgRelPath != "" {
		// Support both BIOS and UEFI boot mode
		log.Infof("Creating hybrid ISO for both BIOS and UEFI boot modes...")
		biosImgRelDir := filepath.Dir(biosImgRelPath)
		xorrisoCmd = fmt.Sprintf("xorriso -as mkisofs -graft-points -r -J -l -b %s", biosImgRelPath)
		xorrisoCmd += " -no-emul-boot -boot-load-size 4 -boot-info-table --grub2-boot-info"
		xorrisoCmd += fmt.Sprintf(" --grub2-mbr %s", filepath.Join(installRoot, biosImgRelDir, "boot_hybrid.img"))
		xorrisoCmd += fmt.Sprintf(" -eltorito-alt-boot -e %s -no-emul-boot", efiFatImgRelPath)
		xorrisoCmd += fmt.Sprintf(" -append_partition 2 0xef %s -appended_part_as_gpt", efiFatImgPath)
		xorrisoCmd += fmt.Sprintf(" -r %s --sort-weight 0 / --sort-weight 1 /boot", installRoot)
		xorrisoCmd += fmt.Sprintf(" -volid \"%s\" --protective-msdos-label -o \"%s\" \"%s\"",
			isoLabel, isoFilePath, installRoot)
	} else {
		// Support only UEFI boot mode
		log.Infof("Creating ISO for UEFI boot mode only...")
		xorrisoCmd = fmt.Sprintf("xorriso -as mkisofs -graft-points -r -J -l --efi-boot %s", efiFatImgPath)
		xorrisoCmd += " -efi-boot-part --efi-boot-image --protective-msdos-label"
		xorrisoCmd += fmt.Sprintf(" -r %s --sort-weight 0 / --sort-weight 1 /boot", installRoot)
		xorrisoCmd += fmt.Sprintf(" -volid \"%s\" -o \"%s\" \"%s\"",
			isoLabel, isoFilePath, installRoot)
	}

	if _, err := shell.ExecCmdWithStream(xorrisoCmd, true, "", nil); err != nil {
		log.Errorf("Failed to create ISO image: %v", err)
		return fmt.Errorf("failed to create ISO image: %w", err)
	}

	if err := cleanIsoInstallRoot(installRoot); err != nil {
		return fmt.Errorf("failed to clean up ISO install root: %w", err)
	}

	log.Infof("ISO creation completed successfully")
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

func copyKernelToIsoImagesPath(initrdRootfsPath, isoImagesPath string) error {
	// Copy kernel to iso image path
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
		log.Errorf("Failed to copy kernel to iso image path: %v", err)
		return fmt.Errorf("failed to copy kernel to iso image path: %w", err)
	}
	return nil
}

func copyInitrdToIsoImagesPath(initrdFilePath, isoImagesPath string) error {
	// Copy initrd image to iso image path
	initrdDestPath := filepath.Join(isoImagesPath, "initrd.img")
	if err := file.CopyFile(initrdFilePath, initrdDestPath, "--preserve=mode", true); err != nil {
		log.Errorf("Failed to copy initrd image to iso image path: %v", err)
		return fmt.Errorf("failed to copy initrd image to iso image path: %w", err)
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

	grubCfgDest := filepath.Join(installRoot, "boot", "grub", "grub.cfg")
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

func copyGrubFilesToGrubPath(initrdRootfsPath, installRoot string) error {

	// Copy GRUB locale files if exists
	localSrcDir := filepath.Join(initrdRootfsPath, "usr", "share", "locale")
	localLangpackDir := filepath.Join(initrdRootfsPath, "usr", "share", "locale-langpack")
	for _, dir := range []string{localSrcDir, localLangpackDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Warnf("GRUB locale directory does not exist: %s", dir)
		} else {
			// traverse locale directory and copy only language directories
			entries, err := os.ReadDir(dir)
			if err != nil {
				log.Errorf("Failed to read GRUB locale directory: %v", err)
				return fmt.Errorf("failed to read GRUB locale directory: %w", err)
			}
			for _, entry := range entries {
				if entry.IsDir() {
					langDir := entry.Name()
					langSrc := filepath.Join(dir, langDir, "LC_MESSAGES", "grub.mo")
					if _, err := os.Stat(langSrc); os.IsNotExist(err) {
						continue
					}
					langDest := filepath.Join(installRoot, "boot", "grub", "locale", fmt.Sprintf("%s.mo", langDir))
					if err := file.CopyFile(langSrc, langDest, "--preserve=mode", true); err != nil {
						log.Errorf("Failed to copy GRUB locale file to iso boot dir: %v", err)
						return fmt.Errorf("failed to copy GRUB locale file to iso boot dir: %w", err)
					}
				}
			}
		}
	}

	// Copy GRUB font file if exists
	fontsSrc := filepath.Join(initrdRootfsPath, "usr", "share", "grub", "unicode.pf2")
	if _, err := os.Stat(fontsSrc); os.IsNotExist(err) {
		log.Warnf("GRUB font file does not exist: %s", fontsSrc)
	} else {
		fontsDest := filepath.Join(installRoot, "boot", "grub", "fonts", "unicode.pf2")
		if err := file.CopyFile(fontsSrc, fontsDest, "--preserve=mode", true); err != nil {
			log.Errorf("Failed to copy GRUB font file to iso boot dir: %v", err)
			return fmt.Errorf("failed to copy GRUB font file to iso boot dir: %w", err)
		}
	}

	return nil
}

func createBiosImage(template *config.ImageTemplate, initrdRootfsPath, installRoot string) (biosImgRelPath string, err error) {
	target := template.GetTargetInfo()
	switch target.Arch {
	case "x86_64", "i386":
		format := "i386-pc"
		prefixDir := "/boot/grub/"

		generalConfigDir, err := config.GetGeneralConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to get general config directory: %w", err)
		}
		loadCfgSrc := filepath.Join(generalConfigDir, "isolinux", "load.cfg")
		if _, err := os.Stat(loadCfgSrc); os.IsNotExist(err) {
			log.Errorf("load.cfg file does not exist: %s", loadCfgSrc)
			return "", fmt.Errorf("load.cfg file does not exist: %s", loadCfgSrc)
		}

		grubLibDir := filepath.Join(initrdRootfsPath, "usr", "lib", "grub", format)
		if _, err := os.Stat(grubLibDir); os.IsNotExist(err) {
			log.Debugf("GRUB modules directory does not exist: %s, skip BIOS boot enabling", grubLibDir)
			return "", nil
		}

		bootGrubLibDir := filepath.Join(installRoot, "boot", "grub", format)
		if err = file.CopyDir(grubLibDir, bootGrubLibDir, "--preserve=mode", true); err != nil {
			log.Errorf("Failed to copy grub modules to iso boot dir: %v", err)
			return "", fmt.Errorf("failed to copy grub modules to iso boot dir: %w", err)
		}

		biosImgRelPath = filepath.Join("boot", "grub", format, "eltorito.img")
		biosImgPath := filepath.Join(installRoot, biosImgRelPath)

		grubmkCmd := fmt.Sprintf("grub-mkimage --format=%s-eltorito --output=%s", format, biosImgPath)
		grubmkCmd += fmt.Sprintf(" --config=%s --directory=%s --prefix=%s", loadCfgSrc, grubLibDir, prefixDir)
		grubmkCmd += " biosdisk iso9660"

		if _, err := shell.ExecCmd(grubmkCmd, true, "", nil); err != nil {
			log.Errorf("Failed to create eltorito image: %v", err)
			return biosImgRelPath, fmt.Errorf("failed to create eltorito image: %w", err)
		}

	default:
		log.Debugf("BIOS boot not supported for architecture: %s", target.Arch)
		return "", nil
	}
	return biosImgRelPath, nil
}

func createEfiFatImage(template *config.ImageTemplate, initrdRootfsPath, installRoot string) (efiFatImgPath string, err error) {
	target := template.GetTargetInfo()
	switch target.Arch {
	case "x86_64":
		format := "x86_64-efi"
		prefixDir := "/boot/grub"

		generalConfigDir, err := config.GetGeneralConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to get general config directory: %w", err)
		}
		loadCfgSrc := filepath.Join(generalConfigDir, "isolinux", "load.cfg")
		if _, err := os.Stat(loadCfgSrc); os.IsNotExist(err) {
			log.Errorf("load.cfg file does not exist: %s", loadCfgSrc)
			return "", fmt.Errorf("load.cfg file does not exist: %s", loadCfgSrc)
		}

		grubLibDir := filepath.Join(initrdRootfsPath, "usr", "lib", "grub", format)
		if _, err := os.Stat(grubLibDir); os.IsNotExist(err) {
			log.Errorf("GRUB modules directory does not exist: %s", grubLibDir)
			return "", fmt.Errorf("GRUB modules directory does not exist: %s", grubLibDir)
		}

		bootGrubLibDir := filepath.Join(installRoot, "boot", "grub", format)
		if err = file.CopyDir(grubLibDir, bootGrubLibDir, "--preserve=mode", true); err != nil {
			log.Errorf("Failed to copy grub modules to iso boot dir: %v", err)
			return "", fmt.Errorf("failed to copy grub modules to iso boot dir: %w", err)
		}

		efiFatImgPath = filepath.Join(installRoot, prefixDir, "efi.img")
		cmdStr := fmt.Sprintf("mformat -C -f 2880 -L 16 -i %s ::.", efiFatImgPath)
		if _, err := shell.ExecCmd(cmdStr, true, "", nil); err != nil {
			log.Errorf("Failed to create EFI FAT image: %v", err)
			return efiFatImgPath, fmt.Errorf("failed to create EFI FAT image: %w", err)
		}

		efiDirPath := filepath.Join(installRoot, "EFI")
		efiImgPath := filepath.Join(efiDirPath, "BOOT", "BOOTX64.EFI")

		grubmkCmd := fmt.Sprintf("grub-mkimage --format=%s --output=%s", format, efiImgPath)
		grubmkCmd += fmt.Sprintf(" --config=%s --directory=%s --prefix=%s", loadCfgSrc, grubLibDir, prefixDir)
		grubmkCmd += " part_gpt part_msdos fat ext2 ntfs search iso9660"

		if _, err := shell.ExecCmd(grubmkCmd, true, "", nil); err != nil {
			log.Errorf("Failed to create EFI image: %v", err)
			return efiFatImgPath, fmt.Errorf("failed to create EFI image: %w", err)
		}

		cmdStr = fmt.Sprintf("mcopy -s -i %s %s ::/.", efiFatImgPath, efiDirPath)
		if _, err := shell.ExecCmd(cmdStr, true, "", nil); err != nil {
			log.Errorf("Failed to copy EFI files to FAT image: %v", err)
			return efiFatImgPath, fmt.Errorf("failed to copy EFI files to FAT image: %w", err)
		}

	default:
		return "", fmt.Errorf("unsupported architecture for EFI FAT image: %s", target.Arch)
	}

	return efiFatImgPath, nil
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
