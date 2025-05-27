package provider

import (
	"github.com/open-edge-platform/image-composer/internal/config"
)

// PackageInfo holds everything you need to fetch + verify one artifact.
type PackageInfo struct {
	Name     string   // e.g. "abseil-cpp"
	URL      string   // download URL
	Checksum string   // optional pre-known digest
	Provides []string // capabilities this package provides (rpm:entry names)
	Requires []string // capabilities this package requires
}

// Provider is the interface every OSV plugin must implement.
type Provider interface {
	// Name is a unique ID, e.g. "azurelinux3" or "emt3".
	Name() string

	// Init does any one-time setup: import GPG keys, register repos, etc.
	Init(spec *config.BuildSpec) error

	// Packages returns the list of PackageInfo for this image build.
	Packages() ([]PackageInfo, error)

	// Validate walks the destDir and verifies each downloaded file.
	Validate(destDir string) error

	// MatchRequested takes the list of requested packages and returns
	// the list of PackageInfo that match.
	MatchRequested(requested []string, all []PackageInfo) ([]PackageInfo, error)

	// Resolve walks your local cache in destDir and returns the full
	Resolve(req []PackageInfo, all []PackageInfo) ([]PackageInfo, error)
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
