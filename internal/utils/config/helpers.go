package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigHelpers provides convenient access to global configuration
type ConfigHelpers struct {
	config *GlobalConfig
}

// NewConfigHelpers creates a new config helpers instance
func NewConfigHelpers(config *GlobalConfig) *ConfigHelpers {
	return &ConfigHelpers{config: config}
}

// Workers returns the number of concurrent workers
func (c *ConfigHelpers) Workers() int {
	return c.config.Workers
}

// CacheDir returns the absolute path to the cache directory
func (c *ConfigHelpers) CacheDir() (string, error) {
	return filepath.Abs(c.config.CacheDir)
}

// WorkDir returns the absolute path to the work directory
func (c *ConfigHelpers) WorkDir() (string, error) {
	return filepath.Abs(c.config.WorkDir)
}

// TempDir returns the temporary directory path
func (c *ConfigHelpers) TempDir() string {
	if c.config.TempDir == "" {
		return os.TempDir()
	}
	return c.config.TempDir
}

// LogLevel returns the configured log level
func (c *ConfigHelpers) LogLevel() string {
	return c.config.Logging.Level
}

// IsDebugMode returns true if debug logging is enabled
func (c *ConfigHelpers) IsDebugMode() bool {
	return c.config.Logging.Level == "debug"
}

// GetConfig returns the underlying global config (for advanced usage)
func (c *ConfigHelpers) GetConfig() *GlobalConfig {
	return c.config
}

// CreateCacheDir ensures the cache directory exists
func (c *ConfigHelpers) CreateCacheDir() error {
	cacheDir, err := c.CacheDir()
	if err != nil {
		return fmt.Errorf("resolving cache directory: %w", err)
	}
	return createDirIfNotExists(cacheDir)
}

// CreateWorkDir ensures the work directory exists
func (c *ConfigHelpers) CreateWorkDir() error {
	workDir, err := c.WorkDir()
	if err != nil {
		return fmt.Errorf("resolving work directory: %w", err)
	}
	return createDirIfNotExists(workDir)
}

// CreateTempDir ensures a temp subdirectory exists
func (c *ConfigHelpers) CreateTempDir(subdir string) (string, error) {
	tempDir := filepath.Join(c.TempDir(), subdir)
	err := createDirIfNotExists(tempDir)
	return tempDir, err
}

// Helper function to create directories
func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
