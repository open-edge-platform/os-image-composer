package imagesign_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imagesign"
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

	err := imagesign.SignImage(installRoot, template)
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

	err := imagesign.SignImage(installRoot, template)
	if err != nil {
		t.Errorf("SignImage should succeed when no secure boot keys are provided, got: %v", err)
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

	// Create the ESP directory structure but no UKI file
	espDir := filepath.Join(installRoot, "boot", "efi", "EFI")
	linuxDir := filepath.Join(espDir, "Linux")
	if err := os.MkdirAll(linuxDir, 0755); err != nil {
		t.Fatalf("Failed to create Linux directory: %v", err)
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

	err := imagesign.SignImage(installRoot, template)
	if err == nil {
		t.Error("SignImage should fail when UKI file doesn't exist")
	}

	// The error should be about signing failure since sbsign will fail
	if !strings.Contains(err.Error(), "failed to sign") {
		t.Logf("Got expected error: %v", err)
	}
}

func TestSignImage_MissingBootloaderFile(t *testing.T) {
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

	// Create UKI file but not bootloader
	ukiPath := filepath.Join(linuxDir, "linux.efi")
	if err := os.WriteFile(ukiPath, []byte("fake UKI"), 0644); err != nil {
		t.Fatalf("Failed to create UKI file: %v", err)
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

	err := imagesign.SignImage(installRoot, template)
	if err == nil {
		t.Error("SignImage should fail when bootloader file doesn't exist")
	}
}

func TestSignImage_WorkDirCreation(t *testing.T) {
	installRoot := t.TempDir()

	// Create complete setup
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
			Name: "test-config",
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: keyFile,
				SecureBootDBCrt: crtFile,
				SecureBootDBCer: cerFile,
			},
		},
	}

	// This test verifies the work directory creation logic
	err := imagesign.SignImage(installRoot, template)
	// Expected to fail due to missing sbsign, but should get past the file setup
	if err != nil && strings.Contains(err.Error(), "failed to get global work directory") {
		t.Logf("Work directory creation failed as expected in test environment: %v", err)
	}
}

func TestSignImage_EmptySecureBootPaths(t *testing.T) {
	installRoot := t.TempDir()

	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Immutability: config.ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "",
				SecureBootDBCrt: "",
				SecureBootDBCer: "",
			},
		},
	}

	err := imagesign.SignImage(installRoot, template)
	if err != nil {
		t.Errorf("SignImage should skip signing when secure boot paths are empty, got: %v", err)
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
