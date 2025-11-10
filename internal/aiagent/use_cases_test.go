package aiagent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUseCasesConfigHelpers(t *testing.T) {
	t.Parallel()

	cfg := &UseCasesConfig{
		UseCases: map[string]UseCaseConfig{
			"web-server": {
				Description:         "Web stack",
				Keywords:            []string{"web", "nginx"},
				EssentialPackages:   []string{"nginx", "httpd"},
				OptionalPackages:    []string{"certbot"},
				SecurityPackages:    []string{"selinux"},
				PerformancePackages: []string{"performance-tools"},
				Kernel: UseCaseKernel{
					DefaultVersion: "6.10",
					Cmdline:        "quiet",
				},
				Disk: UseCaseDisk{DefaultSize: "12GiB"},
			},
			"edge": {
				Description:       "Edge compute",
				Keywords:          []string{"edge", "iot"},
				EssentialPackages: []string{"edge-core"},
				OptionalPackages:  []string{"edge-extras"},
				Kernel:            UseCaseKernel{},
				Disk:              UseCaseDisk{},
			},
		},
	}

	useCase, err := cfg.GetUseCase("web-server")
	if err != nil || useCase.Description != "Web stack" {
		t.Fatalf("expected to retrieve web-server use case, got %+v err=%v", useCase, err)
	}

	if detected := cfg.DetectUseCase("Deploy secure web app with nginx"); detected != "web-server" {
		t.Fatalf("expected detector to select web-server, got %q", detected)
	}

	packages, err := cfg.GetPackagesForUseCase("web-server", []string{"security", "performance"})
	if err != nil {
		t.Fatalf("GetPackagesForUseCase returned error: %v", err)
	}
	expectedPackages := []string{"nginx", "httpd", "selinux", "performance-tools", "certbot"}
	if !reflect.DeepEqual(packages, expectedPackages) {
		t.Fatalf("unexpected packages: got %v want %v", packages, expectedPackages)
	}

	minimalPackages, err := cfg.GetPackagesForUseCase("web-server", []string{"minimal"})
	if err != nil || !reflect.DeepEqual(minimalPackages, []string{"nginx", "httpd"}) {
		t.Fatalf("expected minimal packages only, got %v err=%v", minimalPackages, err)
	}

	if version := cfg.GetKernelVersion("web-server", "azl3"); version != "6.12" {
		t.Fatalf("expected distribution override version 6.12, got %s", version)
	}
	if version := cfg.GetKernelVersion("web-server", "unknown"); version != "6.10" {
		t.Fatalf("expected use-case default kernel, got %s", version)
	}

	if cmdline := cfg.GetKernelCmdline("web-server"); cmdline != "quiet" {
		t.Fatalf("expected cmdline 'quiet', got %q", cmdline)
	}
	if size := cfg.GetDiskSize("web-server"); size != "12GiB" {
		t.Fatalf("expected disk size '12GiB', got %q", size)
	}

	if !cfg.HasUseCase("edge") {
		t.Fatalf("expected HasUseCase to report existing use case")
	}
	if cfg.HasUseCase("missing") {
		t.Fatalf("expected HasUseCase to return false for missing use case")
	}

	names := cfg.GetAllUseCaseNames()
	if len(names) != 2 {
		t.Fatalf("expected two use case names, got %v", names)
	}
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	if !nameSet["web-server"] || !nameSet["edge"] {
		t.Fatalf("expected use case names to include 'web-server' and 'edge', got %v", names)
	}
}

func TestLoadUseCasesReadsFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "use-cases.yml")
	content := `use_cases:
  web-server:
    description: "web"
    keywords: ["web"]
`
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp use cases file: %v", err)
	}

	cfg, err := LoadUseCases(filePath)
	if err != nil {
		t.Fatalf("LoadUseCases returned error: %v", err)
	}
	if len(cfg.UseCases) != 1 {
		t.Fatalf("expected one use case, got %d", len(cfg.UseCases))
	}
}

func TestLoadUseCasesMissingFile(t *testing.T) {
	t.Parallel()

	if _, err := LoadUseCases(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatalf("expected error when loading missing use cases file")
	}
}
