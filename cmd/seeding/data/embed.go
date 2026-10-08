// Package data embeds the authored seed datasets. Each language owns a
// data/<ISO 639-1 code>/ directory; the catalog package resolves files by
// "<language>/<file>" so adding a language adds a directory, not Go symbols.
package data

import "embed"

//go:embed */*.json
var FS embed.FS
