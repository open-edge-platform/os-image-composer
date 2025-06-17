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
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/open-edge-platform/image-composer/internal/ospackage"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// extractBaseRequirement takes a potentially complex requirement string
// and returns only the base package/capability name.
func extractBaseRequirement(req string) string {
	if strings.HasPrefix(req, "(") && strings.Contains(req, " ") {
		trimmed := strings.TrimPrefix(req, "(")
		parts := strings.Fields(trimmed)
		if len(parts) > 0 {
			req = parts[0]
		}
	}
	finalParts := strings.Fields(req)
	if len(finalParts) == 0 {
		return ""
	}
	base := finalParts[0]
	return strings.TrimSuffix(base, "()(64bit)")
}

func GenerateDot(pkgs []ospackage.PackageInfo, file string) error {
	log := logger.Logger()
	log.Infof("Generating DOT file %s", file)

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
	requested []ospackage.PackageInfo,
	all []ospackage.PackageInfo,
) ([]ospackage.PackageInfo, error) {

	// Build helper maps:
	byName := make(map[string]ospackage.PackageInfo, len(all))
	provides := make(map[string][]string) // cap -> pkgNames
	requires := make(map[string][]string) // pkgName -> caps

	for _, pi := range all {
		byName[pi.Name] = pi
		provides[pi.Name] = append(provides[pi.Name], pi.Name)
		for _, cap := range pi.Provides {
			baseCap := extractBaseRequirement(cap)
			if baseCap != "" {
				provides[baseCap] = append(provides[baseCap], pi.Name)
			}
		}
		for _, file := range pi.Files {
			provides[file] = append(provides[file], pi.Name)
		}
		requires[pi.Name] = append([]string{}, pi.Requires...)
	}

	// bestProvider maps a capability to the single "best" package name that provides it.
	bestProvider := make(map[string]string, len(provides))
	for cap, provs := range provides {
		sort.Strings(provs)
		bestProvider[cap] = provs[len(provs)-1]
	}

	// BFS to find the complete set of needed package names.
	queue := make([]string, 0, len(requested))
	for _, pi := range requested {
		if _, ok := byName[pi.Name]; !ok {
			return nil, fmt.Errorf("requested package %q not in repo listing", pi.Name)
		}
		queue = append(queue, pi.Name)
	}

	neededSet := make(map[string]struct{})
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := neededSet[cur]; seen {
			continue
		}
		neededSet[cur] = struct{}{}

		for _, req := range requires[cur] {
			baseReq := extractBaseRequirement(req)
			if baseReq == "" {
				continue
			}
			if best, ok := bestProvider[baseReq]; ok {
				if _, seen := neededSet[best]; !seen {
					queue = append(queue, best)
				}
			}
		}
	}

	// Build the result slice in deterministic order:
	result := make([]ospackage.PackageInfo, 0, len(neededSet))
	for name := range neededSet {
		// Get the original package info.
		originalPI := byName[name]

		// Create a new PackageInfo to hold the cleaned data.
		cleanedPI := ospackage.PackageInfo{
			Name:     originalPI.Name,
			URL:      originalPI.URL,
			Checksum: originalPI.Checksum,
			Provides: originalPI.Provides,
			Files:    originalPI.Files,
			Requires: []string{}, // Start with an empty requires list.
		}

		// For each original requirement, find the concrete package that satisfies it
		// and add that package's name to the cleaned list.
		for _, req := range originalPI.Requires {
			baseReq := extractBaseRequirement(req)
			if baseReq == "" {
				continue
			}

			if providerName, ok := bestProvider[baseReq]; ok {
				// Only add if it's a different package to avoid self-dependencies
				if providerName != cleanedPI.Name {
					cleanedPI.Requires = append(cleanedPI.Requires, providerName)
				}
			}
		}

		// Deduplicate the cleaned requires list.
		reqSet := make(map[string]struct{})
		dedupedReqs := []string{}
		for _, r := range cleanedPI.Requires {
			if _, seen := reqSet[r]; !seen {
				reqSet[r] = struct{}{}
				dedupedReqs = append(dedupedReqs, r)
			}
		}
		cleanedPI.Requires = dedupedReqs

		result = append(result, cleanedPI)
	}
	// Sort the final result for deterministic output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// ParsePrimary parses the repodata/primary.xml(.gz/.zst) file from a given base URL.
func ParsePrimary(baseURL, gzHref string) ([]ospackage.PackageInfo, error) {

	resp, err := http.Get(baseURL + gzHref)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gr io.ReadCloser
	ext := strings.ToLower(filepath.Ext(gzHref))
	switch ext {
	case ".gz":
		gr, err = gzip.NewReader(resp.Body)

	case ".zst":
		zstDecoder, err := zstd.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		gr = zstDecoder.IOReadCloser()

	default:
		err = fmt.Errorf("unsupported compression type %s", ext)
	}

	if err != nil {
		return nil, err
	}
	defer gr.Close()

	dec := xml.NewDecoder(gr)

	var (
		infos          []ospackage.PackageInfo
		currentSection string // "provides", "requires", or ""
		curInfo        *ospackage.PackageInfo
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
				curInfo = &ospackage.PackageInfo{}

			case "location":
				// read the href and build full URL + infer Name
				for _, a := range elem.Attr {
					if a.Name.Local == "href" {
						curInfo.URL = baseURL + a.Value
						curInfo.Name = path.Base(a.Value)
						break
					}
				}
			case "name":
				// Read the canonical package name
				tok2, err2 := dec.Token()
				if err2 == nil {
					if charData, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Name = string(charData)
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
			case "file":
				// Grab the file path provided by the package
				tok2, err2 := dec.Token()
				if err2 == nil {
					if charData, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Files = append(curInfo.Files, string(charData))
					}
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
