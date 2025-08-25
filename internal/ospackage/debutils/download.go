package debutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/config/manifest"
	"github.com/open-edge-platform/image-composer/internal/ospackage"
	"github.com/open-edge-platform/image-composer/internal/ospackage/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/ospackage/pkgsorter"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// repoConfig hold repo related info
type RepoConfig struct {
	Section      string // raw section header
	Name         string // human-readable name from name=
	PkgList      string
	PkgPrefix    string
	GPGCheck     bool
	RepoGPGCheck bool
	Enabled      bool
	PbGPGKey     string
	ReleaseFile  string
	ReleaseSign  string
	BuildPath    string // path to store builds, relative to the root of the repo
}

type pkgChecksum struct {
	Name     string
	Checksum string
}

var (
	RepoCfg     RepoConfig
	PkgChecksum []pkgChecksum
	GzHref      string
)

// Packages returns the list of base packages
func Packages() ([]ospackage.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", RepoCfg.PkgList)

	packages, err := ParsePrimary(RepoCfg.PkgPrefix, GzHref, RepoCfg.ReleaseFile, RepoCfg.ReleaseSign, RepoCfg.PbGPGKey, RepoCfg.BuildPath)
	if err != nil {
		log.Errorf("parsing %s failed: %v", GzHref, err)
		return nil, err
	}

	log.Infof("found %d packages in deb repo", len(packages))
	return packages, nil
}

func UserPackages() ([]ospackage.PackageInfo, error) {

	log := logger.Logger()
	log.Infof("fetching packages from %s", "user package list")

	// Declare a list containing 3 repo configs, will be replace by user input
	repoList := []struct {
		id       string
		codename string
		url      string
		pkey     string
	}{
		{id: "testrepo1", codename: "testrepo1", url: "http://localhost:8080", pkey: "http://localhost:8080/public.gpg.key"},
		{id: "testrepo2", codename: "testrepo2", url: "http://localhost:8081", pkey: "http://localhost:8081/public.gpg.key"},
		{id: "openvino", codename: "ubuntu22", url: "https://apt.repos.intel.com/openvino/2024", pkey: "https://apt.repos.intel.com/intel-gpg-keys/GPG-PUB-KEY-INTEL-SW-PRODUCTS.PUB"},
		//{id: "openvino", codename: "ubuntu22", url: "https://apt.repos.intel.com/openvino/2024", pkey: "http://localhost:8080/intel-openvino.gpg"},
	}

	var userRepo []RepoConfig

	for _, repoItem := range repoList {
		id := repoItem.id
		codename := repoItem.codename
		baseURL := repoItem.url
		pkey := repoItem.pkey
		repo := RepoConfig{
			PkgList:      baseURL + "/dists/" + codename + "/main/binary-amd64/Packages.gz",
			ReleaseFile:  fmt.Sprintf("%s/dists/%s/Release", baseURL, codename),
			ReleaseSign:  fmt.Sprintf("%s/dists/%s/Release.gpg", baseURL, codename),
			PkgPrefix:    baseURL,
			Name:         id,
			GPGCheck:     true,
			RepoGPGCheck: true,
			Enabled:      true,
			PbGPGKey:     pkey, //fmt.Sprintf("%s/%s", baseURL, pkNm),
			BuildPath:    "./builds/elxr",
		}
		userRepo = append(userRepo, repo)
	}

	var allUserPackages []ospackage.PackageInfo
	for _, rpItx := range userRepo {

		userPkgs, err := ParsePrimary(rpItx.PkgPrefix, rpItx.PkgList, rpItx.ReleaseFile, rpItx.ReleaseSign, rpItx.PbGPGKey, rpItx.BuildPath)
		if err != nil {
			log.Errorf("parsing user repo failed: %v %s %s", err, rpItx.ReleaseFile, rpItx.ReleaseSign)
			continue
		}
		allUserPackages = append(allUserPackages, userPkgs...)
	}

	// Print all user packages with their name and URL
	// fmt.Printf("\n\nStart:\n\n")
	// for _, pkg := range allUserPackages {
	// 	fmt.Printf("yockgen: Package: %-40s URL: %s\n", pkg.Name, pkg.URL)
	// }
	// fmt.Printf("\n\nEND\n\n")

	return allUserPackages, nil
	// return nil, fmt.Errorf("yockgen: dummy error for testing")
}

