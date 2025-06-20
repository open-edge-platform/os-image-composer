package rpmutils

import (
	"fmt"
	"io"
	"net/http"
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

// repoConfig holds .repo file values
type RepoConfig struct {
	Section      string // raw section header
	Name         string // human-readable name from name=
	URL          string
	GPGCheck     bool
	RepoGPGCheck bool
	Enabled      bool
	GPGKey       string
}

var (
	RepoCfg RepoConfig
	GzHref  string
)

func Packages() ([]ospackage.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", RepoCfg.URL)

	packages, err := ParsePrimary(RepoCfg.URL, GzHref)
	if err != nil {
		log.Errorf("parsing primary.xml.gz failed: %v", err)
		return nil, err
	}

	log.Infof("found %d packages in rpm repo", len(packages))
	return packages, nil
}

func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	var out []ospackage.PackageInfo

	for _, want := range requests {
		var candidates []ospackage.PackageInfo
		for _, pi := range all {
			// 1) exact name match
			if pi.Name == want || pi.Name == want+".rpm" {
				candidates = append(candidates, pi)
				break
			}
			// 2) prefix by want-version ("acl-")
			// Only match if the part after "want-" is a version (starts with a digit)
			// prevent getting acl-dev when asking for acl-9.2
			if strings.HasPrefix(pi.Name, want+"-") {
				rest := strings.TrimPrefix(pi.Name, want+"-")
				if isValidVersionFormat(rest) {
					candidates = append(candidates, pi)
					continue
				}
			}
			// 3) prefix by want.release ("acl-2.3.1-2.")
			if strings.HasPrefix(pi.Name, want+".") {
				candidates = append(candidates, pi)
			}
		}

		if len(candidates) == 0 {
			return nil, fmt.Errorf("requested package %q not found in repo", want)
		}
		// If we got an exact match in step (1), it's the only candidate
		if len(candidates) == 1 && (candidates[0].Name == want || candidates[0].Name == want+".rpm") {
			out = append(out, candidates[0])
			continue
		}
		// Otherwise pick the "highest" by lex sort
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Name > candidates[j].Name
		})
		out = append(out, candidates[0])
	}
	return out, nil
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isValidVersionFormat(s string) bool {
	// Check if the string is all digits up to the next '.'
	dotIdx := strings.IndexByte(s, '.')
	var prefix string
	if dotIdx == -1 {
		prefix = s
	} else {
		prefix = s[:dotIdx]
	}
	if len(prefix) > 0 && isAllDigits(prefix) {
		return true
	}
	// If we reach here, the format is not valid
	return false
}

func Validate(destDir string) error {
	log := logger.Logger()

	// read the GPG key from the repo config
	resp, err := http.Get(RepoCfg.GPGKey)
	if err != nil {
		return fmt.Errorf("fetch GPG key %s: %w", RepoCfg.GPGKey, err)
	}
	defer resp.Body.Close()

	keyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read GPG key body: %w", err)
	}
	log.Infof("fetched GPG key (%d bytes)", len(keyBytes))
	log.Debugf("GPG key: %s\n", keyBytes)

	// store in a temp file
	tmp, err := os.CreateTemp("", "azurelinux-gpg-*.asc")
	if err != nil {
		return fmt.Errorf("create temp key file: %w", err)
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(keyBytes); err != nil {
		return fmt.Errorf("write key to temp file: %w", err)
	}

	// get all RPMs in the destDir
	rpmPattern := filepath.Join(destDir, "*.rpm")
	rpmPaths, err := filepath.Glob(rpmPattern)
	if err != nil {
		return fmt.Errorf("glob %q: %w", rpmPattern, err)
	}
	if len(rpmPaths) == 0 {
		log.Warn("no RPMs found to verify")
		return nil
	}

	start := time.Now()
	results := VerifyAll(rpmPaths, tmp.Name(), 4)
	log.Infof("RPM verification took %s", time.Since(start))

	// Check results
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("RPM %s failed verification: %v", r.Path, r.Error)
		}
	}
	log.Info("all RPMs verified successfully")

	return nil
}

func Resolve(req []ospackage.PackageInfo, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	log.Infof("resolving dependencies for %d RPMs", len(req))

	// Resolve all the required dependencies for the initial seed of RPMs
	needed, err := ResolvePackageInfos(req, all)
	if err != nil {
		log.Errorf("resolving dependencies failed: %v", err)
		return nil, err
	}
	log.Infof("need a total of %d RPMs (including dependencies)", len(needed))

	for _, pkg := range needed {
		log.Debugf("-> %s", pkg.Name)
	}

	return needed, nil
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
