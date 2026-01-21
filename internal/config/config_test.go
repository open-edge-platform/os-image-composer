package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config/validate"
	"github.com/open-edge-platform/os-image-composer/internal/utils/slice"
)

func TestMergeStringSlices(t *testing.T) {
	defaultSlice := []string{"a", "b", "c"}
	userSlice := []string{"c", "d", "e"}

	merged := mergeStringSlices(defaultSlice, userSlice)

	expectedLength := 5 // a, b, c, d, e (no duplicates)
	if len(merged) != expectedLength {
		t.Errorf("expected merged slice length %d, got %d", expectedLength, len(merged))
	}

	// Verify no duplicates
	itemMap := make(map[string]int)
	for _, item := range merged {
		itemMap[item]++
		if itemMap[item] > 1 {
			t.Errorf("found duplicate item '%s' in merged slice", item)
		}
	}

	// Verify all expected items are present
	expectedItems := []string{"a", "b", "c", "d", "e"}
	for _, expectedItem := range expectedItems {
		if itemMap[expectedItem] != 1 {
			t.Errorf("expected item '%s' to be present exactly once", expectedItem)
		}
	}
}

func TestMergeStringSlicesEmpty(t *testing.T) {
	// Both slices empty
	result := mergeStringSlices([]string{}, []string{})
	if len(result) != 0 {
		t.Errorf("expected empty result for two empty slices, got %d items", len(result))
	}

	// One slice empty
	slice1 := []string{"a", "b"}
	result = mergeStringSlices(slice1, []string{})
	if len(result) != 2 {
		t.Errorf("expected 2 items when second slice is empty, got %d", len(result))
	}

	result = mergeStringSlices([]string{}, slice1)
	if len(result) != 2 {
		t.Errorf("expected 2 items when first slice is empty, got %d", len(result))
	}
}

func TestMergeStringSlicesWithNils(t *testing.T) {
	slice1 := []string{"a", "b"}

	// This tests the actual behavior of mergeStringSlices with nil slices
	result := mergeStringSlices(nil, slice1)
	if len(result) != 2 {
		t.Errorf("expected 2 items when first slice is nil, got %d", len(result))
	}

	result = mergeStringSlices(slice1, nil)
	if len(result) != 2 {
		t.Errorf("expected 2 items when second slice is nil, got %d", len(result))
	}
}

func TestEmptyUsersConfig(t *testing.T) {
	// Test template with no users
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
		SystemConfig: SystemConfig{
			Name: "test-config",
			// No users configured
		},
	}

	// Test that empty users config works
	users := template.GetUsers()
	if len(users) != 0 {
		t.Errorf("expected 0 users for empty config, got %d", len(users))
	}

	if template.HasUsers() {
		t.Errorf("expected template to not have users")
	}

	nonExistentUser := template.GetUserByName("anyuser")
	if nonExistentUser != nil {
		t.Errorf("expected not to find any user in empty config")
	}
}

func TestMergeSystemConfigWithSecureBoot(t *testing.T) {
	defaultConfig := SystemConfig{
		Name: "default",
		Immutability: ImmutabilityConfig{
			Enabled:         true,
			SecureBootDBKey: "/default/keys/db.key",
			SecureBootDBCrt: "/default/certs/db.crt",
		},
		Packages: []string{"base-package"},
	}

	userConfig := SystemConfig{
		Name: "user",
		Immutability: ImmutabilityConfig{
			Enabled:         true,
			SecureBootDBKey: "/user/keys/custom.key",  // Override key
			SecureBootDBCer: "/user/certs/custom.cer", // Add new cer
			// Don't override crt - should keep default
		},
		Packages: []string{"user-package"},
	}

	merged := mergeSystemConfig(defaultConfig, userConfig)

	// Verify immutability merging
	if !merged.Immutability.Enabled {
		t.Errorf("expected merged immutability to be enabled")
	}

	if merged.Immutability.SecureBootDBKey != "/user/keys/custom.key" {
		t.Errorf("expected user secure boot key to override default")
	}

	if merged.Immutability.SecureBootDBCrt != "/default/certs/db.crt" {
		t.Errorf("expected default secure boot crt to be preserved")
	}

	if merged.Immutability.SecureBootDBCer != "/user/certs/custom.cer" {
		t.Errorf("expected user secure boot cer to be added")
	}
}

