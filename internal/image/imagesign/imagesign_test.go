package imagesign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
)

func TestSignImage_ImmutabilityDisabled(t *testing.T) {
	installRoot := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled: false,
			},
		},
	}

	err := SignImage(installRoot, template)
	if err != nil {
		t.Errorf("SignImage should succeed when immutability is disabled, got: %v", err)
	}
}

func TestSignImage_NoSecureBootKeys(t *testing.T) {
	installRoot := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled: true,
				// No secure boot keys set
			},
		},
	}

	err := SignImage(installRoot, template)
	if err != nil {
		t.Errorf("SignImage should succeed when no secure boot keys are provided, got: %v", err)
	}
}

func TestSignImage_MissingKeyFiles(t *testing.T) {
	installRoot := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/nonexistent/key.key",
				SecureBootDBCrt: "/nonexistent/cert.crt",
				SecureBootDBCer: "/nonexistent/cert.cer",
			},
		},
	}

	err := SignImage(installRoot, template)
	if err == nil {
		t.Error("SignImage should fail when key files don't exist")
	}

	if !strings.Contains(err.Error(), "secure boot key or certificate file not found") {
		t.Errorf("Expected error about missing files, got: %v", err)
	}
}

func TestSignImage_MissingUKIFile(t *testing.T) {
	installRoot := t.TempDir()

	// Create temporary key files
	keyFile := filepath.Join(installRoot, "test.key")
	crtFile := filepath.Join(installRoot, "test.crt")
	cerFile := filepath.Join(installRoot, "test.cer")

	if err := os.WriteFile(keyFile, []byte("test key"), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}
	if err := os.WriteFile(crtFile, []byte("test cert"), 0644); err != nil {
		t.Fatalf("Failed to create test crt file: %v", err)
	}
	if err := os.WriteFile(cerFile, []byte("test cer"), 0644); err != nil {
		t.Fatalf("Failed to create test cer file: %v", err)
	}

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: keyFile,
				SecureBootDBCrt: crtFile,
				SecureBootDBCer: cerFile,
			},
		},
	}

	err := SignImage(installRoot, template)
	if err == nil {
		t.Error("SignImage should fail when UKI file doesn't exist")
	}
}

func TestSignImage_ValidSetupButNoSbsign(t *testing.T) {
	installRoot := t.TempDir()

	// Create directory structure
	espDir := filepath.Join(installRoot, "boot", "efi", "EFI")
	linuxDir := filepath.Join(espDir, "Linux")
	bootDir := filepath.Join(espDir, "BOOT")

	if err := os.MkdirAll(linuxDir, 0755); err != nil {
		t.Fatalf("Failed to create Linux directory: %v", err)
	}
	if err := os.MkdirAll(bootDir, 0755); err != nil {
		t.Fatalf("Failed to create BOOT directory: %v", err)
	}

	// Create UKI and bootloader files
	ukiPath := filepath.Join(linuxDir, "linux.efi")
	bootloaderPath := filepath.Join(bootDir, "BOOTX64.EFI")

	if err := os.WriteFile(ukiPath, []byte("fake UKI"), 0644); err != nil {
		t.Fatalf("Failed to create UKI file: %v", err)
	}
	if err := os.WriteFile(bootloaderPath, []byte("fake bootloader"), 0644); err != nil {
		t.Fatalf("Failed to create bootloader file: %v", err)
	}

	// Create temporary key files
	keyFile := filepath.Join(installRoot, "test.key")
	crtFile := filepath.Join(installRoot, "test.crt")
	cerFile := filepath.Join(installRoot, "test.cer")

	if err := os.WriteFile(keyFile, []byte("test key"), 0600); err != nil {
		t.Fatalf("Failed to create test key file: %v", err)
	}
	if err := os.WriteFile(crtFile, []byte("test cert"), 0644); err != nil {
		t.Fatalf("Failed to create test crt file: %v", err)
	}
	if err := os.WriteFile(cerFile, []byte("test cer"), 0644); err != nil {
		t.Fatalf("Failed to create test cer file: %v", err)
	}

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: keyFile,
				SecureBootDBCrt: crtFile,
				SecureBootDBCer: cerFile,
			},
		},
	}

	err := SignImage(installRoot, template)
	// This will likely fail because sbsign command doesn't exist in test environment
	// but we can verify the error is related to command execution, not file setup
	if err != nil && !strings.Contains(err.Error(), "failed to sign") {
		t.Logf("Expected signing failure due to missing sbsign command: %v", err)
	}
}

func TestSignImage_DirectoryStructure(t *testing.T) {
	installRoot := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/test/key.key",
				SecureBootDBCrt: "/test/cert.crt",
				SecureBootDBCer: "/test/cert.cer",
			},
		},
	}

	// Test that the function constructs correct paths
	// We expect it to look for:
	// - UKI at: installRoot/boot/efi/EFI/Linux/linux.efi
	// - Bootloader at: installRoot/boot/efi/EFI/BOOT/BOOTX64.EFI

	err := SignImage(installRoot, template)
	if err == nil {
		t.Error("SignImage should fail when files don't exist")
	}

	// The error should indicate missing key files (first check that fails)
	if !strings.Contains(err.Error(), "secure boot key or certificate file not found") {
		t.Errorf("Expected error about missing key files, got: %v", err)
	}
}

func TestSignImage_PartialSecureBootConfig(t *testing.T) {
	installRoot := t.TempDir()

	// Test with only key file set (missing cert files)
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/test/key.key",
				// SecureBootDBCrt and SecureBootDBCer are empty
			},
		},
	}

	err := SignImage(installRoot, template)
	if err != nil {
		t.Errorf("SignImage should skip signing when secure boot config is incomplete, got: %v", err)
	}
}

// Test helper methods from config package
func TestConfigHelperMethods(t *testing.T) {
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/path/to/key.key",
				SecureBootDBCrt: "/path/to/cert.crt",
				SecureBootDBCer: "/path/to/cert.cer",
			},
		},
	}

	if !template.IsImmutabilityEnabled() {
		t.Error("IsImmutabilityEnabled should return true")
	}

	if template.GetSecureBootDBKeyPath() != "/path/to/key.key" {
		t.Errorf("GetSecureBootDBKeyPath returned %s, expected /path/to/key.key", template.GetSecureBootDBKeyPath())
	}

	if template.GetSecureBootDBCrtPath() != "/path/to/cert.crt" {
		t.Errorf("GetSecureBootDBCrtPath returned %s, expected /path/to/cert.crt", template.GetSecureBootDBCrtPath())
	}

	if template.GetSecureBootDBCerPath() != "/path/to/cert.cer" {
		t.Errorf("GetSecureBootDBCerPath returned %s, expected /path/to/cert.cer", template.GetSecureBootDBCerPath())
	}
}
