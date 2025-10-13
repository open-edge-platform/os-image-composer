package aiagent

import (
	"context"
	"fmt"
	"strings"
)

// ChatModel interface for different LLM providers
type ChatModel interface {
	SendMessage(ctx context.Context, message string) (string, error)
	SendStructuredMessage(ctx context.Context, message string, schema interface{}) error
	SetSystemPrompt(prompt string)
	ResetConversation()
}

type AIAgent struct {
	chatModel ChatModel
	useCases  *UseCasesConfig
}

// NewAIAgent creates an agent with the appropriate provider
func NewAIAgent(provider string, config interface{}) (*AIAgent, error) {
	var chatModel ChatModel
	
	switch provider {
	case "ollama":
		ollamaConfig, ok := config.(OllamaConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Ollama configuration")
		}
		chatModel = NewOllamaChatModel(ollamaConfig)
		
	case "openai":
		openaiConfig, ok := config.(OpenAIConfig)
		if !ok {
			return nil, fmt.Errorf("invalid OpenAI configuration")
		}
		chatModel = NewOpenAIChatModel(openaiConfig)
		
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}
	
	// Load use cases configuration
	useCases, err := LoadUseCases("")
	if err != nil {
		fmt.Printf("Warning: Failed to load use cases config: %v\n", err)
		useCases = &UseCasesConfig{UseCases: make(map[string]UseCaseConfig)}
	}
	
	// Build dynamic system prompt
	systemPrompt := buildSystemPrompt(useCases)
	chatModel.SetSystemPrompt(systemPrompt)
	
	return &AIAgent{
		chatModel: chatModel,
		useCases:  useCases,
	}, nil
}

func buildSystemPrompt(useCases *UseCasesConfig) string {
	availableUseCases := useCases.GetAllUseCaseNames()
	useCasesList := strings.Join(availableUseCases, ", ")
	
	return fmt.Sprintf(`You are an expert AI assistant for the OS Image Composer, a system that builds custom Linux OS images.

Your role is to understand user requirements and generate optimal package selections for OS Image Composer templates.

OS Image Composer Context:
- Supports: Azure Linux (azl3), EMT (emt3), eLxr (elxr12)
- Architectures: x86_64, aarch64
- Image types: raw, iso, img
- Available use cases: %s

IMPORTANT: Return SINGLE values only, not lists!
- architecture: "x86_64" (NOT "x86_64, aarch64")
- distribution: "azl3" (NOT "azl3, emt3")
- image_type: "raw" (NOT "raw, iso")

Always respond with structured JSON:
{
  "use_case": "single value from: %s",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "single value: x86_64 or aarch64",
  "distribution": "single value: azl3, emt3, or elxr12",
  "image_type": "single value: raw, iso, or img",
  "description": "brief description"
}

Return ONLY valid JSON, no markdown formatting.`, useCasesList, useCasesList)
}

func (agent *AIAgent) ProcessUserRequest(ctx context.Context, userInput string) (*OSImageTemplate, error) {
	intent, err := agent.parseUserIntent(ctx, userInput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}
	
	intent = cleanIntent(intent)
	
	template, err := agent.generateTemplate(intent)
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}
	
	return template, nil
}

func (agent *AIAgent) parseUserIntent(ctx context.Context, userInput string) (*TemplateIntent, error) {
	availableUseCases := agent.useCases.GetAllUseCaseNames()
	useCasesList := strings.Join(availableUseCases, ", ")
	
	prompt := fmt.Sprintf(`Parse the following user request for OS Image Composer template generation.

User Request: "%s"

Return ONLY a JSON object. Use SINGLE values (not comma-separated lists):
{
  "use_case": "choose ONE from: %s",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "x86_64 or aarch64 (choose ONE)",
  "distribution": "azl3, emt3, or elxr12 (choose ONE)",
  "image_type": "raw, iso, or img (choose ONE)",
  "description": "brief description"
}

Return ONLY valid JSON, no markdown.`, userInput, useCasesList)
	
	var intent TemplateIntent
	err := agent.chatModel.SendStructuredMessage(ctx, prompt, &intent)
	if err != nil {
		return nil, err
	}
	
	// Apply defaults
	if intent.Architecture == "" {
		intent.Architecture = "x86_64"
	}
	if intent.Distribution == "" {
		intent.Distribution = "azl3"
	}
	if intent.ImageType == "" {
		intent.ImageType = "raw"
	}
	
	// Validate use case exists
	if intent.UseCase == "" || !agent.useCases.HasUseCase(intent.UseCase) {
		intent.UseCase = agent.useCases.DetectUseCase(userInput)
	}
	
	return &intent, nil
}

