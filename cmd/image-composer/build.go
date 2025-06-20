package main

import (
	"fmt"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/provider/azl"
	"github.com/open-edge-platform/image-composer/internal/provider/elxr"
	"github.com/open-edge-platform/image-composer/internal/provider/emt"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
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
	// Note: We update the global singleton with any overrides
	if cmd.Flags().Changed("workers") {
		currentConfig := config.Global()
		currentConfig.Workers = workers
		config.SetGlobal(currentConfig)
	}
	if cmd.Flags().Changed("cache-dir") {
		currentConfig := config.Global()
		currentConfig.CacheDir = cacheDir
		config.SetGlobal(currentConfig)
	}
	if cmd.Flags().Changed("work-dir") {
		currentConfig := config.Global()
		currentConfig.WorkDir = workDir
		config.SetGlobal(currentConfig)
	}

	log := logger.Logger()

	// Check if template file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no template file provided, usage: image-composer build [flags] TEMPLATE_FILE")
	}
	templateFile := args[0]

	// Load user template and merge with default configuration
	template, err := config.LoadAndMergeTemplate(templateFile)
	if err != nil {
		return fmt.Errorf("loading and merging template: %v", err)
	}

	p, err := InitProvider(config.TargetOs, config.TargetDist, config.TargetArch)
	if err != nil {
		return fmt.Errorf("initializing provider failed: %v", err)
	}

	if err := p.PreProcess(template); err != nil {
		return fmt.Errorf("pre-processing failed: %v", err)
	}

	if err := p.BuildImage(template); err != nil {
		return fmt.Errorf("image build failed: %v", err)
	}

	if err := p.PostProcess(template); err != nil {
		return fmt.Errorf("post-processing failed: %v", err)
	}

	log.Info("build completed successfully")
	return nil
}

func InitProvider(os, dist, arch string) (provider.Provider, error) {

	var p provider.Provider
	switch os {
	case "azure-linux":
		azl.Register(dist, arch)
		config.ProviderId = azl.GetProviderId(dist, arch)
	case "edge-microvisor-toolkit":
		emt.Register(dist, arch)
		config.ProviderId = emt.GetProviderId(dist, arch)
	case "elxr":
		elxr.Register(dist, arch)
		config.ProviderId = elxr.GetProviderId(dist, arch)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", os)
	}
	p, ok := provider.Get(config.ProviderId)
	if !ok {
		return nil, fmt.Errorf("provider not found for %s %s %s", os, dist, arch)
	}
	return p, p.Init(dist, arch)
}

// templateFileCompletion helps with suggesting YAML files for template file argument
func templateFileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"*.yml", "*.yaml"}, cobra.ShellCompDirectiveFilterFileExt
}
