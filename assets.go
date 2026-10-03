package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/draw"
)

// assetFS holds the game data. Only the public manifest (game.json) and the
// images are embedded; nothing else under assets/characters is shipped.
//
//go:embed assets/blocks assets/fonts assets/ui assets/characters/*/game.json all:assets/characters/*/images
var assetFS embed.FS

// Expression IDs switched by the board state. game.json lists an image under each ID in expressions.
const (
	ExprNormal   = "normal"   // default
	ExprRelaxed  = "relaxed"  // the road ahead is wide and easy
	ExprHappy    = "happy"    // picked up a sweet
	ExprGreat    = "great"    // picked up a macaron
	ExprExcited  = "excited"  // a bomb (the hammer) went off
	ExprTreat    = "treat"    // picked up the precious sweet (worth three)
	ExprCombo    = "combo"    // five sweets in a row
	ExprPerfect  = "perfect"  // a stage cleared
	ExprWorried  = "worried"  // the road is getting narrow
	ExprNervous  = "nervous"  // the road is narrow
	ExprPanic    = "panic"    // the road is at its narrowest
	ExprCrying   = "crying"   // she ran into a wall
	ExprGameOver = "gameover" // game over

	// Reactions to how the board is built (see PlayScene.readBoard).
	ExprOops    = "oops"    // the precious sweet got away
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

type ImageEntry struct {
	ID    string `json:"id"`
	State string `json:"state"` // state this portrait is for (a state can have several poses)
	Label string `json:"label"`
	Title string `json:"title"`
	// Order sorts the illustrations (the order they unlock in). The key is still "score",
	// from the old scored game where they unlocked at a score, so game.json files load.
	Order int `json:"score"` //nolint:tagliatelle // key used by the existing game.json files

	Image *ebiten.Image `json:"-"` // portraits load lazily via Img; illustrations load on demand via Full / Thumb

	base        string
	full        *ebiten.Image
	pending     chan decodedImg  // the portrait being decoded in the background (prefetchImgs)
	fullPending chan image.Image // the illustration being decoded in the background (PrefetchFull)
	tile        *ebiten.Image    // finished gallery tile (see GalleryScene.tileOf)
	has         int8             // HasImage cache: 0 unknown, 1 yes, -1 no (the assets are embedded and never change)
}

// HasImage reports whether the image file exists (otherwise a placeholder is used).
func (e *ImageEntry) HasImage() bool {
	if e.has == 0 {
		e.has = -1
		for _, ext := range imageExts {
			if _, err := fs.Stat(assetFS, path.Join(e.base, "images", e.ID+ext)); err == nil {
				e.has = 1
				break
			}
		}
	}
	return e.has > 0
}

// standingMaxH caps the height of loaded portraits. Keeping 156 of them at full size (640x1600) uses too much memory, so they are scaled down.
const standingMaxH = 800

// Img loads the portrait (expression or character-select image) scaled down on first use.
// If the picture is being decoded in the background (prefetchImgs), it waits for that
// rather than decoding it twice (unless it has not started yet).
func (e *ImageEntry) Img() *ebiten.Image {
	if e.Image == nil {
		if e.pending != nil && !unqueue(e, false) {
			e.upload(<-e.pending)
		} else {
			e.pending = nil
			e.upload(decodePortrait(e.base, e.ID))
		}
	}
	return e.Image
}

// ImgReady is Img without the wait: nil while the picture is still being decoded in the
// background.
func (e *ImageEntry) ImgReady() *ebiten.Image {
	if e.Image == nil && e.pending != nil {
		select {
		case d := <-e.pending:
			e.upload(d)
		default:
			unqueue(e, true) // wanted now: decoded next
			return nil
		}
	}
	return e.Img()
}

// name is what the placeholder of a missing picture says.
func (e *ImageEntry) name() string {
	if e.Label != "" {
		return e.Label
	}
	return e.ID
}

// upload hands a picture decoded in the background to the GPU, with its figure measured.
func (e *ImageEntry) upload(d decodedImg) {
	e.pending = nil
	if d.img == nil {
		e.Image = placeholderImage(e.name())
		return
	}
	e.Image = ebiten.NewImageFromImage(d.img)
	figureCache[e.Image] = d.fig
}

// ReleaseImg frees the loaded portrait on the GPU (and drops one being decoded); Img
// loads it again when it is wanted.
func (e *ImageEntry) ReleaseImg() {
	if e.pending != nil {
		unqueue(e, false)
		e.pending = nil
	}
	if e.Image != nil {
		delete(figureCache, e.Image)
		e.Image.Deallocate()
		e.Image = nil
	}
}

// decodedImg is a picture decoded off the main goroutine, with its figure measured there
// too (so it is not read back from the GPU).
type decodedImg struct {
	img image.Image
	fig figure
}

// decodePortrait decodes images/<id> as Img shows it (at most standingMaxH high) and
// measures its figure on the CPU, so drawPortrait need not read it back from the GPU. It
// may run on any goroutine.
func decodePortrait(base, id string) decodedImg {
	var d decodedImg
	if d.img = decodeScaled(base, id, standingMaxH); d.img != nil {
		d.fig = measureFigure(alphaPixels(d.img), d.img.Bounds().Dx(), d.img.Bounds().Dy())
	}
	return d
}

// prefetchImgs decodes the pictures of entries, as Img loads them, in the background: in
// the order given, a few at a time. Img then only uploads them (and uploadPrefetched does it
// a few a frame ahead of use). A picture already loaded or on its way is left alone.
func prefetchImgs(entries []*ImageEntry) {
	prefetchQueue.Lock()
	defer prefetchQueue.Unlock()
	for _, e := range entries {
		if e == nil || e.Image != nil || e.pending != nil || !e.HasImage() {
			continue
		}
		ch := make(chan decodedImg, 1)
		e.pending = ch
		prefetchQueue.jobs = append(prefetchQueue.jobs, prefetchJob{e, e.base, e.ID, ch})
	}
	for ; prefetchQueue.workers < min(len(prefetchQueue.jobs), cap(tileDecoders)); prefetchQueue.workers++ {
		go prefetchWorker()
	}
}

// prefetchQueue holds the pictures waiting to be decoded in the background, and how many
// workers are decoding them.
var prefetchQueue struct {
	sync.Mutex
	jobs    []prefetchJob
	workers int
}

// prefetchJob is a picture to decode. The worker reads only base and id; e only tells
// the jobs apart.
type prefetchJob struct {
	e        *ImageEntry
	base, id string
	out      chan<- decodedImg
}

// prefetchWorker decodes queued pictures until the queue is empty.
func prefetchWorker() {
	for {
		prefetchQueue.Lock()
		if len(prefetchQueue.jobs) == 0 {
			prefetchQueue.workers--
			prefetchQueue.Unlock()
			return
		}
		j := prefetchQueue.jobs[0]
		prefetchQueue.jobs = prefetchQueue.jobs[1:]
		prefetchQueue.Unlock()
		tileDecoders <- struct{}{}
		d := decodePortrait(j.base, j.id)
		<-tileDecoders
		j.out <- d
	}
}

// unqueue takes e's picture out of the queue (keep: put it first instead, as it is wanted
// now). It reports whether it was still waiting there.
func unqueue(e *ImageEntry, keep bool) bool {
	prefetchQueue.Lock()
	defer prefetchQueue.Unlock()
	q := prefetchQueue.jobs
	for i, j := range q {
		if j.e == e {
			copy(q[1:i+1], q[:i]) // the jobs before it move back one
			if !keep {
				prefetchQueue.jobs = q[1:]
				return true
			}
			q[0] = j
			return true
		}
	}
	return false
}

// uploadPrefetched uploads up to n of the entries' pictures that finished decoding
// (uploading is quick, but capped so a frame never stalls).
func uploadPrefetched(entries []*ImageEntry, n int) {
	for _, e := range entries {
		if n == 0 {
			return
		}
		if e == nil || e.Image != nil || e.pending == nil {
			continue
		}
		select {
		case d := <-e.pending:
			e.upload(d)
			n--
		default:
		}
	}
}

// Full loads the illustration at full size (for the gallery's enlarged view). Free it with
// ReleaseFull. If PrefetchFull started decoding it, it only waits for that and uploads it.
func (e *ImageEntry) Full() *ebiten.Image {
	if e.Image != nil {
		return e.Image
	}
	if e.full == nil {
		if e.fullPending != nil {
			img := <-e.fullPending
			e.fullPending = nil
			if img != nil {
				e.full = ebiten.NewImageFromImage(img)
			} else {
				e.full = placeholderImage(e.Title)
			}
		} else {
			e.full = loadCharImage(e.base, e.ID, e.Title)
		}
	}
	return e.full
}

// PrefetchFull starts decoding the full-size illustration in the background, so the frame
// that first shows it (Full) only uploads it: decoding a large JPEG on the main goroutine
// dropped frames whenever the road changed its picture.
func (e *ImageEntry) PrefetchFull() {
	if e.Image != nil || e.full != nil || e.fullPending != nil {
		return
	}
	ch := make(chan image.Image, 1)
	e.fullPending = ch
	base, id := e.base, e.ID
	go func() { ch <- decodeCharImage(base, id) }()
}

func (e *ImageEntry) ReleaseFull() {
	e.fullPending = nil
	if e.full != nil {
		e.full.Deallocate()
		e.full = nil
	}
}

type Character struct {
	ID          string       `json:"id"`     // never shown on screen (characters are unnamed)
	Order       int          `json:"order"`  // order on the character select screen
	Secret      bool         `json:"secret"` // the secret character, locked until secretGoal is reached
	Expressions []ImageEntry `json:"expressions"`
	Group       *ImageEntry  `json:"group"`  // pose for the group picture of the secret title command
	Select      *ImageEntry  `json:"select"` // full-body image for character select (modest outfit)
	Cutin       *ImageEntry  `json:"-"`      // the big cut-in when a star candy goes off (images/cutin.png)
	// Ending and EndingExtra are the pictures of the all clear, of the regular and of the
	// extra stages (images/ending, images/ending_extra): the shape of the window, to fill it.
	Ending      *ImageEntry  `json:"-"`
	EndingExtra *ImageEntry  `json:"-"`
	CGs         []ImageEntry `json:"cgs"` //nolint:tagliatelle // key used by the existing game.json files
}

// selectEntry returns the portrait used for character select: the select image, or the normal expression.
func (c *Character) selectEntry() *ImageEntry {
	if c.Select != nil && c.Select.HasImage() {
		return c.Select
	}
	return c.Expression(ExprNormal)
}

// SelectImage returns the character select image, or the default expression if none.
func (c *Character) SelectImage() *ebiten.Image {
	if c.Select != nil && c.Select.HasImage() {
		return c.Select.Img()
	}
	return c.Expression(ExprNormal).Img()
}

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

func (c *Character) Expression(id string) *ImageEntry {
	for i := range c.Expressions {
		if c.Expressions[i].ID == id {
			return &c.Expressions[i]
		}
	}
	return &c.Expressions[0]
}

var (
	characters []*Character
	fontSource *text.GoTextFaceSource // M+ 1p, which also covers Japanese
	popSource  *text.GoTextFaceSource // Lilita One, a pop typeface for Latin text (OFL, assets/fonts/)
)

func loadAssets() {
	var err error
	mplus, err := assetFS.ReadFile("assets/fonts/mplus-1p-regular.ttf")
	if err != nil {
		log.Fatal(err)
	}
	fontSource, err = text.NewGoTextFaceSource(bytes.NewReader(mplus))
	if err != nil {
		log.Fatal(err)
	}
	if raw, err := assetFS.ReadFile("assets/fonts/LilitaOne-Regular.ttf"); err == nil {
		if src, err := text.NewGoTextFaceSource(bytes.NewReader(raw)); err == nil {
			popSource = src
		} else {
			log.Printf("cannot load the pop font, using the default font: %v", err)
		}
	}
	characters, err = readCharacters(assetFS)
	if err != nil {
		log.Fatal(err)
	}
	if len(characters) == 0 {
		log.Fatal("no characters found")
	}
}

// readCharacters parses every assets/characters/*/game.json manifest in fsys.
// hasNormalImage reports whether the character in base has its standing picture
// (images/normal.png or .jpg), the one every screen falls back on.
func hasNormalImage(fsys fs.FS, base string) bool {
	for _, ext := range imageExts {
		if _, err := fs.Stat(fsys, path.Join(base, "images", "normal"+ext)); err == nil {
			return true
		}
	}
	return false
}

// Characters come back sorted by Order and each character's CGs by their Order.
// Images are not loaded; they load lazily on use.
func readCharacters(fsys fs.FS) ([]*Character, error) {
	dirs, err := fs.ReadDir(fsys, "assets/characters")
	if err != nil {
		return nil, err
	}
	var chars []*Character
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		base := path.Join("assets/characters", d.Name())
		raw, err := fs.ReadFile(fsys, path.Join(base, "game.json"))
		if err != nil {
			continue
		}
		c := &Character{}
		if err := json.Unmarshal(raw, c); err != nil {
			return nil, fmt.Errorf("%s: %w", base, err)
		}
		// a character still being made (no standing picture yet) stays out of the game
		if !hasNormalImage(fsys, base) {
			continue
		}
		for i := range c.Expressions {
			e := &c.Expressions[i]
			e.base = base // images load on use (Img)
		}
		if c.Group != nil {
			c.Group.base = base
		}
		if c.Select != nil {
			c.Select.base = base
		}
		c.Cutin = &ImageEntry{ID: "cutin", State: ExprExcited, base: base}
		c.Ending = &ImageEntry{ID: "ending", State: ExprPerfect, base: base}
		c.EndingExtra = &ImageEntry{ID: "ending_extra", State: ExprPerfect, base: base}
		for i := range c.CGs {
			e := &c.CGs[i]
			e.base = base
		}
		sort.SliceStable(c.CGs, func(i, j int) bool { return c.CGs[i].Order < c.CGs[j].Order })
		chars = append(chars, c)
	}
	sort.SliceStable(chars, func(i, j int) bool { return chars[i].Order < chars[j].Order })
	return chars, nil
}

