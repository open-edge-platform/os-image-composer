package debutils

import (
	"reflect"
	"testing"
)

func TestNewAptOutputExtractor(t *testing.T) {
	extractor := NewAptOutputExtractor()

	if extractor == nil {
		t.Fatal("NewAptOutputExtractor() returned nil")
	}

	if extractor.IsExtracting() {
		t.Error("New extractor should not be in extracting mode")
	}

	if extractor.startPattern == nil {
		t.Error("startPattern should be initialized")
	}

	if extractor.summaryPattern == nil {
		t.Error("summaryPattern should be initialized")
	}
}

func TestExtractPackages_StartIndicator(t *testing.T) {
	extractor := NewAptOutputExtractor()

	testCases := []string{
		"The following NEW packages will be installed:",
		"The following additional packages will be installed:",
		"The following packages will be installed:",
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			result := extractor.ExtractPackages(testCase)

			if len(result) != 0 {
				t.Errorf("Expected empty slice for start indicator, got %v", result)
			}

			if !extractor.IsExtracting() {
				t.Error("Extractor should be in extracting mode after start indicator")
			}
		})
	}
}

func TestExtractPackages_EndIndicator(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// First set to extracting mode
	extractor.ExtractPackages("The following NEW packages will be installed:")

	testCases := []string{
		"0 upgraded, 11 newly installed, 0 to remove and 0 not upgraded.",
		"5 upgraded, 3 newly installed, 1 to remove and 12 not upgraded.",
		"10 upgraded, 0 newly installed, 0 to remove and 5 not upgraded.",
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			// Reset to extracting mode for each test
			extractor.isExtracting = true

			result := extractor.ExtractPackages(testCase)

			if len(result) != 0 {
				t.Errorf("Expected empty slice for end indicator, got %v", result)
			}

			if extractor.IsExtracting() {
				t.Error("Extractor should not be in extracting mode after end indicator")
			}
		})
	}
}

func TestExtractPackages_PackageExtraction(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// Set to extracting mode
	extractor.ExtractPackages("The following NEW packages will be installed:")

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single package",
			input:    "libbsd0",
			expected: []string{"libbsd0"},
		},
		{
			name:     "Multiple packages",
			input:    "libbsd0 libcbor0.8 libcom-err2",
			expected: []string{"libbsd0", "libcbor0.8", "libcom-err2"},
		},
		{
			name:     "Packages with spaces and indentation",
			input:    "  libbsd0 libcbor0.8 libcom-err2 libedit2  ",
			expected: []string{"libbsd0", "libcbor0.8", "libcom-err2", "libedit2"},
		},
		{
			name:     "Packages with various valid characters",
			input:    "lib-test lib+plus lib.dot lib123 test-pkg",
			expected: []string{"lib-test", "lib+plus", "lib.dot", "lib123", "test-pkg"},
		},
		{
			name:     "Empty line",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure we're in extracting mode
			extractor.isExtracting = true

			result := extractor.ExtractPackages(tc.input)

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestExtractPackages_NotExtracting(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// Test various inputs when not in extracting mode
	testCases := []string{
		"libbsd0 libcbor0.8",
		"some random text",
		"Reading package lists... Done",
		"Building dependency tree",
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			result := extractor.ExtractPackages(testCase)

			if len(result) != 0 {
				t.Errorf("Expected empty slice when not extracting, got %v", result)
			}
		})
	}
}

