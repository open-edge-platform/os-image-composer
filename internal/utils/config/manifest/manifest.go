// internal/manifest/manifest.go
package manifest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/open-edge-platform/image-composer/internal/utils/general/logger"
)

// SoftwarePackageManifest represents the structure of the manifest file.
type SoftwarePackageManifest struct {
	SchemaVersion     string `json:"schema_version"`
	ImageVersion      string `json:"image_version"`
	BuiltAt           string `json:"built_at"`
	Arch              string `json:"arch"`
	SizeBytes         int64  `json:"size_bytes"`
	Hash              string `json:"hash"`
	HashAlg           string `json:"hash_alg"`
	Signature         string `json:"signature"`
	SigAlg            string `json:"sig_alg"`
	MinCurrentVersion string `json:"min_current_version"`
}

// WriteManifestToFile writes the manifest to the specified output file.
func WriteManifestToFile(manifest SoftwarePackageManifest, outputFile string) error {
	log := logger.Logger()
	log.Infof("Writing the Image Manifest to the file: %s", outputFile)
	// Marshal the manifest struct to JSON
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling manifest to JSON: %w", err)
	}

	// Create or open the output file
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating/opening file: %w", err)
	}
	defer file.Close()

	// Write the JSON data to the file
	_, err = file.Write(manifestJSON)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
