package imagedisc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
	"github.com/open-edge-platform/image-composer/internal/utils/slice"
)

var sizeSuffixesList = []string{"KiB", "MiB", "GiB", "K", "M", "G", "KB", "MB", "GB"}
var sizeBytesMap = []int{1024, 1048576, 1073741824, 1024, 1048576, 1073741824, 1000, 1000000, 1000000000}
var partitionTypeNameToGUID = map[string]string{
	"linux":            "0fc63daf-8483-4772-8e79-3d69d8477de4",
	"bios":             "21686148-6449-6e6f-744e-656564454649",
	"esp":              "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
	"xbootldr":         "bc13c2ff-59e6-4262-a352-b275fd6f7172",
	"linux-root-amd64": "4f68bce3-e8cd-4db1-96e7-fbcaf984b709",
	"linux-swap":       "0657fd6d-a4ab-43c4-84e5-0933c84b4f4f",
	"linux-home":       "933ac7e1-2eb4-4f13-b844-0e14e2aef915",
	"linux-srv":        "3b8f8425-20e0-4f3b-907f-1a25a76f98e8",
	"linux-var":        "4d21b016-b534-45c2-a9fb-5c16e091fd2d",
	"linux-tmp":        "7ec6f557-3bc5-4aca-b293-16ef5df639d1",
	"linux-lvm":        "e6d6d379-f507-44c2-a23c-238f2a3df928",
	"linux-raid":       "a19d880f-05fc-4d3b-a006-743f0f84911e",
	"linux-luks":       "ca7d7ccb-63ed-4c53-861c-1742536059cc",
	"linux-dm-crypt":   "7ffec5c9-2d00-49b7-8941-3ea10a5586b7",
}

// IsDigit checks if a string contains only digits
func IsDigit(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// VerifyFileSize checks and formats the file size string.
func VerifyFileSize(fileSize interface{}) (string, error) {
	switch v := fileSize.(type) {
	case int:
		return fmt.Sprintf("%dMiB", v), nil
	case string:
		if fileSize == "0" {
			return fileSize.(string), nil
		}
		pattern := regexp.MustCompile(`^(\d+)(.*)$`)
		match := pattern.FindStringSubmatch(v)
		if len(match) == 3 {
			num := match[1]
			if !IsDigit(num) {
				return "", fmt.Errorf("file size number incorrect: " + num)
			}
			sizeSuffix := match[2]
			if !slice.Contains(sizeSuffixesList, sizeSuffix) {
				return "", fmt.Errorf("file size suffix incorrect: " + sizeSuffix)
			} else {
				return v, nil
			}
		}
		return "", fmt.Errorf("file size format incorrect: " + v)
	default:
		return "", fmt.Errorf("unsupported fileSize type")
	}
}

// TranslateSizeStrToBytes converts a size string to bytes.
func TranslateSizeStrToBytes(sizeStr string) (int, error) {
	pattern := regexp.MustCompile(`^(\d+)(.*)$`)
	match := pattern.FindStringSubmatch(sizeStr)
	if len(match) == 3 {
		numStr := match[1]
		sizeSuffix := match[2]
		for i, s := range sizeSuffixesList {
			if sizeSuffix == s {
				num, err := strconv.Atoi(numStr)
				if err != nil {
					return 0, err
				}
				return sizeBytesMap[i] * num, nil
			}
		}
		return 0, fmt.Errorf("file size suffix incorrect: " + sizeSuffix)
	}
	return 0, fmt.Errorf("size format incorrect: " + sizeStr)
}

func CreateRawFile(filePath string, fileSize string) error {
	fileSizeStr, err := VerifyFileSize(fileSize)
	if err != nil {
		return err
	}
	fileDir := filepath.Dir(filePath)
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		if err := os.MkdirAll(fileDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", fileDir, err)
		}
	}
	cmd := fmt.Sprintf("fallocate -l %s %s", fileSizeStr, filePath)
	_, err = shell.ExecCmd(cmd, false, "", nil)
	return err
}

func GetDiskNameFromDiskPath(diskPath string) (string, error) {
	re := regexp.MustCompile(`^/dev/(.*)`)
	match := re.FindStringSubmatch(diskPath)
	if len(match) > 1 {
		return match[1], nil
	} else {
		return "", fmt.Errorf("failed to extract disk name from path: %s", diskPath)
	}
}

