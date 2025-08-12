package imagesign

import (
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func SignImage(installRoot string, template *config.ImageTemplate) error {

	//if immutability is not enabled, skip signing
	if !template.IsImmutabilityEnabled() {
		return nil
	}

	// Check if secure boot keys are provided
	// If not, skip signing
	if template.GetSecureBootDBKeyPath() == "" ||
		template.GetSecureBootDBCrtPath() == "" ||
		template.GetSecureBootDBCerPath() == "" {
		return nil
	}

	// pbKeyPath := "/data/secureboot/keys/DB.key"
	// prKeyPath := "/data/secureboot/keys/DB.crt"
	pbKeyPath := template.GetSecureBootDBKeyPath()
	prKeyPath := template.GetSecureBootDBCrtPath()
	prCerPath := template.GetSecureBootDBCerPath()

	// Check if the key and certificate files exist
	if _, err := shell.ExecCmd(fmt.Sprintf("test -f %s && test -f %s && test -f %s", pbKeyPath, prKeyPath, prCerPath), true, "", nil); err != nil {
		return fmt.Errorf("secure boot key or certificate file not found: %w", err)
	}

	espDir := filepath.Join(installRoot, "boot", "efi")
	ukiPath := filepath.Join(espDir, "EFI", "Linux", "linux.efi")
	bootloaderPath := filepath.Join(espDir, "EFI", "BOOT", "BOOTX64.EFI")

	// Sign the UKI (Unified Kernel Image) - create signed file then replace original
	ukiSignedPath := filepath.Join(espDir, "EFI", "Linux", "linux.efi.signed")
	cmd := fmt.Sprintf("sbsign --key %s --cert %s --output %s %s",
		pbKeyPath, prKeyPath, ukiSignedPath, ukiPath)
	if _, err := shell.ExecCmd(cmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to sign UKI: %w", err)
	}

	// Replace original with signed version
	mvCmd := fmt.Sprintf("mv %s %s", ukiSignedPath, ukiPath)
	if _, err := shell.ExecCmd(mvCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to replace UKI with signed version: %w", err)
	}

	// Sign the bootloader - create signed file then replace original
	bootloaderSignedPath := filepath.Join(espDir, "EFI", "BOOT", "BOOTX64.EFI.signed")
	cmd = fmt.Sprintf("sbsign --key %s --cert %s --output %s %s",
		pbKeyPath, prKeyPath, bootloaderSignedPath, bootloaderPath)
	if _, err := shell.ExecCmd(cmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to sign bootloader: %w", err)
	}

	// Replace original with signed version
	mvCmd = fmt.Sprintf("mv %s %s", bootloaderSignedPath, bootloaderPath)
	if _, err := shell.ExecCmd(mvCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to replace bootloader with signed version: %w", err)
	}

	return nil
}
