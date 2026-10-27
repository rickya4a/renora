// Package migrations embeds the SQL migrations so every binary carries the
// schema it was built against.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
