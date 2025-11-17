package aiagent

import (
	"context"
	"strings"
	"testing"
)

func TestCleanIntentTrimsValues(t *testing.T) {
	intent := &TemplateIntent{
		Architecture: "x86_64, aarch64",
		Distribution: "azl3 , emt3",
		ImageType:    "raw , iso",
	}

	result := cleanIntent(intent)

	if result.Architecture != "x86_64" {
		t.Fatalf("expected architecture to be 'x86_64', got %q", result.Architecture)
	}
	if result.Distribution != "azl3" {
		t.Fatalf("expected distribution to be 'azl3', got %q", result.Distribution)
	}
	if result.ImageType != "raw" {
		t.Fatalf("expected image type to be 'raw', got %q", result.ImageType)
	}
	if result.ArtifactType != "raw" {
		t.Fatalf("expected artifact type to default to 'raw', got %q", result.ArtifactType)
	}
}

func TestGenerateDiskConfigForArm(t *testing.T) {
	intent := &TemplateIntent{UseCase: "edge", Architecture: "aarch64"}

	disk := generateDiskConfig(intent, "10GiB")

	if disk.Name != "edge_disk" {
		t.Fatalf("expected disk name 'edge_disk', got %q", disk.Name)
	}
	if len(disk.Partitions) != 2 {
		t.Fatalf("expected 2 partitions, got %d", len(disk.Partitions))
	}
	if disk.Partitions[1].Type != "linux-root-arm64" {
		t.Fatalf("expected second partition type 'linux-root-arm64', got %q", disk.Partitions[1].Type)
	}
	if len(disk.Artifacts) != 1 {
		t.Fatalf("expected a single artifact, got %d", len(disk.Artifacts))
	}
	if disk.Artifacts[0].Type != "raw" || disk.Artifacts[0].Compression != "gz" {
		t.Fatalf("expected raw artifact with gzip compression, got %#v", disk.Artifacts[0])
	}
}

func TestGenerateDiskConfigForX86(t *testing.T) {
	intent := &TemplateIntent{UseCase: "db", Architecture: "x86_64"}

	disk := generateDiskConfig(intent, "12GiB")

	if disk.Size != "12GiB" {
		t.Fatalf("expected disk size '12GiB', got %q", disk.Size)
	}
	if disk.Partitions[1].Type != "linux-root-amd64" {
		t.Fatalf("expected second partition type 'linux-root-amd64', got %q", disk.Partitions[1].Type)
	}
	if len(disk.Artifacts) != 1 {
		t.Fatalf("expected a single artifact, got %d", len(disk.Artifacts))
	}
	if disk.Artifacts[0].Type != "raw" || disk.Artifacts[0].Compression != "gz" {
		t.Fatalf("expected raw artifact with gzip compression, got %#v", disk.Artifacts[0])
	}
}

func TestGenerateDiskConfigUsesArtifactType(t *testing.T) {
	intent := &TemplateIntent{UseCase: "cloud", Architecture: "x86_64", ImageType: "raw", ArtifactType: "qcow2"}

	disk := generateDiskConfig(intent, "15GiB")

	if len(disk.Artifacts) != 1 {
		t.Fatalf("expected a single artifact, got %d", len(disk.Artifacts))
	}
	artifact := disk.Artifacts[0]
	if artifact.Type != "qcow2" {
		t.Fatalf("expected qcow2 artifact, got %q", artifact.Type)
	}
	if artifact.Compression != "" {
		t.Fatalf("expected no compression for qcow2, got %q", artifact.Compression)
	}
}

func TestGenerateDiskConfigUnsupportedArtifactDefaultsToRaw(t *testing.T) {
	intent := &TemplateIntent{UseCase: "cloud", Architecture: "x86_64", ImageType: "img", ArtifactType: "unknown"}

	disk := generateDiskConfig(intent, "15GiB")

	if len(disk.Artifacts) != 1 {
		t.Fatalf("expected a single artifact, got %d", len(disk.Artifacts))
	}
	artifact := disk.Artifacts[0]
	if artifact.Type != "raw" {
		t.Fatalf("expected fallback to raw artifact, got %q", artifact.Type)
	}
	if artifact.Compression != "gz" {
		t.Fatalf("expected gzip compression for raw artifact, got %q", artifact.Compression)
	}
}