// decodeScaled reads images/<id> and scales it down to at most maxH pixels high. It only
// touches the CPU, so it may run on any goroutine. It returns nil if the image is missing.
func decodeScaled(base, id string, maxH int) image.Image {
	for _, ext := range imageExts {
		img, err := decodeAsset(path.Join(base, "images", id+ext))
		if err != nil {
			continue
		}
		b := img.Bounds()
		if b.Dy() > maxH {
			w := b.Dx() * maxH / b.Dy()
			dst := image.NewRGBA(image.Rect(0, 0, w, maxH))
			draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
			img = dst
		}
		return img
	}
	return nil
}

// loadCharImage reads images/<id>.jpg (or .png). If missing, it returns a placeholder.
func loadCharImage(base, id, label string) *ebiten.Image {
	if img := decodeCharImage(base, id); img != nil {
		return ebiten.NewImageFromImage(img)
	}
	return placeholderImage(label)
}

// decodeCharImage decodes images/<id>.jpg (or .png) at full size, or returns nil if it is
// missing. It only touches the CPU, so it may run on any goroutine.
func decodeCharImage(base, id string) image.Image {
	for _, ext := range imageExts {
		img, err := decodeAsset(path.Join(base, "images", id+ext))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			log.Printf("%s%s: %v", id, ext, err)
			continue
		}
		return img
	}
	return nil
}

