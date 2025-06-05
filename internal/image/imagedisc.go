package imagedisc

import (
	"fmt"
	"os"
	"unsafe"

	azcfg "github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
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

var _ unsafe.Pointer // Dummy pointer, this line makes the 'unsafe' import explicitly used.

// Link to InitStderrLog directly
// Signature: func InitStderrLog(logLevel Level, packageName string) error
// Level is an int alias.
//
//go:linkname internalAzureLogInitStderrLog github.com/microsoft/azurelinux/toolkit/tools/internal/logger.InitStderrLog
func internalAzureLogInitStderrLog(levelAsInt int, packageName string) // implemented in another package via go:linkname

// InitializeAzureLogger should be called once at the beginning
func InitializeAzureLogger() error {
	programName := "imagedisc"
	azureLogLevelInfo := 4 // Corresponds to logger.LevelInfo (0:Panic, 1:Fatal, 2:Error, 3:Warn, 4:Info, 5:Debug, 6:Trace)

	log := utils.Logger()
	log.Debugf("Attempting to initialize Azure logger for package '%s' with level %d...\n", programName, azureLogLevelInfo)
	internalAzureLogInitStderrLog(azureLogLevelInfo, programName)
	log.Debugf("Azure logger InitStderrLog call completed successfully for package '%s'.\n", programName)
	return nil
}

func init() {
	err := InitializeAzureLogger()
	if err != nil {
		// Log the detailed, original error
		fmt.Fprintf(os.Stderr, "InitializeAzureLogger() raw error details: %#v\n", err) // Use %#v for detailed struct output
		// Then panic with your formatted message
		panic(fmt.Sprintf("imagedisc: CRITICAL - Failed to initialize Azure logger: %v", err))
	}
}

// CreateImageDisc allocates a new raw disk image file of the given size.
func CreateImageDisc(workDirPath string, discName string, maxSize uint64) error {

	// Validate the image path
	if workDirPath == "" || discName == "" || maxSize == 0 {
		return fmt.Errorf("invalid image path or max size")
	}

	log := utils.Logger()
	log.Debugf("Creating image disk at %s with max size %d bytes", workDirPath, maxSize)

	discFilePath, err := diskutils.CreateEmptyDisk(workDirPath, discName, maxSize)
	if err != nil {
		return fmt.Errorf("failed to create empty disk image: %w", err)
	}
	log.Infof("Created image disk at %s with max size %d bytes", discFilePath, maxSize)
	return nil
}

// SetupLoopbackDevice sets up a loopback device for the specified disk image file.
func SetupLoopbackDevice(discFilePath string) (string, error) {
	log := utils.Logger()
	log.Infof("Setting up loopback device for image disk at %s", discFilePath)

	// Validate the image path
	if discFilePath == "" {
		return "", fmt.Errorf("invalid image path")
	}

	// Call the Azure diskutils to setup the loopback device
	loopDev, err := diskutils.SetupLoopbackDevice(discFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to setup loopback device: %w", err)
	}
	log.Infof("Loopback device set up at %s for image disk %s", loopDev, discFilePath)
	return loopDev, nil
}

// DetachLoopbackDevice detaches the loopback device
func DetachLoopbackDevice(loopDev string) error {
	log := utils.Logger()
	log.Debugf("Detaching loopback device %s", loopDev)

	// Validate the loop device path
	if loopDev == "" {
		return fmt.Errorf("invalid loop device path")
	}

	// Call the Azure diskutils to detach the loopback device
	if err := diskutils.DetachLoopbackDevice(loopDev); err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}
	log.Infof("Detached loopback device %s successfully", loopDev)
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
func PartitionImageDisc(path string, maxSize uint64, parts []PartitionInfo) (partDevPathMap map[string]string,
	partIDToFsTypeMap map[string]string, err error) {

	log := utils.Logger()
	log.Infof("Partitioning image disk at %s with max size %d bytes", path, maxSize)

	// Validate the image path
	if path == "" || maxSize == 0 {
		return nil, nil, fmt.Errorf("invalid image path or max size")
	}

	azParts := toAzurePartitions(parts)
	cfg := azcfg.Disk{
		PartitionTableType: azcfg.PartitionTableTypeGpt,
		MaxSize:            maxSize,
		Partitions:         azParts,
	}
	rootEncryption := azcfg.RootEncryption{
		Enable:   false,
		Password: "",
	}
	partToDev, partIdToFs, encRoot, err := diskutils.CreatePartitions(path, cfg, rootEncryption, false)
	if err != nil {
		return nil, nil, fmt.Errorf("azure diskutils failed: %w", err)
	}
	log.Debugf("Partitioned image disk %s with partitions: %v", path, partToDev)
	log.Debugf("Partitioned image disk %s with filesystem map: %v", path, partIdToFs)
	log.Debugf("Partitioned image disk %s with encrypted root: %v", path, encRoot)
	return partToDev, partIdToFs, nil
}

func FormatPartitions(partDevs map[string]string, partFSTypes map[string]string, parts []PartitionInfo) error {
	log := utils.Logger()

	// Validate the image path and partition ID
	if len(partDevs) < 1 || len(parts) < 1 {
		return fmt.Errorf("invalid image path, partition ID or filesystem type")
	}

	detailsMap := make(map[string]PartitionInfo)
	for _, p := range parts {
		detailsMap[p.Name] = p
	}

	for partID, devPath := range partDevs {
		partInfo, ok := detailsMap[partID]
		if !ok {
			return fmt.Errorf("partition %s does not have a filesystem type defined", partID)
		}
		log.Infof("Formatting partition %s at %s with filesystem type %s", partID, devPath, partInfo.FsType)
		azPart := toAzureSinglePartition(partInfo)

		// Call the Azure diskutils to format the partition
		if _, err := diskutils.FormatSinglePartition(devPath, azPart); err != nil {
			return fmt.Errorf("failed to format partition: %w", err)
		}
		log.Infof("Formatted partition %d of image disk %s with filesystem type %s", partID, devPath, azPart.FsType)
	}
	return nil
}

// toAzurePartitions converts a slice of PartitionInfo to a slice of azcfg.Partition.
func toAzurePartitions(parts []PartitionInfo) []azcfg.Partition {
	azParts := make([]azcfg.Partition, len(parts))
	for i, p := range parts {
		azParts[i] = azcfg.Partition{
			ID:       p.Name, // Assuming the Name is the ID
			Name:     p.Name,
			FsType:   p.FsType,
			Start:    p.StartBytes,
			End:      p.SizeBytes,
			TypeUUID: p.TypeGUID,
		}
	}
	return azParts
}

// to AzureSinglePartition converts a single PartitionInfo to an azcfg.Partition.
func toAzureSinglePartition(part PartitionInfo) azcfg.Partition {
	return azcfg.Partition{
		ID:     part.Name, // Assuming the Name is the ID
		Name:   part.Name,
		FsType: part.FsType,
		Start:  part.StartBytes,
		End:    part.SizeBytes,
		Type:   part.TypeGUID,
	}
}
