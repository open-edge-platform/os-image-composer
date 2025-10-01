package rpmutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/config/manifest"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/pkgfetcher"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/pkgsorter"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/network"
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
	RepoCfg  RepoConfig
	GzHref   string
	UserRepo []config.PackageRepository
)

func Packages() ([]ospackage.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", RepoCfg.URL)

	packages, err := ParseRepositoryMetadata(RepoCfg.URL, GzHref)
	if err != nil {
		log.Errorf("parsing primary.xml.gz failed: %v", err)
		return nil, err
	}

	log.Infof("found %d packages in rpm repo", len(packages))
	return packages, nil
}

func UserPackages() ([]ospackage.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", "user package list")

	repoList := make([]struct {
		id       string
		codename string
		url      string
		pkey     string
	}, len(UserRepo))
	for i, repo := range UserRepo {
		repoList[i] = struct {
			id       string
			codename string
			url      string
			pkey     string
		}{
			id:       fmt.Sprintf("rpmcustrepo%d", i+1),
			codename: repo.Codename,
			url:      repo.URL,
			pkey:     repo.PKey,
		}
	}

	var userRepo []RepoConfig
	for _, repoItem := range repoList {
		id := repoItem.id
		codename := repoItem.codename
		baseURL := repoItem.url
		pkey := repoItem.pkey

		repo := RepoConfig{
			Name:         id,
			GPGCheck:     true,
			RepoGPGCheck: true,
			Enabled:      true,
			GPGKey:       pkey,
			URL:          baseURL,
			Section:      fmt.Sprintf("[%s]", codename),
		}

		userRepo = append(userRepo, repo)
	}

	metadataXmlPath := "repodata/repomd.xml"
	var allUserPackages []ospackage.PackageInfo
	for _, rpItx := range userRepo {

		repoMetaDataURL := GetRepoMetaDataURL(rpItx.URL, metadataXmlPath)
		if repoMetaDataURL == "" {
			log.Errorf("invalid repo metadata URL: %s/%s, skipping", rpItx.URL, metadataXmlPath)
			return nil, fmt.Errorf("invalid repo metadata URL: %s/%s", rpItx.URL, metadataXmlPath)
		}

		primaryXmlURL, err := FetchPrimaryURL(repoMetaDataURL)
		if err != nil {
			return nil, fmt.Errorf("fetching %s URL failed: %w", repoMetaDataURL, err)
		}

		userPkgs, err := ParseRepositoryMetadata(rpItx.URL, primaryXmlURL)
		if err != nil {
			return nil, fmt.Errorf("parsing user repo failed: %w", err)
		}
		allUserPackages = append(allUserPackages, userPkgs...)
	}

	return allUserPackages, nil

	// for _, pkg := range allUserPackages {
	// 	log.Debugf("rpm pkg -> %s %s %s", pkg.Name, pkg.Version, pkg.URL)
	// }
	// return []ospackage.PackageInfo{}, nil //fmt.Errorf("yockgen user package fetching not supported for rpm")
}