func TestExtractPackages_FullWorkflow(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// Simulate a complete apt install output
	testLines := []struct {
		line     string
		expected []string
	}{
		{"Reading package lists... Done", []string{}},
		{"Building dependency tree", []string{}},
		{"The following NEW packages will be installed:", []string{}},
		{"  libbsd0 libcbor0.8 libcom-err2 libedit2 libfido2-1 libgssapi-krb5-2", []string{"libbsd0", "libcbor0.8", "libcom-err2", "libedit2", "libfido2-1", "libgssapi-krb5-2"}},
		{"  libk5crypto3 libkeyutils1 libkrb5-3 libkrb5support0 openssh-client", []string{"libk5crypto3", "libkeyutils1", "libkrb5-3", "libkrb5support0", "openssh-client"}},
		{"0 upgraded, 11 newly installed, 0 to remove and 0 not upgraded.", []string{}},
		{"Need to get 2,500 kB of archives.", []string{}},
	}

	for i, testCase := range testLines {
		t.Run(testCase.line, func(t *testing.T) {
			result := extractor.ExtractPackages(testCase.line)

			if !reflect.DeepEqual(result, testCase.expected) {
				t.Errorf("Line %d: Expected %v, got %v", i+1, testCase.expected, result)
			}
		})
	}
}

func TestParsePackageNames_InvalidPackages(t *testing.T) {
	extractor := NewAptOutputExtractor()

	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Package starting with invalid character",
			input:    "-invalid +invalid .invalid",
			expected: []string{},
		},
		{
			name:     "Mixed valid and invalid packages",
			input:    "valid-pkg -invalid another-valid +invalid",
			expected: []string{"valid-pkg", "another-valid"},
		},
		{
			name:     "Package with invalid characters",
			input:    "pkg@invalid pkg#invalid valid-pkg",
			expected: []string{"valid-pkg"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractor.parsePackageNames(tc.input)

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestIsExtracting(t *testing.T) {
	extractor := NewAptOutputExtractor()

	if extractor.IsExtracting() {
		t.Error("New extractor should not be extracting")
	}

	// Start extracting
	extractor.ExtractPackages("The following NEW packages will be installed:")
	if !extractor.IsExtracting() {
		t.Error("Extractor should be extracting after start indicator")
	}

	// Stop extracting
	extractor.ExtractPackages("0 upgraded, 11 newly installed, 0 to remove and 0 not upgraded.")
	if extractor.IsExtracting() {
		t.Error("Extractor should not be extracting after end indicator")
	}
}

func TestReset(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// Set to extracting mode
	extractor.ExtractPackages("The following NEW packages will be installed:")
	if !extractor.IsExtracting() {
		t.Error("Should be extracting before reset")
	}

	// Reset
	extractor.Reset()
	if extractor.IsExtracting() {
		t.Error("Should not be extracting after reset")
	}
}

func TestExtractPackages_MultipleExtractionSessions(t *testing.T) {
	extractor := NewAptOutputExtractor()

	// First session
	extractor.ExtractPackages("The following NEW packages will be installed:")
	result1 := extractor.ExtractPackages("package1 package2")
	extractor.ExtractPackages("0 upgraded, 2 newly installed, 0 to remove and 0 not upgraded.")

	// Second session
	extractor.ExtractPackages("The following additional packages will be installed:")
	result2 := extractor.ExtractPackages("package3 package4")
	extractor.ExtractPackages("1 upgraded, 2 newly installed, 0 to remove and 5 not upgraded.")

	expectedResult1 := []string{"package1", "package2"}
	expectedResult2 := []string{"package3", "package4"}

	if !reflect.DeepEqual(result1, expectedResult1) {
		t.Errorf("First session: Expected %v, got %v", expectedResult1, result1)
	}

	if !reflect.DeepEqual(result2, expectedResult2) {
		t.Errorf("Second session: Expected %v, got %v", expectedResult2, result2)
	}
}

func BenchmarkExtractPackages(b *testing.B) {
	extractor := NewAptOutputExtractor()
	extractor.ExtractPackages("The following NEW packages will be installed:")

	testLine := "  libbsd0 libcbor0.8 libcom-err2 libedit2 libfido2-1 libgssapi-krb5-2"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor.ExtractPackages(testLine)
	}
}

func BenchmarkNewAptOutputExtractor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewAptOutputExtractor()
	}
}
