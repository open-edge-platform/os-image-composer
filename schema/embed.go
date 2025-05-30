package schema

import _ "embed"

//go:embed os-image-composer.schema.json
var ComposerSchema []byte

//go:embed os-image-template.schema.json
var ImageSchema []byte

//go:embed os-image-composer-config.schema.json
var ConfigSchema []byte