// decodeAsset decodes the embedded image name. It reads the embedded bytes in place:
// assetFS.ReadFile would first copy the whole file. A missing file gives an error
// wrapping fs.ErrNotExist.
func decodeAsset(name string) (image.Image, error) {
	f, err := assetFS.Open(name)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	return img, errors.Join(err, f.Close())
}

// placeholderImage is the stand-in (a human silhouette) used when an image is missing.
func placeholderImage(label string) *ebiten.Image {
	img := ebiten.NewImage(896, 1120)
	sil := color.NRGBA{0xe0, 0xa8, 0xb8, 0xd0}
	vector.FillCircle(img, 448, 300, 130, sil, true)
	fillRoundRect(img, 268, 450, 360, 670, 120, sil)
	drawText(img, label, 448, 700, 56, color.White)
	return img
}

type faceKey struct {
	size float64
	pop  bool
}

var faceCache = map[faceKey]*text.GoTextFace{}

// face returns a font face for the size. ASCII-only strings use the pop typeface (Lilita One).
func face(size float64, s string) *text.GoTextFace {
	pop := popSource != nil && isASCII(s)
	k := faceKey{size, pop}
	f, ok := faceCache[k]
	if !ok {
		src := fontSource
		if pop {
			src = popSource
		}
		f = &text.GoTextFace{Source: src, Size: size}
		faceCache[k] = f
	}
	return f
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 0x7e {
			return false
		}
	}
	return true
}

