package rpmutils

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"github.com/cavaliergopher/rpm"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
	"go.uber.org/zap"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"sort"
)

// Index maps provided capabilities to RPM paths and RPM paths to their requirements.
type Index struct {
	Provides map[string][]string // capability name → []rpm paths
	Requires map[string][]string // rpm path → []required capability names
}

// BuildIndex scans all RPM files under dir and builds the Index.
func BuildIndex(dir string) (*Index, error) {
	//logger := zap.L().Sugar()
	idx := &Index{
		Provides: make(map[string][]string),
		Requires: make(map[string][]string),
	}

	pattern := filepath.Join(dir, "*.rpm")
	rpmFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, rpmPath := range rpmFiles {
		// Open the RPM file
		pkgFile, err := rpm.Open(rpmPath)
		if err != nil {
			return nil, fmt.Errorf("opening RPM %s: %w", rpmPath, err)
		}

		// Extract capabilities it Provides()
		provDeps := pkgFile.Provides() // []rpm.Dependency
		for _, dep := range provDeps {
			name := dep.Name() // call method to get string
			idx.Provides[name] = append(idx.Provides[name], rpmPath)
			// logger.Debugf("RPM %s provides %s", rpmPath, name)
		}

		// Extract its Requires()
		reqDeps := pkgFile.Requires() // []rpm.Dependency
		reqNames := make([]string, len(reqDeps))
		for i, dep := range reqDeps {
			reqNames[i] = dep.Name() // call method
			// logger.Debugf("RPM %s requires %s", rpmPath, reqNames[i])
		}
		idx.Requires[rpmPath] = reqNames
	}

	return idx, nil
}

// ResolveDependencies returns the full set of RPMs needed
// starting from the given root paths, walking requires -> provides.
func ResolveDependencies(roots []string, idx *Index) []string {
	logger := zap.L().Sugar()
	needed := make(map[string]struct{})
	queue := append([]string{}, roots...)

	logger.Infof("resolving dependencies for %d RPMs", len(roots))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := needed[cur]; seen {
			continue
		}
		needed[cur] = struct{}{}

		// Enqueue all providers of each required capability
		for _, capName := range idx.Requires[cur] {
			for _, providerRPM := range idx.Provides[capName] {
				if _, seen := needed[providerRPM]; !seen {
					queue = append(queue, providerRPM)
				}
			}
		}
	}

	// Collect result
	result := make([]string, 0, len(needed))
	for pkg := range needed {
		result = append(result, pkg)
	}
	return result
}

// ResolvePackageInfos takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolvePackageInfos(
	requested []provider.PackageInfo,
	all []provider.PackageInfo,
) ([]provider.PackageInfo, error) {
	// Build helper maps:
	byName := make(map[string]provider.PackageInfo, len(all))
	provides := make(map[string][]string) // cap -> pkgNames
	requires := make(map[string][]string) // pkgName -> caps

	for _, pi := range all {
		byName[pi.Name] = pi
		for _, cap := range pi.Provides {
			provides[cap] = append(provides[cap], pi.Name)
		}
		requires[pi.Name] = append([]string{}, pi.Requires...)
	}

	// Seed the queue with the user‐requested package names:
	queue := make([]string, 0, len(requested))
	for _, pi := range requested {
		if _, ok := byName[pi.Name]; !ok {
			return nil, fmt.Errorf("requested package %q not in repo listing", pi.Name)
		}
		queue = append(queue, pi.Name)
	}

	// BFS over the require→provide graph:
	neededSet := make(map[string]struct{})
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := neededSet[cur]; seen {
			continue
		}
		neededSet[cur] = struct{}{}

		for _, cap := range requires[cur] {
			for _, providerName := range provides[cap] {
				if _, seen := neededSet[providerName]; !seen {
					queue = append(queue, providerName)
				}
			}
		}
	}

	// Build the result slice in deterministic order:
	result := make([]provider.PackageInfo, 0, len(neededSet))
	for name := range neededSet {
		result = append(result, byName[name])
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// ParsePrimary parses the repodata/primary.xml.gz file from a given base URL.
func ParsePrimary(baseURL, gzHref string) ([]provider.PackageInfo, error) {

	resp, err := http.Get(baseURL + gzHref)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	dec := xml.NewDecoder(gr)

	var (
		infos          []provider.PackageInfo
		currentSection string // "provides", "requires", or ""
		curInfo        *provider.PackageInfo
	)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		switch elem := tok.(type) {
		case xml.StartElement:
			switch elem.Name.Local {
			case "package":
				// start a new PackageInfo
				curInfo = &provider.PackageInfo{}

			case "location":
				// read the href and build full URL + infer Name
				for _, a := range elem.Attr {
					if a.Name.Local == "href" {
						curInfo.URL = baseURL + a.Value
						curInfo.Name = path.Base(a.Value)
						break
					}
				}

			case "checksum":
				// grab the checksum text
				tok2, err2 := dec.Token()
				if err2 == nil {
					if charData, ok := tok2.(xml.CharData); ok {
						curInfo.Checksum = string(charData)
					}
				}
			case "provides":
				currentSection = "provides"
			case "requires":
				currentSection = "requires"

			case "entry":
				// grab the name attribute
				var name string
				for _, a := range elem.Attr {
					if a.Name.Local == "name" {
						name = a.Value
						break
					}
				}
				if currentSection == "provides" {
					curInfo.Provides = append(curInfo.Provides, name)
				} else if currentSection == "requires" {
					curInfo.Requires = append(curInfo.Requires, name)
				}
			}

		case xml.EndElement:
			switch elem.Name.Local {
			case "provides", "requires":
				currentSection = ""
			case "package":
				// finish this package
				infos = append(infos, *curInfo)
			}
		}
	}
	return infos, nil
}
