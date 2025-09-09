package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config/version"
	"github.com/open-edge-platform/image-composer/internal/ospackage"
)

func TestWriteSPDXToFile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create the output file path directly in tmpDir (no subdirectory)
	outFile := filepath.Join(tmpDir, "sbom.spdx.json")

	pkgs := []ospackage.PackageInfo{
		{
			Name:        "samplepkg",
			Type:        "rpm",
			Version:     "1.0.0",
			URL:         "https://openedgeplatform.com/samplepkg.rpm",
			Description: "Sample package",
			License:     "Apache-2.0",
			Origin:      "Intel",
			Checksums: []ospackage.Checksum{
				{Algorithm: "sha256", Value: "abcd1234abcd1234abcd1234"},
			},
		},
	}

	err := WriteSPDXToFile(pkgs, outFile)
	if err != nil {
		t.Fatalf("WriteSPDXToFile failed: %v", err)
	}

	// Verify file exists
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read SPDX output: %v", err)
	}

	// Unmarshal to validate structure
	var doc SPDXDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	if len(doc.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(doc.Packages))
	}

	p := doc.Packages[0]
	if p.Name != "samplepkg" {
		t.Errorf("Expected package name 'samplepkg', got %q", p.Name)
	}
	if p.Type != "rpm" {
		t.Errorf("Expected type 'rpm', got %q", p.Type)
	}
	if !strings.HasPrefix(doc.DocumentName, version.Toolname) {
		t.Errorf("Expected document name to start with tool name prefix, got %q", doc.DocumentName)
	}
	if len(p.Checksum) != 1 || p.Checksum[0].Algorithm != "SHA256" {
		t.Errorf("Expected SHA256 checksum, got %+v", p.Checksum)
	}
}

// Alternative test that creates subdirectories to match the original behavior
func TestWriteSPDXToFile_WithSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create output file path with subdirectory (like the original test)
	outFile := filepath.Join(tmpDir, "subdir", "sbom.spdx.json")

	pkgs := []ospackage.PackageInfo{
		{
			Name:        "testpkg",
			Type:        "deb",
			Version:     "2.0.0",
			URL:         "https://example.com/testpkg.deb",
			Description: "Test package with subdirectory",
			License:     "MIT",
			Origin:      "Test Organization",
			Checksums: []ospackage.Checksum{
				{Algorithm: "md5", Value: "d41d8cd98f00b204e9800998ecf8427e"},
			},
		},
	}

	err := WriteSPDXToFile(pkgs, outFile)
	if err != nil {
		t.Fatalf("WriteSPDXToFile with subdirectory failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("Output file was not created: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read SPDX output: %v", err)
	}

	var doc SPDXDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}

	if len(doc.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(doc.Packages))
	}

	p := doc.Packages[0]
	if p.Name != "testpkg" {
		t.Errorf("Expected package name 'testpkg', got %q", p.Name)
	}
}
