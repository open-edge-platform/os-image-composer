package emt30

import (
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/config"
	"github.com/intel-innersource/os.linux.tiberos.os-curation-tool/internal/provider"
	"go.uber.org/zap"
)

// repoConfig holds .repo file values
type repoConfig struct {
	Section      string // raw section header
	Name         string // human-readable name from name=
	BaseURL      string
	GPGCheck     bool
	RepoGPGCheck bool
	Enabled      bool
	GPGKey       string
}

// emt30 implements provider.Provider
type emt30 struct {
	repo repoConfig
	spec *config.BuildSpec
}

func init() {
	provider.Register(&emt30{})
}

// Name returns the unique name of the provider
func (p *emt30) Name() string {
	logger := zap.L().Sugar()
	logger.Infof("Name() called - Placeholder: This function will return the provider's unique name.")
	return "EMT3.0"
}

// Init will initialize the provider, fetching repo configuration
func (p *emt30) Init(spec *config.BuildSpec) error {
	logger := zap.L().Sugar()
	logger.Infof("Init() called - Placeholder: This function will be implemented by the respective owner.")
	p.repo = repoConfig{
		Section: "dummy-section",
		Name:    "Dummy Repo",
		BaseURL: "http://dummy-url/",
	}
	p.spec = spec
	return nil
}

// Packages returns the list of packages
func (p *emt30) Packages() ([]provider.PackageInfo, error) {
	logger := zap.L().Sugar()
	logger.Infof("Packages() called - Placeholder: This function will be implemented by the respective owner.")
	return nil, nil
}

// Validate verifies the downloaded files
func (p *emt30) Validate(destDir string) error {
	logger := zap.L().Sugar()
	logger.Infof("Validate() called with destDir=%s - Placeholder: This function will be implemented by the respective owner.", destDir)
	return nil
}

// Resolve resolves dependencies
func (p *emt30) Resolve(req []provider.PackageInfo, all []provider.PackageInfo) ([]provider.PackageInfo, error) {
	logger := zap.L().Sugar()
	logger.Infof("Resolve() called with destDir=%s - Placeholder: This function will be implemented by the respective owner.")
	return nil, nil
}

// MatchRequested takes the list of requested packages and returns
func (p *emt30) MatchRequested(requested []string, all []provider.PackageInfo) ([]provider.PackageInfo, error) {
	logger := zap.L().Sugar()
	logger.Infof("MatchRequested() called - Placeholder: This function will be implemented by the respective owner.")
	return nil, nil
}
