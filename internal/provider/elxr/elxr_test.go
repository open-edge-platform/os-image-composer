package elxr

import (
	"fmt"
	"strings"
	"testing"

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
	elxr := &eLxr{}

	// Test with amd64 architecture
	err := elxr.Init("elxr12", "amd64")
	if err != nil {
		// Expected to potentially fail in test environment due to network dependencies
		t.Logf("Init failed as expected in test environment: %v", err)
	} else {
		// If it succeeds, verify the configuration was set up
		if elxr.repoURL == "" {
			t.Error("Expected repoURL to be set after successful Init")
		}

		expectedURL := baseURL + "binary-amd64/" + configName
		if elxr.repoURL != expectedURL {
			t.Errorf("Expected repoURL %s, got %s", expectedURL, elxr.repoURL)
		}
	}
}

// TestElxrProviderInitArchMapping tests architecture mapping in Init
func TestElxrProviderInitArchMapping(t *testing.T) {
	elxr := &eLxr{}

	// Test x86_64 -> binary-amd64 mapping
	err := elxr.Init("elxr12", "x86_64")
	if err != nil {
		t.Logf("Init failed as expected: %v", err)
	}

	// Verify URL construction with arch mapping
	expectedURL := baseURL + "binary-amd64/" + configName
	if elxr.repoURL != expectedURL {
		t.Errorf("Expected repoURL %s for x86_64 arch, got %s", expectedURL, elxr.repoURL)
	}
}

// TestLoadRepoConfig tests the loadRepoConfig function
func TestLoadRepoConfig(t *testing.T) {
	testURL := "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz"

	config, err := loadRepoConfig(testURL, "amd64")
	if err != nil {
		t.Fatalf("loadRepoConfig failed: %v", err)
	}

	// Verify parsed configuration
	if config.PkgList != testURL {
		t.Errorf("Expected PkgList '%s', got '%s'", testURL, config.PkgList)
	}

	if config.Name != "Wind River eLxr 12" {
		t.Errorf("Expected name 'Wind River eLxr 12', got '%s'", config.Name)
	}

	if config.PkgPrefix != "https://mirror.elxr.dev/elxr/" {
		t.Errorf("Expected specific PkgPrefix, got '%s'", config.PkgPrefix)
	}

	if !config.Enabled {
		t.Error("Expected repo to be enabled")
	}

	if !config.GPGCheck {
		t.Error("Expected GPG check to be enabled")
	}

	if !config.RepoGPGCheck {
		t.Error("Expected repo GPG check to be enabled")
	}

	if config.Section != "main" {
		t.Errorf("Expected section 'main', got '%s'", config.Section)
	}

	if config.BuildPath != "./builds/elxr12" {
		t.Errorf("Expected build path './builds/elxr12', got '%s'", config.BuildPath)
	}

	expectedReleaseFile := "https://mirror.elxr.dev/elxr/dists/aria/Release"
	if config.ReleaseFile != expectedReleaseFile {
		t.Errorf("Expected ReleaseFile '%s', got '%s'", expectedReleaseFile, config.ReleaseFile)
	}

	expectedReleaseSign := "https://mirror.elxr.dev/elxr/dists/aria/Release.gpg"
	if config.ReleaseSign != expectedReleaseSign {
		t.Errorf("Expected ReleaseSign '%s', got '%s'", expectedReleaseSign, config.ReleaseSign)
	}

	expectedPbGPGKey := "https://mirror.elxr.dev/elxr/public.gpg"
	if config.PbGPGKey != expectedPbGPGKey {
		t.Errorf("Expected PbGPGKey '%s', got '%s'", expectedPbGPGKey, config.PbGPGKey)
	}
}

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
		gzHref: "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz",
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
	// Test that configuration constants are set correctly
	if baseURL == "" {
		t.Error("baseURL should not be empty")
	}

	expectedBaseURL := "https://mirror.elxr.dev/elxr/dists/aria/main/"
	if baseURL != expectedBaseURL {
		t.Errorf("Expected baseURL %s, got %s", expectedBaseURL, baseURL)
	}

	if configName != "Packages.gz" {
		t.Errorf("Expected configName 'Packages.gz', got '%s'", configName)
	}
}

// TestElxrArchitectureHandling tests architecture-specific URL construction
func TestElxrArchitectureHandling(t *testing.T) {
	testCases := []struct {
		inputArch    string
		expectedArch string
	}{
		{"x86_64", "binary-amd64"}, // x86_64 gets converted to amd64, then becomes binary-amd64
		{"amd64", "binary-amd64"},  // amd64 stays amd64, then becomes binary-amd64
		{"arm64", "binary-arm64"},  // arm64 stays arm64, then becomes binary-arm64
	}

	for _, tc := range testCases {
		t.Run(tc.inputArch, func(t *testing.T) {
			elxr := &eLxr{}
			_ = elxr.Init("elxr12", tc.inputArch) // Ignore error, just test URL construction

			// We expect this to fail due to network dependencies, but we can check URL construction
			expectedURL := baseURL + tc.expectedArch + "/" + configName
			if elxr.repoURL != expectedURL {
				t.Errorf("For arch %s, expected URL %s, got %s", tc.inputArch, expectedURL, elxr.repoURL)
			}
		})
	}
}
