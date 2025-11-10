// internal/config/global.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/open-edge-platform/os-image-composer/internal/config/validate"
	"github.com/open-edge-platform/os-image-composer/internal/utils/security"
	"github.com/open-edge-platform/os-image-composer/internal/utils/slice"
	"gopkg.in/yaml.v3"
)

// AIConfig contains AI/LLM configuration
type AIConfig struct {
	Enabled      bool         `yaml:"enabled" json:"enabled"`             // Enable/disable AI features
	Provider     string       `yaml:"provider" json:"provider"`           // LLM provider: "ollama", "openai", etc.
	TemplatesDir string       `yaml:"templates_dir" json:"templates_dir"` // Directory containing template examples for RAG
	Ollama       OllamaConfig `yaml:"ollama" json:"ollama"`               // Ollama-specific configuration
	OpenAI       OpenAIConfig `yaml:"openai" json:"openai"`               // OpenAI specific configuration
}

// OllamaConfig contains Ollama LLM settings
type OllamaConfig struct {
	BaseURL        string  `yaml:"base_url" json:"base_url"`               // Ollama server URL
	Model          string  `yaml:"model" json:"model"`                     // Model name (e.g., llama3.1:8b)
	Temperature    float64 `yaml:"temperature" json:"temperature"`         // Temperature for text generation (0.0-1.0)
	MaxTokens      int     `yaml:"max_tokens" json:"max_tokens"`           // Maximum tokens to generate
	Timeout        int     `yaml:"timeout" json:"timeout"`                 // Request timeout in seconds
	EmbeddingModel string  `yaml:"embedding_model" json:"embedding_model"` // Embedding model for template retrieval
}