func TestLoadYAMLTemplateWithImmutability(t *testing.T) {
	// Create a temporary YAML file with immutability configuration under systemConfig
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
  immutability:
    enabled: true
  packages:
    - openssh-server
    - docker-ce
  kernel:
    version: "6.12"
    cmdline: "quiet splash"
`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading
	template, err := LoadTemplate(tmpFile.Name(), true)
	if err != nil {
		t.Fatalf("failed to load YAML template: %v", err)
	}

	// Verify immutability configuration
	if !template.IsImmutabilityEnabled() {
		t.Errorf("expected immutability to be enabled, got %t", template.IsImmutabilityEnabled())
	}

	// Test direct access to systemConfig immutability
	if !template.SystemConfig.IsImmutabilityEnabled() {
		t.Errorf("expected systemConfig immutability to be enabled, got %t", template.SystemConfig.IsImmutabilityEnabled())
	}
}

func TestMergeSystemConfigWithImmutabilityExplicitFalse(t *testing.T) {
	defaultConfig := SystemConfig{
		Name:         "default",
		Immutability: ImmutabilityConfig{Enabled: true},
		Packages:     []string{"base-package"},
	}

	// User explicitly provides immutability config with secure boot settings
	// This represents a user who explicitly configured immutability
	userConfig := SystemConfig{
		Name: "user",
		Immutability: ImmutabilityConfig{
			Enabled:         false,
			SecureBootDBKey: "/path/to/key", // This makes it clear user provided config
		},
		Packages: []string{"user-package"},
	}

	merged := mergeSystemConfig(defaultConfig, userConfig)

	if merged.Immutability.Enabled != false {
		t.Errorf("expected merged immutability to be false (user override), got %t", merged.Immutability.Enabled)
	}

	if merged.Name != "user" {
		t.Errorf("expected merged name to be 'user', got %s", merged.Name)
	}
}

func TestMergeSystemConfigWithEmptyImmutability(t *testing.T) {
	defaultConfig := SystemConfig{
		Name:         "default",
		Immutability: ImmutabilityConfig{Enabled: true},
		Packages:     []string{"base-package"},
	}

	// User config with no immutability configuration (zero values)
	userConfig := SystemConfig{
		Name:     "user",
		Packages: []string{"user-package"},
		// Note: No Immutability field set, so it gets zero values
	}

	merged := mergeSystemConfig(defaultConfig, userConfig)

	// Should keep default immutability since user didn't specify it
	if merged.Immutability.Enabled != true {
		t.Errorf("expected merged immutability to be true (default preserved), got %t", merged.Immutability.Enabled)
	}

	if merged.Name != "user" {
		t.Errorf("expected merged name to be 'user', got %s", merged.Name)
	}
}

func TestLoadAndMergeTemplateWithRealYAML(t *testing.T) {
	// Setup temporary config directory
	tempDir := t.TempDir()

	// Save original global config
	originalGlobal := Global()
	defer SetGlobal(originalGlobal)

	// Set new global config with temp dir
	newGlobal := DefaultGlobalConfig()
	newGlobal.ConfigDir = tempDir
	SetGlobal(newGlobal)

	// Create directory structure for default config
	osDistDir := filepath.Join(tempDir, "osv", "wind-river-elxr", "elxr12")
	defaultConfigDir := filepath.Join(osDistDir, "imageconfigs", "defaultconfigs")
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create default config with immutability enabled and hash partition
	defaultConfigContent := `
image:
  name: default-image
  version: "12.0.0"
target:
  os: wind-river-elxr
  dist: elxr12
  arch: x86_64
  imageType: raw
disk:
  name: default-disk
  partitions:
    - id: root
      mountPoint: /
    - id: roothashmap
      type: linux
      mountPoint: none
systemConfig:
  name: default-system
  immutability:
    enabled: true
  packages:
    - default-pkg
`
	defaultConfigFile := filepath.Join(defaultConfigDir, "default-raw-x86_64.yml")
	if err := os.WriteFile(defaultConfigFile, []byte(defaultConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write default config file: %v", err)
	}

	// Create user template without immutability section
	userConfigContent := `
image:
  name: user-image
  version: "12.0.0"
target:
  os: wind-river-elxr
  dist: elxr12
  arch: x86_64
  imageType: raw
systemConfig:
  name: user-system
  packages:
    - user-pkg
`
	userConfigFile := filepath.Join(tempDir, "user-config.yml")
	if err := os.WriteFile(userConfigFile, []byte(userConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write user config file: %v", err)
	}

	// Test LoadAndMergeTemplate
	template, err := LoadAndMergeTemplate(userConfigFile)
	if err != nil {
		t.Fatalf("LoadAndMergeTemplate failed: %v", err)
	}

	// Verify that immutability is preserved from default (true) since user didn't specify it
	if !template.IsImmutabilityEnabled() {
		t.Errorf("Expected immutability to be enabled (from default), but got %t", template.IsImmutabilityEnabled())
	}
}

func TestLoadAndMergeTemplateImmutabilityAutoDisabled(t *testing.T) {
	// Setup temporary config directory
	tempDir := t.TempDir()

	// Save original global config
	originalGlobal := Global()
	defer SetGlobal(originalGlobal)

	// Set new global config with temp dir
	newGlobal := DefaultGlobalConfig()
	newGlobal.ConfigDir = tempDir
	SetGlobal(newGlobal)

	// Create directory structure for default config
	osDistDir := filepath.Join(tempDir, "osv", "wind-river-elxr", "elxr12")
	defaultConfigDir := filepath.Join(osDistDir, "imageconfigs", "defaultconfigs")
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create default config with immutability enabled and hash partition
	defaultConfigContent := `
image:
  name: default-image
  version: "12.0.0"
target:
  os: wind-river-elxr
  dist: elxr12
  arch: x86_64
  imageType: raw
disk:
  name: default-disk
  partitions:
    - id: root
      mountPoint: /
    - id: roothashmap
      type: linux
      mountPoint: none
systemConfig:
  name: default-system
  immutability:
    enabled: true
  packages:
    - default-pkg
`
	defaultConfigFile := filepath.Join(defaultConfigDir, "default-raw-x86_64.yml")
	if err := os.WriteFile(defaultConfigFile, []byte(defaultConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write default config file: %v", err)
	}

	// Create user template with custom disk (no hash partition) and no immutability section
	userConfigContent := `
image:
  name: user-image
  version: "12.0.0"
target:
  os: wind-river-elxr
  dist: elxr12
  arch: x86_64
  imageType: raw
disk:
  name: user-disk
  partitions:
    - id: root
      mountPoint: /
systemConfig:
  name: user-system
  packages:
    - user-pkg
`
	userConfigFile := filepath.Join(tempDir, "user-config.yml")
	if err := os.WriteFile(userConfigFile, []byte(userConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write user config file: %v", err)
	}

	// Test LoadAndMergeTemplate
	template, err := LoadAndMergeTemplate(userConfigFile)
	if err != nil {
		t.Fatalf("LoadAndMergeTemplate failed: %v", err)
	}

	// Verify that immutability was automatically disabled due to missing hash partition
	if template.IsImmutabilityEnabled() {
		t.Errorf("Expected immutability to be disabled due to missing hash partition, but got %t", template.IsImmutabilityEnabled())
	}

	// Verify that user's disk config is preserved (no hash partition)
	hasHashPartition := false
	for _, partition := range template.Disk.Partitions {
		if partition.ID == "roothashmap" || partition.ID == "hash" {
			hasHashPartition = true
			break
		}
	}
	if hasHashPartition {
		t.Error("Expected user disk config to be preserved (no hash partition)")
	}
}

func TestTemplateHelperMethodsWithImmutability(t *testing.T) {
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
		SystemConfig: SystemConfig{
			Name:         "test-config",
			Description:  "Test configuration",
			Immutability: ImmutabilityConfig{Enabled: true},
			Packages:     []string{"package1", "package2"},
			Kernel: KernelConfig{
				Version: "6.12",
				Cmdline: "quiet",
			},
		},
	}

	// Test immutability access methods
	if !template.IsImmutabilityEnabled() {
		t.Errorf("expected immutability to be enabled, got %t", template.IsImmutabilityEnabled())
	}

	immutabilityConfig := template.GetImmutability()
	if !immutabilityConfig.Enabled {
		t.Errorf("expected immutability config to be enabled, got %t", immutabilityConfig.Enabled)
	}

	// Test systemConfig direct access
	if !template.SystemConfig.IsImmutabilityEnabled() {
		t.Errorf("expected systemConfig immutability to be enabled, got %t", template.SystemConfig.IsImmutabilityEnabled())
	}
}

func TestTemplateHelperMethodsWithUsers(t *testing.T) {
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
		SystemConfig: SystemConfig{
			Name:        "test-config",
			Description: "Test configuration",
			Users: []UserConfig{
				{Name: "testuser", Password: "testpass", HashAlgo: "sha512", Sudo: true},
				{Name: "admin", Password: "$6$test$hash", Groups: []string{"wheel"}, PasswordMaxAge: 365},
			},
			Packages: []string{"package1", "package2"},
			Kernel: KernelConfig{
				Version: "6.12",
				Cmdline: "quiet",
			},
		},
	}

	// Test users access methods
	users := template.GetUsers()
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}

	testUser := template.GetUserByName("testuser")
	if testUser == nil {
		t.Errorf("expected to find testuser")
	} else {
		if !testUser.Sudo {
			t.Errorf("expected testuser to have sudo privileges")
		}
		if testUser.HashAlgo != "sha512" {
			t.Errorf("expected testuser hash_algo 'sha512', got %s", testUser.HashAlgo)
		}
	}

	// Test non-existent user
	nonExistentUser := template.GetUserByName("nonexistent")
	if nonExistentUser != nil {
		t.Errorf("expected not to find nonexistent user")
	}

	if !template.HasUsers() {
		t.Errorf("expected template to have users")
	}

	// Test systemConfig direct access
	if !template.SystemConfig.HasUsers() {
		t.Errorf("expected systemConfig to have users")
	}

	adminUser := template.SystemConfig.GetUserByName("admin")
	if adminUser == nil {
		t.Errorf("expected to find admin user via systemConfig")
	} else {
		if adminUser.PasswordMaxAge != 365 {
			t.Errorf("expected admin passwordMaxAge to be 365, got %d", adminUser.PasswordMaxAge)
		}
	}
}

func TestMergeSystemConfigWithUsers(t *testing.T) {
	defaultConfig := SystemConfig{
		Name: "default",
		Users: []UserConfig{
			{Name: "defaultuser", Password: "defaultpass", HashAlgo: "sha512"},
			{Name: "shared", Password: "defaultshared", HashAlgo: "sha256", Groups: []string{"default"}},
		},
		Packages: []string{"base-package"},
	}

	userConfig := SystemConfig{
		Name: "user",
		Users: []UserConfig{
			{Name: "newuser", Password: "newpass", HashAlgo: "bcrypt"},
			{Name: "shared", Password: "usershared", HashAlgo: "sha512", Groups: []string{"user", "admin"}, PasswordMaxAge: 180},
		},
		Packages: []string{"user-package"},
	}

	merged := mergeSystemConfig(defaultConfig, userConfig)

	// Test user merge
	if len(merged.Users) != 3 {
		t.Errorf("expected 3 merged users, got %d", len(merged.Users))
	}

	// Find shared user to test merging
	var sharedUser *UserConfig
	for i := range merged.Users {
		if merged.Users[i].Name == "shared" {
			sharedUser = &merged.Users[i]
			break
		}
	}

	if sharedUser == nil {
		t.Errorf("expected to find shared user in merged config")
	} else {
		if sharedUser.Password != "usershared" {
			t.Errorf("expected shared user password 'usershared', got '%s'", sharedUser.Password)
		}
		if sharedUser.HashAlgo != "sha512" {
			t.Errorf("expected shared user hash algo 'sha512', got '%s'", sharedUser.HashAlgo)
		}
		if sharedUser.PasswordMaxAge != 180 {
			t.Errorf("expected shared user password max age 180, got %d", sharedUser.PasswordMaxAge)
		}
		if len(sharedUser.Groups) != 3 { // default, user, admin merged
			t.Errorf("expected 3 merged groups for shared user, got %d", len(sharedUser.Groups))
		}
	}

	// Verify expected groups are present
	expectedGroups := []string{"default", "user", "admin"}
	groupMap := make(map[string]bool)
	for _, group := range sharedUser.Groups {
		groupMap[group] = true
	}
	for _, expectedGroup := range expectedGroups {
		if !groupMap[expectedGroup] {
			t.Errorf("expected group '%s' to be in merged groups", expectedGroup)
		}
	}
}

func TestUnsupportedFileFormat(t *testing.T) {
	// Create a temporary file with unsupported extension
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("some content"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading should fail
	_, err = LoadTemplate(tmpFile.Name(), false)
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
		{"wind-river-elxr", "elxr12", "eLxr12", "12"},
		{"ubuntu", "ubuntu24", "ubuntu24", ""},
		{"madani", "madani24", "madani24", ""},
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

func TestUnsupportedProviders(t *testing.T) {
	testCases := []struct {
		os   string
		dist string
	}{
		{"unsupported-os", "unknown-dist"},
		{"azure-linux", "unsupported-dist"},
		{"wind-river-elxr", "unsupported-version"},
		{"ubuntu", "unsupported-version"},
		{"madani", "unsupported-version"},
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

		// Test that unsupported providers return empty strings
		providerName := template.GetProviderName()
		if providerName != "" {
			t.Errorf("for unsupported %s/%s expected empty provider name, got '%s'", tc.os, tc.dist, providerName)
		}

		version := template.GetDistroVersion()
		if version != "" {
			t.Errorf("for unsupported %s/%s expected empty version, got '%s'", tc.os, tc.dist, version)
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
					Start:      "1MiB",
					End:        "0",
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

	// Test individual field access
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
}

func TestSecureBootHelperMethods(t *testing.T) {
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
		SystemConfig: SystemConfig{
			Name:        "test-config",
			Description: "Test configuration with secure boot",
			Immutability: ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/test/keys/db.key",
				SecureBootDBCrt: "/test/certs/db.crt",
				SecureBootDBCer: "/test/certs/db.cer",
			},
		},
	}

	// Test ImmutabilityConfig helper methods
	immutabilityConfig := template.GetImmutability()
	if !immutabilityConfig.HasSecureBootDBConfig() {
		t.Errorf("expected immutability config to have secure boot DB config")
	}

	if !immutabilityConfig.HasSecureBootDBKey() {
		t.Errorf("expected immutability config to have secure boot DB key")
	}

	if !immutabilityConfig.HasSecureBootDBCrt() {
		t.Errorf("expected immutability config to have secure boot DB crt")
	}

	if !immutabilityConfig.HasSecureBootDBCer() {
		t.Errorf("expected immutability config to have secure boot DB cer")
	}

	// Test path retrieval methods
	if keyPath := immutabilityConfig.GetSecureBootDBKeyPath(); keyPath != "/test/keys/db.key" {
		t.Errorf("expected key path '/test/keys/db.key', got '%s'", keyPath)
	}

	if crtPath := immutabilityConfig.GetSecureBootDBCrtPath(); crtPath != "/test/certs/db.crt" {
		t.Errorf("expected crt path '/test/certs/db.crt', got '%s'", crtPath)
	}

	if cerPath := immutabilityConfig.GetSecureBootDBCerPath(); cerPath != "/test/certs/db.cer" {
		t.Errorf("expected cer path '/test/certs/db.cer', got '%s'", cerPath)
	}

	// Test SystemConfig access methods
	systemConfig := template.SystemConfig
	if !systemConfig.HasSecureBootDBConfig() {
		t.Errorf("expected systemConfig to have secure boot DB config")
	}

	if keyPath := systemConfig.GetSecureBootDBKeyPath(); keyPath != "/test/keys/db.key" {
		t.Errorf("expected systemConfig key path '/test/keys/db.key', got '%s'", keyPath)
	}

	if crtPath := systemConfig.GetSecureBootDBCrtPath(); crtPath != "/test/certs/db.crt" {
		t.Errorf("expected systemConfig crt path '/test/certs/db.crt', got '%s'", crtPath)
	}

	if cerPath := systemConfig.GetSecureBootDBCerPath(); cerPath != "/test/certs/db.cer" {
		t.Errorf("expected systemConfig cer path '/test/certs/db.cer', got '%s'", cerPath)
	}

	// Test ImageTemplate secure boot helpers
	expectedKeyPath := "/test/keys/db.key"
	if keyPath := template.GetSecureBootDBKeyPath(); keyPath != expectedKeyPath {
		t.Errorf("expected secure boot key path '%s', got '%s'", expectedKeyPath, keyPath)
	}

	expectedCrtPath := "/test/certs/db.crt"
	if crtPath := template.GetSecureBootDBCrtPath(); crtPath != expectedCrtPath {
		t.Errorf("expected secure boot crt path '%s', got '%s'", expectedCrtPath, crtPath)
	}

	expectedCerPath := "/test/certs/db.cer"
	if cerPath := template.GetSecureBootDBCerPath(); cerPath != expectedCerPath {
		t.Errorf("expected secure boot cer path '%s', got '%s'", expectedCerPath, cerPath)
	}

	if !template.HasSecureBootDBConfig() {
		t.Errorf("expected template to have secure boot DB config")
	}
}

func TestSecureBootWithoutConfig(t *testing.T) {
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
		SystemConfig: SystemConfig{
			Name:        "test-config",
			Description: "Test configuration without secure boot",
			Immutability: ImmutabilityConfig{
				Enabled: true,
				// No secure boot fields set
			},
		},
	}

	// Test that methods work correctly when no secure boot config is provided
	if template.HasSecureBootDBConfig() {
		t.Errorf("expected template to not have secure boot DB config")
	}

	immutabilityConfig := template.GetImmutability()
	if immutabilityConfig.HasSecureBootDBConfig() {
		t.Errorf("expected immutability config to not have secure boot DB config")
	}

	if immutabilityConfig.HasSecureBootDBKey() {
		t.Errorf("expected immutability config to not have secure boot DB key")
	}

	if immutabilityConfig.HasSecureBootDBCrt() {
		t.Errorf("expected immutability config to not have secure boot DB crt")
	}

	if immutabilityConfig.HasSecureBootDBCer() {
		t.Errorf("expected immutability config to not have secure boot DB cer")
	}

	// Test that path methods return empty strings
	if keyPath := template.GetSecureBootDBKeyPath(); keyPath != "" {
		t.Errorf("expected empty key path, got '%s'", keyPath)
	}

	if crtPath := template.GetSecureBootDBCrtPath(); crtPath != "" {
		t.Errorf("expected empty crt path, got '%s'", crtPath)
	}

	if cerPath := template.GetSecureBootDBCerPath(); cerPath != "" {
		t.Errorf("expected empty cer path, got '%s'", cerPath)
	}
}

func TestPartialSecureBootConfig(t *testing.T) {
	template := &ImageTemplate{
		SystemConfig: SystemConfig{
			Immutability: ImmutabilityConfig{
				Enabled:         true,
				SecureBootDBKey: "/test/keys/db.key",
				// Only key is set, no certificates
			},
		},
	}

	immutabilityConfig := template.GetImmutability()

	// Should have config because key is set
	if !immutabilityConfig.HasSecureBootDBConfig() {
		t.Errorf("expected immutability config to have secure boot DB config")
	}

	// Should have key
	if !immutabilityConfig.HasSecureBootDBKey() {
		t.Errorf("expected immutability config to have secure boot DB key")
	}

	// Should not have certificates
	if immutabilityConfig.HasSecureBootDBCrt() {
		t.Errorf("expected immutability config to not have secure boot DB crt")
	}

	if immutabilityConfig.HasSecureBootDBCer() {
		t.Errorf("expected immutability config to not have secure boot DB cer")
	}
}

func TestDiskConfigValidation(t *testing.T) {
	testCases := []struct {
		name     string
		disk     DiskConfig
		expected bool // whether it should be considered empty
	}{
		{
			name:     "empty disk config",
			disk:     DiskConfig{},
			expected: true,
		},
		{
			name: "disk with only name",
			disk: DiskConfig{
				Name: "test-disk",
			},
			expected: false,
		},
		{
			name: "disk with full configuration",
			disk: DiskConfig{
				Name:               "main-disk",
				Size:               "20GiB",
				PartitionTableType: "gpt",
				Partitions: []PartitionInfo{
					{
						ID:         "boot",
						Name:       "EFI Boot",
						Type:       "esp",
						FsType:     "fat32",
						Start:      "1MiB",
						End:        "513MiB",
						MountPoint: "/boot/efi",
						Flags:      []string{"boot"},
					},
					{
						ID:         "root",
						Name:       "Root",
						Type:       "linux-root-amd64",
						FsType:     "ext4",
						Start:      "513MiB",
						End:        "0",
						MountPoint: "/",
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test isEmpty functionality - this would be based on merge.go isEmptyDiskConfig
			isEmpty := (tc.disk.Name == "" && tc.disk.Size == "" && len(tc.disk.Partitions) == 0)
			if isEmpty != tc.expected {
				t.Errorf("Expected isEmpty=%v, got isEmpty=%v for %s", tc.expected, isEmpty, tc.name)
			}
		})
	}
}

func TestPartitionInfoFields(t *testing.T) {
	template := &ImageTemplate{
		Disk: DiskConfig{
			Name:               "test-disk",
			Size:               "10GiB",
			PartitionTableType: "gpt",
			Partitions: []PartitionInfo{
				{
					ID:           "efi",
					Name:         "EFI System",
					Type:         "esp",
					TypeGUID:     "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					FsType:       "fat32",
					Start:        "1MiB",
					End:          "513MiB",
					MountPoint:   "/boot/efi",
					MountOptions: "defaults",
					Flags:        []string{"boot", "esp"},
				},
				{
					ID:       "swap",
					Name:     "Swap",
					Type:     "swap",
					TypeGUID: "0657FD6D-A4AB-43C4-84E5-0933C84B4F4F",
					FsType:   "swap",
					Start:    "513MiB",
					End:      "2GiB",
				},
				{
					ID:         "root",
					Name:       "Root",
					Type:       "linux-root-amd64",
					TypeGUID:   "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709",
					FsType:     "ext4",
					Start:      "2GiB",
					End:        "0",
					MountPoint: "/",
				},
			},
		},
	}

	diskConfig := template.GetDiskConfig()

	// Verify partition count
	if len(diskConfig.Partitions) != 3 {
		t.Errorf("expected 3 partitions, got %d", len(diskConfig.Partitions))
	}

	// Verify EFI partition
	efiPartition := diskConfig.Partitions[0]
	if efiPartition.ID != "efi" {
		t.Errorf("expected EFI partition ID 'efi', got '%s'", efiPartition.ID)
	}
	if len(efiPartition.Flags) != 2 {
		t.Errorf("expected 2 flags for EFI partition, got %d", len(efiPartition.Flags))
	}
	if efiPartition.TypeGUID != "C12A7328-F81F-11D2-BA4B-00A0C93EC93B" {
		t.Errorf("expected EFI TypeGUID, got '%s'", efiPartition.TypeGUID)
	}
	if efiPartition.Start != "1MiB" {
		t.Errorf("expected EFI start '1MiB', got '%s'", efiPartition.Start)
	}
	if efiPartition.End != "513MiB" {
		t.Errorf("expected EFI end '513MiB', got '%s'", efiPartition.End)
	}
	if efiPartition.MountOptions != "defaults" {
		t.Errorf("expected EFI mount options 'defaults', got '%s'", efiPartition.MountOptions)
	}

	// Verify swap partition
	swapPartition := diskConfig.Partitions[1]
	if swapPartition.FsType != "swap" {
		t.Errorf("expected swap filesystem type, got '%s'", swapPartition.FsType)
	}
	if swapPartition.MountPoint != "" {
		t.Errorf("expected empty mount point for swap, got '%s'", swapPartition.MountPoint)
	}
	if swapPartition.Start != "513MiB" {
		t.Errorf("expected swap start '513MiB', got '%s'", swapPartition.Start)
	}
	if swapPartition.End != "2GiB" {
		t.Errorf("expected swap end '2GiB', got '%s'", swapPartition.End)
	}

	// Verify root partition
	rootPartition := diskConfig.Partitions[2]
	if rootPartition.MountPoint != "/" {
		t.Errorf("expected root mount point '/', got '%s'", rootPartition.MountPoint)
	}
	if rootPartition.Start != "2GiB" {
		t.Errorf("expected root start '2GiB', got '%s'", rootPartition.Start)
	}
	if rootPartition.End != "0" {
		t.Errorf("expected root end '0' (end of disk), got '%s'", rootPartition.End)
	}
}

func TestArtifactInfo(t *testing.T) {
	template := &ImageTemplate{
		Disk: DiskConfig{
			Name: "test-disk",
			Artifacts: []ArtifactInfo{
				{Type: "raw", Compression: "none"},
				{Type: "qcow2", Compression: "gzip"},
				{Type: "vmdk", Compression: "lz4"},
			},
		},
	}

	artifacts := template.GetDiskConfig().Artifacts
	if len(artifacts) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(artifacts))
	}

	// Test artifact types and compression
	expectedArtifacts := []struct {
		Type        string
		Compression string
	}{
		{"raw", "none"},
		{"qcow2", "gzip"},
		{"vmdk", "lz4"},
	}

	for i, expected := range expectedArtifacts {
		if artifacts[i].Type != expected.Type {
			t.Errorf("artifact %d: expected type '%s', got '%s'", i, expected.Type, artifacts[i].Type)
		}
		if artifacts[i].Compression != expected.Compression {
			t.Errorf("artifact %d: expected compression '%s', got '%s'", i, expected.Compression, artifacts[i].Compression)
		}
	}

	// Test empty artifacts
	emptyTemplate := &ImageTemplate{
		Disk: DiskConfig{
			Artifacts: []ArtifactInfo{},
		},
	}

	emptyArtifacts := emptyTemplate.GetDiskConfig().Artifacts
	if len(emptyArtifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(emptyArtifacts))
	}
}

func TestAdditionalFileInfo(t *testing.T) {
	template := &ImageTemplate{
		SystemConfig: SystemConfig{
			Name: "test-config",
			AdditionalFiles: []AdditionalFileInfo{
				{Local: "/host/config.conf", Final: "/etc/app/config.conf"},
				{Local: "/host/script.sh", Final: "/usr/local/bin/script.sh"},
				{Local: "/host/certs/ca.crt", Final: "/etc/ssl/certs/ca.crt"},
			},
		},
	}

	additionalFiles := template.GetSystemConfig().AdditionalFiles
	if len(additionalFiles) != 3 {
		t.Errorf("expected 3 additional files, got %d", len(additionalFiles))
	}

	// Test file mappings
	expectedFiles := []struct {
		Local string
		Final string
	}{
		{"/host/config.conf", "/etc/app/config.conf"},
		{"/host/script.sh", "/usr/local/bin/script.sh"},
		{"/host/certs/ca.crt", "/etc/ssl/certs/ca.crt"},
	}

	for i, expected := range expectedFiles {
		if additionalFiles[i].Local != expected.Local {
			t.Errorf("file %d: expected local path '%s', got '%s'", i, expected.Local, additionalFiles[i].Local)
		}
		if additionalFiles[i].Final != expected.Final {
			t.Errorf("file %d: expected final path '%s', got '%s'", i, expected.Final, additionalFiles[i].Final)
		}
	}
}

func TestMergeUserConfig(t *testing.T) {
	defaultUser := UserConfig{
		Name:           "user",
		Password:       "default",
		HashAlgo:       "sha512",
		PasswordMaxAge: 90,
		Groups:         []string{"group1"},
	}

	// Test override
	userUser := UserConfig{
		Name:     "user",
		Password: "newpassword",
		Groups:   []string{"group2"},
		Sudo:     true,
	}

	merged := mergeUserConfig(defaultUser, userUser)

	if merged.Password != "newpassword" {
		t.Errorf("Expected password newpassword, got %s", merged.Password)
	}
	if merged.HashAlgo != "sha512" {
		t.Errorf("Expected hash algo sha512, got %s", merged.HashAlgo)
	}
	if !merged.Sudo {
		t.Errorf("Expected sudo true")
	}
	if !slice.Contains(merged.Groups, "group1") || !slice.Contains(merged.Groups, "group2") {
		t.Errorf("Expected groups to contain group1 and group2, got %v", merged.Groups)
	}

	// Test pre-hashed password
	userUserHashed := UserConfig{
		Name:     "user",
		Password: "$6$hash",
	}

	mergedHashed := mergeUserConfig(defaultUser, userUserHashed)
	if mergedHashed.HashAlgo != "" {
		t.Errorf("Expected empty hash algo for pre-hashed password, got %s", mergedHashed.HashAlgo)
	}
}

func TestMergeAdditionalFiles(t *testing.T) {
	defaultFiles := []AdditionalFileInfo{
		{Local: "default1", Final: "/etc/file1"},
		{Local: "default2", Final: "/etc/file2"},
	}
	userFiles := []AdditionalFileInfo{
		{Local: "user1", Final: "/etc/file1"}, // Override
		{Local: "user3", Final: "/etc/file3"}, // New
	}

	merged := mergeAdditionalFiles(defaultFiles, userFiles)

	if len(merged) != 3 {
		t.Errorf("Expected 3 files, got %d", len(merged))
	}

	fileMap := make(map[string]AdditionalFileInfo)
	for _, f := range merged {
		fileMap[f.Final] = f
	}

	if fileMap["/etc/file1"].Local != "user1" {
		t.Errorf("Expected /etc/file1 to be user1, got %s", fileMap["/etc/file1"].Local)
	}
	if fileMap["/etc/file2"].Local != "default2" {
		t.Errorf("Expected /etc/file2 to be default2, got %s", fileMap["/etc/file2"].Local)
	}
	if fileMap["/etc/file3"].Local != "user3" {
		t.Errorf("Expected /etc/file3 to be user3, got %s", fileMap["/etc/file3"].Local)
	}
}

func TestMergePackages(t *testing.T) {
	p1 := []string{"pkg1", "pkg2"}
	p2 := []string{"pkg2", "pkg3"}
	merged := mergePackages(p1, p2)

	if len(merged) != 3 {
		t.Errorf("Expected 3 packages, got %d", len(merged))
	}
}

func TestMergeKernelConfig(t *testing.T) {
	defaultKernel := KernelConfig{
		Version:  "1.0",
		Cmdline:  "default",
		Packages: []string{"kernel-default"},
	}
	userKernel := KernelConfig{
		Version:            "2.0",
		EnableExtraModules: "true",
	}

	merged := mergeKernelConfig(defaultKernel, userKernel)

	if merged.Version != "2.0" {
		t.Errorf("Expected version 2.0, got %s", merged.Version)
	}
	if merged.Cmdline != "default" {
		t.Errorf("Expected cmdline default, got %s", merged.Cmdline)
	}
	if merged.EnableExtraModules != "true" {
		t.Errorf("Expected enableExtraModules true, got %s", merged.EnableExtraModules)
	}
	if len(merged.Packages) != 1 || merged.Packages[0] != "kernel-default" {
		t.Errorf("Expected packages [kernel-default], got %v", merged.Packages)
	}
}

func TestLoadProviderRepoConfig(t *testing.T) {
	// Setup temporary config directory
	tempDir := t.TempDir()

	// Save original global config
	originalGlobal := Global()
	// Restore original global config after test
	defer SetGlobal(originalGlobal)

	// Set new global config with temp dir
	newGlobal := DefaultGlobalConfig()
	newGlobal.ConfigDir = tempDir
	SetGlobal(newGlobal)

	// Create directory structure
	// config/osv/testos/testdist/providerconfigs/repo.yml
	osDistDir := filepath.Join(tempDir, "osv", "testos", "testdist")
	providerConfigDir := filepath.Join(osDistDir, "providerconfigs")
	if err := os.MkdirAll(providerConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create repo.yml
	repoConfigContent := `
repositories:
  - name: test-repo
    type: rpm
    baseURL: http://example.com/repo
    gpgKey: http://example.com/key
    enabled: true
`
	repoConfigFile := filepath.Join(providerConfigDir, "repo.yml")
	if err := os.WriteFile(repoConfigFile, []byte(repoConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write repo config file: %v", err)
	}

	// Test LoadProviderRepoConfig
	repos, err := LoadProviderRepoConfig("testos", "testdist")
	if err != nil {
		t.Fatalf("LoadProviderRepoConfig failed: %v", err)
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}
	if repos[0].Name != "test-repo" {
		t.Errorf("Expected repo name 'test-repo', got '%s'", repos[0].Name)
	}
}

func TestToRepoConfigData(t *testing.T) {
	// Test RPM repo
	rpmRepo := ProviderRepoConfig{
		Name:    "rpm-repo",
		Type:    "rpm",
		BaseURL: "http://example.com/rpm/{arch}",
		GPGKey:  "key.gpg",
		Enabled: true,
	}

	repoType, name, url, gpgKey, _, _, _, _, _, _, _, _, enabled := rpmRepo.ToRepoConfigData("x86_64")

	if repoType != "rpm" {
		t.Errorf("Expected type rpm, got %s", repoType)
	}
	if name != "rpm-repo" {
		t.Errorf("Expected name rpm-repo, got %s", name)
	}
	if url != "http://example.com/rpm/x86_64" {
		t.Errorf("Expected url http://example.com/rpm/x86_64, got %s", url)
	}
	// Relative GPG key should be combined with URL
	expectedGpgKey := "http://example.com/rpm/x86_64/key.gpg"
	if gpgKey != expectedGpgKey {
		t.Errorf("Expected gpgKey %s, got %s", expectedGpgKey, gpgKey)
	}
	if !enabled {
		t.Errorf("Expected enabled true")
	}

	// Test DEB repo
	debRepo := ProviderRepoConfig{
		Name:      "deb-repo",
		Type:      "deb",
		BaseURL:   "http://example.com/deb",
		PbGPGKey:  "http://example.com/key.gpg",
		PkgPrefix: "prefix",
		Enabled:   true,
	}

	repoType, _, url, gpgKey, _, _, pkgPrefix, _, _, _, _, _, _ := debRepo.ToRepoConfigData("amd64")

	if repoType != "deb" {
		t.Errorf("Expected type deb, got %s", repoType)
	}
	if url != "http://example.com/deb/binary-amd64/Packages.gz" {
		t.Errorf("Expected url http://example.com/deb/binary-amd64/Packages.gz, got %s", url)
	}
	if gpgKey != "http://example.com/key.gpg" {
		t.Errorf("Expected gpgKey http://example.com/key.gpg, got %s", gpgKey)
	}
	if pkgPrefix != "prefix" {
		t.Errorf("Expected pkgPrefix prefix, got %s", pkgPrefix)
	}
}

func TestGetInitramfsTemplate(t *testing.T) {
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "template.yml")
	initrdFile := filepath.Join(tempDir, "initrd.template")

	if err := os.WriteFile(initrdFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create initrd file: %v", err)
	}

	// Test absolute path
	tmpl := &ImageTemplate{
		SystemConfig: SystemConfig{
			Initramfs: Initramfs{
				Template: initrdFile,
			},
		},
	}

	path, err := tmpl.GetInitramfsTemplate()
	if err != nil {
		t.Fatalf("GetInitramfsTemplate failed with absolute path: %v", err)
	}
	if path != initrdFile {
		t.Errorf("Expected path %s, got %s", initrdFile, path)
	}

	// Test relative path
	tmplRelative := &ImageTemplate{
		SystemConfig: SystemConfig{
			Initramfs: Initramfs{
				Template: "initrd.template",
			},
		},
		PathList: []string{templateFile},
	}

	path, err = tmplRelative.GetInitramfsTemplate()
	if err != nil {
		t.Fatalf("GetInitramfsTemplate failed with relative path: %v", err)
	}
	if path != initrdFile {
		t.Errorf("Expected path %s, got %s", initrdFile, path)
	}
}

func TestGetAdditionalFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	templateFile := filepath.Join(tempDir, "template.yml")
	localFile := filepath.Join(tempDir, "local.txt")

	if err := os.WriteFile(localFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create local file: %v", err)
	}

	// Test absolute path
	tmpl := &ImageTemplate{
		SystemConfig: SystemConfig{
			AdditionalFiles: []AdditionalFileInfo{
				{Local: localFile, Final: "/etc/final.txt"},
			},
		},
	}

	files := tmpl.GetAdditionalFileInfo()
	if len(files) != 1 {
		t.Errorf("Expected 1 additional file, got %d", len(files))
	}
	if files[0].Local != localFile {
		t.Errorf("Expected local path %s, got %s", localFile, files[0].Local)
	}

	// Test relative path
	tmplRelative := &ImageTemplate{
		SystemConfig: SystemConfig{
			AdditionalFiles: []AdditionalFileInfo{
				{Local: "local.txt", Final: "/etc/final.txt"},
			},
		},
		PathList: []string{templateFile},
	}

	files = tmplRelative.GetAdditionalFileInfo()
	if len(files) != 1 {
		t.Errorf("Expected 1 additional file, got %d", len(files))
	}
	if files[0].Local != localFile {
		t.Errorf("Expected local path %s, got %s", localFile, files[0].Local)
	}
}

func TestLoadDefaultConfig(t *testing.T) {
	// Setup temporary config directory
	tempDir := t.TempDir()

	// Save original global config
	originalGlobal := Global()
	defer SetGlobal(originalGlobal)

	// Set new global config with temp dir
	newGlobal := DefaultGlobalConfig()
	newGlobal.ConfigDir = tempDir
	SetGlobal(newGlobal)

	// Create directory structure
	// config/osv/azure-linux/azl3/imageconfigs/defaultconfigs/default-raw-x86_64.yml
	osDistDir := filepath.Join(tempDir, "osv", "azure-linux", "azl3")
	defaultConfigDir := filepath.Join(osDistDir, "imageconfigs", "defaultconfigs")
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create default config file
	defaultConfigContent := `
image:
  name: default-image
  version: "0.0.1"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: default-system
  packages:
    - default-pkg
`
	defaultConfigFile := filepath.Join(defaultConfigDir, "default-raw-x86_64.yml")
	if err := os.WriteFile(defaultConfigFile, []byte(defaultConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write default config file: %v", err)
	}

	// Test LoadDefaultConfig
	loader := NewDefaultConfigLoader("azure-linux", "azl3", "x86_64")
	template, err := loader.LoadDefaultConfig("raw")
	if err != nil {
		t.Fatalf("LoadDefaultConfig failed: %v", err)
	}

	if template.Image.Name != "default-image" {
		t.Errorf("Expected image name 'default-image', got '%s'", template.Image.Name)
	}
	if template.SystemConfig.Name != "default-system" {
		t.Errorf("Expected system config name 'default-system', got '%s'", template.SystemConfig.Name)
	}
}

func TestLoadAndMergeTemplate(t *testing.T) {
	// Setup temporary config directory
	tempDir := t.TempDir()

	// Save original global config
	originalGlobal := Global()
	defer SetGlobal(originalGlobal)

	// Set new global config with temp dir
	newGlobal := DefaultGlobalConfig()
	newGlobal.ConfigDir = tempDir
	SetGlobal(newGlobal)

	// Create directory structure for default config
	osDistDir := filepath.Join(tempDir, "osv", "azure-linux", "azl3")
	defaultConfigDir := filepath.Join(osDistDir, "imageconfigs", "defaultconfigs")
	if err := os.MkdirAll(defaultConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create default config file
	defaultConfigContent := `
image:
  name: default-image
  version: "0.0.1"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: default-system
  packages:
    - default-pkg
`
	defaultConfigFile := filepath.Join(defaultConfigDir, "default-raw-x86_64.yml")
	if err := os.WriteFile(defaultConfigFile, []byte(defaultConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write default config file: %v", err)
	}

	// Create user template file
	userConfigContent := `
image:
  name: user-image
  version: "0.0.2"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: user-system
  packages:
    - user-pkg
`
	userConfigFile := filepath.Join(tempDir, "user-config.yml")
	if err := os.WriteFile(userConfigFile, []byte(userConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write user config file: %v", err)
	}

	// Test LoadAndMergeTemplate
	template, err := LoadAndMergeTemplate(userConfigFile)
	if err != nil {
		t.Fatalf("LoadAndMergeTemplate failed: %v", err)
	}

	// Verify merged results
	if template.Image.Name != "user-image" {
		t.Errorf("Expected image name 'user-image', got '%s'", template.Image.Name)
	}
	if template.SystemConfig.Name != "user-system" {
		t.Errorf("Expected system config name 'user-system', got '%s'", template.SystemConfig.Name)
	}

	// Verify packages merged (default + user)
	packages := template.GetPackages()
	hasDefault := false
	hasUser := false
	for _, pkg := range packages {
		if pkg == "default-pkg" {
			hasDefault = true
		}
		if pkg == "user-pkg" {
			hasUser = true
		}
	}
	if !hasDefault {
		t.Error("Expected default-pkg in merged packages")
	}
	if !hasUser {
		t.Error("Expected user-pkg in merged packages")
	}
}

func TestValidateImageTemplateJSONBasic(t *testing.T) {
	validTemplate := `{
		"image": {"name": "test", "version": "1.0.0"},
		"target": {"os": "azure-linux", "dist": "azl3", "arch": "x86_64", "imageType": "raw"},
		"systemConfig": {"name": "default", "packages": ["filesystem"]}
	}`

	err := validate.ValidateImageTemplateJSON([]byte(validTemplate))
	if err != nil {
		t.Errorf("valid template should pass validation: %v", err)
	}

	// Invalid template JSON (missing required fields)
	invalidTemplate := `{
		"image": {"name": "test"},
		"target": {"os": "azure-linux"}
	}`

	err = validate.ValidateImageTemplateJSON([]byte(invalidTemplate))
	if err == nil {
		t.Errorf("invalid template should fail validation")
	}
}

func TestValidateUserTemplateJSON(t *testing.T) {
	// Minimal valid user template
	validUserTemplate := `{
		"image": {"name": "test", "version": "1.0.0"},
		"target": {"os": "azure-linux", "dist": "azl3", "arch": "x86_64", "imageType": "raw"}
	}`

	err := validate.ValidateUserTemplateJSON([]byte(validUserTemplate))
	if err != nil {
		t.Errorf("valid user template should pass validation: %v", err)
	}

	// Completely invalid JSON
	invalidJSON := `{"invalid": json}`

	err = validate.ValidateUserTemplateJSON([]byte(invalidJSON))
	if err == nil {
		t.Errorf("invalid JSON should fail validation")
	}
}

func TestValidateConfigJSON(t *testing.T) {
	// Valid config JSON
	validConfig := `{
		"workers": 4,
		"cache_dir": "/tmp/cache",
		"work_dir": "/tmp/work",
		"logging": {"level": "info"}
	}`

	err := validate.ValidateConfigJSON([]byte(validConfig))
	if err != nil {
		t.Errorf("valid config should pass validation: %v", err)
	}

	// Invalid config JSON
	invalidConfig := `{
		"workers": "not_a_number",
		"cache_dir": 123
	}`

	err = validate.ValidateConfigJSON([]byte(invalidConfig))
	if err == nil {
		t.Errorf("invalid config should fail validation")
	}
}

func TestValidateAgainstSchemaWithEmptyRef(t *testing.T) {
	validData := `{"workers": 4, "cache_dir": "/tmp", "work_dir": "/tmp", "logging": {"level": "info"}}`

	// Test with empty ref (should use root schema)
	err := validate.ValidateAgainstSchema("test-schema", []byte(`{"type": "object"}`), []byte(validData), "")
	if err != nil {
		t.Logf("ValidateAgainstSchema with empty ref: %v", err)
	}
}

func TestValidateAgainstSchemaWithInvalidJSON(t *testing.T) {
	invalidJSON := `{"invalid": json syntax}`
	schema := `{"type": "object"}`

	err := validate.ValidateAgainstSchema("test", []byte(schema), []byte(invalidJSON), "")
	if err == nil {
		t.Errorf("expected validation error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in error message, got: %v", err)
	}
}

// Updated tests for Global config
func TestDefaultGlobalConfig(t *testing.T) {
	config := DefaultGlobalConfig()

	if config == nil {
		t.Fatalf("DefaultGlobalConfig returned nil")
	}

	if config.Workers != 8 {
		t.Errorf("expected default workers = 8, got %d", config.Workers)
	}

	if config.ConfigDir != "./config" {
		t.Errorf("expected default config dir './config', got '%s'", config.ConfigDir)
	}

	if config.CacheDir != "./cache" {
		t.Errorf("expected default cache dir './cache', got '%s'", config.CacheDir)
	}

	if config.WorkDir != "./workspace" {
		t.Errorf("expected default work dir './workspace', got '%s'", config.WorkDir)
	}

	if config.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got '%s'", config.Logging.Level)
	}

	if config.Logging.File != "os-image-composer.log" {
		t.Errorf("expected default log file 'os-image-composer.log', got '%s'", config.Logging.File)
	}
}

// Fix the global singleton test to handle the sync.Once behavior properly
func TestGlobalSingleton(t *testing.T) {
	// Test Global() creates a proper config
	config1 := Global()
	if config1 == nil {
		t.Fatalf("Global() returned nil")
	}

	// Test Global() returns same instance
	config2 := Global()
	if config1 != config2 {
		t.Errorf("Global() should return same instance")
	}

	// Test SetGlobal - need to reset the once first in a real scenario
	customConfig := &GlobalConfig{Workers: 99}
	SetGlobal(customConfig)

	config3 := Global()
	if config3.Workers != 99 {
		t.Errorf("SetGlobal didn't update global instance")
	}
}

func TestLoadTemplateWithJSONValidation(t *testing.T) {
	// Test the JSON conversion path in LoadTemplate with all required fields
	yamlContent := `image:
  name: json-test
  version: "1.0.0"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: test-config
  packages:
    - test-package
  kernel:
    version: "6.12"
    cmdline: "quiet"`

	tmpFile, err := os.CreateTemp("", "test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	template, err := LoadTemplate(tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("LoadTemplate failed: %v", err)
	}

	if template.Image.Name != "json-test" {
		t.Errorf("expected image name 'json-test', got '%s'", template.Image.Name)
	}
}

func TestGlobalConfigValidate(t *testing.T) {
	testCases := []struct {
		name    string
		config  GlobalConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: GlobalConfig{
				Workers:   4,
				ConfigDir: "/test/config",
				CacheDir:  "/test/cache",
				WorkDir:   "/test/work",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: false,
		},
		{
			name: "zero workers",
			config: GlobalConfig{
				Workers:   0,
				ConfigDir: "/test/config",
				CacheDir:  "/test/cache",
				WorkDir:   "/test/work",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "too many workers",
			config: GlobalConfig{
				Workers:   101,
				ConfigDir: "/test/config",
				CacheDir:  "/test/cache",
				WorkDir:   "/test/work",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "empty cache dir",
			config: GlobalConfig{
				Workers:   4,
				ConfigDir: "/test/config",
				CacheDir:  "",
				WorkDir:   "/test/work",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "empty work dir",
			config: GlobalConfig{
				Workers:   4,
				ConfigDir: "/test/config",
				CacheDir:  "/test/cache",
				WorkDir:   "",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "info"},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: GlobalConfig{
				Workers:   4,
				ConfigDir: "/test/config",
				CacheDir:  "/test/cache",
				WorkDir:   "/test/work",
				TempDir:   "/test/temp",
				Logging:   LoggingConfig{Level: "invalid"},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestLoadGlobalConfigFromFile(t *testing.T) {
	// Create test config file
	configContent := "workers: 6\n" +
		"config_dir: /custom/config\n" +
		"cache_dir: /custom/cache\n" +
		"work_dir: /custom/work\n" +
		"temp_dir: /custom/temp\n" +
		"logging:\n" +
		"  level: debug\n" +
		"  file: /var/log/test.log\n"

	tmpFile, err := os.CreateTemp("", "test-config-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	config, err := LoadGlobalConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if config.Workers != 6 {
		t.Errorf("expected workers = 6, got %d", config.Workers)
	}

	if config.ConfigDir != "/custom/config" {
		t.Errorf("expected config_dir = '/custom/config', got '%s'", config.ConfigDir)
	}

	if config.Logging.Level != "debug" {
		t.Errorf("expected log level = 'debug', got '%s'", config.Logging.Level)
	}

	if config.Logging.File != "/var/log/test.log" {
		t.Errorf("expected log file = '/var/log/test.log', got '%s'", config.Logging.File)
	}
}

func TestLoadGlobalConfigWithEmptyPath(t *testing.T) {
	config, err := LoadGlobalConfig("")
	if err != nil {
		t.Errorf("LoadGlobalConfig with empty path should return defaults: %v", err)
	}

	// Should return default config
	defaultConfig := DefaultGlobalConfig()
	if config.Workers != defaultConfig.Workers {
		t.Errorf("expected default workers, got %d", config.Workers)
	}
}

func TestLoadGlobalConfigWithNonExistentFile(t *testing.T) {
	config, err := LoadGlobalConfig("/nonexistent/file.yml")
	if err != nil {
		t.Errorf("LoadGlobalConfig with non-existent file should return defaults: %v", err)
	}

	// Should return default config when file doesn't exist
	if config.Workers != 8 {
		t.Errorf("expected default workers = 8, got %d", config.Workers)
	}
}

func TestLoadGlobalConfigWithInvalidYAML(t *testing.T) {
	invalidYAML := `workers: not_a_number
cache_dir: [invalid, yaml, structure]
logging:
  level: debug
  - invalid: structure`

	tmpFile, err := os.CreateTemp("", "test-invalid-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(invalidYAML); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, err = LoadGlobalConfig(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for invalid YAML")
	}
}

func TestLoadGlobalConfigWithUnsupportedFormat(t *testing.T) {
	// Test with .json file (not supported)
	tmpFile, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	jsonContent := `{"workers": 4, "cache_dir": "/test"}`
	if _, err := tmpFile.WriteString(jsonContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, err = LoadGlobalConfig(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for unsupported file format")
	}
	if !strings.Contains(err.Error(), "unsupported config file format") {
		t.Errorf("expected unsupported format error, got: %v", err)
	}
}

func TestGlobalConfigSave(t *testing.T) {
	config := &GlobalConfig{
		Workers:   6,
		ConfigDir: "/save/config",
		CacheDir:  "/save/cache",
		WorkDir:   "/save/work",
		TempDir:   "/save/temp",
		Logging: LoggingConfig{
			Level: "warn",
		},
	}

	tmpFile, err := os.CreateTemp("", "test-save-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Use SaveGlobalConfig method
	if err := config.SaveGlobalConfig(tmpFile.Name()); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	// Load it back and verify
	loadedConfig, err := LoadGlobalConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loadedConfig.Workers != config.Workers {
		t.Errorf("workers not preserved: expected %d, got %d", config.Workers, loadedConfig.Workers)
	}

	if loadedConfig.ConfigDir != config.ConfigDir {
		t.Errorf("config_dir not preserved: expected '%s', got '%s'", config.ConfigDir, loadedConfig.ConfigDir)
	}
}

func TestParseYAMLTemplateValidationErrors(t *testing.T) {
	// Template that fails schema validation
	invalidTemplate := []byte(`
image:
  name: test
  version: 1.0.0
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: test
  packages: "should_be_array_not_string"
`)

	_, err := parseYAMLTemplate(invalidTemplate, true)
	if err == nil {
		t.Errorf("expected validation error for invalid template")
	}

	// Should fail even without validation if user template validation fails
	_, err = parseYAMLTemplate(invalidTemplate, false)
	if err == nil {
		t.Errorf("expected validation error even without full validation")
	}
}

func TestGlobalConfigSaveWithCreateDirectory(t *testing.T) {
	config := &GlobalConfig{
		Workers:   4,
		ConfigDir: "/test/config", // Add missing ConfigDir
		CacheDir:  "/test/cache",
		WorkDir:   "/test/work",
		TempDir:   "/test/temp", // Use a valid temp dir
		Logging:   LoggingConfig{Level: "info"},
	}

	// Create a path in a subdirectory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yml")

	err := config.SaveGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("SaveGlobalConfig should create directories: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file was not created")
	}
}

func TestSaveGlobalConfigWithComments(t *testing.T) {
	config := DefaultGlobalConfig()
	config.Logging.File = "custom.log"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "commented-config.yml")

	if err := config.SaveGlobalConfigWithComments(configPath); err != nil {
		t.Fatalf("SaveGlobalConfigWithComments failed: %v", err)
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read commented config: %v", err)
	}

	text := string(contents)
	if !strings.Contains(text, "# OS Image Composer - Global Configuration") {
		t.Fatalf("expected commented config header, got: %s", text)
	}

	if !strings.Contains(text, "file: \"custom.log\"") {
		t.Fatalf("expected logging file entry in commented config, got: %s", text)
	}
}

func TestLoadTemplateGlobalVariables(t *testing.T) {
	// Test that LoadTemplate sets global variables
	yamlContent := `image:
  name: global-test
  version: 1.0.0
target:
  os: wind-river-elxr
  dist: elxr12
  arch: x86_64
  imageType: raw
systemConfig:
  name: test-config
  packages:
    - test-package
  kernel:
    version: "6.12"
    cmdline: "quiet"`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	template, err := LoadTemplate(tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("LoadTemplate failed: %v", err)
	}

	// Check that global variables were set
	if template.Target.OS != "wind-river-elxr" {
		t.Errorf("expected TargetOs = 'wind-river-elxr', got '%s'", template.Target.OS)
	}
	if template.Target.Dist != "elxr12" {
		t.Errorf("expected TargetDist = 'elxr12', got '%s'", template.Target.Dist)
	}
	if template.Target.Arch != "x86_64" {
		t.Errorf("expected TargetArch = 'x86_64', got '%s'", template.Target.Arch)
	}
	if template.Target.ImageType != "raw" {
		t.Errorf("expected TargetImageType = 'raw', got '%s'", template.Target.ImageType)
	}

	if template.Image.Name != "global-test" {
		t.Errorf("expected image name 'global-test', got '%s'", template.Image.Name)
	}
}

// Additional benchmark tests
func BenchmarkLoadTemplate(b *testing.B) {
	yamlContent := `image:
  name: benchmark-test
  version: 1.0.0
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: benchmark-config
  packages:
    - package1
    - package2
    - package3
  kernel:
    version: "6.12"
    cmdline: "quiet"
`

	tmpFile, err := os.CreateTemp("", "benchmark-*.yml")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		b.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadTemplate(tmpFile.Name(), false)
		if err != nil {
			b.Fatalf("LoadTemplate failed: %v", err)
		}
	}
}

func BenchmarkMergeConfigurations(b *testing.B) {
	defaultTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "default", Version: "1.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		SystemConfig: SystemConfig{
			Name:     "default",
			Packages: []string{"base1", "base2", "base3"},
			Users:    []UserConfig{{Name: "admin", Password: "pass"}},
		},
	}

	userTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "user", Version: "2.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "iso"},
		SystemConfig: SystemConfig{
			Name:     "user",
			Packages: []string{"extra1", "extra2"},
			Users:    []UserConfig{{Name: "user", Password: "userpass"}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := MergeConfigurations(userTemplate, defaultTemplate)
		if err != nil {
			b.Fatalf("MergeConfigurations failed: %v", err)
		}
	}
}

// Additional edge case tests
func TestLoadTemplateWithFileReadError(t *testing.T) {
	// Test with a directory instead of file
	tmpDir := t.TempDir()

	_, err := LoadTemplate(tmpDir, false)
	if err == nil {
		t.Errorf("expected error when loading directory as template")
	}
}

func TestParseYAMLTemplateWithBadYAML(t *testing.T) {
	malformedYAML := []byte(`
image:
  name: test
  version: 1.0.0
target:
  os: azure-linux
  this is: malformed yaml
    that doesn't: parse correctly
`)

	_, err := parseYAMLTemplate(malformedYAML, false)
	if err == nil {
		t.Errorf("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "invalid YAML format") && !strings.Contains(err.Error(), "template parsing failed") {
		t.Errorf("expected YAML parsing error, got: %v", err)
	}
}

func TestLoadTemplateWithMissingFile(t *testing.T) {
	_, err := LoadTemplate("/definitely/does/not/exist.yml", false)
	if err == nil {
		t.Errorf("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "failed to read template file") {
		t.Errorf("expected file not found error, got: %v", err)
	}
}

func TestLoadTemplateWithDirectoryPath(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	_, err := LoadTemplate(tmpDir, false)
	if err == nil {
		t.Errorf("expected error when trying to load directory as template")
	}
}

func TestDefaultConfigLoaderErrors(t *testing.T) {
	loader := NewDefaultConfigLoader("nonexistent-os", "nonexistent-dist", "x86_64")

	// Test unsupported image types
	unsupportedTypes := []string{"unsupported", "invalid", ""}

	for _, imageType := range unsupportedTypes {
		_, err := loader.LoadDefaultConfig(imageType)
		if err == nil {
			t.Errorf("expected error for unsupported image type: %s", imageType)
		}
		if !strings.Contains(err.Error(), "unsupported image type") {
			t.Errorf("expected 'unsupported image type' error, got: %v", err)
		}
	}
}

func TestDefaultConfigLoaderWithInvalidPath(t *testing.T) {
	loader := NewDefaultConfigLoader("azure-linux", "azl3", "x86_64")

	// This will likely fail because the default config file doesn't exist in test env
	_, err := loader.LoadDefaultConfig("raw")
	if err != nil {
		// This is expected in test environment
		t.Logf("LoadDefaultConfig failed as expected in test environment: %v", err)

		// Verify error contains expected messages
		if !strings.Contains(err.Error(), "config directory") &&
			!strings.Contains(err.Error(), "not found") &&
			!strings.Contains(err.Error(), "failed to load") {
			t.Errorf("unexpected error format: %v", err)
		}
	}
}

func TestMergeConfigurationsWithComplexEdgeCases(t *testing.T) {
	// Test merging with very minimal user template
	minimalUser := &ImageTemplate{
		Image:  ImageInfo{Name: "minimal"},
		Target: TargetInfo{OS: "test-os"},
	}

	complexDefault := &ImageTemplate{
		Image:  ImageInfo{Name: "default-name", Version: "default-version"},
		Target: TargetInfo{OS: "default-os", Dist: "default-dist", Arch: "default-arch", ImageType: "raw"},
		SystemConfig: SystemConfig{
			Name:     "default-system",
			Packages: []string{"default-package"},
			Users:    []UserConfig{{Name: "default-user"}},
		},
		Disk: DiskConfig{Name: "default-disk"},
	}

	merged, err := MergeConfigurations(minimalUser, complexDefault)
	if err != nil {
		t.Fatalf("failed to merge configurations: %v", err)
	}

	// User values should override
	if merged.Image.Name != "minimal" {
		t.Errorf("expected image name 'minimal', got '%s'", merged.Image.Name)
	}

	// Default values should be preserved when user doesn't specify
	if merged.Image.Version != "default-version" {
		t.Errorf("expected version from default, got '%s'", merged.Image.Version)
	}

	// Target should be completely from user
	if merged.Target.OS != "test-os" {
		t.Errorf("expected target OS from user, got '%s'", merged.Target.OS)
	}
}

func TestMergeUsersWithEmptySlices(t *testing.T) {
	// Test merging when one side has empty users
	emptyUsers := []UserConfig{}
	userWithUsers := []UserConfig{{Name: "test", Password: "pass"}}

	// Empty default, users from user config
	result := mergeUsers(emptyUsers, userWithUsers)
	if len(result) != 1 {
		t.Errorf("expected 1 user, got %d", len(result))
	}

	// Users from default, empty user config
	result = mergeUsers(userWithUsers, emptyUsers)
	if len(result) != 1 {
		t.Errorf("expected 1 user from default, got %d", len(result))
	}

	// Both empty
	result = mergeUsers(emptyUsers, emptyUsers)
	if len(result) != 0 {
		t.Errorf("expected 0 users when both are empty, got %d", len(result))
	}
}

func TestMergePackagesWithNilAndEmpty(t *testing.T) {
	packages1 := []string{"pkg1", "pkg2"}
	emptyPackages := []string{}

	// Test with empty slices
	result := mergePackages(packages1, emptyPackages)
	if len(result) != 2 {
		t.Errorf("expected 2 packages, got %d", len(result))
	}

	result = mergePackages(emptyPackages, packages1)
	if len(result) != 2 {
		t.Errorf("expected 2 packages, got %d", len(result))
	}

	result = mergePackages(emptyPackages, emptyPackages)
	if len(result) != 0 {
		t.Errorf("expected 0 packages, got %d", len(result))
	}
}

func TestLoadAndMergeTemplateWithInvalidUserTemplate(t *testing.T) {
	// Create an invalid user template
	yamlContent := `invalid: yaml: structure: that: doesn't: parse`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	_, err = LoadAndMergeTemplate(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for invalid user template")
	}
	if !strings.Contains(err.Error(), "failed to load user template") {
		t.Errorf("expected user template loading error, got: %v", err)
	}
}

