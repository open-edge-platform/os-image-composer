package imagedisc_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

func TestIsDigit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid_digits", "12345", true},
		{"single_digit", "7", true},
		{"empty_string", "", false},
		{"contains_letters", "123abc", false},
		{"contains_special_chars", "123-456", false},
		{"only_letters", "abc", false},
		{"zero", "0", true},
		{"leading_zero", "0123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imagedisc.IsDigit(tt.input)
			if result != tt.expected {
				t.Errorf("IsDigit(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVerifyFileSize(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    string
		expectError bool
		errorMsg    string
	}{
		{"valid_int", 100, "100MiB", false, ""},
		{"zero_string", "0", "0", false, ""},
		{"valid_mib", "500MiB", "500MiB", false, ""},
		{"valid_gib", "2GiB", "2GiB", false, ""},
		{"valid_kb", "1024KB", "1024KB", false, ""},
		{"invalid_suffix", "100XB", "", true, "file size suffix incorrect"},
		{"invalid_number", "abcMiB", "", true, "file size format incorrect"},
		{"invalid_format", "invalid", "", true, "file size format incorrect"},
		{"unsupported_type", 12.5, "", true, "unsupported fileSize type"},
		{"empty_string", "", "", true, "file size format incorrect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := imagedisc.VerifyFileSize(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %v, but got none", tt.input)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input %v, but got: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestTranslateSizeStrToBytes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    uint64
		expectError bool
		errorMsg    string
	}{
		{"mib_conversion", "1MiB", 1048576, false, ""},
		{"gib_conversion", "1GiB", 1073741824, false, ""},
		{"kib_conversion", "1KiB", 1024, false, ""},
		{"mb_conversion", "1MB", 1000000, false, ""},
		{"gb_conversion", "1GB", 1000000000, false, ""},
		{"large_number", "100MiB", 104857600, false, ""},
		{"invalid_suffix", "1XB", 0, true, "file size suffix incorrect"},
		{"invalid_format", "invalid", 0, true, "size format incorrect"},
		{"no_number", "MiB", 0, true, "size format incorrect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := imagedisc.TranslateSizeStrToBytes(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %s, but got none", tt.input)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input %s, but got: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, but got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestCreateRawFile(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		filePath     string
		fileSize     string
		mockCommands []shell.MockCommand
		expectError  bool
		errorMsg     string
		shouldExist  bool
	}{
		{
			name:     "successful_creation",
			filePath: "/tmp/test/disk.img",
			fileSize: "100MiB",
			mockCommands: []shell.MockCommand{
				{Pattern: "fallocate", Output: "", Error: nil},
			},
			expectError: false,
			shouldExist: true,
		},
		{
			name:     "invalid_file_size",
			filePath: "/tmp/test/disk.img",
			fileSize: "invalidsize",
			mockCommands: []shell.MockCommand{
				{Pattern: "fallocate", Output: "", Error: nil},
			},
			expectError: true,
			errorMsg:    "file size format incorrect",
		},
		{
			name:     "fallocate_failure",
			filePath: "/tmp/test/disk.img",
			fileSize: "100MiB",
			mockCommands: []shell.MockCommand{
				{Pattern: "fallocate", Output: "", Error: fmt.Errorf("fallocate failed")},
			},
			expectError: true,
			errorMsg:    "failed to create raw file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			// Ensure temp directory exists
			tempDir := t.TempDir()
			testFilePath := filepath.Join(tempDir, "disk.img")

			err := imagedisc.CreateRawFile(testFilePath, tt.fileSize, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestGetDiskNameFromDiskPath(t *testing.T) {
	tests := []struct {
		name        string
		diskPath    string
		expected    string
		expectError bool
	}{
		{"valid_sda", "/dev/sda", "sda", false},
		{"valid_nvme", "/dev/nvme0n1", "nvme0n1", false},
		{"valid_loop", "/dev/loop0", "loop0", false},
		{"invalid_path", "/invalid/path", "", true},
		{"no_dev_prefix", "sda", "", true},
		{"empty_path", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := imagedisc.GetDiskNameFromDiskPath(tt.diskPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for path %s, but got none", tt.diskPath)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for path %s, but got: %v", tt.diskPath, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestDiskGetHwSectorSize(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskName     string
		mockCommands []shell.MockCommand
		expected     int
		expectError  bool
	}{
		{
			name:     "successful_read",
			diskName: "sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "cat /sys/block/sda/queue/hw_sector_size", Output: "512\n", Error: nil},
			},
			expected:    512,
			expectError: false,
		},
		{
			name:     "command_failure",
			diskName: "sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "cat /sys/block/sda/queue/hw_sector_size", Output: "", Error: fmt.Errorf("file not found")},
			},
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.DiskGetHwSectorSize(tt.diskName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, but got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestDiskGetPhysicalBlockSize(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskName     string
		mockCommands []shell.MockCommand
		expected     int
		expectError  bool
	}{
		{
			name:     "successful_read",
			diskName: "sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "cat /sys/block/sda/queue/physical_block_size", Output: "4096\n", Error: nil},
			},
			expected:    4096,
			expectError: false,
		},
		{
			name:     "command_failure",
			diskName: "sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "cat /sys/block/sda/queue/physical_block_size", Output: "", Error: fmt.Errorf("file not found")},
			},
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.DiskGetPhysicalBlockSize(tt.diskName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, but got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestDiskGetDevInfo(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskPath     string
		mockCommands []shell.MockCommand
		expectError  bool
		errorMsg     string
	}{
		{
			name:     "successful_read",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: `{"blockdevices":[{"name":"sda","path":"/dev/sda","type":"disk"}]}`, Error: nil},
			},
			expectError: false,
		},
		{
			name:     "command_failure",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: "", Error: fmt.Errorf("lsblk failed")},
			},
			expectError: true,
			errorMsg:    "lsblk failed",
		},
		{
			name:     "invalid_json",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: "invalid json", Error: nil},
			},
			expectError: true,
			errorMsg:    "invalid character",
		},
		{
			name:     "device_not_found",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: `{"blockdevices":[{"name":"sdb","path":"/dev/sdb","type":"disk"}]}`, Error: nil},
			},
			expectError: true,
			errorMsg:    "device not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.DiskGetDevInfo(tt.diskPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result")
				}
			}
		})
	}
}

