package rpmutils

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/ospackage"
	"github.com/open-edge-platform/image-composer/internal/ospackage/resolvertest"
)

func TestRPMResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"rpmutils",
		ResolvePackageInfos, // directly passing your function
	)
}

func TestExtractBaseRequirement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple requirement",
			input:    "bash",
			expected: "bash",
		},
		{
			name:     "requirement with version",
			input:    "glibc >= 2.30",
			expected: "glibc",
		},
		{
			name:     "complex requirement with parentheses",
			input:    "(libssl.so.1.1 >= 1.1.0)",
			expected: "libssl.so.1.1",
		},
		{
			name:     "requirement with 64bit suffix",
			input:    "libpthread.so.0()(64bit)",
			expected: "libpthread.so.0",
		},
		{
			name:     "complex requirement with multiple conditions",
			input:    "(gcc-c++ and make)",
			expected: "gcc-c++",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "requirement with complex versioning",
			input:    "python3-devel >= 3.8.0",
			expected: "python3-devel",
		},
		{
			name:     "parentheses with spaces",
			input:    "( openssl-libs )",
			expected: "openssl-libs",
		},
		{
			name:     "file path requirement",
			input:    "/bin/sh",
			expected: "/bin/sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBaseRequirement(tt.input)
			if result != tt.expected {
				t.Errorf("extractBaseRequirement(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateDot(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "dot_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		packages    []ospackage.PackageInfo
		filename    string
		expectError bool
	}{
		{
			name: "simple package with dependencies",
			packages: []ospackage.PackageInfo{
				{
					Name:     "bash",
					Requires: []string{"glibc", "ncurses"},
				},
				{
					Name:     "glibc",
					Requires: []string{},
				},
				{
					Name:     "ncurses",
					Requires: []string{"glibc"},
				},
			},
			filename:    filepath.Join(tmpDir, "simple.dot"),
			expectError: false,
		},
		{
			name:        "empty package list",
			packages:    []ospackage.PackageInfo{},
			filename:    filepath.Join(tmpDir, "empty.dot"),
			expectError: false,
		},
		{
			name: "package with no dependencies",
			packages: []ospackage.PackageInfo{
				{
					Name:     "standalone",
					Requires: []string{},
				},
			},
			filename:    filepath.Join(tmpDir, "standalone.dot"),
			expectError: false,
		},
		{
			name: "packages with special characters in names",
			packages: []ospackage.PackageInfo{
				{
					Name:     "package-with-dashes",
					Requires: []string{"lib.so.1"},
				},
				{
					Name:     "lib.so.1",
					Requires: []string{},
				},
			},
			filename:    filepath.Join(tmpDir, "special_chars.dot"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GenerateDot(tt.packages, tt.filename)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify the file was created and has expected content
			content, err := os.ReadFile(tt.filename)
			if err != nil {
				t.Fatalf("Failed to read generated file: %v", err)
			}

			contentStr := string(content)

			// Check basic DOT structure
			if !strings.Contains(contentStr, "digraph G {") {
				t.Error("DOT file should start with 'digraph G {'")
			}
			if !strings.Contains(contentStr, "rankdir=LR;") {
				t.Error("DOT file should contain 'rankdir=LR;'")
			}
			if !strings.Contains(contentStr, "}") {
				t.Error("DOT file should end with '}'")
			}

			// Check that all packages are represented
			for _, pkg := range tt.packages {
				expectedNode := fmt.Sprintf("\"%s\" [label=\"%s\"];", pkg.Name, pkg.Name)
				if !strings.Contains(contentStr, expectedNode) {
					t.Errorf("DOT file should contain node definition: %s", expectedNode)
				}

				// Check dependencies
				for _, dep := range pkg.Requires {
					expectedEdge := fmt.Sprintf("\"%s\" -> \"%s\";", pkg.Name, dep)
					if !strings.Contains(contentStr, expectedEdge) {
						t.Errorf("DOT file should contain edge: %s", expectedEdge)
					}
				}
			}
		})
	}
}

func TestParsePrimary(t *testing.T) {
	tests := []struct {
		name          string
		xmlContent    string
		filename      string
		expectedError bool
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "simple gzipped XML",
			xmlContent:    `<?xml version="1.0" encoding="UTF-8"?><metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="2"><package type="rpm"><name>bash</name><arch>x86_64</arch><location href="bash-5.1-8.el9.x86_64.rpm"/><format><rpm:license>GPLv3+</rpm:license><rpm:vendor>Red Hat, Inc.</rpm:vendor><rpm:provides><rpm:entry name="bash"/></rpm:provides><rpm:requires><rpm:entry name="glibc"/></rpm:requires></format></package><package type="rpm"><name>glibc</name><arch>x86_64</arch><location href="glibc-2.32-1.el9.x86_64.rpm"/><format><rpm:license>LGPLv2+</rpm:license><rpm:vendor>Red Hat, Inc.</rpm:vendor><rpm:provides><rpm:entry name="glibc"/></rpm:provides></format></package></metadata>`,
			filename:      "primary.xml.gz",
			expectedError: false,
			expectedCount: 2,
			expectedNames: []string{"bash-5.1-8.el9.x86_64.rpm", "glibc-2.32-1.el9.x86_64.rpm"},
		},
		{
			name:          "empty metadata",
			xmlContent:    `<?xml version="1.0" encoding="UTF-8"?><metadata xmlns="http://linux.duke.edu/metadata/common" packages="0"></metadata>`,
			filename:      "empty.xml.gz",
			expectedError: false,
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name:          "invalid compression",
			xmlContent:    "dummy content",
			filename:      "primary.xml.bz2",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that serves the XML content
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Set appropriate content type and serve compressed content
				if strings.HasSuffix(tt.filename, ".gz") {
					w.Header().Set("Content-Type", "application/gzip")
					// Compress the content properly
					content := compressGzip(t, tt.xmlContent)
					_, _ = w.Write(content)
				} else {
					w.Header().Set("Content-Type", "text/xml")
					_, _ = w.Write([]byte(tt.xmlContent))
				}
			}))
			defer server.Close()

			// Test ParsePrimary
			packages, err := ParsePrimary(server.URL+"/", tt.filename)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(packages) != tt.expectedCount {
				t.Errorf("Expected %d packages, got %d", tt.expectedCount, len(packages))
			}

			// Check that expected packages are present
			foundNames := make(map[string]bool)
			for _, pkg := range packages {
				foundNames[pkg.Name] = true

				// Verify the package has the expected fields
				if pkg.Type != "rpm" {
					t.Errorf("Package %s should have type 'rpm', got %s", pkg.Name, pkg.Type)
				}
				if pkg.URL == "" {
					t.Errorf("Package %s should have URL set", pkg.Name)
				}
			}

			for _, expectedName := range tt.expectedNames {
				if !foundNames[expectedName] {
					t.Errorf("Expected package %s not found", expectedName)
				}
			}

			// Additional checks for the first test case
			if tt.name == "simple gzipped XML" && len(packages) >= 2 {
				// Check bash package details
				var bashPkg *ospackage.PackageInfo
				for _, pkg := range packages {
					if pkg.Name == "bash-5.1-8.el9.x86_64.rpm" {
						bashPkg = &pkg
						break
					}
				}
				if bashPkg == nil {
					t.Fatal("bash-5.1-8.el9.x86_64.rpm package not found")
				}

				if bashPkg.License != "GPLv3+" {
					t.Errorf("bash license should be 'GPLv3+', got %s", bashPkg.License)
				}
				if bashPkg.Origin != "Red Hat, Inc." {
					t.Errorf("bash origin should be 'Red Hat, Inc.', got %s", bashPkg.Origin)
				}
			}
		})
	}
}

// Helper function to compress content with gzip
func compressGzip(t *testing.T, content string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}
