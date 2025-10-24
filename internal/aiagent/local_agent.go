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
	rag       *TemplateRAG
}

// NewAIAgent creates an agent with the appropriate provider and RAG system
func NewAIAgent(provider string, config interface{}) (*AIAgent, error) {
	var chatModel ChatModel
	var embeddingClient EmbeddingGenerator

	switch provider {
	case "ollama":
		ollamaConfig, ok := config.(OllamaConfig)
		if !ok {
			return nil, fmt.Errorf("invalid Ollama configuration")
		}
		chatModel = NewOllamaChatModel(ollamaConfig)

		// Use Ollama for embeddings as well
		embeddingClient = NewOllamaEmbeddingClient(
			ollamaConfig.BaseURL,
			"nomic-embed-text", // Dedicated embedding model
		)

	case "openai":
		openaiConfig, ok := config.(OpenAIConfig)
		if !ok {
			return nil, fmt.Errorf("invalid OpenAI configuration")
		}
		chatModel = NewOpenAIChatModel(openaiConfig)

		// Use OpenAI for embeddings
		embeddingClient = NewOpenAIEmbeddingClient(
			openaiConfig.APIKey,
			"text-embedding-3-small", // Cost-effective embedding model
		)

	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}

	// Initialize RAG system with template examples
	fmt.Println("🔍 Initializing RAG system...")
	rag, err := NewTemplateRAG("./image-templates", embeddingClient)
	if err != nil {
		// Fallback to current directory if image-templates doesn't exist
		fmt.Println("Warning: ./image-templates not found, searching current directory...")
		rag, err = NewTemplateRAG(".", embeddingClient)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RAG system: %w", err)
		}
	}

	// Build dynamic system prompt with available use cases
	systemPrompt := buildSystemPromptWithRAG(rag)
	chatModel.SetSystemPrompt(systemPrompt)

	return &AIAgent{
		chatModel: chatModel,
		rag:       rag,
	}, nil
}

func buildSystemPromptWithRAG(rag *TemplateRAG) string {
	availableUseCases := rag.GetAllUseCases()
	useCasesList := strings.Join(availableUseCases, ", ")

	return fmt.Sprintf(`You are an expert AI assistant for the OS Image Composer, a system that builds custom Linux OS images.

Your role is to understand user requirements and generate optimal OS image templates based on REAL working examples.

OS Image Composer Context:
- Supports: Azure Linux (azl3), EMT (emt3), eLxr (elxr12)
- Architectures: x86_64, aarch64
- Image types: raw, iso, img
- Available use case categories: %s

You will be provided with REAL template examples that match the user's request. Use these as references to generate the new template.

IMPORTANT: Return SINGLE values only, not lists!
- architecture: "x86_64" (NOT "x86_64, aarch64")
- distribution: "azl3" (NOT "azl3, emt3")
- image_type: "raw" (NOT "raw, iso")

Always respond with structured JSON:
{
  "use_case": "primary use case category",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "single value: x86_64 or aarch64",
  "distribution": "single value: azl3, emt3, or elxr12",
  "image_type": "single value: raw, iso, or img",
  "description": "brief description",
  "custom_packages": ["any additional packages user specifically requested"],
  "package_repositories": [
    {
      "codename": "repository codename",
      "url": "repository URL",
      "pkey": "GPG key URL",
      "component": "optional component"
    }
  ]
}

IMPORTANT: Extract repository information from user query!
- Look for: "codename", "url", "pkey", "repository"
- Parse repository details carefully
- Include all repositories mentioned by user

Return ONLY valid JSON, no markdown formatting.`, useCasesList)
}

