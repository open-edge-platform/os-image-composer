package aiagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

var (
	supportedImageTypes = map[string]bool{
		"raw":   true,
		"qcow2": true,
		"vhd":   true,
		"vhdx":  true,
		"vmdk":  true,
		"vdi":   true,
		"iso":   true,
		"img":   true,
	}
	diskArtifactTypes = map[string]bool{
		"raw":   true,
		"qcow2": true,
		"vhd":   true,
		"vhdx":  true,
		"vmdk":  true,
		"vdi":   true,
	}
)

const supportedImageTypePrompt = "raw, qcow2, vhd, vhdx, vmdk, or vdi"

// ChatModel interface for different LLM providers
type ChatModel interface {
	SendMessage(ctx context.Context, message string) (string, error)
	SendStructuredMessage(ctx context.Context, message string, schema interface{}) error
	SetSystemPrompt(prompt string)
	ResetConversation()
}

type AIAgent struct {
	chatModel  ChatModel
	rag        *TemplateRAG
	useCaseRAG *UseCaseRAG
	useCases   *UseCasesConfig
}

// NewAIAgent creates an agent with the appropriate provider and RAG system
func NewAIAgent(provider string, config interface{}, templatesDir string) (*AIAgent, error) {
	var chatModel ChatModel
	var embeddingClient EmbeddingGenerator
	ctx := context.Background()

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
			ollamaConfig.EmbeddingModel,
			ollamaConfig.Timeout,
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
			openaiConfig.EmbeddingModel,
			openaiConfig.Timeout,
		)

	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}

	// Initialize RAG system with template examples
	templatesDir = strings.TrimSpace(templatesDir)
	if templatesDir == "" {
		templatesDir = "./image-templates"
	}

	fmt.Printf("🔍 Initializing RAG system from %s...\n", templatesDir)
	rag, err := NewTemplateRAG(templatesDir, embeddingClient)
	if err != nil {
		// Fallback to current directory if image-templates doesn't exist
		fmt.Printf("Warning: %s not found, searching current directory...\n", templatesDir)
		rag, err = NewTemplateRAG(".", embeddingClient)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RAG system: %w", err)
		}
	}

	// Build dynamic system prompt with available use cases
	var useCases *UseCasesConfig
	var useCaseRAG *UseCaseRAG

	useCases, err = LoadUseCases("")
	if err != nil {
		fmt.Printf("ℹ️  Use case metadata unavailable (%v); continuing without curated guidance.\n", err)
	} else {
		useCaseRAG, err = NewUseCaseRAG(ctx, useCases, embeddingClient)
		if err != nil {
			fmt.Printf("Warning: failed to initialize use case RAG: %v\n", err)
		} else {
			fmt.Printf("📚 Loaded curated guidance for %d use cases.\n", len(useCases.UseCases))
		}
	}

	systemPrompt := buildSystemPromptWithRAG(rag, useCaseRAG)
	chatModel.SetSystemPrompt(systemPrompt)

	return &AIAgent{
		chatModel:  chatModel,
		rag:        rag,
		useCaseRAG: useCaseRAG,
		useCases:   useCases,
	}, nil
}

