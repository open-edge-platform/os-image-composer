package pkg

// PackageInfo holds everything you need to fetch + verify one artifact.
type PackageInfo struct {
	Name     string   // e.g. "abseil-cpp"
	Version  string   // e.g. "7.88.1-10+deb12u5"
	URL      string   // download URL
	Checksum string   // optional pre-known digest
	Provides []string // capabilities this package provides (rpm:entry names)
	Requires []string // capabilities this package requires
}