func DiskGetHwSectorSize(diskName string) (int, error) {
	cmd := fmt.Sprintf("cat /sys/block/%s/queue/hw_sector_size", diskName)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(output))
}

func DiskGetPhysicalBlockSize(diskName string) (int, error) {
	cmd := fmt.Sprintf("cat /sys/block/%s/queue/physical_block_size", diskName)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(output))
}

func DiskGetDevInfo(diskPath string) (map[string]interface{}, error) {
	cmd := fmt.Sprintf("lsblk %s --json --list --output NAME,PATH,PARTTYPE,FSTYPE,UUID,MOUNTPOINT,PARTUUID,PARTLABEL,TYPE", diskPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return nil, err
	}
	var partitionsInfo map[string]interface{}
	if err := json.Unmarshal([]byte(output), &partitionsInfo); err != nil {
		return nil, err
	}
	if blockDevices, ok := partitionsInfo["blockdevices"].([]interface{}); ok {
		for _, device := range blockDevices {
			dev := device.(map[string]interface{})
			if dev["path"] == diskPath {
				return dev, nil
			}
		}
	}
	return nil, errors.New("device not found")
}

func DiskGetPartitionsInfo(diskPath string) ([]map[string]interface{}, error) {
	cmd := fmt.Sprintf("lsblk %s --json --list --output NAME,PATH,PARTTYPE,FSTYPE,UUID,MOUNTPOINT,PARTUUID,PARTLABEL,TYPE", diskPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return nil, err
	}
	var partitionsInfo map[string]interface{}
	if err := json.Unmarshal([]byte(output), &partitionsInfo); err != nil {
		return nil, err
	}
	var partitions []map[string]interface{}
	if blockDevices, ok := partitionsInfo["blockdevices"].([]interface{}); ok {
		for _, device := range blockDevices {
			dev := device.(map[string]interface{})
			if dev["type"] == "part" {
				partitions = append(partitions, dev)
			}
		}
	}
	return partitions, nil
}

func DiskGetInfo(diskPath string) (map[string]interface{}, error) {
	cmd := fmt.Sprintf("fdisk -l %s", diskPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return nil, err
	}
	diskInfo := make(map[string]interface{})
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Disk "+diskPath) {
			diskInfo["device"] = diskPath
			sizeInfo := strings.Split(line, ":")
			if len(sizeInfo) > 1 {
				sizeInfoList := strings.Split(sizeInfo[1], ",")
				if len(sizeInfoList) > 2 {
					bytes, _ := strconv.Atoi(strings.Fields(sizeInfoList[1])[0])
					sectors, _ := strconv.Atoi(strings.Fields(sizeInfoList[2])[0])
					diskInfo["bytes"] = bytes
					diskInfo["sectors"] = sectors
					diskInfo["part_num"] = 0
					diskInfo["part_info"] = []map[string]interface{}{}
				}
			}
		} else if strings.Contains(line, "Sector size") {
			sizes := strings.Split(line, ":")
			if len(sizes) > 1 {
				logicalPhysical := strings.Split(sizes[1], "/")
				if len(logicalPhysical) == 2 {
					diskInfo["logical_size"] = strings.TrimSpace(logicalPhysical[0])
					diskInfo["physical_size"] = strings.TrimSpace(logicalPhysical[1])
				}
			}
		} else if strings.Contains(line, "Disklabel type") {
			diskInfo["part_table_type"] = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.Contains(line, "Disk identifier") {
			diskInfo["disk_id"] = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.Contains(line, diskPath) {
			partInfoList := strings.Fields(line)
			if len(partInfoList) >= 5 {
				partInfo := map[string]interface{}{
					"device":    partInfoList[0],
					"start_sec": partInfoList[1],
					"end_sec":   partInfoList[2],
					"sectors":   partInfoList[3],
					"size":      partInfoList[4],
					"type":      strings.Join(partInfoList[5:], " "),
				}
				diskInfo["part_info"] = append(diskInfo["part_info"].([]map[string]interface{}), partInfo)
				diskInfo["part_num"] = diskInfo["part_num"].(int) + 1
			}
		}
	}
	return diskInfo, nil
}

