package imageboot

import (
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

var log = logger.Logger()

func getDiskPartDevByMountPoint(mountPoint string, diskPathIdMap map[string]string, template *config.ImageTemplate) string {
	diskInfo := template.GetDiskConfig()
	partions := diskInfo.Partitions
	for diskId, diskPath := range diskPathIdMap {
		for _, partition := range partions {
			if partition.ID == diskId && partition.MountPoint == mountPoint {
				return diskPath
			}
		}
	}
	return ""
}

func installGrubWithLegacyMode(installRoot, bootUUID, bootPrefix string, template *config.ImageTemplate) error {
	log.Errorf("Legacy boot mode is not implemented yet")
	return fmt.Errorf("legacy boot mode is not implemented yet")
}

func installGrubWithEfiMode(installRoot, bootUUID, bootPrefix string, template *config.ImageTemplate) error {
	// Expect that shim (bootx64.efi) and grub2 (grub2.efi) are installed
	// into the EFI directory via the package installation step previously.

	log.Infof("Installing Grub2 bootloader with EFI mode")
	efiDir := "/boot/efi"
	configDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	grubAssetPath := filepath.Join(configDir, "image", "efi", "grub", "grub.cfg")
	grubFinalPath := filepath.Join(installRoot, efiDir, "boot/grub2/grub.cfg")

	if err = file.CopyFile(grubAssetPath, grubFinalPath, "", true); err != nil {
		log.Errorf("Failed to copy grub configuration file: %v", err)
		return fmt.Errorf("failed to copy grub configuration file: %w", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.BootUUID}}", bootUUID, grubFinalPath); err != nil {
		log.Errorf("Failed to replace boot UUID in grub configuration: %v", err)
		return fmt.Errorf("failed to replace boot UUID in grub configuration: %w", err)
	}

	// Replace CryptoMountCommand placeholder with an empty string for now.
	if err := file.ReplacePlaceholdersInFile("{{.CryptoMountCommand}}", "", grubFinalPath); err != nil {
		log.Errorf("Failed to replace CryptoMountCommand in grub configuration: %v", err)
		return fmt.Errorf("failed to replace CryptoMountCommand in grub configuration: %w", err)
	}

	prefixPath := fmt.Sprintf("%s/grub2", bootPrefix)
	if err := file.ReplacePlaceholdersInFile("{{.PrefixPath}}", prefixPath, grubFinalPath); err != nil {
		log.Errorf("Failed to replace PrefixPath in grub configuration: %v", err)
		return fmt.Errorf("failed to replace prefix path in grub configuration: %w", err)
	}

	chmodCmd := fmt.Sprintf("chmod -R 700 %s", filepath.Dir(grubFinalPath))
	if _, err = shell.ExecCmd(chmodCmd, true, "", nil); err != nil {
		log.Errorf("Failed to set permissions for grub configuration directory: %v", err)
		return fmt.Errorf("failed to set permissions for grub configuration directory: %w", err)
	}

	chmodCmd = fmt.Sprintf("chmod 400 %s", grubFinalPath)
	if _, err = shell.ExecCmd(chmodCmd, true, "", nil); err != nil {
		log.Errorf("Failed to set permissions for grub configuration file: %v", err)
		return fmt.Errorf("failed to set permissions for grub configuration file: %w", err)
	}

	return nil
}

func copyGrubEnvFile(installRoot string) error {
	configDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}
	grubEnvAssetPath := filepath.Join(configDir, "image", "grub2", "grubenv")
	grubEnvFinalPath := filepath.Join(installRoot, "boot", "grub2", "grubenv")
	if err = file.CopyFile(grubEnvAssetPath, grubEnvFinalPath, "", true); err != nil {
		log.Errorf("Failed to copy grubenv file: %v", err)
		return fmt.Errorf("failed to copy grubenv file: %w", err)
	}
	return nil
}

func updateGrubConfig(installRoot string) error {
	grubConfigFile := "/boot/grub2/grub.cfg"
	cmdStr := fmt.Sprintf("grub2-mkconfig -o %s", grubConfigFile)
	if _, err := shell.ExecCmd(cmdStr, true, installRoot, nil); err != nil {
		log.Errorf("Failed to update grub configuration: %v", err)
		return fmt.Errorf("failed to update grub configuration: %w", err)
	}
	return nil
}

