package schema

import _ "embed"

// OpenAPI holds the embedded OpenAPI (Swagger) YAML for the Lumera Supply API.
//go:embed openapi.yaml
var OpenAPI []byte
