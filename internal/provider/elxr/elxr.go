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
	"github.com/open-edge-platform/image-composer/internal/utils/system"
)

// DEB: https://deb.debian.org/debian/dists/bookworm/main/binary-amd64/Packages.gz
// DEB Download Path: https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
// eLxr: https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz
// eLxr Download Path: https://mirror.elxr.dev/elxr/pool/main/p/python3-defaults/2to3_3.11.2-1_all.deb
const (
	OsName     = "wind-river-elxr"
	baseURL    = "https://mirror.elxr.dev/elxr/dists/aria/main/"
	configName = "Packages.gz"
)

var log = logger.Logger()

// eLxr implements provider.Provider
type eLxr struct {
	repoURL   string
	repoCfg   debutils.RepoConfig
	gzHref    string
	chrootEnv chroot.ChrootEnvInterface
	rawMaker  rawmaker.RawMakerInterface
	isoMaker  isomaker.IsoMakerInterface
}

func Register(targetOs, targetDist, targetArch string) error {
	chrootEnv, err := chroot.NewChrootEnv(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to inject chroot dependency: %w", err)
	}
	rawMaker, err := rawmaker.NewRawMaker(chrootEnv)
	if err != nil {
		return fmt.Errorf("failed to inject raw image maker dependency: %w", err)
	}
	isoMaker, err := isomaker.NewIsoMaker(chrootEnv)
	if err != nil {
		return fmt.Errorf("failed to inject ISO image maker dependency: %w", err)
	}
	provider.Register(&eLxr{
		chrootEnv: chrootEnv,
		rawMaker:  rawMaker,
		isoMaker:  isoMaker,
	}, targetDist, targetArch)

	return nil
}

// Name returns the unique name of the provider
func (p *eLxr) Name(dist, arch string) string {
	return system.GetProviderId(OsName, dist, arch)
}

// Init will initialize the provider, fetching repo configuration
func (p *eLxr) Init(dist, arch string) error {

	//todo: need to correct of how to get the arch once finalized
	if arch == "x86_64" {
		arch = "amd64"
	}
	p.repoURL = baseURL + "binary-" + arch + "/" + configName

	cfg, err := loadRepoConfig(p.repoURL, arch)
	if err != nil {
		log.Errorf("Parsing repo config failed: %v", err)
		return err
	}
	p.repoCfg = cfg
	p.gzHref = cfg.PkgList

	log.Infof("Initialized eLxr provider repo section=%s", cfg.Section)
	log.Infof("name=%s", cfg.Name)
	log.Infof("package list url=%s", cfg.PkgList)
	log.Infof("package download url=%s", cfg.PkgPrefix)
	return nil
}

func (p *eLxr) PreProcess(template *config.ImageTemplate) error {
	if err := p.installHostDependency(); err != nil {
		return fmt.Errorf("failed to install host dependencies: %w", err)
	}

	if err := p.downloadImagePkgs(template); err != nil {
		return fmt.Errorf("failed to download image packages: %w", err)
	}

	if err := p.chrootEnv.InitChrootEnv(template.Target.OS,
		template.Target.Dist, template.Target.Arch); err != nil {
		return fmt.Errorf("failed to initialize chroot environment: %w", err)
	}
	return nil
}

func (p *eLxr) BuildImage(template *config.ImageTemplate) error {
	if template.Target.ImageType == "iso" {
		if err := p.isoMaker.Init(template); err != nil {
			return fmt.Errorf("failed to initialize ISO image maker: %w", err)
		}
		if err := p.isoMaker.BuildIsoImage(template); err != nil {
			return fmt.Errorf("failed to build ISO image: %w", err)
		}
	} else {
		if err := p.rawMaker.Init(template); err != nil {
			return fmt.Errorf("failed to initialize raw image maker: %w", err)
		}
		if err := p.rawMaker.BuildRawImage(template); err != nil {
			return fmt.Errorf("failed to build raw image: %w", err)
		}
	}
	return nil
}

func (p *eLxr) PostProcess(template *config.ImageTemplate, err error) error {
	if err := p.chrootEnv.CleanupChrootEnv(template.Target.OS,
		template.Target.Dist, template.Target.Arch); err != nil {
		return fmt.Errorf("failed to cleanup chroot environment: %w", err)
	}
	return nil
}

func (p *eLxr) installHostDependency() error {
	var dependencyInfo = map[string]string{
		"mmdebstrap":        "mmdebstrap",    // For the chroot env build
		"mkfs.fat":          "dosfstools",    // For the FAT32 boot partition creation
		"xorriso":           "xorriso",       // For ISO image creation
		"qemu-img":          "qemu-utils",    // For image file format conversion
		"ukify":             "systemd-ukify", // For the UKI image creation
		"grub-mkstandalone": "grub-common",   // For ISO image UEFI Grub binary creation
		"veritysetup":       "cryptsetup",    // For the veritysetup command
		"sbsign":            "sbsigntool",    // For the UKI image creation
	}
	hostPkgManager, err := system.GetHostOsPkgManager()
	if err != nil {
		return fmt.Errorf("failed to get host package manager: %w", err)
	}

	for cmd, pkg := range dependencyInfo {
		cmdExist, err := shell.IsCommandExist(cmd, "")
		if err != nil {
			return fmt.Errorf("failed to check command %s existence: %w", cmd, err)
		}
		if !cmdExist {
			cmdStr := fmt.Sprintf("%s install -y %s", hostPkgManager, pkg)
			if _, err := shell.ExecCmdWithStream(cmdStr, true, "", nil); err != nil {
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
	providerId := p.Name(template.Target.Dist, template.Target.Arch)
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	pkgCacheDir := filepath.Join(globalCache, "pkgCache", providerId)
	debutils.RepoCfg = p.repoCfg
	debutils.GzHref = p.gzHref
	debutils.Architecture = p.repoCfg.Arch
	debutils.UserRepo = template.GetPackageRepositories()
	template.FullPkgList, err = debutils.DownloadPackages(pkgList, pkgCacheDir, "")
	return err
}

func loadRepoConfig(repoUrl string, arch string) (debutils.RepoConfig, error) {

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
	rc.Arch = arch

	log.Infof("Repo config: %+v", rc)

	return rc, nil
}
