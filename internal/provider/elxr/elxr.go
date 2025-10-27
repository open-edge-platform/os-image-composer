package elxr

import (
	"fmt"
	"path/filepath"

	"github.com/open-edge-platform/os-image-composer/internal/chroot"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/image/initrdmaker"
	"github.com/open-edge-platform/os-image-composer/internal/image/isomaker"
	"github.com/open-edge-platform/os-image-composer/internal/image/rawmaker"
	"github.com/open-edge-platform/os-image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/os-image-composer/internal/provider"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
	"github.com/open-edge-platform/os-image-composer/internal/utils/system"
)

// DEB: https://deb.debian.org/debian/dists/bookworm/main/binary-amd64/Packages.gz
// DEB Download Path: https://deb.debian.org/debian/pool/main/0/0ad/0ad_0.0.26-3_amd64.deb
// eLxr: https://mirror.elxr.dev/elxr/dists/aria/main/binary-amd64/Packages.gz
// eLxr Download Path: https://mirror.elxr.dev/elxr/pool/main/p/python3-defaults/2to3_3.11.2-1_all.deb
const (
	OsName = "wind-river-elxr"
)

var log = logger.Logger()

// eLxr implements provider.Provider
type eLxr struct {
	repoCfg   debutils.RepoConfig
	gzHref    string
	chrootEnv chroot.ChrootEnvInterface
}

func Register(targetOs, targetDist, targetArch string) error {
	chrootEnv, err := chroot.NewChrootEnv(targetOs, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to inject chroot dependency: %w", err)
	}
	provider.Register(&eLxr{
		chrootEnv: chrootEnv,
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

	cfg, err := loadRepoConfig("", arch) // repoURL no longer needed
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
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	log.Infof("Building image: %s", template.GetImageName())

	// Create makers with template when needed
	switch template.Target.ImageType {
	case "raw":
		return p.buildRawImage(template)
	case "img":
		return p.buildInitrdImage(template)
	case "iso":
		return p.buildIsoImage(template)
	default:
		return fmt.Errorf("unsupported image type: %s", template.Target.ImageType)
	}
}

func (p *eLxr) buildRawImage(template *config.ImageTemplate) error {
	// Create RawMaker with template (dependency injection)
	rawMaker, err := rawmaker.NewRawMaker(p.chrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create raw maker: %w", err)
	}

	// Use the maker
	if err := rawMaker.Init(); err != nil {
		return fmt.Errorf("failed to initialize raw maker: %w", err)
	}

	return rawMaker.BuildRawImage()
}

func (p *eLxr) buildInitrdImage(template *config.ImageTemplate) error {
	// Create InitrdMaker with template (dependency injection)
	initrdMaker, err := initrdmaker.NewInitrdMaker(p.chrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create initrd maker: %w", err)
	}

	// Use the maker
	if err := initrdMaker.Init(); err != nil {
		return fmt.Errorf("failed to initialize initrd image maker: %w", err)
	}
	if err := initrdMaker.BuildInitrdImage(); err != nil {
		return fmt.Errorf("failed to build initrd image: %w", err)
	}
	if err := initrdMaker.CleanInitrdRootfs(); err != nil {
		return fmt.Errorf("failed to clean initrd rootfs: %w", err)
	}

	return nil
}

func (p *eLxr) buildIsoImage(template *config.ImageTemplate) error {
	// Create IsoMaker with template (dependency injection)
	isoMaker, err := isomaker.NewIsoMaker(p.chrootEnv, template)
	if err != nil {
		return fmt.Errorf("failed to create iso maker: %w", err)
	}

	// Use the maker
	if err := isoMaker.Init(); err != nil {
		return fmt.Errorf("failed to initialize iso maker: %w", err)
	}

	return isoMaker.BuildIsoImage()
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
		"mmdebstrap":   "mmdebstrap",    // For the chroot env build
		"mkfs.fat":     "dosfstools",    // For the FAT32 boot partition creation
		"mformat":      "mtools",        // For writing files to FAT32 partition
		"xorriso":      "xorriso",       // For ISO image creation
		"qemu-img":     "qemu-utils",    // For image file format conversion
		"ukify":        "systemd-ukify", // For the UKI image creation
		"grub-mkimage": "grub-common",   // For ISO image UEFI Grub binary creation
		"veritysetup":  "cryptsetup",    // For the veritysetup command
		"sbsign":       "sbsigntool",    // For the UKI image creation
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
	if err := p.chrootEnv.UpdateSystemPkgs(template); err != nil {
		return fmt.Errorf("failed to update system packages: %w", err)
	}
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

	// Load provider repo config using the centralized config function
	providerConfig, err := config.LoadProviderRepoConfig(OsName, "elxr12")
	if err != nil {
		return rc, fmt.Errorf("failed to load provider repo config: %w", err)
	}

	// Convert ProviderRepoConfig to debutils.RepoConfig using the unified conversion method
	repoType, name, url, gpgKey, component, buildPath, pkgPrefix, releaseFile, releaseSign, gpgCheck, repoGPGCheck, enabled := providerConfig.ToRepoConfigData(arch)

	// Verify this is a DEB repository
	if repoType != "deb" {
		return rc, fmt.Errorf("expected DEB repository type, got: %s", repoType)
	}

	rc = debutils.RepoConfig{
		Section:      component, // Map component to Section for DEB utils
		Name:         name,
		PkgList:      url, // For DEB repos, this is the Packages.gz URL
		PkgPrefix:    pkgPrefix,
		GPGCheck:     gpgCheck,
		RepoGPGCheck: repoGPGCheck,
		Enabled:      enabled,
		PbGPGKey:     gpgKey, // For DEB repos, gpgKey contains the pbGPGKey value
		ReleaseFile:  releaseFile,
		ReleaseSign:  releaseSign,
		BuildPath:    buildPath,
		Arch:         arch,
	}

	log.Infof("Loaded repo config for %s: %+v", OsName, rc)

	return rc, nil
}
