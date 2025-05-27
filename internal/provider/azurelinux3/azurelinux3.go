package azurelinux3

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/rpmutils"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
)

const (
	baseURL    = "https://packages.microsoft.com/azurelinux/3.0/prod/base/"
	configName = "config.repo"
	repodata   = "repodata/repomd.xml"
)

// repoConfig holds .repo file values
type repoConfig struct {
	Section      string // raw section header
	Name         string // human-readable name from name=
	URL          string
	GPGCheck     bool
	RepoGPGCheck bool
	Enabled      bool
	GPGKey       string
}

// AzureLinux3 implements provider.Provider
type AzureLinux3 struct {
	repoURL string
	repoCfg repoConfig
	//repomd		string
	//primaryURL	string
	gzHref string
	spec   *config.BuildSpec
}

func init() {
	provider.Register(&AzureLinux3{})
}

// Name returns the unique name of the provider
func (p *AzureLinux3) Name() string { return "AzureLinux3" }

// Init will initialize the provider, fetching repo configuration
func (p *AzureLinux3) Init(spec *config.BuildSpec) error {

	logger := utils.Logger()
	p.repoURL = baseURL + spec.Arch + "/" + configName

	resp, err := http.Get(p.repoURL)
	if err != nil {
		logger.Errorf("downloading repo config %s failed: %v", p.repoURL, err)
		return err
	}
	defer resp.Body.Close()

	cfg, err := loadRepoConfig(resp.Body)
	if err != nil {
		logger.Errorf("parsing repo config failed: %v", err)
		return err
	}

	repoDataURL := baseURL + spec.Arch + "/" + repodata
	href, err := fetchPrimaryURL(repoDataURL)
	if err != nil {
		logger.Errorf("fetch primary.xml.gz failed: %v", err)
	}

	p.repoCfg = cfg
	p.spec = spec
	p.gzHref = href

	logger.Infof("initialized AzureLinux3 provider repo section=%s", cfg.Section)
	logger.Infof("name=%s", cfg.Name)
	logger.Infof("url=%s", cfg.URL)
	logger.Infof("primary.xml.gz=%s", p.gzHref)
	return nil
}
func (p *AzureLinux3) Packages() ([]provider.PackageInfo, error) {
	logger := utils.Logger()
	logger.Infof("fetching packages from %s", p.repoCfg.URL)

	packages, err := rpmutils.ParsePrimary(p.repoCfg.URL, p.gzHref)
	if err != nil {
		logger.Errorf("parsing primary.xml.gz failed: %v", err)
	}

	logger.Infof("found %d packages in AzureLinux3 repo", len(packages))
	return packages, nil
}

func (p *AzureLinux3) MatchRequested(requests []string, all []provider.PackageInfo) ([]provider.PackageInfo, error) {
	var out []provider.PackageInfo

	for _, want := range requests {
		var candidates []provider.PackageInfo
		for _, pi := range all {
			// 1) exact name match
			if pi.Name == want || pi.Name == want+".rpm" {
				candidates = append(candidates, pi)
				break
			}
			// 2) prefix by want-version (“acl-”)
			if strings.HasPrefix(pi.Name, want+"-") {
				candidates = append(candidates, pi)
				continue
			}
			// 3) prefix by want.release (“acl-2.3.1-2.”)
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
		// Otherwise pick the “highest” by lex sort
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Name > candidates[j].Name
		})
		out = append(out, candidates[0])
	}
	return out, nil
}
func (p *AzureLinux3) Validate(destDir string) error {
	logger := utils.Logger()

	// read the GPG key from the repo config
	resp, err := http.Get(p.repoCfg.GPGKey)
	if err != nil {
		return fmt.Errorf("fetch GPG key %s: %w", p.repoCfg.GPGKey, err)
	}
	defer resp.Body.Close()

	keyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read GPG key body: %w", err)
	}
	logger.Infof("fetched GPG key (%d)", len(keyBytes))
	logger.Debugf("GPG key: %s\n", keyBytes)

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
		logger.Warn("no RPMs found to verify")
		return nil
	}

	start := time.Now()
	results := rpmutils.VerifyAll(rpmPaths, tmp.Name(), 4)
	logger.Infof("RPM verification took %s", time.Since(start))

	// Check results
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("RPM %s failed verification: %v", r.Path, r.Error)
		}
	}
	logger.Info("all RPMs verified successfully")

	return nil
}

func (p *AzureLinux3) Resolve(req []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	logger := utils.Logger()

	logger.Infof("resolving dependencies for %d RPMs", len(req))

	// Resolve all the required dependencies for the initial seed of RPMs
	needed, err := rpmutils.ResolvePackageInfos(req, all)
	if err != nil {
		logger.Errorf("resolving dependencies failed: %v", err)
		return nil, err
	}
	logger.Infof("need a total of %d RPMs (including dependencies)", len(needed))

	for _, pkg := range needed {
		logger.Debugf("-> %s", pkg.Name)
	}

	return needed, nil
}

// loadRepoConfig parses the repo configuration data
func loadRepoConfig(r io.Reader) (repoConfig, error) {
	s := bufio.NewScanner(r)
	var rc repoConfig
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
