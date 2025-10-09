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

			case "version":
				// Parse version attributes and combine them
				var epoch, ver, rel string
				for _, attr := range elem.Attr {
					switch attr.Name.Local {
					case "epoch":
						epoch = attr.Value
					case "ver":
						ver = attr.Value
					case "rel":
						rel = attr.Value
					}
				}

				// Build version string in format: epoch:ver-rel
				if curInfo != nil {
					// Fill missing fields with "0"
					if epoch == "" {
						epoch = "0"
					}
					if ver == "" {
						ver = "0"
					}
					if rel == "" {
						rel = "0"
					}
					versionStr := fmt.Sprintf("%s:%s-%s", epoch, ver, rel)
					curInfo.Version = versionStr
				}

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
							// rpm:entry name="..." ver="..." rel="..." epoch="..." flags="..."
							var name, version, release, epoch, flags string
							for _, a := range inner.Attr {
								switch a.Name.Local {
								case "name":
									name = a.Value
								case "ver":
									version = a.Value
								case "rel":
									release = a.Value
								case "epoch":
									epoch = a.Value
								case "flags":
									flags = a.Value
								}
							}
							if name != "" && curInfo != nil {
								if section == "provides" {
									curInfo.Provides = append(curInfo.Provides, name)
								} else if section == "requires" {
									// Store the base name in Requires
									curInfo.Requires = append(curInfo.Requires, name)

									// Store version constraint with package name prefix in RequiresVer
									if version != "" || release != "" || epoch != "" || flags != "" {
										versionPart := ""
										if epoch != "" {
											versionPart = epoch + ":"
										}
										if version != "" {
											versionPart += version
										}
										if release != "" {
											versionPart += "-" + release
										}

										var versionConstraint string
										if flags != "" && versionPart != "" {
											// Convert flags to readable format (GE = >=, EQ = =, etc.)
											operator := convertFlags(flags)
											versionConstraint = fmt.Sprintf("%s (%s %s)", name, operator, versionPart) // samuel (>=2.3)
										} else if versionPart != "" {
											// Version info but no operator, assume equality
											versionConstraint = fmt.Sprintf("%s = %s", name, versionPart)
										} else {
											// Only package name
											versionConstraint = name
										}
										curInfo.RequiresVer = append(curInfo.RequiresVer, versionConstraint)
									} else {
										// No version constraint, just store the package name
										curInfo.RequiresVer = append(curInfo.RequiresVer, name)
									}
								}
							}

						// some repos list <file> entries inside <format> without a namespace
						case inner.Name.Local == "file" && inner.Name.Space != rpmNS:
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
				if curInfo.Arch == "src" {
					continue
				}
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

// Helper function to convert RPM flags to readable operators
func convertFlags(flags string) string {
	switch flags {
	case "EQ":
		return "="
	case "GE":
		return ">="
	case "LE":
		return "<="
	case "GT":
		return ">"
	case "LT":
		return "<"
	default:
		return flags
	}
}

// MatchRequested matches requested package names to the best available versions in the repo.
func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {

	var out []ospackage.PackageInfo

	for _, want := range requests {
		if pkg, found := ResolveTopPackageConflicts(want, "rpm", all); found {
			out = append(out, pkg)

		} else {
			return nil, fmt.Errorf("requested package '%q' not found in repo", want)
		}
	}
	return out, nil
}