// Validate verifies the downloaded files
func Validate(destDir string) error {
	log := logger.Logger()

	// get all DEBs in the destDir
	debPattern := filepath.Join(destDir, "*.deb")
	debPaths, err := filepath.Glob(debPattern)
	if err != nil {
		return fmt.Errorf("glob %q: %w", debPattern, err)
	}
	if len(debPaths) == 0 {
		log.Warn("no DEBs found to verify")
		return nil
	}

	// Create a simple dictionary (map) to store all records from PkgChecksum
	checksumMap := make(map[string]string)
	for _, pc := range PkgChecksum {
		checksumMap[pc.Name] = pc.Checksum
	}

	start := time.Now()
	results := VerifyDEBs(debPaths, checksumMap, 4)
	log.Infof("Debian verification took %s", time.Since(start))

	// Check results
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("deb %s failed verification: %v", r.Path, r.Error)
		}
	}
	log.Info("all DEBs verified successfully")

	return nil
}

// Resolve resolves dependencies
func Resolve(req []ospackage.PackageInfo, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	log.Infof("resolving dependencies for %d DEBIANs", len(req))
	// Resolve all the required dependencies for the initial seed of Debian packages
	needed, err := ResolvePackageInfos(req, all)
	if err != nil {
		log.Errorf("resolving dependencies failed: %v", err)
		return nil, err
	}

	log.Infof("requested %d packages, resolved to %d packages", len(req), len(needed))
	log.Infof("need a total of %d DEBs (including dependencies)", len(needed))

	for _, pkg := range needed {
		log.Debugf("-> %s", filepath.Base(pkg.URL))
	}

	// Adding full packages to the pkgChecksum list
	for _, pkg := range all {
		var sha256 string
		for _, c := range pkg.Checksums {
			if strings.EqualFold(c.Algorithm, "SHA256") {
				sha256 = c.Value
				break
			}
		}
		PkgChecksum = append(PkgChecksum, pkgChecksum{
			Name:     filepath.Base(pkg.URL),
			Checksum: sha256,
		})
	}

	return needed, nil
}

// compareVersions compares two Debian package versions
// Returns 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {

	// Extract version from Debian package names like "acct_6.6.4-5+b1_amd64.deb"
	extractVersion := func(name string) string {
		// Find first underscore, then take everything until the next underscore
		parts := strings.Split(name, "_")
		if len(parts) >= 2 {
			return parts[1] // This is the version part
		}
		return name
	}

	ver1 := extractVersion(v1)
	ver2 := extractVersion(v2)

	// Simple string comparison works for most cases like "6.6.4-5+b1" vs "7.6.4-5+b1"
	if ver1 > ver2 {
		return 1
	} else if ver1 < ver2 {
		return -1
	}
	return 0
}

// MatchRequested matches requested packages
func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	// Write all package info to /data/yockgen/allrepo.txt
	// f, err := os.Create("/data/yockgen/allrepo.txt")
	// if err != nil {
	// 	log.Errorf("failed to create allrepo.txt: %v", err)
	// } else {
	// 	defer f.Close()
	// 	for _, itx := range all {
	// 		// Write package name and URL (if available)
	// 		line := fmt.Sprintf("Name: %s\tURL: %s\n", itx.Name, itx.URL)
	// 		_, _ = f.WriteString(line)
	// 	}
	// }

	var out []ospackage.PackageInfo

	for _, want := range requests {
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
				continue
			}
			// 2) prefix by want-version ("acl-")
			if strings.HasPrefix(pi.Name, want+"-") {
				candidates = append(candidates, pi)
				continue
			}
			// 3) prefix by want.release ("acl-2.3.1-2.")
			if strings.HasPrefix(pi.Name, want+".") {
				candidates = append(candidates, pi)
			}
			// 4) Debian package format (packagename_version_arch.deb)
			if strings.HasPrefix(pi.Name, want+"_") {
				candidates = append(candidates, pi)
			}
		}

		// fmt.Printf("yockgen2: %s %d\n", want, len(candidates))

		if len(candidates) == 0 {
			log.Infof("requested package %q not found in repo", want)
			continue
		}
		// If we got an exact match in step (1), it's the only candidate
		if len(candidates) == 1 && (candidates[0].Name == want || candidates[0].Name == want+".deb") {
			out = append(out, candidates[0])
			continue
		}

		// Sort by version (highest version first)
		sort.Slice(candidates, func(i, j int) bool {
			return compareVersions(candidates[i].URL, candidates[j].URL) > 0
		})
		out = append(out, candidates[0])
	}

	// fmt.Printf("\n\nStart:\n\n")
	// for itx := range out {
	// 	fmt.Printf("yockgen: Found package: %s %s %s\n", out[itx].Name, out[itx].Version, out[itx].URL)
	// }
	// fmt.Printf("\n\nEND\n\n")
	// return nil, fmt.Errorf("yockgen test failed")

	log.Infof("found %d packages in request of %d", len(out), len(requests))
	return out, nil
}

