package validate

import (
    "bytes"
    "encoding/json"
    "fmt"

    "github.com/santhosh-tekuri/jsonschema/v5"
    schema_pkg "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/schema"
)

// ValidateJSON checks the given JSON data against the embedded schema.
func ValidateJSON(data []byte) error {
    comp := jsonschema.NewCompiler()
    const name = "os-image-composer.schema.json"
    if err := comp.AddResource(name, bytes.NewReader(schema_pkg.JSON)); err != nil {
        return fmt.Errorf("loading schema: %w", err)
    }
    sch, err := comp.Compile(name)
    if err != nil {
        return fmt.Errorf("compiling schema: %w", err)
    }

    var doc interface{}
    if err := json.Unmarshal(data, &doc); err != nil {
        return fmt.Errorf("invalid JSON: %w", err)
    }
    if err := sch.Validate(doc); err != nil {
        return fmt.Errorf("schema validation failed: %w", err)
    }
    return nil
}