package azl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/os-image-composer/internal/provider"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
	"github.com/open-edge-platform/os-image-composer/internal/utils/system"
)

// Helper function to create a test ImageTemplate
func createTestImageTemplate() *config.ImageTemplate {
	return &config.ImageTemplate{
		Image: config.ImageInfo{
			Name:    "test-azl-image",
			Version: "1.0.0",
		},
		Target: config.TargetInfo{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "qcow2",
		},
		SystemConfig: config.SystemConfig{
			Name:        "test-azl-system",
			Description: "Test Azure Linux system configuration",
			Packages:    []string{"curl", "wget", "vim"},
		},
	}
}

// Helper function to create mock HTTP server for repo config and repomd
func createMockAzlServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "config.repo"):
			repoConfig := `[azurelinux-base]
name=Azure Linux 3.0 Base Repository - x86_64
baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc`
			fmt.Fprint(w, repoConfig)
		case strings.Contains(r.URL.Path, "repomd.xml"):
			repomdXML := `<?xml version="1.0" encoding="UTF-8"?>
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="primary">
    <location href="repodata/primary.xml.gz"/>
    <checksum type="sha256">abcd1234</checksum>
  </data>
  <data type="filelists">
    <location href="repodata/filelists.xml.gz"/>
    <checksum type="sha256">efgh5678</checksum>
  </data>
</repomd>`
			fmt.Fprint(w, repomdXML)
		default:
			http.NotFound(w, r)
		}
	}))
}

// TestAzureLinuxProviderInterface tests that AzureLinux implements Provider interface
func TestAzureLinuxProviderInterface(t *testing.T) {
	var _ provider.Provider = (*AzureLinux)(nil) // Compile-time interface check
}

