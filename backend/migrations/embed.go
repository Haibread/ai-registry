// Package migrations embeds all SQL migration files so the Go binary is
// self-contained and can run migrations without filesystem access.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
