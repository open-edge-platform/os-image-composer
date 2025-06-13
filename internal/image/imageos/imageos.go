package imageos

import (
	"fmt"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/imagesecure"
	"github.com/open-edge-platform/image-composer/internal/image/imagesign"
)

func InstallImageOs(diskPath string, template *config.ImageTemplate) error {
	installRoot := "" // This is a placeholder for the install root path.
	err := preImageOsInstall(installRoot, template)
	if err != nil {
		return fmt.Errorf("pre-install failed: %w", err)
	}
	err = installImagePkgs(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to install image packages: %w", err)
	}
	err = updateImageConfig(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image config: %w", err)
	}
	err = imagesecure.ConfigImageSecurity(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to configure image security: %w", err)
	}
	err = imagesign.SignImage(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to sign image: %w", err)
	}
	err = postImageOsInstall(installRoot, template)
	if err != nil {
		return fmt.Errorf("post-install failed: %w", err)
	}
	return nil
}

func preImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func installImagePkgs(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageConfig(installRoot string, template *config.ImageTemplate) error {
	err := updateImageHostname(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image hostname: %w", err)
	}
	err = updateImageUsrGroup(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image user/group: %w", err)
	}
	err = updateImageNetwork(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to update image network: %w", err)
	}
	err = addImageAdditionalFiles(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to add additional files to image: %w", err)
	}
	err = configImageUKI(installRoot, template)
	if err != nil {
		return fmt.Errorf("failed to configure UKI: %w", err)
	}
	return nil
}

func postImageOsInstall(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageHostname(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageUsrGroup(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func updateImageNetwork(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func addImageAdditionalFiles(installRoot string, template *config.ImageTemplate) error {
	return nil
}

func configImageUKI(installRoot string, template *config.ImageTemplate) error {
	return nil
}
