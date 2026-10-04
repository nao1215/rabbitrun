package game

import (
	"errors"
	"io/fs"
	"log"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/gfx"
)

// characters are the playable characters, in the order of the select screen.
var characters []*character.Character

// loadAssets makes fsys the assets directory and loads the fonts and the characters from it.
func loadAssets(fsys fs.FS) error {
	assets.Use(fsys)
	mplus, err := assets.ReadFile("fonts/mplus-1p-regular.ttf")
	if err != nil {
		return err
	}
	pop, err := assets.ReadFile("fonts/LilitaOne-Regular.ttf")
	if err != nil {
		log.Printf("cannot read the pop font, using the default font: %v", err)
	}
	if err := gfx.LoadFonts(mplus, pop); err != nil {
		return err
	}
	chars, err := character.Read(assets.FS())
	if err != nil {
		return err
	}
	if len(chars) == 0 {
		return errors.New("no characters found")
	}
	characters = chars
	return nil
}

// playCGs are the illustrations the game in play unlocks: the extras in the extra mode.
func playCGs(c *character.Character) []character.ImageEntry {
	if extraMode() {
		return c.ExtraCGs()
	}
	return c.MainCGs()
}

// galleryCGs are the illustrations the gallery lists: only the regular ones until the
// hidden command has been found, so a full gallery looks complete. --debug opens
// everything for its run, the extras too.
func galleryCGs(c *character.Character) []character.ImageEntry {
	if extrasListed() {
		return c.CGs
	}
	return c.MainCGs()
}

// extrasListed reports whether the gallery lists the extra illustrations: once the hidden
// command has been found, or under --debug.
func extrasListed() bool { return store.Data.ExtraFound || debugMode }

// galleryIllustrations are the illustrations the gallery lists, in its order: the regular
// ones and her no-miss picture of the regular stages, then (once the hidden command has
// been found, as galleryCGs) the extra ones and her no-miss picture of the extra stages.
// A no-miss picture is listed only when she has one drawn. The endings are not listed:
// every clear shows them again, while a no-miss picture is a reward kept for looking at.
func galleryIllustrations(c *character.Character) []*character.ImageEntry {
	out := make([]*character.ImageEntry, 0, len(c.CGs)+2)
	main := c.MainCGs()
	for i := range main {
		out = append(out, &main[i])
	}
	if c.NoMiss != nil {
		out = append(out, c.NoMiss)
	}
	if !extrasListed() {
		return out // the extras are not known yet
	}
	extra := c.ExtraCGs()
	for i := range extra {
		out = append(out, &extra[i])
	}
	if c.NoMissExtra != nil {
		out = append(out, c.NoMissExtra)
	}
	return out
}

// extraMode reports whether the extra stages are being played.
func extraMode() bool { return store.Data.ExtraFound && store.Data.ExtraMode }
