package temchik

import (
	"embed"
	"io/fs"
)

// Prisma SQL migrations (used by the original TypeScript project).
//
//go:embed packages/prisma/src/migrations
var embeddedMigrations embed.FS

func MigrationsFS() (fs.FS, error) {
	return fs.Sub(embeddedMigrations, "packages/prisma/src/migrations")
}
