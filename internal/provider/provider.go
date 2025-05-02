package provider

// PackageInfo holds everything you need to fetch + verify one artifact.
type PackageInfo struct {
    Name     string // e.g. "abseil-cpp"
    URL      string // download URL
    Checksum string // optional pre-known digest
}

// Provider is the interface every OSV plugin must implement.
type Provider interface {
    // Name is a unique ID, e.g. "azurelinux3" or "debian12".
    Name() string

    // Init does any one-time setup: import GPG keys, register repos, etc.
    Init() error

    // Packages returns the list of PackageInfo for this image build.
    Packages() ([]PackageInfo, error)

    // Validate walks the destDir and verifies each downloaded file.
    // You can shell out to `rpm -Kv`, `dpkg-sig`, or use Go APIs here.
    Validate(destDir string) error
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