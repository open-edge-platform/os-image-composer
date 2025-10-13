package aiagent

import (
    "fmt"
    "os"
    "strings"
    
    "gopkg.in/yaml.v3"
)

// UseCaseConfig defines a use case and its package mappings
type UseCaseConfig struct {
    Name                string            `yaml:"name"`
    Description         string            `yaml:"description"`
    Keywords            []string          `yaml:"keywords"`
    EssentialPackages   []string          `yaml:"essential_packages"`
    OptionalPackages    []string          `yaml:"optional_packages"`
    SecurityPackages    []string          `yaml:"security_packages"`
    PerformancePackages []string          `yaml:"performance_packages"`
    Kernel              UseCaseKernel     `yaml:"kernel"`
    Disk                UseCaseDisk       `yaml:"disk"`
}

type UseCaseKernel struct {
    DefaultVersion string `yaml:"default_version"`
    Cmdline        string `yaml:"cmdline"`
}

type UseCaseDisk struct {
    DefaultSize string `yaml:"default_size"`
}

// UseCasesConfig contains all use case definitions
type UseCasesConfig struct {
    UseCases map[string]UseCaseConfig `yaml:"use_cases"`
}

var (
    defaultUseCasesPath = "use-cases.yml"
    loadedUseCases      *UseCasesConfig
)

// LoadUseCases loads use cases from configuration file
func LoadUseCases(path string) (*UseCasesConfig, error) {
    if path == "" {
        path = defaultUseCasesPath
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read use cases config: %w", err)
    }
    
    var config UseCasesConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse use cases config: %w", err)
    }
    
    loadedUseCases = &config
    return &config, nil
}

// GetUseCase returns a specific use case configuration
func (uc *UseCasesConfig) GetUseCase(name string) (*UseCaseConfig, error) {
    useCase, exists := uc.UseCases[name]
    if !exists {
        return nil, fmt.Errorf("use case '%s' not found", name)
    }
    return &useCase, nil
}

// DetectUseCase detects use case from user input based on keywords
func (uc *UseCasesConfig) DetectUseCase(input string) string {
    input = strings.ToLower(input)
    
    // Score each use case based on keyword matches
    scores := make(map[string]int)
    for useCaseName, useCase := range uc.UseCases {
        for _, keyword := range useCase.Keywords {
            if strings.Contains(input, keyword) {
                scores[useCaseName]++
            }
        }
    }
    
    // Return use case with highest score
    maxScore := 0
    bestUseCase := "web-server" // default fallback
    for useCaseName, score := range scores {
        if score > maxScore {
            maxScore = score
            bestUseCase = useCaseName
        }
    }
    
    return bestUseCase
}

// GetAllUseCaseNames returns list of all available use case names
func (uc *UseCasesConfig) GetAllUseCaseNames() []string {
    names := make([]string, 0, len(uc.UseCases))
    for name := range uc.UseCases {
        names = append(names, name)
    }
    return names
}

// GetPackagesForUseCase returns packages based on use case and requirements
func (uc *UseCasesConfig) GetPackagesForUseCase(useCaseName string, requirements []string) ([]string, error) {
    useCase, err := uc.GetUseCase(useCaseName)
    if err != nil {
        return nil, err
    }
    
    packages := make([]string, 0)
    
    // Always include essential packages
    packages = append(packages, useCase.EssentialPackages...)
    
    // Add packages based on requirements
    hasMinimal := false
    for _, req := range requirements {
        switch req {
        case "security":
            packages = append(packages, useCase.SecurityPackages...)
        case "performance":
            packages = append(packages, useCase.PerformancePackages...)
        case "minimal":
            hasMinimal = true
        }
    }
    
    // Add optional packages unless minimal is specified
    if !hasMinimal && len(useCase.OptionalPackages) > 0 {
        packages = append(packages, useCase.OptionalPackages...)
    }
    
    return uniqueStrings(packages), nil
}

// GetKernelVersion returns kernel version for use case and distribution
func (uc *UseCasesConfig) GetKernelVersion(useCaseName, distribution string) string {
    // Distribution-specific versions
    distVersions := map[string]string{
        "azl3":   "6.12",
        "emt3":   "6.12",
        "elxr12": "6.1",
    }
    
    // First check distribution-specific version
    if ver, ok := distVersions[distribution]; ok {
        return ver
    }
    
    // Fallback to use case default
    if useCase, err := uc.GetUseCase(useCaseName); err == nil {
        if useCase.Kernel.DefaultVersion != "" {
            return useCase.Kernel.DefaultVersion
        }
    }
    
    return "6.12" // final fallback
}

// GetKernelCmdline returns kernel cmdline for use case
func (uc *UseCasesConfig) GetKernelCmdline(useCaseName string) string {
    if useCase, err := uc.GetUseCase(useCaseName); err == nil {
        if useCase.Kernel.Cmdline != "" {
            return useCase.Kernel.Cmdline
        }
    }
    
    return "console=ttyS0,115200 console=tty0 loglevel=7" // default
}

// GetDiskSize returns default disk size for use case
func (uc *UseCasesConfig) GetDiskSize(useCaseName string) string {
    if useCase, err := uc.GetUseCase(useCaseName); err == nil {
        if useCase.Disk.DefaultSize != "" {
            return useCase.Disk.DefaultSize
        }
    }
    
    return "8GiB" // default
}

// HasUseCase checks if a use case exists
func (uc *UseCasesConfig) HasUseCase(name string) bool {
	_, exists := uc.UseCases[name]
	return exists
}
