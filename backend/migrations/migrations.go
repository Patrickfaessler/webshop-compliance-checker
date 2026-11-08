// Package migrations embeds the versioned SQL schema files (ADR-5). Files are applied in name order,
// each exactly once; never edit a file that has been applied somewhere, add a new one instead.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
