package main

import (
	"fmt"

	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
	"github.com/spf13/cobra"
)

// createValidateCommand creates the validate subcommand
func createValidateCommand() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate [flags] TEMPLATE_FILE",
		Short: "Validate an image template file",
		Long: `Validate an image template file against the schema without building it.
The template file must be in YAML format following the image template schema.
This allows checking for errors in your template before committing to a full build process.`,
		Args:              cobra.ExactArgs(1),
		RunE:              executeValidate,
		ValidArgsFunction: templateFileCompletion,
	}

	return validateCmd
}

// executeValidate handles the validate command logic
func executeValidate(cmd *cobra.Command, args []string) error {
	logger := logger.Logger()

	// Check if template file is provided as first positional argument
	if len(args) < 1 {
		return fmt.Errorf("no template file provided, usage: image-composer validate TEMPLATE_FILE")
	}
	templateFile := args[0]

	logger.Infof("validating template file: %s", templateFile)

	// Load and validate the image template
	template, err := config.LoadTemplate(templateFile)
	if err != nil {
		return fmt.Errorf("template validation failed: %v", err)
	}

	logger.Infof("✓ Template validation successful")
	logger.Infof("  Image: %s v%s", template.Image.Name, template.Image.Version)
	logger.Infof("  Target: %s %s (%s)", template.Target.OS, template.Target.Dist, template.Target.Arch)
	logger.Infof("  Output: %s", template.Target.ImageType)
	logger.Infof("  Provider: %s", template.GetProviderName())
	logger.Infof("  System Configs: %d", len(template.SystemConfigs))

	if len(template.SystemConfigs) > 0 {
		config := template.SystemConfigs[0]
		logger.Infof("  Config: %s (%s)", config.Name, config.Description)
		logger.Infof("  Packages: %d", len(config.Packages))
		logger.Infof("  Kernel: %s (%s)", config.Kernel.Version, config.Kernel.Cmdline)

		if verbose && len(config.Packages) > 0 {
			logger.Infof("  Package list:")
			for _, pkg := range config.Packages {
				logger.Infof("    - %s", pkg)
			}
		}
	}

	return nil
}