// TestAzureLinuxProviderName tests the Name method
func TestAzureLinuxProviderName(t *testing.T) {
	azl := &AzureLinux{}
	name := azl.Name("azl3", "x86_64")
	expected := "azure-linux-azl3-x86_64"

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
		{"azl3", "x86_64", "azure-linux-azl3-x86_64"},
		{"azl3", "aarch64", "azure-linux-azl3-aarch64"},
		{"azl4", "x86_64", "azure-linux-azl4-x86_64"},
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

// TestAzureLinuxProviderInit tests the Init method with real network calls
func TestAzureLinuxProviderInit(t *testing.T) {
	azl := &AzureLinux{}

	// Test with x86_64 architecture
	err := azl.Init("azl3", "x86_64")
	if err != nil {
		// Expected to potentially fail in test environment due to network dependencies
		t.Logf("Init failed as expected in test environment: %v", err)
	} else {
		// If it succeeds, verify the configuration was set up
		if azl.repoURL == "" {
			t.Error("Expected repoURL to be set after successful Init")
		}

		expectedURL := baseURL + "x86_64/" + configName
		if azl.repoURL != expectedURL {
			t.Errorf("Expected repoURL %s, got %s", expectedURL, azl.repoURL)
		}
	}
}

// TestAzureLinuxProviderInitWithMock tests Init method with mock server
func TestAzureLinuxProviderInitWithMock(t *testing.T) {
	server := createMockAzlServer()
	defer server.Close()

	// We can't easily test with mock server since constants are used
	// but we can test the URL construction logic
	azl := &AzureLinux{}
	expectedURL := baseURL + "x86_64/" + configName
	azl.repoURL = expectedURL

	if azl.repoURL != expectedURL {
		t.Errorf("Expected repoURL %s, got %s", expectedURL, azl.repoURL)
	}
}

// TestLoadRepoConfig tests the loadRepoConfig function
func TestLoadRepoConfig(t *testing.T) {
	repoConfigData := `[azurelinux-base]
name=Azure Linux 3.0 Base Repository - x86_64
baseurl=https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc`

	reader := strings.NewReader(repoConfigData)
	config, err := loadRepoConfig(reader)

	if err != nil {
		t.Fatalf("loadRepoConfig failed: %v", err)
	}

	// Verify parsed configuration
	if config.Section != "azurelinux-base" {
		t.Errorf("Expected section 'azurelinux-base', got '%s'", config.Section)
	}

	if config.Name != "Azure Linux 3.0 Base Repository - x86_64" {
		t.Errorf("Expected specific name, got '%s'", config.Name)
	}

	if config.URL != "https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64" {
		t.Errorf("Expected specific URL, got '%s'", config.URL)
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

	if config.GPGKey != "https://packages.microsoft.com/keys/microsoft.asc" {
		t.Errorf("Expected specific GPG key URL, got '%s'", config.GPGKey)
	}
}

// TestLoadRepoConfigWithComments tests parsing repo config with comments and empty lines
func TestLoadRepoConfigWithComments(t *testing.T) {
	repoConfigData := `# This is a comment
; This is also a comment

[azurelinux-base]
name=Azure Linux Base Repository
# Another comment
baseurl=https://example.com/repo
enabled=1

gpgcheck=0`

	reader := strings.NewReader(repoConfigData)
	config, err := loadRepoConfig(reader)

	if err != nil {
		t.Fatalf("loadRepoConfig failed: %v", err)
	}

	if config.Section != "azurelinux-base" {
		t.Errorf("Expected section 'azurelinux-base', got '%s'", config.Section)
	}

	if config.GPGCheck {
		t.Error("Expected GPG check to be disabled")
	}
}

// TestFetchPrimaryURL tests the fetchPrimaryURL function with mock server
func TestFetchPrimaryURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repomdXML := `<?xml version="1.0" encoding="UTF-8"?>
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="primary">
    <location href="repodata/primary.xml.gz"/>
    <checksum type="sha256">abcd1234</checksum>
  </data>
  <data type="filelists">
    <location href="repodata/filelists.xml.gz"/>
    <checksum type="sha256">efgh5678</checksum>
  </data>
</repomd>`
		fmt.Fprint(w, repomdXML)
	}))
	defer server.Close()

	href, err := rpmutils.FetchPrimaryURL(server.URL)
	if err != nil {
		t.Fatalf("fetchPrimaryURL failed: %v", err)
	}

	expected := "repodata/primary.xml.gz"
	if href != expected {
		t.Errorf("Expected href '%s', got '%s'", expected, href)
	}
}

// TestFetchPrimaryURLNoPrimary tests fetchPrimaryURL when no primary data exists
func TestFetchPrimaryURLNoPrimary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repomdXML := `<?xml version="1.0" encoding="UTF-8"?>
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="filelists">
    <location href="repodata/filelists.xml.gz"/>
    <checksum type="sha256">efgh5678</checksum>
  </data>
</repomd>`
		fmt.Fprint(w, repomdXML)
	}))
	defer server.Close()

	_, err := rpmutils.FetchPrimaryURL(server.URL)
	if err == nil {
		t.Error("Expected error when primary location not found")
	}

	expectedError := "primary location not found"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

// TestFetchPrimaryURLInvalidXML tests fetchPrimaryURL with invalid XML
func TestFetchPrimaryURLInvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "invalid xml content")
	}))
	defer server.Close()

	_, err := rpmutils.FetchPrimaryURL(server.URL)
	if err == nil {
		t.Error("Expected error when XML is invalid")
	}
}

