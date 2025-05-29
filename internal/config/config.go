// internal/config/config.go
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/validate"
	"gopkg.in/yaml.v3"
)

// BuildSpec represents your JSON schema.
type BuildSpec struct {
	Distro    string       `json:"distro"`
	Version   string       `json:"version"`
	Arch      string       `json:"arch"`
	Packages  []string     `json:"packages"`
	Immutable bool         `json:"immutable"`
	Output    string       `json:"output"`
	Kernel    KernelConfig `json:"kernel"`
}

// KernelConfig holds the nested "kernel" object.
type KernelConfig struct {
	Version string `json:"version"`
	Cmdline string `json:"cmdline"`
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

// Load loads a BuildSpec from the specified path
func Load(path string) (*BuildSpec, error) {
	logger := utils.Logger()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// validate raw JSON against schema
	if err := validate.ValidateComposerJSON(data); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	// unmarshal into typed struct
	var bc BuildSpec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bc); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	logger.Infof("loaded config: \n%s", string(data))
	return &bc, nil
}

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Workers:  8,
		CacheDir: "./cache",     // Changed from ./downloads for consistency
		WorkDir:  "./workspace", // Changed from ./builds to avoid conflict with build artifacts
		TempDir:  "",            // Will use system temp

		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// LoadGlobalConfig loads configuration from the specified path
func LoadGlobalConfig(configPath string) (*GlobalConfig, error) {
	config := DefaultGlobalConfig()

	if configPath == "" {
		return config, nil
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil // Return defaults if file doesn't exist
	}

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
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	// Validate and set defaults for empty values
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

	data, err := yaml.Marshal(gc)
	if err != nil {
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks the configuration for consistency and sets defaults
func (gc *GlobalConfig) Validate() error {
	// Validate workers
	if gc.Workers <= 0 {
		gc.Workers = 8
	}
	if gc.Workers > 100 {
		return fmt.Errorf("workers cannot exceed 100, got %d", gc.Workers)
	}

	// Validate cache directory
	if gc.CacheDir == "" {
		gc.CacheDir = "./cache"
	}

	// Validate work directory
	if gc.WorkDir == "" {
		gc.WorkDir = "./workspace"
	}

	// Validate temp directory (empty means use system temp)
	if gc.TempDir == "" {
		gc.TempDir = os.TempDir()
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, gc.Logging.Level) {
		return fmt.Errorf("invalid log level %q, must be one of: %s",
			gc.Logging.Level, strings.Join(validLevels, ", "))
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
