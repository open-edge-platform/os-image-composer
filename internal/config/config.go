// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config/validate"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"gopkg.in/yaml.v3"
)

type ImageInfo struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type TargetInfo struct {
	OS        string `yaml:"os"`
	Dist      string `yaml:"dist"`
	Arch      string `yaml:"arch"`
	ImageType string `yaml:"imageType"`
}

type ArtifactInfo struct {
	Type        string `yaml:"type"`
	Compression string `yaml:"compression"`
}

type DiskConfig struct {
	Name               string          `yaml:"name"`
	Artifacts          []ArtifactInfo  `yaml:"artifacts"`
	Size               string          `yaml:"size"`
	PartitionTableType string          `yaml:"partitionTableType"`
	Partitions         []PartitionInfo `yaml:"partitions"`
}

// ImageTemplate represents the YAML image template structure
type ImageTemplate struct {
	Image         ImageInfo      `yaml:"image"`
	Target        TargetInfo     `yaml:"target"`
	DiskConfigs   []DiskConfig   `yaml:"disk"`
	SystemConfigs []SystemConfig `yaml:"systemConfigs"`
}

// SystemConfig represents a system configuration within the template
type SystemConfig struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Packages    []string     `yaml:"packages"`
	Kernel      KernelConfig `yaml:"kernel"`
}

// KernelConfig holds the kernel configuration
type KernelConfig struct {
	Version string `yaml:"version"`
	Cmdline string `yaml:"cmdline"`
}

// PartitionInfo holds information about a partition in the disk layout
type PartitionInfo struct {
	Name       string   // Name: label for the partition
	ID         string   // ID: unique identifier for the partition; can be used as a key
	Flags      []string // Flags: optional flags for the partition (e.g., "boot", "hidden")
	TypeGUID   string   // TypeGUID: GPT type GUID for the partition (e.g., "8300" for Linux filesystem)
	FsType     string   // FsType: filesystem type (e.g., "ext4", "xfs", etc.);
	SizeBytes  uint64   // SizeBytes: size of the partition in bytes
	StartBytes uint64   // StartBytes: absolute start offset in bytes; if zero, partitions are laid out sequentially
	MountPoint string   // MountPoint: optional mount point for the partition (e.g., "/boot", "/rootfs")
}

// Disk Info holds information about the disk layout
type Disk struct {
	Name               string          `yaml:"name"`               // Name of the disk
	Compression        string          `yaml:"compression"`        // Compression type (e.g., "gzip", "zstd", "none")
	Size               uint64          `yaml:"size"`               // Size of the disk in bytes (4GB, 4GiB, 4096Mib also valid)
	PartitionTableType string          `yaml:"partitionTableType"` // Type of partition table (e.g., "gpt", "mbr")
	Partitions         []PartitionInfo `yaml:"partitions"`         // List of partitions to create in the disk image
}

var (
	TargetOs        string
	TargetDist      string
	TargetArch      string
	TargetImageType string
	ProviderId      string
)

// LoadTemplate loads an ImageTemplate from the specified YAML template path
func LoadTemplate(path string) (*ImageTemplate, error) {
	log := logger.Logger()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Only support YAML/YML files
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("unsupported file format: %s (only .yml and .yaml are supported)", ext)
	}

	template, err := parseYAMLTemplate(data)
	if err != nil {
		return nil, fmt.Errorf("loading YAML template: %w", err)
	}

	TargetOs = template.Target.OS
	TargetDist = template.Target.Dist
	TargetArch = template.Target.Arch
	log.Infof("loaded image template from %s: name=%s, os=%s, dist=%s, arch=%s",
		path, template.Image.Name, template.Target.OS, template.Target.Dist, template.Target.Arch)
	return template, nil
}

// parseYAMLTemplate loads an ImageTemplate from YAML data
func parseYAMLTemplate(data []byte) (*ImageTemplate, error) {
	// Parse YAML to generic interface for validation
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Convert to JSON for schema validation
	jsonData, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("converting to JSON for validation: %w", err)
	}

	// Validate against image template schema
	if err := validate.ValidateImageTemplateJSON(jsonData); err != nil {
		return nil, fmt.Errorf("template validation error: %w", err)
	}

	// Parse into template structure
	var template ImageTemplate
	if err := yaml.Unmarshal(data, &template); err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	return &template, nil
}

// GetProviderName returns the provider name for the given template
func (t *ImageTemplate) GetProviderName() string {
	// Map OS/dist combinations to provider names
	providerMap := map[string]map[string]string{
		"azure-linux": {"azl3": "AzureLinux3"},
		"emt":         {"emt3": "EMT3.0"},
		"elxr":        {"elxr12": "eLxr12"},
	}

	if providers, ok := providerMap[t.Target.OS]; ok {
		if provider, ok := providers[t.Target.Dist]; ok {
			return provider
		}
	}
	return ""
}

// GetDistroVersion returns the version string expected by providers
func (t *ImageTemplate) GetDistroVersion() string {
	versionMap := map[string]string{
		"azl3":   "3",
		"emt3":   "3.0",
		"elxr12": "12",
	}
	return versionMap[t.Target.Dist]
}

func (t *ImageTemplate) GetImageName() string {
	return t.Image.Name
}

func (t *ImageTemplate) GetTargetInfo() TargetInfo {
	return t.Target
}

func (t *ImageTemplate) GetDiskConfig() DiskConfig {
	if len(t.DiskConfigs) > 0 {
		return t.DiskConfigs[0]
	}
	return DiskConfig{}
}

func (t *ImageTemplate) GetSystemConfig() SystemConfig {
	if len(t.SystemConfigs) > 0 {
		return t.SystemConfigs[0]
	}
	return SystemConfig{}
}

// GetPackages returns all packages from the first system configuration
// TODO: In the future, we might want to support multiple configs or allow selection
func (t *ImageTemplate) GetPackages() []string {
	if len(t.SystemConfigs) > 0 {
		return t.SystemConfigs[0].Packages
	}
	return []string{}
}

// GetKernel returns the kernel configuration from the first system configuration
func (t *ImageTemplate) GetKernel() KernelConfig {
	if len(t.SystemConfigs) > 0 {
		return t.SystemConfigs[0].Kernel
	}
	return KernelConfig{}
}

// GetSystemConfigName returns the name of the first system configuration
func (t *ImageTemplate) GetSystemConfigName() string {
	if len(t.SystemConfigs) > 0 {
		return t.SystemConfigs[0].Name
	}
	return ""
}

func SaveUpdatedConfigFile(path string, config *ImageTemplate) error {
	return nil
}
