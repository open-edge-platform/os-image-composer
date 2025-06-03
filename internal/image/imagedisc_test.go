package imagedisc

import (
	"path/filepath"
	"testing"

	"github.com/diskfs/go-diskfs/partition/gpt"
)

func TestBasicImageDiscWorkflow(t *testing.T) {
	// Setup temp image file
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "test.img")
	sizeBytes := uint64(10 * 1024 * 1024) // 5MiB

	// Create image
	if err := CreateImageDisc(imgPath, sizeBytes); err != nil {
		t.Fatalf("CreateImageDisc failed: %v", err)
	}
	// Ensure cleanup
	defer func() {
		if err := DeleteImageDisc(imgPath); err != nil {
			t.Errorf("DeleteImageDisc failed: %v", err)
		}
	}()

	// Define a single 1MiB partition
	parts := []PartitionInfo{
		{
			Name:       "root",
			SizeBytes:  5 * 1024 * 1024,
			StartBytes: 0,
			TypeGUID:   string(gpt.LinuxFilesystem),
		},
	}

	// Partition the image
	if err := PartitionImageDisc(imgPath, parts); err != nil {
		t.Fatalf("PartitionImageDisc failed: %v", err)
	}

	// Format the first partition as ext4
	if err := FormatPartition(imgPath, 1, "fat32"); err != nil {
		t.Fatalf("FormatPartition failed: %v", err)
	}

	// Mount and verify filesystem is empty
	fs, err := MountImageDisc(imgPath, 1)
	if err != nil {
		t.Fatalf("MountImageDisc failed: %v", err)
	}
	entries, err := fs.ReadDir("/")
	if err != nil {
		t.Fatalf("filesystem ReadDir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty root filesystem, got %d entries", len(entries))
	}

	// Unmount/close filesystem
	if err := UnmountImageDisc(fs); err != nil {
		t.Fatalf("UnmountImageDisc failed: %v", err)
	}
}
