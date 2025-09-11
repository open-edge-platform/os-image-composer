package debutils

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/open-edge-platform/image-composer/internal/ospackage"
	"github.com/open-edge-platform/image-composer/internal/ospackage/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

func GenerateDot(pkgs []ospackage.PackageInfo, file string) error {
	return nil
}

// ParseRepositoryMetadata parses the Packages.gz file from gzHref.
func ParseRepositoryMetadata(baseURL string, pkggz string, releaseFile string, releaseSign string, pbGPGKey string, buildPath string, arch string) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	// Ensure pkgMetaDir exists, create if not
	// pkgMetaDir := "./builds/elxr12"
	pkgMetaDir := buildPath
	if err := os.MkdirAll(pkgMetaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pkgMetaDir: %w", err)
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
				return nil, fmt.Errorf("failed to remove old file %s: %w", f, remErr)
			}
		}
	}

	// Download the debian repo files
	err := pkgfetcher.FetchPackages([]string{pkggz, releaseFile, releaseSign, pbGPGKey}, pkgMetaDir, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch critical repo config packages: %w", err)
	}
	// Verify the release file
	relVryResult, err := VerifyRelease(localReleaseFile, localReleaseSign, localPBGPGKey)
	if err != nil {
		return nil, fmt.Errorf("failed to verify release file: %w", err)
	}
	if !relVryResult {
		return nil, fmt.Errorf("release file verification failed")
	}

	// verify the sham256 checksum of the Packages.gz file
	log.Infof("verifying checksum of package metadata file %s %s", baseURL, localPkggzFile)
	// get component from buildPath
	component := "main"
	// Detect last underscore and extract the word after it as component
	if idx := strings.LastIndex(buildPath, "_"); idx != -1 && len(buildPath) > idx+1 {
		component = buildPath[idx+1:]
	}
	//
	pkggzVryResult, err := VerifyPackagegz(localReleaseFile, localPkggzFile, arch, component)
	if err != nil {
		return nil, fmt.Errorf("failed to verify pkg file: %w", err)
	}
	if !pkggzVryResult {
		return nil, fmt.Errorf("package file verification failed")
	}

	//Decompress the Packages (xz or gz) file
	// The decompressed file will be named as Packages
	PkgMetaFile := filepath.Join(pkgMetaDir, filepath.Base(pkggz))
	pkgMetaFileNoExt := filepath.Join(filepath.Dir(PkgMetaFile), strings.TrimSuffix(filepath.Base(PkgMetaFile), filepath.Ext(PkgMetaFile)))
	log.Infof("decompressing package metadata file %s to %s", PkgMetaFile, pkgMetaFileNoExt)

	files, err := Decompress(PkgMetaFile, pkgMetaFileNoExt)
	if err != nil {
		return []ospackage.PackageInfo{}, fmt.Errorf("failed package decompress: %w", err)
	}
	log.Infof("decompressed files: %v", files)

	//Parse the decompressed file
	if len(files) == 0 {
		return nil, fmt.Errorf("no decompressed files found")
	}
	f, err := os.Open(files[0])
	if err != nil {
		return nil, fmt.Errorf("failed to open decompressed file: %w", err)
	}
	defer f.Close()

	var pkgs []ospackage.PackageInfo
	pkg := ospackage.PackageInfo{}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading file: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// End of one package entry
			if pkg.Name != "" {
				pkgs = append(pkgs, pkg)
				pkg = ospackage.PackageInfo{}
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
			pkg.Type = "deb"
		case "Version":
			pkg.Version = val
		case "Pre-Depends":
			// Split dependencies by comma and clean each dependency
			deps := strings.Split(val, ",")
			for _, dep := range deps {
				cleanedDep := CleanDependencyName(dep)
				if cleanedDep != "" {
					pkg.Requires = append(pkg.Requires, cleanedDep)
				}
			}
		case "Depends":
			// Split dependencies by comma and clean each dependency
			deps := strings.Split(val, ",")
			pkg.RequiresVer = append(pkg.RequiresVer, deps...)
			for _, dep := range deps {
				cleanedDep := CleanDependencyName(dep)
				if cleanedDep != "" {
					pkg.Requires = append(pkg.Requires, cleanedDep)
				}
			}
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
			pkg.Checksums = append(pkg.Checksums, ospackage.Checksum{
				Algorithm: "SHA256",
				Value:     val,
			})

		case "SHA1":
			pkg.Checksums = append(pkg.Checksums, ospackage.Checksum{
				Algorithm: "SHA1",
				Value:     val,
			})
		case "SHA512":
			pkg.Checksums = append(pkg.Checksums, ospackage.Checksum{
				Algorithm: "SHA512",
				Value:     val,
			})
		case "Description":
			pkg.Description = val
		case "Architecture":
			if val == "all" || val == "any" {
				pkg.Arch = "noarch"
			} else {
				pkg.Arch = val
			}
		case "Maintainer":
			pkg.Origin = val
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

	neededSet := make(map[string]struct{})
	queue := make([]ospackage.PackageInfo, 0, len(requested))
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

	// depedencies resolution logic
	result := make([]ospackage.PackageInfo, 0)
	var parentChildPairs [][]ospackage.PackageInfo // Track parent->child relationships for reporting
	gotMissingPkg := false

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if _, seen := neededSet[cur.Name]; seen {
			continue
		}
		neededSet[cur.Name] = struct{}{}
		result = append(result, cur)

		// Traverse dependencies
		for _, dep := range cur.Requires {

			depName := CleanDependencyName(dep)
			if depName == "" || neededSet[depName] != struct{}{} {
				continue
			}
			if _, seen := neededSet[depName]; seen {
				// Check if the new package can use the existing package. If it cannot, then error out; otherwise, continue.
				// get the package from current queue based on name
				existing := findAllCandidates(depName, queue)
				if len(existing) > 0 {
					// check if the existing package can satisfy the version requirement
					_, err := resolveMultiCandidates(cur, existing)
					if err != nil {
						// get require version
						var requiredVer string
						for _, req := range cur.RequiresVer {
							if strings.Contains(req, depName) {
								requiredVer = req
								break
							}
						}
						return nil, fmt.Errorf("conflicting package dependencies: %s_%s requires %s, but %s_%s is to be installed", cur.Name, cur.Version, requiredVer, existing[0].Name, existing[0].Version)
					}
				}
				continue
			}

			candidates := findAllCandidates(depName, all)
			if len(candidates) >= 1 {
				// Pick the candidate using the resolver and add it to the queue
				chosenCandidate, err := resolveMultiCandidates(cur, candidates)
				if err != nil {
					gotMissingPkg = true
					AddParentMissingChildPair(cur, depName+"(missing)", &parentChildPairs)
					log.Warnf("failed to resolve multiple candidates for dependency %q of package %q: %v", depName, cur.Name, err)
					continue
				}
				queue = append(queue, chosenCandidate)
				AddParentChildPair(cur, chosenCandidate, &parentChildPairs)
				continue
			} else {
				log.Warnf("no candidates found for dependency %q of package %q", depName, cur.Name)
				gotMissingPkg = true
				AddParentMissingChildPair(cur, depName+"(missing)", &parentChildPairs)
				continue
			}
		}
	}

	// check missing dep and write report
	if gotMissingPkg {
		report := BuildDependencyChains(parentChildPairs)
		return nil, fmt.Errorf("one or more requested dependencies not found. See list in %s", report)
	}

	// Sort result by package name for determinism
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
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
	// Empty-version handling: empty < any non-empty
	if a == "" && b == "" {
		return 0, nil
	}
	if a == "" {
		return -1, nil
	}
	if b == "" {
		return 1, nil
	}

	// Helper to split epoch
	splitEpoch := func(ver string) (epoch int, rest string) {
		parts := strings.SplitN(ver, ":", 2)
		if len(parts) == 2 {
			if _, err := fmt.Sscanf(parts[0], "%d", &epoch); err != nil {
				epoch = 0
			}
			rest = parts[1]
		} else {
			epoch = 0
			rest = ver
		}
		return
	}

	// Split upstream_version and debian_revision at last hyphen
	splitRevision := func(ver string) (upstream string, debian string) {
		if i := strings.LastIndex(ver, "-"); i >= 0 {
			return ver[:i], ver[i+1:]
		}
		return ver, ""
	}

	// nextSegment returns the next contiguous numeric or non-numeric segment.
	nextSegment := func(s string) (seg string, rest string, numeric bool) {
		if s == "" {
			return "", "", false
		}
		// numeric segment
		if s[0] >= '0' && s[0] <= '9' {
			i := 0
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			return s[:i], s[i:], true
		}
		// non-numeric segment
		i := 0
		for i < len(s) && (s[i] < '0' || s[i] > '9') {
			i++
		}
		return s[:i], s[i:], false
	}

	// Character ordering per Debian: '~' < end-of-string < letters < other characters.
	// This ordering is crucial for correct Debian version comparison, as defined in
	// Debian Policy Manual section 5.6.12 ("Version"). See:
	// https://www.debian.org/doc/debian-policy/ch-controlfields.html#version
	charOrder := func(r rune) int {
		if r == '~' {
			return -2
		}
		if r == 0 {
			return -1
		}
		if unicode.IsLetter(r) {
			return int(r)
		}
		return 0x100 + int(r)
	}

	// Compare two non-digit segments using Debian ordering
	compareNonDigitSegments := func(aSeg, bSeg string) int {
		ai, bi := 0, 0
		for {
			var ra, rb rune
			if ai < len(aSeg) {
				ra = rune(aSeg[ai])
			} else {
				ra = 0
			}
			if bi < len(bSeg) {
				rb = rune(bSeg[bi])
			} else {
				rb = 0
			}
			// both ended
			if ra == 0 && rb == 0 {
				return 0
			}
			if ra != rb {
				oa := charOrder(ra)
				ob := charOrder(rb)
				if oa < ob {
					return -1
				}
				return 1
			}
			ai++
			bi++
		}
	}

	// Compare numeric segments (as dpkg: strip leading zeros, compare length, then lexicographically)
	compareNumericSegments := func(aSeg, bSeg string) int {
		aTrim := strings.TrimLeft(aSeg, "0")
		bTrim := strings.TrimLeft(bSeg, "0")
		// treat empty as zero
		if aTrim == "" && bTrim == "" {
			return 0
		}
		if aTrim == "" {
			return -1
		}
		if bTrim == "" {
			return 1
		}
		// longer numeric (more digits) is greater
		if len(aTrim) > len(bTrim) {
			return 1
		}
		if len(aTrim) < len(bTrim) {
			return -1
		}
		// same length -> lexical compare works
		if aTrim > bTrim {
			return 1
		}
		if aTrim < bTrim {
			return -1
		}
		return 0
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

	// Split upstream and debian revisions
	upA, debA := splitRevision(restA)
	upB, debB := splitRevision(restB)

	// Compare iterative parts (used for upstream version and debian revision)
	compareParts := func(sa, sb string) int {
		for sa != "" || sb != "" {
			// Handle tilde first: '~' sorts before everything (including end-of-string)
			if (len(sa) > 0 && sa[0] == '~') || (len(sb) > 0 && sb[0] == '~') {
				if len(sa) > 0 && sa[0] == '~' && !(len(sb) > 0 && sb[0] == '~') {
					return -1
				}
				if len(sb) > 0 && sb[0] == '~' && !(len(sa) > 0 && sa[0] == '~') {
					return 1
				}
				// both have tilde: consume and continue
				if len(sa) > 0 && sa[0] == '~' && len(sb) > 0 && sb[0] == '~' {
					sa = sa[1:]
					sb = sb[1:]
					continue
				}
			}

			// After tilde handling, if either side is exhausted, the exhausted side is less
			if sa == "" && sb == "" {
				break
			}
			if sa == "" {
				return -1
			}
			if sb == "" {
				return 1
			}

			segA, restASeg, numA := nextSegment(sa)
			segB, restBSeg, numB := nextSegment(sb)

			// both empty segments -> continue
			if segA == "" && segB == "" {
				sa, sb = restASeg, restBSeg
				continue
			}

			// numeric vs non-numeric: numeric < non-numeric
			if numA != numB {
				if numA {
					return -1
				}
				return 1
			}

			// both numeric
			if numA && numB {
				if cmp := compareNumericSegments(segA, segB); cmp != 0 {
					return cmp
				}
			} else { // both non-numeric
				if cmp := compareNonDigitSegments(segA, segB); cmp != 0 {
					return cmp
				}
			}

			sa, sb = restASeg, restBSeg
		}
		return 0
	}

	// Compare upstream versions first, then debian revisions
	if cmp := compareParts(upA, upB); cmp != 0 {
		return cmp, nil
	}
	if cmp := compareParts(debA, debB); cmp != 0 {
		return cmp, nil
	}
	return 0, nil
}

