package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imagedisc"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

func getCurrentDirPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current directory path")
	}
	return filepath.Dir(filename), nil
}

func main() {
	log := logger.Logger()
	currentDir, err := getCurrentDirPath()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}
	buildDir := filepath.Join(currentDir, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			log.Fatalf("Failed to create build directory: %v", err)
		}
	}
	log.Infof("Build directory initialized at: %s", buildDir)
	imageFile := filepath.Join(buildDir, "image.raw")

	template := config.ImageTemplate{}
	template.Disk = config.DiskConfig{
		Name:               "disk1",
		Size:               "2GiB",
		PartitionTableType: "gpt",
		Partitions: []config.PartitionInfo{
			{
				Name:       "boot",
				ID:         "boot",
				Type:       "esp",
				Start:      "1MiB",
				End:        "513MiB",
				FsType:     "fat32",
				MountPoint: "/boot/efi",
			},
			{
				Name:       "rootfs",
				ID:         "rootfs",
				Type:       "linux-root-amd64",
				Start:      "513MiB",
				End:        "0",
				FsType:     "ext4",
				MountPoint: "/",
			},
		},
	}
	loopDevPath, diskPathIdMap, err := imagedisc.CreateRawImage(imageFile, &template)
	if err != nil {
		log.Fatalf("Failed to create raw image: %v", err)
	}
	for diskPath, id := range diskPathIdMap {
		log.Infof("Disk created: %s with ID: %s", diskPath, id)
	}

	err = imagedisc.DetachLoopbackDevice(imageFile, loopDevPath)
	if err != nil {
		log.Fatalf("Failed to detach loopback device: %v", err)
	}
	log.Infof("Image creation completed successfully. Image file: %s", imageFile)
}