func updateBootConfigTemplate(installRoot, rootDevID, bootUUID, bootPrefix, hashDevID, rootHashPH string, template *config.ImageTemplate) error {
	log.Infof("Updating boot configurations")

	configDir, err := config.GetGeneralConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get general config directory: %w", err)
	}

	var configAssetPath string
	var configFinalPath string
	bootloaderConfig := template.GetBootloaderConfig()
	switch bootloaderConfig.Provider {
	case "grub":
		configAssetPath = filepath.Join(configDir, "image", "grub2", "grub")
		configFinalPath = filepath.Join(installRoot, "etc", "default", "grub")
		if err = file.CopyFile(configAssetPath, configFinalPath, "", true); err != nil {
			log.Errorf("Failed to copy boot configuration file: %v", err)
			return fmt.Errorf("failed to copy boot configuration file: %w", err)
		}
	case "systemd-boot":
		configAssetPath = filepath.Join(configDir, "image", "efi", "bootParams.conf")
		configFinalPath = filepath.Join(installRoot, "boot", "cmdline.conf")
		if err = file.CopyFile(configAssetPath, configFinalPath, "", true); err != nil {
			log.Errorf("Failed to copy boot configuration file: %v", err)
			return fmt.Errorf("failed to copy boot configuration file: %w", err)
		}
	default:
		log.Errorf("Unsupported bootloader provider: %s", bootloaderConfig.Provider)
		return fmt.Errorf("unsupported bootloader provider: %s", bootloaderConfig.Provider)
	}

	if err := file.ReplacePlaceholdersInFile("{{.BootUUID}}", bootUUID, configFinalPath); err != nil {
		log.Errorf("Failed to replace BootUUID in boot configuration: %v", err)
		return fmt.Errorf("failed to replace BootUUID in boot configuration: %w", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.BootPrefix}}", bootPrefix, configFinalPath); err != nil {
		log.Errorf("Failed to replace BootPrefix in boot configuration: %v", err)
		return fmt.Errorf("failed to replace BootPrefix in boot configuration: %w", err)
	}

	if template.IsImmutabilityEnabled() {
		if err := file.ReplacePlaceholdersInFile("{{.RootPartition}}", "/dev/mapper/root", configFinalPath); err != nil {
			log.Errorf("Failed to replace RootPartition in boot configuration: %v", err)
			return fmt.Errorf("failed to replace RootPartition in boot configuration: %w", err)
		}
		// Construct systemd verity command line if hashDevID is provided
		verityCmd := ""
		if hashDevID != "" {
			verityCmd = fmt.Sprintf("systemd.verity_name=root systemd.verity_root_data=%s systemd.verity_root_hash=%s", rootDevID, hashDevID)
		}
		if err := file.ReplacePlaceholdersInFile("{{.SystemdVerity}}", verityCmd, configFinalPath); err != nil {
			log.Errorf("Failed to replace dm verity arg in boot configuration: %v", err)
			return fmt.Errorf("failed to replace dm verity arg in boot configuration: %w", err)
		}
		if err := file.ReplacePlaceholdersInFile("{{.RootHash}}", rootHashPH, configFinalPath); err != nil {
			log.Errorf("Failed to replace dm verity roothash in boot configuration: %v", err)
			return fmt.Errorf("failed to replace dm verity roothash in boot configuration: %w", err)
		}
	} else {
		if err := file.ReplacePlaceholdersInFile("{{.RootPartition}}", rootDevID, configFinalPath); err != nil {
			log.Errorf("Failed to replace RootPartition in boot configuration: %v", err)
			return fmt.Errorf("failed to replace RootPartition in boot configuration: %w", err)
		}
		if err := file.ReplacePlaceholdersInFile("{{.SystemdVerity}}", "", configFinalPath); err != nil {
			log.Errorf("Failed to replace dm verity arg in boot configuration: %v", err)
			return fmt.Errorf("failed to replace dm verity arg in boot configuration: %w", err)
		}
		if err := file.ReplacePlaceholdersInFile("{{.RootHash}}", "", configFinalPath); err != nil {
			log.Errorf("Failed to replace dm verity roothash in boot configuration: %v", err)
			return fmt.Errorf("failed to replace dm verity roothash in boot configuration: %w", err)
		}
	}

	// For now, we do not support LUKS encryption, so we replace the LuksUUID placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.LuksUUID}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace LuksUUID in boot configuration: %v", err)
		return fmt.Errorf("failed to replace LuksUUID in boot configuration: %w", err)
	}

	// For now, we do not support LVM, so we replace the LVM placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.LVM}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace LVM in boot configuration: %v", err)
		return fmt.Errorf("failed to replace LVM in boot configuration: %w", err)
	}

	// For now, we do not support IMAPolicy, so we replace the IMAPolicy placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.IMAPolicy}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace IMAPolicy in boot configuration: %v", err)
		return fmt.Errorf("failed to replace IMAPolicy in boot configuration: %w", err)
	}

	// For now, we do not support SELinux, so we replace the SELinux placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.SELinux}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace SELinux in boot configuration: %v", err)
		return fmt.Errorf("failed to replace SELinux in boot configuration: %w", err)
	}

	// For now, we do not support FIPS, so we replace the FIPS placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.FIPS}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace FIPS in boot configuration: %v", err)
		return fmt.Errorf("failed to replace FIPS in boot configuration: %w", err)
	}

	// For now, we do not support CGroup, so we replace the CGroup placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.CGroup}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace CGroup in boot configuration: %v", err)
		return fmt.Errorf("failed to replace CGroup in boot configuration: %w", err)
	}

	kernelConfig := template.GetKernel()
	if err := file.ReplacePlaceholdersInFile("{{.ExtraCommandLine}}", kernelConfig.Cmdline, configFinalPath); err != nil {
		log.Errorf("Failed to replace ExtraCommandLine in boot configuration: %v", err)
		return fmt.Errorf("failed to replace ExtraCommandLine in boot configuration: %w", err)
	}

	// For now, we do not support EncryptionBootUUID, so we replace the EncryptionBootUUID placeholder with an empty string.
	if err := file.ReplacePlaceholdersInFile("{{.EncryptionBootUUID}}", "", configFinalPath); err != nil {
		log.Errorf("Failed to replace EncryptionBootUUID in boot configuration: %v", err)
		return fmt.Errorf("failed to replace EncryptionBootUUID in boot configuration: %w", err)
	}

	if err := file.ReplacePlaceholdersInFile("{{.rdAuto}}", "rd.auto=1", configFinalPath); err != nil {
		log.Errorf("Failed to replace rdAuto in boot configuration: %v", err)
		return fmt.Errorf("failed to replace rdAuto in boot configuration: %w", err)
	}

	return nil
}