func TestFilterEssentialPackagesKeepsMatches(t *testing.T) {
	packages := []string{"systemd-udev", "my-kernel-mod", "htop"}

	filtered := filterEssentialPackages(packages)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 essential packages, got %d", len(filtered))
	}
	if filtered[0] != "systemd-udev" || filtered[1] != "my-kernel-mod" {
		t.Fatalf("unexpected filtered packages: %#v", filtered)
	}
}

func TestFilterEssentialPackagesFallback(t *testing.T) {
	packages := make([]string, 20)
	for i := range packages {
		packages[i] = "pkg" + string(rune('a'+i))
	}

	filtered := filterEssentialPackages(packages)

	if len(filtered) != 15 {
		t.Fatalf("expected fallback to first 15 packages, got %d", len(filtered))
	}
	if filtered[0] != "pkga" || filtered[14] != "pkgo" {
		t.Fatalf("unexpected fallback slice: %#v", filtered[:2])
	}
}

func TestExtractJSONStripsCodeFence(t *testing.T) {
	response := "```json\n{\"key\":\"value\"}\n```"

	got := extractJSON(response)

	if got != "{\"key\":\"value\"}" {
		t.Fatalf("expected JSON payload, got %q", got)
	}
}

func TestExtractJSONNoBracesReturnsInput(t *testing.T) {
	response := "no json content here"

	got := extractJSON(response)

	if got != response {
		t.Fatalf("expected original string when braces missing, got %q", got)
	}
}

func TestUniqueStringsRemovesDuplicates(t *testing.T) {
	input := []string{"alpha", "beta", "alpha", "gamma", "beta"}

	got := uniqueStrings(input)

	if len(got) != 3 {
		t.Fatalf("expected 3 unique elements, got %d", len(got))
	}
	if strings.Count(strings.Join(got, ","), "alpha") != 1 {
		t.Fatalf("expected 'alpha' once, got %v", got)
	}
}

func TestContainsMatchesExactString(t *testing.T) {
	items := []string{"minimal", "security"}

	if !contains(items, "minimal") {
		t.Fatalf("expected to find 'minimal'")
	}
	if contains(items, "performance") {
		t.Fatalf("did not expect to find 'performance'")
	}
}

func TestGenerateTemplateFromExamplesMergesData(t *testing.T) {
	webConfig := UseCaseConfig{
		Name:                "web",
		Description:         "Web server stack",
		EssentialPackages:   []string{"systemd", "openssl"},
		OptionalPackages:    []string{"curl"},
		PerformancePackages: []string{"varnish"},
	}

	agent := &AIAgent{
		useCases: &UseCasesConfig{
			UseCases: map[string]UseCaseConfig{
				"web": webConfig,
			},
		},
	}
	intent := &TemplateIntent{
		UseCase:      "web",
		Requirements: []string{"performance"},
		Architecture: "x86_64",
		Distribution: "azl3",
		ImageType:    "raw",
		CustomPackages: []string{
			"htop",
		},
		PackageRepositories: []RepoIntent{{
			Codename: "main",
			URL:      "https://example.repo",
			PKey:     "https://example.repo/key",
		}},
	}

	examples := []*SearchResult{
		{
			Template: &TemplateExample{
				UseCase:      "web",
				Distribution: "azl3",
				Packages:     []string{"nginx", "openssl"},
				HasDisk:      true,
				KernelInfo:   "6.6",
			},
			Score: 0.9,
		},
		{
			Template: &TemplateExample{
				Packages: []string{"openssl", "curl"},
			},
			Score: 0.8,
		},
	}

	primaryMatch := &UseCaseMatch{
		Name:   "web",
		Score:  0.9,
		Config: &webConfig,
	}

	tmpl, err := agent.generateTemplateFromExamples(intent, examples, primaryMatch)
	if err != nil {
		t.Fatalf("generateTemplateFromExamples returned error: %v", err)
	}

	if tmpl.Image.Name != "azl3-x86_64-web" {
		t.Fatalf("expected image name 'azl3-x86_64-web', got %q", tmpl.Image.Name)
	}
	if tmpl.Target.OS != "azure-linux" {
		t.Fatalf("expected target OS 'azure-linux', got %q", tmpl.Target.OS)
	}
	if tmpl.SystemConfig.Kernel.Version != "6.6" {
		t.Fatalf("expected kernel version '6.6', got %q", tmpl.SystemConfig.Kernel.Version)
	}
	if tmpl.SystemConfig.Immutability == nil || tmpl.SystemConfig.Immutability.Enabled {
		t.Fatalf("expected immutability to be disabled but present")
	}

	packages := tmpl.SystemConfig.Packages
	if len(packages) != 6 {
		t.Fatalf("expected 6 packages with curated additions, got %d", len(packages))
	}
	if !contains(packages, "systemd") {
		t.Fatalf("expected curated package 'systemd', got %v", packages)
	}
	if !contains(packages, "varnish") {
		t.Fatalf("expected performance package from use case, got %v", packages)
	}
	if !contains(packages, "htop") {
		t.Fatalf("expected custom package 'htop' to be included, got %#v", packages)
	}

	if tmpl.Disk == nil || tmpl.Disk.Size != "8GiB" {
		t.Fatalf("expected disk size '8GiB', got %#v", tmpl.Disk)
	}
	if len(tmpl.PackageRepositories) != 1 || tmpl.PackageRepositories[0].Codename != "main" {
		t.Fatalf("expected package repository to be copied, got %#v", tmpl.PackageRepositories)
	}
}

