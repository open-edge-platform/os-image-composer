package debutils

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/network"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

// GenerateSPDXFileName creates a SPDX manifest filename based on repository configuration
func GenerateSPDXFileName(repoNm string) string {
	timestamp := time.Now().Format("20060102_150405")
	SPDXFileNm := filepath.Join("spdx_manifest_deb_" + strings.ReplaceAll(repoNm, " ", "_") + "_" + timestamp + ".json")
	return SPDXFileNm
}

// CreateTemporaryRepository creates a temporary Debian repository from a source directory containing .deb files
// Returns: repository path, HTTP server URL, cleanup function, and error
func CreateTemporaryRepository(sourcePath, repoName string) (repoPath, serverURL string, cleanup func(), err error) {
	log := logger.Logger()

	// Validate input path
	sourcePath, err = filepath.Abs(sourcePath)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get absolute path of source directory: %w", err)
	}

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", "", nil, fmt.Errorf("source directory does not exist: %s", sourcePath)
	}

	// Check if source contains DEB files
	pattern := filepath.Join(sourcePath, "*.deb")
	debFiles, err := filepath.Glob(pattern)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to search for DEB files in %s: %w", sourcePath, err)
	}
	if len(debFiles) == 0 {
		return "", "", nil, fmt.Errorf("no DEB files found in source directory: %s", sourcePath)
	}

	log.Infof("found %d DEB files in source directory: %s", len(debFiles), sourcePath)

	// Create temporary repository directory with Debian structure
	tempRepoPath := filepath.Join("/tmp", fmt.Sprintf("debrepo_%s_%d", repoName, time.Now().Unix()))
	if err := os.MkdirAll(tempRepoPath, 0755); err != nil {
		return "", "", nil, fmt.Errorf("failed to create temporary repository directory: %w", err)
	}

	// Create pool/main subdirectory for proper Debian repository structure
	poolPath := filepath.Join(tempRepoPath, "pool", "main")
	if err := os.MkdirAll(poolPath, 0755); err != nil {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to create pool directory: %w", err)
	}

	// Create dists/stable/main/binary-amd64 subdirectory for metadata
	distsPath := filepath.Join(tempRepoPath, "dists", "stable", "main", "binary-amd64")
	if err := os.MkdirAll(distsPath, 0755); err != nil {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to create dists directory: %w", err)
	}

	log.Infof("created temporary repository directory: %s", tempRepoPath)

	// Copy all DEB files from source to pool/main directory
	copyCmd := fmt.Sprintf("cp %s/*.deb %s/", sourcePath, poolPath)
	if _, err := shell.ExecCmd(copyCmd, false, shell.HostPath, nil); err != nil {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to copy DEB files to temporary repository: %w", err)
	}

	log.Infof("copied DEB files from %s to %s", sourcePath, poolPath)

	// Generate Packages file using dpkg-scanpackages
	packagesPath := filepath.Join(distsPath, "Packages")
	// Use absolute paths for dpkg-scanpackages command
	poolRelativePath := "pool/main"
	scanPackagesCmd := fmt.Sprintf("cd %s && dpkg-scanpackages %s /dev/null > %s",
		tempRepoPath, poolRelativePath, packagesPath)

	output, err := shell.ExecCmd(scanPackagesCmd, false, shell.HostPath, nil)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to create Packages file: %w", err)
	}

	log.Debugf("dpkg-scanpackages output: %s", output)

	// Create Release file
	releasePath := filepath.Join(tempRepoPath, "dists", "stable", "Release")
	releaseContent := fmt.Sprintf(`Suite: stable
Codename: stable
Components: main
Architectures: amd64
Date: %s
`, time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"))

	if err := os.WriteFile(releasePath, []byte(releaseContent), 0644); err != nil {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to create Release file: %w", err)
	}

	log.Infof("generated repository metadata for %s", tempRepoPath)

	// Verify that the repository structure was created correctly
	if _, err := os.Stat(packagesPath); os.IsNotExist(err) {
		// Clean up on failure
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("repository metadata was not created properly: missing %s", packagesPath)
	}

	log.Infof("verified repository metadata exists: %s", packagesPath)

	// Start HTTP server to serve the repository
	serverURL, serverCleanup, err := network.ServeRepositoryHTTP(tempRepoPath)
	if err != nil {
		// Clean up repository if server fails
		os.RemoveAll(tempRepoPath)
		return "", "", nil, fmt.Errorf("failed to serve repository via HTTP: %w", err)
	}

	// Combined cleanup function
	cleanup = func() {
		serverCleanup()            // Stop HTTP server first
		os.RemoveAll(tempRepoPath) // Then remove repository directory
	}

	// Verify HTTP server is working by fetching Packages file
	packagesURL := serverURL + "/dists/stable/main/binary-amd64/Packages"
	log.Infof("verifying HTTP server by fetching: %s", packagesURL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(packagesURL)
	if err != nil {
		// Clean up if verification fails
		cleanup()
		return "", "", nil, fmt.Errorf("failed to verify HTTP server - could not fetch Packages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Clean up if verification fails
		cleanup()
		return "", "", nil, fmt.Errorf("failed to verify HTTP server - Packages returned status %d", resp.StatusCode)
	}

	log.Infof("HTTP server verification successful - Packages accessible at %s", packagesURL)
	log.Infof("successfully created and serving temporary DEB repository: %s", tempRepoPath)

	return tempRepoPath, serverURL, cleanup, nil
}
