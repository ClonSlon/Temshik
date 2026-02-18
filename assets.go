package temchik

import (
	"embed"
	"io/fs"
)

// Frontend assets (prebuilt) copied from the TypeScript UI build.
//
//go:embed app/frontend
var embeddedFrontend embed.FS

func FrontendFS() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "app/frontend")
}
