package imageinspect

import (
	"testing"
)

// Example test demonstrating bootloader configuration extraction and comparison

func TestParseGrubConfig(t *testing.T) {
	grubContent := `
menuentry 'Ubuntu 24.04 LTS (5.15.0-105-generic)' {
	search --no-floppy --label BOOT --set root
	echo	'Loading Ubuntu 24.04 LTS (5.15.0-105-generic)'
	linux	/vmlinuz-5.15.0-105-generic root=UUID=550e8400-e29b-41d4-a716-446655440000 ro quiet splash
	echo	'Loading initial ramdisk'
	initrd	/initrd.img-5.15.0-105-generic
}

menuentry 'Ubuntu 24.04 LTS (5.14.0-104-generic)' {
	search --no-floppy --label BOOT --set root
	echo	'Loading Ubuntu 24.04 LTS (5.14.0-104-generic)'
	linux	/vmlinuz-5.14.0-104-generic root=UUID=550e8400-e29b-41d4-a716-446655440000 ro quiet splash
	echo	'Loading initial ramdisk'
	initrd	/initrd.img-5.14.0-104-generic
}
`

	cfg := parseGrubConfigContent(grubContent)

	// Verify boot entries were parsed
	if len(cfg.BootEntries) != 2 {
		t.Errorf("Expected 2 boot entries, got %d", len(cfg.BootEntries))
	}

	// Verify kernel references were extracted
	if len(cfg.KernelReferences) != 2 {
		t.Errorf("Expected 2 kernel references, got %d", len(cfg.KernelReferences))
	}

	// Verify UUIDs were extracted
	if len(cfg.UUIDReferences) == 0 {
		t.Errorf("Expected UUID references, got none")
	}

	// Check specific entry
	if cfg.BootEntries[0].Name != "Ubuntu 24.04 LTS (5.15.0-105-generic)" {
		t.Errorf("Boot entry name mismatch: %s", cfg.BootEntries[0].Name)
	}

	if cfg.BootEntries[0].Kernel != "/vmlinuz-5.15.0-105-generic" {
		t.Errorf("Kernel path mismatch: %s", cfg.BootEntries[0].Kernel)
	}

	if cfg.BootEntries[0].Initrd != "/initrd.img-5.15.0-105-generic" {
		t.Errorf("Initrd path mismatch: %s", cfg.BootEntries[0].Initrd)
	}

	if cfg.BootEntries[0].RootDevice == "" {
		t.Errorf("Root device not extracted")
	}
}

func TestExtractUUIDs(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
	}{
		{
			input:    "UUID=550e8400-e29b-41d4-a716-446655440000",
			expected: 1,
		},
		{
			input:    "PARTUUID=550e8400-e29b-41d4-a716-446655440000 root=/dev/vda2",
			expected: 1,
		},
		{
			input:    "UUID=550e8400-e29b-41d4-a716-446655440000 and UUID=550e8400-e29b-41d4-a716-446655440001",
			expected: 2,
		},
		{
			input:    "no uuids here",
			expected: 0,
		},
	}

	for _, tc := range testCases {
		uuids := extractUUIDsFromString(tc.input)
		if len(uuids) != tc.expected {
			t.Errorf("Input: %q - Expected %d UUIDs, got %d", tc.input, tc.expected, len(uuids))
		}
	}
}

func TestCompareBootloaderConfigs(t *testing.T) {
	cfgFrom := &BootloaderConfig{
		ConfigFiles: map[string]string{
			"/boot/grub/grub.cfg": "abc123",
		},
		BootEntries: []BootEntry{
			{
				Name:   "Linux (old)",
				Kernel: "/vmlinuz-5.14",
				Initrd: "/initrd-5.14",
			},
		},
		KernelReferences: []KernelReference{
			{
				Path: "/vmlinuz-5.14",
			},
		},
	}

	cfgTo := &BootloaderConfig{
		ConfigFiles: map[string]string{
			"/boot/grub/grub.cfg": "def456", // Changed
		},
		BootEntries: []BootEntry{
			{
				Name:   "Linux (old)",
				Kernel: "/vmlinuz-5.15", // Changed
				Initrd: "/initrd-5.15",  // Changed
			},
			{
				Name:   "Linux (new)",
				Kernel: "/vmlinuz-5.16",
				Initrd: "/initrd-5.16",
			},
		},
		KernelReferences: []KernelReference{
			{
				Path: "/vmlinuz-5.15",
			},
			{
				Path: "/vmlinuz-5.16",
			},
		},
	}

	diff := compareBootloaderConfigs(cfgFrom, cfgTo)

	if diff == nil {
		t.Fatalf("Expected diff, got nil")
	}

	// Should detect config file change
	if len(diff.ConfigFileChanges) != 1 || diff.ConfigFileChanges[0].Status != "modified" {
		t.Errorf("Expected 1 modified config file, got %d changes", len(diff.ConfigFileChanges))
	}

	// Should detect boot entry modification
	if len(diff.BootEntryChanges) != 2 {
		t.Errorf("Expected 2 boot entry changes (1 modified, 1 added), got %d", len(diff.BootEntryChanges))
	}

	// Should detect kernel reference changes
	// Old vmlinuz-5.14 removed, vmlinuz-5.15 modified, vmlinuz-5.16 added = 3 changes
	if len(diff.KernelRefChanges) != 3 {
		t.Errorf("Expected 3 kernel reference changes, got %d", len(diff.KernelRefChanges))
	}
}

