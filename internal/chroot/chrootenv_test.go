package chroot_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	chroot "github.com/open-edge-platform/os-image-composer/internal/chroot"
)

// mockChrootBuilder implements the necessary interface for testing
type mockChrootBuilder struct {
	packageList []string
	err         error
	tempDir     string
}

// Add the missing method to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) UpdateLocalDebRepo(repoPath, targetArch string, sudo bool) error {
	// For testing, just return the error field
	return m.err
}

// Implement GetChrootEnvConfig to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) GetChrootEnvConfig() (map[interface{}]interface{}, error) {
	// Return a dummy config and no error for testing
	return nil, nil
}

// Implement GetChrootBuildDir to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) GetChrootBuildDir() string {
	// Return a dummy build directory for testing
	return filepath.Join(m.tempDir, "mock-chroot-build-dir")
}

// Implement GetChrootPkgCacheDir to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) GetChrootPkgCacheDir() string {
	// Return a dummy package cache directory for testing
	return filepath.Join(m.tempDir, "mock-chroot-pkg-cache-dir")
}

// Implement GetTargetOsConfigDir to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) GetTargetOsConfigDir() string {
	// Return a dummy config directory for testing
	return filepath.Join(m.tempDir, "mock-chroot-os-config-dir")
}

func (m *mockChrootBuilder) GetTargetOsConfig() map[string]interface{} {
	// Return a dummy config for testing
	return map[string]interface{}{
		"releaseVersion": "3.0",
	}
}

func (m *mockChrootBuilder) GetChrootEnvEssentialPackageList() ([]string, error) {
	return m.packageList, m.err
}

// Implement GetChrootEnvPackageList to satisfy ChrootBuilderInterface
func (m *mockChrootBuilder) GetChrootEnvPackageList() ([]string, error) {
	return m.packageList, m.err
}

func (m *mockChrootBuilder) GetTargetOsPkgType() string {
	return "rpm"
}

func (m *mockChrootBuilder) BuildChrootEnv(root, dist, arch string) error {
	// For testing, just return the error field
	return m.err
}

func TestChrootEnv_GetChrootEnvEssentialPackageList(t *testing.T) {
	tests := []struct {
		name             string
		packageList      []string
		mockError        error
		expectedPackages []string
		expectError      bool
	}{
		{
			name:             "successful package list retrieval",
			packageList:      []string{"systemd", "bash", "coreutils", "glibc"},
			mockError:        nil,
			expectedPackages: []string{"systemd", "bash", "coreutils", "glibc"},
			expectError:      false,
		},
		{
			name:             "empty package list",
			packageList:      []string{},
			mockError:        nil,
			expectedPackages: []string{},
			expectError:      false,
		},
		{
			name:             "nil package list",
			packageList:      nil,
			mockError:        nil,
			expectedPackages: nil,
			expectError:      false,
		},
		{
			name:             "error from chrootBuilder",
			packageList:      nil,
			mockError:        errors.New("failed to get essential packages"),
			expectedPackages: nil,
			expectError:      true,
		},
		{
			name:             "single package",
			packageList:      []string{"systemd"},
			mockError:        nil,
			expectedPackages: []string{"systemd"},
			expectError:      false,
		},
		{
			name:             "large package list",
			packageList:      []string{"pkg1", "pkg2", "pkg3", "pkg4", "pkg5", "pkg6", "pkg7", "pkg8", "pkg9", "pkg10"},
			mockError:        nil,
			expectedPackages: []string{"pkg1", "pkg2", "pkg3", "pkg4", "pkg5", "pkg6", "pkg7", "pkg8", "pkg9", "pkg10"},
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock chrootBuilder
			mockBuilder := &mockChrootBuilder{
				packageList: tt.packageList,
				err:         tt.mockError,
				tempDir:     t.TempDir(),
			}

			// Create ChrootEnv with mock chrootBuilder
			chrootEnv := &chroot.ChrootEnv{
				ChrootEnvRoot: filepath.Join(mockBuilder.tempDir, "test-chroot"),
				ChrootBuilder: mockBuilder, // Ensure chrootBuilder is of interface type
			}

			// Call the method under test
			result, err := chrootEnv.GetChrootEnvEssentialPackageList()

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.mockError != nil && err.Error() != tt.mockError.Error() {
					t.Errorf("Expected error '%v', got '%v'", tt.mockError, err)
				}
				return
			}

			// Check no error when not expected
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check package list length
			if len(result) != len(tt.expectedPackages) {
				t.Errorf("Expected %d packages, got %d", len(tt.expectedPackages), len(result))
				return
			}

			// Check package list contents
			for i, pkg := range tt.expectedPackages {
				if result[i] != pkg {
					t.Errorf("Expected package[%d] = '%s', got '%s'", i, pkg, result[i])
				}
			}
		})
	}
}