func IsDiskPartitionExist(diskPath string) (bool, error) {
	diskInfo, err := DiskGetInfo(diskPath)
	if err != nil {
		return false, err
	}
	if partInfo, ok := diskInfo["part_info"].([]map[string]interface{}); ok && len(partInfo) >= 1 {
		return true, nil
	}
	return false, nil
}

func CheckDiskIOStats(diskPath string) (bool, error) {
	ioIsBusy := false
	log := logger.Logger()
	diskName, err := GetDiskNameFromDiskPath(diskPath)
	if err != nil {
		return false, err
	}
	cmd := fmt.Sprintf("cat /proc/diskstats | grep %s*", diskName)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return false, err
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		ioStats := strings.Fields(line)
		if len(ioStats) < 14 {
			continue
		}
		ioInProgress := ioStats[11]
		if ioInProgress != "0" {
			ioIsBusy = true
		}

		log.Debugf(fmt.Sprintf("%s io stats: ", ioStats[2]))
		for i, key := range []string{
			"major_num", "minor_num", "dev_name", "read_completed", "read_merged", "read_sectors",
			"read_milliseconds", "write_completed", "write_merged", "write_sectors", "write_milliseconds",
			"io_in_progress", "io_milliseconds", "io_weighted_milliseconds"} {
			log.Debugf(fmt.Sprintf("	%s: %s", key, ioStats[i]))
		}
	}
	return ioIsBusy, nil
}

func TranslateSectorToBytes(diskName string, sectorOffset int) (int, error) {
	hwSectorSize, err := DiskGetHwSectorSize(diskName)
	if err != nil {
		return 0, err
	}
	return sectorOffset * hwSectorSize, nil
}

func GetAlignedSectorOffset(diskName string, sectorOffset int) (int, error) {
	hwSectorSize, err := DiskGetHwSectorSize(diskName)
	if err != nil {
		return 0, err
	}
	physicalBlockSize, err := DiskGetPhysicalBlockSize(diskName)
	if err != nil {
		return 0, err
	}
	if physicalBlockSize == hwSectorSize {
		return sectorOffset, nil
	}
	physicalBlockSectorNum := physicalBlockSize / hwSectorSize
	if sectorOffset%physicalBlockSectorNum == 0 {
		return sectorOffset, nil
	}
	return ((sectorOffset / physicalBlockSectorNum) + 1) * physicalBlockSectorNum, nil
}

func getSectorOffsetFromSize(diskName, sizeStr string) (int, error) {
	hwSectorSize, err := DiskGetHwSectorSize(diskName)
	if err != nil {
		return 0, err
	}
	physicalBlockSize, err := DiskGetPhysicalBlockSize(diskName)
	if err != nil {
		return 0, err
	}
	byteSize, err := TranslateSizeStrToBytes(sizeStr)
	if err != nil {
		return 0, err
	}
	if byteSize < physicalBlockSize {
		if byteSize%hwSectorSize == 0 {
			return byteSize / hwSectorSize, nil
		}
	} else if byteSize%physicalBlockSize == 0 {
		return byteSize / hwSectorSize, nil
	} else {
		alignedSize := ((byteSize / physicalBlockSize) + 1) * physicalBlockSize
		return alignedSize / hwSectorSize, nil
	}
	return 0, errors.New("invalid size alignment")
}

func PartitionTypeStrToGUID(partitionTypeStr string) (string, error) {
	if guid, ok := partitionTypeNameToGUID[partitionTypeStr]; ok {
		return guid, nil
	}
	return "", fmt.Errorf("partition type not found: %s", partitionTypeStr)
}

func PartitionGUIDToTypeStr(partitionGUID string) (string, error) {
	for k, v := range partitionTypeNameToGUID {
		if v == partitionGUID {
			return k, nil
		}
	}
	return "", fmt.Errorf("partition GUID not found: %s", partitionGUID)
}