func TestDiskGetPartitionsInfo(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskPath     string
		mockCommands []shell.MockCommand
		expectError  bool
		expectedLen  int
	}{
		{
			name:     "with_partitions",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: `{"blockdevices":[{"name":"sda1","path":"/dev/sda1","type":"part"},{"name":"sda2","path":"/dev/sda2","type":"part"}]}`, Error: nil},
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name:     "no_partitions",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: `{"blockdevices":[{"name":"sda","path":"/dev/sda","type":"disk"}]}`, Error: nil},
			},
			expectError: false,
			expectedLen: 0,
		},
		{
			name:     "command_failure",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "lsblk /dev/sda", Output: "", Error: fmt.Errorf("lsblk failed")},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.DiskGetPartitionsInfo(tt.diskPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if len(result) != tt.expectedLen {
					t.Errorf("Expected %d partitions, but got %d", tt.expectedLen, len(result))
				}
			}
		})
	}
}

func TestPartitionTypeStrToGUID(t *testing.T) {
	tests := []struct {
		name          string
		partitionType string
		expectedGUID  string
		expectError   bool
	}{
		{"linux_type", "linux", "0fc63daf-8483-4772-8e79-3d69d8477de4", false},
		{"esp_type", "esp", "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", false},
		{"bios_type", "bios", "21686148-6449-6e6f-744e-656564454649", false},
		{"invalid_type", "invalid", "", true},
		{"empty_type", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := imagedisc.PartitionTypeStrToGUID(tt.partitionType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for type %s, but got none", tt.partitionType)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for type %s, but got: %v", tt.partitionType, err)
				}
				if result != tt.expectedGUID {
					t.Errorf("Expected GUID %s, but got %s", tt.expectedGUID, result)
				}
			}
		})
	}
}

