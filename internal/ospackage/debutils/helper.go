package debutils

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/open-edge-platform/os-image-composer/internal/ospackage"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
)

// GenerateSPDXFileName creates a SPDX manifest filename based on repository configuration
func GenerateSPDXFileName(repoNm string) string {
	timestamp := time.Now().Format("20060102_150405")
	SPDXFileNm := filepath.Join("spdx_manifest_deb_" + strings.ReplaceAll(repoNm, " ", "_") + "_" + timestamp + ".json")
	return SPDXFileNm
}

func GetPackageFromList(all []ospackage.PackageInfo, pkgNm string) ospackage.PackageInfo {
	for _, pkg := range all {
		if pkg.Name == pkgNm {
			return pkg
		}
	}
	return ospackage.PackageInfo{}
}

func ExtractPkgListFrmAptOutput(output, sourcePackage string, downloaded []ospackage.PackageInfo, installed []ospackage.PackageInfo) []ospackage.PackageInfo {

	log := logger.Logger()

	var foundPackages []ospackage.PackageInfo

	if output == "" {
		return foundPackages
	}

	extractor := NewAptOutputExtractor()
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		packages := extractor.ExtractPackages(line)
		if len(packages) > 0 {
			log.Infof("Extracted packages apt output from %s: %v", sourcePackage, packages)
			for _, p := range packages {
				pkgInfo := GetPackageFromList(downloaded, p)
				if pkgInfo.Name != "" {
					// Check if package is already in installed list
					alreadyInstalled := false
					for _, installedPkg := range installed {
						if installedPkg.Name == pkgInfo.Name {
							alreadyInstalled = true
							break
						}
					}
					// Only add if not already installed
					if !alreadyInstalled {
						foundPackages = append(foundPackages, pkgInfo)
					}
				}
			}
		}
	}

	return foundPackages
}
