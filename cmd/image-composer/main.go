package main

import (
	"fmt"
	"os"

	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
	"github.com/spf13/cobra"
)

// Version information
var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	CommitSHA = "unknown"
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

	var err error
	config.GlConfig, err = config.LoadGlobalConfig(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger with configured level
	_, cleanup := logger.InitWithLevel(config.GlConfig.Logging.Level)
	defer cleanup()

	// Create and execute root command
	rootCmd := createRootCommand()

	// Handle log level override after flag parsing
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if logLevel != "" {
			config.GlConfig.Logging.Level = logLevel
			logger.SetLogLevel(logLevel)
		}
	}

	// Log configuration info
	log := logger.Logger()
	if configFilePath != "" {
		log.Infof("Using configuration from: %s", configFilePath)
	}
	if config.GlConfig.Logging.Level == "debug" {
		log.Debugf("Config: workers=%d, cache_dir=%s, work_dir=%s, temp_dir=%s",
			config.GlConfig.Workers, config.GlConfig.CacheDir, config.GlConfig.WorkDir, config.GlConfig.TempDir)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// createRootCommand creates and configures the root cobra command with all subcommands
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "image-composer",
		Short: "Image Composer Tool (ICT) for building Linux distributions",
		Long: `Image Composer Tool (ICT) is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).`,
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
