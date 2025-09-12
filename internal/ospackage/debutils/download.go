package debutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/config/manifest"
	"github.com/open-edge-platform/image-composer/internal/ospackage"
	"github.com/open-edge-platform/image-composer/internal/ospackage/pkgfetcher"
	"github.com/open-edge-platform/image-composer/internal/ospackage/pkgsorter"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/slice"
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
	Arch         string // architecture, e.g., amd64, all
}

type pkgChecksum struct {
	Name     string
	Checksum string
}

var (
	RepoCfg      RepoConfig
	PkgChecksum  []pkgChecksum
	GzHref       string
	Architecture string
	UserRepo     []config.PackageRepository
	ReportPath   = "builds"
)

// Packages returns the list of base packages
func Packages() ([]ospackage.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", RepoCfg.PkgList)

	packages, err := ParseRepositoryMetadata(RepoCfg.PkgPrefix, GzHref, RepoCfg.ReleaseFile, RepoCfg.ReleaseSign, RepoCfg.PbGPGKey, RepoCfg.BuildPath, RepoCfg.Arch)
	if err != nil {
		return nil, fmt.Errorf("parsing default repo failed: %w", err)
	}

	log.Infof("found %d packages in deb repo", len(packages))
	return packages, nil
}

func UserPackages() ([]ospackage.PackageInfo, error) {

	log := logger.Logger()
	log.Infof("fetching packages from %s", "user package list")

	repoList := make([]struct {
		id        string
		codename  string
		url       string
		pkey      string
		component string
	}, len(UserRepo))
	for i, repo := range UserRepo {
		repoList[i] = struct {
			id        string
			codename  string
			url       string
			pkey      string
			component string
		}{
			id:        fmt.Sprintf("custrepo%d", i+1),
			codename:  repo.Codename,
			url:       repo.URL,
			pkey:      repo.PKey,
			component: repo.Component,
		}
	}

	var userRepo []RepoConfig
	for _, repoItem := range repoList {
		id := repoItem.id
		codename := repoItem.codename
		baseURL := repoItem.url
		pkey := repoItem.pkey
		archs := Architecture + ",all"
		releaseNm := "Release"
		component := repoItem.component
		if strings.TrimSpace(component) == "" {
			component = "main"
		}
		for _, componentName := range slice.SplitBySpace(component) {
			for _, arch := range strings.Split(archs, ",") {
				package_list_url := GetPackagesNames(baseURL, codename, arch, componentName)
				if package_list_url == "" {
					continue // No valid package list found for this arch/component
				}
				repo := RepoConfig{
					PkgList:      package_list_url,
					ReleaseFile:  fmt.Sprintf("%s/dists/%s/%s", baseURL, codename, releaseNm),
					ReleaseSign:  fmt.Sprintf("%s/dists/%s/%s.gpg", baseURL, codename, releaseNm),
					PkgPrefix:    baseURL,
					Name:         id,
					GPGCheck:     true,
					RepoGPGCheck: true,
					Enabled:      true,
					PbGPGKey:     pkey,
					BuildPath:    fmt.Sprintf("./builds/%s_%s_%s", id, arch, componentName),
					Arch:         arch,
				}
				userRepo = append(userRepo, repo)
			}
		}
	}

	var allUserPackages []ospackage.PackageInfo
	for _, rpItx := range userRepo {

		userPkgs, err := ParseRepositoryMetadata(rpItx.PkgPrefix, rpItx.PkgList, rpItx.ReleaseFile, rpItx.ReleaseSign, rpItx.PbGPGKey, rpItx.BuildPath, rpItx.Arch)
		if err != nil {
			return nil, fmt.Errorf("parsing user repo failed: %w", err)
		}
		allUserPackages = append(allUserPackages, userPkgs...)
	}

	return allUserPackages, nil
}

