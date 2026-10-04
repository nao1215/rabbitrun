// Package character loads the characters: their manifests (characters/<id>/game.json in the
// assets directory) and their pictures, the portraits of each situation and the
// illustrations, decoded on use or ahead in the background.
package character

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/assets"
)

// Expression IDs switched by the road ahead and what happens on it. game.json lists an image under each ID in expressions.
const (
	ExprNormal   = "normal"   // default
	ExprRelaxed  = "relaxed"  // the road ahead is wide and easy
	ExprHappy    = "happy"    // picked up a sweet
	ExprGreat    = "great"    // picked up a macaron
	ExprExcited  = "excited"  // a hammer swung, an extra life picked up
	ExprTreat    = "treat"    // picked up an extra life on the road
	ExprCombo    = "combo"    // five sweets in a row
	ExprPerfect  = "perfect"  // a stage cleared
	ExprWorried  = "worried"  // the road is getting narrow
	ExprNervous  = "nervous"  // the road is narrow
	ExprPanic    = "panic"    // the road is at its narrowest
	ExprCrying   = "crying"   // she ran into a wall
	ExprGameOver = "gameover" // game over

	// Reactions to the road and to events (see the play screen in package game: readRoad and handleEvents).
	// blocked and ready are not reacted to in play; their portraits show in the gallery.
	ExprOops    = "oops"    // an extra life on the road got away
	ExprBlocked = "blocked" // the road ahead is shut by a gate or a wall across
	ExprReady   = "ready"   // the countdown before the road moves
	ExprWaiting = "waiting" // a sweet is coming up ahead
	ExprRelief  = "relief"  // back on a wide road after a narrow stretch
	ExprLevelUp = "levelup" // a course cleared
	ExprDrought = "drought" // a long stretch without sweets

	// ExprComeback is the "let's go!" pose shown when the player retries after a game over.
	ExprComeback = "comeback"
)

// exprFallback is the situation whose portraits stand in when a character has no
// portrait for a reaction yet.
var exprFallback = map[string]string{
	ExprOops: ExprWorried, ExprBlocked: ExprNervous, ExprReady: ExprGreat, ExprWaiting: ExprExcited,
	ExprRelief: ExprRelaxed, ExprLevelUp: ExprHappy, ExprDrought: ExprWorried,
	ExprComeback: ExprExcited,
}

// StandIn is the situation whose portraits stand in for the reaction state when a
// character has no portrait of her own for it; ok is false for a state without one.
func StandIn(state string) (string, bool) {
	s, ok := exprFallback[state]
	return s, ok
}

// Character is a playable character, as her manifest (game.json) describes her.
type Character struct {
	ID          string       `json:"id"`     // never shown on screen (characters are unnamed)
	Order       int          `json:"order"`  // order on the character select screen
	Secret      bool         `json:"secret"` // the secret character, locked until the others clear
	Expressions []ImageEntry `json:"expressions"`
	Group       *ImageEntry  `json:"group"`  // pose for the group picture of the secret title command
	Select      *ImageEntry  `json:"select"` // full-body image for character select (modest outfit)
	Cutin       *ImageEntry  `json:"-"`      // the big cut-in when a hammer is swung (images/cutin.png)
	// Ending and EndingExtra are the pictures of the all clear, of the regular and of the
	// extra stages (images/ending, images/ending_extra): the shape of the window, to fill it.
	Ending      *ImageEntry  `json:"-"`
	EndingExtra *ImageEntry  `json:"-"`
	CGs         []ImageEntry `json:"cgs"` //nolint:tagliatelle // key used by the existing game.json files
}

// SelectEntry returns the portrait used for character select: the select image, or the
// normal expression.
func (c *Character) SelectEntry() *ImageEntry {
	if c.Select != nil && c.Select.HasImage() {
		return c.Select
	}
	return c.Expression(ExprNormal)
}

