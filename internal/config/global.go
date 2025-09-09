// internal/config/global.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-edge-platform/image-composer/internal/config/validate"
	"github.com/open-edge-platform/image-composer/internal/utils/security"
	"gopkg.in/yaml.v3"
)

// GlobalConfig holds essential tool-level configuration parameters
type GlobalConfig struct {
	// Core tool settings
	Workers   int    `yaml:"workers" json:"workers"`       // Number of concurrent download workers (1-100, default: 8)
	ConfigDir string `yaml:"config_dir" json:"config_dir"` // Directory for configuration files (default: ./config)
	CacheDir  string `yaml:"cache_dir" json:"cache_dir"`   // Package cache directory where downloaded RPMs/DEBs are stored (default: ./cache)
	WorkDir   string `yaml:"work_dir" json:"work_dir"`     // Working directory for build operations and image assembly (default: ./workspace)
	TempDir   string `yaml:"temp_dir" json:"temp_dir"`     // Temporary directory for short-lived files like GPG keys and metadata parsing (empty = system default)

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"` // Logging behavior settings
}

// LoggingConfig controls basic logging behavior
type LoggingConfig struct {
	Level string `yaml:"level" json:"level"` // Log verbosity level: debug (most verbose), info (default), warn (warnings only), error (errors only)
}

// Global singleton variables
var (
	globalInstance *GlobalConfig
	globalMutex    sync.RWMutex
	once           sync.Once
)

// SetGlobal sets the global config instance (call once at startup in main.go)
func SetGlobal(config *GlobalConfig) {
	globalMutex.Lock()
	defer globalMutex.Unlock()
	globalInstance = config
}

// Global returns the global config instance
func Global() *GlobalConfig {
	once.Do(func() {
		globalMutex.Lock()
		defer globalMutex.Unlock()
		if globalInstance == nil {
			globalInstance = DefaultGlobalConfig()
		}
	})

	globalMutex.RLock()
	defer globalMutex.RUnlock()
	return globalInstance
}

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		Workers:   8,
		ConfigDir: "./config",
		CacheDir:  "./cache",
		WorkDir:   "./workspace",
		TempDir:   "./tmp",

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

	// Load and merge config file values with symlink protection
	data, err := security.SafeReadFile(configPath, security.RejectSymlinks)
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
		if err := os.MkdirAll(dir, 0700); err != nil {
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

	// Use safe write to prevent symlink attacks
	if err := security.SafeWriteFile(configPath, data, 0600, security.RejectSymlinks); err != nil {
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

// Convenience functions that can be used anywhere in the codebase
func Workers() int {
	return Global().Workers
}

func VerificationWorkers() int {
	workers := Global().Workers
	if workers > 4 {
		workers = 4 // Keep the cap as requested
	}
	return workers
}

func ConfigDir() (string, error) {
	return filepath.Abs(Global().ConfigDir)
}

func CacheDir() (string, error) {
	return filepath.Abs(Global().CacheDir)
}

func WorkDir() (string, error) {
	return filepath.Abs(Global().WorkDir)
}

func TempDir() string {
	tempDir := Global().TempDir
	if tempDir == "" {
		return os.TempDir()
	}
	return tempDir
}

func LogLevel() string {
	return Global().Logging.Level
}

func IsDebugMode() bool {
	return Global().Logging.Level == "debug"
}

// Directory creation helpers
func EnsureCacheDir() error {
	cacheDir, err := CacheDir()
	if err != nil {
		return fmt.Errorf("resolving cache directory: %w", err)
	}
	return ensureDirExists(cacheDir)
}

func EnsureWorkDir() error {
	workDir, err := WorkDir()
	if err != nil {
		return fmt.Errorf("resolving work directory: %w", err)
	}
	return ensureDirExists(workDir)
}

func EnsureTempDir(subdir string) (string, error) {
	tempDir := filepath.Join(TempDir(), subdir)
	err := ensureDirExists(tempDir)
	return tempDir, err
}

func ensureDirExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0700)
	}
	return nil
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

func GetGeneralConfigDir() (string, error) {
	configPath, err := ConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config path: %w", err)
	}
	generalConfigDir := filepath.Join(configPath, "general")
	if _, err := os.Stat(generalConfigDir); os.IsNotExist(err) {
		return "", fmt.Errorf("general config directory does not exist: %s", generalConfigDir)
	}
	return generalConfigDir, nil
}

func GetTargetOsConfigDir(targetOs, targetDist string) (string, error) {
	configPath, err := ConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config path: %w", err)
	}
	targetOsConfigPath := filepath.Join(configPath, "osv", targetOs, targetDist)
	if _, err := os.Stat(targetOsConfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("target OS config directory does not exist: %s", targetOsConfigPath)
	}
	return targetOsConfigPath, nil
}
