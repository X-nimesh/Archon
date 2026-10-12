// Package migrations embeds the versioned SQL schema files.
//
// The .sql files sit next to this file so they stay visible and reviewable in
// the repository. Embedding them (rather than reading from disk) means the
// compiled binary is self-contained: the same migrations run in a container, on
// a laptop, and in CI without shipping loose files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
