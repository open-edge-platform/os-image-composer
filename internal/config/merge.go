package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/utils/slice"
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

	// Determine the default config file based on image type
	var defaultConfigFile string
	switch imageType {
	case "raw":
		defaultConfigFile = fmt.Sprintf("default-raw-%s.yml", d.targetArch)
	case "img":
		defaultConfigFile = fmt.Sprintf("default-initrd-%s.yml", d.targetArch)
	case "iso":
		defaultConfigFile = fmt.Sprintf("default-iso-%s.yml", d.targetArch)
	default:
		log.Errorf("Unsupported image type: %s", imageType)
		return nil, fmt.Errorf("unsupported image type: %s", imageType)
	}

	// Get the target OS config directory
	targetOsConfigDir, err := GetTargetOsConfigDir(d.targetOs, d.targetDist)
	if err != nil {
		return nil, fmt.Errorf("failed to get target OS config directory: %w", err)
	}

	// Build the full path to the default config
	defaultConfigPath := filepath.Join(targetOsConfigDir, "imageconfigs", "defaultconfigs", defaultConfigFile)

	log.Infof("Loading default configuration from: %s", defaultConfigPath)

	// Check if the file exists
	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		log.Errorf("Default config file not found: %s", defaultConfigPath)
		return nil, fmt.Errorf("default config file not found: %s", defaultConfigPath)
	}

	// Load the default configuration
	defaultTemplate, err := LoadTemplate(defaultConfigPath, true)
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

	if userTemplate == nil {
		log.Errorf("User template cannot be nil")
		return nil, fmt.Errorf("user template cannot be nil")
	}

	// If no default template, use user template as-is
	if defaultTemplate == nil {
		log.Warn("Default template is nil, using user template as-is")
		return userTemplate, nil
	}

	// Start with a copy of the default template
	mergedTemplate := *defaultTemplate

	// Update the template path list
	for _, path := range userTemplate.PathList {
		if !slice.Contains(mergedTemplate.PathList, path) {
			mergedTemplate.PathList = append(mergedTemplate.PathList, path)
		}
	}

	// Override with user-specified values
	// Image section - always use user values if provided
	if userTemplate.Image.Name != "" {
		mergedTemplate.Image.Name = userTemplate.Image.Name
	}
	if userTemplate.Image.Version != "" {
		mergedTemplate.Image.Version = userTemplate.Image.Version
	}

	mergedTemplate.Target = userTemplate.Target

	// Disk configuration - user override if provided
	if !isEmptyDiskConfig(userTemplate.Disk) {
		mergedTemplate.Disk = userTemplate.Disk
		log.Debugf("User disk config overrides default")
	}

	// System configuration - merge intelligently
	if !isEmptySystemConfig(userTemplate.SystemConfig) {
		mergedTemplate.SystemConfig = mergeSystemConfig(defaultTemplate.SystemConfig, userTemplate.SystemConfig)
		log.Debugf("Merged system config: %s", mergedTemplate.SystemConfig.Name)
	} else {
		// Use default system config if user didn't provide one
		mergedTemplate.SystemConfig = defaultTemplate.SystemConfig
	}

	// Package repositories - merge intelligently
	mergedTemplate.PackageRepositories = mergePackageRepositories(
		defaultTemplate.PackageRepositories,
		userTemplate.PackageRepositories,
	)
	if len(mergedTemplate.PackageRepositories) > 0 {
		log.Debugf("Merged %d package repositories", len(mergedTemplate.PackageRepositories))
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

	log.Debugf("Merged template: name=%s, systemConfig=%s, immutability=%t users=%d",
		mergedTemplate.Image.Name, mergedTemplate.SystemConfig.Name, mergedTemplate.IsImmutabilityEnabled(), len(mergedTemplate.GetUsers()))

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

	if userConfig.HostName != "" {
		merged.HostName = userConfig.HostName
	}

	if userConfig.Initramfs.Template != "" {
		merged.Initramfs.Template = userConfig.Initramfs.Template
	}

	// Merge immutability config
	merged.Immutability = mergeImmutabilityConfig(defaultConfig.Immutability, userConfig.Immutability)

	// Merge users config
	if len(userConfig.Users) > 0 {
		merged.Users = mergeUsers(defaultConfig.Users, userConfig.Users)
	}

	if len(userConfig.AdditionalFiles) > 0 {
		merged.AdditionalFiles = mergeAdditionalFiles(defaultConfig.AdditionalFiles, userConfig.AdditionalFiles)
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

// mergeImmutabilityConfig merges immutability configurations including secure boot settings
func mergeImmutabilityConfig(defaultImmutability, userImmutability ImmutabilityConfig) ImmutabilityConfig {
	merged := defaultImmutability // Start with default

	// User configuration takes precedence
	// If user explicitly sets enabled (either true or false), use that value
	// Otherwise, keep the default value
	if userImmutability.Enabled != defaultImmutability.Enabled {
		merged.Enabled = userImmutability.Enabled
	}

	// Merge secure boot configuration - user values override defaults
	if userImmutability.SecureBootDBKey != "" {
		merged.SecureBootDBKey = userImmutability.SecureBootDBKey
	}

	if userImmutability.SecureBootDBCrt != "" {
		merged.SecureBootDBCrt = userImmutability.SecureBootDBCrt
	}

	if userImmutability.SecureBootDBCer != "" {
		merged.SecureBootDBCer = userImmutability.SecureBootDBCer
	}

	return merged
}

func mergeAdditionalFiles(defaultFiles, userFiles []AdditionalFileInfo) []AdditionalFileInfo {
	// Create a map to track unique additional files by their final path
	fileMap := make(map[string]AdditionalFileInfo)

	// Add default files first
	for _, file := range defaultFiles {
		fileMap[file.Final] = file
	}

	// Add/override with user files
	for _, file := range userFiles {
		fileMap[file.Final] = file
	}

	// Convert map back to slice
	mergedFiles := make([]AdditionalFileInfo, 0, len(fileMap))
	for _, file := range fileMap {
		mergedFiles = append(mergedFiles, file)
	}

	return mergedFiles
}

// mergeUsers merges user configurations
func mergeUsers(defaultUsers, userUsers []UserConfig) []UserConfig {
	merged := make([]UserConfig, 0, len(defaultUsers)+len(userUsers))
	userMap := make(map[string]UserConfig)

	// Create a map of user configurations by name for easy lookup
	for _, user := range userUsers {
		userMap[user.Name] = user
	}

	// Add default users, but override with user values if same name exists
	for _, defaultUser := range defaultUsers {
		if userUser, exists := userMap[defaultUser.Name]; exists {
			// User exists in both default and user config - merge them
			merged = append(merged, mergeUserConfig(defaultUser, userUser))
			delete(userMap, defaultUser.Name) // Remove from map to avoid duplicate addition
		} else {
			// User only exists in default config
			merged = append(merged, defaultUser)
		}
	}

	// Add remaining user-only configurations
	for _, userUser := range userMap {
		merged = append(merged, userUser)
	}

	return merged
}

// mergeUserConfig merges individual user configurations
func mergeUserConfig(defaultUser, userUser UserConfig) UserConfig {
	merged := defaultUser // Start with default

	// Override basic fields
	if userUser.Name != "" {
		merged.Name = userUser.Name
	}

	// Password and hash algorithm merging logic
	if userUser.Password != "" {
		merged.Password = userUser.Password

		// If user provides hash_algo, use it
		if userUser.HashAlgo != "" {
			merged.HashAlgo = userUser.HashAlgo
		}

		// Special case: if user provides pre-hashed password, clear hash_algo
		if strings.HasPrefix(userUser.Password, "$") {
			merged.HashAlgo = ""
		}
	} else if userUser.HashAlgo != "" {
		// User only changed algorithm for default password
		merged.HashAlgo = userUser.HashAlgo
	}

	// Other field merging
	if userUser.PasswordMaxAge != 0 {
		merged.PasswordMaxAge = userUser.PasswordMaxAge
	}
	if userUser.StartupScript != "" {
		merged.StartupScript = userUser.StartupScript
	}
	if userUser.Home != "" {
		merged.Home = userUser.Home
	}
	if userUser.Shell != "" {
		merged.Shell = userUser.Shell
	}

	// Merge groups
	if len(userUser.Groups) > 0 {
		merged.Groups = mergeStringSlices(defaultUser.Groups, userUser.Groups)
	}

	// Override sudo setting
	merged.Sudo = userUser.Sudo

	return merged
}

// mergeStringSlices combines two string slices, removing duplicates
func mergeStringSlices(defaultSlice, userSlice []string) []string {
	itemSet := make(map[string]bool)
	var merged []string

	// Add default items first
	for _, item := range defaultSlice {
		if !itemSet[item] {
			itemSet[item] = true
			merged = append(merged, item)
		}
	}

	// Add user items, avoiding duplicates
	for _, item := range userSlice {
		if !itemSet[item] {
			itemSet[item] = true
			merged = append(merged, item)
		}
	}

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
// Note: User KernelConfig only has Version and Cmdline, but merged config
// can include additional fields (name, uki) from defaults
func mergeKernelConfig(defaultKernel, userKernel KernelConfig) KernelConfig {
	merged := defaultKernel // Start with default (may include name, uki)

	// Override with user values where provided
	if userKernel.Version != "" {
		merged.Version = userKernel.Version
	}
	if userKernel.Cmdline != "" {
		merged.Cmdline = userKernel.Cmdline
	}

	if len(userKernel.Packages) > 0 {
		merged.Packages = userKernel.Packages
	}

	// Add the EnableExtraModules field merge logic
	if userKernel.EnableExtraModules != "" {
		merged.EnableExtraModules = userKernel.EnableExtraModules
	}

	// Note: name and uki fields come from defaults and are preserved

	return merged
}

func mergePackageRepositories(defaultRepos, userRepos []PackageRepository) []PackageRepository {
	if len(userRepos) == 0 {
		return defaultRepos
	}

	// User repositories take precedence - they completely override defaults
	// This gives users full control over repository configuration
	return userRepos
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

	// Load the user template first
	userTemplate, err := LoadTemplate(templatePath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load user template: %w", err)
	}

	log.Infof("Loaded user template: %s (type: %s)", userTemplate.Image.Name, userTemplate.Target.ImageType)

	// Create default config loader
	loader := NewDefaultConfigLoader(userTemplate.Target.OS, userTemplate.Target.Dist, userTemplate.Target.Arch)

	// Load the appropriate default configuration
	defaultTemplate, err := loader.LoadDefaultConfig(userTemplate.Target.ImageType)
	if err != nil {
		log.Debugf("Default template: %+v", defaultTemplate)
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