// SelectImage returns the character select image, or the default expression if none.
func (c *Character) SelectImage() *ebiten.Image { return c.SelectEntry().Img() }

// Variants returns the portraits (pose variants) for state that have an image.
// If there are none, it falls back to the default portrait.
func (c *Character) Variants(state string) []*ImageEntry {
	return c.variantsWith(state, (*ImageEntry).HasImage)
}

// variantsWith is Variants with the image check passed in, so it can be tested without files.
func (c *Character) variantsWith(state string, has func(*ImageEntry) bool) []*ImageEntry {
	var out []*ImageEntry
	for i := range c.Expressions {
		e := &c.Expressions[i]
		if (e.State == state || (e.State == "" && e.ID == state)) && has(e) {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		if fb, ok := exprFallback[state]; ok {
			return c.variantsWith(fb, has)
		}
		out = append(out, &c.Expressions[0])
	}
	return out
}

// Expression returns the portrait id, or the first (normal) one if she has no such portrait.
func (c *Character) Expression(id string) *ImageEntry {
	for i := range c.Expressions {
		if c.Expressions[i].ID == id {
			return &c.Expressions[i]
		}
	}
	return &c.Expressions[0]
}

// MainCGCount is how many illustrations a regular game unlocks (one a course for every
// course but the last). A character's illustrations past these are the extras, which
// only the hidden command brings out.
const MainCGCount = 15

// MainCGs are the illustrations of the regular game.
func (c *Character) MainCGs() []ImageEntry { return c.CGs[:min(MainCGCount, len(c.CGs))] }

// ExtraCGs are the illustrations of the extra stages (hidden until the command is entered).
func (c *Character) ExtraCGs() []ImageEntry { return c.CGs[min(MainCGCount, len(c.CGs)):] }

// charactersDir is where the characters are in the assets directory.
const charactersDir = "characters"

// Read parses every characters/*/game.json manifest in fsys (the assets directory).
// Characters come back sorted by Order and each character's CGs by their Order. Images are
// not loaded; they load lazily on use, from fsys.
func Read(fsys fs.FS) ([]*Character, error) {
	dirs, err := fs.ReadDir(fsys, charactersDir)
	if err != nil {
		return nil, err
	}
	var chars []*Character
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		base := path.Join(charactersDir, d.Name())
		raw, err := fs.ReadFile(fsys, path.Join(base, "game.json"))
		if err != nil {
			continue
		}
		c := &Character{}
		if err := json.Unmarshal(raw, c); err != nil {
			return nil, fmt.Errorf("%s: %w", base, err)
		}
		// a character still being made (no standing picture yet) stays out of the game
		if !assets.HasImage(fsys, path.Join(base, "images", ExprNormal)) {
			continue
		}
		c.Cutin = &ImageEntry{ID: "cutin", State: ExprExcited}
		c.Ending = &ImageEntry{ID: "ending", State: ExprPerfect}
		c.EndingExtra = &ImageEntry{ID: "ending_extra", State: ExprPerfect}
		for _, e := range c.entries() {
			e.fsys, e.base = fsys, base // images load on use (Img)
		}
		sort.SliceStable(c.CGs, func(i, j int) bool { return c.CGs[i].Order < c.CGs[j].Order })
		chars = append(chars, c)
	}
	sort.SliceStable(chars, func(i, j int) bool { return chars[i].Order < chars[j].Order })
	return chars, nil
}

// entries are every picture of c.
func (c *Character) entries() []*ImageEntry {
	out := make([]*ImageEntry, 0, len(c.Expressions)+len(c.CGs)+5)
	for i := range c.Expressions {
		out = append(out, &c.Expressions[i])
	}
	for _, e := range []*ImageEntry{c.Group, c.Select, c.Cutin, c.Ending, c.EndingExtra} {
		if e != nil {
			out = append(out, e)
		}
	}
	for i := range c.CGs {
		out = append(out, &c.CGs[i])
	}
	return out
}
