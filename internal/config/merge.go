package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/utils/file"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// DefaultConfigLoader handles loading and merging default configurations
type DefaultConfigLoader struct {
	targetOs   string
	targetDist string
	targetArch string
}

// NewDefaultConfigLoader creates a new config loader
func NewDefaultConfigLoader(os, dist, arch string) *DefaultConfigLoader {
	return &DefaultConfigLoader{
		targetOs:   os,
		targetDist: dist,
		targetArch: arch,
	}
}

// LoadDefaultConfig loads the appropriate default configuration based on image type
func (d *DefaultConfigLoader) LoadDefaultConfig(imageType string) (*ImageTemplate, error) {
	log := logger.Logger()

	// Determine the default config file based on image type
	var defaultConfigFile string
	switch imageType {
	case "raw":
		defaultConfigFile = fmt.Sprintf("default-raw-%s.yml", d.targetArch)
	case "iso":
		defaultConfigFile = fmt.Sprintf("default-iso-%s.yml", d.targetArch)
	default:
		return nil, fmt.Errorf("unsupported image type: %s", imageType)
	}

	// Get the target OS config directory
	targetOsConfigDir, err := file.GetTargetOsConfigDir(d.targetOs, d.targetDist)
	if err != nil {
		return nil, fmt.Errorf("failed to get target OS config directory: %w", err)
	}

	// Build the full path to the default config
	defaultConfigPath := filepath.Join(targetOsConfigDir, "imageconfigs", "defaultconfigs", defaultConfigFile)

	log.Infof("Loading default configuration from: %s", defaultConfigPath)

	// Check if the file exists
	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("default config file not found: %s", defaultConfigPath)
	}

	// Load the default configuration
	defaultTemplate, err := LoadTemplate(defaultConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load default configuration: %w", err)
	}

	log.Infof("Successfully loaded default configuration for %s/%s/%s-%s",
		d.targetOs, d.targetDist, d.targetArch, imageType)

	return defaultTemplate, nil
}

// MergeConfigurations merges user template with default configuration
// User configuration takes precedence over default configuration
func MergeConfigurations(userTemplate, defaultTemplate *ImageTemplate) (*ImageTemplate, error) {
	log := logger.Logger()

	if userTemplate == nil {
		return nil, fmt.Errorf("user template cannot be nil")
	}

	if defaultTemplate == nil {
		log.Warn("Default template is nil, using user template as-is")
		return userTemplate, nil
	}

	// Start with a copy of the default template
	mergedTemplate := *defaultTemplate

	// Override with user-specified values
	// Image section - always use user values if provided
	if userTemplate.Image.Name != "" {
		mergedTemplate.Image.Name = userTemplate.Image.Name
	}
	if userTemplate.Image.Version != "" {
		mergedTemplate.Image.Version = userTemplate.Image.Version
	}

	// Target section - always use user values (these should be consistent)
	mergedTemplate.Target = userTemplate.Target

	// Disk configuration - simple override if user provides one
	if !isEmptyDiskConfig(userTemplate.Disk) {
		mergedTemplate.Disk = userTemplate.Disk
		log.Debugf("User disk config overrides default")
	}

	// System configuration - merge intelligently
	if !isEmptySystemConfig(userTemplate.SystemConfig) {
		mergedTemplate.SystemConfig = mergeSystemConfig(defaultTemplate.SystemConfig, userTemplate.SystemConfig)
		log.Debugf("Merged system config: %s", mergedTemplate.SystemConfig.Name)
	}

	log.Infof("Successfully merged user and default configurations")

	// Debug mode: Pretty print the merged template
	if IsDebugMode() {
		pretty, err := json.MarshalIndent(mergedTemplate, "", "  ")
		if err != nil {
			log.Warnf("Failed to pretty print merged template: %v", err)
		} else {
			log.Debugf("Merged Template:\n%s", string(pretty))
		}
	}

	log.Debugf("Merged template: name=%s, systemConfig=%s",
		mergedTemplate.Image.Name, mergedTemplate.SystemConfig.Name)

	return &mergedTemplate, nil
}