// CleanDependencyName extracts the base package name from a complex dependency string.
// Handles version constraints, alternatives, and architecture qualifiers.
// Examples:
//
//	"libc6 (>= 2.34)" -> "libc6"
//	"python3 | python3-dev" -> "python3"
//	"gcc:amd64" -> "gcc"
func CleanDependencyName(dep string) string {
	depName := strings.TrimSpace(dep)

	// Handle alternatives (|) - take the first option
	if idx := strings.Index(depName, "|"); idx > 0 {
		depName = strings.TrimSpace(depName[:idx])
	}

	// Handle architecture qualifiers (:)
	if idx := strings.Index(depName, ":"); idx > 0 {
		depName = depName[:idx]
	}

	// Handle version constraints - remove everything from opening parenthesis
	if idx := strings.Index(depName, "("); idx > 0 {
		depName = strings.TrimSpace(depName[:idx])
	} else if idx := strings.Index(depName, " "); idx > 0 {
		// Handle cases where there's a space but no parentheses
		depName = depName[:idx]
	}

	return strings.TrimSpace(depName)
}

// compareVersions compares two Debian package versions
// Returns 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Extract version from Debian package names like "acct_6.6.4-5+b1_amd64.deb"
	extractVersion := func(name string) string {
		parts := strings.Split(name, "_")
		if len(parts) >= 2 {
			return parts[1]
		}
		return name
	}
	ver1 := extractVersion(v1)
	ver2 := extractVersion(v2)
	cmp, _ := compareDebianVersions(ver1, ver2)
	return cmp
}