func TestGenerateTemplateFromExamplesMinimalRequirement(t *testing.T) {
	minimalConfig := UseCaseConfig{
		Name:              "minimal",
		Description:       "Minimal footprint",
		EssentialPackages: []string{"systemd-networkd", "kernel-core"},
	}

	agent := &AIAgent{
		useCases: &UseCasesConfig{
			UseCases: map[string]UseCaseConfig{
				"minimal": minimalConfig,
			},
		},
	}
	intent := &TemplateIntent{
		UseCase:      "minimal",
		Requirements: []string{"minimal"},
		Architecture: "x86_64",
		Distribution: "azl3",
		ImageType:    "raw",
	}

	heavyPackages := make([]string, 25)
	for i := range heavyPackages {
		heavyPackages[i] = "pkg" + string(rune('a'+i))
	}
	heavyPackages = append(heavyPackages, "systemd-networkd", "kernel-core")

	examples := []*SearchResult{
		{
			Template: &TemplateExample{
				Distribution: "azl3",
				Packages:     heavyPackages,
				HasDisk:      true,
				KernelInfo:   "6.8",
			},
			Score: 0.95,
		},
	}

	primaryMatch := &UseCaseMatch{
		Name:   "minimal",
		Score:  0.85,
		Config: &minimalConfig,
	}

	tmpl, err := agent.generateTemplateFromExamples(intent, examples, primaryMatch)
	if err != nil {
		t.Fatalf("generateTemplateFromExamples returned error: %v", err)
	}

	if len(tmpl.SystemConfig.Packages) >= len(heavyPackages) {
		t.Fatalf("expected package list to be reduced for minimal requirement")
	}
	var hasSystemd, hasKernel bool
	for _, pkg := range tmpl.SystemConfig.Packages {
		if strings.Contains(pkg, "systemd") {
			hasSystemd = true
		}
		if strings.Contains(pkg, "kernel") {
			hasKernel = true
		}
	}
	if !hasSystemd || !hasKernel {
		t.Fatalf("expected essential packages to remain, got %v", tmpl.SystemConfig.Packages)
	}
	if tmpl.SystemConfig.Immutability != nil {
		t.Fatalf("expected immutability omitted for minimal requirement")
	}
}

