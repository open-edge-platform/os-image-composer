package main

import (
	"fmt"
	"os"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/security"
	"github.com/spf13/cobra"
)

// Command-line flags that can override config file settings
var (
	configFile string = "" // Path to config file
	logLevel   string = "" // Empty means use config file value
)

func main() {
	// Initialize global configuration first
	configFilePath := configFile
	if configFilePath == "" {
		configFilePath = config.FindConfigFile()
	}

	globalConfig, err := config.LoadGlobalConfig(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Set global config singleton
	config.SetGlobal(globalConfig)

	// Setup logger with configured level
	_, cleanup := logger.InitWithLevel(globalConfig.Logging.Level)
	defer cleanup()

	// Create and execute root command
	rootCmd := createRootCommand()
	security.AttachRecursive(rootCmd, security.DefaultLimits())

	// Handle log level override after flag parsing
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if logLevel != "" {
			// Update both the local config and the global singleton
			globalConfig.Logging.Level = logLevel
			config.SetGlobal(globalConfig) // Update singleton with new log level
			logger.SetLogLevel(logLevel)
		}
	}

	// Log configuration info using global config functions
	log := logger.Logger()
	if configFilePath != "" {
		log.Infof("Using configuration from: %s", configFilePath)
	}
	cacheDir, _ := config.CacheDir()
	workDir, _ := config.WorkDir()
	log.Debugf("Config: workers=%d, cache_dir=%s, work_dir=%s, temp_dir=%s",
		config.Workers(), cacheDir, workDir, config.TempDir())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// createRootCommand creates and configures the root cobra command with all subcommands
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "os-image-composer",
		Short: "OS Image Composer for building Linux distributions",
		Long: `OS Image Composer is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).

The tool supports building custom images for:
- EMT (Edge Microvisor Toolkit)
- Azure Linux
- Wind River eLxr

Use 'os-image-composer --help' to see available commands.
Use 'os-image-composer <command> --help' for more information about a command.`,
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "",
		"Log level (debug, info, warn, error)")

	// Add all subcommands
	rootCmd.AddCommand(createBuildCommand())
	rootCmd.AddCommand(createValidateCommand())
	rootCmd.AddCommand(createVersionCommand())
	rootCmd.AddCommand(createConfigCommand())
	rootCmd.AddCommand(createInstallCompletionCommand())

	return rootCmd
}
