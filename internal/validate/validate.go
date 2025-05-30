package validate

import (
	"bytes"
	"encoding/json"
	"fmt"

	schema_pkg "github.com/open-edge-platform/image-composer/schema"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ValidateAgainstSchema compiles the given schema bytes and runs it against
// the JSON in data.  The `name` is only used to identify the schema in errors.
func ValidateAgainstSchema(name string, schemaBytes, data []byte) error {
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(name, bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("loading schema %q: %w", name, err)
	}
	sch, err := comp.Compile(name)
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

// ValidateComposerJSON runs the composer schema against data
func ValidateComposerJSON(data []byte) error {
	return ValidateAgainstSchema(
		"os-image-composer.schema.json",
		schema_pkg.ComposerSchema,
		data,
	)
}

// ValidateTemplateJSON runs the template schema against data
func ValidateImageJSON(data []byte) error {
	return ValidateAgainstSchema(
		"os-image-template.schema.json",
		schema_pkg.ImageSchema,
		data,
	)
}

// ValidateConfigJSON runs the config schema against data
func ValidateConfigJSON(data []byte) error {
	return ValidateAgainstSchema(
		"os-image-composer-config.schema.json",
		schema_pkg.ConfigSchema,
		data,
	)
}
