package aiagent

// ChatMessage for Ollama communication
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RepoIntent captures repository information from user query
type RepoIntent struct {
	Codename  string `json:"codename"`
	URL       string `json:"url"`
	PKey      string `json:"pkey"`
	Component string `json:"component,omitempty"`
}

// Internal types for AI processing
type TemplateIntent struct {
	UseCase             string       `json:"use_case"`
	Requirements        []string     `json:"requirements"`
	Architecture        string       `json:"architecture"`
	Distribution        string       `json:"distribution"`
	ImageType           string       `json:"image_type"`
	ArtifactType        string       `json:"artifact_type"`
	Description         string       `json:"description"`
	CustomPackages      []string     `json:"custom_packages"`
	PackageRepositories []RepoIntent `json:"package_repositories"`
}

// OSImageTemplate - OS Image Composer compatible template structure (matches UserTemplate schema)
type OSImageTemplate struct {
	Image               ImageConfig         `yaml:"image" json:"image"`
	Target              TargetConfig        `yaml:"target" json:"target"`
	Disk                *DiskConfig         `yaml:"disk,omitempty" json:"disk,omitempty"`
	SystemConfig        SystemConfig        `yaml:"systemConfig" json:"systemConfig"`
	PackageRepositories []PackageRepository `yaml:"packageRepositories,omitempty" json:"packageRepositories,omitempty"`
}

type ImageConfig struct {
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

type TargetConfig struct {
	OS        string `yaml:"os" json:"os"`
	Dist      string `yaml:"dist" json:"dist"`
	Arch      string `yaml:"arch" json:"arch"`
	ImageType string `yaml:"imageType" json:"imageType"`
}

type DiskConfig struct {
	Name               string         `yaml:"name" json:"name"`
	Artifacts          []ArtifactSpec `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Size               string         `yaml:"size,omitempty" json:"size,omitempty"`
	PartitionTableType string         `yaml:"partitionTableType,omitempty" json:"partitionTableType,omitempty"`
	Partitions         []Partition    `yaml:"partitions,omitempty" json:"partitions,omitempty"`
}

type ArtifactSpec struct {
	Type        string `yaml:"type" json:"type"`
	Compression string `yaml:"compression,omitempty" json:"compression,omitempty"`
}

type Partition struct {
	ID           string   `yaml:"id" json:"id"`
	Type         string   `yaml:"type" json:"type"`
	Flags        []string `yaml:"flags,omitempty" json:"flags,omitempty"`
	Start        string   `yaml:"start" json:"start"`
	End          string   `yaml:"end" json:"end"`
	FSType       string   `yaml:"fsType" json:"fsType"`
	MountPoint   string   `yaml:"mountPoint" json:"mountPoint"`
	MountOptions string   `yaml:"mountOptions,omitempty" json:"mountOptions,omitempty"`
}

type SystemConfig struct {
	Name         string        `yaml:"name" json:"name"`
	Description  string        `yaml:"description,omitempty" json:"description,omitempty"`
	Immutability *Immutability `yaml:"immutability,omitempty" json:"immutability,omitempty"`
	Users        []User        `yaml:"users,omitempty" json:"users,omitempty"`
	Packages     []string      `yaml:"packages,omitempty" json:"packages,omitempty"`
	Kernel       *KernelConfig `yaml:"kernel,omitempty" json:"kernel,omitempty"`
}

type Immutability struct {
	Enabled         bool   `yaml:"enabled" json:"enabled"`
	SecureBootDBKey string `yaml:"secureBootDBKey,omitempty" json:"secureBootDBKey,omitempty"`
	SecureBootDBCrt string `yaml:"secureBootDBCrt,omitempty" json:"secureBootDBCrt,omitempty"`
	SecureBootDBCer string `yaml:"secureBootDBCer,omitempty" json:"secureBootDBCer,omitempty"`
}

type User struct {
	Name     string   `yaml:"name" json:"name"`
	Password string   `yaml:"password,omitempty" json:"password,omitempty"`
	Groups   []string `yaml:"groups,omitempty" json:"groups,omitempty"`
	Sudo     bool     `yaml:"sudo,omitempty" json:"sudo,omitempty"`
}

type KernelConfig struct {
	Version  string   `yaml:"version,omitempty" json:"version,omitempty"`
	Cmdline  string   `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
	Packages []string `yaml:"packages,omitempty" json:"packages,omitempty"`
}

type PackageRepository struct {
	Codename  string `yaml:"codename" json:"codename"`
	URL       string `yaml:"url" json:"url"`
	PKey      string `yaml:"pkey" json:"pkey"`
	Component string `yaml:"component,omitempty" json:"component,omitempty"`
}
