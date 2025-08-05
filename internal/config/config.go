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

// ImageTemplate represents the YAML image template structure (unchanged)
type ImageTemplate struct {
	Image        ImageInfo    `yaml:"image"`
	Target       TargetInfo   `yaml:"target"`
	Disk         DiskConfig   `yaml:"disk,omitempty"`
	SystemConfig SystemConfig `yaml:"systemConfig"`
}

type Bootloader struct {
	BootType string `yaml:"bootType"` // BootType: type of bootloader (e.g., "efi", "legacy")
	Provider string `yaml:"provider"` // Provider: bootloader provider (e.g., "grub2", "systemd-boot")
}

// ImmutabilityConfig holds the immutability configuration
type ImmutabilityConfig struct {
	Enabled bool `yaml:"enabled"` // Enabled: whether immutability is enabled (default: false)
}

// UserConfig holds the user configuration
type UserConfig struct {
	Name           string   `yaml:"name"`                     // Name: username for the user account
	Password       string   `yaml:"password,omitempty"`       // Password: plain text password (discouraged for security)
	HashAlgo       string   `yaml:"hash_algo,omitempty"`      // HashAlgo: algorithm to be used to hash the password (e.g., "sha512", "bcrypt")
	PasswordMaxAge int      `yaml:"passwordMaxAge,omitempty"` // PasswordMaxAge: maximum password age in days (like /etc/login.defs PASS_MAX_DAYS)
	StartupScript  string   `yaml:"startupScript,omitempty"`  // StartupScript: shell/script to run on login
	Groups         []string `yaml:"groups,omitempty"`         // Groups: additional groups to add user to
	Sudo           bool     `yaml:"sudo,omitempty"`           // Sudo: whether to grant sudo permissions
	Home           string   `yaml:"home,omitempty"`           // Home: custom home directory path
	Shell          string   `yaml:"shell,omitempty"`          // Shell: login shell (e.g., /bin/bash, /bin/zsh)
}

// SystemConfig represents a system configuration within the template
type SystemConfig struct {
	Name            string               `yaml:"name"`
	Description     string               `yaml:"description"`
	Immutability    ImmutabilityConfig   `yaml:"immutability,omitempty"`
	Users           []UserConfig         `yaml:"users,omitempty"`
	Bootloader      Bootloader           `yaml:"bootloader"`
	Packages        []string             `yaml:"packages"`
	AdditionalFiles []AdditionalFileInfo `yaml:"additionalFiles"`
	Kernel          KernelConfig         `yaml:"kernel"`
}

// AdditionalFileInfo holds information about local file and final path to be placed in the image
type AdditionalFileInfo struct {
	Local string `yaml:"local"` // path to the file on the host system
	Final string `yaml:"final"` // path where the file should be placed in the image
}

// KernelConfig holds the kernel configuration
type KernelConfig struct {
	Version string `yaml:"version"`
	Cmdline string `yaml:"cmdline"`
}

// PartitionInfo holds information about a partition in the disk layout
type PartitionInfo struct {
	Name         string   `yaml:"name"`         // Name: label for the partition
	ID           string   `yaml:"id"`           // ID: unique identifier for the partition; can be used as a key
	Flags        []string `yaml:"flags"`        // Flags: optional flags for the partition (e.g., "boot", "hidden")
	Type         string   `yaml:"type"`         // Type: partition type (e.g., "esp", "linux-root-amd64")
	TypeGUID     string   `yaml:"typeUUID"`     // TypeGUID: GPT type GUID for the partition (e.g., "8300" for Linux filesystem)
	FsType       string   `yaml:"fsType"`       // FsType: filesystem type (e.g., "ext4", "xfs", etc.);
	Start        string   `yaml:"start"`        // Start: start offset of the partition; can be a absolute size (e.g., "512MiB")
	End          string   `yaml:"end"`          // End: end offset of the partition; can be a absolute size (e.g., "2GiB") or "0" for the end of the disk
	MountPoint   string   `yaml:"mountPoint"`   // MountPoint: optional mount point for the partition (e.g., "/boot", "/rootfs")
	MountOptions string   `yaml:"mountOptions"` // MountOptions: optional mount options for the partition (e.g., "defaults", "noatime")
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
	FullPkgList     []string
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
	TargetImageType = template.Target.ImageType
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

// Updated methods to work with single objects instead of arrays
func (t *ImageTemplate) GetDiskConfig() DiskConfig {
	return t.Disk
}

func (t *ImageTemplate) GetSystemConfig() SystemConfig {
	return t.SystemConfig
}

func (t *ImageTemplate) GetBootloaderConfig() Bootloader {
	return t.SystemConfig.Bootloader
}

// GetPackages returns all packages from the system configuration
func (t *ImageTemplate) GetPackages() []string {
	return t.SystemConfig.Packages
}

// GetKernel returns the kernel configuration from the system configuration
func (t *ImageTemplate) GetKernel() KernelConfig {
	return t.SystemConfig.Kernel
}

// GetSystemConfigName returns the name of the system configuration
func (t *ImageTemplate) GetSystemConfigName() string {
	return t.SystemConfig.Name
}

func SaveUpdatedConfigFile(path string, config *ImageTemplate) error {
	return nil
}

// GetImmutability returns the immutability configuration from systemConfig
func (t *ImageTemplate) GetImmutability() ImmutabilityConfig {
	return t.SystemConfig.Immutability
}

// IsImmutabilityEnabled returns whether immutability is enabled
func (t *ImageTemplate) IsImmutabilityEnabled() bool {
	return t.SystemConfig.Immutability.Enabled
}

// GetImmutability returns the immutability configuration (SystemConfig method)
func (sc *SystemConfig) GetImmutability() ImmutabilityConfig {
	return sc.Immutability
}

// IsImmutabilityEnabled returns whether immutability is enabled (SystemConfig method)
func (sc *SystemConfig) IsImmutabilityEnabled() bool {
	return sc.Immutability.Enabled
}

// GetUsers returns the user configurations from systemConfig
func (t *ImageTemplate) GetUsers() []UserConfig {
	return t.SystemConfig.Users
}

// GetUserByName returns a user configuration by name, or nil if not found
func (t *ImageTemplate) GetUserByName(name string) *UserConfig {
	for i := range t.SystemConfig.Users {
		if t.SystemConfig.Users[i].Name == name {
			return &t.SystemConfig.Users[i]
		}
	}
	return nil
}

// HasUsers returns whether any users are configured
func (t *ImageTemplate) HasUsers() bool {
	return len(t.SystemConfig.Users) > 0
}

// GetUsers returns the user configurations (SystemConfig method)
func (sc *SystemConfig) GetUsers() []UserConfig {
	return sc.Users
}

// GetUserByName returns a user configuration by name (SystemConfig method)
func (sc *SystemConfig) GetUserByName(name string) *UserConfig {
	for i := range sc.Users {
		if sc.Users[i].Name == name {
			return &sc.Users[i]
		}
	}
	return nil
}

// HasUsers returns whether any users are configured (SystemConfig method)
func (sc *SystemConfig) HasUsers() bool {
	return len(sc.Users) > 0
}