// ResolvePackage finds the best matching package for a given package name
func ResolveTopPackageConflicts(want string, all []ospackage.PackageInfo) (ospackage.PackageInfo, bool) {
	var candidates []ospackage.PackageInfo
	for _, pi := range all {
		// 1) exact name and version matched with .deb filenamae, e.g. acct_7.6.4-5+b1_amd64
		if filepath.Base(pi.URL) == want+".deb" {
			candidates = append(candidates, pi)
			break
		}
		// 2) exact name, e.g. acct
		if pi.Name == want {
			candidates = append(candidates, pi)
			break
		}
		// 3) prefix by want-version ("acl-")
		if strings.HasPrefix(pi.Name, want+"-") {
			candidates = append(candidates, pi)
			continue
		}
		// 4) prefix by want.release ("acl-2.3.1-2.")
		if strings.HasPrefix(pi.Name, want+".") {
			candidates = append(candidates, pi)
			continue
		}
		// 5) Debian package format (packagename_version_arch.deb)
		if strings.HasPrefix(pi.Name, want+"_") {
			candidates = append(candidates, pi)
		}
	}

	if len(candidates) == 0 {
		return ospackage.PackageInfo{}, false
	}

	// If we got an exact match in step (1), it's the only candidate
	if len(candidates) == 1 && (candidates[0].Name == want || candidates[0].Name == want+".deb") {
		return candidates[0], true
	}

	// Sort by version (highest version first)
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersions(candidates[i].URL, candidates[j].URL) > 0
	})

	return candidates[0], true
}

