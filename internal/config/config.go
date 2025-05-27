// internal/config/config.go
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	utils "github.com/open-edge-platform/image-composer/internal/utils/logger"
	"github.com/open-edge-platform/image-composer/internal/validate"
)

// BuildSpec represents your JSON schema.
type BuildSpec struct {
	Distro    string       `json:"distro"`
	Version   string       `json:"version"`
	Arch      string       `json:"arch"`
	Packages  []string     `json:"packages"`
	Immutable bool         `json:"immutable"`
	Output    string       `json:"output"`
	Kernel    KernelConfig `json:"kernel"`
}

// KernelConfig holds the nested “kernel” object.
type KernelConfig struct {
	Version string `json:"version"`
	Cmdline string `json:"cmdline"`
}

func Load(path string) (*BuildSpec, error) {
	logger := utils.Logger()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// validate raw JSON against schema
	if err := validate.ValidateComposerJSON(data); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	// unmarshal into typed struct
	var bc BuildSpec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bc); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	logger.Infof("loaded config: \n%s", string(data))
	return &bc, nil
}
