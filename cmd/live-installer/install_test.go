package main

import (
	"testing"
	"github.com/open-edge-platform/os-image-composer/internal/config"
)

func TestInstall_NoDiskPath(t *testing.T) {
	template := &config.ImageTemplate{
		Disk: config.DiskConfig{Path: ""},
		Target: config.TargetConfig{OS: "ubuntu", Dist: "focal", Arch: "amd64"},
	}
	err := install(template, "/tmp/config", "/tmp/repo")
	if err == nil || err.Error() != "no target disk path specified in the template" {
		t.Errorf("Expected error for missing disk path, got: %v", err)
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
			Bootloader: config.BootloaderConfig{BootType: "legacy"},
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
