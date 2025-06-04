package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadYAMLTemplate(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `image:
  name: azl3-x86_64-edge
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

systemConfigs:
  - name: edge
    description: Default yml configuration for edge image
    packages:
      - openssh-server
      - docker-ce
      - vim
      - curl
      - wget
    kernel:
      version: "6.12"
      cmdline: "quiet splash"
`

	tmpFile, err := os.CreateTemp("", "test-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading
	template, err := LoadTemplate(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load YAML template: %v", err)
	}

	// Verify the loaded template
	if template.Target.OS != "azure-linux" {
		t.Errorf("expected OS 'azure-linux', got %s", template.Target.OS)
	}
	if template.Target.Dist != "azl3" {
		t.Errorf("expected dist 'azl3', got %s", template.Target.Dist)
	}
	if template.Target.Arch != "x86_64" {
		t.Errorf("expected arch 'x86_64', got %s", template.Target.Arch)
	}
	if len(template.GetPackages()) != 5 {
		t.Errorf("expected 5 packages, got %d", len(template.GetPackages()))
	}
	if template.Target.ImageType != "raw" {
		t.Errorf("expected imageType 'raw', got %s", template.Target.ImageType)
	}
	if template.GetKernel().Version != "6.12" {
		t.Errorf("expected kernel version '6.12', got %s", template.GetKernel().Version)
	}
}

func TestTemplateHelperMethods(t *testing.T) {
	template := &ImageTemplate{
		Image: struct {
			Name    string `yaml:"name"`
			Version string `yaml:"version"`
		}{
			Name:    "test-image",
			Version: "1.0.0",
		},
		Target: struct {
			OS        string `yaml:"os"`
			Dist      string `yaml:"dist"`
			Arch      string `yaml:"arch"`
			ImageType string `yaml:"imageType"`
		}{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "iso",
		},
		SystemConfigs: []SystemConfig{
			{
				Name:        "test-config",
				Description: "Test configuration",
				Packages:    []string{"package1", "package2"},
				Kernel: KernelConfig{
					Version: "6.12",
					Cmdline: "quiet",
				},
			},
		},
	}

	// Test provider name mapping
	providerName := template.GetProviderName()
	if providerName != "AzureLinux3" {
		t.Errorf("expected provider 'AzureLinux3', got %s", providerName)
	}

	// Test version mapping
	version := template.GetDistroVersion()
	if version != "3" {
		t.Errorf("expected version '3', got %s", version)
	}

	// Test package extraction
	packages := template.GetPackages()
	if len(packages) != 2 {
		t.Errorf("expected 2 packages, got %d", len(packages))
	}

	// Test kernel extraction
	kernel := template.GetKernel()
	if kernel.Version != "6.12" {
		t.Errorf("expected kernel version '6.12', got %s", kernel.Version)
	}

	// Test system config name extraction
	configName := template.GetSystemConfigName()
	if configName != "test-config" {
		t.Errorf("expected config name 'test-config', got %s", configName)
	}
}

func TestUnsupportedFileFormat(t *testing.T) {
	// Create a temporary file with unsupported extension
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("some content"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading should fail
	_, err = LoadTemplate(tmpFile.Name())
	if err == nil {
		t.Errorf("expected error for unsupported file format")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("expected unsupported file format error, got: %v", err)
	}
}

func TestUnsupportedDistribution(t *testing.T) {
	// This test verifies that schema validation catches invalid OS/dist combinations

	invalidYaml := `image:
  name: test
  version: "1.0.0"
target:
  os: azure-linux
  dist: emt3  # Invalid: azure-linux should only use azl3
  arch: x86_64
  imageType: raw
systemConfigs:
  - name: test
    packages: ["test"]
    kernel:
      version: "6.12"
`

	// This should fail during schema validation
	_, err := parseYAMLTemplate([]byte(invalidYaml))
	if err == nil {
		t.Errorf("expected schema validation error for invalid OS/dist combination")
	}
	if !strings.Contains(err.Error(), "template validation error") {
		t.Errorf("expected schema validation error, got: %v", err)
	}
}

func TestEmptySystemConfigs(t *testing.T) {
	// This test verifies that schema validation catches empty systemConfigs

	invalidYaml := `image:
  name: test
  version: "1.0.0"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfigs: []  # Invalid: minItems is 1
`

	// This should fail during schema validation
	_, err := parseYAMLTemplate([]byte(invalidYaml))
	if err == nil {
		t.Errorf("expected schema validation error for empty systemConfigs")
	}
}

func TestAllSupportedProviders(t *testing.T) {
	testCases := []struct {
		os       string
		dist     string
		expected string
		version  string
	}{
		{"azure-linux", "azl3", "AzureLinux3", "3"},
		{"emt", "emt3", "EMT3.0", "3.0"},
		{"elxr", "elxr12", "eLxr12", "12"},
	}

	for _, tc := range testCases {
		template := &ImageTemplate{
			Target: struct {
				OS        string `yaml:"os"`
				Dist      string `yaml:"dist"`
				Arch      string `yaml:"arch"`
				ImageType string `yaml:"imageType"`
			}{
				OS:        tc.os,
				Dist:      tc.dist,
				Arch:      "x86_64",
				ImageType: "iso",
			},
			SystemConfigs: []SystemConfig{
				{
					Name:     "test",
					Packages: []string{"test-package"},
					Kernel:   KernelConfig{Version: "6.12"},
				},
			},
		}

		// Test provider name
		providerName := template.GetProviderName()
		if providerName != tc.expected {
			t.Errorf("for %s/%s expected provider '%s', got '%s'", tc.os, tc.dist, tc.expected, providerName)
		}

		// Test version
		version := template.GetDistroVersion()
		if version != tc.version {
			t.Errorf("for %s/%s expected version '%s', got '%s'", tc.os, tc.dist, tc.version, version)
		}
	}
}
