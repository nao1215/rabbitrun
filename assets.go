package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/gfx"
)

// assetFS holds the game data. Only the public manifest (game.json) and the
// images are embedded; nothing else under assets/characters is shipped.
//
//go:embed assets/blocks assets/fonts assets/ui assets/characters/*/game.json all:assets/characters/*/images
var assetFS embed.FS

// embeddedAssets is the embedded assets directory, as package assets reads it.
func embeddedAssets() fs.FS {
	sub, err := fs.Sub(assetFS, "assets")
	if err != nil {
		log.Fatal(err) // "assets" is a valid path: fs.Sub cannot fail on it
	}
	return sub
}

// characters are the playable characters, in the order of the select screen.
var characters []*character.Character

// loadAssets loads the fonts and the characters from the assets directory (assets.Use).
func loadAssets() {
	mplus, err := assets.ReadFile("fonts/mplus-1p-regular.ttf")
	if err != nil {
		log.Fatal(err)
	}
	pop, err := assets.ReadFile("fonts/LilitaOne-Regular.ttf")
	if err != nil {
		log.Printf("cannot read the pop font, using the default font: %v", err)
	}
	if err := gfx.LoadFonts(mplus, pop); err != nil {
		log.Fatal(err)
	}
	characters, err = character.Read(assets.FS())
	if err != nil {
		log.Fatal(err)
	}
	if len(characters) == 0 {
		log.Fatal("no characters found")
	}
}

// playCGs are the illustrations the game in play unlocks: the extras in the extra mode.
func playCGs(c *character.Character) []character.ImageEntry {
	if extraMode() {
		return c.ExtraCGs()
	}
	return c.MainCGs()
}

// galleryCGs are the illustrations the gallery lists: only the regular ones until the
// hidden command has been found, so a full gallery looks complete.
func galleryCGs(c *character.Character) []character.ImageEntry {
	if store.Data.ExtraFound {
		return c.CGs
	}
	return c.MainCGs()
}

// extraMode reports whether the extra stages are being played.
func extraMode() bool { return store.Data.ExtraFound && store.Data.ExtraMode }