func TestPartitionGUIDToTypeStr(t *testing.T) {
	tests := []struct {
		name          string
		partitionGUID string
		expectedType  string
		expectError   bool
	}{
		{"linux_guid", "0fc63daf-8483-4772-8e79-3d69d8477de4", "linux", false},
		{"esp_guid", "c12a7328-f81f-11d2-ba4b-00a0c93ec93b", "esp", false},
		{"invalid_guid", "invalid-guid", "", true},
		{"empty_guid", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := imagedisc.PartitionGUIDToTypeStr(tt.partitionGUID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for GUID %s, but got none", tt.partitionGUID)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for GUID %s, but got: %v", tt.partitionGUID, err)
				}
				if result != tt.expectedType {
					t.Errorf("Expected type %s, but got %s", tt.expectedType, result)
				}
			}
		})
	}
}

func TestIsDiskPartitionExist(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskPath     string
		mockCommands []shell.MockCommand
		expected     bool
		expectError  bool
	}{
		{
			name:     "has_partitions",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "Disk /dev/sda: 372.61 GiB, 400088457216 bytes, 781422768 sectors\n/dev/sda1 * 2048 204799 202752 99M EFI System", Error: nil},
			},
			expected:    true,
			expectError: false,
		},
		{
			name:     "no_partitions",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "Disk /dev/sda: 372.61 GiB, 400088457216 bytes, 781422768 sectors\nSector size (logical/physical): 512 bytes / 512 bytes", Error: nil},
			},
			expected:    false,
			expectError: false,
		},
		{
			name:     "command_failure",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "", Error: fmt.Errorf("fdisk failed")},
			},
			expected:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.IsDiskPartitionExist(tt.diskPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %v, but got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestWipePartitions(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskPath     string
		mockCommands []shell.MockCommand
		expectError  bool
		errorMsg     string
	}{
		{
			name:     "successful_wipe",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "wipefs", Output: "", Error: nil},
				{Pattern: "sync", Output: "", Error: nil},
			},
			expectError: false,
		},
		{
			name:     "wipefs_failure",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "wipefs", Output: "", Error: fmt.Errorf("wipefs failed")},
			},
			expectError: true,
			errorMsg:    "failed to wipe disk",
		},
		{
			name:     "sync_failure",
			diskPath: "/dev/sda",
			mockCommands: []shell.MockCommand{
				{Pattern: "wipefs", Output: "", Error: nil},
				{Pattern: "sync", Output: "", Error: fmt.Errorf("sync failed")},
			},
			expectError: true,
			errorMsg:    "failed to sync after wiping disk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			err := imagedisc.WipePartitions(tt.diskPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestGetUUID(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		partPath     string
		mockCommands []shell.MockCommand
		expected     string
		expectError  bool
	}{
		{
			name:     "successful_uuid",
			partPath: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s UUID -o value", Output: "12345678-1234-1234-1234-123456789abc\n", Error: nil},
			},
			expected:    "12345678-1234-1234-1234-123456789abc",
			expectError: false,
		},
		{
			name:     "command_failure",
			partPath: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s UUID -o value", Output: "", Error: fmt.Errorf("blkid failed")},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.GetUUID(tt.partPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestGetPartUUID(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		partPath     string
		mockCommands []shell.MockCommand
		expected     string
		expectError  bool
	}{
		{
			name:     "successful_partuuid",
			partPath: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s PARTUUID -o value", Output: "12345678-1234-1234-1234-123456789abc\n", Error: nil},
			},
			expected:    "12345678-1234-1234-1234-123456789abc",
			expectError: false,
		},
		{
			name:     "command_failure",
			partPath: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s PARTUUID -o value", Output: "", Error: fmt.Errorf("blkid failed")},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.GetPartUUID(tt.partPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestDiskPartitionsCreate(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name               string
		diskPath           string
		partitionsList     []config.PartitionInfo
		partitionTableType string
		mockCommands       []shell.MockCommand
		expectError        bool
		errorMsg           string
		expectedDevices    int
	}{
		{
			name:     "gpt_single_partition",
			diskPath: "/dev/sda",
			partitionsList: []config.PartitionInfo{
				{
					ID:     "root",
					Name:   "root",
					Start:  "1MiB",
					End:    "100MiB",
					FsType: "ext4",
					Type:   "linux",
				},
			},
			partitionTableType: "gpt",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "Disk /dev/sda: 1 GiB", Error: nil},
				{Pattern: "echo 'label: gpt'", Output: "", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/hw_sector_size", Output: "512", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/physical_block_size", Output: "4096", Error: nil},
				{Pattern: "echo", Output: "", Error: nil},
				{Pattern: "partx -u /dev/sda", Output: "", Error: nil},
				{Pattern: "mkfs", Output: "", Error: nil},
			},
			expectError:     false,
			expectedDevices: 1,
		},
		{
			name:     "mbr_single_partition",
			diskPath: "/dev/sda",
			partitionsList: []config.PartitionInfo{
				{
					ID:     "root",
					Name:   "root",
					Start:  "1MiB",
					End:    "100MiB",
					FsType: "ext4",
				},
			},
			partitionTableType: "mbr",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "Disk /dev/sda: 1 GiB", Error: nil},
				{Pattern: "echo 'label: dos'", Output: "", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/hw_sector_size", Output: "512", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/physical_block_size", Output: "4096", Error: nil},
				{Pattern: "echo", Output: "", Error: nil},
				{Pattern: "partx -u /dev/sda", Output: "", Error: nil},
				{Pattern: "mkfs", Output: "", Error: nil},
			},
			expectError:     false,
			expectedDevices: 1,
		},
		{
			name:     "partition_creation_failure",
			diskPath: "/dev/sda",
			partitionsList: []config.PartitionInfo{
				{
					ID:     "root",
					Start:  "1MiB",
					End:    "100MiB",
					FsType: "ext4",
				},
			},
			partitionTableType: "gpt",
			mockCommands: []shell.MockCommand{
				{Pattern: "fdisk -l /dev/sda", Output: "Disk /dev/sda: 1 GiB", Error: nil},
				{Pattern: "echo 'label: gpt'", Output: "", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/hw_sector_size", Output: "512", Error: nil},
				{Pattern: "cat /sys/block/sda/queue/physical_block_size", Output: "4096", Error: nil},
				{Pattern: "echo", Output: "", Error: fmt.Errorf("sfdisk failed")},
			},
			expectError: true,
			errorMsg:    "failed to create partition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.DiskPartitionsCreate(tt.diskPath, tt.partitionsList, tt.partitionTableType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if len(result) != tt.expectedDevices {
					t.Errorf("Expected %d devices, but got %d", tt.expectedDevices, len(result))
				}
			}
		})
	}
}

func TestGetPartitionLabel(t *testing.T) {
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	tests := []struct {
		name         string
		diskPartDev  string
		mockCommands []shell.MockCommand
		expected     string
		expectError  bool
	}{
		{
			name:        "successful_label",
			diskPartDev: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s PARTLABEL -o value", Output: "EFI System\n", Error: nil},
			},
			expected:    "EFI System",
			expectError: false,
		},
		{
			name:        "command_failure",
			diskPartDev: "/dev/sda1",
			mockCommands: []shell.MockCommand{
				{Pattern: "blkid /dev/sda1 -s PARTLABEL -o value", Output: "", Error: fmt.Errorf("blkid failed")},
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell.Default = shell.NewMockExecutor(tt.mockCommands)

			result, err := imagedisc.GetPartitionLabel(tt.diskPartDev)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, but got %s", tt.expected, result)
				}
			}
		})
	}
}