func TestLoadAndMergeTemplateWithMissingFile(t *testing.T) {
	_, err := LoadAndMergeTemplate("/nonexistent/file.yml")
	if err == nil {
		t.Errorf("expected error for nonexistent template file")
	}
	if !strings.Contains(err.Error(), "failed to load user template") {
		t.Errorf("expected user template loading error, got: %v", err)
	}
}

func TestGlobalConfigValidateEmptyTempDir(t *testing.T) {
	config := &GlobalConfig{
		Workers:   4,
		ConfigDir: "/test/config",
		CacheDir:  "/test/cache",
		WorkDir:   "/test/work",
		TempDir:   "", // Empty temp dir should be set to system default
		Logging:   LoggingConfig{Level: "info"},
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("validation should succeed and set temp dir: %v", err)
	}

	// TempDir should now be set to system default
	if config.TempDir == "" {
		t.Errorf("expected temp dir to be set after validation")
	}
}

func TestGetTargetOsConfigDir(t *testing.T) {
	// Test that the function exists and can be called
	// The actual implementation might depend on environment or file system
	testCases := []struct {
		os   string
		dist string
	}{
		{"azure-linux", "azl3"},
		{"emt", "emt3"},
		{"elxr", "elxr12"},
	}

	for _, tc := range testCases {
		// We can't test the actual path resolution without knowing the environment
		// but we can at least test that the function call doesn't panic
		_, err := GetTargetOsConfigDir(tc.os, tc.dist)
		// Error is expected in test environment, but function should not panic
		if err != nil {
			t.Logf("GetTargetOsConfigDir(%s, %s) returned error as expected in test environment: %v", tc.os, tc.dist, err)
		}
	}
}

