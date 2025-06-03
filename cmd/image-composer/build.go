package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/provider"
	_ "github.com/open-edge-platform/image-composer/internal/provider/azurelinux3" // register provider
	_ "github.com/open-edge-platform/image-composer/internal/provider/elxr12"      // register provider
	_ "github.com/open-edge-platform/image-composer/internal/provider/emt3_0"      // register provider
	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/package/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/utils/package/rpmutils"
	"github.com/spf13/cobra"
)

// Build command flags
var (
	workers  int    = -1 // -1 means use config file value
	cacheDir string = "" // Empty means use config file value
	workDir  string = "" // Empty means use config file value
	verbose  bool   = false
	dotFile  string = ""
)

// createBuildCommand creates the build subcommand
func createBuildCommand() *cobra.Command {
	buildCmd := &cobra.Command{
		Use:   "build [flags] TEMPLATE_FILE",
		Short: "Build a Linux distribution image",
		Long: `Build a Linux distribution image based on the specified image template file.
The template file must be in YAML format following the image template schema.`,
		Args:              cobra.ExactArgs(1),
		RunE:              executeBuild,
		ValidArgsFunction: templateFileCompletion,
	}

	// Add flags
	buildCmd.Flags().IntVarP(&workers, "workers", "w", -1,
		"Number of concurrent download workers")
	buildCmd.Flags().StringVarP(&cacheDir, "cache-dir", "d", "",
		"Package cache directory")
	buildCmd.Flags().StringVar(&workDir, "work-dir", "",
		"Working directory for builds")
	buildCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	buildCmd.Flags().StringVarP(&dotFile, "dotfile", "f", "", "Generate a dot file for the dependency graph")

	return buildCmd
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

	logger := logger.Logger()

	// Check if template file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no template file provided, usage: image-composer build [flags] TEMPLATE_FILE")
	}
	templateFile := args[0]

	// Load and validate the image template
	template, err := config.LoadTemplate(templateFile)
	if err != nil {
		return fmt.Errorf("loading template file: %v", err)
	}

	providerName := template.GetProviderName()
	if providerName == "" {
		return fmt.Errorf("no provider found for OS %s with distribution %s", template.Target.OS, template.Target.Dist)
	}

	// Get provider by name
	p, ok := provider.Get(providerName)
	if !ok {
		return fmt.Errorf("provider not found: %s", providerName)
	}

	// Initialize provider with the template
	if err := p.Init(template); err != nil {
		return fmt.Errorf("provider init: %v", err)
	}

	// Fetch the entire package list
	all, err := p.Packages()
	if err != nil {
		return fmt.Errorf("getting packages: %v", err)
	}

	// Match the packages in the template against all the packages
	req, err := p.MatchRequested(template.GetPackages(), all)
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

// templateFileCompletion helps with suggesting YAML files for template file argument
func templateFileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"*.yml", "*.yaml"}, cobra.ShellCompDirectiveFilterFileExt
}
