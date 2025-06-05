package provider

import (
	"github.com/open-edge-platform/image-composer/internal/utils/config"
	"github.com/open-edge-platform/image-composer/internal/utils/pkg"
)

// Provider is the interface every OSV plugin must implement.
type Provider interface {
	// Name is a unique ID, e.g. "AzureLinux3" or "EMT3.0".
	Name() string

	// Init does any one-time setup: import GPG keys, register repos, etc.
	Init(template *config.ImageTemplate) error

	// Packages returns the list of PackageInfo for this image build.
	Packages() ([]pkg.PackageInfo, error)

	// Validate walks the destDir and verifies each downloaded file.
	Validate(destDir string) error

	// MatchRequested takes the list of requested packages and returns
	// the list of PackageInfo that match.
	MatchRequested(requested []string, all []pkg.PackageInfo) ([]pkg.PackageInfo, error)

	// Resolve walks dependencies and returns the full list of packages needed.
	Resolve(req []pkg.PackageInfo, all []pkg.PackageInfo) ([]pkg.PackageInfo, error)
}

var (
	providers = make(map[string]Provider)
)

// Register makes a Provider available under its Name().
func Register(p Provider) {
	providers[p.Name()] = p
}

// Get returns the Provider by name.
func Get(name string) (Provider, bool) {
	p, ok := providers[name]
	return p, ok
}