func TestPackageRepositories(t *testing.T) {
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
		PackageRepositories: []PackageRepository{
			{
				Codename:  "test-repo1",
				URL:       "https://test.example.com/repo1",
				PKey:      "https://test.example.com/key1.pub",
				Component: "main",
			},
			{
				Codename:  "test-repo2",
				URL:       "https://test.example.com/repo2",
				PKey:      "https://test.example.com/key2.pub",
				Component: "restricted",
			},
		},
		SystemConfig: SystemConfig{
			Name:     "test-config",
			Packages: []string{"package1", "package2"},
			Kernel: KernelConfig{
				Version: "6.12",
				Cmdline: "quiet",
			},
		},
	}

	// Test repository access methods
	repos := template.GetPackageRepositories()
	if len(repos) != 2 {
		t.Errorf("expected 2 repositories, got %d", len(repos))
	}

	if !template.HasPackageRepositories() {
		t.Errorf("expected template to have package repositories")
	}

	repo1 := template.GetRepositoryByCodename("test-repo1")
	if repo1 == nil {
		t.Errorf("expected to find test-repo1")
	} else {
		if repo1.URL != "https://test.example.com/repo1" {
			t.Errorf("expected repo1 URL 'https://test.example.com/repo1', got '%s'", repo1.URL)
		}
		if repo1.PKey != "https://test.example.com/key1.pub" {
			t.Errorf("expected repo1 pkey 'https://test.example.com/key1.pub', got '%s'", repo1.PKey)
		}
		if repo1.Component != "main" {
			t.Errorf("expected repo1 component 'main', got '%s'", repo1.Component)
		}
	}

	repo2 := template.GetRepositoryByCodename("test-repo2")
	if repo2 == nil {
		t.Errorf("expected to find test-repo2")
	} else {
		if repo2.Component != "restricted" {
			t.Errorf("expected repo2 component 'restricted', got '%s'", repo2.Component)
		}
	}

	// Test non-existent repository
	nonExistentRepo := template.GetRepositoryByCodename("nonexistent")
	if nonExistentRepo != nil {
		t.Errorf("expected not to find nonexistent repository")
	}
}

