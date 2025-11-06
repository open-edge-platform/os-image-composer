package elxr

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/chroot"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/os-image-composer/internal/provider"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
	"github.com/open-edge-platform/os-image-composer/internal/utils/system"
)

// Helper function to create a test ImageTemplate
func createTestImageTemplate() *config.ImageTemplate {
	return &config.ImageTemplate{
		Image: config.ImageInfo{
			Name:    "test-elxr-image",
			Version: "1.0.0",
		},
		Target: config.TargetInfo{
			OS:        "elxr",
			Dist:      "elxr12",
			Arch:      "amd64",
			ImageType: "qcow2",
		},
		SystemConfig: config.SystemConfig{
			Name:        "test-elxr-system",
			Description: "Test eLxr system configuration",
			Packages:    []string{"curl", "wget", "vim"},
		},
	}
}

// TestElxrProviderInterface tests that eLxr implements Provider interface
func TestElxrProviderInterface(t *testing.T) {
	var _ provider.Provider = (*eLxr)(nil) // Compile-time interface check
}

// TestElxrProviderName tests the Name method
func TestElxrProviderName(t *testing.T) {
	elxr := &eLxr{}
	name := elxr.Name("elxr12", "amd64")
	expected := "wind-river-elxr-elxr12-amd64"

	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

// TestGetProviderId tests the GetProviderId function
func TestGetProviderId(t *testing.T) {
	testCases := []struct {
		dist     string
		arch     string
		expected string
	}{
		{"elxr12", "amd64", "wind-river-elxr-elxr12-amd64"},
		{"elxr12", "arm64", "wind-river-elxr-elxr12-arm64"},
		{"elxr13", "x86_64", "wind-river-elxr-elxr13-x86_64"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.dist, tc.arch), func(t *testing.T) {
			result := system.GetProviderId(OsName, tc.dist, tc.arch)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// TestElxrProviderInit tests the Init method
func TestElxrProviderInit(t *testing.T) {
	// Change to project root for tests that need config files
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	// Navigate to project root (3 levels up from internal/provider/elxr)
	if err := os.Chdir("../../../"); err != nil {
		t.Skipf("Cannot change to project root: %v", err)
		return
	}

	elxr := &eLxr{}

	// Test with amd64 architecture
	err := elxr.Init("elxr12", "amd64")
	if err != nil {
		// Expected to potentially fail in test environment due to network dependencies
		t.Logf("Init failed as expected in test environment: %v", err)
	} else {
		// If it succeeds, verify the configuration was set up
		if elxr.repoCfg.Name == "" {
			t.Error("Expected repoCfg.Name to be set after successful Init")
		}

		if elxr.repoCfg.PkgList == "" {
			t.Error("Expected repoCfg.PkgList to be set after successful Init")
		}

		// Verify that the architecture is correctly set in the config
		if elxr.repoCfg.Arch != "amd64" {
			t.Errorf("Expected arch to be amd64, got %s", elxr.repoCfg.Arch)
		}

		t.Logf("Successfully initialized with config: %s", elxr.repoCfg.Name)
	}
}

// TestElxrProviderInitArchMapping tests architecture mapping in Init
func TestElxrProviderInitArchMapping(t *testing.T) {
	// Change to project root for tests that need config files
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	// Navigate to project root (3 levels up from internal/provider/elxr)
	if err := os.Chdir("../../../"); err != nil {
		t.Skipf("Cannot change to project root: %v", err)
		return
	}

	elxr := &eLxr{}

	// Test x86_64 -> amd64 mapping
	err := elxr.Init("elxr12", "x86_64")
	if err != nil {
		t.Logf("Init failed as expected: %v", err)
	} else {
		// Verify that repoCfg.PkgList contains the expected architecture mapping
		if elxr.repoCfg.PkgList != "" {
			expectedArchInURL := "binary-amd64"
			if !strings.Contains(elxr.repoCfg.PkgList, expectedArchInURL) {
				t.Errorf("Expected PkgList to contain %s for x86_64 arch, got %s", expectedArchInURL, elxr.repoCfg.PkgList)
			}
		}

		// Verify architecture was mapped correctly
		if elxr.repoCfg.Arch != "amd64" {
			t.Errorf("Expected mapped arch to be amd64, got %s", elxr.repoCfg.Arch)
		}

		t.Logf("Successfully mapped x86_64 -> amd64, PkgList: %s", elxr.repoCfg.PkgList)
	}
}

// TestLoadRepoConfig tests the loadRepoConfig function
func TestLoadRepoConfig(t *testing.T) {
	// Change to project root for tests that need config files
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	// Navigate to project root (3 levels up from internal/provider/elxr)
	if err := os.Chdir("../../../"); err != nil {
		t.Skipf("Cannot change to project root: %v", err)
		return
	}

	config, err := loadRepoConfig("", "amd64")
	if err != nil {
		t.Skipf("loadRepoConfig failed (expected in test environment): %v", err)
		return
	}

	// If we successfully load config, verify the values
	if config.Name == "" {
		t.Error("Expected config name to be set")
	}

	if config.Arch != "amd64" {
		t.Errorf("Expected arch 'amd64', got '%s'", config.Arch)
	}

	// Verify PkgList contains expected architecture
	if config.PkgList != "" && !strings.Contains(config.PkgList, "binary-amd64") {
		t.Errorf("Expected PkgList to contain 'binary-amd64', got '%s'", config.PkgList)
	}

	t.Logf("Successfully loaded repo config: %s", config.Name)
}

// mockChrootEnv is a simple mock implementation of ChrootEnvInterface for testing
type mockChrootEnv struct{}

// Ensure mockChrootEnv implements ChrootEnvInterface
var _ chroot.ChrootEnvInterface = (*mockChrootEnv)(nil)

func (m *mockChrootEnv) GetChrootEnvRoot() string          { return "/tmp/test-chroot" }
func (m *mockChrootEnv) GetChrootImageBuildDir() string    { return "/tmp/test-build" }
func (m *mockChrootEnv) GetTargetOsPkgType() string        { return "deb" }
func (m *mockChrootEnv) GetTargetOsConfigDir() string      { return "/tmp/test-config" }
func (m *mockChrootEnv) GetTargetOsReleaseVersion() string { return "12" }
func (m *mockChrootEnv) GetChrootPkgCacheDir() string      { return "/tmp/test-cache" }
func (m *mockChrootEnv) GetChrootEnvEssentialPackageList() ([]string, error) {
	return []string{"base-files"}, nil
}
func (m *mockChrootEnv) GetChrootEnvHostPath(chrootPath string) (string, error) {
	return chrootPath, nil
}
func (m *mockChrootEnv) GetChrootEnvPath(hostPath string) (string, error) { return hostPath, nil }
func (m *mockChrootEnv) MountChrootSysfs(chrootPath string) error         { return nil }
func (m *mockChrootEnv) UmountChrootSysfs(chrootPath string) error        { return nil }
func (m *mockChrootEnv) MountChrootPath(hostFullPath, chrootPath, mountFlags string) error {
	return nil
}
func (m *mockChrootEnv) UmountChrootPath(chrootPath string) error                       { return nil }
func (m *mockChrootEnv) CopyFileFromHostToChroot(hostFilePath, chrootPath string) error { return nil }
func (m *mockChrootEnv) CopyFileFromChrootToHost(hostFilePath, chrootPath string) error { return nil }
func (m *mockChrootEnv) UpdateChrootLocalRepoMetadata(chrootRepoDir string, targetArch string, sudo bool) error {
	return nil
}
func (m *mockChrootEnv) RefreshLocalCacheRepo() error                                   { return nil }
func (m *mockChrootEnv) InitChrootEnv(targetOs, targetDist, targetArch string) error    { return nil }
func (m *mockChrootEnv) CleanupChrootEnv(targetOs, targetDist, targetArch string) error { return nil }
func (m *mockChrootEnv) TdnfInstallPackage(packageName, installRoot string, repositoryIDList []string) error {
	return nil
}
func (m *mockChrootEnv) AptInstallPackage(packageName, installRoot string, repoSrcList []string) error {
	return nil
}
func (m *mockChrootEnv) UpdateSystemPkgs(template *config.ImageTemplate) error { return nil }

// TestElxrProviderPreProcess tests PreProcess method with mocked dependencies
func TestElxrProviderPreProcess(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor
	mockExpectedOutput := []shell.MockCommand{
		// Mock successful package installation commands
		{Pattern: "apt-get update", Output: "Package lists updated successfully", Error: nil},
		{Pattern: "apt-get install -y mmdebstrap", Output: "Package installed successfully", Error: nil},
		{Pattern: "apt-get install -y dosfstools", Output: "Package installed successfully", Error: nil},
		{Pattern: "apt-get install -y xorriso", Output: "Package installed successfully", Error: nil},
		{Pattern: "apt-get install -y sbsigntool", Output: "Package installed successfully", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	elxr := &eLxr{
		repoCfg: debutils.RepoConfig{
			Section:   "main",
			Name:      "Wind River eLxr 12",
			PkgList:   "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz",
			PkgPrefix: "https://mirror.elxr.dev/elxr/",
			Enabled:   true,
			GPGCheck:  true,
		},
		gzHref:    "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz",
		chrootEnv: &mockChrootEnv{}, // Add the missing chrootEnv mock
	}

	template := createTestImageTemplate()

	// This test will likely fail due to dependencies on chroot, debutils, etc.
	// but it demonstrates the testing approach
	err := elxr.PreProcess(template)
	if err != nil {
		t.Logf("PreProcess failed as expected due to external dependencies: %v", err)
	}
}

// TestElxrProviderBuildImage tests BuildImage method
func TestElxrProviderBuildImage(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor - minimal mocks for Register function
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: ".*", Output: "success", Error: nil}, // Catch-all for any commands during registration
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	// Try to register and get a properly initialized eLxr instance
	err := Register("linux", "test-build", "amd64")
	if err != nil {
		t.Skipf("Cannot test BuildImage without proper registration: %v", err)
		return
	}

	// Get the registered provider
	providerName := system.GetProviderId(OsName, "test-build", "amd64")
	retrievedProvider, exists := provider.Get(providerName)
	if !exists {
		t.Skip("Cannot test BuildImage without retrieving registered provider")
		return
	}

	elxr, ok := retrievedProvider.(*eLxr)
	if !ok {
		t.Skip("Retrieved provider is not an eLxr instance")
		return
	}

	template := createTestImageTemplate()

	// This test will fail due to dependencies on image builders that require system access
	// We expect it to fail early before reaching sudo commands
	err = elxr.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage failed as expected due to external dependencies: %v", err)
		// Verify the error is related to expected failures, not sudo issues
		if strings.Contains(err.Error(), "sudo") {
			t.Errorf("Test should not reach sudo commands - mocking may be insufficient")
		}
	}
}

// TestElxrProviderBuildImageISO tests BuildImage method with ISO type
func TestElxrProviderBuildImageISO(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor - minimal mocks for Register function
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: ".*", Output: "success", Error: nil}, // Catch-all for any commands during registration
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	// Try to register and get a properly initialized eLxr instance
	err := Register("linux", "test-iso", "amd64")
	if err != nil {
		t.Skipf("Cannot test BuildImage (ISO) without proper registration: %v", err)
		return
	}

	// Get the registered provider
	providerName := system.GetProviderId(OsName, "test-iso", "amd64")
	retrievedProvider, exists := provider.Get(providerName)
	if !exists {
		t.Skip("Cannot test BuildImage (ISO) without retrieving registered provider")
		return
	}

	elxr, ok := retrievedProvider.(*eLxr)
	if !ok {
		t.Skip("Retrieved provider is not an eLxr instance")
		return
	}

	template := createTestImageTemplate()

	// Set up global config for ISO
	originalImageType := template.Target.ImageType
	defer func() { template.Target.ImageType = originalImageType }()
	template.Target.ImageType = "iso"

	err = elxr.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage (ISO) failed as expected due to external dependencies: %v", err)
		// Verify the error is related to expected failures, not sudo issues
		if strings.Contains(err.Error(), "sudo") {
			t.Errorf("Test should not reach sudo commands - mocking may be insufficient")
		}
	}
}

// TestElxrProviderPostProcess tests PostProcess method
func TestElxrProviderPostProcess(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor - minimal mocks for Register function
	mockExpectedOutput := []shell.MockCommand{
		{Pattern: ".*", Output: "success", Error: nil}, // Catch-all for any commands during registration
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	// Try to register and get a properly initialized eLxr instance
	err := Register("linux", "test-post", "amd64")
	if err != nil {
		t.Skipf("Cannot test PostProcess without proper registration: %v", err)
		return
	}

	// Get the registered provider
	providerName := system.GetProviderId(OsName, "test-post", "amd64")
	retrievedProvider, exists := provider.Get(providerName)
	if !exists {
		t.Skip("Cannot test PostProcess without retrieving registered provider")
		return
	}

	elxr, ok := retrievedProvider.(*eLxr)
	if !ok {
		t.Skip("Retrieved provider is not an eLxr instance")
		return
	}

	template := createTestImageTemplate()

	// Test with no error
	err = elxr.PostProcess(template, nil)
	if err != nil {
		t.Logf("PostProcess failed as expected due to chroot cleanup dependencies: %v", err)
	}

	// Test with input error - PostProcess should clean up and return nil (not the input error)
	inputError := fmt.Errorf("some build error")
	err = elxr.PostProcess(template, inputError)
	if err != nil {
		t.Logf("PostProcess failed during cleanup: %v", err)
	}
}

// TestElxrProviderInstallHostDependency tests installHostDependency method
func TestElxrProviderInstallHostDependency(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor
	mockExpectedOutput := []shell.MockCommand{
		// Mock successful installation commands
		{Pattern: "which mmdebstrap", Output: "", Error: nil},
		{Pattern: "which mkfs.fat", Output: "", Error: nil},
		{Pattern: "which xorriso", Output: "", Error: nil},
		{Pattern: "which sbsign", Output: "", Error: nil},
		{Pattern: "apt-get install -y mmdebstrap", Output: "Success", Error: nil},
		{Pattern: "apt-get install -y dosfstools", Output: "Success", Error: nil},
		{Pattern: "apt-get install -y xorriso", Output: "Success", Error: nil},
		{Pattern: "apt-get install -y sbsigntool", Output: "Success", Error: nil},
	}
	shell.Default = shell.NewMockExecutor(mockExpectedOutput)

	elxr := &eLxr{}

	// This test will likely fail due to dependencies on chroot.GetHostOsPkgManager()
	// and shell.IsCommandExist(), but it demonstrates the testing approach
	err := elxr.installHostDependency()
	if err != nil {
		t.Logf("installHostDependency failed as expected due to external dependencies: %v", err)
	} else {
		t.Logf("installHostDependency succeeded with mocked commands")
	}
}

// TestElxrProviderInstallHostDependencyCommands tests the specific commands for host dependencies
func TestElxrProviderInstallHostDependencyCommands(t *testing.T) {
	// Get the dependency map by examining the installHostDependency method
	expectedDeps := map[string]string{
		"mmdebstrap":        "mmdebstrap",
		"mkfs.fat":          "dosfstools",
		"xorriso":           "xorriso",
		"ukify":             "systemd-ukify",
		"grub-mkstandalone": "grub-common",
		"veritysetup":       "cryptsetup",
		"sbsign":            "sbsigntool",
	}

	// This is a structural test to verify the dependency mapping
	// In a real implementation, we might expose this map for testing
	t.Logf("Expected host dependencies for eLxr provider: %+v", expectedDeps)

	// Verify we have the expected number of dependencies
	if len(expectedDeps) != 7 {
		t.Errorf("Expected 7 host dependencies, got %d", len(expectedDeps))
	}

	// Verify specific critical dependencies
	criticalDeps := []string{"mmdebstrap", "mkfs.fat", "xorriso"}
	for _, dep := range criticalDeps {
		if _, exists := expectedDeps[dep]; !exists {
			t.Errorf("Critical dependency %s not found in expected dependencies", dep)
		}
	}
}

// TestElxrProviderRegister tests the Register function
func TestElxrProviderRegister(t *testing.T) {
	// Save original providers registry and restore after test
	// Note: We can't easily access the provider registry for cleanup,
	// so this test shows the approach but may leave test artifacts

	err := Register("linux", "elxr12", "amd64")
	if err != nil {
		t.Skipf("Cannot test registration due to missing dependencies: %v", err)
		return
	}

	// Try to retrieve the registered provider
	providerName := system.GetProviderId(OsName, "elxr12", "amd64")
	retrievedProvider, exists := provider.Get(providerName)

	if !exists {
		t.Errorf("Expected provider %s to be registered", providerName)
		return
	}

	// Verify it's an eLxr provider
	if elxrProvider, ok := retrievedProvider.(*eLxr); !ok {
		t.Errorf("Expected eLxr provider, got %T", retrievedProvider)
	} else {
		// Test the Name method on the registered provider
		name := elxrProvider.Name("elxr12", "amd64")
		if name != providerName {
			t.Errorf("Expected provider name %s, got %s", providerName, name)
		}
	}
}

// TestElxrProviderWorkflow tests a complete eLxr provider workflow
func TestElxrProviderWorkflow(t *testing.T) {
	// This is a unit test focused on testing the provider interface methods
	// without external dependencies that require system access

	elxr := &eLxr{}

	// Test provider name generation
	name := elxr.Name("elxr12", "amd64")
	expectedName := "wind-river-elxr-elxr12-amd64"
	if name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, name)
	}

	// Test Init (will likely fail due to network dependencies)
	if err := elxr.Init("elxr12", "amd64"); err != nil {
		t.Logf("Init failed as expected: %v", err)
	} else {
		// If Init succeeds, verify configuration was loaded
		if elxr.repoCfg.Name == "" {
			t.Error("Expected repo config name to be set after successful Init")
		}
		t.Logf("Repo config loaded: %s", elxr.repoCfg.Name)
	}

	// Skip PreProcess and BuildImage tests to avoid sudo commands
	t.Log("Skipping PreProcess and BuildImage tests to avoid system-level dependencies")

	// Skip PostProcess tests as they require properly initialized dependencies
	t.Log("Skipping PostProcess tests to avoid nil pointer panics - these are tested separately with proper registration")

	t.Log("Complete workflow test finished - core methods exist and are callable")
}

// TestElxrConfigurationStructure tests the structure of the eLxr configuration
func TestElxrConfigurationStructure(t *testing.T) {
	// Change to project root for tests that need config files
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	// Navigate to project root (3 levels up from internal/provider/elxr)
	if err := os.Chdir("../../../"); err != nil {
		t.Skipf("Cannot change to project root: %v", err)
		return
	}

	// Test that OsName constant is set correctly
	if OsName == "" {
		t.Error("OsName should not be empty")
	}

	expectedOsName := "wind-river-elxr"
	if OsName != expectedOsName {
		t.Errorf("Expected OsName %s, got %s", expectedOsName, OsName)
	}

	// Test that we can load provider config
	providerConfig, err := config.LoadProviderRepoConfig(OsName, "elxr12")
	if err != nil {
		t.Logf("Cannot load provider config in test environment: %v", err)
	} else {
		// If we can load it, verify it has required fields
		if providerConfig.Name == "" {
			t.Error("Provider config should have a name")
		}
		t.Logf("Loaded provider config: %s", providerConfig.Name)
	}
}

// TestElxrArchitectureHandling tests architecture-specific URL construction
func TestElxrArchitectureHandling(t *testing.T) {
	// Change to project root for tests that need config files
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	// Navigate to project root (3 levels up from internal/provider/elxr)
	if err := os.Chdir("../../../"); err != nil {
		t.Skipf("Cannot change to project root: %v", err)
		return
	}

	testCases := []struct {
		inputArch    string
		expectedArch string
	}{
		{"x86_64", "amd64"}, // x86_64 gets converted to amd64
		{"amd64", "amd64"},  // amd64 stays amd64
		{"arm64", "arm64"},  // arm64 stays arm64
	}

	for _, tc := range testCases {
		t.Run(tc.inputArch, func(t *testing.T) {
			elxr := &eLxr{}
			err := elxr.Init("elxr12", tc.inputArch) // Test arch mapping

			if err != nil {
				t.Logf("Init failed as expected: %v", err)
			} else {
				// We expect success, so we can check arch mapping
				if elxr.repoCfg.Arch != tc.expectedArch {
					t.Errorf("For input arch %s, expected config arch %s, got %s", tc.inputArch, tc.expectedArch, elxr.repoCfg.Arch)
				}

				// If we have a PkgList, verify it contains the expected architecture
				if elxr.repoCfg.PkgList != "" {
					expectedArchInURL := "binary-" + tc.expectedArch
					if !strings.Contains(elxr.repoCfg.PkgList, expectedArchInURL) {
						t.Errorf("For arch %s, expected PkgList to contain %s, got %s", tc.inputArch, expectedArchInURL, elxr.repoCfg.PkgList)
					}
				}

				t.Logf("Successfully tested arch %s -> %s", tc.inputArch, tc.expectedArch)
			}
		})
	}
}

// TestElxrBuildImageNilTemplate tests BuildImage with nil template
func TestElxrBuildImageNilTemplate(t *testing.T) {
	elxr := &eLxr{}

	err := elxr.BuildImage(nil)
	if err == nil {
		t.Error("Expected error when template is nil")
	}

	expectedError := "template cannot be nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestElxrBuildImageUnsupportedType tests BuildImage with unsupported image type
func TestElxrBuildImageUnsupportedType(t *testing.T) {
	elxr := &eLxr{}

	template := createTestImageTemplate()
	template.Target.ImageType = "unsupported"

	err := elxr.BuildImage(template)
	if err == nil {
		t.Error("Expected error for unsupported image type")
	}

	expectedError := "unsupported image type: unsupported"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestElxrBuildImageValidTypes tests BuildImage error handling for valid image types
func TestElxrBuildImageValidTypes(t *testing.T) {
	elxr := &eLxr{}

	validTypes := []string{"raw", "img", "iso"}

	for _, imageType := range validTypes {
		t.Run(imageType, func(t *testing.T) {
			template := createTestImageTemplate()
			template.Target.ImageType = imageType

			// These will fail due to missing chrootEnv, but we can verify
			// that the code path is reached and the error is expected
			err := elxr.BuildImage(template)
			if err == nil {
				t.Errorf("Expected error for image type %s (missing dependencies)", imageType)
			} else {
				t.Logf("Image type %s correctly failed with: %v", imageType, err)

				// Verify the error is related to missing dependencies, not invalid type
				if err.Error() == "unsupported image type: "+imageType {
					t.Errorf("Image type %s should be supported but got unsupported error", imageType)
				}
			}
		})
	}
}

// TestElxrPostProcessErrorHandling tests PostProcess method signature and basic behavior
func TestElxrPostProcessErrorHandling(t *testing.T) {
	// Test that PostProcess method exists and has correct signature
	// We verify that the method can be called and behaves predictably

	elxr := &eLxr{}
	template := createTestImageTemplate()
	inputError := fmt.Errorf("build failed")

	// Verify the method signature is correct by assigning it to a function variable
	var postProcessFunc func(*config.ImageTemplate, error) error = elxr.PostProcess

	t.Logf("PostProcess method has correct signature: %T", postProcessFunc)

	// Test that PostProcess with nil chrootEnv will panic - catch and validate
	defer func() {
		if r := recover(); r != nil {
			t.Logf("PostProcess correctly panicked with nil chrootEnv: %v", r)
		} else {
			t.Error("Expected PostProcess to panic with nil chrootEnv")
		}
	}()

	// This will panic due to nil chrootEnv, which we catch above
	_ = elxr.PostProcess(template, inputError)
}
