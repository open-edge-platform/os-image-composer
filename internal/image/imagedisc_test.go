package imagedisc

import (
	"path/filepath"
	"testing"
)

func TestBasicImageDiscWorkflow(t *testing.T) {

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

	// Setup loopback device for the image
	dev, err := SetupLoopbackDevice(imgPath)
	if err != nil {
		t.Fatalf("SetupLoopbackDevice failed: %v", err)
	}

	// Define a single 8 MiB partition (aligned at LBA 2048 internally)
	parts := []PartitionInfo{
		{
			Name:       "root",                                 // Partition label
			FsType:     "ext4",                                 // Filesystem type
			SizeBytes:  8 * 1024 * 1024,                        // 8 MiB
			StartBytes: 2048,                                   // set the LBA=2048 offset
			TypeGUID:   "0FC63DAF-8483-4772-8E79-3D69D8477DE4", // Linux filesystem type GUID
		},
	}

	// Partition the image
	if _, _, err := PartitionImageDisc(dev, maxSize, parts); err != nil {
		t.Fatalf("PartitionImageDisc failed: %v", err)
	}

	// Format the partitions
	if err := FormatPartitions(dev, parts); err != nil {
		t.Fatalf("FormatPartitions failed: %v", err)
	}

	// Detach the loopback device
	if err := DetachLoopbackDevice(dev); err != nil {
		t.Fatalf("DetachLoopbackDevice failed: %v", err)
	}

}