func diskPartitionCreate(
	diskPath string,
	partitionNum int,
	partitionInfo config.PartitionInfo,
	partitionTableType string,
	partitionType string) (string, error) {

	partitionTypeList := []string{"primary", "extended", "logical"}
	fsTypeList := []string{"fat32", "fat16", "vfat", "ext2", "ext3", "ext4", "xfs", "linux-swap"}
	log := logger.Logger()

	// Partition info
	partitionName := partitionInfo.Name
	partitionID := partitionInfo.ID

	// Validate partition type
	if partitionTableType == "mbr" {
		if !slice.Contains(partitionTypeList, partitionType) {
			return "", fmt.Errorf("unknown partition type: %s", partitionType)
		}
	} else if partitionTableType == "gpt" {
		if partitionInfo.Name != "" {
			partitionType = partitionName
		}
	}

	if partitionName == "" && partitionID != "" {
		partitionName = partitionID
	}

	log.Infof(fmt.Sprintf("Creating partition %d on disk %s for %s", partitionNum, diskPath, partitionName))

	startSizeStr, err := VerifyFileSize(partitionInfo.Start)
	if err != nil {
		return "", fmt.Errorf("invalid start size: %v", err)
	}
	endSizeStr, err := VerifyFileSize(partitionInfo.End)
	if err != nil {
		return "", fmt.Errorf("invalid end size: %v", err)
	}

	if !slice.Contains(fsTypeList, partitionInfo.FsType) {
		return "", fmt.Errorf("unknown fs type: %s", partitionInfo.FsType)
	}

	diskName, err := GetDiskNameFromDiskPath(diskPath)
	if err != nil {
		return "", fmt.Errorf("failed to get disk name from path: %s", diskPath)
	}
	startSector, _ := getSectorOffsetFromSize(diskName, startSizeStr)
	var endSector int
	if partitionInfo.End == "0" {
		endSector = 0
	} else {
		endSector, _ = getSectorOffsetFromSize(diskName, endSizeStr)
		endSector--
	}

	if partitionType == "logical" {
		// extended partition takes one sector, the following logical partitions will be aligned to the next sector
		startSector++
		if endSector != 0 {
			endSector++
		}
	}

	startSectorStr := fmt.Sprintf("%ds", startSector)
	endSectorStr := fmt.Sprintf("%ds", endSector)
	log.Infof("Input partition start: " + startSizeStr + ", aligned start sector: " + startSectorStr)
	log.Infof("Input partition end: " + endSizeStr + ", aligned end sector: " + endSectorStr)

	// Create partition
	var sfdiskScript strings.Builder
	sfdiskScript.WriteString(fmt.Sprintf("start=%d\n", startSector))
	if endSector != 0 {
		size := endSector - startSector
		sfdiskScript.WriteString(fmt.Sprintf("size=%d\n", size))
	}

	// Set partition type
	if partitionTableType == "gpt" {
		// For GPT, use GUID
		typeGUID := partitionInfo.TypeGUID
		if typeGUID == "" && partitionInfo.Type != "" {
			typeGUID, _ = PartitionTypeStrToGUID(partitionInfo.Type)
		}
		if typeGUID != "" {
			sfdiskScript.WriteString(fmt.Sprintf("type=%s\n", typeGUID))
		}
		// Set partition name if provided
		if partitionName != "" {
			sfdiskScript.WriteString(fmt.Sprintf("name=\"%s\"\n", partitionName))
		}
	} else {
		// For MBR, use hex type code
		var typeCode string
		switch {
		case partitionType == "extended":
			typeCode = "5"
		case partitionInfo.FsType == "linux-swap":
			typeCode = "82"
		default:
			typeCode = "83" // Linux
		}
		sfdiskScript.WriteString(fmt.Sprintf("type=%s\n", typeCode))
	}

	// Handle boot flag
	for _, flag := range partitionInfo.Flags {
		if flag == "boot" {
			sfdiskScript.WriteString("bootable\n")
			break
		}
	}

	// Create the partition using sfdisk
	cmdStr := fmt.Sprintf("echo '%s' | sudo sfdisk --no-reread --append %s",
		sfdiskScript.String(), diskPath)
	_, err = shell.ExecCmd(cmdStr, false, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create partition %d: %v", partitionNum, err)
	}

	// Refresh partition table using partx
	cmdStr = fmt.Sprintf("partx -u %s", diskPath)
	_, err = shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to refresh partition table after creating partition %d: %v", partitionNum, err)
	}

	// Format partition
	var diskPartDev string
	if strings.Contains(diskPath, "loop") || strings.Contains(diskPath, "nvme") {
		diskPartDev = fmt.Sprintf("%sp%d", diskPath, partitionNum)
	} else {
		diskPartDev = fmt.Sprintf("%s%d", diskPath, partitionNum)
	}

	if partitionInfo.FsType == "fat32" || partitionInfo.FsType == "fat16" || partitionInfo.FsType == "vfat" {
		cmdStr = fmt.Sprintf("mkfs -t vfat %s", diskPartDev)
		_, err := shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			return "", fmt.Errorf("failed to format partition %d with fs type %s: %v", partitionNum, partitionInfo.FsType, err)
		}
	} else if partitionInfo.FsType == "ext2" || partitionInfo.FsType == "ext3" || partitionInfo.FsType == "ext4" || partitionInfo.FsType == "xfs" {
		var additionalFlags string
		switch partitionInfo.FsType {
		case "ext2":
			additionalFlags = "-b 4096 -O none,sparse_super,large_file,filetype,resize_inode,dir_index,ext_attr"
		case "ext3":
			additionalFlags = "-b 4096 -O none,sparse_super,large_file,filetype,resize_inode,dir_index,ext_attr,has_journal"
		case "ext4":
			additionalFlags = "-b 4096 -O none,sparse_super,large_file,filetype,resize_inode,dir_index,ext_attr,has_journal,extent,huge_file,flex_bg,metadata_csum,64bit,dir_nlink,extra_isize"
		}
		if additionalFlags != "" {
			cmdStr = fmt.Sprintf("mkfs -t %s %s %s", partitionInfo.FsType, additionalFlags, diskPartDev)
		} else {
			cmdStr = fmt.Sprintf("mkfs -t %s %s", partitionInfo.FsType, diskPartDev)
		}
		_, err := shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			return "", fmt.Errorf("failed to format partition %d with fs type %s: %v", partitionNum, partitionInfo.FsType, err)
		}
	} else if partitionInfo.FsType == "linux-swap" {
		cmdStr = fmt.Sprintf("mkswap %s", diskPartDev)
		_, err := shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			return "", fmt.Errorf("failed to format partition %d with fs type %s: %v", partitionNum, partitionInfo.FsType, err)
		}
		cmdStr = fmt.Sprintf("swapon %s", diskPartDev)
		_, err = shell.ExecCmd(cmdStr, true, "", nil)
		if err != nil {
			return "", fmt.Errorf("failed to enable swap on partition %d: %v", partitionNum, err)
		}
	}

	return diskPartDev, nil
}