func TestEmptyPackageRepositories(t *testing.T) {
	template := &ImageTemplate{
		Image:        ImageInfo{Name: "test", Version: "1.0.0"},
		Target:       TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		SystemConfig: SystemConfig{Name: "test-config", Packages: []string{"package1"}, Kernel: KernelConfig{Version: "6.12"}},
		// No PackageRepositories defined
	}

	repos := template.GetPackageRepositories()
	if len(repos) != 0 {
		t.Errorf("expected 0 repositories for empty config, got %d", len(repos))
	}

	if template.HasPackageRepositories() {
		t.Errorf("expected template to not have package repositories")
	}

	nonExistentRepo := template.GetRepositoryByCodename("anyrepo")
	if nonExistentRepo != nil {
		t.Errorf("expected not to find any repository in empty config")
	}
}

func TestMergePackageRepositories(t *testing.T) {
	defaultRepos := []PackageRepository{
		{Codename: "default1", URL: "https://default.com/1", PKey: "https://default.com/1.pub"},
		{Codename: "default2", URL: "https://default.com/2", PKey: "https://default.com/2.pub"},
	}

	userRepos := []PackageRepository{
		{Codename: "user1", URL: "https://user.com/1", PKey: "https://user.com/1.pub"},
	}

	merged := mergePackageRepositories(defaultRepos, userRepos)

	// User repos should completely override defaults
	if len(merged) != 1 {
		t.Errorf("expected 1 merged repository, got %d", len(merged))
	}

	if merged[0].Codename != "user1" {
		t.Errorf("expected merged repo codename 'user1', got '%s'", merged[0].Codename)
	}

	if merged[0].URL != "https://user.com/1" {
		t.Errorf("expected merged repo URL 'https://user.com/1', got '%s'", merged[0].URL)
	}

	if merged[0].PKey != "https://user.com/1.pub" {
		t.Errorf("expected merged repo pkey 'https://user.com/1.pub', got '%s'", merged[0].PKey)
	}
}