func MatchRequested(requests []string, all []ospackage.PackageInfo) ([]ospackage.PackageInfo, error) {
	var out []ospackage.PackageInfo

	for _, want := range requests {
		var candidates []ospackage.PackageInfo
		for _, pi := range all {
			if pi.Arch == "src" {
				continue
			}
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

func isAcceptedChar(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '-' {
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
	if len(prefix) > 0 && isAcceptedChar(prefix) {
		return true
	}
	// If we reach here, the format is not valid
	return false
}

// createTempGPGKeyFiles downloads multiple GPG keys from URLs and creates temporary files.
// Returns the file paths and a cleanup function. The caller is responsible for calling cleanup.
func createTempGPGKeyFiles(gpgKeyURLs []string) (keyPaths []string, cleanup func(), err error) {
	log := logger.Logger()

	if len(gpgKeyURLs) == 0 {
		return nil, nil, fmt.Errorf("no GPG key URLs provided")
	}

	var tempFiles []*os.File
	var filePaths []string

	client := network.NewSecureHTTPClient()

	// Download and create temp files for each GPG key
	for i, gpgKeyURL := range gpgKeyURLs {
		resp, err := client.Get(gpgKeyURL)
		if err != nil {
			// Cleanup any files created so far
			for _, f := range tempFiles {
				f.Close()
				os.Remove(f.Name())
			}
			return nil, nil, fmt.Errorf("fetch GPG key %s: %w", gpgKeyURL, err)
		}

		keyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			// Cleanup any files created so far
			for _, f := range tempFiles {
				f.Close()
				os.Remove(f.Name())
			}
			return nil, nil, fmt.Errorf("read GPG key body from %s: %w", gpgKeyURL, err)
		}

		log.Infof("fetched GPG key %d (%d bytes) from %s", i+1, len(keyBytes), gpgKeyURL)

		// Create temp file with unique pattern
		tmp, err := os.CreateTemp("", fmt.Sprintf("azurelinux-gpg-%d-*.asc", i))
		if err != nil {
			// Cleanup any files created so far
			for _, f := range tempFiles {
				f.Close()
				os.Remove(f.Name())
			}
			return nil, nil, fmt.Errorf("create temp key file %d: %w", i, err)
		}

		if _, err := tmp.Write(keyBytes); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			// Cleanup any files created so far
			for _, f := range tempFiles {
				f.Close()
				os.Remove(f.Name())
			}
			return nil, nil, fmt.Errorf("write key to temp file %d: %w", i, err)
		}

		tempFiles = append(tempFiles, tmp)
		filePaths = append(filePaths, tmp.Name())
	}

	cleanup = func() {
		for _, f := range tempFiles {
			f.Close()
			os.Remove(f.Name())
		}
	}

	return filePaths, cleanup, nil
}

func Validate(destDir string) error {
	log := logger.Logger()

	// Collect all GPG key URLs (could be from RepoCfg and UserRepo)
	var gpgKeyURLs []string

	// Add main repo GPG key
	if RepoCfg.GPGKey != "" {
		gpgKeyURLs = append(gpgKeyURLs, RepoCfg.GPGKey)
	}

	//Add user repo GPG keys
	for _, userRepo := range UserRepo {
		if userRepo.PKey != "" {
			gpgKeyURLs = append(gpgKeyURLs, userRepo.PKey)
		} else {
			return fmt.Errorf("no GPG key URL configured for user repo: %s", userRepo.URL)
		}
	}

	if len(gpgKeyURLs) == 0 {
		return fmt.Errorf("no GPG keys configured for verification")
	}

	// Create temporary GPG key files
	gpgKeyPaths, cleanup, err := createTempGPGKeyFiles(gpgKeyURLs)
	if err != nil {
		return fmt.Errorf("failed to create temp GPG key files: %w", err)
	}
	defer cleanup()

	log.Infof("created %d temporary GPG key files for verification", len(gpgKeyPaths))

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
	results := VerifyAll(rpmPaths, gpgKeyPaths, 4)
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
	needed, err := ResolveDependencies(req, all)
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

func DownloadPackages(pkgList []string, destDir, dotFile string) ([]string, error) {
	var downloadPkgList []string

	log := logger.Logger()
	// Fetch the entire package list
	all, err := Packages()
	if err != nil {
		log.Errorf("base packages fetch failed: %v", err)
		return downloadPkgList, fmt.Errorf("base package fetch failed: %v", err)
	}

	// Fetch the entire user repos package list
	userpkg, err := UserPackages()
	if err != nil {
		log.Errorf("getting user packages failed: %v", err)
		return downloadPkgList, fmt.Errorf("user package fetch failed: %w", err)
	}
	all = append(all, userpkg...)

	// Match the packages in the template against all the packages
	req, err := MatchRequested(pkgList, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("matching packages: %v", err)
	}
	log.Infof("Matched a total of %d packages", len(req))

	for _, pkg := range req {
		log.Debugf("-> %s", pkg.Name)
	}

	// Resolve the dependencies of the requested packages
	needed, err := Resolve(req, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving packages: %v", err)
	}

	// Generate SPDX manifest, generated in temp directory
	spdxFile := filepath.Join(config.TempDir(), manifest.DefaultSPDXFile)
	if err := manifest.WriteSPDXToFile(needed, spdxFile); err != nil {
		return downloadPkgList, fmt.Errorf("SPDX file: %v", err)
	}

	sorted_pkgs, err := pkgsorter.SortPackages(needed)
	if err != nil {
		log.Errorf("sorting packages: %v", err)
	}
	log.Infof("Sorted %d packages for installation", len(sorted_pkgs))

	// If a dot file is specified, generate the dependency graph
	if dotFile != "" {
		if err := GenerateDot(sorted_pkgs, dotFile); err != nil {
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
	log.Infof("Downloading %d packages to %s using %d workers", len(urls), absDestDir, config.Workers())
	if err := pkgfetcher.FetchPackages(urls, absDestDir, config.Workers()); err != nil {
		return downloadPkgList, fmt.Errorf("fetch failed: %v", err)
	}
	log.Info("All downloads complete")

	// Verify downloaded packages
	if err := Validate(destDir); err != nil {
		return downloadPkgList, fmt.Errorf("verification failed: %v", err)
	}

	return downloadPkgList, nil
}