func buildSystemPromptWithRAG(rag *TemplateRAG, useCaseRAG *UseCaseRAG) string {
	useCaseSet := make(map[string]struct{})

	if rag != nil {
		for _, name := range rag.GetAllUseCases() {
			useCaseSet[name] = struct{}{}
		}
	}

	if useCaseRAG != nil {
		for _, name := range useCaseRAG.UseCaseNames() {
			useCaseSet[name] = struct{}{}
		}
	}

	combined := make([]string, 0, len(useCaseSet))
	for name := range useCaseSet {
		combined = append(combined, name)
	}

	sort.Strings(combined)
	if len(combined) == 0 {
		combined = append(combined, "general")
	}

	useCasesList := strings.Join(combined, ", ")

	return fmt.Sprintf(`You are an expert AI assistant for the OS Image Composer, a system that builds custom Linux OS images.

Your role is to understand user requirements and generate optimal OS image templates based on REAL working examples.

OS Image Composer Context:
- Supports: Azure Linux (azl3), EMT (emt3), eLxr (elxr12)
- Architectures: x86_64, aarch64
- Disk artifact types: %s
- Available use case categories: %s

You will be provided with REAL template examples that match the user's request. Use these as references to generate the new template.

IMPORTANT: Return SINGLE values only, not lists!
- architecture: "x86_64" (NOT "x86_64, aarch64")
- distribution: "azl3" (NOT "azl3, emt3")
- image_type: choose ONE of %s

Always respond with structured JSON:
{
  "use_case": "primary use case category",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "single value: x86_64 or aarch64",
  "distribution": "single value: azl3, emt3, or elxr12",
	"image_type": "single value: one of %s",
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

Return ONLY valid JSON, no markdown formatting.`, supportedImageTypePrompt, useCasesList, supportedImageTypePrompt, supportedImageTypePrompt)
}

func (agent *AIAgent) ProcessUserRequest(ctx context.Context, userInput string) (*OSImageTemplate, error) {
	var useCaseMatches []*UseCaseMatch

	if agent.useCaseRAG != nil {
		fmt.Println("🧭 Matching curated use cases...")
		matches, err := agent.useCaseRAG.FindRelevantUseCases(ctx, userInput, 3)
		if err != nil {
			fmt.Printf("Warning: failed to match curated use cases: %v\n", err)
		} else {
			for _, match := range matches {
				if match.Score <= 0 {
					continue
				}
				useCaseMatches = append(useCaseMatches, match)
			}
			if len(useCaseMatches) > 0 {
				fmt.Println("📚 Curated use case hints:")
				for i, match := range useCaseMatches {
					fmt.Printf("   %d. %s (score: %.2f)\n", i+1, match.Name, match.Score)
				}
			} else {
				fmt.Println("ℹ️  No curated use case matches above threshold; using template-only search.")
			}
		}
	}

	fmt.Println("🔎 Finding relevant template examples...")
	useCaseFilter := make([]string, 0, len(useCaseMatches))
	for _, match := range useCaseMatches {
		if match.Score >= 0.2 {
			useCaseFilter = append(useCaseFilter, match.Name)
		}
	}

	relevantTemplates, err := agent.rag.FindRelevantTemplatesForUseCases(ctx, userInput, useCaseFilter, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find relevant templates: %w", err)
	}
	if len(relevantTemplates) == 0 {
		relevantTemplates, err = agent.rag.FindRelevantTemplates(ctx, userInput, 3)
		if err != nil {
			return nil, fmt.Errorf("failed to find relevant templates: %w", err)
		}
	}
	if len(relevantTemplates) == 0 {
		return nil, fmt.Errorf("no template examples available for generation")
	}

	fmt.Printf("📋 Found %d relevant templates:\n", len(relevantTemplates))
	for i, result := range relevantTemplates {
		fmt.Printf("   %d. %s (similarity: %.2f)\n", i+1, result.Template.Name, result.Score)
	}

	intent, err := agent.parseUserIntentWithExamples(ctx, userInput, relevantTemplates, useCaseMatches)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	intent = cleanIntent(intent)

	if intent.UseCase == "" {
		if len(useCaseMatches) > 0 {
			intent.UseCase = useCaseMatches[0].Name
		} else if len(relevantTemplates) > 0 && relevantTemplates[0].Template.UseCase != "" {
			intent.UseCase = relevantTemplates[0].Template.UseCase
		} else {
			intent.UseCase = "general"
		}
	}
	if agent.useCases != nil && !agent.useCases.HasUseCase(intent.UseCase) && len(useCaseMatches) > 0 {
		intent.UseCase = useCaseMatches[0].Name
	}

	var primaryUseCase *UseCaseMatch
	if len(useCaseMatches) > 0 {
		primaryUseCase = useCaseMatches[0]
	}

	template, err := agent.generateTemplateFromExamples(intent, relevantTemplates, primaryUseCase)
	if err != nil {
		return nil, fmt.Errorf("failed to generate template: %w", err)
	}

	return template, nil
}