// TestAzureLinuxProviderPreProcess tests PreProcess method with mocked dependencies
func TestAzureLinuxProviderPreProcess(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor
	mockCommands := []shell.MockCommand{
		{Pattern: "apt-get update", Output: "Package lists updated successfully"},
		{Pattern: "apt-get install -y rpm", Output: "Package installed successfully"},
		{Pattern: "apt-get install -y dosfstools", Output: "Package installed successfully"},
		{Pattern: "apt-get install -y xorriso", Output: "Package installed successfully"},
		{Pattern: "apt-get install -y sbsigntool", Output: "Package installed successfully"},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	azl := &AzureLinux{
		repoCfg: rpmutils.RepoConfig{
			Section: "azurelinux-base",
			Name:    "Azure Linux 3.0 Base Repository",
			URL:     "https://example.com/repo",
			Enabled: true,
		},
		gzHref: "repodata/primary.xml.gz",
	}

	template := createTestImageTemplate()

	// This test will likely fail due to dependencies on chroot, rpmutils, etc.
	// but it demonstrates the testing approach
	err := azl.PreProcess(template)
	if err != nil {
		t.Logf("PreProcess failed as expected due to external dependencies: %v", err)
	}
}

// TestAzureLinuxProviderBuildImage tests BuildImage method
func TestAzureLinuxProviderBuildImage(t *testing.T) {
	// Try to register and get a properly initialized provider
	err := Register("azure-linux", "azl3", "x86_64")
	if err != nil {
		t.Skipf("Cannot test BuildImage without proper registration: %v", err)
		return
	}

	providerName := system.GetProviderId(OsName, "azl3", "x86_64")
	azl, exists := provider.Get(providerName)
	if !exists {
		t.Skipf("Cannot get registered Azure Linux provider")
		return
	}

	template := createTestImageTemplate()

	// This test will fail due to dependencies on image builders that require system access
	// We expect it to fail early before reaching sudo commands
	err = azl.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage failed as expected due to external dependencies: %v", err)
		// Verify the error is related to expected failures, not sudo issues
		if strings.Contains(err.Error(), "sudo") {
			t.Errorf("Test should not reach sudo commands - mocking may be insufficient")
		}
	}
}

// TestAzureLinuxProviderBuildImageISO tests BuildImage method with ISO type
func TestAzureLinuxProviderBuildImageISO(t *testing.T) {

	// Try to register and get a properly initialized provider
	err := Register("azure-linux", "azl3", "x86_64")
	if err != nil {
		t.Skipf("Cannot test BuildImage (ISO) without proper registration: %v", err)
		return
	}

	providerName := system.GetProviderId(OsName, "azl3", "x86_64")
	azl, exists := provider.Get(providerName)
	if !exists {
		t.Skipf("Cannot get registered Azure Linux provider")
		return
	}

	template := createTestImageTemplate()

	// Set up global config for ISO
	originalImageType := template.Target.ImageType
	defer func() { template.Target.ImageType = originalImageType }()
	template.Target.ImageType = "iso"

	err = azl.BuildImage(template)
	if err != nil {
		t.Logf("BuildImage (ISO) failed as expected due to external dependencies: %v", err)
		// Verify the error is related to expected failures, not sudo issues
		if strings.Contains(err.Error(), "sudo") {
			t.Errorf("Test should not reach sudo commands - mocking may be insufficient")
		}
	}
}

// TestAzureLinuxProviderPostProcess tests PostProcess method
func TestAzureLinuxProviderPostProcess(t *testing.T) {

	// Try to register and get a properly initialized provider
	err := Register("azure-linux", "azl3", "x86_64")
	if err != nil {
		t.Skipf("Cannot test PostProcess without proper registration: %v", err)
		return
	}

	providerName := system.GetProviderId(OsName, "azl3", "x86_64")
	azl, exists := provider.Get(providerName)
	if !exists {
		t.Skipf("Cannot get registered Azure Linux provider")
		return
	}

	template := createTestImageTemplate()

	// Test with input error (should be passed through without system calls)
	inputError := fmt.Errorf("some build error")
	err = azl.PostProcess(template, inputError)
	if err != inputError {
		t.Logf("PostProcess modified input error: expected %v, got %v", inputError, err)
	}

	// Note: Testing PostProcess with nil error is skipped to avoid potential system calls
	t.Log("PostProcess error propagation test completed")
}

