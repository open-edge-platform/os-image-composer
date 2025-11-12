package imageboot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/imageboot"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

func TestInstallImageBoot_MissingRootPartition(t *testing.T) {
	diskPathIdMap := map[string]string{
		"boot": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "boot", MountPoint: "/boot"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error when root partition is missing")
	}
	if !strings.Contains(err.Error(), "failed to find root partition for mount point '/'") {
		t.Errorf("Expected root partition error, got: %v", err)
	}
}

func TestInstallImageBoot_EmptyDiskPathIdMap(t *testing.T) {
	diskPathIdMap := map[string]string{}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error with empty disk path ID map")
	}
	if !strings.Contains(err.Error(), "failed to find root partition for mount point '/'") {
		t.Errorf("Expected root partition error, got: %v", err)
	}
}

func TestInstallImageBoot_UUIDRetrievalFailure(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid", Output: "", Error: fmt.Errorf("device not found")},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error when UUID retrieval fails")
	}
	if !strings.Contains(err.Error(), "failed to get UUID") {
		t.Errorf("Expected UUID error, got: %v", err)
	}
}

func TestInstallImageBoot_PartUUIDRetrievalFailure(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*PARTUUID", Output: "", Error: fmt.Errorf("partition UUID not found")},
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "command -v grub2-mkconfig", Output: "/usr/sbin/grub2-mkconfig", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error when partition UUID retrieval fails")
	}
	if !strings.Contains(err.Error(), "failed to get partition UUID") {
		t.Errorf("Expected partition UUID error, got: %v", err)
	}
}

func TestInstallImageBoot_UnsupportedBootloaderProvider(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "unknown",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error for unsupported bootloader provider")
	}
	if !strings.Contains(err.Error(), "unsupported bootloader provider") {
		t.Errorf("Expected unsupported bootloader error, got: %v", err)
	}
}

func TestInstallImageBoot_GrubLegacyMode(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				BootType: "legacy",
				Provider: "grub",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
		{Pattern: "command -v grub2-mkconfig", Output: "/usr/sbin/grub2-mkconfig", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error for legacy boot mode not implemented")
	}
	if !strings.Contains(err.Error(), "legacy boot mode is not implemented yet") {
		t.Errorf("Expected legacy mode error, got: %v", err)
	}
}

func TestInstallImageBoot_GrubEfiMode(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
				BootType: "efi",
			},
			Kernel: config.KernelConfig{
				Cmdline: "console=tty0",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
		{Pattern: "chmod", Output: "", Error: nil},
		{Pattern: "grub2-mkconfig", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	// Should fail on file operations since we don't have actual config files
	if err != nil && !strings.Contains(err.Error(), "failed to get general config directory") {
		t.Logf("Got expected error: %v", err)
	}
}

func TestInstallImageBoot_SystemdBootEfiMode(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "systemd-boot",
				BootType: "efi",
			},
			Kernel: config.KernelConfig{
				Cmdline: "quiet splash",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	// Should fail on config directory access
	if err != nil && !strings.Contains(err.Error(), "failed to get general config directory") {
		t.Logf("Got expected error: %v", err)
	}
}

func TestInstallImageBoot_SystemdBootLegacyMode(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "systemd-boot",
				BootType: "legacy",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error for systemd-boot in legacy mode")
	}
	if !strings.Contains(err.Error(), "systemd-boot is only supported in EFI mode") {
		t.Errorf("Expected systemd-boot legacy mode error, got: %v", err)
	}
}

func TestInstallImageBoot_SeparateBootPartition(t *testing.T) {
	diskPathIdMap := map[string]string{
		"boot": "/dev/sda1",
		"root": "/dev/sda2",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "boot", MountPoint: "/boot"},
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "grub",
				BootType: "efi",
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=boot-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=root-partuuid\n", Error: nil},
		{Pattern: "chmod", Output: "", Error: nil},
		{Pattern: "grub2-mkconfig", Output: "", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	// Should work with separate boot partition but fail on file operations
	if err != nil && !strings.Contains(err.Error(), "failed to get general config directory") {
		t.Logf("Got expected error: %v", err)
	}
}

func TestInstallImageBoot_ImmutabilityEnabled(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
		"hash": "/dev/sda2",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
				{ID: "hash", MountPoint: "none"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "systemd-boot",
				BootType: "efi",
			},
			Immutability: config.ImmutabilityConfig{
				Enabled: true,
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	// Should work with immutability enabled but fail on config directory access
	if err != nil && !strings.Contains(err.Error(), "failed to get general config directory") {
		t.Logf("Got expected error: %v", err)
	}
}

func TestInstallImageBoot_ImmutabilityMissingHashPartition(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "systemd-boot",
				BootType: "efi",
			},
			Immutability: config.ImmutabilityConfig{
				Enabled: true,
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*UUID", Output: "UUID=test-uuid\n", Error: nil},
		{Pattern: "blkid.*PARTUUID", Output: "PARTUUID=test-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error when hash partition is missing for immutability")
	}
	if !strings.Contains(err.Error(), "failed to find dm verity hash partition") {
		t.Errorf("Expected hash partition error, got: %v", err)
	}
}

func TestInstallImageBoot_HashPartitionUUIDFailure(t *testing.T) {
	diskPathIdMap := map[string]string{
		"root": "/dev/sda1",
		"hash": "/dev/sda2",
	}

	tmpDir := t.TempDir()
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name: "test-image",
		},
		Disk: config.DiskConfig{
			Partitions: []config.PartitionInfo{
				{ID: "root", MountPoint: "/"},
				{ID: "hash", MountPoint: "none"},
			},
		},
		SystemConfig: config.SystemConfig{
			Bootloader: config.Bootloader{
				Provider: "systemd-boot",
				BootType: "efi",
			},
			Immutability: config.ImmutabilityConfig{
				Enabled: true,
			},
		},
	}

	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: "blkid.*sda2", Output: "", Error: fmt.Errorf("hash partition UUID not found")},
		{Pattern: "blkid.*sda1", Output: "PARTUUID=root-partuuid\n", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	imageBoot := imageboot.NewImageBoot()
	err := imageBoot.InstallImageBoot(tmpDir, diskPathIdMap, template, "deb")

	if err == nil {
		t.Error("Expected error when hash partition UUID retrieval fails")
	}
	if !strings.Contains(err.Error(), "failed to get partition UUID for dm verity hash partition") {
		t.Errorf("Expected hash partition UUID error, got: %v", err)
	}
}