func (agent *AIAgent) generateTemplate(intent *TemplateIntent) (*OSImageTemplate, error) {
	// Get packages from configurable use cases
	packages, err := agent.useCases.GetPackagesForUseCase(intent.UseCase, intent.Requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to get packages for use case: %w", err)
	}
	
	// Get kernel configuration
	kernelVersion := agent.useCases.GetKernelVersion(intent.UseCase, intent.Distribution)
	kernelCmdline := agent.useCases.GetKernelCmdline(intent.UseCase)
	
	template := &OSImageTemplate{
		Image: ImageConfig{
			Name:    fmt.Sprintf("%s-%s-%s", intent.Distribution, intent.Architecture, intent.UseCase),
			Version: "1.0.0",
		},
		Target: TargetConfig{
			OS:        mapDistToOS(intent.Distribution),
			Dist:      intent.Distribution,
			Arch:      intent.Architecture,
			ImageType: intent.ImageType,
		},
		SystemConfig: SystemConfig{
			Name:        intent.UseCase,
			Description: intent.Description,
			Packages:    packages,
			Kernel: &KernelConfig{
				Version: kernelVersion,
				Cmdline: kernelCmdline,
			},
		},
	}
	
	// Add disk configuration for raw images
	if intent.ImageType == "raw" {
		diskSize := agent.useCases.GetDiskSize(intent.UseCase)
		template.Disk = generateDiskConfig(intent, diskSize)
	}
	
	// Add immutability configuration if not minimal
	if !contains(intent.Requirements, "minimal") {
		template.SystemConfig.Immutability = &Immutability{
			Enabled: false,
		}
	}
	
	return template, nil
}

func generateDiskConfig(intent *TemplateIntent, size string) *DiskConfig {
	archType := "linux-root-amd64"
	if intent.Architecture == "aarch64" {
		archType = "linux-root-arm64"
	}
	
	return &DiskConfig{
		Name: fmt.Sprintf("%s_disk", intent.UseCase),
		Artifacts: []ArtifactSpec{
			{
				Type:        "raw",
				Compression: "gz",
			},
		},
		Size:               size,
		PartitionTableType: "gpt",
		Partitions: []Partition{
			{
				ID:           "boot",
				Type:         "esp",
				Flags:        []string{"esp", "boot"},
				Start:        "1MiB",
				End:          "513MiB",
				FSType:       "fat32",
				MountPoint:   "/boot/efi",
				MountOptions: "umask=0077",
			},
			{
				ID:           "rootfs",
				Type:         archType,
				Start:        "513MiB",
				End:          "0",
				FSType:       "ext4",
				MountPoint:   "/",
				MountOptions: "defaults",
			},
		},
	}
}

func mapDistToOS(dist string) string {
	mapping := map[string]string{
		"azl3":   "azure-linux",
		"emt3":   "edge-microvisor-toolkit",
		"elxr12": "wind-river-elxr",
	}
	if os, ok := mapping[dist]; ok {
		return os
	}
	return "azure-linux"
}

func cleanIntent(intent *TemplateIntent) *TemplateIntent {
	intent.Architecture = strings.TrimSpace(strings.Split(intent.Architecture, ",")[0])
	intent.Distribution = strings.TrimSpace(strings.Split(intent.Distribution, ",")[0])
	intent.ImageType = strings.TrimSpace(strings.Split(intent.ImageType, ",")[0])
	return intent
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, item := range input {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
