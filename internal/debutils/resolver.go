package debutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/provider"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// ResolvePackageInfos takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolvePackageInfos(requested []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	// Build a map for fast lookup by package name and version
	byNameVer := make(map[string]provider.PackageInfo, len(all))
	byName := make(map[string]provider.PackageInfo, len(all))
	byProvides := make(map[string]provider.PackageInfo)
	for _, pi := range all {
		key := pi.Name
		if pi.Version != "" {
			// contruct key as "name=version" for exact matches
			key = fmt.Sprintf("%s=%s", pi.Name, pi.Version)
			byNameVer[key] = pi
		}
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
		key := pi.Name
		if pi.Version != "" {
			key = fmt.Sprintf("%s=%s", pi.Name, pi.Version)
			if pkg, ok := byNameVer[key]; ok {
				queue = append(queue, pkg)
				continue
			}
		}
		if pkg, ok := byName[pi.Name]; ok {
			queue = append(queue, pkg)
			continue
		}
		if provPkg, ok := byProvides[pi.Name]; ok {
			queue = append(queue, provPkg)
			continue
		}
		return nil, fmt.Errorf("requested package %q not in repo listing", pi.Name)
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
			// dep may be "foo (>= 1.2)" or "foo (= 1.2)" or just "foo"
			depName := dep
			depVersion := ""
			// handles "|" in package names, e.g. "perl | perl-base"
			// currently only the first part is used, TODO: handle multiple parts
			if idx := strings.Index(depName, "|"); idx > 0 {
				depName = strings.TrimSpace(depName[:idx])
			}
			// Remove architecture qualifiers, e.g. "perl:any" -> "perl"
			if idx := strings.Index(depName, ":"); idx > 0 {
				depName = depName[:idx]
			}
			// Check for version constraint
			if idx := strings.Index(depName, "("); idx > 0 {
				name := strings.TrimSpace(depName[:idx])
				verPart := strings.TrimSpace(depName[idx:])
				verPart = strings.Trim(verPart, "() ")
				// Only enforce exact version if constraint is "="
				if strings.HasPrefix(verPart, "=") {
					depVersion = strings.TrimSpace(strings.TrimPrefix(verPart, "="))
					depName = name
				} else {
					depName = name
				}
			} else if idx := strings.Index(depName, " "); idx > 0 {
				depName = depName[:idx]
			}
			depName = strings.TrimSpace(depName)
			if depName == "" {
				continue
			}
			if _, seen := neededSet[depName]; seen {
				continue
			}
			// If version is enforced, match by name+version
			if depVersion != "" {
				// Try exact version first
				key := fmt.Sprintf("%s=%s", depName, depVersion)
				if depPkg, ok := byNameVer[key]; ok {
					// exact version match found
					queue = append(queue, depPkg)
					continue
				}
				// Try to find any package with higher version using Debian version semantics
				var found *provider.PackageInfo
				for _, pi := range all {
					if pi.Name == depName {
						// Use Debian version comparison
						cmp, err := compareDebianVersions(pi.Version, depVersion)
						if err != nil {
							return nil, fmt.Errorf("failed to compare versions: %v", err)
						}
						if cmp >= 0 {
							// sorting by version, pick the lowest one that satisfies the constraint
							if found == nil {
								tmp := pi
								found = &tmp
							} else {
								// Pick the lowest version that satisfies the constraint
								cmp2, err := compareDebianVersions(pi.Version, found.Version)
								if err != nil {
									return nil, fmt.Errorf("failed to compare versions: %v", err)
								}
								if cmp2 < 0 {
									tmp := pi
									found = &tmp
								}
							}
						}
					}
				}
				if found != nil {
					queue = append(queue, *found)
					continue
				}
				return nil, fmt.Errorf("dependency %q (version %q or higher) required by %q not found in repo", depName, depVersion, cur.Name)
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
	logger := utils.Logger()

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
		case "Version":
			pkg.Version = val
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

// compareDebianVersions compares two Debian version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareDebianVersions(a, b string) (int, error) {
	// Helper to split epoch
	splitEpoch := func(ver string) (epoch int, rest string) {
		parts := strings.SplitN(ver, ":", 2)
		if len(parts) == 2 {
			fmt.Sscanf(parts[0], "%d", &epoch)
			rest = parts[1]
		} else {
			epoch = 0
			rest = ver
		}
		return
	}

	// Helper to get next segment (numeric or non-numeric)
	nextSegment := func(s string) (seg string, rest string, numeric bool) {
		if s == "" {
			return "", "", false
		}
		if s[0] >= '0' && s[0] <= '9' {
			i := 0
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			return s[:i], s[i:], true
		}
		i := 0
		for i < len(s) && (s[i] < '0' || s[i] > '9') {
			i++
		}
		return s[:i], s[i:], false
	}

	// Handle epoch
	epochA, restA := splitEpoch(a)
	epochB, restB := splitEpoch(b)
	if epochA < epochB {
		return -1, nil
	}
	if epochA > epochB {
		return 1, nil
	}

	// Compare the rest
	sa, sb := restA, restB
	for sa != "" || sb != "" {
		// Handle tilde (~)
		if len(sa) > 0 && sa[0] == '~' {
			if len(sb) == 0 || sb[0] != '~' {
				return -1, nil
			}
			sa = sa[1:]
			sb = sb[1:]
			continue
		}
		if len(sb) > 0 && sb[0] == '~' {
			return 1, nil
		}

		segA, restA, numA := nextSegment(sa)
		segB, restB, numB := nextSegment(sb)

		if segA == "" && segB == "" {
			sa, sb = restA, restB
			continue
		}

		if numA && numB {
			// Remove leading zeros
			segA = strings.TrimLeft(segA, "0")
			segB = strings.TrimLeft(segB, "0")
			// Compare by length
			if len(segA) > len(segB) {
				return 1, nil
			}
			if len(segA) < len(segB) {
				return -1, nil
			}
			// Compare lexicographically
			if segA > segB {
				return 1, nil
			}
			if segA < segB {
				return -1, nil
			}
		} else if !numA && !numB {
			if segA > segB {
				return 1, nil
			}
			if segA < segB {
				return -1, nil
			}
		} else {
			// Numeric segments are always less than non-numeric
			if numA {
				return -1, nil
			}
			return 1, nil
		}
		sa, sb = restA, restB
	}
	return 0, nil
}
