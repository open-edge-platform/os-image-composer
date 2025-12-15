package debutils

import (
	"regexp"
	"strings"
)

// AptOutputExtractor handles the extraction of package names from apt install output
type AptOutputExtractor struct {
	isExtracting   bool
	startPattern   *regexp.Regexp
	summaryPattern *regexp.Regexp
}

// NewAptOutputExtractor creates a new instance of the apt output extractor
func NewAptOutputExtractor() *AptOutputExtractor {
	return &AptOutputExtractor{
		isExtracting:   false,
		startPattern:   regexp.MustCompile(`^The following.*packages will be installed:`),
		summaryPattern: regexp.MustCompile(`^\d+\s+(upgraded|newly installed|to remove|not upgraded)`),
	}
}

// ExtractPackages processes a single line of apt output and returns extracted package names
// Returns an empty slice when not in extraction mode or when encountering start/end markers
func (e *AptOutputExtractor) ExtractPackages(line string) []string {
	line = strings.TrimSpace(line)

	// Check if this is the start of package listing
	if e.startPattern.MatchString(line) {
		e.isExtracting = true
		return []string{} // Indicate start of extraction but return no packages
	}

	// Check if this is the summary line (end of package listing)
	if e.summaryPattern.MatchString(line) {
		e.isExtracting = false
		return []string{} // Indicate end of extraction
	}

	// If we're in extraction mode and line is not empty, extract package names
	if e.isExtracting && line != "" {
		return e.parsePackageNames(line)
	}

	return []string{} // Return empty slice if not in extraction mode
}

// parsePackageNames extracts and validates package names from a line
func (e *AptOutputExtractor) parsePackageNames(line string) []string {
	fields := strings.Fields(line)
	packages := make([]string, 0, len(fields))

	// Regex for valid Debian package names
	validPackagePattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9+.-]*$`)

	for _, field := range fields {
		if field != "" && validPackagePattern.MatchString(field) {
			packages = append(packages, field)
		}
	}

	return packages
}

// IsExtracting returns whether the extractor is currently in extraction mode
func (e *AptOutputExtractor) IsExtracting() bool {
	return e.isExtracting
}

// Reset resets the extractor state to start fresh
func (e *AptOutputExtractor) Reset() {
	e.isExtracting = false
}
