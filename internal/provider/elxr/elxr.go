package elxr

import (
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/isomaker"
	"github.com/open-edge-platform/image-composer/internal/image/rawmaker"
	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
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
	if err := p.installHostDependency(); err != nil {
		return fmt.Errorf("failed to install host dependencies: %v", err)
	}

	if err := p.downloadImagePkgs(template); err != nil {
		return fmt.Errorf("failed to download image packages: %v", err)
	}

	if err := chroot.InitChrootEnv(config.TargetOs, config.TargetDist, config.TargetArch); err != nil {
		return fmt.Errorf("failed to initialize chroot environment: %v", err)
	}
	return nil
}

func (p *eLxr) BuildImage(template *config.ImageTemplate) error {
	if config.TargetImageType == "iso" {
		err := isomaker.BuildISOImage(template)
		if err != nil {
			return fmt.Errorf("failed to build ISO image: %v", err)
		}
	} else {
		err := rawmaker.BuildRawImage(template)
		if err != nil {
			return fmt.Errorf("failed to build raw image: %v", err)
		}
	}
	return nil
}

func (p *eLxr) PostProcess(template *config.ImageTemplate, err error) error {
	log := logger.Logger()
	if err != nil {
		log.Errorf("post-process error: %v", err)
	}

	if err := chroot.CleanupChrootEnv(config.TargetOs, config.TargetDist, config.TargetArch); err != nil {
		return fmt.Errorf("failed to cleanup chroot environment: %v", err)
	}
	return nil
}

func (p *eLxr) installHostDependency() error {
	log := logger.Logger()
	var depedencyInfo = map[string]string{
		"mmdebstrap":        "mmdebstrap",    // For the chroot env build
		"mkfs.fat":          "dosfstools",    // For the FAT32 boot partition creation
		"xorriso":           "xorriso",       // For ISO image creation
		"ukify":             "systemd-ukify", // For the UKI image creation
		"grub-mkstandalone": "grub-common",   // For ISO image UEFI Grub binary creation
		"veritysetup":       "cryptsetup",    // For the veritysetup command
		"sbsign":            "sbsigntool",    // For the UKI image creation
	}
	hostPkgManager, err := chroot.GetHostOsPkgManager()
	if err != nil {
		return fmt.Errorf("failed to get host package manager: %w", err)
	}

	for cmd, pkg := range depedencyInfo {
		cmdExist, err := shell.IsCommandExist(cmd, "")
		if err != nil {
			return fmt.Errorf("failed to check command %s existence: %w", cmd, err)
		}
		if !cmdExist {
			cmdStr := fmt.Sprintf("%s install -y %s", hostPkgManager, pkg)
			_, err := shell.ExecCmdWithStream(cmdStr, true, "", nil)
			if err != nil {
				return fmt.Errorf("failed to install host dependency %s: %w", pkg, err)
			}
			log.Debugf("Installed host dependency: %s", pkg)
		} else {
			log.Debugf("Host dependency %s is already installed", pkg)
		}
	}
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
	config.FullPkgList, err = debutils.DownloadPackages(pkgList, pkgCacheDir, "")
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
