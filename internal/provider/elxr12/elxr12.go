package elxr12

import (
	"sort"
	"strings"

	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/config"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/debutils"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
	"go.uber.org/zap"
)

// ref: https://packages.microsoft.com/azurelinux/3.0/prod/base/
// example: https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
const (
	baseURL    = "https://deb.debian.org/debian/dists/bookworm/main/"
	configName = "Packages.gz"
	repodata   = ""
)

// repoConfig holds .repo file values
type repoConfig struct {
	Section      string // raw section header
	Name         string // human-readable name from name=
	CfgURL       string
	PkgPrefixUrl string
	GPGCheck     bool
	RepoGPGCheck bool
	Enabled      bool
	GPGKey       string
}

// eLxr12 implements provider.Provider
type eLxr12 struct {
	repoURL string
	repoCfg repoConfig
	gzHref  string
	spec    *config.BuildSpec
}

func init() {
	provider.Register(&eLxr12{})
}

// Name returns the unique name of the provider
func (p *eLxr12) Name() string {
	logger := zap.L().Sugar()
	logger.Infof("Name() called - Placeholder: This function will return the provider's unique name.")
	return "eLxr12"
}

// Init will initialize the provider, fetching repo configuration
func (p *eLxr12) Init(spec *config.BuildSpec) error {

	logger := zap.L().Sugar()

	//todo: need to correct of how to get the arch once finalized
	if spec.Arch == "x86_64" {
		spec.Arch = "binary-amd64"
	}
	p.repoURL = baseURL + spec.Arch + "/" + configName

	cfg, err := loadRepoConfig(p.repoURL)
	if err != nil {
		logger.Errorf("parsing repo config failed: %v", err)
		return err
	}
	p.repoCfg = cfg
	p.gzHref = cfg.CfgURL

	logger.Infof("initialized eLxr provider repo section=%s", cfg.Section)
	logger.Infof("name=%s", cfg.Name)
	logger.Infof("config url=%s", cfg.CfgURL)
	logger.Infof("package download url=%s", cfg.PkgPrefixUrl)
	logger.Infof("primary.xml.gz=%s", p.gzHref)
	return nil

}

// Packages returns the list of packages
func (p *eLxr12) Packages() ([]provider.PackageInfo, error) {

	logger := zap.L().Sugar()
	logger.Infof("fetching packages from %s", p.repoCfg.CfgURL)

	packages, err := debutils.ParsePrimary(p.repoCfg.PkgPrefixUrl, p.gzHref)
	if err != nil {
		logger.Errorf("parsing %s failed: %v", p.gzHref, err)
	}

	logger.Infof("found %d packages in eLxr repo", len(packages))
	return packages, nil
}

// Validate verifies the downloaded files
func (p *eLxr12) Validate(destDir string) error {
	logger := zap.L().Sugar()
	logger.Infof("Validate() called with destDir=%s - Placeholder: This function will be implemented by the respective owner.", destDir)
	return nil
}

// Resolve resolves dependencies
func (p *eLxr12) Resolve(req []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {
	// get sugar logger from zap
	logger := zap.L().Sugar()

	logger.Infof("resolving dependencies for %d RPMs", len(req))
	// Resolve all the required dependencies for the initial seed of RPMs
	needed, err := debutils.ResolvePackageInfos(req, all)
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

// MatchRequested matches requested packages
func (p *eLxr12) MatchRequested(requests []string, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	logger := zap.L().Sugar()

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
			// return nil, fmt.Errorf("requested package %q not found in repo", want)
			logger.Infof("requested package %q not found in repo", want)
			continue
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

	logger.Infof("found %d packages in request of %d", len(out), len(requests))
	return out, nil

}

func loadRepoConfig(repoUrl string) (repoConfig, error) {
	logger := zap.L().Sugar()

	var rc repoConfig

	//example direct download: wget https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
	rc.CfgURL = repoUrl
	rc.PkgPrefixUrl = "https://deb.debian.org/debian/"
	rc.Name = "Debian Bookworm Main"
	rc.GPGCheck = true
	rc.RepoGPGCheck = true
	rc.Enabled = true
	rc.GPGKey = "https://ftp-master.debian.org/keys/release-12.asc"
	rc.Section = "main"

	logger.Infof("repo config: %+v", rc)

	return rc, nil
}