// Helper function to find all candidates for a dependency
func findAllCandidates(depName string, all []ospackage.PackageInfo) []ospackage.PackageInfo {
	var candidates []ospackage.PackageInfo

	// First pass: look for exact name matches
	for _, pi := range all {
		if pi.Name == depName {
			candidates = append(candidates, pi)
		}
	}

	// If no direct matches found, search in Provides field
	if len(candidates) == 0 {
		for _, pi := range all {
			for _, provided := range pi.Provides {
				if provided == depName {
					candidates = append(candidates, pi)
				}
			}
		}
	}

	return candidates
}

// Helper function to resolve multiple candidates by picking the last one
// extractRepoBase extracts the Debian repo base URL (everything up to /pool/)
func extractRepoBase(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Split path by "/pool/"
	parts := strings.SplitN(u.Path, "/pool/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("URL does not contain /pool/: %s", rawURL)
	}

	// Rebuild base URL: scheme + host + prefix before /pool/
	base := fmt.Sprintf("%s://%s%s/", u.Scheme, u.Host, parts[0])
	return base, nil
}

func extractVersionRequirement(reqVers []string, depName string) (op string, ver string, found bool) {
	for _, reqVer := range reqVers {
		reqVer = strings.TrimSpace(reqVer)

		// Handle alternatives (|) - check if our depName is in any of the alternatives
		alternatives := strings.Split(reqVer, "|")
		for _, alt := range alternatives {
			alt = strings.TrimSpace(alt)

			// Check if this alternative starts with the dependency name we're looking for
			cleanReqName := CleanDependencyName(alt)
			if cleanReqName != depName {
				continue // Skip to next alternative
			}

			// Found our dependency in this alternative, now extract version constraint
			// Find version constraint inside parentheses
			if idx := strings.Index(alt, "("); idx != -1 {
				verConstraint := alt[idx+1:]
				if idx2 := strings.Index(verConstraint, ")"); idx2 != -1 {
					verConstraint = verConstraint[:idx2]
				}

				// Split into operator and version
				parts := strings.Fields(verConstraint)
				if len(parts) == 2 {
					op := parts[0]
					ver := parts[1]
					return op, ver, true
				}
			}

			// If we found the dependency but no version constraint, return found=false
			return "", "", false
		}
	}

	return "", "", false
}

