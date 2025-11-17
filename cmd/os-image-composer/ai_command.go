package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/os-image-composer/internal/aiagent"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type aiAgent interface {
	ProcessUserRequest(ctx context.Context, userInput string) (*aiagent.OSImageTemplate, error)
}

var newAIAgent = func(provider string, config interface{}, templatesDir string, options *aiagent.AgentOptions) (aiAgent, error) {
	return aiagent.NewAIAgent(provider, config, templatesDir, options)
}

func createAICommand() *cobra.Command {
	var output string
	var attachmentPaths []string

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

			return runAIGeneration(args[0], aiConfig, output, attachmentPaths)
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "Output file path (default: stdout)")
	cmd.Flags().StringArrayVarP(&attachmentPaths, "file", "f", nil, "Path to a file to include as additional prompt context (repeatable)")

	return cmd
}

const maxAttachmentBytes = 64 * 1024

func runAIGeneration(userInput string, aiConfig config.AIConfig, outputPath string, attachmentPaths []string) error {
	ctx := context.Background()

	fmt.Printf("🤖 Generating template using %s", aiConfig.Provider)

	var (
		attachmentContext string
		attachmentCount   int
		err               error
	)

	if attachmentContext, attachmentCount, err = buildAttachmentContext(attachmentPaths); err != nil {
		return err
	}

	var agent aiAgent

	options := &aiagent.AgentOptions{
		TemplateContributionThreshold: aiConfig.TemplateContributionThreshold,
		UseCaseMatchThreshold:         aiConfig.UseCaseMatchThreshold,
	}

	switch aiConfig.Provider {
	case "ollama":
		fmt.Printf(" (%s)...\n", aiConfig.Ollama.Model)
		ollamaConfig := aiagent.OllamaConfig{
			BaseURL:        aiConfig.Ollama.BaseURL,
			Model:          aiConfig.Ollama.Model,
			Temperature:    aiConfig.Ollama.Temperature,
			NumPredict:     aiConfig.Ollama.MaxTokens,
			Timeout:        aiConfig.Ollama.Timeout,
			EmbeddingModel: aiConfig.Ollama.EmbeddingModel,
		}
		agent, err = newAIAgent("ollama", ollamaConfig, aiConfig.TemplatesDir, options)

	case "openai":
		fmt.Printf(" (%s)...\n", aiConfig.OpenAI.Model)
		openaiConfig := aiagent.OpenAIConfig{
			APIKey:         aiConfig.OpenAI.APIKey,
			Model:          aiConfig.OpenAI.Model,
			Temperature:    aiConfig.OpenAI.Temperature,
			MaxTokens:      aiConfig.OpenAI.MaxTokens,
			Timeout:        aiConfig.OpenAI.Timeout,
			EmbeddingModel: aiConfig.OpenAI.EmbeddingModel,
		}
		agent, err = newAIAgent("openai", openaiConfig, aiConfig.TemplatesDir, options)

	default:
		return fmt.Errorf("unsupported AI provider: %s", aiConfig.Provider)
	}

	if err != nil {
		return fmt.Errorf("failed to create AI agent: %w", err)
	}

	prompt := userInput
	if attachmentCount > 0 {
		fmt.Printf("📎 Included %d attachment(s) in prompt context\n", attachmentCount)
		prompt = strings.TrimSpace(userInput) + "\n\nATTACHMENT CONTEXT:\n" + attachmentContext
	}

	template, err := agent.ProcessUserRequest(ctx, prompt)
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

func buildAttachmentContext(paths []string) (string, int, error) {
	if len(paths) == 0 {
		return "", 0, nil
	}

	var sb strings.Builder
	count := 0

	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", 0, fmt.Errorf("failed to read attachment %s: %w", path, err)
		}

		if bytes.IndexByte(data, 0) >= 0 {
			fmt.Printf("⚠️  Skipping binary attachment: %s\n", path)
			continue
		}

		if len(data) == 0 {
			fmt.Printf("ℹ️  Attachment is empty, skipping: %s\n", path)
			continue
		}

		content := data
		if len(content) > maxAttachmentBytes {
			fmt.Printf("⚠️  Truncating attachment %s to %d bytes\n", path, maxAttachmentBytes)
			content = content[:maxAttachmentBytes]
		}

		sb.WriteString("\n--- Attachment: ")
		sb.WriteString(filepath.Base(path))
		sb.WriteString(" ---\n")
		sb.Write(content)
		if len(content) == 0 || content[len(content)-1] != '\n' {
			sb.WriteByte('\n')
		}
		count++
	}

	if count == 0 {
		return "", 0, nil
	}

	return sb.String(), count, nil
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
