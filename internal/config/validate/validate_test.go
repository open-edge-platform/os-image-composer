package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

// loadFile reads a test file from the project root testdata directory.
func loadFile(t *testing.T, relPath string) []byte {
	t.Helper()
	// Determine project root relative to this test file
	root := filepath.Join("..") //, "..") //, "..", "..")
	fullPath := filepath.Join(root, relPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", fullPath, err)
	}
	return data
}

// Test new YAML image template format
func TestValidImageTemplate(t *testing.T) {
	v := loadFile(t, "../../image-templates/azl3-x86_64-edge-raw.yml")

	// Parse to generic JSON interface
	var raw interface{}
	if err := yaml.Unmarshal(v, &raw); err != nil {
		t.Errorf("yml parsing error: %v", err)
		return
	}

	// Re‐marshal to JSON bytes
	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Errorf("json marshaling error: %v", err)
		return
	}
	if err := ValidateImageTemplateJSON(dataJSON); err != nil {
		t.Errorf("expected image-templates/azl3-x86_64-edge-raw.yml to pass, but got: %v", err)
	}
}

func TestInvalidImageTemplate(t *testing.T) {
	v := loadFile(t, "/testdata/invalid-image.yml")

	// Parse to generic JSON interface
	var raw interface{}
	if err := yaml.Unmarshal(v, &raw); err != nil {
		t.Errorf("yml parsing error: %v", err)
		return
	}

	// Re‐marshal to JSON bytes
	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Errorf("json marshaling error: %v", err)
		return
	}

	if err := ValidateImageTemplateJSON(dataJSON); err == nil {
		t.Errorf("expected testdata/invalid-image.yml to fail validation")
	}
}

// Test merged template validation with the new single-object structure
func TestValidMergedTemplate(t *testing.T) {
	// Create a sample merged template in the new format
	mergedTemplateYAML := `image:
  name: test-merged-image
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

disk:
  name: Default
  size: 4GiB
  partitionTableType: gpt
  partitions:
    - id: boot
      type: esp
      flags:
        - esp
        - boot
      start: 1MiB
      end: 513MiB
      fsType: fat32
      mountPoint: /boot/efi
    - id: rootfs
      type: linux-root-amd64
      start: 513MiB
      end: "0"
      fsType: ext4
      mountPoint: /

systemConfig:
  name: default
  description: Default system configuration
  bootloader:
    bootType: efi
    provider: systemd-boot
  users:
    - name: admin
      sudo: true
      shell: /bin/bash
  packages:
    - filesystem
    - kernel
    - systemd
  kernel:
    version: "6.12"
    cmdline: "quiet splash"
`

	// Parse to generic JSON interface
	var raw interface{}
	if err := yaml.Unmarshal([]byte(mergedTemplateYAML), &raw); err != nil {
		t.Fatalf("yml parsing error: %v", err)
	}

	// Re-marshal to JSON bytes
	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("json marshaling error: %v", err)
	}

	if err := ValidateImageTemplateJSON(dataJSON); err != nil {
		t.Errorf("expected merged template to pass validation, but got: %v", err)
	}
}

func TestInvalidMergedTemplate(t *testing.T) {
	// Create an invalid merged template (missing required fields)
	invalidMergedTemplateYAML := `image:
  name: test-merged-image
  version: "1.0.0"

target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw

# Missing systemConfig which is required
`

	// Parse to generic JSON interface
	var raw interface{}
	if err := yaml.Unmarshal([]byte(invalidMergedTemplateYAML), &raw); err != nil {
		t.Fatalf("yml parsing error: %v", err)
	}

	// Re-marshal to JSON bytes
	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("json marshaling error: %v", err)
	}

	if err := ValidateImageTemplateJSON(dataJSON); err == nil {
		t.Errorf("expected invalid merged template to fail validation")
	}
}

