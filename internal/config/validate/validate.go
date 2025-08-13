package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/open-edge-platform/image-composer/internal/config/schema"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	imageSchemaName  = "os-image-template.schema.json"
	configSchemaName = "os-image-composer-config.schema.json"
	userRef          = "#/$defs/UserTemplate"
	fullRef          = "#/$defs/FullTemplate"
)

// ValidateAgainstSchema compiles the given schema bytes and runs it against
// the JSON in data.  The `name` is only used to identify the schema in errors.
func ValidateAgainstSchema(name string, schemaBytes, data []byte, ref string) error {
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(name, bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("loading schema %q: %w", name, err)
	}

	// If ref is empty we compile the root; otherwise compile the subschema.
	target := name
	if ref != "" {
		switch {
		case strings.HasPrefix(ref, "#"):
			target = name + ref
		case strings.HasPrefix(ref, "/"):
			target = name + "#" + ref
		default:
			// treat as anchor name (e.g., "UserTemplate")
			target = name + "#" + ref
		}
	}
	sch, err := comp.Compile(target)
	if err != nil {
		return fmt.Errorf("compiling schema %q: %w", name, err)
	}

	// unmarshal into interface{} so the validator can walk it
	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid JSON for %q: %w", name, err)
	}
	if err := sch.Validate(doc); err != nil {
		return fmt.Errorf("schema validation against %q failed: %w", name, err)
	}
	return nil
}

// ValidateImageTemplateJSON runs the template schema against data
func ValidateImageTemplateJSON(data []byte) error {
	return ValidateAgainstSchema(
		imageSchemaName,
		schema.ImageTemplateSchema,
		data,
		fullRef,
	)
}

// User-provided (minimal) template
func ValidateUserTemplateJSON(data []byte) error {
	return ValidateAgainstSchema(
		imageSchemaName,
		schema.ImageTemplateSchema,
		data,
		userRef,
	)
}

// ValidateConfigJSON runs the config schema against data
func ValidateConfigJSON(data []byte) error {
	return ValidateAgainstSchema(
		configSchemaName,
		schema.ConfigSchema,
		data,
		"",
	)
}
