package imagesecure

import (
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
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

	// updateImageFstab has set the rootfs to read-only in fstab - not additional code required
	// TODO: mounting overlay

	log.Debugf("Root filesystem made read-only successfully")
	return nil
}
