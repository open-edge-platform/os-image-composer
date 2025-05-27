package rpmutils

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/open-edge-platform/image-composer/internal/provider"

	"github.com/cavaliergopher/rpm"
	"go.uber.org/zap"
)

// Index maps provided capabilities to RPM paths and RPM paths to their requirements.
type Index struct {
	Provides map[string][]string // capability name → []rpm paths
	Requires map[string][]string // rpm path → []required capability names
}

// BuildIndex scans all RPM files under dir and builds the Index.
func BuildIndex(dir string) (*Index, error) {

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

func GenerateDot(pkgs []provider.PackageInfo, file string) error {
	logger := zap.L().Sugar()
	logger.Infof("Generating DOT file %s", file)

	outFile, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("creating DOT file: %w", err)
	}
	defer outFile.Close()

	fmt.Fprintln(outFile, "digraph G {")
	fmt.Fprintln(outFile, "  rankdir=LR;")
	for _, pkg := range pkgs {
		// Quote the node ID and label
		fmt.Fprintf(outFile, "\t\"%s\" [label=\"%s\"];\n", pkg.Name, pkg.Name)
		for _, dep := range pkg.Requires {
			// Quote both source and target
			fmt.Fprintf(outFile, "\t\"%s\" -> \"%s\";\n", pkg.Name, dep)
		}
	}

	fmt.Fprintln(outFile, "}")
	return nil
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

	// bestProvider maps cap -> the single “best” pkgName
	bestProvider := make(map[string]string, len(provides))
	for cap, provs := range provides {
		// pick the lexically greatest filename;
		// for real RPM semver we might want to check rpmutils.VersionCompare()
		sort.Strings(provs)
		bestProvider[cap] = provs[len(provs)-1]
	}

	// BFS over the require->provide graph:
	neededSet := make(map[string]struct{})
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := neededSet[cur]; seen {
			continue
		}
		neededSet[cur] = struct{}{}

		for _, cap := range requires[cur] {
			if best, ok := bestProvider[cap]; ok {
				if _, seen := neededSet[best]; !seen {
					queue = append(queue, best)
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