func (agent *AIAgent) ProcessUserRequest(ctx context.Context, userInput string) (*OSImageTemplate, error) {
	// Step 1: Find relevant template examples using RAG
	fmt.Println("🔎 Finding relevant template examples...")
	relevantTemplates, err := agent.rag.FindRelevantTemplates(ctx, userInput, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find relevant templates: %w", err)
	}

	// Display found templates
	fmt.Printf("📋 Found %d relevant templates:\n", len(relevantTemplates))
	for i, result := range relevantTemplates {
		fmt.Printf("   %d. %s (similarity: %.2f)\n", i+1, result.Template.Name, result.Score)
	}

	// Step 2: Parse user intent with context from examples
	intent, err := agent.parseUserIntentWithExamples(ctx, userInput, relevantTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	intent = cleanIntent(intent)

	// Step 3: Generate template based on examples
	template, err := agent.generateTemplateFromExamples(intent, relevantTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}

	return template, nil
}

func (agent *AIAgent) parseUserIntentWithExamples(ctx context.Context, userInput string, examples []*SearchResult) (*TemplateIntent, error) {
	// Build prompt with example context
	examplesContext := agent.buildExamplesContext(examples)

	prompt := fmt.Sprintf(`Parse the following user request for OS Image Composer template generation.

RELEVANT TEMPLATE EXAMPLES FOR CONTEXT:
%s

User Request: "%s"

Based on the examples above and the user's request, return ONLY a JSON object with SINGLE values:
{
  "use_case": "choose the most appropriate use case",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "x86_64 or aarch64 (choose ONE)",
  "distribution": "azl3, emt3, or elxr12 (choose ONE based on examples)",
  "image_type": "raw, iso, or img (choose ONE)",
  "description": "brief description",
  "custom_packages": ["any specific packages the user mentioned"]
}

Return ONLY valid JSON, no markdown.`, examplesContext, userInput)

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
		// Use distribution from best matching example
		if len(examples) > 0 && examples[0].Template.Distribution != "" {
			intent.Distribution = examples[0].Template.Distribution
		} else {
			intent.Distribution = "azl3"
		}
	}
	if intent.ImageType == "" {
		intent.ImageType = "raw"
	}

	return &intent, nil
}

func (agent *AIAgent) buildExamplesContext(examples []*SearchResult) string {
	var sb strings.Builder

	for i, result := range examples {
		sb.WriteString(fmt.Sprintf("\n--- Example %d (Similarity: %.2f) ---\n", i+1, result.Score))
		sb.WriteString(agent.rag.FormatTemplateForPrompt(result.Template, false))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (agent *AIAgent) generateTemplateFromExamples(intent *TemplateIntent, examples []*SearchResult) (*OSImageTemplate, error) {
	// Use the best matching template as base
	baseTemplate := examples[0].Template

	// Merge packages from top examples
	packages := make([]string, 0)

	// Collect packages from relevant examples
	for _, result := range examples {
		if result.Score > 0.7 { // Only use highly relevant examples
			packages = append(packages, result.Template.Packages...)
		}
	}

	// Add custom packages requested by user
	if len(intent.CustomPackages) > 0 {
		packages = append(packages, intent.CustomPackages...)
		fmt.Printf("➕ Adding custom packages: %v\n", intent.CustomPackages)
	}

	// Remove duplicates
	packages = uniqueStrings(packages)

	// Filter packages based on requirements
	if contains(intent.Requirements, "minimal") && len(packages) > 20 {
		// Keep only essential packages for minimal requirement
		packages = filterEssentialPackages(packages)
	}

	// Get kernel configuration from example
	kernelVersion := baseTemplate.KernelInfo
	if kernelVersion == "" {
		kernelVersion = "6.12" // fallback
	}

	kernelCmdline := "console=ttyS0,115200 console=tty0 loglevel=7"

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

	if len(intent.PackageRepositories) > 0 {
		repos := make([]PackageRepository, len(intent.PackageRepositories))
		for i, repoIntent := range intent.PackageRepositories {
			repos[i] = PackageRepository{
				Codename:  repoIntent.Codename,
				URL:       repoIntent.URL,
				PKey:      repoIntent.PKey,
				Component: repoIntent.Component,
			}
		}
		template.PackageRepositories = repos
		fmt.Printf("📦 Added %d custom repositories\n", len(repos))
	}

	// Add disk configuration for raw images (learn from examples)
	if intent.ImageType == "raw" && baseTemplate.HasDisk {
		diskSize := "8GiB"
		if len(packages) > 50 {
			diskSize = "12GiB"
		}
		if len(packages) > 100 {
			diskSize = "20GiB"
		}
		template.Disk = generateDiskConfig(intent, diskSize)
	}

	// Add package repositories if examples have them
	if len(baseTemplate.Repositories) > 0 {
		fmt.Printf("📦 Template requires custom repositories: %v\n", baseTemplate.Repositories)
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

func filterEssentialPackages(packages []string) []string {
	// Keep only essential system packages for minimal installs
	essential := []string{
		"systemd", "kernel", "bash", "coreutils", "util-linux",
		"filesystem", "glibc", "openssl", "busybox",
	}

	filtered := make([]string, 0)
	for _, pkg := range packages {
		for _, ess := range essential {
			if strings.Contains(strings.ToLower(pkg), ess) {
				filtered = append(filtered, pkg)
				break
			}
		}
	}

	// Ensure we have at least some packages
	if len(filtered) == 0 {
		filtered = packages[:min(15, len(packages))]
	}

	return filtered
}