func TestGenerateTemplateFromExamplesSupportsQcow2Artifact(t *testing.T) {
	cloudConfig := UseCaseConfig{
		Name: "cloud",
		Disk: UseCaseDisk{DefaultSize: "20GiB"},
	}

	agent := &AIAgent{
		useCases: &UseCasesConfig{UseCases: map[string]UseCaseConfig{
			"cloud": cloudConfig,
		}},
	}

	intent := cleanIntent(&TemplateIntent{
		UseCase:      "cloud",
		Requirements: nil,
		Architecture: "x86_64",
		Distribution: "elxr12",
		ImageType:    "qcow2",
	})

	examples := []*SearchResult{
		{
			Template: &TemplateExample{
				UseCase:      "cloud",
				Distribution: "elxr12",
				Packages:     []string{"base-cloud"},
				HasDisk:      false,
				KernelInfo:   "6.1",
			},
			Score: 0.8,
		},
	}

	primaryMatch := &UseCaseMatch{Name: "cloud", Score: 0.9, Config: &cloudConfig}

	tmpl, err := agent.generateTemplateFromExamples(intent, examples, primaryMatch)
	if err != nil {
		t.Fatalf("generateTemplateFromExamples returned error: %v", err)
	}

	if tmpl.Target.ImageType != "raw" {
		t.Fatalf("expected target image type to fallback to 'raw', got %q", tmpl.Target.ImageType)
	}
	if tmpl.Disk == nil {
		t.Fatalf("expected disk configuration to be generated")
	}
	if tmpl.Disk.Size != "20GiB" {
		t.Fatalf("expected disk size from curated config, got %q", tmpl.Disk.Size)
	}
	if len(tmpl.Disk.Artifacts) != 1 || tmpl.Disk.Artifacts[0].Type != "qcow2" {
		t.Fatalf("expected qcow2 artifact, got %#v", tmpl.Disk.Artifacts)
	}
	if tmpl.Disk.Artifacts[0].Compression != "" {
		t.Fatalf("expected no compression for qcow2, got %q", tmpl.Disk.Artifacts[0].Compression)
	}
}

func TestMapDistToOS(t *testing.T) {
	if got := mapDistToOS("azl3"); got != "azure-linux" {
		t.Fatalf("expected 'azure-linux', got %q", got)
	}
	if got := mapDistToOS("emt3"); got != "edge-microvisor-toolkit" {
		t.Fatalf("expected 'edge-microvisor-toolkit', got %q", got)
	}
	if got := mapDistToOS("unknown"); got != "azure-linux" {
		t.Fatalf("expected fallback 'azure-linux', got %q", got)
	}
}

func TestBuildSystemPromptWithRAGListsUseCases(t *testing.T) {
	rag := &TemplateRAG{templatesByUse: map[string][]*TemplateExample{
		"edge":       nil,
		"web-server": nil,
	}}

	prompt := buildSystemPromptWithRAG(rag, nil)

	if !strings.Contains(prompt, "edge, web-server") {
		t.Fatalf("expected prompt to include use cases, got %q", prompt)
	}
	if !strings.Contains(prompt, "Return ONLY valid JSON") {
		t.Fatalf("expected JSON instructions in prompt")
	}
}

type stubChatModel struct {
	response        TemplateIntent
	structuredCalls int
	lastPrompt      string
}

func (s *stubChatModel) SendMessage(ctx context.Context, message string) (string, error) {
	return "{}", nil
}

func (s *stubChatModel) SendStructuredMessage(ctx context.Context, message string, schema interface{}) error {
	s.structuredCalls++
	s.lastPrompt = message

	if intent, ok := schema.(*TemplateIntent); ok {
		*intent = s.response
	}
	return nil
}

func (s *stubChatModel) SetSystemPrompt(prompt string) {
	s.lastPrompt = prompt
}

func (s *stubChatModel) ResetConversation() {}

type staticEmbedding struct {
	vector []float64
}

func (s *staticEmbedding) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	return append([]float64(nil), s.vector...), nil
}

func (s *staticEmbedding) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = append([]float64(nil), s.vector...)
	}
	return out, nil
}

