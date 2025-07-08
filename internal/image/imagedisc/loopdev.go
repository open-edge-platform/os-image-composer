package imagedisc

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func LoopSetupCreate(imagePath string) (string, error) {
	log := logger.Logger()
	cmd := fmt.Sprintf("losetup --direct-io=on --show -f -P %s", imagePath)
	loopDevPath, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return "", err
	}

	loopDevPath = strings.TrimSpace(loopDevPath)
	if strings.Contains(loopDevPath, "/dev/loop") {
		log.Infof(fmt.Sprintf("losetup %s created loopback device at %s\n", imagePath, loopDevPath))
		return loopDevPath, nil
	} else {
		return "", fmt.Errorf("failed to create loopback device for %s", imagePath)
	}
}

func LoopSetupCreateEmptyRawDisk(filePath, fileSize string) (string, error) {
	if err := CreateRawFile(filePath, fileSize); err != nil {
		return "", err
	}

	if _, err := os.Stat(filePath); err == nil {
		return LoopSetupCreate(filePath)
	}
	return "", fmt.Errorf("can't find %s", filePath)
}

func LoopSetupDelete(loopDevPath string) error {
	cmd := fmt.Sprintf("losetup -d %s", loopDevPath)
	_, err := shell.ExecCmd(cmd, true, "", nil)
	return err
}

func LoopDevGetInfo(loopDevPath string) (map[string]interface{}, error) {
	cmd := fmt.Sprintf("losetup -l %s --json", loopDevPath)
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}

	if devices, ok := result["loopdevices"].([]interface{}); ok && len(devices) > 0 {
		if info, ok := devices[0].(map[string]interface{}); ok {
			return info, nil
		}
	}
	return nil, fmt.Errorf("no loop device info found")
}

func LoopDevGetBackFile(loopDevPath string) (string, error) {
	info, err := LoopDevGetInfo(loopDevPath)
	if err != nil {
		return "", err
	}

	if backFile, ok := info["back-file"].(string); ok {
		return backFile, nil
	}
	return "", fmt.Errorf("back-file not found")
}

func LoopDevGetInfoAll() ([]map[string]interface{}, error) {
	cmd := "losetup -l --json"
	output, err := shell.ExecCmd(cmd, true, "", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}

	var list []map[string]interface{}
	if devices, ok := result["loopdevices"].([]interface{}); ok {
		for _, dev := range devices {
			if m, ok := dev.(map[string]interface{}); ok {
				list = append(list, m)
			}
		}
	}
	return list, nil
}

func GetLoopDevPathFromLoopDevPart(loopDevPart string) (string, error) {
	re := regexp.MustCompile(`^(/dev/loop\d+)p(\d+)`)
	match := re.FindStringSubmatch(loopDevPart)
	if len(match) > 1 {
		return match[1], nil
	} else {
		return "", fmt.Errorf("invalid loop device partition format: %s", loopDevPart)
	}
}

func CreateRawImage(filePath string, template *config.ImageTemplate) (string, map[string]string, error) {
	var diskPathIdMap map[string]string
	var loopDevPath string

	log := logger.Logger()
	diskInfo := template.GetDiskConfig()
	loopDevPath, err := LoopSetupCreateEmptyRawDisk(filePath, diskInfo.Size)
	if err != nil {
		return loopDevPath, diskPathIdMap, fmt.Errorf("failed to create loop device: %w", err)
	}
	diskPathIdMap, err = DiskPartitionsCreate(loopDevPath, diskInfo.Partitions, diskInfo.PartitionTableType)
	if err != nil {
		log.Errorf("failed to create partitions on loop device %s: %v", loopDevPath, err)
		goto fail
	}
	return loopDevPath, diskPathIdMap, nil

fail:
	errDel := LoopSetupDelete(loopDevPath)
	if errDel != nil {
		logger.Logger().Errorf("failed to delete loop device %s after error: %v", loopDevPath, errDel)
	}
	return loopDevPath, diskPathIdMap, fmt.Errorf("failed to create raw image: %w", err)
}