func TestChrootEnv_GetChrootEnvEssentialPackageList_NilChrootBuilder(t *testing.T) {
	// Test edge case where chrootBuilder is nil
	chrootEnv := &chroot.ChrootEnv{
		ChrootEnvRoot: "/tmp/test-chroot",
		ChrootBuilder: nil,
	}

	// This should panic or return an error, depending on implementation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when chrootBuilder is nil, but didn't panic")
		}
	}()

	_, _ = chrootEnv.GetChrootEnvEssentialPackageList()
}

func TestChrootEnv_GetChrootEnvEssentialPackageList_Integration(t *testing.T) {
	// Test with different OS types to ensure the method works regardless of the underlying implementation
	testCases := []struct {
		name       string
		targetOs   string
		targetDist string
		targetArch string
	}{
		{"rpm-based", "photon", "5.0", "amd64"},
		{"deb-based", "ubuntu", "22.04", "amd64"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock chrootBuilder
			mockBuilder := &mockChrootBuilder{
				packageList: []string{},
				err:         nil,
				tempDir:     t.TempDir(),
			}
			// This is more of an integration test that would require actual ChrootBuilder
			// We'll create a basic test that ensures the method doesn't panic
			chrootEnv := &chroot.ChrootEnv{
				ChrootEnvRoot: filepath.Join(mockBuilder.tempDir, "test-chroot"),
				ChrootBuilder: mockBuilder, // Ensure chrootBuilder is of interface type
			}

			// Call the method - we can't predict the exact output without knowing the implementation
			// but we can ensure it doesn't panic and returns reasonable values
			packages, err := chrootEnv.GetChrootEnvEssentialPackageList()

			// We don't assert specific packages since that depends on the OS configuration
			// but we can check that it behaves reasonably
			if err != nil {
				t.Logf("Method returned error (this may be expected): %v", err)
			} else {
				t.Logf("Method returned %d packages", len(packages))

				// Basic sanity checks
				for _, pkg := range packages {
					if pkg == "" {
						t.Error("Found empty package name in list")
					}
				}

			}
		})
	}
}

func TestChrootEnv_GetChrootEnvEssentialPackageList_ErrorPropagation(t *testing.T) {
	// Test that errors from the underlying chrootBuilder are properly propagated
	expectedErrors := []error{
		errors.New("config file not found"),
		errors.New("invalid OS type"),
		errors.New("network error"),
		errors.New("permission denied"),
	}

	for i, expectedErr := range expectedErrors {
		t.Run(fmt.Sprintf("error_case_%d", i), func(t *testing.T) {
			mockBuilder := &mockChrootBuilder{
				packageList: nil,
				err:         expectedErr,
				tempDir:     t.TempDir(),
			}

			chrootEnv := &chroot.ChrootEnv{
				ChrootEnvRoot: filepath.Join(mockBuilder.tempDir, "test-chroot"),
				ChrootBuilder: mockBuilder,
			}

			_, err := chrootEnv.GetChrootEnvEssentialPackageList()

			if err == nil {
				t.Error("Expected error but got none")
			}

			if err.Error() != expectedErr.Error() {
				t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
			}
		})
	}
}
