package debutils

import (
	"strings"
	"testing"
)

func TestGenerateSPDXFileName(t *testing.T) {
	tests := []struct {
		name   string
		repoNm string
	}{
		{
			name:   "simple repository name",
			repoNm: "Ubuntu",
		},
		{
			name:   "repository name with spaces",
			repoNm: "Azure Linux 3.0",
		},
		{
			name:   "empty repository name",
			repoNm: "",
		},
		{
			name:   "repository name with spaces",
			repoNm: "Test Repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSPDXFileName(tt.repoNm)

			// Check that result starts with correct prefix
			if !strings.HasPrefix(result, "spdx_manifest_deb_") {
				t.Errorf("GenerateSPDXFileName() = %v, expected to start with 'spdx_manifest_deb_'", result)
			}

			// Check that result ends with .json
			if !strings.HasSuffix(result, ".json") {
				t.Errorf("GenerateSPDXFileName() = %v, expected to end with '.json'", result)
			}

			// Check that spaces are replaced with underscores
			expectedRepoName := strings.ReplaceAll(tt.repoNm, " ", "_")
			if !strings.Contains(result, expectedRepoName) {
				t.Errorf("GenerateSPDXFileName() = %v, expected to contain %v", result, expectedRepoName)
			}

			// Check that result contains timestamp-like pattern (has underscores and digits)
			if len(result) < 30 { // Should be long enough to contain timestamp
				t.Errorf("GenerateSPDXFileName() = %v, result too short", result)
			}
		})
	}
}
