package imageinspect

import (
	"bytes"
	"testing"
)

// TestRenderBootloaderConfigText demonstrates text rendering of bootloader config
func TestRenderBootloaderConfigText(t *testing.T) {
	cfg := &BootloaderConfig{
		ConfigFiles: map[string]string{
			"/boot/grub/grub.cfg": "abc123def456789abc123def456789ab",
		},
		BootEntries: []BootEntry{
			{
				Name:          "Ubuntu 24.04",
				Kernel:        "/vmlinuz-5.15",
				Initrd:        "/initrd-5.15",
				Cmdline:       "root=UUID=550e8400-e29b-41d4-a716-446655440000 ro quiet splash",
				IsDefault:     true,
				PartitionUUID: "550e8400-e29b-41d4-a716-446655440000",
				RootDevice:    "UUID=550e8400-e29b-41d4-a716-446655440000",
			},
			{
				Name:      "Recovery",
				Kernel:    "/vmlinuz-5.14",
				Initrd:    "/initrd-5.14",
				IsDefault: false,
			},
		},
		KernelReferences: []KernelReference{
			{
				Path:          "/vmlinuz-5.15",
				PartitionUUID: "550e8400-e29b-41d4-a716-446655440000",
				BootEntry:     "Ubuntu 24.04",
			},
			{
				Path:      "/vmlinuz-5.14",
				BootEntry: "Recovery",
			},
		},
		UUIDReferences: []UUIDReference{
			{
				UUID:                "550e8400-e29b-41d4-a716-446655440000",
				Context:             "boot_entry",
				ReferencedPartition: 3,
				Mismatch:            false,
			},
		},
		Issues: []string{},
	}

	var buf bytes.Buffer
	renderBootloaderConfigDetails(&buf, "/EFI/BOOT/BOOTX64.EFI", cfg)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Bootloader configuration")) {
		t.Errorf("Expected 'Bootloader configuration' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Ubuntu 24.04")) {
		t.Errorf("Expected 'Ubuntu 24.04' boot entry in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("/vmlinuz-5.15")) {
		t.Errorf("Expected kernel path in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Config files:")) {
		t.Errorf("Expected 'Config files:' section in output")
	}

	t.Logf("Rendered output:\n%s", output)
}

// TestRenderBootloaderConfigDiffText demonstrates text rendering of bootloader config differences
func TestRenderBootloaderConfigDiffText(t *testing.T) {
	cfgFrom := &BootloaderConfig{
		BootEntries: []BootEntry{
			{
				Name:    "Ubuntu",
				Kernel:  "/vmlinuz-5.14",
				Initrd:  "/initrd-5.14",
				Cmdline: "ro quiet",
			},
		},
	}

	cfgTo := &BootloaderConfig{
		BootEntries: []BootEntry{
			{
				Name:    "Ubuntu",
				Kernel:  "/vmlinuz-5.15",
				Initrd:  "/initrd-5.15",
				Cmdline: "ro quiet splash",
			},
		},
	}

	diff := compareBootloaderConfigs(cfgFrom, cfgTo)
	if diff == nil {
		t.Fatalf("Expected diff, got nil")
	}

	var buf bytes.Buffer
	renderBootloaderConfigDiffText(&buf, diff, "  ")

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Boot entries:")) {
		t.Errorf("Expected 'Boot entries:' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("modified")) {
		t.Errorf("Expected 'modified' status in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("/vmlinuz-5.14")) {
		t.Errorf("Expected old kernel version in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("/vmlinuz-5.15")) {
		t.Errorf("Expected new kernel version in output")
	}

	t.Logf("Rendered diff output:\n%s", output)
}

// TestRenderBootloaderConfigWithMismatch demonstrates critical issue rendering
func TestRenderBootloaderConfigWithMismatch(t *testing.T) {
	cfg := &BootloaderConfig{
		BootEntries: []BootEntry{
			{
				Name:       "Linux",
				Kernel:     "/vmlinuz",
				RootDevice: "UUID=99999999-9999-9999-9999-999999999999",
			},
		},
		UUIDReferences: []UUIDReference{
			{
				UUID:     "99999999-9999-9999-9999-999999999999",
				Context:  "boot_entry",
				Mismatch: true, // Critical!
			},
		},
		Issues: []string{
			"UUID 99999999-9999-9999-9999-999999999999 not found in partition table",
		},
	}

	var buf bytes.Buffer
	renderBootloaderConfigDetails(&buf, "/EFI/BOOT/BOOTX64.EFI", cfg)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("MISMATCH")) {
		t.Errorf("Expected 'MISMATCH' indicator in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Issues:")) {
		t.Errorf("Expected 'Issues:' section in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("99999999-9999-9999-9999-999999999999")) {
		t.Errorf("Expected invalid UUID in output")
	}

	t.Logf("Rendered output with mismatch:\n%s", output)
}
