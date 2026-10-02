// Package schemas embeds the existing review contract for executable adapters.
package schemas

import _ "embed"

//go:embed review.schema.json
var Review []byte
