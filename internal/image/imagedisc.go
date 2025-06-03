package imagedisc

import (
	"fmt"
	"os"

	diskfs "github.com/diskfs/go-diskfs"
	disk "github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// PartitionInfo describes a single partition to create
// Name: label for the partition
// SizeBytes: size of the partition in bytes
// StartBytes: absolute start offset in bytes; if zero, partitions are laid out sequentially
// TypeGUID: GPT type GUID for the partition (e.g., "8300" for Linux filesystem)
type PartitionInfo struct {
	Name       string
	SizeBytes  uint64
	StartBytes uint64
	TypeGUID   string
}

// CreateImageDisc allocates a new raw disk image file of the given size.
// It rounds up sizeBytes to the nearest 512-byte sector if needed.
func CreateImageDisc(path string, sizeBytes uint64) error {
	// Align size to sector boundary
	sectorSize := uint64(diskfs.SectorSize512)
	if sizeBytes%sectorSize != 0 {
		sizeBytes = ((sizeBytes + sectorSize - 1) / sectorSize) * sectorSize
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}
	defer f.Close()

	if err := f.Truncate(int64(sizeBytes)); err != nil {
		return fmt.Errorf("truncate image file: %w", err)
	}
	return nil
}

// DeleteImageDisc removes the disk image file.
func DeleteImageDisc(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete image file: %w", err)
	}
	return nil
}

// PartitionImageDisc creates a GPT partition table in the image according to parts.
func PartitionImageDisc(path string, parts []PartitionInfo) error {

	logger := utils.Logger()
	logger.Infof("Partitioning image %s with %d partitions", path, len(parts))

	for i, p := range parts {
		logger.Infof("Partition %d: %s, Size: %d bytes, Start: %d bytes, TypeGUID: %s",
			i+1, p.Name, p.SizeBytes, p.StartBytes, p.TypeGUID)
		if p.SizeBytes == 0 {
			return fmt.Errorf("partition %d has zero size", i+1)
		}
		if p.TypeGUID == "" {
			return fmt.Errorf("partition %d has empty TypeGUID", i+1)
		}
		if p.Name == "" {
			return fmt.Errorf("partition %d has empty name", i+1)
		}
	}

	// Open image with explicit sector size for consistent block size
	dsk, err := diskfs.Open(path, diskfs.WithSectorSize(diskfs.SectorSize512))
	if err != nil {
		return fmt.Errorf("open disk image: %w", err)
	}

	defer dsk.Close()

	// Build GPT table
	table := &gpt.Table{
		LogicalSectorSize:  int(dsk.LogicalBlocksize),
		PhysicalSectorSize: int(dsk.PhysicalBlocksize),
		ProtectiveMBR:      true,
		Partitions:         []*gpt.Partition{},
	}

	// Use 1 MiB alignment (start at LBA 2048)
	sectorSize := uint64(dsk.LogicalBlocksize)
	var cursorLBA uint64 = 2048
	for _, p := range parts {
		var startLBA uint64
		if p.StartBytes > 0 {
			startLBA = p.StartBytes / sectorSize
		} else {
			startLBA = cursorLBA
		}
		sizeLBA := p.SizeBytes / sectorSize
		table.Partitions = append(table.Partitions, &gpt.Partition{
			Start: startLBA,
			Size:  sizeLBA,
			Type:  gpt.Type(p.TypeGUID),
			Name:  p.Name,
		})
		cursorLBA = startLBA + sizeLBA
	}

	if err := dsk.Partition(table); err != nil {
		return fmt.Errorf("write GPT table: %w", err)
	}
	logger.Infof("Wrote GPT partition table with %d partitions", len(table.Partitions))
	return nil
}

// FormatPartition formats the specified partition (1-based) with the given filesystem type.
func FormatPartition(path string, partNum int, fsType string) error {
	// Open image with explicit sector size
	dsk, err := diskfs.Open(path, diskfs.WithSectorSize(diskfs.SectorSize512))
	if err != nil {
		return fmt.Errorf("open disk image: %w", err)
	}
	defer dsk.Close()
	spec := disk.FilesystemSpec{
		Partition: partNum,
	}
	switch fsType {
	case "ext4":
		spec.FSType = filesystem.TypeExt4
	case "fat32", "vfat":
		spec.FSType = filesystem.TypeFat32
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fsType)
	}

	if _, err := dsk.CreateFilesystem(spec); err != nil {
		return fmt.Errorf("format partition: %w", err)
	}
	return nil
}

// MountImageDisc retrieves the filesystem for the given partition (1-based).
func MountImageDisc(path string, partNum int) (filesystem.FileSystem, error) {
	dsk, err := diskfs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open disk image: %w", err)
	}
	fs, err := dsk.GetFilesystem(partNum)
	if err != nil {
		return nil, fmt.Errorf("get filesystem: %w", err)
	}
	return fs, nil
}

// UnmountImageDisc closes the filesystem, flushing any pending writes.
func UnmountImageDisc(fs filesystem.FileSystem) error {
	if err := fs.Close(); err != nil {
		return fmt.Errorf("close filesystem: %w", err)
	}
	return nil
}