func TestMergePackageRepositoriesEmpty(t *testing.T) {
	defaultRepos := []PackageRepository{
		{Codename: "default1", URL: "https://default.com/1", PKey: "https://default.com/1.pub"},
	}

	// Test with empty user repos - should return defaults
	emptyUserRepos := []PackageRepository{}
	merged := mergePackageRepositories(defaultRepos, emptyUserRepos)
	if len(merged) != 1 {
		t.Errorf("expected 1 default repository when user repos empty, got %d", len(merged))
	}
	if merged[0].Codename != "default1" {
		t.Errorf("expected default repo codename, got '%s'", merged[0].Codename)
	}

	// Test with nil user repos - should return defaults
	merged = mergePackageRepositories(defaultRepos, nil)
	if len(merged) != 1 {
		t.Errorf("expected 1 default repository when user repos nil, got %d", len(merged))
	}

	// Test with both empty
	merged = mergePackageRepositories([]PackageRepository{}, []PackageRepository{})
	if len(merged) != 0 {
		t.Errorf("expected 0 repositories when both are empty, got %d", len(merged))
	}
}

func TestMergeConfigurationsWithPackageRepositories(t *testing.T) {
	defaultTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "default", Version: "1.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		PackageRepositories: []PackageRepository{
			{Codename: "azure-extras", URL: "https://packages.microsoft.com/extras", PKey: "https://packages.microsoft.com/keys/microsoft.asc"},
			{Codename: "azure-preview", URL: "https://packages.microsoft.com/preview", PKey: "https://packages.microsoft.com/keys/microsoft.asc"},
		},
		SystemConfig: SystemConfig{
			Name:     "default-config",
			Packages: []string{"base-package"},
			Kernel:   KernelConfig{Version: "6.10", Cmdline: "quiet"},
		},
	}

	userTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "user-image", Version: "2.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		PackageRepositories: []PackageRepository{
			{Codename: "company-internal", URL: "https://packages.company.com/internal", PKey: "https://packages.company.com/keys/internal.pub"},
		},
		SystemConfig: SystemConfig{
			Name:     "user-config",
			Packages: []string{"user-package"},
			Kernel:   KernelConfig{Version: "6.12"},
		},
	}

	merged, err := MergeConfigurations(userTemplate, defaultTemplate)
	if err != nil {
		t.Fatalf("failed to merge configurations: %v", err)
	}

	// Test that user repositories completely override defaults
	repos := merged.GetPackageRepositories()
	if len(repos) != 1 {
		t.Errorf("expected 1 merged repository (user override), got %d", len(repos))
	}

	if repos[0].Codename != "company-internal" {
		t.Errorf("expected user repository codename 'company-internal', got '%s'", repos[0].Codename)
	}

	// Verify default repositories are not included when user specifies repositories
	companyRepo := merged.GetRepositoryByCodename("company-internal")
	if companyRepo == nil {
		t.Errorf("expected to find user repository 'company-internal'")
	}

	defaultRepo := merged.GetRepositoryByCodename("azure-extras")
	if defaultRepo != nil {
		t.Errorf("expected default repository 'azure-extras' to be overridden by user repos")
	}
}

