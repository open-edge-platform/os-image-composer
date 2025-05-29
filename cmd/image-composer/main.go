package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/provider"
	_ "github.com/open-edge-platform/image-composer/internal/provider/azurelinux3" // register provider
	_ "github.com/open-edge-platform/image-composer/internal/provider/elxr12"      // register provider
	_ "github.com/open-edge-platform/image-composer/internal/provider/emt3_0"      // register provider
	"github.com/open-edge-platform/image-composer/internal/rpmutils"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/validate"
	"github.com/spf13/cobra"
)

// Version information
var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	CommitSHA = "unknown"
)

// Global configuration
var globalConfig *config.GlobalConfig

// Command-line flags that can override config file settings
var (
	configFile string = "" // Path to config file
	workers    int    = -1 // -1 means use config file value
	cacheDir   string = "" // Empty means use config file value
	workDir    string = "" // Empty means use config file value
	verbose    bool   = false
	dotFile    string = ""
	logLevel   string = "" // Empty means use config file value
)

// initializeGlobalConfig loads and initializes the global configuration
func initializeGlobalConfig() error {
	var err error

	// If no config file specified, try to find one
	if configFile == "" {
		configFile = config.FindConfigFile()
	}

	// Load global configuration
	globalConfig, err = config.LoadGlobalConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	// Override config with command-line flags if provided
	// Note: This logic is now moved to individual command handlers
	// to ensure flags are properly parsed before override

	// Update the module-level variables for backward compatibility
	workers = globalConfig.Workers
	cacheDir = globalConfig.CacheDir

	return nil
}

// jsonFileCompletion helps with suggesting JSON files for spec file argument
func jsonFileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"*.json"}, cobra.ShellCompDirectiveFilterFileExt
}

// executeBuild handles the build command execution logic
func executeBuild(cmd *cobra.Command, args []string) error {
	// Parse command-line flags and override global config
	if cmd.Flags().Changed("workers") {
		globalConfig.Workers = workers
	}
	if cmd.Flags().Changed("cache-dir") {
		globalConfig.CacheDir = cacheDir
	}
	if cmd.Flags().Changed("work-dir") {
		globalConfig.WorkDir = workDir
	}

	logger := utils.Logger()

	// Check if spec file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no spec file provided, usage: image-composer build [flags] SPEC_FILE")
	}
	specFile := args[0]

	// Load and validate the configuration
	bc, err := config.Load(specFile)
	if err != nil {
		return fmt.Errorf("loading spec file: %v", err)
	}

	providerName := bc.Distro + bc.Version

	// Get provider by name
	p, ok := provider.Get(providerName)
	if !ok {
		return fmt.Errorf("provider not found: %s", providerName)
	}

	// Initialize provider
	if err := p.Init(bc); err != nil {
		return fmt.Errorf("provider init: %v", err)
	}

	// Fetch the entire package list
	all, err := p.Packages()
	if err != nil {
		return fmt.Errorf("getting packages: %v", err)
	}

	// Match the packages in the build spec against all the packages
	req, err := p.MatchRequested(bc.Packages, all)
	if err != nil {
		return fmt.Errorf("matching packages: %v", err)
	}
	logger.Infof("matched a total of %d packages", len(req))
	if verbose {
		for _, pkg := range req {
			logger.Infof("-> %s", pkg.Name)
		}
	}

	// Resolve the dependencies of the requested packages
	needed, err := p.Resolve(req, all)
	if err != nil {
		return fmt.Errorf("resolving packages: %v", err)
	}
	logger.Infof("resolved %d packages", len(needed))

	// If a dot file is specified, generate the dependency graph
	if dotFile != "" {
		if err := rpmutils.GenerateDot(needed, dotFile); err != nil {
			logger.Errorf("generating dot file: %v", err)
		}
	}

	// Extract URLs
	urls := make([]string, len(needed))
	for i, pkg := range needed {
		urls[i] = pkg.URL
	}

	// Ensure cache directory exists
	absCacheDir, err := filepath.Abs(globalConfig.CacheDir)
	if err != nil {
		return fmt.Errorf("resolving cache directory: %v", err)
	}
	if err := os.MkdirAll(absCacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory %s: %v", absCacheDir, err)
	}

	// Ensure work directory exists
	absWorkDir, err := filepath.Abs(globalConfig.WorkDir)
	if err != nil {
		return fmt.Errorf("resolving work directory: %v", err)
	}
	if err := os.MkdirAll(absWorkDir, 0755); err != nil {
		return fmt.Errorf("creating work directory %s: %v", absWorkDir, err)
	}

	// Download packages using configured workers and cache directory
	logger.Infof("downloading %d packages to %s using %d workers", len(urls), absCacheDir, globalConfig.Workers)
	if err := pkgfetcher.FetchPackages(urls, absCacheDir, globalConfig.Workers); err != nil {
		return fmt.Errorf("fetch failed: %v", err)
	}
	logger.Info("all downloads complete")

	// Verify downloaded packages
	if err := p.Validate(globalConfig.CacheDir); err != nil {
		return fmt.Errorf("verification failed: %v", err)
	}

	logger.Info("build completed successfully")
	return nil
}

