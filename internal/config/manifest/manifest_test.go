package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config/version"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
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
func TestFallbackToDefault(t *testing.T) {
	tests := []struct {
		val      string
		fallback string
		want     string
	}{
		{"", "fallback", "fallback"},
		{"value", "fallback", "value"},
	}
	for _, tt := range tests {
		got := fallbackToDefault(tt.val, tt.fallback)
		if got != tt.want {
			t.Errorf("fallbackToDefault(%q, %q) = %q; want %q", tt.val, tt.fallback, got, tt.want)
		}
	}
}

func TestGenerateDocumentNamespace(t *testing.T) {
	ns1 := generateDocumentNamespace()
	ns2 := generateDocumentNamespace()
	if ns1 == ns2 {
		t.Errorf("Expected different namespaces, got %q and %q", ns1, ns2)
	}
	if !strings.HasPrefix(ns1, SPDXNamespaceBase+"/") {
		t.Errorf("Namespace does not start with SPDXNamespaceBase: %q", ns1)
	}
}

func TestSpdxSupplier(t *testing.T) {
	tests := []struct {
		origin string
		want   string
	}{
		{"", "NOASSERTION"},
		{"Intel", "Organization: Intel"},
		{"John Doe <john@example.com>", "Person: John Doe (john@example.com)"},
		{"Acme Corp", "Organization: Acme Corp"},
		{"Jane <jane@corp.com>", "Person: Jane (jane@corp.com)"},
		{"  ", "NOASSERTION"},
	}
	for _, tt := range tests {
		got := spdxSupplier(tt.origin)
		if got != tt.want {
			t.Errorf("spdxSupplier(%q) = %q; want %q", tt.origin, got, tt.want)
		}
	}
}

func TestWriteManifestToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "manifest.json")
	manifest := SoftwarePackageManifest{
		SchemaVersion:     "1.0",
		ImageVersion:      "v1.2.3",
		BuiltAt:           "2024-01-01T00:00:00Z",
		Arch:              "amd64",
		SizeBytes:         123456,
		Hash:              "deadbeef",
		HashAlg:           "sha256",
		Signature:         "sig",
		SigAlg:            "rsa",
		MinCurrentVersion: "v1.0.0",
	}
	err := WriteManifestToFile(manifest, outFile)
	if err != nil {
		t.Fatalf("WriteManifestToFile failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read manifest file: %v", err)
	}
	var got SoftwarePackageManifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}
	if got.ImageVersion != manifest.ImageVersion {
		t.Errorf("Expected ImageVersion %q, got %q", manifest.ImageVersion, got.ImageVersion)
	}
}

func TestWriteManifestToFile_InvalidPath(t *testing.T) {
	// Try to write to a directory that doesn't exist and can't be created
	// On Unix, /root/ is usually not writable by non-root users
	badPath := "/root/should_not_exist/manifest.json"
	manifest := SoftwarePackageManifest{}
	err := WriteManifestToFile(manifest, badPath)
	if err == nil {
		t.Errorf("Expected error when writing to unwritable path")
	}
}

func TestWriteSPDXToFile_InvalidChecksumAlgorithm(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "sbom.spdx.json")
	pkgs := []ospackage.PackageInfo{
		{
			Name:        "pkg",
			Type:        "deb",
			Version:     "1.0",
			URL:         "https://example.com/pkg.deb",
			Description: "desc",
			License:     "MIT",
			Origin:      "Org",
			Checksums: []ospackage.Checksum{
				{Algorithm: "sha512", Value: "notused"},
				{Algorithm: "sha256", Value: "used"},
			},
		},
	}
	err := WriteSPDXToFile(pkgs, outFile)
	if err != nil {
		t.Fatalf("WriteSPDXToFile failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read SPDX output: %v", err)
	}
	var doc SPDXDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(doc.Packages))
	}
	if len(doc.Packages[0].Checksum) != 1 {
		t.Errorf("Expected only 1 valid checksum, got %d", len(doc.Packages[0].Checksum))
	}
	if doc.Packages[0].Checksum[0].Algorithm != "SHA256" {
		t.Errorf("Expected SHA256 checksum, got %q", doc.Packages[0].Checksum[0].Algorithm)
	}
}

func TestWriteSPDXToFile_MissingFields(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "sbom.spdx.json")
	pkgs := []ospackage.PackageInfo{
		{
			Name: "empty",
		},
	}
	err := WriteSPDXToFile(pkgs, outFile)
	if err != nil {
		t.Fatalf("WriteSPDXToFile failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Failed to read SPDX output: %v", err)
	}
	var doc SPDXDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Failed to parse SPDX JSON: %v", err)
	}
	if len(doc.Packages) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(doc.Packages))
	}
	p := doc.Packages[0]
	if p.LicenseDeclared != "NOASSERTION" {
		t.Errorf("Expected LicenseDeclared to be NOASSERTION, got %q", p.LicenseDeclared)
	}
	if p.Supplier != "NOASSERTION" {
		t.Errorf("Expected Supplier to be NOASSERTION, got %q", p.Supplier)
	}
}
