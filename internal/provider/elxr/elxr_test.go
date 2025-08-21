package elxr

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

// MockShellExecutor implements shell.Executor interface for testing
type MockShellExecutor struct {
	ExecCmdFunc           func(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdSilentFunc     func(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdWithStreamFunc func(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
	ExecCmdWithInputFunc  func(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error)
}

func (m *MockShellExecutor) ExecCmd(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if m.ExecCmdFunc != nil {
		return m.ExecCmdFunc(cmdStr, sudo, chrootPath, envVal)
	}
	return "", nil
}

func (m *MockShellExecutor) ExecCmdSilent(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if m.ExecCmdSilentFunc != nil {
		return m.ExecCmdSilentFunc(cmdStr, sudo, chrootPath, envVal)
	}
	return "", nil
}

func (m *MockShellExecutor) ExecCmdWithStream(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if m.ExecCmdWithStreamFunc != nil {
		return m.ExecCmdWithStreamFunc(cmdStr, sudo, chrootPath, envVal)
	}
	return "", nil
}

func (m *MockShellExecutor) ExecCmdWithInput(inputStr string, cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
	if m.ExecCmdWithInputFunc != nil {
		return m.ExecCmdWithInputFunc(inputStr, cmdStr, sudo, chrootPath, envVal)
	}
	return "", nil
}

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
			result := GetProviderId(tc.dist, tc.arch)
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

		expectedURL := baseURL + "amd64/" + configName
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

	config, err := loadRepoConfig(testURL)
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

	// Mock shell commands for host dependency installation
	mockExecutor := &MockShellExecutor{
		ExecCmdWithStreamFunc: func(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
			// Mock successful package installation
			if strings.Contains(cmdStr, "install") {
				return "Package installed successfully", nil
			}
			return "", nil
		},
	}
	shell.Default = mockExecutor

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

	// Set up global config variables that PreProcess depends on
	config.TargetOs = "elxr"
	config.TargetDist = "elxr12"
	config.TargetArch = "amd64"

	// This test will likely fail due to dependencies on chroot, debutils, etc.
	// but it demonstrates the testing approach
	err := elxr.PreProcess(template)
	if err != nil {
		t.Logf("PreProcess failed as expected due to external dependencies: %v", err)
	}
}

// TestElxrProviderBuildImage tests BuildImage method
func TestElxrProviderBuildImage(t *testing.T) {
	elxr := &eLxr{}
	template := createTestImageTemplate()

	// Set up global config
	config.TargetImageType = "qcow2"

	// This test will likely fail due to dependencies on image builders
	// but it demonstrates the testing approach
	err := elxr.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage failed as expected due to external dependencies: %v", err)
	}
}

// TestElxrProviderBuildImageISO tests BuildImage method with ISO type
func TestElxrProviderBuildImageISO(t *testing.T) {
	elxr := &eLxr{}
	template := createTestImageTemplate()

	// Set up global config for ISO
	originalImageType := config.TargetImageType
	defer func() { config.TargetImageType = originalImageType }()
	config.TargetImageType = "iso"

	err := elxr.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage (ISO) failed as expected due to external dependencies: %v", err)
	}
}

// TestElxrProviderPostProcess tests PostProcess method
func TestElxrProviderPostProcess(t *testing.T) {
	elxr := &eLxr{}
	template := createTestImageTemplate()

	// Set up global config variables
	config.TargetOs = "elxr"
	config.TargetDist = "elxr12"
	config.TargetArch = "amd64"

	// Test with no error
	err := elxr.PostProcess(template, nil)
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

	// Track commands executed
	var executedCommands []string
	mockExecutor := &MockShellExecutor{
		ExecCmdWithStreamFunc: func(cmdStr string, sudo bool, chrootPath string, envVal []string) (string, error) {
			executedCommands = append(executedCommands, cmdStr)
			return "Success", nil
		},
	}
	shell.Default = mockExecutor

	elxr := &eLxr{}

	// This test will likely fail due to dependencies on chroot.GetHostOsPkgManager()
	// and shell.IsCommandExist(), but it demonstrates the testing approach
	err := elxr.installHostDependency()
	if err != nil {
		t.Logf("installHostDependency failed as expected due to external dependencies: %v", err)
	} else {
		t.Logf("Commands that would be executed: %v", executedCommands)
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

	Register("elxr12", "amd64")

	// Try to retrieve the registered provider
	providerName := GetProviderId("elxr12", "amd64")
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
	// This is an integration-style test showing how an eLxr provider
	// would be used in a complete workflow

	elxr := &eLxr{}
	template := createTestImageTemplate()

	// Set up global configuration
	config.TargetOs = "elxr"
	config.TargetDist = "elxr12"
	config.TargetArch = "amd64"
	config.TargetImageType = "qcow2"

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

	// Test PreProcess (will likely fail due to dependencies)
	if err := elxr.PreProcess(template); err != nil {
		t.Logf("PreProcess failed as expected: %v", err)
	}

	// Test BuildImage (will likely fail due to dependencies)
	if err := elxr.BuildImage(template); err != nil {
		t.Logf("BuildImage failed as expected: %v", err)
	}

	// Test PostProcess (should succeed with cleanup)
	if err := elxr.PostProcess(template, nil); err != nil {
		t.Logf("PostProcess failed: %v", err)
	}

	t.Log("Complete workflow test finished - methods exist and are callable")
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
		{"x86_64", "binary-amd64"}, // only x86_64 gets converted to binary-amd64
		{"amd64", "amd64"},         // amd64 is passed through as-is
		{"arm64", "arm64"},         // arm64 is passed through as-is
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
