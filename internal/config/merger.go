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

	// Disk configurations - merge or override
	if len(userTemplate.DiskConfigs) > 0 {
		mergedTemplate.DiskConfigs = mergeDiskConfigs(defaultTemplate.DiskConfigs, userTemplate.DiskConfigs)
	}

	// System configurations - merge
	if len(userTemplate.SystemConfigs) > 0 {
		mergedTemplate.SystemConfigs = mergeSystemConfigs(defaultTemplate.SystemConfigs, userTemplate.SystemConfigs)
	}

	log.Infof("Successfully merged user and default configurations")
	log.Debugf("Merged template: name=%s, systemConfigs=%d, diskConfigs=%d",
		mergedTemplate.Image.Name, len(mergedTemplate.SystemConfigs), len(mergedTemplate.DiskConfigs))

	return &mergedTemplate, nil
}

// mergeDiskConfigs merges disk configurations, with user configs taking precedence
func mergeDiskConfigs(defaultConfigs, userConfigs []DiskConfig) []DiskConfig {
	if len(userConfigs) == 0 {
		return defaultConfigs
	}

	// For now, user disk configs completely override default ones
	// This can be made more sophisticated if needed
	return userConfigs
}

// mergeSystemConfigs merges system configurations by name
func mergeSystemConfigs(defaultConfigs, userConfigs []SystemConfig) []SystemConfig {
	log := logger.Logger()

	if len(userConfigs) == 0 {
		return defaultConfigs
	}

	// Create a map of default configs by name for easy lookup
	defaultConfigMap := make(map[string]SystemConfig)
	for _, config := range defaultConfigs {
		defaultConfigMap[config.Name] = config
	}

	var mergedConfigs []SystemConfig

	// Process user configs
	for _, userConfig := range userConfigs {
		if defaultConfig, exists := defaultConfigMap[userConfig.Name]; exists {
			// Merge this specific system config
			merged := mergeSystemConfig(defaultConfig, userConfig)
			mergedConfigs = append(mergedConfigs, merged)
			log.Debugf("Merged system config: %s", userConfig.Name)
		} else {
			// User config doesn't have a default counterpart, use as-is
			mergedConfigs = append(mergedConfigs, userConfig)
			log.Debugf("Added user-only system config: %s", userConfig.Name)
		}
	}

	// Add any default configs that weren't overridden by user
	for _, defaultConfig := range defaultConfigs {
		found := false
		for _, userConfig := range userConfigs {
			if userConfig.Name == defaultConfig.Name {
				found = true
				break
			}
		}
		if !found {
			mergedConfigs = append(mergedConfigs, defaultConfig)
			log.Debugf("Added default-only system config: %s", defaultConfig.Name)
		}
	}

	return mergedConfigs
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

	// Merge packages - user packages are added to default packages
	if len(userConfig.Packages) > 0 {
		merged.Packages = mergePackages(defaultConfig.Packages, userConfig.Packages)
	}

	// Merge kernel config
	merged.Kernel = mergeKernelConfig(defaultConfig.Kernel, userConfig.Kernel)

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

	mergedTemplatePretty, err := json.MarshalIndent(mergedTemplate, "", "  ")
	if err != nil {
		log.Debugf("Failed to pretty print merged template: %v", err)
	} else {
		log.Debugf("Final merged template:\n%s", string(mergedTemplatePretty))
	}

	log.Infof("Successfully created merged configuration with %d system configs and %d disk configs",
		len(mergedTemplate.SystemConfigs), len(mergedTemplate.DiskConfigs))

	return mergedTemplate, nil
}