func TestProcessUserRequestGeneratesTemplate(t *testing.T) {
	t.Parallel()

	chat := &stubChatModel{
		response: TemplateIntent{
			UseCase:      "edge",
			Requirements: []string{"security"},
			Architecture: "x86_64",
			Distribution: "azl3",
			ImageType:    "raw",
			Description:  "Edge image",
			CustomPackages: []string{
				"custom-agent",
			},
			PackageRepositories: []RepoIntent{{
				Codename: "custom",
				URL:      "https://repo.example",
				PKey:     "https://repo.example/key",
			}},
		},
	}

	example := &TemplateExample{
		Name:         "edge-template.yml",
		UseCase:      "edge",
		Description:  "Edge template",
		Distribution: "azl3",
		Architecture: "x86_64",
		ImageType:    "raw",
		Packages:     []string{"nginx", "openssl"},
		HasDisk:      true,
		KernelInfo:   "6.9",
		Embedding:    []float64{1.0, 0.5, 0.25},
	}

	edgeConfig := UseCaseConfig{
		Name:              "edge",
		Description:       "Edge compute node",
		EssentialPackages: []string{"systemd"},
		SecurityPackages:  []string{"selinux-policy"},
		Disk: UseCaseDisk{
			DefaultSize: "12GiB",
		},
	}

	useCases := &UseCasesConfig{UseCases: map[string]UseCaseConfig{
		"edge": edgeConfig,
	}}

	useCaseRAG := &UseCaseRAG{
		useCases:        useCases,
		embeddingClient: &staticEmbedding{vector: []float64{1.0, 0.5, 0.25}},
		entries: map[string]*useCaseEntry{
			"edge": {
				Name:      "edge",
				Config:    edgeConfig,
				Embedding: []float64{1.0, 0.5, 0.25},
				Context:   buildUseCaseContextText("edge", edgeConfig),
			},
		},
	}

	agent := &AIAgent{
		chatModel: chat,
		rag: &TemplateRAG{
			templates: map[string]*TemplateExample{
				example.Name: example,
			},
			templatesByUse: map[string][]*TemplateExample{
				"edge": {example},
			},
			embeddingClient: &staticEmbedding{vector: []float64{1.0, 0.5, 0.25}},
		},
		useCaseRAG: useCaseRAG,
		useCases:   useCases,
	}

	tmpl, err := agent.ProcessUserRequest(context.Background(), "edge compute node")
	if err != nil {
		t.Fatalf("ProcessUserRequest returned error: %v", err)
	}

	if tmpl.Image.Name != "azl3-x86_64-edge" {
		t.Fatalf("expected image name 'azl3-x86_64-edge', got %q", tmpl.Image.Name)
	}
	if tmpl.SystemConfig.Description != "Edge image" {
		t.Fatalf("expected description to propagate, got %q", tmpl.SystemConfig.Description)
	}
	if !contains(tmpl.SystemConfig.Packages, "custom-agent") {
		t.Fatalf("expected custom package to be included, got %v", tmpl.SystemConfig.Packages)
	}
	if !contains(tmpl.SystemConfig.Packages, "systemd") {
		t.Fatalf("expected curated package 'systemd', got %v", tmpl.SystemConfig.Packages)
	}
	if len(tmpl.PackageRepositories) != 1 || tmpl.PackageRepositories[0].Codename != "custom" {
		t.Fatalf("expected custom repository, got %v", tmpl.PackageRepositories)
	}
	if tmpl.SystemConfig.Immutability == nil || tmpl.SystemConfig.Immutability.Enabled {
		t.Fatalf("expected immutability to be present but disabled")
	}
	if chat.structuredCalls != 1 {
		t.Fatalf("expected a single structured prompt, got %d", chat.structuredCalls)
	}
}

func TestBuildExamplesContextIncludesTemplateDetails(t *testing.T) {
	t.Parallel()

	agent := &AIAgent{}
	contextStr := agent.buildExamplesContext([]*SearchResult{
		{
			Template: &TemplateExample{
				Name:     "web-server.yml",
				UseCase:  "web-server",
				Packages: []string{"nginx"},
			},
			Score: 0.87,
		},
	})

	if !strings.Contains(contextStr, "Example 1") {
		t.Fatalf("expected context to label examples, got %q", contextStr)
	}
	if !strings.Contains(contextStr, "nginx") {
		t.Fatalf("expected package listing in context, got %q", contextStr)
	}
}

func TestMinAndMaxHelpers(t *testing.T) {
	t.Parallel()

	if min(2, 5) != 2 {
		t.Fatalf("expected min(2,5) = 2")
	}
	if min(10, 3) != 3 {
		t.Fatalf("expected min(10,3) = 3")
	}
	if max(2, 5) != 5 {
		t.Fatalf("expected max(2,5) = 5")
	}
	if max(10, 3) != 10 {
		t.Fatalf("expected max(10,3) = 10")
	}
}