// mergeSystemConfig merges a single system configuration
func mergeSystemConfig(defaultConfig, userConfig SystemConfig) SystemConfig {
	merged := defaultConfig // Start with default

	// Override with user values where provided
	if userConfig.Name != "" {
		merged.Name = userConfig.Name
	}
	if userConfig.Description != "" {
		merged.Description = userConfig.Description
	}

	// Merge bootloader config
	if !isEmptyBootloader(userConfig.Bootloader) {
		merged.Bootloader = mergeBootloader(defaultConfig.Bootloader, userConfig.Bootloader)
	}

	// Merge packages - user packages are added to default packages
	if len(userConfig.Packages) > 0 {
		merged.Packages = mergePackages(defaultConfig.Packages, userConfig.Packages)
	}

	// Merge kernel config
	merged.Kernel = mergeKernelConfig(defaultConfig.Kernel, userConfig.Kernel)

	return merged
}

// mergeBootloader merges bootloader configurations
func mergeBootloader(defaultBootloader, userBootloader Bootloader) Bootloader {
	merged := defaultBootloader

	if userBootloader.BootType != "" {
		merged.BootType = userBootloader.BootType
	}
	if userBootloader.Provider != "" {
		merged.Provider = userBootloader.Provider
	}

	return merged
}

// mergePackages combines default and user packages, removing duplicates
func mergePackages(defaultPackages, userPackages []string) []string {
	// Create a set to track unique packages
	packageSet := make(map[string]bool)
	var mergedPackages []string

	// Add default packages first
	for _, pkg := range defaultPackages {
		if !packageSet[pkg] {
			packageSet[pkg] = true
			mergedPackages = append(mergedPackages, pkg)
		}
	}

	// Add user packages, avoiding duplicates
	for _, pkg := range userPackages {
		if !packageSet[pkg] {
			packageSet[pkg] = true
			mergedPackages = append(mergedPackages, pkg)
		}
	}

	return mergedPackages
}

// mergeKernelConfig merges kernel configurations
func mergeKernelConfig(defaultKernel, userKernel KernelConfig) KernelConfig {
	merged := defaultKernel // Start with default

	// Override with user values where provided
	if userKernel.Version != "" {
		merged.Version = userKernel.Version
	}
	if userKernel.Cmdline != "" {
		merged.Cmdline = userKernel.Cmdline
	}

	return merged
}

// Helper functions to check if structures are empty

func isEmptyDiskConfig(disk DiskConfig) bool {
	return disk.Name == "" && disk.Size == "" && len(disk.Partitions) == 0
}

func isEmptySystemConfig(config SystemConfig) bool {
	return config.Name == ""
}

func isEmptyBootloader(bootloader Bootloader) bool {
	return bootloader.BootType == "" && bootloader.Provider == ""
}

// LoadAndMergeTemplate loads a user template and merges it with the appropriate default config
func LoadAndMergeTemplate(templatePath string) (*ImageTemplate, error) {
	log := logger.Logger()

	// Load the user template first
	userTemplate, err := LoadTemplate(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load user template: %w", err)
	}

	log.Infof("Loaded user template: %s (type: %s)", userTemplate.Image.Name, userTemplate.Target.ImageType)

	// Create default config loader
	loader := NewDefaultConfigLoader(userTemplate.Target.OS, userTemplate.Target.Dist, userTemplate.Target.Arch)

	// Load the appropriate default configuration
	defaultTemplate, err := loader.LoadDefaultConfig(userTemplate.Target.ImageType)
	if err != nil {
		log.Warnf("Could not load default configuration: %v", err)
		log.Info("Proceeding with user template only")
		return userTemplate, nil
	}

	// Merge configurations
	mergedTemplate, err := MergeConfigurations(userTemplate, defaultTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configurations: %w", err)
	}

	log.Infof("Successfully created merged configuration with system config: %s and disk config: %s",
		mergedTemplate.SystemConfig.Name, mergedTemplate.Disk.Name)

	return mergedTemplate, nil
}