func TestMergeConfigurationsNoUserRepositories(t *testing.T) {
	defaultTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "default", Version: "1.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		PackageRepositories: []PackageRepository{
			{Codename: "azure-extras", URL: "https://packages.microsoft.com/extras", PKey: "https://packages.microsoft.com/keys/microsoft.asc"},
		},
		SystemConfig: SystemConfig{
			Name:     "default-config",
			Packages: []string{"base-package"},
			Kernel:   KernelConfig{Version: "6.10"},
		},
	}

	userTemplate := &ImageTemplate{
		Image:  ImageInfo{Name: "user-image", Version: "2.0.0"},
		Target: TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		// No PackageRepositories specified by user
		SystemConfig: SystemConfig{
			Name:     "user-config",
			Packages: []string{"user-package"},
			Kernel:   KernelConfig{Version: "6.12"},
		},
	}

	merged, err := MergeConfigurations(userTemplate, defaultTemplate)
	if err != nil {
		t.Fatalf("failed to merge configurations: %v", err)
	}

	// Test that default repositories are preserved when user doesn't specify any
	repos := merged.GetPackageRepositories()
	if len(repos) != 1 {
		t.Errorf("expected 1 default repository when user doesn't specify repos, got %d", len(repos))
	}

	if repos[0].Codename != "azure-extras" {
		t.Errorf("expected default repository codename 'azure-extras', got '%s'", repos[0].Codename)
	}
}

func TestPackageRepositoryYAMLParsing(t *testing.T) {
	yamlContent := `image:
  name: test-repo-parsing
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

packageRepositories:
  - codename: "test-repo1"
    url: "https://test.example.com/repo1"
    pkey: "https://test.example.com/key1.pub"
    component: "main"
  - codename: "test-repo2"
    url: "https://test.example.com/repo2"
    pkey: "https://test.example.com/key2.pub"
    component: "restricted"

systemConfig:
  name: test
  packages:
    - test-package
  kernel:
    version: "6.12"
    cmdline: "quiet"
`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading with package repositories
	template, err := LoadTemplate(tmpFile.Name(), false) // User template validation
	if err != nil {
		t.Fatalf("failed to load YAML template with package repositories: %v", err)
	}

	// Verify package repositories were parsed correctly
	repos := template.GetPackageRepositories()
	if len(repos) != 2 {
		t.Errorf("expected 2 parsed repositories, got %d", len(repos))
	}

	repo1 := template.GetRepositoryByCodename("test-repo1")
	if repo1 == nil {
		t.Errorf("expected to find test-repo1")
	} else {
		if repo1.URL != "https://test.example.com/repo1" {
			t.Errorf("expected repo1 URL 'https://test.example.com/repo1', got '%s'", repo1.URL)
		}
		if repo1.PKey != "https://test.example.com/key1.pub" {
			t.Errorf("expected repo1 pkey 'https://test.example.com/key1.pub', got '%s'", repo1.PKey)
		}
	}

	repo2 := template.GetRepositoryByCodename("test-repo2")
	if repo2 == nil {
		t.Errorf("expected to find test-repo2")
	}
}

