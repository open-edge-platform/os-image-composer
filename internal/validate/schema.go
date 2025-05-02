package validate

import (
    "bytes"

    "github.com/santhosh-tekuri/jsonschema/v5"
    schema_pkg "github.com/intel-innersource/os.linux.tiberos.os-curation-tool/schema"
)

func MustCompileSchema() *jsonschema.Schema {
    comp := jsonschema.NewCompiler()
    const name = "os-image-composer.schema.json"

    // Use the bytes from schema_pkg.JSON
    if err := comp.AddResource(name, bytes.NewReader(schema_pkg.JSON)); err != nil {
        panic(err)
    }
    sch, err := comp.Compile(name)
    if err != nil {
        panic(err)
    }
    return sch
}