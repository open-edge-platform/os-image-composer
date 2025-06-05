package main

import (
	"fmt"
	"os"

	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/spf13/cobra"
)

// createConfigCommand creates the config command with subcommands
func createConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage global configuration",
	}

	// Add subcommands
	configCmd.AddCommand(createConfigShowCommand())
	configCmd.AddCommand(createConfigInitCommand())

	return configCmd
}

// createConfigShowCommand creates the config show subcommand
func createConfigShowCommand() *cobra.Command {
	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run:   executeConfigShow,
	}

	return configShowCmd
}

// createConfigInitCommand creates the config init subcommand
func createConfigInitCommand() *cobra.Command {
	configInitCmd := &cobra.Command{
		Use:   "init [config-file]",
		Short: "Initialize a new configuration file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  executeConfigInit,
	}

	return configInitCmd
}

// executeConfigShow shows the current configuration
func executeConfigShow(cmd *cobra.Command, args []string) {
	if configFile != "" {
		fmt.Printf("Configuration file: %s\n", configFile)
	} else {
		fmt.Printf("Configuration file: <using defaults>\n")
	}
	fmt.Printf("Workers: %d\n", config.GlConfig.Workers)
	fmt.Printf("Cache directory: %s\n", config.GlConfig.CacheDir)
	fmt.Printf("Work directory: %s\n", config.GlConfig.WorkDir)
	fmt.Printf("Temp directory: %s\n", config.GlConfig.TempDir)
	fmt.Printf("Log level: %s\n", config.GlConfig.Logging.Level)
}

// executeConfigInit creates a new configuration file
func executeConfigInit(cmd *cobra.Command, args []string) error {
	configPath := "image-composer.yml" // Default to image-composer.yml
	if len(args) > 0 {
		configPath = args[0]
	}

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("configuration file already exists: %s", configPath)
	}

	// Create default config and save it
	defaultConfig := config.DefaultGlobalConfig()
	if err := defaultConfig.SaveGlobalConfig(configPath); err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}

	fmt.Printf("Configuration file created: %s\n", configPath)
	fmt.Printf("Edit the file to customize settings for your environment.\n")
	return nil
}