// CheckFileExists sends a HEAD request to the given URL and
// returns true if the file exists (status 200).
func checkFileExists(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Validate verifies the downloaded files
func Validate(destDir string, downloadPkgList []string) error {
	log := logger.Logger()

	// get all DEBs in the destDir
	debPattern := filepath.Join(destDir, "*.deb")
	debPaths, err := filepath.Glob(debPattern)
	if err == nil {
		// Filter debPaths to only those in downloadPkgList
		downloadSet := make(map[string]struct{}, len(downloadPkgList))
		for _, name := range downloadPkgList {
			downloadSet[name] = struct{}{}
		}
		var filtered []string
		for _, path := range debPaths {
			if _, ok := downloadSet[filepath.Base(path)]; ok {
				filtered = append(filtered, path)
			}
		}
		debPaths = filtered
	} else {
		return fmt.Errorf("glob %q: %w", debPattern, err)
	}
	// If no DEBs found, log a warning and return
	if len(debPaths) == 0 {
		log.Warn("no DEBs found to verify")
		return nil
	}

	// Create a simple dictionary (map) to store all records from PkgChecksum
	checksumMap := make(map[string][]string)
	for _, pc := range PkgChecksum {
		checksumMap[pc.Name] = append(checksumMap[pc.Name], pc.Checksum)
	}

	start := time.Now()
	results := VerifyDEBs(debPaths, checksumMap, 4)
	log.Infof("Debian verification took %s", time.Since(start))

	// Check results
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("deb %s failed verification: %w", r.Path, r.Error)
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
	needed, err := ResolveDependencies(req, all)
	if err != nil {
		log.Debugf("resolving dependencies failed: %v", err)
		return nil, fmt.Errorf("resolving dependencies failed: %w", err)
	}

	log.Infof("requested %d packages, resolved to %d packages", len(req), len(needed))
	log.Infof("need a total of %d DEBs (including dependencies)", len(needed))

	for _, pkg := range needed {
		log.Debugf("%s %s -> %s", pkg.Name, pkg.Version, filepath.Base(pkg.URL))
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

// MatchRequested matches requested packages
func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	log := logger.Logger()

	var out []ospackage.PackageInfo
	var requestedPkgs []string
	gotMissingPkg := false

	for _, want := range requests {
		if pkg, found := ResolveTopPackageConflicts(want, all); found {
			out = append(out, pkg)
		} else {
			requestedPkgs = append(requestedPkgs, want)
			log.Warnf("requested package '%q' not found in repo", want)
			gotMissingPkg = true
		}
	}

	log.Infof("found %d packages in request of %d", len(out), len(requests))
	if gotMissingPkg {
		report, err := WriteArrayToFile(requestedPkgs, "Missing Requested Packages")
		if err != nil {
			return out, fmt.Errorf("writing missing packages report failed: %w", err)
		}
		return out, fmt.Errorf("one or more requested packages not found. See list in %s", report)
	}
	return out, nil
}

// WriteArrayToFile writes the contents of arr to a JSON file.
// The file will contain a report_type and a "missing" array of strings.
// The filename will be prefixed with the current date and time in "YYYYMMDD_HHMMSS_" format.
func WriteArrayToFile(arr []string, title string) (string, error) {
	now := time.Now()
	if err := os.MkdirAll(ReportPath, 0755); err != nil {
		return "", fmt.Errorf("creating base path: %w", err)
	}
	filename := filepath.Join(ReportPath, fmt.Sprintf("%s_%s.json", strings.ReplaceAll(title, " ", "_"), now.Format("20060102_150405")))

	// Ensure "report_type" is the first key in the output
	type reportStruct struct {
		ReportType string   `json:"report_type"`
		Missing    []string `json:"missing"`
	}

	report := reportStruct{
		ReportType: "missing_packages_report",
		Missing:    arr,
	}

	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return "", fmt.Errorf("writing json: %w", err)
	}

	return filename, nil
}

func DownloadPackages(pkgList []string, destDir string, dotFile string) ([]string, error) {
	var downloadPkgList []string

	log := logger.Logger()

	// Fetch the entire base package list
	all, err := Packages()
	if err != nil {
		return downloadPkgList, fmt.Errorf("getting packages: %w", err)
	}

	// Fetch the entire user repos package list
	userpkg, err := UserPackages()
	if err != nil {
		log.Debugf("getting user packages failed: %v", err)
		return downloadPkgList, fmt.Errorf("user package fetch failed: %w", err)
	}
	all = append(all, userpkg...)

	// Match the packages in the template against all the packages
	req, err := MatchRequested(pkgList, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("matching packages: %w", err)
	}
	log.Infof("matched a total of %d packages", len(req))

	// Resolve the dependencies of the requested packages
	needed, err := Resolve(req, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving packages: %w", err)
	}
	log.Infof("resolved %d packages", len(needed))

	// Generate SPDX manifest, generated in temp directory
	spdxFile := filepath.Join(config.TempDir(), manifest.DefaultSPDXFile)
	if err := manifest.WriteSPDXToFile(needed, spdxFile); err != nil {
		return downloadPkgList, fmt.Errorf("SPDX file: %w", err)
	}

	sorted_pkgs, err := pkgsorter.SortPackages(needed)
	if err != nil {
		log.Debugf("sorting packages: %w", err)
	}
	log.Infof("sorted %d packages for installation", len(sorted_pkgs))

	// If a dot file is specified, generate the dependency graph
	if dotFile != "" {
		if err := GenerateDot(needed, dotFile); err != nil {
			log.Debugf("generating dot file: %w", err)
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
		return downloadPkgList, fmt.Errorf("resolving cache directory: %w", err)
	}
	if err := os.MkdirAll(absDestDir, 0755); err != nil {
		return downloadPkgList, fmt.Errorf("creating cache directory %s: %w", absDestDir, err)
	}

	// Download packages using configured workers and cache directory
	log.Infof("downloading %d packages to %s using %d workers", len(urls), absDestDir, config.Workers())
	if err := pkgfetcher.FetchPackages(urls, absDestDir, config.Workers()); err != nil {
		return downloadPkgList, fmt.Errorf("fetch failed: %w", err)
	}
	log.Info("all downloads complete")

	// Verify downloaded packages
	if err := Validate(destDir, downloadPkgList); err != nil {
		return downloadPkgList, fmt.Errorf("verification failed: %w", err)
	}

	return downloadPkgList, nil
}
