//go:build js

package main

import (
	"embed"
	"io/fs"
	"log"
)

// assetFS holds the game data of the browser build: webassets/, the smaller copy of
// assets/ that scripts/web_assets.py writes (make web-assets), with the characters'
// pictures as WebP at the size the game draws them. The whole WebAssembly file is
// downloaded before the game starts, and assets/ as it is would make it over 200 MB.
// webassets/ is generated and not committed: run make web, not a plain GOOS=js go build.
//
//go:embed all:webassets
var assetFS embed.FS

// embeddedAssets is the embedded assets directory, as the game reads it (the root holds
// ui/, blocks/, fonts/ and characters/).
func embeddedAssets() fs.FS {
	sub, err := fs.Sub(assetFS, "webassets")
	if err != nil {
		log.Fatal(err) // "webassets" is a valid path: fs.Sub cannot fail on it
	}
	return sub
}
