package debutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/config"
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

// Packages returns the list of packages
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
		log.Debugf("-> %s", pkg.Name)
	}

	// Adding needed packages to the pkgChecksum list
	for _, pkg := range needed {
		PkgChecksum = append(PkgChecksum, pkgChecksum{
			Name:     pkg.Name,
			Checksum: pkg.Checksum,
		})
	}

	return needed, nil
}

// MatchRequested matches requested packages
func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
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
	// Fetch the entire package list
	all, err := Packages()
	if err != nil {
		return downloadPkgList, fmt.Errorf("getting packages: %v", err)
	}

	// Match the packages in the template against all the packages
	req, err := MatchRequested(pkgList, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("matching packages: %v", err)
	}
	log.Infof("matched a total of %d packages", len(req))

	for _, pkg := range req {
		log.Debugf("-> %s", pkg.Name)
	}

	// Resolve the dependencies of the requested packages
	needed, err := Resolve(req, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving packages: %v", err)
	}
	log.Infof("resolved %d packages", len(needed))

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
		downloadPkgList = append(downloadPkgList, pkg.Name)
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
