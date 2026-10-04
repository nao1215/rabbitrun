package main

import (
	"embed"
	"io/fs"
	"log"
)

// assetFS holds the game data. Only the public manifest (game.json) and the
// images are embedded; nothing else under assets/characters is shipped. It is declared
// here, at the module root, because go:embed only reaches files under the package's own
// directory; the game reads it through embeddedAssets.
//
//go:embed assets/blocks assets/fonts assets/ui assets/characters/*/game.json all:assets/characters/*/images
var assetFS embed.FS

// embeddedAssets is the embedded assets directory, as the game reads it (the root holds
// ui/, blocks/, fonts/ and characters/).
func embeddedAssets() fs.FS {
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		log.Fatal(err) // "assets" is a valid path: fs.Sub cannot fail on it
	}
	return sub
}
