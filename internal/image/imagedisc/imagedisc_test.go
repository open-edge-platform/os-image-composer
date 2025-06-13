package imagedisc

import (
	"path/filepath"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
)

var (
	singlePartition = []config.PartitionInfo{
		{
			Name:       "root",
			ID:         "rootfs",
			FsType:     "ext4",
			StartBytes: 1024 * 1024,                            // 1 MiB
			SizeBytes:  8 * 1024 * 1024,                        // 8 MiB
			TypeGUID:   "0FC63DAF-8483-4772-8E79-3D69D8477DE4", // Linux filesystem
		},
	}

	multiPartitions = []config.PartitionInfo{
		{
			Name:       "boot",
			ID:         "boot",
			FsType:     "fat32",
			StartBytes: 1024 * 1024,                            // 1 MiB
			SizeBytes:  100 * 1024 * 1024,                      // 100 MiB
			TypeGUID:   "C12A7328-F81F-11D2-BA4B-00A0C93EC93B", // EFI System Partition
		},
		{
			Name:       "root",
			ID:         "rootfs",
			FsType:     "ext4",
			StartBytes: 101 * 1024 * 1024,                      // Start after boot (1 + 100 MiB)
			SizeBytes:  200 * 1024 * 1024,                      // 200 MiB
			TypeGUID:   "0FC63DAF-8483-4772-8E79-3D69D8477DE4", // Linux filesystem
		},
	}
)

func TestImageCreation(t *testing.T) {

	// Create a temporary directory and image path
	tempDir := t.TempDir()
	imageName := "test.img"

	// Make the image 10 MiB so we can carve out an 8 MiB ext4 partition
	maxSize := uint64(10 * 1024 * 1024)

	// Create the raw image file
	if err := CreateImageDisc(tempDir, imageName, maxSize); err != nil {
		t.Fatalf("CreateImageDisc failed: %v", err)
	}

	// Ensure the image is deleted at the end
	imgPath := filepath.Join(tempDir, imageName)
	defer func() {
		if err := DeleteImageDisc(imgPath); err != nil {
			t.Errorf("DeleteImageDisc failed: %v", err)
		}
	}()
}
func TestImagePartitioning(t *testing.T) {

	testCases := []struct {
		name       string
		partitions []config.PartitionInfo
		imageSize  uint64 // in bytes
	}{
		{
			name:       "SingleRootPartition",
			partitions: singlePartition,
			imageSize:  10 * 1024 * 1024, // 10 MiB image is sufficient
		},
		{
			name:       "BootAndRootPartitions",
			partitions: multiPartitions,
			imageSize:  350 * 1024 * 1024, // ~350 MiB partitions need a larger image
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			imageName := "test.img"
			imgPath := filepath.Join(tempDir, imageName)

			if err := CreateImageDisc(tempDir, imageName, tc.imageSize); err != nil {
				t.Fatalf("CreateImageDisc failed: %v", err)
			}

			dev, err := SetupLoopbackDevice(imgPath)
			if err != nil {
				t.Fatalf("SetupLoopbackDevice failed: %v", err)
			}

			t.Cleanup(func() {
				if err := DetachLoopbackDevice(dev, imgPath); err != nil {
					t.Errorf("DetachLoopbackDevice failed: %v", err)
				}
			})

			partDevPathMap, partIDToFsTypeMap, err := PartitionImageDisc(dev, tc.imageSize, tc.partitions)
			if err != nil {
				t.Fatalf("PartitionImageDisc failed: %v", err)
			}

			if err := FormatPartitions(partDevPathMap, partIDToFsTypeMap, tc.partitions); err != nil {
				t.Fatalf("FormatPartitions failed: %v", err)
			}
		})
	}
}