func (agent *AIAgent) parseUserIntentWithExamples(ctx context.Context, userInput string, examples []*SearchResult, useCases []*UseCaseMatch) (*TemplateIntent, error) {
	examplesContext := agent.buildExamplesContext(examples)
	useCaseContext := agent.buildUseCaseContext(useCases)

	prompt := fmt.Sprintf(`Parse the following user request for OS Image Composer template generation.

CURATED USE CASE HINTS:
%s

RELEVANT TEMPLATE EXAMPLES FOR CONTEXT:
%s

User Request: "%s"

Based on the curated hints, examples above, and the user's request, return ONLY a JSON object with SINGLE values:
{
  "use_case": "choose the most appropriate use case",
  "requirements": ["array of: security, performance, minimal"],
  "architecture": "x86_64 or aarch64 (choose ONE)",
	"distribution": "azl3, emt3, or elxr12 (choose ONE based on examples)",
	"image_type": "raw, qcow2, vhd, vhdx, vmdk, or vdi (choose ONE)",
  "description": "brief description",
  "custom_packages": ["any specific packages the user mentioned"],
  "package_repositories": [
    {
      "codename": "repository codename",
      "url": "repository URL",
      "pkey": "GPG key URL",
      "component": "optional component"
    }
  ]
}

Return ONLY valid JSON, no markdown.`, useCaseContext, examplesContext, userInput)

	var intent TemplateIntent
	if err := agent.chatModel.SendStructuredMessage(ctx, prompt, &intent); err != nil {
		return nil, err
	}

	if intent.Architecture == "" {
		intent.Architecture = "x86_64"
	}
	if intent.Distribution == "" {
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

func (agent *AIAgent) buildUseCaseContext(matches []*UseCaseMatch) string {
	if len(matches) == 0 {
		return "No curated use case hints available."
	}

	var sb strings.Builder
	for i, match := range matches {
		sb.WriteString(fmt.Sprintf("\n--- Use Case %d (Score: %.2f) ---\n", i+1, match.Score))
		if match.Config != nil {
			if match.Config.Description != "" {
				sb.WriteString(fmt.Sprintf("Description: %s\n", match.Config.Description))
			}
			if len(match.Config.Keywords) > 0 {
				sb.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(match.Config.Keywords, ", ")))
			}
			if len(match.Config.EssentialPackages) > 0 {
				sb.WriteString(fmt.Sprintf("Essential packages: %s\n", strings.Join(match.Config.EssentialPackages, ", ")))
			}
			if len(match.Config.OptionalPackages) > 0 {
				sb.WriteString(fmt.Sprintf("Optional packages: %s\n", strings.Join(match.Config.OptionalPackages, ", ")))
			}
			if len(match.Config.SecurityPackages) > 0 {
				sb.WriteString(fmt.Sprintf("Security packages: %s\n", strings.Join(match.Config.SecurityPackages, ", ")))
			}
			if len(match.Config.PerformancePackages) > 0 {
				sb.WriteString(fmt.Sprintf("Performance packages: %s\n", strings.Join(match.Config.PerformancePackages, ", ")))
			}
			if match.Config.Kernel.DefaultVersion != "" {
				sb.WriteString(fmt.Sprintf("Default kernel version: %s\n", match.Config.Kernel.DefaultVersion))
			}
			if match.Config.Kernel.Cmdline != "" {
				sb.WriteString(fmt.Sprintf("Kernel cmdline: %s\n", match.Config.Kernel.Cmdline))
			}
			if match.Config.Disk.DefaultSize != "" {
				sb.WriteString(fmt.Sprintf("Suggested disk size: %s\n", match.Config.Disk.DefaultSize))
			}
		} else if match.ContextText != "" {
			sb.WriteString(match.ContextText)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (agent *AIAgent) generateTemplateFromExamples(intent *TemplateIntent, examples []*SearchResult, primaryUseCase *UseCaseMatch) (*OSImageTemplate, error) {
	if len(examples) == 0 {
		return nil, fmt.Errorf("no example templates provided for generation")
	}

	baseTemplate := examples[0].Template

	var curatedConfig *UseCaseConfig
	if primaryUseCase != nil && primaryUseCase.Config != nil {
		curatedConfig = primaryUseCase.Config
	} else if agent.useCases != nil {
		if cfg, err := agent.useCases.GetUseCase(intent.UseCase); err == nil {
			curatedConfig = cfg
		}
	}

	packages := make([]string, 0)
	if curatedConfig != nil {
		curatedPackages := curatedConfig.PackagesForRequirements(intent.Requirements)
		if len(curatedPackages) > 0 {
			packages = append(packages, curatedPackages...)
			fmt.Printf("🧭 Curated packages for use case '%s' (%d): %v\n", intent.UseCase, len(curatedPackages), curatedPackages)
		}
	}

	for _, result := range examples {
		if result.Score >= 0.60 {
			if len(result.Template.Packages) > 0 {
				fmt.Printf("📘 Template '%s' contributing packages (%d, score %.2f): %v\n", result.Template.Name, len(result.Template.Packages), result.Score, result.Template.Packages)
			}
			packages = append(packages, result.Template.Packages...)
		} else {
			fmt.Printf("ℹ️  Skipping template '%s' (score %.2f below threshold).\n", result.Template.Name, result.Score)
		}
	}

	if len(intent.CustomPackages) > 0 {
		packages = append(packages, intent.CustomPackages...)
		fmt.Printf("➕ Adding custom packages: %v\n", intent.CustomPackages)
	}

	packages = uniqueStrings(packages)

	if contains(intent.Requirements, "minimal") && len(packages) > 20 {
		packages = filterEssentialPackages(packages)
	}

	kernelVersion := baseTemplate.KernelInfo
	kernelCmdline := "console=ttyS0,115200 console=tty0 loglevel=7"
	if agent.useCases != nil {
		if kernelVersion == "" {
			kernelVersion = agent.useCases.GetKernelVersion(intent.UseCase, intent.Distribution)
		}
		kernelCmdline = agent.useCases.GetKernelCmdline(intent.UseCase)
	} else if kernelVersion == "" {
		kernelVersion = "6.12"
	}

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

	if len(baseTemplate.Repositories) > 0 {
		fmt.Printf("📦 Template requires custom repositories: %v\n", baseTemplate.Repositories)
	}

	createDisk := isDiskArtifactType(intent.ImageType)
	if !createDisk && baseTemplate.HasDisk && intent.ImageType == "raw" {
		createDisk = true
	}

	if createDisk {
		diskSize := ""
		if baseTemplate.HasDisk {
			diskSize = "8GiB"
			if len(packages) > 50 {
				diskSize = "12GiB"
			}
			if len(packages) > 100 {
				diskSize = "20GiB"
			}
		}
		if diskSize == "" && curatedConfig != nil {
			diskSize = curatedConfig.Disk.DefaultSize
		}
		if diskSize == "" {
			diskSize = "8GiB"
		}
		template.Disk = generateDiskConfig(intent, diskSize)
	}

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

	diskType := intent.ImageType
	if diskType == "" || !supportedImageTypes[diskType] {
		diskType = "raw"
	}

	artifact := ArtifactSpec{Type: diskType}
	if diskType == "raw" {
		artifact.Compression = "gz"
	}

	return &DiskConfig{
		Name:               fmt.Sprintf("%s_disk", intent.UseCase),
		Artifacts:          []ArtifactSpec{artifact},
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
	intent.ImageType = normalizeImageType(intent.ImageType)
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

func normalizeImageType(value string) string {
	primary := ""
	if value != "" {
		primary = strings.TrimSpace(strings.Split(value, ",")[0])
	}
	primary = strings.ToLower(primary)
	if primary == "" {
		return "raw"
	}
	if supportedImageTypes[primary] {
		return primary
	}
	return "raw"
}

func isDiskArtifactType(value string) bool {
	if value == "" {
		return false
	}
	return diskArtifactTypes[strings.ToLower(value)]
}
