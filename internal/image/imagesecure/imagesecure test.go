package imagesecure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-edge-platform/image-composer/internal/config"
)

func TestConfigImageSecurity(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Define a sample image template with correct structure
	template := &config.ImageTemplate{
		Image: config.ImageInfo{
			Name:    "test-image",
			Version: "1.0.0",
		},
		Target: config.TargetInfo{
			OS:        "azure-linux",
			Dist:      "azl3",
			Arch:      "x86_64",
			ImageType: "raw",
		},
	}

	// Create the etc directory and a sample fstab file
	etcDir := filepath.Join(tempDir, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatalf("Failed to create etc directory: %v", err)
	}

	fstabPath := filepath.Join(etcDir, "fstab")
	// Create a sample fstab with a root filesystem entry
	sampleFstab := `# /etc/fstab: static file system information.
#
# <file system> <mount point>   <type>  <options>       <dump>  <pass>
UUID=12345678-1234-1234-1234-123456789012 /               ext4    defaults        1       1
UUID=87654321-4321-4321-4321-210987654321 /boot           ext4    defaults        1       2
`
	if err := os.WriteFile(fstabPath, []byte(sampleFstab), 0644); err != nil {
		t.Fatalf("Failed to create sample fstab file: %v", err)
	}

	// Call the function to configure image security
	err := ConfigImageSecurity(tempDir, template)
	if err != nil {
		t.Fatalf("ConfigImageSecurity failed: %v", err)
	}

	// Check if the fstab file still exists
	if _, err := os.Stat(fstabPath); os.IsNotExist(err) {
		t.Errorf("Expected fstab file to exist at %s, but it does not exist", fstabPath)
		return
	}

	// Read the modified fstab and verify it contains 'ro' option for root filesystem
	modifiedContent, err := os.ReadFile(fstabPath)
	if err != nil {
		t.Fatalf("Failed to read modified fstab file: %v", err)
	}

	fstabLines := strings.Split(string(modifiedContent), "\n")
	foundRootWithRO := false
	for _, line := range fstabLines {
		if strings.Contains(line, "/") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[1] == "/" {
				// This is the root filesystem entry
				options := fields[3]
				if strings.Contains(options, "ro") {
					foundRootWithRO = true
					break
				}
			}
		}
	}

	if !foundRootWithRO {
		t.Errorf("Expected root filesystem entry to have 'ro' option in fstab, but it was not found")
		t.Logf("Modified fstab content:\n%s", string(modifiedContent))
	}
}