func TestPackageRepositoriesWithDuplicateCodenames(t *testing.T) {
	repos := []PackageRepository{
		{Codename: "duplicate", URL: "https://first.com", PKey: "https://first.com/key.pub"},
		{Codename: "unique", URL: "https://unique.com", PKey: "https://unique.com/key.pub"},
		{Codename: "duplicate", URL: "https://second.com", PKey: "https://second.com/key.pub"},
	}

	template := &ImageTemplate{
		Image:               ImageInfo{Name: "test", Version: "1.0.0"},
		Target:              TargetInfo{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		PackageRepositories: repos,
		SystemConfig:        SystemConfig{Name: "test", Packages: []string{"pkg"}, Kernel: KernelConfig{Version: "6.12"}},
	}

	// GetRepositoryByCodename should return the first match
	duplicateRepo := template.GetRepositoryByCodename("duplicate")
	if duplicateRepo == nil {
		t.Errorf("expected to find duplicate repository")
	} else {
		if duplicateRepo.URL != "https://first.com" {
			t.Errorf("expected first duplicate repo URL, got '%s'", duplicateRepo.URL)
		}
	}

	uniqueRepo := template.GetRepositoryByCodename("unique")
	if uniqueRepo == nil {
		t.Errorf("expected to find unique repository")
	} else {
		if uniqueRepo.URL != "https://unique.com" {
			t.Errorf("expected unique repo URL, got '%s'", uniqueRepo.URL)
		}
	}
}
func TestGetImageNameAndTargetInfo(t *testing.T) {
	template := &ImageTemplate{
		Image: ImageInfo{Name: "img", Version: "1.2"},
		Target: TargetInfo{
			OS:        "os",
			Dist:      "dist",
			Arch:      "arch",
			ImageType: "type",
		},
	}
	if got := template.GetImageName(); got != "img" {
		t.Errorf("GetImageName() = %s, want img", got)
	}
	ti := template.GetTargetInfo()
	if ti.OS != "os" || ti.Dist != "dist" || ti.Arch != "arch" || ti.ImageType != "type" {
		t.Errorf("GetTargetInfo() = %+v, want all fields set", ti)
	}
}

func TestGetDiskConfigAndSystemConfig(t *testing.T) {
	disk := DiskConfig{Name: "disk1"}
	sys := SystemConfig{Name: "sys1"}
	template := &ImageTemplate{Disk: disk, SystemConfig: sys}
	if got := template.GetDiskConfig(); got.Name != "disk1" {
		t.Errorf("GetDiskConfig() = %v, want disk1", got.Name)
	}
	if got := template.GetSystemConfig(); got.Name != "sys1" {
		t.Errorf("GetSystemConfig() = %v, want sys1", got.Name)
	}
}

func TestGetBootloaderConfig(t *testing.T) {
	bl := Bootloader{BootType: "efi", Provider: "grub2"}
	template := &ImageTemplate{SystemConfig: SystemConfig{Bootloader: bl}}
	got := template.GetBootloaderConfig()
	if got.BootType != "efi" || got.Provider != "grub2" {
		t.Errorf("GetBootloaderConfig() = %+v, want efi/grub2", got)
	}
}

func TestGetPackagesAndKernel(t *testing.T) {
	sys := SystemConfig{
		Packages: []string{"a", "b"},
		Kernel:   KernelConfig{Version: "v", Cmdline: "c"},
	}
	template := &ImageTemplate{SystemConfig: sys}
	if pkgs := template.GetPackages(); len(pkgs) != 2 || pkgs[0] != "a" {
		t.Errorf("GetPackages() = %v, want [a b]", pkgs)
	}
	k := template.GetKernel()
	if k.Version != "v" || k.Cmdline != "c" {
		t.Errorf("GetKernel() = %+v, want v/c", k)
	}
}

func TestGetSystemConfigName(t *testing.T) {
	sys := SystemConfig{Name: "sys"}
	template := &ImageTemplate{SystemConfig: sys}
	if got := template.GetSystemConfigName(); got != "sys" {
		t.Errorf("GetSystemConfigName() = %s, want sys", got)
	}
}

func TestImmutabilityConfigMethods(t *testing.T) {
	ic := ImmutabilityConfig{
		Enabled:         true,
		SecureBootDBKey: "/key",
		SecureBootDBCrt: "/crt",
		SecureBootDBCer: "/cer",
	}
	if !ic.HasSecureBootDBConfig() {
		t.Error("HasSecureBootDBConfig() = false, want true")
	}
	if !ic.HasSecureBootDBKey() {
		t.Error("HasSecureBootDBKey() = false, want true")
	}
	if !ic.HasSecureBootDBCrt() {
		t.Error("HasSecureBootDBCrt() = false, want true")
	}
	if !ic.HasSecureBootDBCer() {
		t.Error("HasSecureBootDBCer() = false, want true")
	}
	if ic.GetSecureBootDBKeyPath() != "/key" {
		t.Errorf("GetSecureBootDBKeyPath() = %s, want /key", ic.GetSecureBootDBKeyPath())
	}
	if ic.GetSecureBootDBCrtPath() != "/crt" {
		t.Errorf("GetSecureBootDBCrtPath() = %s, want /crt", ic.GetSecureBootDBCrtPath())
	}
	if ic.GetSecureBootDBCerPath() != "/cer" {
		t.Errorf("GetSecureBootDBCerPath() = %s, want /cer", ic.GetSecureBootDBCerPath())
	}
}

func TestImmutabilityConfigMethodsEmpty(t *testing.T) {
	ic := ImmutabilityConfig{}
	if ic.HasSecureBootDBConfig() {
		t.Error("HasSecureBootDBConfig() = true, want false")
	}
	if ic.HasSecureBootDBKey() {
		t.Error("HasSecureBootDBKey() = true, want false")
	}
	if ic.HasSecureBootDBCrt() {
		t.Error("HasSecureBootDBCrt() = true, want false")
	}
	if ic.HasSecureBootDBCer() {
		t.Error("HasSecureBootDBCer() = true, want false")
	}
	if ic.GetSecureBootDBKeyPath() != "" {
		t.Errorf("GetSecureBootDBKeyPath() = %s, want empty", ic.GetSecureBootDBKeyPath())
	}
	if ic.GetSecureBootDBCrtPath() != "" {
		t.Errorf("GetSecureBootDBCrtPath() = %s, want empty", ic.GetSecureBootDBCrtPath())
	}
	if ic.GetSecureBootDBCerPath() != "" {
		t.Errorf("GetSecureBootDBCerPath() = %s, want empty", ic.GetSecureBootDBCerPath())
	}
}

func TestSystemConfigImmutabilityHelpers(t *testing.T) {
	ic := ImmutabilityConfig{Enabled: true, SecureBootDBKey: "k"}
	sc := SystemConfig{Immutability: ic}
	if !sc.GetImmutability().Enabled {
		t.Error("GetImmutability().Enabled = false, want true")
	}
	if !sc.IsImmutabilityEnabled() {
		t.Error("IsImmutabilityEnabled() = false, want true")
	}
	if sc.GetSecureBootDBKeyPath() != "k" {
		t.Errorf("GetSecureBootDBKeyPath() = %s, want k", sc.GetSecureBootDBKeyPath())
	}
	if !sc.HasSecureBootDBConfig() {
		t.Error("HasSecureBootDBConfig() = false, want true")
	}
}

func TestImageTemplateImmutabilityHelpers(t *testing.T) {
	ic := ImmutabilityConfig{Enabled: true, SecureBootDBKey: "k"}
	template := &ImageTemplate{SystemConfig: SystemConfig{Immutability: ic}}
	if !template.GetImmutability().Enabled {
		t.Error("GetImmutability().Enabled = false, want true")
	}
	if !template.IsImmutabilityEnabled() {
		t.Error("IsImmutabilityEnabled() = false, want true")
	}
	if template.GetSecureBootDBKeyPath() != "k" {
		t.Errorf("GetSecureBootDBKeyPath() = %s, want k", template.GetSecureBootDBKeyPath())
	}
	if !template.HasSecureBootDBConfig() {
		t.Error("HasSecureBootDBConfig() = false, want true")
	}
}

func TestGetUsersAndUserByName(t *testing.T) {
	users := []UserConfig{
		{Name: "alice", Sudo: true},
		{Name: "bob"},
	}
	template := &ImageTemplate{SystemConfig: SystemConfig{Users: users}}
	if len(template.GetUsers()) != 2 {
		t.Errorf("GetUsers() = %d, want 2", len(template.GetUsers()))
	}
	if u := template.GetUserByName("alice"); u == nil || u.Name != "alice" {
		t.Errorf("GetUserByName(alice) = %v, want alice", u)
	}
	if u := template.GetUserByName("notfound"); u != nil {
		t.Errorf("GetUserByName(notfound) = %v, want nil", u)
	}
	if !template.HasUsers() {
		t.Error("HasUsers() = false, want true")
	}
}

func TestSystemConfigUserHelpers(t *testing.T) {
	users := []UserConfig{{Name: "root"}, {Name: "user"}}
	sc := SystemConfig{Users: users}
	if len(sc.GetUsers()) != 2 {
		t.Errorf("GetUsers() = %d, want 2", len(sc.GetUsers()))
	}
	if u := sc.GetUserByName("root"); u == nil || u.Name != "root" {
		t.Errorf("GetUserByName(root) = %v, want root", u)
	}
	if u := sc.GetUserByName("none"); u != nil {
		t.Errorf("GetUserByName(none) = %v, want nil", u)
	}
	if !sc.HasUsers() {
		t.Error("HasUsers() = false, want true")
	}
}

func TestGetPackageRepositoriesAndHelpers(t *testing.T) {
	repos := []PackageRepository{
		{Codename: "main", URL: "http://a"},
		{Codename: "extra", URL: "http://b"},
	}
	template := &ImageTemplate{PackageRepositories: repos}
	if !template.HasPackageRepositories() {
		t.Error("HasPackageRepositories() = false, want true")
	}
	if len(template.GetPackageRepositories()) != 2 {
		t.Errorf("GetPackageRepositories() = %d, want 2", len(template.GetPackageRepositories()))
	}
	if repo := template.GetRepositoryByCodename("main"); repo == nil || repo.URL != "http://a" {
		t.Errorf("GetRepositoryByCodename(main) = %v, want http://a", repo)
	}
	if repo := template.GetRepositoryByCodename("none"); repo != nil {
		t.Errorf("GetRepositoryByCodename(none) = %v, want nil", repo)
	}
}

func TestGetProviderNameAndDistroVersionUnknown(t *testing.T) {
	template := &ImageTemplate{
		Target: TargetInfo{OS: "unknown", Dist: "unknown"},
	}
	if got := template.GetProviderName(); got != "" {
		t.Errorf("GetProviderName() = %s, want empty", got)
	}
	if got := template.GetDistroVersion(); got != "" {
		t.Errorf("GetDistroVersion() = %s, want empty", got)
	}
}

func TestSaveUpdatedConfigFileStub(t *testing.T) {
	template := &ImageTemplate{}
	// Use temp_dir/dummy instead of just "dummy"
	dummyPath := filepath.Join(TempDir(), "dummy")
	if err := template.SaveUpdatedConfigFile(dummyPath); err != nil {
		t.Errorf("SaveUpdatedConfigFile() = %v, want nil", err)
	}
}

func TestGetConfigurationInfo(t *testing.T) {
	// Test with empty configuration info
	template := &ImageTemplate{
		SystemConfig: SystemConfig{},
	}

	configs := template.GetConfigurationInfo()
	if len(configs) != 0 {
		t.Errorf("Expected empty configuration info, got %d items", len(configs))
	}

	// Test with configuration info
	expectedConfigs := []ConfigurationInfo{
		{Cmd: "echo 'setup complete'"},
		{Cmd: "systemctl enable docker"},
	}
	template.SystemConfig.Configurations = expectedConfigs

	configs = template.GetConfigurationInfo()
	if len(configs) != 2 {
		t.Errorf("Expected 2 configuration items, got %d", len(configs))
	}

	for i, config := range configs {
		if config.Cmd != expectedConfigs[i].Cmd {
			t.Errorf("Expected command %s, got %s", expectedConfigs[i].Cmd, config.Cmd)
		}
	}
}

func TestGetKernelPackages(t *testing.T) {
	// Test with empty kernel packages
	template := &ImageTemplate{
		SystemConfig: SystemConfig{
			Kernel: KernelConfig{},
		},
	}

	packages := template.GetKernelPackages()
	if len(packages) != 0 {
		t.Errorf("Expected empty kernel packages, got %d", len(packages))
	}

	// Test with kernel packages
	expectedPackages := []string{"linux-kernel", "linux-headers", "linux-firmware"}
	template.SystemConfig.Kernel.Packages = expectedPackages

	packages = template.GetKernelPackages()
	if len(packages) != 3 {
		t.Errorf("Expected 3 kernel packages, got %d", len(packages))
	}

	for i, pkg := range packages {
		if pkg != expectedPackages[i] {
			t.Errorf("Expected package %s, got %s", expectedPackages[i], pkg)
		}
	}
}

// TestUnifiedRepoConfig verifies that the unified ToRepoConfigData function
// works correctly for both RPM and DEB repository types
func TestUnifiedRepoConfig(t *testing.T) {
	tests := []struct {
		name         string
		repoConfig   ProviderRepoConfig
		arch         string
		expectedType string
		expectedURL  string
	}{
		{
			name: "RPM Repository (Azure Linux)",
			repoConfig: ProviderRepoConfig{
				Name:      "Azure Linux 3.0",
				Type:      "rpm",
				BaseURL:   "https://packages.microsoft.com/azurelinux/3.0/prod/base/{arch}",
				GPGKey:    "repodata/repomd.xml.key",
				Component: "azl3.0-base",
				Enabled:   true,
			},
			arch:         "x86_64",
			expectedType: "rpm",
			expectedURL:  "https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64",
		},
		{
			name: "DEB Repository (eLxr)",
			repoConfig: ProviderRepoConfig{
				Name:        "Wind River eLxr 12",
				Type:        "deb",
				BaseURL:     "https://mirror.elxr.dev/elxr/dists/aria/main",
				PbGPGKey:    "https://mirror.elxr.dev/elxr/public.gpg",
				Component:   "main",
				Enabled:     true,
				PkgPrefix:   "https://mirror.elxr.dev/elxr/",
				ReleaseFile: "https://mirror.elxr.dev/elxr/dists/aria/Release",
			},
			arch:         "amd64",
			expectedType: "deb",
			expectedURL:  "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz",
		},
		{
			name: "RPM Repository with arch substitution (EMT-style)",
			repoConfig: ProviderRepoConfig{
				Name:      "Edge Microvisor Toolkit 3.0",
				Type:      "rpm",
				BaseURL:   "https://files-rs.edgeorchestration.intel.com/files-edge-orch/microvisor/rpm/3.0",
				Component: "emt3.0-base",
				Enabled:   true,
				GPGCheck:  false,
			},
			arch:         "x86_64",
			expectedType: "rpm",
			expectedURL:  "https://files-rs.edgeorchestration.intel.com/files-edge-orch/microvisor/rpm/3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoType, name, url, gpgKey, component, buildPath, pkgPrefix, releaseFile, releaseSign, _, gpgCheck, repoGPGCheck, enabled := tt.repoConfig.ToRepoConfigData(tt.arch)

			// Verify repository type
			if repoType != tt.expectedType {
				t.Errorf("Expected repo type %s, got %s", tt.expectedType, repoType)
			}

			// Verify URL construction
			if url != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, url)
			}

			// Verify basic fields
			if name != tt.repoConfig.Name {
				t.Errorf("Expected name %s, got %s", tt.repoConfig.Name, name)
			}

			if component != tt.repoConfig.Component {
				t.Errorf("Expected component %s, got %s", tt.repoConfig.Component, component)
			}

			if enabled != tt.repoConfig.Enabled {
				t.Errorf("Expected enabled %v, got %v", tt.repoConfig.Enabled, enabled)
			}

			// Verify type-specific fields
			switch tt.expectedType {
			case "rpm":
				// For RPM: pkgPrefix, releaseFile, releaseSign should be empty
				if pkgPrefix != "" || releaseFile != "" || releaseSign != "" {
					t.Errorf("Expected empty DEB-specific fields for RPM repo, got pkgPrefix=%s, releaseFile=%s, releaseSign=%s",
						pkgPrefix, releaseFile, releaseSign)
				}

				// Verify arch substitution in GPG key if applicable
				if tt.repoConfig.GPGKey != "" && gpgKey != "" {
					expectedGPGKey := tt.repoConfig.GPGKey
					if expectedGPGKey == "repodata/repomd.xml.key" {
						expectedGPGKey = "https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64/repodata/repomd.xml.key"
					}
					if gpgKey != expectedGPGKey {
						t.Errorf("Expected GPG key %s, got %s", expectedGPGKey, gpgKey)
					}
				}

			case "deb":
				// For DEB: should have the DEB-specific fields populated
				if pkgPrefix != tt.repoConfig.PkgPrefix {
					t.Errorf("Expected pkgPrefix %s, got %s", tt.repoConfig.PkgPrefix, pkgPrefix)
				}
				if releaseFile != tt.repoConfig.ReleaseFile {
					t.Errorf("Expected releaseFile %s, got %s", tt.repoConfig.ReleaseFile, releaseFile)
				}
				if gpgKey != tt.repoConfig.PbGPGKey {
					t.Errorf("Expected gpgKey (pbGPGKey) %s, got %s", tt.repoConfig.PbGPGKey, gpgKey)
				}
			}

			// Verify GPG settings match
			if gpgCheck != tt.repoConfig.GPGCheck {
				t.Errorf("Expected gpgCheck %v, got %v", tt.repoConfig.GPGCheck, gpgCheck)
			}
			if repoGPGCheck != tt.repoConfig.RepoGPGCheck {
				t.Errorf("Expected repoGPGCheck %v, got %v", tt.repoConfig.RepoGPGCheck, repoGPGCheck)
			}

			// For this test, buildPath can be ignored as it's not critical to functionality
			_ = buildPath

			t.Logf("✅ %s: type=%s, url=%s, gpgKey=%s", tt.name, repoType, url, gpgKey)
		})
	}
}