// OpenAIConfig contains OpenAI-specific settings
type OpenAIConfig struct {
	APIKey         string  `yaml:"api_key" json:"api_key"` // OpenAI API key
	Model          string  `yaml:"model" json:"model"`     // e.g., "gpt-4", "gpt-4-turbo"
	Temperature    float64 `yaml:"temperature" json:"temperature"`
	MaxTokens      int     `yaml:"max_tokens" json:"max_tokens"`
	Timeout        int     `yaml:"timeout" json:"timeout"`
	EmbeddingModel string  `yaml:"embedding_model" json:"embedding_model"` // Embedding model for template retrieval
}

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

	// AI Configuration
	AI AIConfig `yaml:"ai" json:"ai"`
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

		AI: AIConfig{
			Enabled:      false,
			Provider:     "ollama",
			TemplatesDir: "./image-templates",
			Ollama: OllamaConfig{
				BaseURL:        "http://localhost:11434",
				Model:          "llama3.1:8b",
				Temperature:    0.7,
				MaxTokens:      2000,
				Timeout:        120,
				EmbeddingModel: "nomic-embed-text",
			},
			OpenAI: OpenAIConfig{
				APIKey:         "",
				Model:          "gpt-4-turbo",
				Temperature:    0.7,
				MaxTokens:      2000,
				Timeout:        120,
				EmbeddingModel: "text-embedding-3-small",
			},
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
		log.Errorf("Error reading config file %s: %v", configPath, err)
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Determine format by extension
	ext := strings.ToLower(filepath.Ext(configPath))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			log.Errorf("Error parsing YAML config: %v", err)
			return nil, fmt.Errorf("parsing YAML config: %w", err)
		}

		// Convert to JSON for schema validation
		jsonData, err := json.Marshal(config)
		if err != nil {
			log.Errorf("Error converting config to JSON for validation: %v", err)
			return nil, fmt.Errorf("converting config to JSON for validation: %w", err)
		}

		// Validate against schema
		if err := validate.ValidateConfigJSON(jsonData); err != nil {
			log.Errorf("Schema validation failed: %v", err)
			return nil, fmt.Errorf("schema validation failed: %w", err)
		}

	default:
		log.Errorf("Unsupported config file format: %s", ext)
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	// Validate the final configuration
	if err := config.Validate(); err != nil {
		log.Errorf("Config validation failed: %v", err)
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// SaveGlobalConfig saves the configuration to the specified path
func (gc *GlobalConfig) SaveGlobalConfig(configPath string) error {
	gc.applyAIDefaults()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Errorf("Failed to create config directory: %v", err)
			return fmt.Errorf("creating config directory: %w", err)
		}
	}

	// Convert to JSON for schema validation before saving
	jsonData, err := json.Marshal(gc)
	if err != nil {
		log.Errorf("Error converting config to JSON for validation: %v", err)
		return fmt.Errorf("converting config to JSON for validation: %w", err)
	}

	if err := validate.ValidateConfigJSON(jsonData); err != nil {
		log.Errorf("Config validation failed before save: %v", err)
		return fmt.Errorf("config validation failed before save: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(gc)
	if err != nil {
		log.Errorf("Error marshaling config to YAML: %v", err)
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}

	// Use safe write to prevent symlink attacks
	if err := security.SafeWriteFile(configPath, data, 0600, security.RejectSymlinks); err != nil {
		log.Errorf("Error writing config file: %v", err)
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

func (gc *GlobalConfig) applyAIDefaults() {
	defaults := DefaultGlobalConfig().AI

	if gc.AI.Provider == "" {
		gc.AI.Provider = defaults.Provider
	}
	if gc.AI.TemplatesDir == "" {
		gc.AI.TemplatesDir = defaults.TemplatesDir
	}

	// Normalize Ollama configuration
	if gc.AI.Ollama.BaseURL == "" {
		gc.AI.Ollama.BaseURL = defaults.Ollama.BaseURL
	}
	if gc.AI.Ollama.Model == "" {
		gc.AI.Ollama.Model = defaults.Ollama.Model
	}
	if gc.AI.Ollama.Temperature == 0 && defaults.Ollama.Temperature != 0 {
		gc.AI.Ollama.Temperature = defaults.Ollama.Temperature
	}
	if gc.AI.Ollama.MaxTokens < 100 {
		gc.AI.Ollama.MaxTokens = defaults.Ollama.MaxTokens
	}
	if gc.AI.Ollama.Timeout < 10 {
		gc.AI.Ollama.Timeout = defaults.Ollama.Timeout
	}
	if gc.AI.Ollama.EmbeddingModel == "" {
		gc.AI.Ollama.EmbeddingModel = defaults.Ollama.EmbeddingModel
	}

	// Normalize OpenAI configuration
	if gc.AI.OpenAI.Model == "" {
		gc.AI.OpenAI.Model = defaults.OpenAI.Model
	}
	if gc.AI.OpenAI.Temperature == 0 && defaults.OpenAI.Temperature != 0 {
		gc.AI.OpenAI.Temperature = defaults.OpenAI.Temperature
	}
	if gc.AI.OpenAI.MaxTokens < 100 {
		gc.AI.OpenAI.MaxTokens = defaults.OpenAI.MaxTokens
	}
	if gc.AI.OpenAI.Timeout < 10 {
		gc.AI.OpenAI.Timeout = defaults.OpenAI.Timeout
	}
	if gc.AI.OpenAI.EmbeddingModel == "" {
		gc.AI.OpenAI.EmbeddingModel = defaults.OpenAI.EmbeddingModel
	}
}

// Validate checks the configuration for consistency and applies constraints
// Note: This should NOT set defaults - that's done in DefaultGlobalConfig()
func (gc *GlobalConfig) Validate() error {
	// Validate workers range
	if gc.Workers <= 0 {
		log.Errorf("Workers must be greater than 0, got %d", gc.Workers)
		return fmt.Errorf("workers must be greater than 0, got %d", gc.Workers)
	}
	if gc.Workers > 100 {
		log.Errorf("Workers cannot exceed 100, got %d", gc.Workers)
		return fmt.Errorf("workers cannot exceed 100, got %d", gc.Workers)
	}

	// Validate required fields are not empty
	if gc.CacheDir == "" {
		log.Errorf("CacheDir cannot be empty")
		return fmt.Errorf("CacheDir cannot be empty")
	}
	if gc.WorkDir == "" {
		log.Errorf("WorkDir cannot be empty")
		return fmt.Errorf("WorkDir cannot be empty")
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error"}
	if !slice.Contains(validLevels, gc.Logging.Level) {
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
		"os-image-composer.yml",   // Primary config location (root directory)
		".os-image-composer.yml",  // Hidden file in current directory
		"os-image-composer.yaml",  // Alternative extension
		".os-image-composer.yaml", // Hidden file alternative
	}

	if homeDir != "" {
		paths = append(paths,
			filepath.Join(homeDir, ".os-image-composer", "config.yml"),
			filepath.Join(homeDir, ".os-image-composer", "config.yaml"),
			filepath.Join(homeDir, ".config", "os-image-composer", "config.yml"),
			filepath.Join(homeDir, ".config", "os-image-composer", "config.yaml"),
		)
	}

	// System-wide config paths
	paths = append(paths,
		"/etc/os-image-composer/config.yml",
		"/etc/os-image-composer/config.yaml",
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
	configDir, err := filepath.Abs(Global().ConfigDir)
	if err != nil {
		log.Errorf("Failed to resolve config directory: %v", err)
		return "", fmt.Errorf("failed to resolving config directory: %w", err)
	}
	return configDir, nil
}

func CacheDir() (string, error) {
	cacheDir, err := filepath.Abs(Global().CacheDir)
	if err != nil {
		log.Errorf("Failed to resolve cache directory: %v", err)
		return "", fmt.Errorf("failed to resolve cache directory: %w", err)
	}
	return cacheDir, nil
}

func WorkDir() (string, error) {
	workDir, err := filepath.Abs(Global().WorkDir)
	if err != nil {
		log.Errorf("Failed to resolve work directory: %v", err)
		return "", fmt.Errorf("failed to resolve work directory: %w", err)
	}
	return workDir, nil
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

func GetGeneralConfigDir() (string, error) {
	configPath, err := ConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config path: %w", err)
	}
	generalConfigDir := filepath.Join(configPath, "general")
	if _, err := os.Stat(generalConfigDir); os.IsNotExist(err) {
		log.Errorf("General config directory does not exist: %s", generalConfigDir)
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
		log.Errorf("Target OS config directory does not exist: %s", targetOsConfigPath)
		return "", fmt.Errorf("target OS config directory does not exist: %s", targetOsConfigPath)
	}
	return targetOsConfigPath, nil
}

func AIEnabled() bool {
	return Global().AI.Enabled
}

func GetAIConfig() AIConfig {
	return Global().AI
}
