package rpmutils

import (
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/network"
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

// ResolveDependencies takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolveDependencies(
	requested []ospackage.PackageInfo,
	all []ospackage.PackageInfo,
) ([]ospackage.PackageInfo, error) {

	// Build helper maps:
	byName := make(map[string]ospackage.PackageInfo, len(all))
	provides := make(map[string][]string) // cap -> pkgNames
	requires := make(map[string][]string) // pkgName -> caps

	for _, pi := range all {
		if pi.Arch == "src" {
			continue
		}
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
			Name:        originalPI.Name,
			Description: originalPI.Description,
			Type:        originalPI.Type,
			Arch:        originalPI.Arch,
			License:     originalPI.License,
			Origin:      originalPI.Origin,
			Version:     originalPI.Version,
			Checksums:   originalPI.Checksums,
			URL:         originalPI.URL,
			Provides:    originalPI.Provides,
			Files:       originalPI.Files,
			Requires:    []string{}, // Start with an empty requires list.
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

// ParseRepositoryMetadata parses the repodata/primary.xml(.gz/.zst) file from a given base URL.
func ParseRepositoryMetadata(baseURL, gzHref string) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	fullURL := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(gzHref, "/")
	log.Infof("Fetching and parsing repository metadata from %s", fullURL)

	client := network.NewSecureHTTPClient()
	resp, err := client.Get(fullURL)
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
		infos   []ospackage.PackageInfo
		curInfo *ospackage.PackageInfo
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
				curInfo.Type = "rpm"

			case "location":
				// read the href and build full URL + infer Name (filename)
				for _, a := range elem.Attr {
					if a.Name.Local == "href" {
						curInfo.URL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(a.Value, "/")
						curInfo.Name = path.Base(a.Value)
						break
					}
				}

			case "format":
				const rpmNS = "http://linux.duke.edu/metadata/rpm"

				// parse everything inside <format> (including rpm:provides/requires)
				section := "" // "provides" | "requires" | ""

			FormatLoop:
				for {
					tok2, err2 := dec.Token()
					if err2 != nil {
						break // EOF or error
					}
					switch inner := tok2.(type) {
					case xml.StartElement:
						switch {
						case inner.Name.Local == "license" && inner.Name.Space == rpmNS:
							if tok3, err := dec.Token(); err == nil {
								if cd, ok := tok3.(xml.CharData); ok && curInfo != nil {
									curInfo.License = strings.TrimSpace(string(cd))
								}
							}

						case inner.Name.Local == "vendor" && inner.Name.Space == rpmNS:
							if tok3, err := dec.Token(); err == nil {
								if cd, ok := tok3.(xml.CharData); ok && curInfo != nil {
									curInfo.Origin = strings.TrimSpace(string(cd))
								}
							}

						case inner.Name.Local == "provides" && inner.Name.Space == rpmNS:
							section = "provides"

						case inner.Name.Local == "requires" && inner.Name.Space == rpmNS:
							section = "requires"

						case inner.Name.Local == "entry" && inner.Name.Space == rpmNS:
							// rpm:entry name="..."
							var name string
							for _, a := range inner.Attr {
								if a.Name.Local == "name" {
									name = a.Value
									break
								}
							}
							if name != "" && curInfo != nil {
								if section == "provides" {
									curInfo.Provides = append(curInfo.Provides, name)
								} else if section == "requires" {
									curInfo.Requires = append(curInfo.Requires, name)
								}
							}

						// some repos list <file> entries inside <format> without a namespace
						case inner.Name.Local == "file" && inner.Name.Space == "":
							if tok3, err := dec.Token(); err == nil {
								if cd, ok := tok3.(xml.CharData); ok && curInfo != nil {
									curInfo.Files = append(curInfo.Files, strings.TrimSpace(string(cd)))
								}
							}
						}

					case xml.EndElement:
						switch {
						case inner.Name.Local == "provides" && inner.Name.Space == rpmNS:
							section = ""
						case inner.Name.Local == "requires" && inner.Name.Space == rpmNS:
							section = ""
						case inner.Name.Local == "format":
							break FormatLoop
						}
					}
				}

			case "name":
				// canonical package name
				if tok2, err2 := dec.Token(); err2 == nil {
					if cd, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Name = string(cd)
					}
				}

			case "description":
				if tok2, err2 := dec.Token(); err2 == nil {
					if cd, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Description = string(cd)
					}
				}

			case "arch":
				if tok2, err2 := dec.Token(); err2 == nil {
					if cd, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Arch = string(cd)
					}
				}

			case "checksum":
				// primary.xml checksum for the rpm payload (outside <format>)
				cs := ospackage.Checksum{}
				for _, attr := range elem.Attr {
					if attr.Name.Local == "type" {
						cs.Algorithm = strings.ToUpper(attr.Value) // SHA256, etc.
						break
					}
				}
				if tok2, err2 := dec.Token(); err2 == nil {
					if cd, ok := tok2.(xml.CharData); ok && curInfo != nil {
						cs.Value = string(cd)
						curInfo.Checksums = append(curInfo.Checksums, cs)
					}
				}

			case "file":
				// sometimes <file> is outside <format> as well
				if tok2, err2 := dec.Token(); err2 == nil {
					if cd, ok := tok2.(xml.CharData); ok && curInfo != nil {
						curInfo.Files = append(curInfo.Files, strings.TrimSpace(string(cd)))
					}
				}
			}

		case xml.EndElement:
			switch elem.Name.Local {
			case "package":
				// finish this package
				infos = append(infos, *curInfo)
			}
		}
	}
	return infos, nil
}

// FetchPrimaryURL downloads repomd.xml and returns the href of the primary metadata.
func FetchPrimaryURL(repomdURL string) (string, error) {
	client := network.NewSecureHTTPClient()
	resp, err := client.Get(repomdURL)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", repomdURL, err)
	}
	defer resp.Body.Close()

	dec := xml.NewDecoder(resp.Body)

	// Walk the tokens looking for <data type="primary">
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "data" {
			continue
		}
		// Check its type attribute
		var isPrimary bool
		for _, attr := range se.Attr {
			if attr.Name.Local == "type" && attr.Value == "primary" {
				isPrimary = true
				break
			}
		}
		if !isPrimary {
			// Skip this <data> section
			if err := dec.Skip(); err != nil {
				return "", fmt.Errorf("error skipping token: %w", err)
			}
			continue
		}

		// Inside <data type="primary">, look for <location href="..."/>
		for {
			tok2, err := dec.Token()
			if err != nil {
				if err == io.EOF {
					break
				}
				return "", err
			}
			// If we hit the end of this <data> element, bail out
			if ee, ok := tok2.(xml.EndElement); ok && ee.Name.Local == "data" {
				break
			}
			if le, ok := tok2.(xml.StartElement); ok && le.Name.Local == "location" {
				// Pull the href attribute
				for _, attr := range le.Attr {
					if attr.Name.Local == "href" {
						return attr.Value, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("primary location not found in %s", repomdURL)
}

func GetRepoMetaDataURL(baseURL, repoMetaXmlPath string) string {
	repoMetaDataURL := strings.TrimRight(baseURL, "/") + "/" + repoMetaXmlPath
	// Check if baseURL is a valid URL,
	if !strings.HasPrefix(repoMetaDataURL, "http://") && !strings.HasPrefix(repoMetaDataURL, "https://") {
		return ""
	}
	return repoMetaDataURL
}