// Test global config validation
func TestValidConfig(t *testing.T) {
	v := loadFile(t, "/testdata/valid-config.yml")

	if v == nil {
		t.Fatal("failed to load testdata/valid-config.yml")
	}
	dataJSON, err := yaml.YAMLToJSON(v)

	if err != nil {
		t.Fatalf("YAML→JSON conversion failed: %v", err)
	}
	if err := ValidateConfigJSON(dataJSON); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestInvalidConfig(t *testing.T) {
	v := loadFile(t, "/testdata/invalid-config.yml")

	// Parse to generic JSON interface
	var raw interface{}
	if err := yaml.Unmarshal(v, &raw); err != nil {
		t.Errorf("yml parsing error: %v", err)
		return
	}

	// Re‐marshal to JSON bytes
	dataJSON, err := yaml.YAMLToJSON(v)
	if err != nil {
		t.Errorf("json marshaling error: %v", err)
		return
	}

	if err := ValidateConfigJSON(dataJSON); err == nil {
		t.Errorf("expected invalid-config.json to fail validation: %v", err)
	}
}

// Test validation of template structure using external test files
func TestImageTemplateStructure(t *testing.T) {
	v := loadFile(t, "/testdata/complete-valid-template.yml")

	var raw interface{}
	if err := yaml.Unmarshal(v, &raw); err != nil {
		t.Fatalf("failed to parse minimal template: %v", err)
	}

	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	if err := ValidateImageTemplateJSON(dataJSON); err != nil {
		t.Errorf("minimal template should be valid, but got: %v", err)
	}
}

func TestImageTemplateMissingFields(t *testing.T) {
	v := loadFile(t, "/testdata/incomplete-template.yml")

	var raw interface{}
	if err := yaml.Unmarshal(v, &raw); err != nil {
		t.Fatalf("failed to parse invalid template: %v", err)
	}

	dataJSON, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	if err := ValidateImageTemplateJSON(dataJSON); err == nil {
		t.Errorf("incomplete template should fail validation")
	}
}

// Table-driven test for multiple template validation scenarios
func TestImageTemplateValidation(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		shouldPass  bool
		description string
	}{
		{
			name:        "ValidComplete",
			file:        "/testdata/complete-valid-template.yml",
			shouldPass:  true,
			description: "complete template with all optional fields",
		},
		{
			name:        "InvalidMissingImage",
			file:        "/testdata/missing-image-section.yml",
			shouldPass:  false,
			description: "template missing image section",
		},
		{
			name:        "InvalidMissingTarget",
			file:        "/testdata/missing-target-section.yml",
			shouldPass:  false,
			description: "template missing target section",
		},
		{
			name:        "InvalidWrongTypes",
			file:        "/testdata/wrong-field-types.yml",
			shouldPass:  false,
			description: "template with incorrect field types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := loadFile(t, tt.file)

			var raw interface{}
			if err := yaml.Unmarshal(v, &raw); err != nil {
				t.Fatalf("failed to parse template %s: %v", tt.file, err)
			}

			dataJSON, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("failed to marshal to JSON: %v", err)
			}

			err = ValidateImageTemplateJSON(dataJSON)
			if tt.shouldPass && err != nil {
				t.Errorf("expected %s to pass validation (%s), but got error: %v", tt.file, tt.description, err)
			} else if !tt.shouldPass && err == nil {
				t.Errorf("expected %s to fail validation (%s), but it passed", tt.file, tt.description)
			}
		})
	}
}

// Test merged template validation scenarios
func TestMergedTemplateValidation(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		shouldPass  bool
		description string
	}{
		{
			name: "ValidMinimalMerged",
			template: `image:
  name: test
  version: "1.0.0"
target:
  os: azure-linux
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: minimal
  packages:
    - filesystem
  kernel:
    version: "6.12"`,
			shouldPass:  true,
			description: "minimal valid merged template",
		},
		{
			name: "InvalidOSDistMismatch",
			template: `image:
  name: test
  version: "1.0.0"
target:
  os: azure-linux
  dist: emt3
  arch: x86_64
  imageType: raw
systemConfig:
  name: test
  packages:
    - filesystem
  kernel:
    version: "6.12"`,
			shouldPass:  false,
			description: "invalid OS/dist combination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw interface{}
			if err := yaml.Unmarshal([]byte(tt.template), &raw); err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			dataJSON, err := json.Marshal(raw)
			if err != nil {
				t.Fatalf("failed to marshal to JSON: %v", err)
			}

			err = ValidateImageTemplateJSON(dataJSON)
			if tt.shouldPass && err != nil {
				t.Errorf("expected %s to pass validation (%s), but got error: %v", tt.name, tt.description, err)
			} else if !tt.shouldPass && err == nil {
				t.Errorf("expected %s to fail validation (%s), but it passed", tt.name, tt.description)
			}
		})
	}
}