// ResolveDependencies takes a seed list of PackageInfos (the exact versions
// matched) and the full list of all PackageInfos from the repo, and
// returns the minimal closure of PackageInfos needed to satisfy all Requires.
func ResolveDependencies(requested []ospackage.PackageInfo, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	// Build maps for fast lookup
	byNameVer := make(map[string]ospackage.PackageInfo, len(all))
	for _, pi := range all {
		if pi.Version != "" {
			key := fmt.Sprintf("%s=%s", pi.Name, pi.Version)
			byNameVer[key] = pi
		}
	}

	// Clear required fields for all and requested
	for i := range all {
		all[i].Requires = nil
	}
	for i := range requested {
		requested[i].Requires = nil
	}

	neededSet := make(map[string]struct{})
	queue := make([]ospackage.PackageInfo, 0, len(requested))

	// Initialize queue with requested packages
	for _, pi := range requested {
		if pi.Version != "" {
			key := fmt.Sprintf("%s=%s", pi.Name, pi.Version)
			if pkg, ok := byNameVer[key]; ok {
				queue = append(queue, pkg)
				continue
			}
		}
		return nil, fmt.Errorf("requested package %q not in repo listing", pi.Name)
	}

	// Use a map to store results so we can modify them
	resultMap := make(map[string]*ospackage.PackageInfo)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if _, seen := neededSet[cur.Name]; seen {
			continue
		}
		neededSet[cur.Name] = struct{}{}

		// Store a copy in the result map so we can modify it
		curCopy := cur
		resultMap[cur.Name] = &curCopy

		// Process dependencies
		for _, dep := range cur.RequiresVer {
			// Use proper dependency name cleaning
			// depName := extractBaseRequirement(dep)
			depName := extractBaseNameFromDep(dep)
			filename, seen := findMatchingKeyInNeededSet(neededSet, depName)
			if depName == "" || neededSet[filename] != struct{}{} {
				continue
			}

			// Check if already resolved
			// if _, seen := neededSet[depName]; seen {
			if seen {
				// ENHANCEMENT: Check version compatibility for already-resolved dependencies
				existing, err := findAllCandidates(cur, depName, queue) //convertMapToSlice(resultMap))
				if err == nil && len(existing) > 0 {
					// Validate that existing package satisfies current requirement
					_, err := resolveMultiCandidates(cur, existing)
					if err != nil {
						// Find the specific version constraint from RequiresVer
						var requiredVer string
						for _, req := range cur.RequiresVer {
							if strings.Contains(req, depName) {
								requiredVer = req
								break
							}
						}
						return nil, fmt.Errorf("conflicting package dependencies: %s_%s requires %s, but %s is already selected",
							cur.Name, cur.Version, requiredVer, existing[0].Name)
					}
				}
				// Append to parent's Requires field even if already resolved
				if resultPkg, exists := resultMap[cur.Name]; exists {
					resultPkg.Requires = append(resultPkg.Requires, filename)
				}

				continue
			}

			// Find candidates for this dependency
			candidates, err := findAllCandidates(cur, depName, all)
			if err != nil {
				return nil, fmt.Errorf("failed to find candidates for dependency %q of package %q: %v", depName, cur.Name, err)
			}

			if len(candidates) >= 1 {
				chosenCandidate, err := resolveMultiCandidates(cur, candidates)
				if err != nil {
					log.Errorf("failed to resolve multiple candidates for dependency %q of package %q: %v", depName, cur.Name, err)
					return nil, fmt.Errorf("failed to resolve multiple candidates for dependency %q of package %q: %v", depName, cur.Name, err)
				}

				// Update the parent's Requires field with the chosen candidate's name
				if resultPkg, exists := resultMap[cur.Name]; exists {
					resultPkg.Requires = append(resultPkg.Requires, chosenCandidate.Name)
				}

				// Add chosen candidate to the queue for further processing
				queue = append(queue, chosenCandidate)
			} else {
				// FAIL FAST instead of just warning
				return nil, fmt.Errorf("no candidates found for required dependency %q of package %q", depName, cur.Name)
			}
		}
	}

	// Convert result map back to slice
	result := make([]ospackage.PackageInfo, 0, len(resultMap))
	for _, pkg := range resultMap {
		result = append(result, *pkg)
	}

	// Sort result by package name for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	log.Infof("Successfully resolved %d packages from %d requested packages", len(result), len(requested))
	return result, nil
}

// findMatchingKeyInNeededSet checks if any key in neededSet contains depName as a substring,
// and returns the first matching key whose base package name equals depName.
func findMatchingKeyInNeededSet(neededSet map[string]struct{}, depName string) (string, bool) {
	for k := range neededSet {
		if strings.Contains(k, depName) {
			fileName := extractBasePackageNameFromFile(k)
			if fileName == depName {
				return k, true
			}
		}
	}
	return "", false
}