func diskPartitionDelete(diskPath string, partitionNum int) error {
	log := logger.Logger()
	if partitionNum < 1 {
		return fmt.Errorf("invalid partition number: %d", partitionNum)
	}
	cmdStr := fmt.Sprintf("sfdisk --delete %s %d", diskPath, partitionNum)
	_, err := shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to delete partition %d: %v", partitionNum, err)
	}

	// Refresh partition table
	cmdStr = fmt.Sprintf("partx -d --nr %d %s", partitionNum, diskPath)
	_, err = shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		// Non-fatal if partition is already gone
		log.Warnf("Could not remove partition %d from kernel table: %v", partitionNum, err)
	}

	return nil
}

func DiskPartitionsCreate(diskPath string, partitionsList []config.PartitionInfo, partitionTableType string) (map[string]string, error) {
	partIDDiskDevMap := make(map[string]string)
	log := logger.Logger()

	partitionExist, err := IsDiskPartitionExist(diskPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if disk %s has partitions: %v", diskPath, err)
	}
	if partitionExist {
		// Wipe the disk first
		log.Infof(fmt.Sprintf("Disk %s already has partitions, wiping it before creating new partitions", diskPath))
		if err := DiskPartitionsWipe(diskPath, false); err != nil {
			return nil, fmt.Errorf("failed to wipe disk before creating partitions: %v", err)
		}
	}

	if partitionTableType == "gpt" {
		cmdStr := fmt.Sprintf("echo 'label: gpt' | sudo sfdisk %s", diskPath)
		_, err := shell.ExecCmd(cmdStr, false, "", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create GPT partition table on disk %s: %v", diskPath, err)
		}

		for i, partitionInfo := range partitionsList {
			partitionNum := i + 1
			diskPartDev, err := diskPartitionCreate(diskPath, partitionNum, partitionInfo, partitionTableType, "primary")
			if err != nil {
				for i := 1; i < partitionNum; i++ {
					// Clean up previously created partitions if any
					if err := diskPartitionDelete(diskPath, i); err != nil {
						log.Errorf(fmt.Sprintf("%v", err))
					}
				}
				return nil, fmt.Errorf("failed to create partition %d: %v", partitionNum, err)
			}
			partIDDiskDevMap[partitionInfo.ID] = diskPartDev
		}
	} else if partitionTableType == "mbr" {
		var partitionType string
		var partitionNum int
		maxPrimaryPartitionsNum := 4
		cmdStr := fmt.Sprintf("echo 'label: dos' | sudo sfdisk %s", diskPath)
		_, err := shell.ExecCmd(cmdStr, false, "", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create MBR partition table on disk %s: %v", diskPath, err)
		}

		partitionCount := len(partitionsList)
		for i, partitionInfo := range partitionsList {
			if i >= maxPrimaryPartitionsNum-1 && partitionCount > maxPrimaryPartitionsNum {
				// If we have more than 4 partitions, the last one will be an extended partition
				if i == maxPrimaryPartitionsNum-1 {
					partitionType = "extended"
					partitionNum = i + 1
					logicalPartitionEnd := partitionInfo.End
					extendedPartitionEnd := partitionsList[partitionCount-1].End
					partitionInfo.End = extendedPartitionEnd
					_, err := diskPartitionCreate(diskPath, partitionNum, partitionInfo, partitionTableType, partitionType)
					if err != nil {
						for i := 1; i < partitionNum; i++ {
							// Clean up previously created partitions if any
							if err := diskPartitionDelete(diskPath, i); err != nil {
								log.Errorf(fmt.Sprintf("%v", err))
							}
						}
						return nil, fmt.Errorf("failed to create extended partition %d: %v", partitionNum, err)
					}
					partitionInfo.End = logicalPartitionEnd
					partitionType = "logical"
					partitionNum = i + 1
				} else {
					// For logical partitions, we can create multiple logical partitions within the extended partition
					partitionType = "logical"
					partitionNum = i + 1
				}
			} else {
				// For primary partitions, we can create up to 4 primary partitions
				partitionType = "primary"
				partitionNum = i + 1
			}
			diskPartDev, err := diskPartitionCreate(diskPath, partitionNum, partitionInfo, partitionTableType, partitionType)
			if err != nil {
				for i := 1; i < partitionNum; i++ {
					// Clean up previously created partitions if any
					if err := diskPartitionDelete(diskPath, i); err != nil {
						log.Errorf(fmt.Sprintf("%v", err))
					}
				}
				return nil, fmt.Errorf("failed to create partition %d: %v", partitionNum, err)
			}
			partIDDiskDevMap[partitionInfo.ID] = diskPartDev
		}
	}
	return partIDDiskDevMap, nil
}