func MatchRequested1(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	var out []ospackage.PackageInfo

	for _, want := range requests {
		var candidates []ospackage.PackageInfo
		for _, pi := range all {
			// 1) exact name match
			if pi.Name == want || pi.Name == want+".deb" {
				candidates = append(candidates, pi)
				break
			}
			// 2) prefix by want-version ("acl-")
			if strings.HasPrefix(pi.Name, want+"-") {
				candidates = append(candidates, pi)
				continue
			}
			// 3) prefix by want.release ("acl-2.3.1-2.")
			if strings.HasPrefix(pi.Name, want+".") {
				candidates = append(candidates, pi)
			}
		}

		if len(candidates) == 0 {
			log.Infof("requested package %q not found in repo", want)
			continue
		}
		// If we got an exact match in step (1), it's the only candidate
		if len(candidates) == 1 && (candidates[0].Name == want || candidates[0].Name == want+".deb") {
			out = append(out, candidates[0])
			continue
		}
		// Otherwise pick the "highest" by lex sort
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Name > candidates[j].Name
		})
		out = append(out, candidates[0])
	}

	log.Infof("found %d packages in request of %d", len(out), len(requests))
	return out, nil
}

func DownloadPackages(pkgList []string, destDir string, dotFile string) ([]string, error) {
	var downloadPkgList []string

	log := logger.Logger()

	// Fetch the entire base package list
	all, err := Packages()
	if err != nil {
		return downloadPkgList, fmt.Errorf("getting packages: %v", err)
	}

	// Fetch the entire user repos package list
	userpkg, err := UserPackages()
	if err == nil {
		all = append(all, userpkg...)
	}

	if err != nil {
		log.Warnf("getting user packages failed: %v", err)
		// Continue even if user packages failed
		return downloadPkgList, fmt.Errorf("User package fetch failed: %v", err)

	}

	// Match the packages in the template against all the packages
	req, err := MatchRequested(pkgList, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("matching packages: %v", err)
	}
	log.Infof("matched a total of %d packages", len(req))

	for _, pkg := range req {
		log.Debugf("-> %s", filepath.Base(pkg.URL))
	}

	// Resolve the dependencies of the requested packages
	needed, err := Resolve(req, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving packages: %v", err)
	}
	log.Infof("resolved %d packages", len(needed))

	// Generate SPDX manifest, generated in temp directory
	spdxFile := filepath.Join(config.TempDir(), manifest.DefaultSPDXFile)
	if err := manifest.WriteSPDXToFile(needed, spdxFile); err != nil {
		return downloadPkgList, fmt.Errorf("SPDX file: %v", err)
	}

	sorted_pkgs, err := pkgsorter.SortPackages(needed)
	if err != nil {
		log.Errorf("sorting packages: %v", err)
	}
	log.Infof("sorted %d packages for installation", len(sorted_pkgs))

	// If a dot file is specified, generate the dependency graph
	if dotFile != "" {
		if err := GenerateDot(needed, dotFile); err != nil {
			log.Errorf("generating dot file: %v", err)
		}
	}

	// Extract URLs
	urls := make([]string, len(sorted_pkgs))
	for i, pkg := range sorted_pkgs {
		urls[i] = pkg.URL
		downloadPkgList = append(downloadPkgList, filepath.Base(pkg.URL))
	}

	// Ensure dest directory exists
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving cache directory: %v", err)
	}
	if err := os.MkdirAll(absDestDir, 0755); err != nil {
		return downloadPkgList, fmt.Errorf("creating cache directory %s: %v", absDestDir, err)
	}

	// Download packages using configured workers and cache directory
	log.Infof("downloading %d packages to %s using %d workers", len(urls), absDestDir, config.Workers())
	if err := pkgfetcher.FetchPackages(urls, absDestDir, config.Workers()); err != nil {
		return downloadPkgList, fmt.Errorf("fetch failed: %v", err)
	}
	log.Info("all downloads complete")

	// Verify downloaded packages
	if err := Validate(destDir); err != nil {
		return downloadPkgList, fmt.Errorf("verification failed: %v", err)
	}

	return downloadPkgList, nil
}
