package debian13

import (
	"fmt"
	"testing"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/provider"
	"github.com/open-edge-platform/os-image-composer/internal/utils/system"
)

// Helper function to create a test ImageTemplate for debian13
func createTestImageTemplate() *config.ImageTemplate {
	return &config.ImageTemplate{
		Image: config.ImageInfo{
			Name:    "test-debian-image",
			Version: "13.0.0",
		},
		Target: config.TargetInfo{
			OS:        "debian",
			Dist:      "debian13",
			Arch:      "amd64",
			ImageType: "raw",
		},
		SystemConfig: config.SystemConfig{
			Name:        "test-debian-system",
			Description: "Test Debian system configuration",
			Packages:    []string{"curl", "wget", "vim"},
		},
	}
}

// TestDebian13ProviderInterface tests that debian13 implements Provider interface
func TestDebian13ProviderInterface(t *testing.T) {
	var _ provider.Provider = (*debian13)(nil) // Compile-time interface check
}

// TestDebian13ProviderName tests the Name method
func TestDebian13ProviderName(t *testing.T) {
	provider := &debian13{}
	name := provider.Name("debian13", "amd64")
	expected := "debian-debian13-amd64"

	if name != expected {
		t.Errorf("Expected name %s, got %s", expected, name)
	}
}

// TestGetProviderId tests the GetProviderId function for debian13
func TestGetProviderId(t *testing.T) {
	testCases := []struct {
		dist     string
		arch     string
		expected string
	}{
		{"debian13", "amd64", "debian-debian13-amd64"},
		{"debian13", "arm64", "debian-debian13-arm64"},
		{"debian13", "x86_64", "debian-debian13-x86_64"},
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

// TestBuildUserRepoList tests the buildUserRepoList function
func TestBuildUserRepoList(t *testing.T) {
	testCases := []struct {
		name     string
		input    []config.PackageRepository
		expected int
	}{
		{
			name:     "empty repositories",
			input:    []config.PackageRepository{},
			expected: 0,
		},
		{
			name: "valid repository",
			input: []config.PackageRepository{
				{
					Codename:  "testing",
					URL:       "http://example.com/debian",
					PKey:      "http://example.com/key.gpg",
					Component: "main",
				},
			},
			expected: 1,
		},
		{
			name: "filter placeholder repositories",
			input: []config.PackageRepository{
				{
					Codename: "valid",
					URL:      "http://example.com/debian",
				},
				{
					Codename: "placeholder",
					URL:      "<URL>",
				},
			},
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildUserRepoList(tc.input)
			if len(result) != tc.expected {
				t.Errorf("Expected %d repositories, got %d", tc.expected, len(result))
			}
		})
	}
}

// TestDebian13ProviderImageType tests that BuildImage handles different image types
func TestDebian13ProviderImageType(t *testing.T) {
	testCases := []struct {
		name      string
		imageType string
		shouldErr bool
	}{
		{"raw", "raw", false},
		{"img", "img", false},
		{"iso", "iso", false},
		{"unsupported", "vdi", true},
	}

	// Note: This is a unit test that doesn't require actual chroot initialization
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			template := createTestImageTemplate()
			template.Target.ImageType = tc.imageType
			provider := &debian13{}

			if template.Target.ImageType == "vdi" && tc.shouldErr {
				// This should fail with BuildImage
				err := provider.BuildImage(template)
				if err == nil {
					t.Errorf("Expected error for unsupported image type %s", tc.imageType)
				}
			}
		})
	}
}
