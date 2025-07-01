package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadYAMLTemplate(t *testing.T) {
	// Create a temporary YAML file with new single object structure
	yamlContent := `image:
  name: azl3-x86_64-edge
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

systemConfig:
  name: edge
  description: Default yml configuration for edge image
  packages:
    - openssh-server
    - docker-ce
    - vim
    - curl
    - wget
  kernel:
    version: "6.12"
    cmdline: "quiet splash"
`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading
	template, err := LoadTemplate(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load YAML template: %v", err)
	}

	// Verify the loaded template
	if template.Target.OS != "azure-linux" {
		t.Errorf("expected OS 'azure-linux', got %s", template.Target.OS)
	}
	if template.Target.Dist != "azl3" {
		t.Errorf("expected dist 'azl3', got %s", template.Target.Dist)
	}
	if template.Target.Arch != "x86_64" {
		t.Errorf("expected arch 'x86_64', got %s", template.Target.Arch)
	}
	if len(template.GetPackages()) != 5 {
		t.Errorf("expected 5 packages, got %d", len(template.GetPackages()))
	}
	if template.Target.ImageType != "raw" {
		t.Errorf("expected imageType 'raw', got %s", template.Target.ImageType)
	}
	if template.GetKernel().Version != "6.12" {
		t.Errorf("expected kernel version '6.12', got %s", template.GetKernel().Version)
	}
}

func TestTemplateHelperMethods(t *testing.T) {
	template := &ImageTemplate{
		Image: ImageInfo{
			Name:    "test-image",
			Version: "1.0.0",
		},
		Target: TargetInfo{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "iso",
		},
		// Updated to use single SystemConfig instead of array
		SystemConfig: SystemConfig{
			Name:        "test-config",
			Description: "Test configuration",
			Packages:    []string{"package1", "package2"},
			Kernel: KernelConfig{
				Version: "6.12",
				Cmdline: "quiet",
			},
		},
	}

	// Test provider name mapping
	providerName := template.GetProviderName()
	if providerName != "AzureLinux3" {
		t.Errorf("expected provider 'AzureLinux3', got %s", providerName)
	}

	// Test version mapping
	version := template.GetDistroVersion()
	if version != "3" {
		t.Errorf("expected version '3', got %s", version)
	}

	// Test package extraction
	packages := template.GetPackages()
	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(packages))
	}

	// Test kernel extraction
	kernel := template.GetKernel()
	if kernel.Version != "6.12" {
		t.Errorf("expected kernel version '6.12', got %s", kernel.Version)
	}

	// Test system config name extraction
	configName := template.GetSystemConfigName()
	if configName != "test-config" {
		t.Errorf("expected config name 'test-config', got %s", configName)
	}

	// Test disk config (empty in this test)
	diskConfig := template.GetDiskConfig()
	if diskConfig.Name != "" {
		t.Errorf("expected empty disk config name, got %s", diskConfig.Name)
	}
}

func TestUnsupportedFileFormat(t *testing.T) {
	// Create a temporary file with unsupported extension
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("some content"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading should fail
	_, err = LoadTemplate(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for unsupported file format")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("expected unsupported file format error, got: %v", err)
	}
}

func TestEmptySystemConfig(t *testing.T) {
	// Test template with empty system config
	template := &ImageTemplate{
		Image: ImageInfo{
			Name:    "test",
			Version: "1.0.0",
		},
		Target: TargetInfo{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "raw",
		},
		// Empty SystemConfig
		SystemConfig: SystemConfig{},
	}

	// Test that empty config still works
	packages := template.GetPackages()
	if len(packages) != 0 {
		t.Errorf("expected 0 packages for empty config, got %d", len(packages))
	}

	configName := template.GetSystemConfigName()
	if configName != "" {
		t.Errorf("expected empty config name, got %s", configName)
	}
}

func TestAllSupportedProviders(t *testing.T) {
	testCases := []struct {
		os       string
		dist     string
		expected string
		version  string
	}{
		{"azure-linux", "azl3", "AzureLinux3", "3"},
		{"emt", "emt3", "EMT3.0", "3.0"},
		{"elxr", "elxr12", "eLxr12", "12"},
	}

	for _, tc := range testCases {
		template := &ImageTemplate{
			Target: TargetInfo{
				OS:        tc.os,
				Dist:      tc.dist,
				Arch:      "x86_64",
				ImageType: "iso",
			},
			SystemConfig: SystemConfig{
				Name:     "test",
				Packages: []string{"test-package"},
				Kernel:   KernelConfig{Version: "6.12"},
			},
		}

		// Test provider name
		providerName := template.GetProviderName()
		if providerName != tc.expected {
			t.Errorf("for %s/%s expected provider '%s', got '%s'", tc.os, tc.dist, tc.expected, providerName)
		}

		// Test version
		version := template.GetDistroVersion()
		if version != tc.version {
			t.Errorf("for %s/%s expected version '%s', got '%s'", tc.os, tc.dist, tc.version, version)
		}
	}
}

func TestDiskAndSystemConfigGetters(t *testing.T) {
	template := &ImageTemplate{
		Image: ImageInfo{
			Name:    "test-image",
			Version: "1.0.0",
		},
		Target: TargetInfo{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "raw",
		},
		Disk: DiskConfig{
			Name: "test-disk",
			Size: "4GiB",
			Partitions: []PartitionInfo{
				{
					ID:         "root",
					FsType:     "ext4",
					MountPoint: "/",
				},
			},
		},
		SystemConfig: SystemConfig{
			Name: "test-config",
			Bootloader: Bootloader{
				BootType: "efi",
				Provider: "grub2",
			},
			Packages: []string{"package1", "package2"},
			Kernel: KernelConfig{
				Version: "6.12",
				Cmdline: "quiet splash",
			},
		},
	}

	// Test disk config getter
	diskConfig := template.GetDiskConfig()
	if diskConfig.Name != "test-disk" {
		t.Errorf("expected disk name 'test-disk', got %s", diskConfig.Name)
	}
	if diskConfig.Size != "4GiB" {
		t.Errorf("expected disk size '4GiB', got %s", diskConfig.Size)
	}
	if len(diskConfig.Partitions) != 1 {
		t.Errorf("expected 1 partition, got %d", len(diskConfig.Partitions))
	}

	// Test system config getter
	systemConfig := template.GetSystemConfig()
	if systemConfig.Name != "test-config" {
		t.Errorf("expected system config name 'test-config', got %s", systemConfig.Name)
	}

	// Test bootloader config getter
	bootloaderConfig := template.GetBootloaderConfig()
	if bootloaderConfig.BootType != "efi" {
		t.Errorf("expected bootloader type 'efi', got %s", bootloaderConfig.BootType)
	}
	if bootloaderConfig.Provider != "grub2" {
		t.Errorf("expected bootloader provider 'grub2', got %s", bootloaderConfig.Provider)
	}
}
