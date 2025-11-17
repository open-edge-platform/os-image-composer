package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/aiagent"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"gopkg.in/yaml.v3"
)

type stubAgent struct {
	template *aiagent.OSImageTemplate
	err      error
	input    string
}

func (s *stubAgent) ProcessUserRequest(ctx context.Context, userInput string) (*aiagent.OSImageTemplate, error) {
	s.input = userInput
	if s.err != nil {
		return nil, s.err
	}
	return s.template, nil
}

func TestRunAIGeneration_OllamaSuccess(t *testing.T) {

	originalFactory := newAIAgent
	defer func() { newAIAgent = originalFactory }()

	stub := &stubAgent{
		template: &aiagent.OSImageTemplate{
			Image:  aiagent.ImageConfig{Name: "example", Version: "1.0.0"},
			Target: aiagent.TargetConfig{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
			SystemConfig: aiagent.SystemConfig{
				Name:     "example",
				Packages: []string{"openssh", "htop"},
			},
		},
	}

	var capturedProvider string
	var capturedDir string
	var capturedConfig aiagent.OllamaConfig
	var capturedOptions *aiagent.AgentOptions

	newAIAgent = func(provider string, cfg interface{}, templatesDir string, options *aiagent.AgentOptions) (aiAgent, error) {
		capturedProvider = provider
		capturedDir = templatesDir
		capturedOptions = options

		ollamaCfg, ok := cfg.(aiagent.OllamaConfig)
		if !ok {
			t.Fatalf("expected OllamaConfig, got %T", cfg)
		}
		capturedConfig = ollamaCfg
		return stub, nil
	}

	cfg := config.AIConfig{
		Enabled:                       true,
		Provider:                      "ollama",
		TemplatesDir:                  "./custom-templates",
		TemplateContributionThreshold: 0.7,
		UseCaseMatchThreshold:         0.8,
		Ollama: config.OllamaConfig{
			BaseURL:        "http://localhost:1234",
			Model:          "test-model",
			Temperature:    0.5,
			MaxTokens:      512,
			Timeout:        45,
			EmbeddingModel: "test-embed",
		},
	}

	outputPath := filepath.Join(t.TempDir(), "template.yml")

	err := runAIGeneration("create something", cfg, outputPath, nil)
	if err != nil {
		t.Fatalf("runAIGeneration returned error: %v", err)
	}

	if stub.input != "create something" {
		t.Fatalf("expected stub to receive prompt, got %q", stub.input)
	}

	if capturedProvider != "ollama" {
		t.Fatalf("expected provider 'ollama', got %q", capturedProvider)
	}

	if capturedDir != cfg.TemplatesDir {
		t.Fatalf("expected templates dir %q, got %q", cfg.TemplatesDir, capturedDir)
	}

	if capturedOptions == nil {
		t.Fatalf("expected options to be passed to agent factory")
	}
	if capturedOptions.TemplateContributionThreshold != cfg.TemplateContributionThreshold {
		t.Fatalf("expected template contribution threshold %.2f, got %.2f", cfg.TemplateContributionThreshold, capturedOptions.TemplateContributionThreshold)
	}
	if capturedOptions.UseCaseMatchThreshold != cfg.UseCaseMatchThreshold {
		t.Fatalf("expected use case match threshold %.2f, got %.2f", cfg.UseCaseMatchThreshold, capturedOptions.UseCaseMatchThreshold)
	}

	if capturedConfig.BaseURL != cfg.Ollama.BaseURL {
		t.Errorf("expected base URL %q, got %q", cfg.Ollama.BaseURL, capturedConfig.BaseURL)
	}
	if capturedConfig.Model != cfg.Ollama.Model {
		t.Errorf("expected model %q, got %q", cfg.Ollama.Model, capturedConfig.Model)
	}
	if capturedConfig.NumPredict != cfg.Ollama.MaxTokens {
		t.Errorf("expected num predict %d, got %d", cfg.Ollama.MaxTokens, capturedConfig.NumPredict)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var generated aiagent.OSImageTemplate
	if err := yaml.Unmarshal(data, &generated); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	if generated.Image.Name != stub.template.Image.Name {
		t.Errorf("expected image name %q, got %q", stub.template.Image.Name, generated.Image.Name)
	}
	if len(generated.SystemConfig.Packages) != len(stub.template.SystemConfig.Packages) {
		t.Errorf("expected %d packages, got %d", len(stub.template.SystemConfig.Packages), len(generated.SystemConfig.Packages))
	}
}

func TestRunAIGenerationIncludesAttachments(t *testing.T) {

	originalFactory := newAIAgent
	defer func() { newAIAgent = originalFactory }()

	stub := &stubAgent{
		template: &aiagent.OSImageTemplate{
			Image:  aiagent.ImageConfig{Name: "example", Version: "1.0.0"},
			Target: aiagent.TargetConfig{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
			SystemConfig: aiagent.SystemConfig{
				Name:     "example",
				Packages: []string{"pkg"},
			},
		},
	}

	newAIAgent = func(provider string, cfg interface{}, templatesDir string, options *aiagent.AgentOptions) (aiAgent, error) {
		return stub, nil
	}

	attachmentPath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(attachmentPath, []byte("details go here\nline two"), 0644); err != nil {
		t.Fatalf("failed to write attachment: %v", err)
	}

	cfg := config.AIConfig{Provider: "ollama"}

	if err := runAIGeneration("base prompt", cfg, "", []string{attachmentPath}); err != nil {
		t.Fatalf("runAIGeneration returned error: %v", err)
	}

	if !strings.Contains(stub.input, "base prompt") {
		t.Fatalf("expected prompt to contain original input, got %q", stub.input)
	}
	if !strings.Contains(stub.input, "ATTACHMENT CONTEXT:") {
		t.Fatalf("expected prompt to include attachment marker, got %q", stub.input)
	}
	if !strings.Contains(stub.input, "notes.txt") {
		t.Fatalf("expected prompt to include attachment name, got %q", stub.input)
	}
	if !strings.Contains(stub.input, "details go here") {
		t.Fatalf("expected prompt to include attachment content, got %q", stub.input)
	}
}

func TestRunAIGenerationAttachmentReadError(t *testing.T) {

	originalFactory := newAIAgent
	defer func() { newAIAgent = originalFactory }()

	called := false
	newAIAgent = func(provider string, cfg interface{}, templatesDir string, options *aiagent.AgentOptions) (aiAgent, error) {
		called = true
		return nil, fmt.Errorf("should not be called")
	}

	cfg := config.AIConfig{Provider: "ollama"}
	missingPath := filepath.Join(t.TempDir(), "missing.txt")

	err := runAIGeneration("prompt", cfg, "", []string{missingPath})
	if err == nil {
		t.Fatalf("expected error when attachment missing")
	}
	if !strings.Contains(err.Error(), "failed to read attachment") {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatalf("expected agent factory not to be invoked when attachment fails")
	}
}

func TestRunAIGeneration_ProcessError(t *testing.T) {

	originalFactory := newAIAgent
	defer func() { newAIAgent = originalFactory }()

	stub := &stubAgent{err: io.EOF}

	newAIAgent = func(provider string, cfg interface{}, templatesDir string, options *aiagent.AgentOptions) (aiAgent, error) {
		return stub, nil
	}

	cfg := config.AIConfig{Provider: "ollama"}

	err := runAIGeneration("fail please", cfg, "", nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to generate template") {
		t.Fatalf("expected generate template error, got %v", err)
	}
}

func TestRunAIGenerationUnsupportedProvider(t *testing.T) {

	cfg := config.AIConfig{Provider: "something-else"}

	err := runAIGeneration("anything", cfg, "", nil)
	if err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
}

func TestDisplayTemplateSummaryTruncatesPackages(t *testing.T) {
	t.Parallel()

	template := &aiagent.OSImageTemplate{
		Image:  aiagent.ImageConfig{Name: "test", Version: "1.0.0"},
		Target: aiagent.TargetConfig{OS: "azure-linux", Dist: "azl3", Arch: "x86_64", ImageType: "raw"},
		SystemConfig: aiagent.SystemConfig{
			Name: "summary",
			Packages: []string{
				"pkg1", "pkg2", "pkg3", "pkg4", "pkg5", "pkg6", "pkg7", "pkg8", "pkg9", "pkg10", "pkg11",
			},
			Kernel: &aiagent.KernelConfig{Version: "6.12"},
		},
		Disk: &aiagent.DiskConfig{Size: "8GiB"},
	}

	output := captureStdout(t, func() {
		displayTemplateSummary(template)
	})

	if !strings.Contains(output, "... and 1 more") {
		t.Fatalf("expected truncated packages message, got %s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = original

	data, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}

	return string(data)
}