// executeValidate handles the validate command execution logic
func executeValidate(cmd *cobra.Command, args []string) error {
	logger := utils.Logger()

	// Check if spec file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no spec file provided, usage: image-composer validate SPEC_FILE")
	}
	specFile := args[0]

	logger.Infof("Validating spec file: %s", specFile)

	// Read the file
	data, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("reading spec file: %v", err)
	}

	// Validate the JSON against schema
	if err := validate.ValidateComposerJSON(data); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	logger.Info("Spec file is valid")
	return nil
}

// executeVersion handles the version command logic
func executeVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("Image Composer Tool v%s\n", Version)
	fmt.Printf("Build Date: %s\n", BuildDate)
	fmt.Printf("Commit: %s\n", CommitSHA)
}

// executeConfigShow shows the current configuration
func executeConfigShow(cmd *cobra.Command, args []string) {
	if configFile != "" {
		fmt.Printf("Configuration file: %s\n", configFile)
	} else {
		fmt.Printf("Configuration file: <using defaults>\n")
	}
	fmt.Printf("Workers: %d\n", globalConfig.Workers)
	fmt.Printf("Cache directory: %s\n", globalConfig.CacheDir)
	fmt.Printf("Work directory: %s\n", globalConfig.WorkDir)
	fmt.Printf("Temp directory: %s\n", globalConfig.TempDir)
	fmt.Printf("Log level: %s\n", globalConfig.Logging.Level)
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

// executeInstallCompletion handles installation of shell completion scripts
func executeInstallCompletion(cmd *cobra.Command, args []string) error {
	shellType := ""
	userForce := false

	// Process flags
	if cmd.Flags().Changed("shell") {
		var err error
		shellType, err = cmd.Flags().GetString("shell")
		if err != nil {
			return err
		}
	}

	if cmd.Flags().Changed("force") {
		var err error
		userForce, err = cmd.Flags().GetBool("force")
		if err != nil {
			return err
		}
	}

	// If no shell specified, detect current shell
	if shellType == "" {
		shellEnv := os.Getenv("SHELL")
		if shellEnv != "" {
			switch {
			case strings.Contains(shellEnv, "bash"):
				shellType = "bash"
			case strings.Contains(shellEnv, "zsh"):
				shellType = "zsh"
			case strings.Contains(shellEnv, "fish"):
				shellType = "fish"
			default:
				return fmt.Errorf("unsupported shell: %s. Please specify shell with --shell flag", shellEnv)
			}
		} else {
			// On Windows, we may not have $SHELL
			if os.Getenv("PSModulePath") != "" {
				shellType = "powershell"
			} else {
				return fmt.Errorf("could not detect shell. Please specify with --shell flag")
			}
		}
	}

	// Generate completion script
	var buf bytes.Buffer
	switch shellType {
	case "bash":
		if err := cmd.Root().GenBashCompletion(&buf); err != nil {
			return fmt.Errorf("error generating Bash completion: %w", err)
		}
	case "zsh":
		if err := cmd.Root().GenZshCompletion(&buf); err != nil {
			return fmt.Errorf("error generating Zsh completion: %w", err)
		}
	case "fish":
		if err := cmd.Root().GenFishCompletion(&buf, true); err != nil {
			return fmt.Errorf("error generating Fish completion: %w", err)
		}
	case "powershell":
		if err := cmd.Root().GenPowerShellCompletion(&buf); err != nil {
			return fmt.Errorf("error generating PowerShell completion: %w", err)
		}
	default:
		return fmt.Errorf("unsupported shell type: %s", shellType)
	}

	// Determine where to save the completion script
	var targetPath string
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %v", err)
	}

	switch shellType {
	case "bash":
		// Try to detect if bash-completion is installed
		completionDir := "/etc/bash_completion.d"
		if _, err := os.Stat(completionDir); os.IsNotExist(err) {
			// Fallback to user's directory
			completionDir = filepath.Join(homeDir, ".bash_completion.d")
			if _, err := os.Stat(completionDir); os.IsNotExist(err) {
				if err := os.MkdirAll(completionDir, 0755); err != nil {
					return fmt.Errorf("could not create directory %s: %v", completionDir, err)
				}
			}
		}
		targetPath = filepath.Join(completionDir, "image-composer.bash")
	case "zsh":
		completionDir := filepath.Join(homeDir, ".zsh/completion")
		if _, err := os.Stat(completionDir); os.IsNotExist(err) {
			if err := os.MkdirAll(completionDir, 0755); err != nil {
				return fmt.Errorf("could not create directory %s: %v", completionDir, err)
			}
		}
		targetPath = filepath.Join(completionDir, "_image-composer")
	case "fish":
		completionDir := filepath.Join(homeDir, ".config/fish/completions")
		if _, err := os.Stat(completionDir); os.IsNotExist(err) {
			if err := os.MkdirAll(completionDir, 0755); err != nil {
				return fmt.Errorf("could not create directory %s: %v", completionDir, err)
			}
		}
		targetPath = filepath.Join(completionDir, "image-composer.fish")
	case "powershell":
		profilePath := filepath.Join(homeDir, "Documents/WindowsPowerShell")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			if err := os.MkdirAll(profilePath, 0755); err != nil {
				return fmt.Errorf("could not create directory %s: %v", profilePath, err)
			}
		}
		targetPath = filepath.Join(profilePath, "image-composer-completion.ps1")
	}

	// Check if file exists
	if _, err := os.Stat(targetPath); err == nil && !userForce {
		return fmt.Errorf("completion file already exists at %s. Use --force to overwrite", targetPath)
	}

	// Write completion script to file
	if err := os.WriteFile(targetPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("could not write completion file: %v", err)
	}

	fmt.Printf("Shell completion installed for %s at %s\n", shellType, targetPath)
	fmt.Printf("Restart your shell or source your profile to enable completion.\n")

	return nil
}

