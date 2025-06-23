package elxr

import (
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

// DEB: https://deb.debian.org/debian/dists/bookworm/main/binary-amd64/Packages.gz
// DEB Download Path: https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
// eLxr: https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz
// eLxr Download Path: https://mirror.elxr.dev/elxr/pool/main/p/python3-defaults/2to3_3.11.2-1_all.deb
const (
	baseURL    = "https://mirror.elxr.dev/elxr/dists/aria/main/"
	configName = "Packages.gz"
)

// eLxr implements provider.Provider
type eLxr struct {
	repoURL string
	repoCfg debutils.RepoConfig
	gzHref  string
}

func Register(dist, arch string) {
	provider.Register(&eLxr{}, dist, arch)
}

// Name returns the unique name of the provider
func (p *eLxr) Name(dist, arch string) string {
	return GetProviderId(dist, arch)
}

// Init will initialize the provider, fetching repo configuration
func (p *eLxr) Init(dist, arch string) error {
	log := logger.Logger()

	//todo: need to correct of how to get the arch once finalized
	if arch == "x86_64" {
		arch = "binary-amd64"
	}
	p.repoURL = baseURL + arch + "/" + configName

	cfg, err := loadRepoConfig(p.repoURL)
	if err != nil {
		log.Errorf("parsing repo config failed: %v", err)
		return err
	}
	p.repoCfg = cfg
	p.gzHref = cfg.PkgList

	log.Infof("initialized eLxr provider repo section=%s", cfg.Section)
	log.Infof("name=%s", cfg.Name)
	log.Infof("package list url=%s", cfg.PkgList)
	log.Infof("package download url=%s", cfg.PkgPrefix)
	return nil
}

func (p *eLxr) PreProcess(template *config.ImageTemplate) error {
	err := p.downloadImagePkgs(template)
	if err != nil {
		return fmt.Errorf("failed to download image packages: %v", err)
	}
	return nil
}

func (p *eLxr) BuildImage(template *config.ImageTemplate) error {
	return nil
}

func (p *eLxr) PostProcess(template *config.ImageTemplate, err error) error {
	return nil
}

func (p *eLxr) downloadImagePkgs(template *config.ImageTemplate) error {
	pkgList := template.GetPackages()
	providerId := p.Name(config.TargetDist, config.TargetArch)
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	pkgCacheDir := filepath.Join(globalCache, "pkgCache", providerId)
	debutils.RepoCfg = p.repoCfg
	debutils.GzHref = p.gzHref
	_, err = debutils.DownloadPackages(pkgList, pkgCacheDir, "")
	return err
}

func GetProviderId(dist, arch string) string {
	return "wind-river-elxr" + "-" + dist + "-" + arch
}

func loadRepoConfig(repoUrl string) (debutils.RepoConfig, error) {
	log := logger.Logger()

	var rc debutils.RepoConfig

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

	log.Infof("repo config: %+v", rc)

	return rc, nil
}
