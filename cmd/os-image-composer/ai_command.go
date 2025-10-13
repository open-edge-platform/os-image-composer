package main

import (
	"context"
	"fmt"
	"os"
	
	"github.com/open-edge-platform/os-image-composer/internal/aiagent"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func createAICommand() *cobra.Command {
	var output string
	
	cmd := &cobra.Command{
		Use:   "ai [prompt]",
		Short: "Generate OS Image Composer templates using AI (local LLM)",
		Long: `Generate OS Image Composer templates from natural language using local Ollama LLM.

Configuration is read from os-image-composer.yml under the 'ai' section.

Examples:
  os-image-composer ai "secure web server for production"
  os-image-composer ai "python web application with redis"
  os-image-composer ai "docker container host" --output template.yml
  os-image-composer ai "database server" --output db.yml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get AI config from global config (already loaded in main.go)
			aiConfig := config.GetAIConfig()
			
			// Check if AI is enabled
			if !config.AIEnabled() {
				return fmt.Errorf("AI feature is disabled in configuration. Enable it in os-image-composer.yml")
			}
			
			return runAIGeneration(args[0], aiConfig, output)
		},
	}
	
	cmd.Flags().StringVar(&output, "output", "", "Output file path (default: stdout)")
	
	return cmd
}

func runAIGeneration(userInput string, aiConfig config.AIConfig, outputPath string) error {
	ctx := context.Background()
	
	fmt.Printf("🤖 Generating template using %s", aiConfig.Provider)
	
	var agent *aiagent.AIAgent
	var err error
	
	switch aiConfig.Provider {
	case "ollama":
		fmt.Printf(" (%s)...\n", aiConfig.Ollama.Model)
		ollamaConfig := aiagent.OllamaConfig{
			BaseURL:     aiConfig.Ollama.BaseURL,
			Model:       aiConfig.Ollama.Model,
			Temperature: aiConfig.Ollama.Temperature,
			NumPredict:  aiConfig.Ollama.MaxTokens,
		}
		agent, err = aiagent.NewAIAgent("ollama", ollamaConfig)
		
	case "openai":
		fmt.Printf(" (%s)...\n", aiConfig.OpenAI.Model)
		openaiConfig := aiagent.OpenAIConfig{
			APIKey:      aiConfig.OpenAI.APIKey,
			Model:       aiConfig.OpenAI.Model,
			Temperature: aiConfig.OpenAI.Temperature,
			MaxTokens:   aiConfig.OpenAI.MaxTokens,
		}
		agent, err = aiagent.NewAIAgent("openai", openaiConfig)
		
	default:
		return fmt.Errorf("unsupported AI provider: %s", aiConfig.Provider)
	}
	
	if err != nil {
		return fmt.Errorf("failed to create AI agent: %w", err)
	}
	
	template, err := agent.ProcessUserRequest(ctx, userInput)
	if err != nil {
		return fmt.Errorf("failed to generate template: %w", err)
	}
	
	fmt.Printf("✓ Template generated successfully\n\n")
	displayTemplateSummary(template)
	
	// Convert to YAML
	yamlData, err := yaml.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to format template as YAML: %w", err)
	}
	
	// Save or print
	if outputPath != "" {
		if err := os.WriteFile(outputPath, yamlData, 0644); err != nil {
			return fmt.Errorf("failed to save template: %w", err)
		}
		fmt.Printf("\n💾 Saved to: %s\n", outputPath)
	} else {
		fmt.Printf("\n📄 Generated Template:\n")
		fmt.Printf("---\n%s", string(yamlData))
	}
	
	return nil
}

func displayTemplateSummary(template *aiagent.OSImageTemplate) {
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   Image: %s v%s\n", template.Image.Name, template.Image.Version)
	fmt.Printf("   OS: %s (%s)\n", template.Target.OS, template.Target.Dist)
	fmt.Printf("   Architecture: %s\n", template.Target.Arch)
	fmt.Printf("   Image Type: %s\n", template.Target.ImageType)
	
	if template.Disk != nil {
		fmt.Printf("   Disk Size: %s\n", template.Disk.Size)
	}
	
	if template.SystemConfig.Kernel != nil {
		fmt.Printf("   Kernel: %s\n", template.SystemConfig.Kernel.Version)
	}
	
	fmt.Printf("\n📦 Packages (%d):\n", len(template.SystemConfig.Packages))
	for i, pkg := range template.SystemConfig.Packages {
		if i < 10 {
			fmt.Printf("   - %s\n", pkg)
		} else if i == 10 {
			fmt.Printf("   ... and %d more\n", len(template.SystemConfig.Packages)-10)
			break
		}
	}
}