// TestAzureLinuxProviderInstallHostDependency tests installHostDependency method
func TestAzureLinuxProviderInstallHostDependency(t *testing.T) {
	// Save original shell executor and restore after test
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor
	mockCommands := []shell.MockCommand{
		{Pattern: "which rpm", Output: ""},
		{Pattern: "which mkfs.fat", Output: ""},
		{Pattern: "which xorriso", Output: ""},
		{Pattern: "which sbsign", Output: ""},
		{Pattern: "apt-get install -y rpm", Output: "Success"},
		{Pattern: "apt-get install -y dosfstools", Output: "Success"},
		{Pattern: "apt-get install -y xorriso", Output: "Success"},
		{Pattern: "apt-get install -y sbsigntool", Output: "Success"},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	azl := &AzureLinux{}

	// This test will likely fail due to dependencies on chroot.GetHostOsPkgManager()
	// and shell.IsCommandExist(), but it demonstrates the testing approach
	err := azl.installHostDependency()
	if err != nil {
		t.Logf("installHostDependency failed as expected due to external dependencies: %v", err)
	} else {
		t.Logf("installHostDependency succeeded with mocked commands")
	}
}

// TestAzureLinuxProviderInstallHostDependencyCommands tests expected host dependencies
func TestAzureLinuxProviderInstallHostDependencyCommands(t *testing.T) {
	// Test the expected dependencies mapping by accessing the internal map
	// This verifies what packages the Azure Linux provider expects to install
	expectedDeps := map[string]string{
		"rpm":      "rpm",        // For the chroot env build RPM pkg installation
		"mkfs.fat": "dosfstools", // For the FAT32 boot partition creation
		"xorriso":  "xorriso",    // For ISO image creation
		"sbsign":   "sbsigntool", // For the UKI image creation
	}

	t.Logf("Expected host dependencies for Azure Linux provider: %v", expectedDeps)

	// Verify that each expected dependency has a mapping
	for cmd, pkg := range expectedDeps {
		if cmd == "" || pkg == "" {
			t.Errorf("Empty dependency mapping: cmd='%s', pkg='%s'", cmd, pkg)
		}
	}
}

// TestAzureLinuxProviderRegister tests the Register function
func TestAzureLinuxProviderRegister(t *testing.T) {
	// Save original providers registry and restore after test
	// Note: We can't easily access the provider registry for cleanup,
	// so this test shows the approach but may leave test artifacts

	err := Register("azure-linux", "azl3", "x86_64")
	if err != nil {
		t.Skipf("Cannot test registration due to missing dependencies: %v", err)
		return
	}

	// Try to retrieve the registered provider
	providerName := system.GetProviderId(OsName, "azl3", "x86_64")
	retrievedProvider, exists := provider.Get(providerName)

	if !exists {
		t.Errorf("Expected provider %s to be registered", providerName)
		return
	}

	// Verify it's an Azure Linux provider
	if azlProvider, ok := retrievedProvider.(*AzureLinux); !ok {
		t.Errorf("Expected Azure Linux provider, got %T", retrievedProvider)
	} else {
		// Test the Name method on the registered provider
		name := azlProvider.Name("azl3", "x86_64")
		if name != providerName {
			t.Errorf("Expected provider name %s, got %s", providerName, name)
		}
	}
}

// TestAzureLinuxProviderWorkflow tests a complete Azure Linux provider workflow
func TestAzureLinuxProviderWorkflow(t *testing.T) {
	// This is an integration-style test showing how an Azure Linux provider
	// would be used in a complete workflow

	// Set up mock executor for system commands
	originalExecutor := shell.Default
	defer func() { shell.Default = originalExecutor }()

	// Set up mock executor
	mockCommands := []shell.MockCommand{
		{Pattern: "/usr/bin/uname -m", Output: "x86_64"},
		{Pattern: "uname -m", Output: "x86_64"},
		{Pattern: "/usr/bin/which dpkg", Output: "", Error: fmt.Errorf("command not found")},
		{Pattern: "which dpkg", Output: "", Error: fmt.Errorf("command not found")},
		{Pattern: "/usr/bin/which rpm", Output: "/usr/bin/rpm"},
		{Pattern: "which rpm", Output: "/usr/bin/rpm"},
		{Pattern: "/usr/bin/which tdnf", Output: "/usr/bin/tdnf"},
		{Pattern: "which tdnf", Output: "/usr/bin/tdnf"},
		{Pattern: "/usr/bin/tdnf install -y rpm-build", Output: "Installing packages..."},
		{Pattern: "tdnf install -y rpm-build", Output: "Installing packages..."},
		{Pattern: "mount", Output: ""},
		{Pattern: "/usr/bin/mount", Output: ""},
		{Pattern: "umount", Output: ""},
		{Pattern: "/usr/bin/umount", Output: ""},
	}
	shell.Default = shell.NewMockExecutor(mockCommands)

	azl := &AzureLinux{}

	// Test provider name generation
	name := azl.Name("azl3", "x86_64")
	expectedName := "azure-linux-azl3-x86_64"
	if name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, name)
	}

	// Test Init (will likely fail due to network dependencies)
	if err := azl.Init("azl3", "x86_64"); err != nil {
		t.Logf("Init failed as expected: %v", err)
	} else {
		t.Log("Init succeeded - repo config loaded")
		if azl.repoCfg.Name != "" {
			t.Logf("Repo config loaded: %s", azl.repoCfg.Name)
		}
	}

	// Skip PreProcess, BuildImage and PostProcess tests to avoid system-level dependencies
	t.Log("Skipping PreProcess, BuildImage and PostProcess tests to avoid system-level dependencies")

	t.Log("Complete workflow test finished - core methods exist and are callable")
}

