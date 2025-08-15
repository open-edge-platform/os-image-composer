package emt

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/chroot"
	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/image/isomaker"
	"github.com/open-edge-platform/image-composer/internal/image/rawmaker"
	"github.com/open-edge-platform/image-composer/internal/ospackage/rpmutils"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/utils/shell"
)

const (
	configURL = "https://raw.githubusercontent.com/open-edge-platform/edge-microvisor-toolkit/refs/heads/3.0/SPECS/edge-repos/edge-base.repo"
	gpgkeyURL = "https://raw.githubusercontent.com/open-edge-platform/edge-microvisor-toolkit/refs/heads/3.0/SPECS/edge-repos/INTEL-RPM-GPG-KEY"
	repomdURL = "https://files-rs.edgeorchestration.intel.com/files-edge-orch/microvisor/rpm/3.0/repodata/repomd.xml"
)

// Emt implements provider.Provider
type Emt struct {
	repoURL string
	repoCfg rpmutils.RepoConfig
	zstHref string
}

func Register(dist, arch string) {
	provider.Register(&Emt{}, dist, arch)
}

// Name returns the unique name of the provider
func (p *Emt) Name(dist, arch string) string {
	return GetProviderId(dist, arch)
}

// Init will initialize the provider, fetching repo configuration
func (p *Emt) Init(dist, arch string) error {
	log := logger.Logger()

	resp, err := http.Get(configURL)
	if err != nil {
		log.Errorf("downloading repo config %s failed: %v", configURL, err)
		return err
	}
	defer resp.Body.Close()

	cfg, err := loadRepoConfig(resp.Body)
	if err != nil {
		log.Errorf("parsing repo config failed: %v", err)
		return err
	}

	href, err := fetchPrimaryURL(repomdURL)
	if err != nil {
		log.Errorf("fetch primary.xml.zst failed: %v", err)
		return err
	}

	p.repoURL = configURL
	p.repoCfg = cfg
	p.zstHref = href

	log.Infof("initialized EMT3.0 provider repo section=%s", cfg.Section)
	log.Infof("name=%s", cfg.Name)
	log.Infof("url=%s", cfg.URL)
	log.Infof("primary.xml.zst=%s", p.zstHref)
	return nil
}

func (p *Emt) PreProcess(template *config.ImageTemplate) error {
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

func (p *Emt) BuildImage(template *config.ImageTemplate) error {
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

func (p *Emt) PostProcess(template *config.ImageTemplate, err error) error {
	log := logger.Logger()
	if err != nil {
		log.Errorf("post-process error: %v", err)
	}

	if err := chroot.CleanupChrootEnv(config.TargetOs, config.TargetDist, config.TargetArch); err != nil {
		return fmt.Errorf("failed to cleanup chroot environment: %v", err)
	}
	return err
}

func (p *Emt) installHostDependency() error {
	log := logger.Logger()
	var depedencyInfo = map[string]string{
		"rpm":      "rpm",        // For the chroot env build RPM pkg installation
		"mkfs.fat": "dosfstools", // For the FAT32 boot partition creation
		"xorriso":  "xorriso",    // For ISO image creation
		"sbsign":   "sbsigntool", // For the UKI image creation
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

func (p *Emt) downloadImagePkgs(template *config.ImageTemplate) error {
	pkgList := template.GetPackages()
	providerId := p.Name(config.TargetDist, config.TargetArch)
	globalCache, err := config.CacheDir()
	if err != nil {
		return fmt.Errorf("failed to get global cache dir: %w", err)
	}
	pkgCacheDir := filepath.Join(globalCache, "pkgCache", providerId)
	rpmutils.RepoCfg = p.repoCfg
	rpmutils.GzHref = p.zstHref
	config.FullPkgList, err = rpmutils.DownloadPackages(pkgList, pkgCacheDir, "")
	return err
}

func GetProviderId(dist, arch string) string {
	return "edge-microvisor-toolkit" + "-" + dist + "-" + arch
}

// loadRepoConfig parses the repo configuration data
func loadRepoConfig(r io.Reader) (rpmutils.RepoConfig, error) {
	s := bufio.NewScanner(r)
	var rc rpmutils.RepoConfig
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
			rc.GPGKey = gpgkeyURL
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
