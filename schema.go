package bango

import _ "embed"

//go:embed panel.schema.json
var schema []byte

func Schema() []byte { return schema }