func InstallImageBoot(installRoot string, diskPathIdMap map[string]string, template *config.ImageTemplate) error {
	var bootUUID string
	var bootPrefix string = ""
	var rootDev string
	var hashDev string
	var err error

	log.Infof("Installing image bootloader for: %s", template.Image.Name)

	bootPartDev := getDiskPartDevByMountPoint("/boot", diskPathIdMap, template)
	if bootPartDev == "" {
		// /boot is not a separate partition, use root partition instead
		bootPrefix = "/boot"
		rootDev = getDiskPartDevByMountPoint("/", diskPathIdMap, template)
		if rootDev == "" {
			return fmt.Errorf("failed to find root partition for mount point '/'")
		}
		bootUUID, err = imagedisc.GetUUID(rootDev)
		if err != nil {
			return fmt.Errorf("failed to get UUID for boot partition %s: %w", rootDev, err)
		}
	} else {
		bootUUID, err = imagedisc.GetUUID(bootPartDev)
		if err != nil {
			return fmt.Errorf("failed to get UUID for boot partition %s: %w", bootPartDev, err)
		}
		rootDev = getDiskPartDevByMountPoint("/", diskPathIdMap, template)
		if rootDev == "" {
			return fmt.Errorf("failed to find root partition for mount point '/'")
		}
	}

	rootPartUUID, err := imagedisc.GetPartUUID(rootDev)
	if err != nil {
		return fmt.Errorf("failed to get partition UUID for root partition %s: %w", rootDev, err)
	}
	rootDevID := fmt.Sprintf("PARTUUID=%s", rootPartUUID)

	bootloaderConfig := template.GetBootloaderConfig()
	switch bootloaderConfig.Provider {
	case "grub":
		log.Infof("Installing GRUB bootloader")
		if bootloaderConfig.BootType == "efi" {
			if err := installGrubWithEfiMode(installRoot, bootUUID, bootPrefix, template); err != nil {
				return fmt.Errorf("failed to install GRUB bootloader with EFI mode: %w", err)
			}
		} else if bootloaderConfig.BootType == "legacy" {
			if err := installGrubWithLegacyMode(installRoot, bootUUID, bootPrefix, template); err != nil {
				return fmt.Errorf("failed to install GRUB bootloader with legacy mode: %w", err)
			}
		}

		if err := updateBootConfigTemplate(installRoot, rootDevID, bootUUID, bootPrefix, "", "", template); err != nil {
			return fmt.Errorf("failed to update boot configuration: %w", err)
		}

		if err := copyGrubEnvFile(installRoot); err != nil {
			return fmt.Errorf("failed to copy grubenv file: %w", err)
		}

		if err := updateGrubConfig(installRoot); err != nil {
			return fmt.Errorf("failed to update grub configuration: %w", err)
		}

	case "systemd-boot":
		log.Infof("Installing systemd-boot bootloader")
		if bootloaderConfig.BootType == "efi" {

			if template.IsImmutabilityEnabled() {
				hashDev = getDiskPartDevByMountPoint("none", diskPathIdMap, template)
				if hashDev == "" {
					return fmt.Errorf("failed to find dm verity hash partition")
				}
				hashPartUUID, err := imagedisc.GetPartUUID(hashDev)
				if err != nil {
					return fmt.Errorf("failed to get partition UUID for dm verity hash partition %s: %w", rootDev, err)
				}
				hashDevID := fmt.Sprintf("PARTUUID=%s", hashPartUUID)
				rootHashPH := fmt.Sprintf("roothash=%s-%s", rootDev, hashDev)
				if err := updateBootConfigTemplate(installRoot, rootDevID, bootUUID, bootPrefix, hashDevID, rootHashPH, template); err != nil {
					return fmt.Errorf("failed to update boot configuration: %w", err)
				}
			} else {
				if err := updateBootConfigTemplate(installRoot, rootDevID, bootUUID, bootPrefix, "", "", template); err != nil {
					return fmt.Errorf("failed to update boot configuration: %w", err)
				}
			}
		} else {
			return fmt.Errorf("systemd-boot is only supported in EFI mode")
		}
	default:
		return fmt.Errorf("unsupported bootloader provider: %s", bootloaderConfig.Provider)
	}

	return nil
}
