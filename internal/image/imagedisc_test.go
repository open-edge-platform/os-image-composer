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
	if err := PartitionImageDisc(imgPath, maxSize, parts); err != nil {
		t.Fatalf("PartitionImageDisc failed: %v", err)
	}

	// // 5. Format that partition as ext4 (shells out to mkfs.ext4)
	// if err := FormatPartition(imgPath, 1, "ext4"); err != nil {
	// 	t.Fatalf("FormatPartition failed: %v", err)
	// }

	// // 6. Mount the ext4 partition in-memory and verify it's empty
	// fs, dsk, err := MountImageDisc(imgPath, 1)
	// if err != nil {
	// 	t.Fatalf("MountImageDisc failed: %v", err)
	// }
	// defer func() {
	// 	if err := UnmountImageDisc(fs, dsk); err != nil {
	// 		t.Errorf("UnmountImageDisc failed: %v", err)
	// 	}
	// }()

	// entries, err := fs.ReadDir("/")
	// if err != nil {
	// 	t.Fatalf("filesystem ReadDir failed: %v", err)
	// }
	// if len(entries) != 0 {
	// 	t.Errorf("expected empty root filesystem, got %d entries", len(entries))
	// }
}
