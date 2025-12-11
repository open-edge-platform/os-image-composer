package debutils

import (
	"path/filepath"
	"strings"
	"time"
)

// GenerateSPDXFileName creates a SPDX manifest filename based on repository configuration
func GenerateSPDXFileName(repoNm string) string {
	timestamp := time.Now().Format("20060102_150405")
	SPDXFileNm := filepath.Join("spdx_manifest_deb_" + strings.ReplaceAll(repoNm, " ", "_") + "_" + timestamp + ".json")
	return SPDXFileNm
}
