package imagedisc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	azcfg "github.com/microsoft/azurelinux/toolkit/tools/imagegen/configuration"
	"github.com/microsoft/azurelinux/toolkit/tools/imagegen/diskutils"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/convert"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

const (
	MiB = (1024 * 1024) // 1 MiB = 1024 KiB * 1024 bytes
)

var _ unsafe.Pointer // Dummy pointer, this line makes the 'unsafe' import explicitly used.

// Link to InitStderrLog directly
// Signature: func InitStderrLog(logLevel Level, packageName string) error
// Level is an int alias.
//
//go:linkname internalAzureLogInitStderrLog github.com/microsoft/azurelinux/toolkit/tools/internal/logger.InitStderrLog
func internalAzureLogInitStderrLog(levelAsInt int, packageName string) // implemented in another package via go:linkname

// InitializeAzureLogger should be called once at the beginning
func InitializeAzureLogger() error {
	// Corresponds to logger.LevelInfo (0:Panic, 1:Fatal, 2:Error, 3:Warn, 4:Info, 5:Debug, 6:Trace)
	var azureLogLevelInfo int = 4 // Default to Info level
	programName := "imagedisc"
	logLevel := config.LogLevel()

	switch logLevel {
	case "info":
		azureLogLevelInfo = 4
	case "debug":
		azureLogLevelInfo = 5
	case "warn", "warning":
		azureLogLevelInfo = 3
	case "error":
		azureLogLevelInfo = 2
	}
	log := utils.Logger()
	log.Debugf("Attempting to initialize Azure logger for package '%s' with level %d...\n", programName, azureLogLevelInfo)
	internalAzureLogInitStderrLog(azureLogLevelInfo, programName)
	log.Debugf("Azure logger InitStderrLog call completed successfully for package '%s'.\n", programName)
	return nil
}

func CreateRawImage(filePath string, template *config.ImageTemplate) (string, map[string]string, error) {
	var diskPathIdMap map[string]string
	var loopDevPath string

	diskInfo := template.GetDiskConfig()
	diskSize, err := convert.NormalizeSizeToBytes(diskInfo.Size)
	if err != nil {
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to normalize disk size: %w", err)
	}

	fileDir := filepath.Dir(filePath)
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		if err := os.MkdirAll(fileDir, 0755); err != nil {
			return loopDevPath, diskPathIdMap, fmt.Errorf("failed to create directory for image file: %w", err)
		}
	}

	err = InitializeAzureLogger()
	if err != nil {
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to initialize Azure logger: %w", err)
	}

	if err := CreateImageDisc(fileDir, filepath.Base(filePath), diskSize); err != nil {
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to create image file: %w", err)
	}

	loopDevPath, err = SetupLoopbackDevice(filePath)
	if err != nil {
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to setup loopback device: %w", err)
	}

	diskPathIdMap, _, err = PartitionImageDisc(loopDevPath, diskSize, diskInfo.Partitions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "PartitionImageDisc error: %v\n", err)
		if err = DetachLoopbackDevice(filePath, loopDevPath); err != nil {
			return loopDevPath, diskPathIdMap, fmt.Errorf("failed to detach loopback device after partitioning error: %w", err)
		}
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to partition image file: %w", err)
	}

	return loopDevPath, diskPathIdMap, nil
}

