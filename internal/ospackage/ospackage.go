package ospackage

// PackageInfo holds everything you need to fetch + verify one artifact.
type PackageInfo struct {
	Name     string   // e.g. "abseil-cpp"
	Version  string   // e.g. "7.88.1-10+deb12u5"
	Arch     string   // e.g. "x86_64", "noarch", "src"
	URL      string   // download URL
	Checksum string   // optional pre-known digest
	Provides []string // capabilities this package provides (rpm:entry names)
	Requires []string // capabilities this package requires
	Files    []string // list of files in this package (rpm:files)
}
