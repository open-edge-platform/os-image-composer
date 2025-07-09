package imagesecure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

func ConfigImageSecurity(installRoot string, template *config.ImageTemplate) error {

	log := logger.Logger()

	// 0. Check if the input indicates immutable rootfs
	result := ""
	prtCfg := template.GetDiskConfig()
	for _, p := range prtCfg.Partitions {
		if p.Type == "linux-root-amd64" || p.ID == "rootfs" || p.Name == "rootfs" {
			result = p.MountOptions
		}
	}

	hasRO := false
	for _, opt := range strings.Split(result, ",") {
		if strings.TrimSpace(opt) == "ro" {
			hasRO = true
			break
		}
	}

	if !hasRO { // no further action if immutable rootfs is not enable
		return nil
	}

	// 1. make rootfs read-only
	err := makeRootfsReadOnly(installRoot)
	if err != nil {
		return fmt.Errorf("failed to create readonly rootfs: %w", err)
	}

	log.Debugf("Root filesystem made read-only successfully")
	return nil
}

// makeRootfsReadOnly modifies the fstab to make the root filesystem read-only
func makeRootfsReadOnly(installRoot string) error {
	log := logger.Logger()

	// Path to the fstab file in the target rootfs
	fstabPath := filepath.Join(installRoot, "etc", "fstab")

	log.Debugf("Reading fstab file: %s", fstabPath)

	// Read the current fstab content
	readCmd := fmt.Sprintf("cat %s", fstabPath)
	fstabContent, err := shell.ExecCmd(readCmd, true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to read fstab file %s: %w", fstabPath, err)
	}

	if strings.TrimSpace(fstabContent) == "" {
		return fmt.Errorf("fstab file %s is empty", fstabPath)
	}

	// Process each line in fstab
	lines := strings.Split(fstabContent, "\n")
	var modifiedLines []string
	rootfsModified := false

	for _, line := range lines {
		// Skip empty lines and comments
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			modifiedLines = append(modifiedLines, line)
			continue
		}

		// Parse fstab entry: device mountpoint fstype options dump pass
		fields := strings.Fields(trimmedLine)
		if len(fields) < 4 {
			// Invalid fstab entry, keep as is
			modifiedLines = append(modifiedLines, line)
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]
		options := fields[3]
		dump := "0"
		pass := "0"

		if len(fields) >= 5 {
			dump = fields[4]
		}
		if len(fields) >= 6 {
			pass = fields[5]
		}

		// Check if this is the root filesystem entry
		if mountPoint == "/" {
			log.Debugf("Found root filesystem entry: %s", line)

			// Add 'ro' option to make it read-only
			if !strings.Contains(options, "ro") {
				// Remove any 'rw' option if present and add 'ro'
				optionList := strings.Split(options, ",")
				var newOptions []string

				for _, opt := range optionList {
					if opt != "rw" {
						newOptions = append(newOptions, opt)
					}
				}
				newOptions = append(newOptions, "ro")
				options = strings.Join(newOptions, ",")
				rootfsModified = true
				log.Debugf("Modified root filesystem options to: %s", options)
			}
		}

		// Reconstruct the fstab entry
		newLine := fmt.Sprintf("%s %s %s %s %s %s", device, mountPoint, fsType, options, dump, pass)
		modifiedLines = append(modifiedLines, newLine)
	}

	if !rootfsModified {
		log.Warnf("No root filesystem entry found in fstab or it was already read-only")
		return nil
	}

	// Write the modified fstab back
	modifiedContent := strings.Join(modifiedLines, "\n")

	// Create a temporary file with the new content
	tempFstabPath := fstabPath + ".tmp"
	if err := os.WriteFile(tempFstabPath, []byte(modifiedContent), 0644); err != nil {
		return fmt.Errorf("failed to write temporary fstab file %s: %w", tempFstabPath, err)
	}

	// Move the temporary file to replace the original fstab
	mvCmd := fmt.Sprintf("mv %s %s", tempFstabPath, fstabPath)
	if _, err := shell.ExecCmd(mvCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to replace fstab file: %w", err)
	}

	log.Infof("Successfully modified fstab to make root filesystem read-only")
	return nil
}
