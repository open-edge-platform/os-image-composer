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
	configFile       string = "" // Path to config file
	logLevel         string = "" // Empty means use config file value
	logFilePath      string = "" // Optional log file override
	actualConfigFile string = "" // Actual config file path found during init
	loggerCleanup    func()
)

func main() {
	cobra.OnInitialize(initConfig)

	defer func() {
		if loggerCleanup != nil {
			loggerCleanup()
		}
	}()

	// Create and execute root command
	rootCmd := createRootCommand()
	security.AttachRecursive(rootCmd, security.DefaultLimits())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Initialize global configuration
	configFilePath := configFile
	if configFilePath == "" {
		configFilePath = config.FindConfigFile()
	}
	actualConfigFile = configFilePath

	globalConfig, err := config.LoadGlobalConfig(configFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	if logFilePath != "" {
		globalConfig.Logging.File = logFilePath
	}
	globalConfig = config.Global()
	globalConfig.Logging.Level = logLevel
	config.SetGlobal(globalConfig)
	logger.SetLogLevel(logLevel)

	// Set global config singleton
	config.SetGlobal(globalConfig)

	// Setup logger with configured level and optional file output (overridden later if needed)
	_, cleanup, logErr := logger.InitWithConfig(logger.Config{
		Level:    globalConfig.Logging.Level,
		FilePath: globalConfig.Logging.File,
	})
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", logErr)
		os.Exit(1)
	}
	loggerCleanup = cleanup
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
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Handle log level override after flag parsing
			if logLevel != "" {
				globalConfig := config.Global()
				globalConfig.Logging.Level = logLevel
				config.SetGlobal(globalConfig)
				logger.SetLogLevel(logLevel)
			}

			// Log configuration info after log level is finalized
			log := logger.Logger()
			if actualConfigFile != "" {
				log.Infof("Using configuration from: %s", actualConfigFile)
			}
			cacheDir, _ := config.CacheDir()
			workDir, _ := config.WorkDir()
			log.Debugf("Config: workers=%d, cache_dir=%s, work_dir=%s, temp_dir=%s",
				config.Workers(), cacheDir, workDir, config.TempDir())
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "",
		"Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFilePath, "log-file", "",
		"Log file path to tee logs (overrides configuration file)")

	// Add all subcommands
	rootCmd.AddCommand(createBuildCommand())
	rootCmd.AddCommand(createValidateCommand())
	rootCmd.AddCommand(createVersionCommand())
	rootCmd.AddCommand(createConfigCommand())
	rootCmd.AddCommand(createCacheCommand())
	rootCmd.AddCommand(createInstallCompletionCommand())

	return rootCmd
}
