package debutils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage"
)

// TestPackages tests the Packages function
func TestPackages(t *testing.T) {
	// Save original values
	origRepoCfg := RepoCfg
	origGzHref := GzHref
	defer func() {
		RepoCfg = origRepoCfg
		GzHref = origGzHref
	}()

	tests := []struct {
		name          string
		setupRepo     func() error
		expectError   bool
		errorContains string
		expectedCount int
	}{
		{
			name: "empty package list URL",
			setupRepo: func() error {
				RepoCfg = RepoConfig{
					PkgList:     "",
					PkgPrefix:   "http://example.com",
					ReleaseFile: "http://example.com/Release",
					ReleaseSign: "http://example.com/Release.gpg",
					PbGPGKey:    "dummy-key",
					BuildPath:   "./test-build",
					Arch:        "amd64",
				}
				GzHref = ""
				return nil
			},
			expectError:   true,
			errorContains: "parsing default repo failed",
		},
		{
			name: "invalid URL format",
			setupRepo: func() error {
				RepoCfg = RepoConfig{
					PkgList:     "invalid-url",
					PkgPrefix:   "invalid-prefix",
					ReleaseFile: "invalid-release",
					ReleaseSign: "invalid-sign",
					PbGPGKey:    "dummy-key",
					BuildPath:   "./test-build",
					Arch:        "amd64",
				}
				GzHref = "invalid-gz-href"
				return nil
			},
			expectError:   true,
			errorContains: "parsing default repo failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setupRepo(); err != nil {
				t.Fatalf("Failed to setup test repo: %v", err)
			}

			packages, err := Packages()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(packages) != tt.expectedCount {
					t.Errorf("Expected %d packages, got %d", tt.expectedCount, len(packages))
				}
			}
		})
	}
}

// TestUserPackages tests the UserPackages function
func TestUserPackages(t *testing.T) {
	// Save original values
	origUserRepo := UserRepo
	origArch := Architecture
	defer func() {
		UserRepo = origUserRepo
		Architecture = origArch
	}()

	tests := []struct {
		name          string
		setupUserRepo func()
		expectError   bool
		errorContains string
	}{
		{
			name: "empty user repo",
			setupUserRepo: func() {
				UserRepo = nil
				Architecture = "amd64"
			},
			expectError: false,
		},
		{
			name: "invalid user repo URL - skips gracefully",
			setupUserRepo: func() {
				UserRepo = []config.PackageRepository{
					{
						URL:      "invalid-url",
						Codename: "stable",
						PKey:     "dummy-key",
					},
				}
				Architecture = "amd64"
			},
			expectError: false, // The function skips invalid repos and continues
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupUserRepo()

			packages, err := UserPackages()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				// packages can be nil or empty slice, both are acceptable for empty repos
				_ = packages // Use the variable to avoid "declared and not used" error
			}
		})
	}
}

