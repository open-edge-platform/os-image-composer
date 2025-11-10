package aiagent

import (
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
	agent := &AIAgent{}
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

	tmpl, err := agent.generateTemplateFromExamples(intent, examples)
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
	if len(packages) != 4 {
		t.Fatalf("expected 4 packages, got %d", len(packages))
	}
	var opensslCount int
	for _, pkg := range packages {
		if pkg == "openssl" {
			opensslCount++
		}
	}
	if opensslCount != 1 {
		t.Fatalf("expected openssl to appear once, got %d", opensslCount)
	}
	if packages[len(packages)-1] != "htop" {
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
	agent := &AIAgent{}
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

	tmpl, err := agent.generateTemplateFromExamples(intent, examples)
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

	prompt := buildSystemPromptWithRAG(rag)

	if !strings.Contains(prompt, "edge, web-server") {
		t.Fatalf("expected prompt to include use cases, got %q", prompt)
	}
	if !strings.Contains(prompt, "Return ONLY valid JSON") {
		t.Fatalf("expected JSON instructions in prompt")
	}
}
