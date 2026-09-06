// Package store contains the embedded schema migrations and (once generated)
// the sqlc-produced type-safe query layer for Pinalert.
//
// Migrations are embedded so that both cmd/migrate and internal/testutil can
// apply the exact same schema without depending on the goose CLI or any file
// on disk relative to the binary's working directory.
package store

import "embed"

// MigrationsFS embeds every goose SQL migration in this directory. It is the
// single source of schema truth consumed by cmd/migrate (real deploys) and
// internal/testutil (test databases, including CI's postgres:16 service
// container, which has no goose CLI installed).
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