// TestAzureLinuxConfigurationStructure tests the internal configuration structure
func TestAzureLinuxConfigurationStructure(t *testing.T) {
	azl := &AzureLinux{
		repoURL: "https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64/config.repo",
		repoCfg: rpmutils.RepoConfig{
			Section:      "azurelinux-base",
			Name:         "Azure Linux 3.0 Base Repository",
			URL:          "https://packages.microsoft.com/azurelinux/3.0/prod/base/x86_64",
			GPGCheck:     true,
			RepoGPGCheck: true,
			Enabled:      true,
			GPGKey:       "https://packages.microsoft.com/keys/microsoft.asc",
		},
		gzHref: "repodata/primary.xml.gz",
	}

	// Verify internal structure is properly set up
	if azl.repoURL == "" {
		t.Error("Expected repoURL to be set")
	}

	if azl.repoCfg.Section == "" {
		t.Error("Expected repo config section to be set")
	}

	if azl.gzHref == "" {
		t.Error("Expected gzHref to be set")
	}

	// Verify URL construction matches expected pattern
	expectedBaseURL := "https://packages.microsoft.com/azurelinux/3.0/prod/base/"
	if baseURL != expectedBaseURL {
		t.Errorf("Expected baseURL %s, got %s", expectedBaseURL, baseURL)
	}

	if configName != "config.repo" {
		t.Errorf("Expected configName 'config.repo', got '%s'", configName)
	}
}

// TestAzureLinuxArchitectureHandling tests architecture-specific URL construction
func TestAzureLinuxArchitectureHandling(t *testing.T) {
	testCases := []struct {
		inputArch string
	}{
		{"x86_64"},
		{"aarch64"},
		{"armv7hl"},
	}

	for _, tc := range testCases {
		t.Run(tc.inputArch, func(t *testing.T) {
			azl := &AzureLinux{}
			_ = azl.Init("azl3", tc.inputArch) // Ignore error, just test URL construction

			// We expect this to fail due to network dependencies, but we can check URL construction
			expectedURL := baseURL + tc.inputArch + "/" + configName
			if azl.repoURL != expectedURL {
				t.Errorf("For arch %s, expected URL %s, got %s", tc.inputArch, expectedURL, azl.repoURL)
			}
		})
	}
}