// CreateImageDisc allocates a new raw disk image file of the given size.
func CreateImageDisc(workDirPath string, discName string, maxSize uint64) error {

	// Validate the image path
	if workDirPath == "" || discName == "" || maxSize == 0 {
		return fmt.Errorf("invalid image path or max size")
	}
	maxSizeMb := (maxSize / MiB) // Convert maxSize to MiB for diskutils

	log := utils.Logger()
	log.Debugf("Creating image disk at %s with max size %d MiB", workDirPath, maxSizeMb)

	discFilePath, err := diskutils.CreateEmptyDisk(workDirPath, discName, maxSizeMb)
	if err != nil {
		return fmt.Errorf("failed to create empty disk image: %w", err)
	}
	log.Infof("Created image disk at %s with max size %d MiB", discFilePath, maxSizeMb)
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
func DetachLoopbackDevice(diskPath string, loopDevPath string) error {
	log := utils.Logger()
	log.Debugf("Detaching loopback device %s", loopDevPath)

	// Validate the loop device path
	if loopDevPath == "" {
		return fmt.Errorf("invalid loop device path")
	}

	// Wait for any pending disk IO operations to complete
	if err := diskutils.BlockOnDiskIO(loopDevPath); err != nil {
		return fmt.Errorf("failed to block on disk IO for loopback device %s: %w", loopDevPath, err)
	}

	// Call the Azure diskutils to detach the loopback device
	if err := diskutils.DetachLoopbackDevice(loopDevPath); err != nil {
		return fmt.Errorf("failed to detach loopback device: %w", err)
	}

	// Wait for the loopback device to be detached
	if err := waitForLoopbackToDetach(diskPath, loopDevPath); err != nil {
		return fmt.Errorf("failed to wait for loopback detach: %w", err)
	}

	log.Infof("Loopback device %s detached successfully from %s", diskPath, loopDevPath)

	return nil
}

// WaitForLoopbackDetach waits for the loopback device to be detached.
func waitForLoopbackToDetach(diskPath string, loopDevPath string) error {
	log := utils.Logger()
	log.Debugf("Waiting for loopback device %s to be detached from %s", diskPath, loopDevPath)
	// Validate the loop device path
	if diskPath == "" || loopDevPath == "" {
		return fmt.Errorf("invalid loop device or device path")
	}
	// Call the Azure diskutils to wait for the loopback device to be detached
	if err := diskutils.WaitForLoopbackToDetach(loopDevPath, diskPath); err != nil {
		return fmt.Errorf("failed to wait for loopback detach: %w", err)
	}
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
func PartitionImageDisc(path string, maxSize uint64, parts []config.PartitionInfo) (partDevPathMap map[string]string,
	partIDToFsTypeMap map[string]string, err error) {

	maxSizeMiB := maxSize / MiB // Convert maxSize to MiB for diskutils

	log := utils.Logger()
	log.Infof("Partitioning image disk at %s with max size %d MiB", path, maxSizeMiB)

	// Validate the image path
	if path == "" || maxSizeMiB == 0 {
		return nil, nil, fmt.Errorf("invalid image path or max size")
	}

	azParts, err := toAzurePartitions(parts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert partitions: %w", err)
	}

	cfg := azcfg.Disk{
		PartitionTableType: azcfg.PartitionTableTypeGpt,
		MaxSize:            maxSizeMiB, // Use MiB for diskutils
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

	log.Infof("Partitioned image disk %s with partitions: %v", path, partToDev)
	log.Infof("Partitioned image disk %s with filesystem map: %v", path, partIdToFs)
	log.Infof("Partitioned image disk %s with encrypted root: %v", path, encRoot)
	return partToDev, partIdToFs, nil
}

// toAzurePartitions converts a slice of PartitionInfo to a slice of azcfg.Partition.
func toAzurePartitions(parts []config.PartitionInfo) ([]azcfg.Partition, error) {
	azParts := make([]azcfg.Partition, len(parts))
	for i, p := range parts {
		StartBytes, err := convert.NormalizeSizeToBytes(p.Start)
		if err != nil {
			return azParts, fmt.Errorf("failed to normalize partition start size %s: %v", p.Start, err)
		}
		EndBytes, err := convert.NormalizeSizeToBytes(p.End)
		if err != nil {
			return azParts, fmt.Errorf("failed to normalize partition end size %s: %v", p.End, err)
		}

		azFlags := make([]azcfg.PartitionFlag, 0, len(p.Flags))
		for _, flag := range p.Flags {
			switch flag {
			case "boot":
				azFlags = append(azFlags, azcfg.PartitionFlagBoot)
			case "esp":
				azFlags = append(azFlags, azcfg.PartitionFlagESP)
			case "grub":
				azFlags = append(azFlags, azcfg.PartitionFlagGrub)
			case "bios_grub":
				azFlags = append(azFlags, azcfg.PartitionFlagBiosGrub)
			case "bios-grub":
				azFlags = append(azFlags, azcfg.PartitionFlagBiosGrubLegacy)
			case "dmroot":
				azFlags = append(azFlags, azcfg.PartitionFlagDeviceMapperRoot)
			}
		}

		// If the partition ID is "rootfs", set the Name to "ROOT"
		if p.ID == "rootfs" {
			p.Name = "ROOT"
		}
		azParts[i] = azcfg.Partition{
			ID:       p.ID,
			Name:     p.Name,
			FsType:   p.FsType,
			Start:    StartBytes / MiB,
			End:      EndBytes / MiB, // Convert to MiB
			TypeUUID: p.TypeGUID,
			Type:     p.Type,
			Flags:    azFlags,
		}
	}
	return azParts, nil
}

func GetUUID(diskPartitionPath string) (string, error) {
	cmd := fmt.Sprintf("blkid %s -s UUID -o value", diskPartitionPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return output, fmt.Errorf("failed to get partition UUID for %s: %w", diskPartitionPath, err)
	}
	return strings.TrimSpace(output), nil
}

func GetPartUUID(diskPartitionPath string) (string, error) {
	cmd := fmt.Sprintf("blkid %s -s PARTUUID -o value", diskPartitionPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return output, fmt.Errorf("failed to get partition UUID for %s: %w", diskPartitionPath, err)
	}
	return strings.TrimSpace(output), nil
}