// TestCheckFileExists tests the checkFileExists function
func TestCheckFileExists(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/exists":
			w.WriteHeader(http.StatusOK)
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/servererror":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "file exists",
			url:      server.URL + "/exists",
			expected: true,
		},
		{
			name:     "file not found",
			url:      server.URL + "/notfound",
			expected: false,
		},
		{
			name:     "server error",
			url:      server.URL + "/servererror",
			expected: false,
		},
		{
			name:     "invalid URL",
			url:      "invalid-url",
			expected: false,
		},
		{
			name:     "empty URL",
			url:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkFileExists(tt.url)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestValidate tests the Validate function
func TestValidate(t *testing.T) {
	// Save original PkgChecksum
	origPkgChecksum := PkgChecksum
	defer func() {
		PkgChecksum = origPkgChecksum
	}()

	tests := []struct {
		name          string
		setupFiles    func(tempDir string) error
		setupChecksum func()
		expectError   bool
		errorContains string
	}{
		{
			name: "no DEB files",
			setupFiles: func(tempDir string) error {
				// Create an empty directory
				return nil
			},
			setupChecksum: func() {
				PkgChecksum = nil
			},
			expectError: false,
		},
		{
			name: "valid DEB file",
			setupFiles: func(tempDir string) error {
				// Create a test DEB file
				debPath := filepath.Join(tempDir, "test.deb")
				content := []byte("test deb content")
				return os.WriteFile(debPath, content, 0644)
			},
			setupChecksum: func() {
				// This will fail because we don't have the actual checksum
				PkgChecksum = []pkgChecksum{
					{
						Name:     "test.deb",
						Checksum: "wrongchecksum",
					},
				}
			},
			expectError:   true,
			errorContains: "failed verification",
		},
		{
			name: "DEB file without checksum",
			setupFiles: func(tempDir string) error {
				debPath := filepath.Join(tempDir, "missing.deb")
				content := []byte("missing deb content")
				return os.WriteFile(debPath, content, 0644)
			},
			setupChecksum: func() {
				PkgChecksum = []pkgChecksum{
					{
						Name:     "other.deb",
						Checksum: "somechecksum",
					},
				}
			},
			expectError:   true,
			errorContains: "failed verification",
		},
		{
			name: "invalid directory",
			setupFiles: func(tempDir string) error {
				// Don't create the directory
				return nil
			},
			setupChecksum: func() {
				PkgChecksum = nil
			},
			expectError: false, // Glob on non-existent directory returns empty, which is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "validate_test")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tempDir)

			if err := tt.setupFiles(tempDir); err != nil {
				t.Fatalf("Failed to setup test files: %v", err)
			}

			tt.setupChecksum()

			files, err := filepath.Glob(filepath.Join(tempDir, "*.deb"))
			if err != nil {
				t.Fatalf("Failed to glob deb files: %v", err)
			}
			var debFiles []string
			for _, f := range files {
				debFiles = append(debFiles, filepath.Base(f))
			}

			err = Validate(tempDir, debFiles)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestResolve tests the Resolve function
func TestResolve(t *testing.T) {
	// Save original PkgChecksum
	origPkgChecksum := PkgChecksum
	defer func() {
		PkgChecksum = origPkgChecksum
	}()

	tests := []struct {
		name          string
		req           []ospackage.PackageInfo
		all           []ospackage.PackageInfo
		expectError   bool
		errorContains string
	}{
		{
			name: "empty request",
			req:  []ospackage.PackageInfo{},
			all: []ospackage.PackageInfo{
				{
					Name: "test-pkg",
					URL:  "http://example.com/test-pkg.deb",
					Checksums: []ospackage.Checksum{
						{Algorithm: "SHA256", Value: "abc123"},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset PkgChecksum for each test
			PkgChecksum = []pkgChecksum{}

			resolved, err := Resolve(tt.req, tt.all)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if resolved == nil {
					t.Error("Expected resolved packages to be non-nil")
				}
				// Verify that PkgChecksum was populated
				if len(PkgChecksum) != len(tt.all) {
					t.Errorf("Expected PkgChecksum length %d, got %d", len(tt.all), len(PkgChecksum))
				}
			}
		})
	}
}

// TestMatchRequested tests the MatchRequested function
func TestMatchRequested(t *testing.T) {
	tests := []struct {
		name          string
		requests      []string
		all           []ospackage.PackageInfo
		expectError   bool
		errorContains string
		expectedCount int
	}{
		{
			name:     "empty request",
			requests: []string{},
			all: []ospackage.PackageInfo{
				{Name: "pkg1", URL: "http://example.com/pkg1.deb"},
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:     "found package",
			requests: []string{"pkg1"},
			all: []ospackage.PackageInfo{
				{Name: "pkg1", URL: "http://example.com/pkg1.deb"},
				{Name: "pkg2", URL: "http://example.com/pkg2.deb"},
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:     "missing package",
			requests: []string{"nonexistent"},
			all: []ospackage.PackageInfo{
				{Name: "pkg1", URL: "http://example.com/pkg1.deb"},
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:     "mixed found and missing",
			requests: []string{"pkg1", "nonexistent"},
			all: []ospackage.PackageInfo{
				{Name: "pkg1", URL: "http://example.com/pkg1.deb"},
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := MatchRequested(tt.requests, tt.all)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(matched) != tt.expectedCount {
					t.Errorf("Expected %d matched packages, got %d", tt.expectedCount, len(matched))
				}
			}
		})
	}
}

// TestWriteArrayToFile tests the WriteArrayToFile function
func TestWriteArrayToFile(t *testing.T) {
	// Save original ReportPath
	origReportPath := ReportPath
	defer func() {
		ReportPath = origReportPath
	}()

	tests := []struct {
		name          string
		arr           []string
		title         string
		setupDir      func() (string, error)
		expectError   bool
		errorContains string
	}{
		{
			name:  "empty array",
			arr:   []string{},
			title: "Test Report",
			setupDir: func() (string, error) {
				tempDir, err := os.MkdirTemp("", "report_test")
				if err != nil {
					return "", err
				}
				ReportPath = tempDir
				return tempDir, nil
			},
			expectError: false,
		},
		{
			name:  "single item",
			arr:   []string{"missing-pkg"},
			title: "Missing Packages",
			setupDir: func() (string, error) {
				tempDir, err := os.MkdirTemp("", "report_test")
				if err != nil {
					return "", err
				}
				ReportPath = tempDir
				return tempDir, nil
			},
			expectError: false,
		},
		{
			name:  "multiple items",
			arr:   []string{"pkg1", "pkg2", "pkg3"},
			title: "Test Items",
			setupDir: func() (string, error) {
				tempDir, err := os.MkdirTemp("", "report_test")
				if err != nil {
					return "", err
				}
				ReportPath = tempDir
				return tempDir, nil
			},
			expectError: false,
		},
		{
			name:  "title with spaces",
			arr:   []string{"test"},
			title: "Title With Spaces",
			setupDir: func() (string, error) {
				tempDir, err := os.MkdirTemp("", "report_test")
				if err != nil {
					return "", err
				}
				ReportPath = tempDir
				return tempDir, nil
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := tt.setupDir()
			if err != nil {
				t.Fatalf("Failed to setup test directory: %v", err)
			}
			if tempDir != "" {
				defer os.RemoveAll(tempDir)
			}

			filename, err := WriteArrayToFile(tt.arr, tt.title)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && err != nil && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if filename == "" {
					t.Error("Expected non-empty filename")
				}

				// Verify the file was created
				if _, statErr := os.Stat(filename); os.IsNotExist(statErr) {
					t.Errorf("File %s was not created", filename)
				} else {
					// Verify the file content
					content, readErr := os.ReadFile(filename)
					if readErr != nil {
						t.Errorf("Failed to read created file: %v", readErr)
					} else {
						var report struct {
							ReportType string   `json:"report_type"`
							Missing    []string `json:"missing"`
						}
						if jsonErr := json.Unmarshal(content, &report); jsonErr != nil {
							t.Errorf("Failed to parse JSON: %v", jsonErr)
						} else {
							if report.ReportType != "missing_packages_report" {
								t.Errorf("Expected report_type 'missing_packages_report', got %s", report.ReportType)
							}
							if len(report.Missing) != len(tt.arr) {
								t.Errorf("Expected %d missing items, got %d", len(tt.arr), len(report.Missing))
							}
							for i, item := range tt.arr {
								if i < len(report.Missing) && report.Missing[i] != item {
									t.Errorf("Expected missing item %s, got %s", item, report.Missing[i])
								}
							}
						}
					}
				}

				// Verify filename format
				expectedPrefix := strings.ReplaceAll(tt.title, " ", "_")
				if !strings.Contains(filename, expectedPrefix) {
					t.Errorf("Expected filename to contain %s, got %s", expectedPrefix, filename)
				}
				if !strings.HasSuffix(filename, ".json") {
					t.Errorf("Expected filename to end with .json, got %s", filename)
				}
			}
		})
	}
}

// TestDownloadPackages tests the DownloadPackages function
func TestDownloadPackages(t *testing.T) {
	// Save original values
	origRepoCfg := RepoCfg
	origUserRepo := UserRepo
	origPkgChecksum := PkgChecksum
	origGzHref := GzHref
	defer func() {
		RepoCfg = origRepoCfg
		UserRepo = origUserRepo
		PkgChecksum = origPkgChecksum
		GzHref = origGzHref
	}()

	tests := []struct {
		name          string
		pkgList       []string
		setup         func() error
		expectError   bool
		errorContains string
	}{
		{
			name:    "empty package list",
			pkgList: []string{},
			setup: func() error {
				// Set up invalid configuration to trigger early error
				RepoCfg = RepoConfig{}
				UserRepo = nil
				GzHref = ""
				return nil
			},
			expectError:   true,
			errorContains: "getting packages",
		},
		{
			name:    "invalid configuration",
			pkgList: []string{"test-pkg"},
			setup: func() error {
				RepoCfg = RepoConfig{
					PkgList: "invalid-url",
				}
				UserRepo = nil
				GzHref = "invalid-gz"
				return nil
			},
			expectError:   true,
			errorContains: "getting packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "download_test")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tempDir)

			if err := tt.setup(); err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			downloadList, err := DownloadPackages(tt.pkgList, tempDir, "")

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if downloadList == nil {
					t.Error("Expected download list to be non-nil")
				}
			}
		})
	}
}

// TestRepoConfig tests the RepoConfig struct
func TestRepoConfig(t *testing.T) {
	config := RepoConfig{
		Section:      "[main]",
		Name:         "Main Repository",
		PkgList:      "http://example.com/packages.gz",
		PkgPrefix:    "http://example.com",
		GPGCheck:     true,
		RepoGPGCheck: true,
		Enabled:      true,
		PbGPGKey:     "test-key",
		ReleaseFile:  "http://example.com/Release",
		ReleaseSign:  "http://example.com/Release.gpg",
		BuildPath:    "./builds/main",
		Arch:         "amd64",
	}

	// Verify all fields are set correctly
	if config.Section != "[main]" {
		t.Errorf("Expected Section '[main]', got %s", config.Section)
	}
	if config.Name != "Main Repository" {
		t.Errorf("Expected Name 'Main Repository', got %s", config.Name)
	}
	if config.PkgList != "http://example.com/packages.gz" {
		t.Errorf("Expected PkgList 'http://example.com/packages.gz', got %s", config.PkgList)
	}
	if !config.GPGCheck {
		t.Error("Expected GPGCheck to be true")
	}
	if !config.RepoGPGCheck {
		t.Error("Expected RepoGPGCheck to be true")
	}
	if !config.Enabled {
		t.Error("Expected Enabled to be true")
	}
}

// TestPkgChecksum tests the pkgChecksum struct
func TestPkgChecksum(t *testing.T) {
	checksum := pkgChecksum{
		Name:     "test-package.deb",
		Checksum: "abc123def456",
	}

	if checksum.Name != "test-package.deb" {
		t.Errorf("Expected Name 'test-package.deb', got %s", checksum.Name)
	}
	if checksum.Checksum != "abc123def456" {
		t.Errorf("Expected Checksum 'abc123def456', got %s", checksum.Checksum)
	}
}

// TestGlobalVariables tests global variable initialization
func TestGlobalVariables(t *testing.T) {
	// Test that global variables can be modified
	origReportPath := ReportPath
	defer func() {
		ReportPath = origReportPath
	}()

	// Test default value
	if ReportPath != "builds" {
		t.Errorf("Expected default ReportPath 'builds', got %s", ReportPath)
	}

	// Test modification
	ReportPath = "custom-builds"
	if ReportPath != "custom-builds" {
		t.Errorf("Expected ReportPath 'custom-builds', got %s", ReportPath)
	}

	// Test that slices can be modified
	origPkgChecksum := PkgChecksum
	PkgChecksum = []pkgChecksum{
		{Name: "test1.deb", Checksum: "hash1"},
		{Name: "test2.deb", Checksum: "hash2"},
	}

	if len(PkgChecksum) != 2 {
		t.Errorf("Expected PkgChecksum length 2, got %d", len(PkgChecksum))
	}

	PkgChecksum = origPkgChecksum
}
