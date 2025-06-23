package emt

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config"
	"github.com/open-edge-platform/image-composer/internal/provider"
	"github.com/open-edge-platform/image-composer/internal/utils/logger"
)

const (
	configURL = "https://raw.githubusercontent.com/open-edge-platform/edge-microvisor-toolkit/refs/heads/3.0/SPECS/edge-repos/edge-base.repo"
	repomdURL = "https://files-rs.edgeorchestration.intel.com/files-edge-orch/microvisor/rpm/3.0/repodata/repomd.xml"
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

// Emt implements provider.Provider
type Emt struct {
	repoURL string
	repoCfg repoConfig
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
	return nil
}

func (p *Emt) BuildImage(template *config.ImageTemplate) error {
	return nil
}

func (p *Emt) PostProcess(template *config.ImageTemplate, err error) error {
	return nil
}

func GetProviderId(dist, arch string) string {
	return "edge-microvisor-toolkit" + "-" + dist + "-" + arch
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