// drawText draws s horizontally centered on x.
func drawText(dst *ebiten.Image, s string, x, y, size float64, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = size * 1.4
	op.PrimaryAlign = text.AlignCenter
	text.Draw(dst, s, face(size, s), op)
}

// ---- Save data ----

type CharProgress struct {
	HighScore  int  `json:"high_score"`  // from the old scored game; kept so old saves load
	TotalScore int  `json:"total_score"` // from the old scored game; kept so old saves load
	BestStage  int  `json:"best_stage"`  // the furthest stage reached (1 for the first)
	Cleared    bool `json:"cleared"`     // the regular stages run to the end
	// ClearedExtra is set once the extra stages have been run to the end.
	ClearedExtra bool            `json:"cleared_extra"`
	UnlockedCG   map[string]bool `json:"unlocked_cg"`      // illustrations unlock one per stage cleared
	SeenExpr     map[string]bool `json:"seen_expressions"` //nolint:tagliatelle // key used by existing save files, which must keep loading
	// PlaySeconds is the total time played with the character (it counts toward the secret character).
	PlaySeconds int `json:"play_seconds"`
}

type SaveData struct {
	Characters map[string]*CharProgress `json:"characters"`
	// Announced lists the secret characters whose unlock has been announced on the select screen.
	Announced map[string]bool `json:"announced"`
	// ExtraFound is set once the hidden command has been entered: from then on the gallery
	// also shows the extra illustrations (until then it only knows the regular ones).
	ExtraFound bool `json:"extra_found"`
	// WordTold is set once the title has told the secret word (after the secret character
	// cleared the regular stages).
	WordTold bool `json:"word_told"`
	// ExtraMode plays the extra stages: the extra illustrations on a harder road. It is not
	// saved: every launch starts on the regular stages, and the secret word switches over.
	ExtraMode bool `json:"-"`
}

