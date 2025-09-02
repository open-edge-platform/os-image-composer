package debutils_test

import (
	"testing"

	"github.com/open-edge-platform/image-composer/internal/ospackage/debutils"
	"github.com/open-edge-platform/image-composer/internal/ospackage/resolvertest"
)

func TestDEBResolver(t *testing.T) {
	resolvertest.RunResolverTestsFunc(
		t,
		"debutils",
		debutils.ResolveDependencies, // directly passing your function
	)
}

func TestCleanDependencyName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		// Simple package name
		{"libc6", "libc6"},

		// Version constraints
		{"libc6 (>= 2.34)", "libc6"},
		{"python3 (= 3.9.2-1)", "python3"},

		// Alternatives - should take first option
		{"python3 | python3-dev", "python3"},
		{"mailx | bsd-mailx | s-nail", "mailx"},

		// Architecture qualifiers
		{"gcc:amd64", "gcc"},
		{"g++:arm64", "g++"},

		// Complex combinations
		{"gcc-aarch64-linux-gnu (>= 4:10.2) | gcc:arm64", "gcc-aarch64-linux-gnu"},
		{"systemd | systemd-standalone-sysusers | systemd-sysusers", "systemd"},

		// Edge cases
		{"", ""},
		{"   spaced   ", "spaced"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := debutils.CleanDependencyName(tc.input)
			if result != tc.expected {
				t.Errorf("cleanDependencyName(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
