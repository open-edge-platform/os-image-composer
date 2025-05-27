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

// Config options with defaults
var (
	workers  int    = 8
	cacheDir string = "./downloads"
	verbose  bool
	dotFile  string
)

// jsonFileCompletion helps with suggesting JSON files for spec file argument
func jsonFileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"*.json"}, cobra.ShellCompDirectiveFilterFileExt
}

// executeBuild handles the build command execution logic
func executeBuild(cmd *cobra.Command, args []string) error {
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

	// Populate the cache download
	absCacheDir, err := filepath.Abs(cacheDir)
	if err != nil {
		return fmt.Errorf("invalid cache directory: %v", err)
	}
	logger.Infof("downloading %d packages to %s", len(urls), absCacheDir)
	if err := pkgfetcher.FetchPackages(urls, absCacheDir, workers); err != nil {
		return fmt.Errorf("fetch failed: %v", err)
	}
	logger.Info("all downloads complete")

	// Verify downloaded packages
	if err := p.Validate(cacheDir); err != nil {
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
	var sourceCmd string

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

		// If we're using home directory version, we need to source it in .bashrc
		if strings.HasPrefix(completionDir, homeDir) {
			sourceCmd = fmt.Sprintf("source %s", targetPath)

			// Check if it's already in .bashrc
			bashrcPath := filepath.Join(homeDir, ".bashrc")
			if _, err := os.Stat(bashrcPath); err == nil {
				content, err := os.ReadFile(bashrcPath)
				if err == nil && !strings.Contains(string(content), sourceCmd) {
					fmt.Printf("Adding completion source to %s\n", bashrcPath)
					f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						defer f.Close()
						if _, err := f.WriteString("\n# Image Composer Tool completion\n" + sourceCmd + "\n"); err != nil {
							fmt.Printf("Warning: Could not update %s: %v\n", bashrcPath, err)
						}
					}
				}
			}
		}

	case "zsh":
		// Check for common zsh completion paths
		completionDir := "/usr/local/share/zsh/site-functions"
		if _, err := os.Stat(completionDir); os.IsNotExist(err) {
			completionDir = filepath.Join(homeDir, ".zsh/completion")
			if _, err := os.Stat(completionDir); os.IsNotExist(err) {
				if err := os.MkdirAll(completionDir, 0755); err != nil {
					return fmt.Errorf("could not create directory %s: %v", completionDir, err)
				}
			}

			// Add to .zshrc if needed
			sourceCmd = fmt.Sprintf("fpath=(%s $fpath)", completionDir)
			zshrcPath := filepath.Join(homeDir, ".zshrc")
			if _, err := os.Stat(zshrcPath); err == nil {
				content, err := os.ReadFile(zshrcPath)
				if err == nil && !strings.Contains(string(content), sourceCmd) {
					fmt.Printf("Adding completion directory to $fpath in %s\n", zshrcPath)
					f, err := os.OpenFile(zshrcPath, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						defer f.Close()
						if _, err := f.WriteString("\n# Image Composer Tool completion\n" + sourceCmd + "\nautoload -Uz compinit && compinit\n"); err != nil {
							fmt.Printf("Warning: Could not update %s: %v\n", zshrcPath, err)
						}
					}
				}
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
		// Check if PowerShell profile exists
		psPath := os.Getenv("PSMODULEPATH")
		if psPath == "" {
			psPath = filepath.Join(homeDir, "Documents/WindowsPowerShell/Modules")
			if _, err := os.Stat(psPath); os.IsNotExist(err) {
				if err := os.MkdirAll(psPath, 0755); err != nil {
					return fmt.Errorf("could not create directory %s: %v", psPath, err)
				}
			}
		}

		completionDir := filepath.Join(psPath, "image-composer")
		if _, err := os.Stat(completionDir); os.IsNotExist(err) {
			if err := os.MkdirAll(completionDir, 0755); err != nil {
				return fmt.Errorf("could not create directory %s: %v", completionDir, err)
			}
		}

		targetPath = filepath.Join(completionDir, "image-composer-completion.ps1")

		// We may need to add module to profile
		profilePath := os.Getenv("PROFILE")
		if profilePath == "" {
			fmt.Println("PowerShell profile not found. You may need to manually import the module.")
		} else {
			moduleCmd := fmt.Sprintf("Import-Module %s", completionDir)
			if _, err := os.Stat(profilePath); err == nil {
				content, err := os.ReadFile(profilePath)
				if err == nil && !strings.Contains(string(content), moduleCmd) {
					fmt.Printf("Adding module import to PowerShell profile at %s\n", profilePath)
					f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						defer f.Close()
						if _, err := f.WriteString("\n# Image Composer Tool completion\n" + moduleCmd + "\n"); err != nil {
							fmt.Printf("Warning: Could not update profile: %v\n", err)
						}
					}
				}
			}
		}
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

	if sourceCmd != "" {
		fmt.Printf("\nTo enable completion in your current shell, run:\n  %s\n", sourceCmd)
	}

	if shellType == "powershell" {
		fmt.Printf("\nTo enable completion in your current PowerShell session, run:\n  Import-Module %s\n", filepath.Dir(targetPath))
	}

	return nil
}

func main() {

	// Setup zap logger, and defer the cleanup function
	_, cleanup := utils.Init()
	defer cleanup()

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

	// Install completion command
	installCompletionCmd := &cobra.Command{
		Use:   "install-completion",
		Short: "Install shell completion script",
		Long: `Install shell completion script for Bash, Zsh, Fish, or PowerShell.
Automatically detects your shell and installs the appropriate completion script.
If needed, it will also update your shell profile to load the completions.`,
		RunE: executeInstallCompletion,
	}

	// Add flags to install-completion command
	installCompletionCmd.Flags().String("shell", "", "Specify shell type (bash, zsh, fish, powershell)")
	installCompletionCmd.Flags().Bool("force", false, "Force overwrite existing completion files")

	// Add flags to build command
	buildCmd.Flags().IntVarP(&workers, "workers", "w", workers, "Number of concurrent download workers")
	buildCmd.Flags().StringVarP(&cacheDir, "cache-dir", "d", cacheDir, "Package cache directory")
	buildCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	buildCmd.Flags().StringVarP(&dotFile, "dotfile", "f", "", "Generate a dot file for the dependency graph")

	// Add commands to root command
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCompletionCmd)

	// Add global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