func DiskPartDevGetUUID(diskPartDev string, verbose bool) (string, error) {
	cmdStr := fmt.Sprintf("blkid %s -s UUID -o value", diskPartDev)
	uuid, err := shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get partition UUID for %s: %v", diskPartDev, err)
	}
	return strings.TrimSpace(uuid), nil
}

func DiskPartDevGetPartitionUUID(diskPartDev string, verbose bool) (string, error) {
	cmdStr := fmt.Sprintf("blkid %s -s PARTUUID -o value", diskPartDev)
	uuid, err := shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get partition UUID for %s: %v", diskPartDev, err)
	}
	return strings.TrimSpace(uuid), nil
}

func DiskPartDevGetPartitionLabel(diskPartDev string, verbose bool) (string, error) {
	cmdStr := fmt.Sprintf("blkid %s -s PARTLABEL -o value", diskPartDev)
	label, err := shell.ExecCmd(cmdStr, true, "", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get partition label for %s: %v", diskPartDev, err)
	}
	return strings.TrimSpace(label), nil
}

func DiskPartitionsWipe(diskPath string, verbose bool) error {
	// Wipe filesystem signatures
	_, err := shell.ExecCmd(fmt.Sprintf("wipefs -a -f %s", diskPath), true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to wipe disk %s: %v", diskPath, err)
	}

	_, err = shell.ExecCmd("sync", true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to sync after wiping disk %s: %v", diskPath, err)
	}
	return nil
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
