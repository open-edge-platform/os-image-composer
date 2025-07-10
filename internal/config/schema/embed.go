package schema

import _ "embed"

//go:embed os-image-template.schema.json
var ImageTemplateSchema []byte

//go:embed os-image-composer-config.schema.json
var ConfigSchema []byte
