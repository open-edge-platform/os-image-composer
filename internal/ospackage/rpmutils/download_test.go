package rpmutils_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/rpmutils"
)

func TestPackages(t *testing.T) {
	// Save original global variables
	originalRepoCfg := rpmutils.RepoCfg
	originalGzHref := rpmutils.GzHref
	defer func() {
		rpmutils.RepoCfg = originalRepoCfg
		rpmutils.GzHref = originalGzHref
	}()

	testCases := []struct {
		name        string
		repoURL     string
		gzHref      string
		expectError bool
	}{
		{
			name:        "empty repository URL",
			repoURL:     "",
			gzHref:      "repodata/primary.xml.gz",
			expectError: true,
		},
		{
			name:        "invalid URL format",
			repoURL:     "invalid-url",
			gzHref:      "repodata/primary.xml.gz",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rpmutils.RepoCfg = rpmutils.RepoConfig{
				URL: tc.repoURL,
			}
			rpmutils.GzHref = tc.gzHref

			packages, err := rpmutils.Packages()

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if packages == nil {
					t.Error("Expected packages to be non-nil")
				}
			}
		})
	}
}

func TestMatchRequested(t *testing.T) {
	testCases := []struct {
		name        string
		requests    []string
		all         []ospackage.PackageInfo
		expectError bool
		expectCount int
	}{
		{
			name:     "empty request",
			requests: []string{},
			all: []ospackage.PackageInfo{
				{Name: "test-package", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name:     "exact name match",
			requests: []string{"test-package"},
			all: []ospackage.PackageInfo{
				{Name: "test-package", Arch: "x86_64"},
				{Name: "other-package", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:     "exact name match with .rpm extension",
			requests: []string{"test-package"},
			all: []ospackage.PackageInfo{
				{Name: "test-package.rpm", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:     "version prefix match",
			requests: []string{"acl"},
			all: []ospackage.PackageInfo{
				{Name: "acl-2.3.1-2.el8", Arch: "x86_64"},
				{Name: "acl-dev", Arch: "x86_64"}, // Should not match - not a version
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:     "release prefix match",
			requests: []string{"package"},
			all: []ospackage.PackageInfo{
				{Name: "package-1.0.0", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:     "skip src packages",
			requests: []string{"test-package"},
			all: []ospackage.PackageInfo{
				{Name: "test-package", Arch: "src"}, // Should be skipped
				{Name: "test-package", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 1,
		},
		{
			name:     "package not found",
			requests: []string{"nonexistent"},
			all: []ospackage.PackageInfo{
				{Name: "test-package", Arch: "x86_64"},
			},
			expectError: true,
			expectCount: 0,
		},
		{
			name:     "multiple candidates - pick highest",
			requests: []string{"package"},
			all: []ospackage.PackageInfo{
				{Name: "package-1.0.0", Arch: "x86_64"},
				{Name: "package-2.0.0", Arch: "x86_64"},
				{Name: "package-1.5.0", Arch: "x86_64"},
			},
			expectError: false,
			expectCount: 1, // Should pick package-2.0.0 (highest lex sort)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := rpmutils.MatchRequested(tc.requests, tc.all)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(result) != tc.expectCount {
					t.Errorf("Expected %d packages, got %d", tc.expectCount, len(result))
				}
			}
		})
	}
}

func TestIsAcceptedChar(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "only digits",
			input:    "123",
			expected: true,
		},
		{
			name:     "digits with dash",
			input:    "1-2-3",
			expected: true,
		},
		{
			name:     "contains letters",
			input:    "1a2",
			expected: false,
		},
		{
			name:     "contains special chars",
			input:    "1.2",
			expected: false,
		},
		{
			name:     "only dash",
			input:    "-",
			expected: true,
		},
		{
			name:     "mixed valid chars",
			input:    "123-456-789",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Since isAcceptedChar is not exported, we need to test it indirectly through isValidVersionFormat
			// or we need to make it exported for testing. For now, let's test it indirectly.
			t.Skip("isAcceptedChar is not exported - testing indirectly through isValidVersionFormat")
		})
	}
}

func TestIsValidVersionFormat(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "simple version",
			input:    "1.0.0",
			expected: true,
		},
		{
			name:     "version with dash",
			input:    "1-2.el8",
			expected: true,
		},
		{
			name:     "version without dot",
			input:    "123",
			expected: true,
		},
		{
			name:     "invalid version with letters",
			input:    "abc.def",
			expected: false,
		},
		{
			name:     "version starting with letter",
			input:    "a1.0.0",
			expected: false,
		},
		{
			name:     "complex version",
			input:    "2-3-1.el8_5",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Since isValidVersionFormat is not exported, we'll test it indirectly
			// Let's create a test that exercises this through MatchRequested
			all := []ospackage.PackageInfo{
				{Name: fmt.Sprintf("package-%s", tc.input), Arch: "x86_64"},
			}

			result, err := rpmutils.MatchRequested([]string{"package"}, all)

			if tc.expected {
				// If the version format is valid, we should find a match
				if err != nil || len(result) == 0 {
					t.Errorf("Expected to find match for valid version format %q", tc.input)
				}
			} else {
				// If the version format is invalid, we should not find a match (unless it's exact)
				if err == nil && len(result) > 0 && result[0].Name != "package" {
					// Only fail if we found a match that wasn't exact
					exactMatch := false
					for _, pkg := range all {
						if pkg.Name == "package" {
							exactMatch = true
							break
						}
					}
					if !exactMatch {
						t.Errorf("Expected no match for invalid version format %q, but got: %v", tc.input, result)
					}
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// Save original global variables
	originalRepoCfg := rpmutils.RepoCfg
	defer func() {
		rpmutils.RepoCfg = originalRepoCfg
	}()

	testCases := []struct {
		name        string
		setupRepo   func() *httptest.Server
		destDir     func() string
		expectError bool
	}{
		{
			name: "invalid GPG key URL",
			setupRepo: func() *httptest.Server {
				return nil // No server
			},
			destDir: func() string {
				tmpDir, _ := os.MkdirTemp("", "test-rpms-*")
				return tmpDir
			},
			expectError: true,
		},
		{
			name: "no RPMs to verify",
			setupRepo: func() *httptest.Server {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					_, _ = w.Write([]byte("dummy-gpg-key-content"))
				}))
				return server
			},
			destDir: func() string {
				tmpDir, _ := os.MkdirTemp("", "test-rpms-*")
				return tmpDir
			},
			expectError: false, // Should not error when no RPMs found
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := tc.setupRepo()
			if server != nil {
				defer server.Close()
				rpmutils.RepoCfg = rpmutils.RepoConfig{
					GPGKey: server.URL,
				}
			} else {
				rpmutils.RepoCfg = rpmutils.RepoConfig{
					GPGKey: "invalid://invalid-url-scheme",
				}
			}

			destDir := tc.destDir()
			defer os.RemoveAll(destDir)

			err := rpmutils.Validate(destDir)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestResolve(t *testing.T) {
	testCases := []struct {
		name        string
		req         []ospackage.PackageInfo
		all         []ospackage.PackageInfo
		expectError bool
	}{
		{
			name: "empty request",
			req:  []ospackage.PackageInfo{},
			all: []ospackage.PackageInfo{
				{
					Name:        "test-package",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/test-package-1.0.0-1.el9.x86_64.rpm",
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			expectError: false,
		},
		{
			name: "simple resolve",
			req: []ospackage.PackageInfo{
				{
					Name:        "test-package",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/test-package-1.0.0-1.el9.x86_64.rpm",
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "test-package",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/test-package-1.0.0-1.el9.x86_64.rpm",
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			expectError: false,
		},
		{
			name: "resolve with dependencies",
			req: []ospackage.PackageInfo{
				{
					Name:        "app",
					Version:     "2.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/app-2.0.0-1.el9.x86_64.rpm",
					Requires:    []string{"libfoo"},
					RequiresVer: []string{"libfoo >= 1.0.0"},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "app",
					Version:     "2.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/app-2.0.0-1.el9.x86_64.rpm",
					Requires:    []string{"libfoo"},
					RequiresVer: []string{"libfoo >= 1.0.0"},
				},
				{
					Name:        "libfoo",
					Version:     "1.5.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/libfoo-1.5.0-1.el9.x86_64.rpm",
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := rpmutils.Resolve(tc.req, tc.all)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Error("Expected result to be non-nil")
				}
			}
		})
	}
}

func TestDownloadPackages(t *testing.T) {
	// Save original global variables
	originalRepoCfg := rpmutils.RepoCfg
	originalGzHref := rpmutils.GzHref
	defer func() {
		rpmutils.RepoCfg = originalRepoCfg
		rpmutils.GzHref = originalGzHref
	}()

	testCases := []struct {
		name        string
		pkgList     []string
		destDir     string
		dotFile     string
		expectError bool
	}{
		{
			name:        "empty package list",
			pkgList:     []string{},
			destDir:     "",
			dotFile:     "",
			expectError: true, // Will fail when trying to fetch packages
		},
		{
			name:        "invalid destination directory",
			pkgList:     []string{"test-package"},
			destDir:     "/invalid/path/that/cannot/be/created",
			dotFile:     "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up invalid repo config to trigger early error
			rpmutils.RepoCfg = rpmutils.RepoConfig{
				URL: "invalid-url",
			}
			rpmutils.GzHref = "repodata/primary.xml.gz"

			if tc.destDir == "" {
				tmpDir, err := os.MkdirTemp("", "test-download-*")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				tc.destDir = tmpDir
				defer os.RemoveAll(tmpDir)
			}

			result, err := rpmutils.DownloadPackages(tc.pkgList, tc.destDir, tc.dotFile)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if result == nil {
					t.Error("Expected result to be non-nil")
				}
			}
		})
	}
}

func TestRepoConfig(t *testing.T) {
	// Test RepoConfig struct initialization
	config := rpmutils.RepoConfig{
		Section:      "[test-repo]",
		Name:         "Test Repository",
		URL:          "https://example.com/repo",
		GPGCheck:     true,
		RepoGPGCheck: true,
		Enabled:      true,
		GPGKey:       "https://example.com/key.asc",
	}

	if config.Section != "[test-repo]" {
		t.Errorf("Expected Section to be '[test-repo]', got %s", config.Section)
	}
	if config.Name != "Test Repository" {
		t.Errorf("Expected Name to be 'Test Repository', got %s", config.Name)
	}
	if config.URL != "https://example.com/repo" {
		t.Errorf("Expected URL to be 'https://example.com/repo', got %s", config.URL)
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
	if config.GPGKey != "https://example.com/key.asc" {
		t.Errorf("Expected GPGKey to be 'https://example.com/key.asc', got %s", config.GPGKey)
	}
}

func TestGlobalVariables(t *testing.T) {
	// Test that global variables can be accessed and modified
	originalRepoCfg := rpmutils.RepoCfg
	originalGzHref := rpmutils.GzHref

	// Modify globals
	rpmutils.RepoCfg = rpmutils.RepoConfig{
		URL:  "test-url",
		Name: "test-name",
	}
	rpmutils.GzHref = "test-href"

	// Verify changes
	if rpmutils.RepoCfg.URL != "test-url" {
		t.Errorf("Expected RepoCfg.URL to be 'test-url', got %s", rpmutils.RepoCfg.URL)
	}
	if rpmutils.RepoCfg.Name != "test-name" {
		t.Errorf("Expected RepoCfg.Name to be 'test-name', got %s", rpmutils.RepoCfg.Name)
	}
	if rpmutils.GzHref != "test-href" {
		t.Errorf("Expected GzHref to be 'test-href', got %s", rpmutils.GzHref)
	}

	// Restore original values
	rpmutils.RepoCfg = originalRepoCfg
	rpmutils.GzHref = originalGzHref
}

// TestResolveEdgeCases tests edge cases in the Resolve function
func TestResolveEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		req         []ospackage.PackageInfo
		all         []ospackage.PackageInfo
		expectError bool
		description string
	}{
		{
			name:        "Circular dependencies",
			description: "Package A depends on B, B depends on A",
			req: []ospackage.PackageInfo{
				{
					Name:        "package-a",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/package-a-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"package-a"},
					Requires:    []string{"package-b"},
					RequiresVer: []string{"package-b >= 1.0.0"},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "package-a",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/package-a-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"package-a"},
					Requires:    []string{"package-b"},
					RequiresVer: []string{"package-b >= 1.0.0"},
				},
				{
					Name:        "package-b",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/package-b-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"package-b"},
					Requires:    []string{"package-a"},
					RequiresVer: []string{"package-a >= 1.0.0"},
				},
			},
			expectError: false, // Should handle circular deps gracefully
		},
		{
			name:        "Self dependency",
			description: "Package depends on itself",
			req: []ospackage.PackageInfo{
				{
					Name:        "self-dep",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/self-dep-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"self-dep"},
					Requires:    []string{"self-dep"},
					RequiresVer: []string{"self-dep >= 1.0.0"},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "self-dep",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/self-dep-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"self-dep"},
					Requires:    []string{"self-dep"},
					RequiresVer: []string{"self-dep >= 1.0.0"},
				},
			},
			expectError: false, // Should handle self-deps
		},
		{
			name:        "Deep dependency chain",
			description: "Package with many levels of dependencies",
			req: []ospackage.PackageInfo{
				{
					Name:        "level-0",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/level-0-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"level-0"},
					Requires:    []string{"level-1"},
					RequiresVer: []string{"level-1 >= 1.0.0"},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "level-0",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/level-0-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"level-0"},
					Requires:    []string{"level-1"},
					RequiresVer: []string{"level-1 >= 1.0.0"},
				},
				{
					Name:        "level-1",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/level-1-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"level-1"},
					Requires:    []string{"level-2"},
					RequiresVer: []string{"level-2 >= 1.0.0"},
				},
				{
					Name:        "level-2",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/level-2-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"level-2"},
					Requires:    []string{"level-3"},
					RequiresVer: []string{"level-3 >= 1.0.0"},
				},
				{
					Name:        "level-3",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/level-3-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"level-3"},
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			expectError: false,
		},
		{
			name:        "Package not in all list",
			description: "Requested package not available in repository",
			req: []ospackage.PackageInfo{
				{
					Name:        "missing-package",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/missing-package-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"missing-package"},
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			all: []ospackage.PackageInfo{
				{
					Name:        "other-package",
					Version:     "1.0.0",
					Arch:        "x86_64",
					URL:         "https://repo.example.com/rpm/other-package-1.0.0-1.el9.x86_64.rpm",
					Provides:    []string{"other-package"},
					Requires:    []string{},
					RequiresVer: []string{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rpmutils.Resolve(tt.req, tt.all)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s but got none", tt.description)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result for %s", tt.description)
			}

			// Verify that requested packages are in the result
			resultMap := make(map[string]bool)
			for _, pkg := range result {
				resultMap[pkg.Name] = true
			}

			for _, reqPkg := range tt.req {
				if !resultMap[reqPkg.Name] {
					t.Errorf("Requested package %s not found in result for %s", reqPkg.Name, tt.description)
				}
			}
		})
	}
}

// TestMatchRequestedPerformance tests performance with large package lists
func TestMatchRequestedPerformance(t *testing.T) {
	// Generate a large list of packages
	var allPackages []ospackage.PackageInfo
	for i := 0; i < 1000; i++ {
		pkg := ospackage.PackageInfo{
			Name:    fmt.Sprintf("package-%04d", i),
			Version: "1.0.0",
			Arch:    "x86_64",
			URL:     fmt.Sprintf("https://repo.example.com/package-%04d-1.0.0-1.el9.x86_64.rpm", i),
		}
		allPackages = append(allPackages, pkg)
	}

	// Test with various request sizes
	tests := []struct {
		name         string
		requestCount int
	}{
		{"Single package", 1},
		{"Small batch", 10},
		{"Medium batch", 100},
		{"Large batch", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []string
			for i := 0; i < tt.requestCount; i++ {
				requests = append(requests, fmt.Sprintf("package-%04d", i))
			}

			result, err := rpmutils.MatchRequested(requests, allPackages)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != tt.requestCount {
				t.Errorf("Expected %d packages, got %d", tt.requestCount, len(result))
			}
		})
	}
}
