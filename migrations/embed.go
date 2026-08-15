// Package migrations embeds the SQL migration files into the server
// binary so the deployment stays a single artifact (see DESIGN.md
// section 12): no separate migrations directory has to be shipped or
// mounted alongside the executable.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
