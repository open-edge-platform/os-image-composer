package debutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/open-edge-platform/image-composer/internal/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/provider"
)

// ResolvePackageInfos takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolvePackageInfos(requested []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	// Build a map for fast lookup by package name
	byName := make(map[string]provider.PackageInfo, len(all))
	// Build a map for lookup by provided virtual package name
	byProvides := make(map[string]provider.PackageInfo)
	for _, pi := range all {
		byName[pi.Name] = pi
		for _, prov := range pi.Provides {
			byProvides[prov] = pi
		}
	}

	// Track which packages we've already added
	neededSet := make(map[string]struct{})
	// Start with the requested packages
	queue := make([]provider.PackageInfo, 0, len(requested))
	for _, pi := range requested {
		if _, ok := byName[pi.Name]; !ok {
			// Try to resolve via Provides
			if provPkg, ok := byProvides[pi.Name]; ok {
				queue = append(queue, provPkg)
				continue
			}
			return nil, fmt.Errorf("requested package %q not in repo listing", pi.Name)
		}
		queue = append(queue, pi)
	}

	result := make([]provider.PackageInfo, 0)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if _, seen := neededSet[cur.Name]; seen {
			continue
		}
		neededSet[cur.Name] = struct{}{}
		result = append(result, cur)

		// Traverse dependencies (Requires)
		for _, dep := range cur.Requires {
			// Remove version constraints if present, e.g. "foo (>= 1.2)" -> "foo"
			depName := dep
			if idx := strings.Index(dep, " "); idx > 0 {
				depName = dep[:idx]
			}
			// Remove architecture qualifiers, e.g. "perl:any" -> "perl"
			if idx := strings.Index(depName, ":"); idx > 0 {
				depName = depName[:idx]
			}
			depName = strings.TrimSpace(depName)
			if depName == "" {
				continue
			}
			if _, seen := neededSet[depName]; seen {
				continue
			}
			if depPkg, ok := byName[depName]; ok {
				queue = append(queue, depPkg)
			} else if provPkg, ok := byProvides[depName]; ok {
				queue = append(queue, provPkg)
			} else {
				return nil, fmt.Errorf("dependency %q required by %q not found in repo", depName, cur.Name)
			}
		}
	}

	// Sort result by package name for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// ParsePrimary parses the Packages.gz file from gzHref.
func ParsePrimary(baseURL string, pkggz string, releaseFile string, releaseSign string, pbGPGKey string, buildPath string) ([]provider.PackageInfo, error) {
	logger := zap.L().Sugar()

	// Ensure pkgMetaDir exists, create if not
	// pkgMetaDir := "./builds/elxr12"
	pkgMetaDir := buildPath
	if err := os.MkdirAll(pkgMetaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pkgMetaDir: %v", err)
	}

	//verify release file
	localPkggzFile := filepath.Join(pkgMetaDir, filepath.Base(pkggz))
	localReleaseFile := filepath.Join(pkgMetaDir, filepath.Base(releaseFile))
	localReleaseSign := filepath.Join(pkgMetaDir, filepath.Base(releaseSign))
	localPBGPGKey := filepath.Join(pkgMetaDir, filepath.Base(pbGPGKey))

	// Remove any existing local files to ensure fresh downloads
	localFiles := []string{localPkggzFile, localReleaseFile, localReleaseSign, localPBGPGKey}
	for _, f := range localFiles {
		if _, err := os.Stat(f); err == nil {
			if remErr := os.Remove(f); remErr != nil {
				return nil, fmt.Errorf("failed to remove old file %s: %v", f, remErr)
			}
		}
	}

	// Download the debian repo files
	err := pkgfetcher.FetchPackages([]string{pkggz, releaseFile, releaseSign, pbGPGKey}, pkgMetaDir, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch critical repo config packages: %v", err)
	}
	// Verify the release file
	relVryResult, err := VerifyRelease(localReleaseFile, localReleaseSign, localPBGPGKey)
	if err != nil {
		return nil, fmt.Errorf("failed to verify release file: %v", err)
	}
	if !relVryResult {
		return nil, fmt.Errorf("release file verification failed")
	}

	// verify the sham256 checksum of the Packages.gz file
	pkggzVryResult, err := VerifyPackagegz(localReleaseFile, localPkggzFile)
	if err != nil {
		return nil, fmt.Errorf("failed to verify pkg file: %v", err)
	}
	if !pkggzVryResult {
		return nil, fmt.Errorf("package file verification failed")
	}

	// Getting sha256sum of the Packages.gz file from the release file
	// and verifying it with the local Packages.gz file
	// localPkgMetaFile := filepath.Join(pkgMetaDir, filepath.Base(pkggz))
	localPkgMetaFile := filepath.Join(pkgMetaDir, "Packages.gz")
	logger.Infof("localPkgMetaFile: %s", localPkgMetaFile)

	//Decompress the Packages.gz file
	// The decompressed file will be named Packages (without .gz)
	PkgMetaFile := pkgMetaDir + "/Packages.gz"
	pkgMetaFileNoExt := filepath.Join(filepath.Dir(PkgMetaFile), strings.TrimSuffix(filepath.Base(PkgMetaFile), filepath.Ext(PkgMetaFile)))

	files, err := Decompress(PkgMetaFile, pkgMetaFileNoExt)
	if err != nil {
		return []provider.PackageInfo{}, err
	}
	logger.Infof("decompressed files: %v", files)

	//Parse the decompressed file
	f, err := os.Open(files[0])
	if err != nil {
		return nil, fmt.Errorf("failed to open decompressed file: %v", err)
	}
	defer f.Close()

	var pkgs []provider.PackageInfo
	pkg := provider.PackageInfo{}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading file: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// End of one package entry
			if pkg.Name != "" {
				pkgs = append(pkgs, pkg)
				pkg = provider.PackageInfo{}
			}
			if err == io.EOF {
				break
			}
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			if err == io.EOF {
				break
			}
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "Package":
			pkg.Name = val
		case "Depends":
			// Split dependencies by comma and trim spaces
			deps := strings.Split(val, ",")
			for i := range deps {
				deps[i] = strings.TrimSpace(deps[i])
			}
			pkg.Requires = deps
		case "Provides":
			// Split provides by comma and trim spaces, remove version constraints
			deps := strings.Split(val, ",")
			for i := range deps {
				dep := strings.TrimSpace(deps[i])
				// Remove version constraints, e.g. "foo (= 1.2)" -> "foo"
				if idx := strings.Index(dep, " "); idx > 0 {
					dep = dep[:idx]
				}
				deps[i] = dep
			}
			pkg.Provides = deps
		case "Filename":
			pkg.URL, _ = getFullUrl(val, baseURL)
		case "SHA256":
			pkg.Checksum = val
			// Add more fields as needed
		}
		if err == io.EOF {
			break
		}
	}

	// Add the last package if file doesn't end with a blank line
	if pkg.Name != "" {
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
}

func getFullUrl(filePath string, baseUrl string) (string, error) {
	// Check if the file path is already a full URL
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return filePath, nil
	}

	// If not, construct the full URL using the base URL
	fullURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(baseUrl, "/"), filePath)
	return fullURL, nil
}
