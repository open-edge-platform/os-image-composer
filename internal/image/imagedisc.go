package imagedisc

import (
	"fmt"
	"os"

	azcfg "github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	azdisc "github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	azlog "github.com/microsoft/azurelinux/toolkit/tools/internal/logger"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// PartitionInfo holds information about a partition to be created in an image.
type PartitionInfo struct {
	Name       string // Name: label for the partition
	TypeGUID   string // TypeGUID: GPT type GUID for the partition (e.g., "8300" for Linux filesystem)
	FsType     string // FsType: filesystem type (e.g., "ext4", "xfs", etc.);
	SizeBytes  uint64 // SizeBytes: size of the partition in bytes
	StartBytes uint64 // StartBytes: absolute start offset in bytes; if zero, partitions are laid out sequentially
}

// CreateImageDisc allocates a new raw disk image file of the given size.
func CreateImageDisc(workDirPath string, discName string, maxSize uint64) error {

	azlog.Init(utils.Logger()) // Initialize the logger for Azure diskutils

	// Validate the image path
	if workDirPath == "" || discName == "" || maxSize == 0 {
		return fmt.Errorf("invalid image path or max size")
	}

	log := utils.Logger()
	log.Debugf("Creating image disk at %s with max size %d bytes", workDirPath, maxSize)

	discFilePath, err := azdisc.CreateEmptyDisk(workDirPath, discName, maxSize)
	if err != nil {
		return fmt.Errorf("failed to create empty disk image: %w", err)
	}
	log.Infof("Created image disk at %s with max size %d bytes", discFilePath, maxSize)
	return nil
}

// DeleteImageDisc deletes the specified disk image file.
func DeleteImageDisc(discFilePath string) error {

	if err := os.Remove(discFilePath); err != nil {
		return fmt.Errorf("delete image file: %w", err)
	}
	return nil
}

// PartitionImageDisc partitions the specified disk image file according to the
// provided partition information.
func PartitionImageDisc(path string, maxSize uint64, parts []PartitionInfo) error {

	log := utils.Logger()
	log.Infof("Partitioning image disk at %s with max size %d bytes", path, maxSize)
	// Validate the image path
	if path == "" || maxSize == 0 {
		return fmt.Errorf("invalid image path or max size")
	}

	// Convert PartitionInfo -> azdisk.PartitionType, etc.
	azParts := make([]azcfg.Partition, len(parts))
	for i, p := range parts {
		azParts[i] = azcfg.Partition{
			Name:     p.Name,
			FsType:   p.FsType,
			Start:    p.StartBytes,
			End:      p.SizeBytes,
			TypeUUID: p.TypeGUID,
		}

	}
	cfg := azcfg.Disk{
		PartitionTableType: azcfg.PartitionTableTypeGpt,
		MaxSize:            maxSize,
		Partitions:         azParts,
	}
	rootEncryption := azcfg.RootEncryption{
		Enable:   false,
		Password: "",
	}
	// Now call the Azure helper:
	partDevMap, partIdToFs, encRoot, err := azdisc.CreatePartitions(path, cfg, rootEncryption, true)
	if err != nil {
		return fmt.Errorf("azure diskutils failed: %w", err)
	}
	log.Infof("Partitioned image disk %s with partitions: %v", path, partDevMap)
	log.Infof("Partitioned image disk %s with filesystem map: %v", path, partIdToFs)
	log.Infof("Partitioned image disk %s with encrypted root: %v", path, encRoot)
	return nil
}
