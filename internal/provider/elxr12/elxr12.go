package elxr12

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/debutils"
	"github.com/open-edge-platform/image-composer/internal/provider"
	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// DEB: https://deb.debian.org/debian/dists/bookworm/main/binary-amd64/Packages.gz
// DEB Download Path: https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
// eLxr: https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz
// eLxr Donwload Path: https://mirror.elxr.dev/elxr/pool/main/p/python3-defaults/2to3_3.11.2-1_all.deb
const (
	baseURL    = "https://mirror.elxr.dev/elxr/dists/aria/main/"
	configName = "Packages.gz"
)

// repoConfig hold repo related info
type repoConfig struct {
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

// eLxr12 implements provider.Provider
type eLxr12 struct {
	repoURL     string
	repoCfg     repoConfig
	pkgChecksum []pkgChecksum //this is not using for debian
	gzHref      string
	spec        *config.BuildSpec
}

func init() {
	provider.Register(&eLxr12{})
}

// Name returns the unique name of the provider
func (p *eLxr12) Name() string { return "eLxr12" }

// Init will initialize the provider, fetching repo configuration
func (p *eLxr12) Init(spec *config.BuildSpec) error {

	logger := utils.Logger()

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
	p.gzHref = cfg.PkgList
	p.spec = spec

	logger.Infof("initialized eLxr provider repo section=%s", cfg.Section)
	logger.Infof("name=%s", cfg.Name)
	logger.Infof("package list url=%s", cfg.PkgList)
	logger.Infof("package download url=%s", cfg.PkgPrefix)
	return nil

}

// Packages returns the list of packages
func (p *eLxr12) Packages() ([]provider.PackageInfo, error) {

	logger := utils.Logger()
	logger.Infof("fetching packages from %s", p.repoCfg.PkgList)

	packages, err := debutils.ParsePrimary(p.repoCfg.PkgPrefix, p.gzHref, p.repoCfg.ReleaseFile, p.repoCfg.ReleaseSign, p.repoCfg.PbGPGKey, p.repoCfg.BuildPath)
	if err != nil {
		logger.Errorf("parsing %s failed: %v", p.gzHref, err)
	}

	logger.Infof("found %d packages in eLxr repo", len(packages))
	return packages, nil
}

// Validate verifies the downloaded files
func (p *eLxr12) Validate(destDir string) error {
	logger := utils.Logger()

	// get all DEBs in the destDir
	debPattern := filepath.Join(destDir, "*.deb")
	debPaths, err := filepath.Glob(debPattern)
	if err != nil {
		return fmt.Errorf("glob %q: %w", debPattern, err)
	}
	if len(debPaths) == 0 {
		logger.Warn("no DEBs found to verify")
		return nil
	}

	// Create a simple dictionary (map) to store all records from p.pkgChecksum
	checksumMap := make(map[string]string)
	for _, pc := range p.pkgChecksum {
		checksumMap[pc.Name] = pc.Checksum
	}

	start := time.Now()
	results := debutils.VerifyDEBs(debPaths, checksumMap, 4)
	logger.Infof("Debian verification took %s", time.Since(start))

	// Check results
	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("deb %s failed verification: %v", r.Path, r.Error)
		}
	}
	logger.Info("all DEBs verified successfully")

	return nil
}

// Resolve resolves dependencies
func (p *eLxr12) Resolve(req []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {
	logger := utils.Logger()

	logger.Infof("resolving dependencies for %d DEBIANs", len(req))
	// Resolve all the required dependencies for the initial seed of Debian packages
	needed, err := debutils.ResolvePackageInfos(req, all)
	if err != nil {
		logger.Errorf("resolving dependencies failed: %v", err)
		return nil, err
	}

	logger.Infof("requested %d packages, resolved to %d packages", len(req), len(needed))
	logger.Infof("need a total of %d RPMs (including dependencies)", len(needed))

	for _, pkg := range needed {
		logger.Debugf("-> %s", pkg.Name)
	}

	// Adding needed packages to the pkgChecksum list
	for _, pkg := range needed {
		p.pkgChecksum = append(p.pkgChecksum, pkgChecksum{
			Name:     pkg.Name,
			Checksum: pkg.Checksum,
		})
	}

	return needed, nil
}

// MatchRequested matches requested packages
func (p *eLxr12) MatchRequested(requests []string, all []provider.PackageInfo) ([]provider.PackageInfo, error) {

	logger := utils.Logger()

	var out []provider.PackageInfo

	for _, want := range requests {
		var candidates []provider.PackageInfo
		for _, pi := range all {

			// 1) exact name match
			if pi.Name == want || pi.Name == want+".deb" {
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
			logger.Infof("requested package %q not found in repo", want)
			continue
		}
		// If we got an exact match in step (1), it's the only candidate
		if len(candidates) == 1 && (candidates[0].Name == want || candidates[0].Name == want+".deb") {
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
	logger := utils.Logger()

	var rc repoConfig

	rc.PkgList = repoUrl
	// rc.ReleaseFile = "https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Release" //negative test
	rc.ReleaseFile = "https://mirror.elxr.dev/elxr/dists/aria/Release"
	rc.PkgPrefix = "https://mirror.elxr.dev/elxr/"
	rc.Name = "Wind River eLxr 12"
	rc.GPGCheck = true
	rc.RepoGPGCheck = true
	rc.Enabled = true
	rc.PbGPGKey = "https://mirror.elxr.dev/elxr/public.gpg"
	rc.ReleaseSign = "https://mirror.elxr.dev/elxr/dists/aria/Release.gpg"
	rc.Section = "main"
	rc.BuildPath = "./builds/elxr12"

	logger.Infof("repo config: %+v", rc)

	return rc, nil
}
