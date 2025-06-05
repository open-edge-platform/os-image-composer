package rpmutils

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/pkg"
	"github.com/open-edge-platform/image-composer/internal/utils/pkg/pkgfetcher"
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
	repoCfg RepoConfig
	gzHref  string
)

func getCurrentDirPath() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current directory path")
	}
	return filepath.Dir(filename), nil
}

// This func should be in the config package
func GetTargetOsConfigDir(targetOs string, targetDist string) (string, error) {
	currentPath, err := getCurrentDirPath()
	if err != nil {
		return "", err
	}
	pkgPath := filepath.Dir(currentPath)
	internalPath := filepath.Dir(pkgPath)
	rootPath := filepath.Dir(internalPath)
	osvConfigDir := filepath.Join(rootPath, "config", "osv")
	targetOsConfigDir := filepath.Join(osvConfigDir, targetOs, targetDist)
	if _, err := os.Stat(targetOsConfigDir); os.IsNotExist(err) {
		return "", fmt.Errorf("target OS configuration directory %s does not exist", targetOsConfigDir)
	}
	return targetOsConfigDir, nil
}

func RepoInit(targetOs string, targetDist string, targetArch string) error {
	log := logger.Logger()

	targetOsConfigDir, err := GetTargetOsConfigDir(targetOs, targetDist)
	if err != nil {
		return fmt.Errorf("failed to get target OS config directory: %v", err)
	}
	repoConfigPath := filepath.Join(targetOsConfigDir, "providerconfigs", "repo.yml")
	if _, err := os.Stat(repoConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("repo configuration file %s does not exist", repoConfigPath)
	}

	// The base Url should be fetched from the repo config file, implement later
	// Should support multiple repo.
	var baseUrl string
	if targetOs == "azure-linux" {
		baseUrl = "https://packages.microsoft.com/azurelinux/3.0/prod/base/"
	} else {
		return fmt.Errorf("unsupported OS %s for RPM repository initialization", targetOs)
	}

	configName := "config.repo"
	repodata := "repodata/repomd.xml"
	repoURL := baseUrl + targetArch + "/" + configName

	resp, err := http.Get(repoURL)
	if err != nil {
		log.Errorf("downloading repo config %s failed: %v", repoURL, err)
		return err
	}
	defer resp.Body.Close()

	cfg, err := loadRepoConfig(resp.Body)
	if err != nil {
		log.Errorf("parsing repo config failed: %v", err)
		return err
	}

	repoDataURL := baseUrl + targetArch + "/" + repodata
	href, err := fetchPrimaryURL(repoDataURL)
	if err != nil {
		log.Errorf("fetch primary.xml.gz failed: %v", err)
		return err
	}

	repoCfg = cfg
	gzHref = href

	log.Infof("initialized rpm repo section=%s", cfg.Section)
	log.Infof("name=%s", cfg.Name)
	log.Infof("url=%s", cfg.URL)
	log.Infof("primary.xml.gz=%s", gzHref)
	return nil
}

func Packages() ([]pkg.PackageInfo, error) {
	log := logger.Logger()
	log.Infof("fetching packages from %s", repoCfg.URL)

	packages, err := ParsePrimary(repoCfg.URL, gzHref)
	if err != nil {
		log.Errorf("parsing primary.xml.gz failed: %v", err)
		return nil, err
	}

	log.Infof("found %d packages in rpm repo", len(packages))
	return packages, nil
}

func MatchRequested(requests []string, all []pkg.PackageInfo) ([]pkg.PackageInfo, error) {
	var out []pkg.PackageInfo

	for _, want := range requests {
		var candidates []pkg.PackageInfo
		for _, pi := range all {
			// 1) exact name match
			if pi.Name == want || pi.Name == want+".rpm" {
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

func Validate(destDir string) error {
	log := logger.Logger()

	// read the GPG key from the repo config
	resp, err := http.Get(repoCfg.GPGKey)
	if err != nil {
		return fmt.Errorf("fetch GPG key %s: %w", repoCfg.GPGKey, err)
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

func Resolve(req []pkg.PackageInfo, all []pkg.PackageInfo) ([]pkg.PackageInfo, error) {
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

// fetchPrimaryURL downloads repomd.xml and returns the href of the primary metadata.
func fetchPrimaryURL(repomdURL string) (string, error) {
	resp, err := http.Get(repomdURL)
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

// loadRepoConfig parses the repo configuration data
func loadRepoConfig(r io.Reader) (RepoConfig, error) {
	s := bufio.NewScanner(r)
	var rc RepoConfig
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		// skip comments or empty
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			rc.Section = strings.Trim(line, "[]")
			continue
		}
		// key=value lines
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			rc.Name = val
		case "baseurl":
			rc.URL = val
		case "gpgcheck":
			rc.GPGCheck = (val == "1")
		case "repo_gpgcheck":
			rc.RepoGPGCheck = (val == "1")
		case "enabled":
			rc.Enabled = (val == "1")
		case "gpgkey":
			rc.GPGKey = val
		}
	}
	if err := s.Err(); err != nil {
		return rc, err
	}
	return rc, nil
}

func DownloadPackages(pkgList []string, destDir string, dotFile string, verbose bool) ([]string, error) {
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
	if verbose {
		for _, pkg := range req {
			log.Infof("-> %s", pkg.Name)
		}
	}

	// Resolve the dependencies of the requested packages
	needed, err := Resolve(req, all)
	if err != nil {
		return downloadPkgList, fmt.Errorf("resolving packages: %v", err)
	}
	log.Infof("resolved %d packages", len(needed))

	// If a dot file is specified, generate the dependency graph
	if dotFile != "" {
		if err := GenerateDot(needed, dotFile); err != nil {
			log.Errorf("generating dot file: %v", err)
		}
	}

	// Extract URLs
	urls := make([]string, len(needed))
	for i, pkg := range needed {
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
	log.Infof("downloading %d packages to %s using %d workers", len(urls), absDestDir, config.GlConfig.Workers)
	if err := pkgfetcher.FetchPackages(urls, absDestDir, config.GlConfig.Workers); err != nil {
		return downloadPkgList, fmt.Errorf("fetch failed: %v", err)
	}
	log.Info("all downloads complete")

	// Verify downloaded packages
	if err := Validate(destDir); err != nil {
		return downloadPkgList, fmt.Errorf("verification failed: %v", err)
	}

	return downloadPkgList, nil
}
