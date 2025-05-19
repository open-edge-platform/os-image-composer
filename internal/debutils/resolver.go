package debutils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
)

// ResolvePackageInfos takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolvePackageInfos(requested []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	// Build a map for fast lookup by package name
	byName := make(map[string]provider.PackageInfo, len(all))
	for _, pi := range all {
		byName[pi.Name] = pi
	}

	// Track which packages we've already added
	neededSet := make(map[string]struct{})
	// Start with the requested packages
	queue := make([]provider.PackageInfo, 0, len(requested))
	for _, pi := range requested {
		if _, ok := byName[pi.Name]; !ok {
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
			} else {
				return nil, fmt.Errorf("dependency %q required by %q not found in repo", depName, cur.Name)
			}
		}
	}

	// Sort result by package name for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	orgCnt := len(requested)
	finalCnt := len(result)
	fmt.Printf("yockgen: requested %d packages, resolved to %d packages\n", orgCnt, finalCnt)

	return result, nil
}

// ParsePrimary parses the repodata/primary.xml.gz file from a given base URL.
func ParsePrimary(baseURL, gzHref string) ([]provider.PackageInfo, error) {

	// Download the debian repo .gz file with all components meta data
	PkgMetaFile := "/tmp/Packages.gz"
	zipFiles, err := Download(gzHref, PkgMetaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to download repo file: %v", err)
	}

	// Decompress the .gz file and store the decompressed file in the same location
	if len(zipFiles) == 0 {
		return []provider.PackageInfo{}, fmt.Errorf("no files downloaded from repo URL: %s", gzHref)
	}
	files, err := Decompress(zipFiles[0])
	if err != nil {
		return []provider.PackageInfo{}, err
	}
	fmt.Printf("decompressed files: %v\n", files)

	// Parse the decompressed file
	f, err := os.Open(files[0])
	if err != nil {
		return nil, fmt.Errorf("failed to open decompressed file: %v", err)
	}
	defer f.Close()

	// packages file parser
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

	// Store the result in /tmp/Packages.trim
	// outFile := "/tmp/Packages.trim"
	// out, err := os.Create(outFile)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create output file: %v", err)
	// }
	// defer out.Close()

	// for _, pkg := range pkgs {
	// 	_, err := fmt.Fprintf(out, "Name: %s\nRequires: %s\nURL: %s\nChecksum: %s\n\n",
	// 		pkg.Name, pkg.Requires, pkg.URL, pkg.Checksum)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to write to output file: %v", err)
	// 	}
	// }

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
