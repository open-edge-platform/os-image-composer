// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/utils/config/validate"
	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
	"gopkg.in/yaml.v3"
)

// ImageTemplate represents the YAML image template structure
type ImageTemplate struct {
	Image struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"image"`
	Target struct {
		OS        string `yaml:"os"`
		Dist      string `yaml:"dist"`
		Arch      string `yaml:"arch"`
		ImageType string `yaml:"imageType"`
	} `yaml:"target"`
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

// GlobalConfig holds essential tool-level configuration parameters
type GlobalConfig struct {
	// Core tool settings
	Workers  int    `yaml:"workers" json:"workers"`     // Number of concurrent download workers (1-100, default: 8)
	CacheDir string `yaml:"cache_dir" json:"cache_dir"` // Package cache directory where downloaded RPMs/DEBs are stored (default: ./cache)
	WorkDir  string `yaml:"work_dir" json:"work_dir"`   // Working directory for build operations and image assembly (default: ./workspace)
	TempDir  string `yaml:"temp_dir" json:"temp_dir"`   // Temporary directory for short-lived files like GPG keys and metadata parsing (empty = system default)

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"` // Logging behavior settings
}

// LoggingConfig controls basic logging behavior
type LoggingConfig struct {
	Level string `yaml:"level" json:"level"` // Log verbosity level: debug (most verbose), info (default), warn (warnings only), error (errors only)
}

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

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Workers:  8,
		CacheDir: "./cache",
		WorkDir:  "./workspace",
		TempDir:  "./tmp",

		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// LoadGlobalConfig loads configuration from the specified path
func LoadGlobalConfig(configPath string) (*GlobalConfig, error) {
	// Start with defaults
	config := DefaultGlobalConfig()

	// If no config file specified or doesn't exist, return defaults
	if configPath == "" {
		return config, nil
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil // Return defaults if file doesn't exist
	}

	// Load and merge config file values
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Determine format by extension
	ext := strings.ToLower(filepath.Ext(configPath))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("parsing YAML config: %w", err)
		}

		// Convert to JSON for schema validation
		jsonData, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("converting config to JSON for validation: %w", err)
		}

		// Validate against schema
		if err := validate.ValidateConfigJSON(jsonData); err != nil {
			return nil, fmt.Errorf("schema validation failed: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// SaveGlobalConfig saves the configuration to the specified path
func (gc *GlobalConfig) SaveGlobalConfig(configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
	}

	// Convert to JSON for schema validation before saving
	jsonData, err := json.Marshal(gc)
	if err != nil {
		return fmt.Errorf("converting config to JSON for validation: %w", err)
	}

	if err := validate.ValidateConfigJSON(jsonData); err != nil {
		return fmt.Errorf("config validation failed before save: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(gc)
	if err != nil {
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks the configuration for consistency and applies constraints
// Note: This should NOT set defaults - that's done in DefaultGlobalConfig()
func (gc *GlobalConfig) Validate() error {
	// Validate workers range
	if gc.Workers <= 0 {
		return fmt.Errorf("workers must be greater than 0, got %d", gc.Workers)
	}
	if gc.Workers > 100 {
		return fmt.Errorf("workers cannot exceed 100, got %d", gc.Workers)
	}

	// Validate required fields are not empty
	if gc.CacheDir == "" {
		return fmt.Errorf("cache_dir cannot be empty")
	}
	if gc.WorkDir == "" {
		return fmt.Errorf("work_dir cannot be empty")
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, gc.Logging.Level) {
		return fmt.Errorf("invalid log level %q, must be one of: %s",
			gc.Logging.Level, strings.Join(validLevels, ", "))
	}

	// Ensure temp directory is set (can be empty to use system default)
	if gc.TempDir == "" {
		gc.TempDir = os.TempDir()
	}

	return nil
}

// GetConfigPaths returns the standard configuration file paths to check
func GetConfigPaths() []string {
	homeDir, _ := os.UserHomeDir()

	paths := []string{
		"image-composer.yml",   // Primary config location (root directory)
		".image-composer.yml",  // Hidden file in current directory
		"image-composer.yaml",  // Alternative extension
		".image-composer.yaml", // Hidden file alternative
	}

	if homeDir != "" {
		paths = append(paths,
			filepath.Join(homeDir, ".image-composer", "config.yml"),
			filepath.Join(homeDir, ".image-composer", "config.yaml"),
			filepath.Join(homeDir, ".config", "image-composer", "config.yml"),
			filepath.Join(homeDir, ".config", "image-composer", "config.yaml"),
		)
	}

	// System-wide config paths
	paths = append(paths,
		"/etc/image-composer/config.yml",
		"/etc/image-composer/config.yaml",
	)

	return paths
}

// FindConfigFile searches for a configuration file in standard locations
func FindConfigFile() string {
	for _, path := range GetConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
