// Package migrations embeds the SQL migration files so they ship inside the
// binary (golang-migrate iofs source). Numbering uses a timestamp-prefix
// sequence to avoid merge collisions (docs/delivery/01-repository-structure.md §5).
package migrations

import "embed"

// FS holds every *.sql migration (NNNN_description.{up,down}.sql).
//
//go:embed *.sql
var FS embed.FS