// MainCGCount is how many illustrations a regular game unlocks (one a course for the
// first GameCourses-1 courses). A character's illustrations past these are the extras,
// which only the hidden command brings out.
const MainCGCount = 15

// MainCGs are the illustrations of the regular game.
func (c *Character) MainCGs() []ImageEntry { return c.CGs[:min(MainCGCount, len(c.CGs))] }

// ExtraCGs are the illustrations of the extra stages (hidden until the command is entered).
func (c *Character) ExtraCGs() []ImageEntry { return c.CGs[min(MainCGCount, len(c.CGs)):] }

// PlayCGs are the illustrations the game in play unlocks: the extras in the extra mode.
func (c *Character) PlayCGs() []ImageEntry {
	if extraMode() {
		return c.ExtraCGs()
	}
	return c.MainCGs()
}

// GalleryCGs are the illustrations the gallery lists: only the regular ones until the
// hidden command has been found, so a full gallery looks complete.
func (c *Character) GalleryCGs() []ImageEntry {
	if save.ExtraFound {
		return c.CGs
	}
	return c.MainCGs()
}

// extraMode reports whether the extra stages are being played.
func extraMode() bool { return save.ExtraFound && save.ExtraMode }

var save = &SaveData{Characters: map[string]*CharProgress{}}

// saveDirName is the directory under the user config dir that holds the save data.
const saveDirName = "rabbitrun"

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return dir
}

func savePath() string {
	return filepath.Join(configDir(), saveDirName, "save.json")
}

// resetSave starts the save data over (the --reset-save option): the save file is moved
// aside to save.json.bak next to it.
func resetSave() error {
	p := savePath()
	if err := os.Rename(p, p+".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func loadSave() {
	raw, err := os.ReadFile(savePath())
	if err != nil {
		return
	}
	if err := decodeSave(save, raw); err != nil {
		log.Printf("cannot read save data: %v", err)
	}
}

// decodeSave unmarshals raw into dst and makes sure dst.Characters is usable
// even when raw is broken. On error, whatever was decoded before the error
// stays in dst.
func decodeSave(dst *SaveData, raw []byte) error {
	err := json.Unmarshal(raw, dst)
	if dst.Characters == nil {
		dst.Characters = map[string]*CharProgress{}
	}
	return err
}

func writeSave() {
	raw, err := json.MarshalIndent(save, "", "  ")
	if err != nil {
		return
	}
	p := savePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		log.Printf("failed to save: %v", err)
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("failed to save: %v", err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		log.Printf("failed to save: %v", err)
	}
}

func progress(id string) *CharProgress { return save.progress(id) }

// progress returns the progress for the character id, creating it (with the
// default portrait already seen) when the save data has none. A null entry in
// the save file is treated the same as a missing one.
func (s *SaveData) progress(id string) *CharProgress {
	p := s.Characters[id]
	if p == nil {
		p = &CharProgress{}
		s.Characters[id] = p
	}
	if p.UnlockedCG == nil {
		p.UnlockedCG = map[string]bool{}
	}
	if p.SeenExpr == nil {
		p.SeenExpr = map[string]bool{ExprNormal: true}
	}
	return p
}