func TestUUIDResolution(t *testing.T) {
	pt := PartitionTableSummary{
		Partitions: []PartitionSummary{
			{
				Index: 1,
				GUID:  "550e8400-e29b-41d4-a716-446655440000",
			},
			{
				Index: 2,
				GUID:  "550e8400-e29b-41d4-a716-446655440001",
				Filesystem: &FilesystemSummary{
					UUID: "550e8400-e29b-41d4-a716-446655440002",
				},
			},
		},
	}

	uuidRefs := []UUIDReference{
		{
			UUID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			UUID: "550e8400-e29b-41d4-a716-446655440002",
		},
		{
			UUID: "99999999-9999-9999-9999-999999999999", // Non-existent
		},
	}

	resolved := resolveUUIDsToPartitions(uuidRefs, pt)

	if len(resolved) != 2 {
		t.Errorf("Expected 2 resolved UUIDs, got %d", len(resolved))
	}

	if part, ok := resolved["550e8400-e29b-41d4-a716-446655440000"]; !ok || part != 1 {
		t.Errorf("First UUID should resolve to partition 1")
	}

	if part, ok := resolved["550e8400-e29b-41d4-a716-446655440002"]; !ok || part != 2 {
		t.Errorf("Second UUID should resolve to partition 2 (filesystem UUID)")
	}
}

func TestValidateBootloaderConfig(t *testing.T) {
	pt := PartitionTableSummary{
		Partitions: []PartitionSummary{
			{
				Index: 1,
				GUID:  "550e8400-e29b-41d4-a716-446655440000",
			},
		},
	}

	cfg := &BootloaderConfig{
		BootEntries: []BootEntry{
			{
				Name:   "Test",
				Kernel: "", // Empty kernel - should trigger issue
			},
		},
		UUIDReferences: []UUIDReference{
			{
				UUID:    "99999999-9999-9999-9999-999999999999", // Invalid UUID
				Context: "test",
			},
		},
	}

	ValidateBootloaderConfig(cfg, pt)

	if len(cfg.Issues) == 0 {
		t.Errorf("Expected validation issues, got none")
	}

	// Should have issue about missing kernel path
	hasKernelIssue := false
	hasMismatchIssue := false

	for _, issue := range cfg.Issues {
		if len(issue) > 0 {
			if issue[0] == 'B' { // "Boot entry..."
				hasKernelIssue = true
			}
			if issue[0] == 'U' { // "UUID..."
				hasMismatchIssue = true
			}
		}
	}

	if !hasKernelIssue {
		t.Errorf("Expected kernel issue not found")
	}

	if !hasMismatchIssue {
		t.Errorf("Expected UUID mismatch issue not found")
	}
}

// Example demonstrating real-world usage
func ExampleBootloaderConfig() {
	// Simulate extracting config from two images
	grubConfig1 := `
menuentry 'Linux' {
	linux /vmlinuz-5.14 root=UUID=550e8400-e29b-41d4-a716-446655440000 ro
	initrd /initrd-5.14
}
`

	grubConfig2 := `
menuentry 'Linux' {
	linux /vmlinuz-5.15 root=UUID=550e8400-e29b-41d4-a716-446655440000 ro
	initrd /initrd-5.15
}
`

	cfg1 := parseGrubConfigContent(grubConfig1)
	cfg2 := parseGrubConfigContent(grubConfig2)

	// Compare configurations
	diff := compareBootloaderConfigs(&cfg1, &cfg2)

	if diff != nil && len(diff.BootEntryChanges) > 0 {
		// Kernel was updated in the boot entry
		for _, change := range diff.BootEntryChanges {
			if change.Status == "modified" && change.KernelFrom != change.KernelTo {
				// Log that kernel version changed
			}
		}
	}

	// Check for UUID mismatches
	pt := PartitionTableSummary{
		Partitions: []PartitionSummary{
			{Index: 1, GUID: "550e8400-e29b-41d4-a716-446655440000"},
		},
	}

	ValidateBootloaderConfig(&cfg1, pt)
	// cfg1.Issues would contain any validation problems
}