func main() {
	// Initialize global configuration first
	if err := initializeGlobalConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Handle global log level override
	if logLevel != "" {
		globalConfig.Logging.Level = logLevel
	}

	// Setup logger with configured level
	_, cleanup := utils.InitWithLevel(globalConfig.Logging.Level)
	defer cleanup()

	// Log configuration info
	logger := utils.Logger()
	if configFile != "" {
		logger.Infof("Using configuration from: %s", configFile)
	}
	if globalConfig.Logging.Level == "debug" {
		logger.Debugf("Config: workers=%d, cache_dir=%s, work_dir=%s, temp_dir=%s",
			globalConfig.Workers, globalConfig.CacheDir, globalConfig.WorkDir, globalConfig.TempDir)
	}

	// Root command
	rootCmd := &cobra.Command{
		Use:   "image-composer",
		Short: "Image Composer Tool (ICT) for building Linux distributions",
		Long: `Image Composer Tool (ICT) is a toolchain that enables building immutable
Linux distributions using a simple toolchain from pre-built packages emanating
from different Operating System Vendors (OSVs).`,
	}

	// Build command
	buildCmd := &cobra.Command{
		Use:   "build [flags] SPEC_FILE",
		Short: "Build a Linux distribution image",
		Long: `Build a Linux distribution image based on the specified spec file.
The spec file should be in JSON format according to the schema.`,
		Args:              cobra.ExactArgs(1),
		RunE:              executeBuild,
		ValidArgsFunction: jsonFileCompletion,
	}

	// Validate command
	validateCmd := &cobra.Command{
		Use:               "validate SPEC_FILE",
		Short:             "Validate a spec file against the schema",
		Long:              `Validate that the given JSON spec file conforms to the schema.`,
		Args:              cobra.ExactArgs(1),
		RunE:              executeValidate,
		ValidArgsFunction: jsonFileCompletion,
	}

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Run:   executeVersion,
	}

	// Config command
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage global configuration",
	}

	// Config show command
	configShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Run:   executeConfigShow,
	}

	// Config init command
	configInitCmd := &cobra.Command{
		Use:   "init [config-file]",
		Short: "Initialize a new configuration file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  executeConfigInit,
	}

	// Install completion command
	installCompletionCmd := &cobra.Command{
		Use:   "install-completion",
		Short: "Install shell completion script",
		Long: `Install shell completion script for Bash, Zsh, Fish, or PowerShell.
Automatically detects your shell and installs the appropriate completion script.`,
		RunE: executeInstallCompletion,
	}

	// Add flags to commands
	buildCmd.Flags().IntVarP(&workers, "workers", "w", -1,
		"Number of concurrent download workers")
	buildCmd.Flags().StringVarP(&cacheDir, "cache-dir", "d", "",
		"Package cache directory")
	buildCmd.Flags().StringVar(&workDir, "work-dir", "",
		"Working directory for builds")
	buildCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	buildCmd.Flags().StringVarP(&dotFile, "dotfile", "f", "", "Generate a dot file for the dependency graph")

	// Add flags to install-completion command
	installCompletionCmd.Flags().String("shell", "", "Specify shell type (bash, zsh, fish, powershell)")
	installCompletionCmd.Flags().Bool("force", false, "Force overwrite existing completion files")

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "",
		"Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", globalConfig.Logging.Level,
		"Log level (debug, info, warn, error)")

	// Add subcommands to config command
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)

	// Add commands to root command
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(installCompletionCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