func resolveMultiCandidates(parentPkg ospackage.PackageInfo, candidates []ospackage.PackageInfo) (ospackage.PackageInfo, error) {
	parentBase, err := extractRepoBase(parentPkg.URL)
	if err != nil {
		return ospackage.PackageInfo{}, fmt.Errorf("failed to extract repo base from parent package URL: %w", err)
	}

	/////////////////////////////////////
	//A: if version is specified
	/////////////////////////////////////
	// All candidates have the same .Name, so just use candidates[0].Name for version extraction
	op := ""
	ver := ""
	hasVersionConstraint := false
	if len(candidates) > 0 {
		op, ver, hasVersionConstraint = extractVersionRequirement(parentPkg.RequiresVer, candidates[0].Name)
	}

	if hasVersionConstraint {
		// First pass: look for candidates from the same repo that meet version constraint
		var sameRepoMatches []ospackage.PackageInfo
		var otherRepoMatches []ospackage.PackageInfo

		for _, candidate := range candidates {
			candidateBase, err := extractRepoBase(candidate.URL)
			if err != nil {
				continue
			}

			// Check if version constraint is satisfied
			cmp, err := compareDebianVersions(candidate.Version, ver)
			if err != nil {
				continue
			}

			versionMatches := false
			switch op {
			case "=":
				versionMatches = (cmp == 0)
			case "<<", "<":
				versionMatches = (cmp < 0)
			case "<=":
				versionMatches = (cmp <= 0)
			case ">>", ">":
				versionMatches = (cmp > 0)
			case ">=":
				versionMatches = (cmp >= 0)
			}

			if versionMatches {
				if candidateBase == parentBase {
					sameRepoMatches = append(sameRepoMatches, candidate)
				} else {
					otherRepoMatches = append(otherRepoMatches, candidate)
				}
			}
		}

		// Priority 1: return first match from same repo
		if len(sameRepoMatches) > 0 {
			return sameRepoMatches[0], nil
		}

		// Priority 2: return first match from other repos
		if len(otherRepoMatches) > 0 {
			return otherRepoMatches[0], nil
		}

		return ospackage.PackageInfo{}, fmt.Errorf("no candidates satisfy version constraint = %s%s", op, ver)
	}

	/////////////////////////////////////
	// B: if version is not specified
	//////////////////////////////////////

	// Check for empty candidates list
	if len(candidates) == 0 {
		return ospackage.PackageInfo{}, fmt.Errorf("no candidates provided for selection")
	}

	// If only one candidate, return it
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Rule 1: find all candidates with the same base URL and return the latest version
	var sameBaseCandidates []ospackage.PackageInfo
	for _, candidate := range candidates {
		candidateBase, err := extractRepoBase(candidate.URL)
		if err != nil {
			continue
		}
		if candidateBase == parentBase {
			sameBaseCandidates = append(sameBaseCandidates, candidate)
		}
	}

	// If we found candidates with the same base URL, return the one with the latest version
	if len(sameBaseCandidates) > 0 {
		if len(sameBaseCandidates) == 1 {
			return sameBaseCandidates[0], nil
		}

		// Find the candidate with the latest version
		latest := sameBaseCandidates[0]
		for _, candidate := range sameBaseCandidates[1:] {
			cmp := compareVersions(candidate.Version, latest.Version)
			if cmp > 0 {
				latest = candidate
			}
		}

		return latest, nil
	}

	// Rule 2: If no candidate has the same repo, return the first candidate in other repos
	return candidates[0], nil
}
