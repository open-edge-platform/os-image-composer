package main

import (
	"os"
	"testing"
	"github.com/open-edge-platform/os-image-composer/internal/config"
)

func TestNewChrootBuilder_InvalidConfigDir(t *testing.T) {
	_, err := newChrootBuilder("invalid_dir", "repo", "ubuntu", "focal", "amd64")
	if err == nil {
		t.Error("Expected error for invalid config dir, got nil")
	}
}

func TestDependencyCheck_UnsupportedOS(t *testing.T) {
	err := dependencyCheck("unsupported-os")
	if err == nil {
		t.Error("Expected error for unsupported OS, got nil")
	}
}

func TestUpdateBootOrder_NonEFI(t *testing.T) {
	template := &config.ImageTemplate{
		SystemConfig: config.SystemConfig{
			Bootloader: config.BootloaderConfig{
				BootType: "legacy",
			},
		},
	}
	err := updateBootOrder(template, map[string]string{})
	if err != nil {
		t.Errorf("Expected nil error for non-efi boot type, got %v", err)
	}
}

func TestCreateNewBootEntry_NoDiskPath(t *testing.T) {
	template := &config.ImageTemplate{}
	err := createNewBootEntry(template, map[string]string{})
	if err == nil {
		t.Error("Expected error for missing disk path, got nil")
	}
}
