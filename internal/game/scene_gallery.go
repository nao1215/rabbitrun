package game

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/gfx"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// galleryScene is the screen for browsing portraits and illustrations, reached from the title.
// Small character cards along the top switch characters (LB/RB, Q/E); the grid below lists portraits then illustrations.
// Only portraits seen during play and illustrations earned in play can be viewed.
type galleryScene struct {
	charIdx int
	sel     int
	viewing bool
	frame   int

	scroll, scrollView float64 // vertical grid scroll (target and displayed value)
	pull               []float64

	// list and open are the current character's entries and whether each can be viewed.
	// They are worked out once per character (listFor), not every frame.
	list     []galleryItem
	open     []bool
	listChar int

	// Pictures for tiles are decoded on worker goroutines (decoding a PNG is the slow part)
	// and arrive on decoded; the tile itself is made on the main goroutine. quit is closed
	// when the gallery is left: the decodes not started yet are given up.
	loading map[*character.ImageEntry]bool
	decoded chan decodedPicture
	quit    chan struct{}

	// The enlarged view never decodes on the main goroutine: the picture shown and those
	// beside it are decoded ahead in the background (held), and until the one chosen is in,
	// the one shown before stays (shown, of the entry shownOf).
	held              []*character.ImageEntry // illustrations decoded ahead at full size
	loaded            []*character.ImageEntry // portraits the enlarged view loaded (freed when it closes)
	shown             *ebiten.Image
	shownOf           galleryItem
	dwell             int // frames the grid selection has stayed on its tile (warmSel of warmChar)
	warmSel, warmChar int
}

// decodedPicture is a tile picture decoded off the main goroutine, a portrait's figure
// measured there too.
type decodedPicture struct {
	it  galleryItem
	img image.Image
	fig character.Figure
}

const (
	tileW, tileGap       = 160.0, 12.0
	gridTop              = 170.0
	gridBottom           = ScreenH - 12.0
	galleryCols          = 4
	galleryRows          = 3 // rows that fit the grid exactly, so no row is ever cut off
	tileH                = (gridBottom - gridTop - tileGap*(galleryRows-1)) / galleryRows
	headCardW, headCardH = 118.0, 118.0 // character cards at the top: a face close-up each
)

// galleryItem is one grid entry, either a portrait or an illustration.
type galleryItem struct {
	e  *character.ImageEntry
	cg bool
}

// newGalleryScene opens on the first character (the silver-haired one, leftmost).
func newGalleryScene() *galleryScene { return &galleryScene{charIdx: 0} }

func (s *galleryScene) char() *character.Character { return characters[s.charIdx] }

// items returns the portraits followed by the illustrations.
func (s *galleryScene) items() []galleryItem {
	s.listFor()
	return s.list
}

// listFor builds the entry list and their unlocked flags when the character changes.
func (s *galleryScene) listFor() {
	if s.list != nil && s.listChar == s.charIdx {
		return
	}
	c := s.char()
	cgs := galleryCGs(c)
	s.list = make([]galleryItem, 0, len(c.Expressions)+len(cgs))
	// only entries with a picture: an unlocked illustration not drawn yet showed as a
	// locked tile among the unlocked ones
	for i := range c.Expressions {
		if c.Expressions[i].HasImage() {
			s.list = append(s.list, galleryItem{e: &c.Expressions[i]})
		}
	}
	for i := range cgs {
		if cgs[i].HasImage() {
			s.list = append(s.list, galleryItem{e: &cgs[i], cg: true})
		}
	}
	s.open = make([]bool, len(s.list))
	for i, it := range s.list {
		s.open[i] = s.unlocked(it)
	}
	s.listChar = s.charIdx
}

func (s *galleryScene) unlocked(it galleryItem) bool {
	if debugMode {
		return it.e.HasImage()
	}
	p := progress(s.char().ID)
	if it.cg {
		return p.UnlockedCG[it.e.ID] && it.e.HasImage()
	}
	return p.SeenExpr[it.e.ID] && it.e.HasImage()
}

func (s *galleryScene) Update(g *Game) {
	s.frame++
	if sound.CurrentSong() != sound.GallerySong {
		sound.StartBGM(sound.GallerySong)
	}
	sound.SetBGMState(0) // calm: the pictures are looked at slowly
	bg.set(popCream)
	bg.setImage("gallery")
	items := s.items()
	n := len(items)
	if s.viewing {
		// step to the previous or next unlocked entry
		step := 0
		if g.in.Repeat(input.Left) {
			step = -1
		}
		if g.in.Repeat(input.Right) {
			step = 1
		}
		if step != 0 {
			if j := s.nextOpen(s.sel, step); j != s.sel {
				s.sel = j
				s.prepareView()
				sound.Play(sound.Move)
			}
		}
		if g.in.Pressed(input.Cancel) || g.in.Pressed(input.Confirm) {
			s.closeView()
			sound.Play(sound.Cancel)
		}
		return
	}
	// switch characters
	if d := boolInt(g.in.Pressed(input.TabNext)) - boolInt(g.in.Pressed(input.TabPrev)); d != 0 {
		// Skip characters that are still locked.
		n := len(characters)
		for range n {
			s.charIdx = (s.charIdx + d + n) % n
			if !locked(characters[s.charIdx]) {
				break
			}
		}
		s.sel, s.scroll, s.scrollView = 0, 0, 0
		s.hold(nil)
		sound.Play(sound.Move)
		return
	}
	old := s.sel
	if g.in.Repeat(input.Left) && s.sel > 0 {
		s.sel--
	}
	if g.in.Repeat(input.Right) && s.sel < n-1 {
		s.sel++
	}
	if g.in.Repeat(input.Up) && s.sel >= galleryCols {
		s.sel -= galleryCols
	}
	if g.in.Repeat(input.Down) && s.sel+galleryCols < n {
		s.sel += galleryCols
	}
	if s.sel != old {
		sound.Play(sound.Move)
	}
	// an illustration the selection rests on is decoded ahead, so it opens at once
	if s.sel != s.warmSel || s.listChar != s.warmChar {
		s.warmSel, s.warmChar, s.dwell = s.sel, s.listChar, 0
		s.hold(nil)
	}
	if s.dwell++; s.dwell == warmFrames && s.open[s.sel] && items[s.sel].cg {
		s.hold([]*character.ImageEntry{items[s.sel].e})
	}
	// Scroll by whole rows so the selected row is on screen and no row is cut off.
	row := s.sel / galleryCols
	first := int(s.scroll/(tileH+tileGap) + 0.5)
	if row < first {
		first = row
	}
	if row >= first+galleryRows {
		first = row - galleryRows + 1
	}
	s.scroll = float64(first) * (tileH + tileGap)
	s.scrollView += (s.scroll - s.scrollView) * 0.25
	if g.in.Pressed(input.Confirm) {
		if s.open[s.sel] {
			s.viewing = true
			s.prepareView()
			sound.Play(sound.Confirm)
		} else {
			sound.Play(sound.Denied)
		}
	}
	if g.in.Pressed(input.Cancel) {
		sound.Play(sound.Cancel)
		g.SetScene(newTitleScene())
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// warmFrames is how long the grid selection rests on an illustration before it is decoded
// ahead (moving across the grid decodes nothing).
const warmFrames = 10

// nextOpen is the open entry step entries (one way or the other) from i, going around;
// i itself when no other is open.
func (s *galleryScene) nextOpen(i, step int) int {
	n := len(s.list)
	for k := 1; k < n; k++ {
		if j := ((i+step*k)%n + n) % n; s.open[j] {
			return j
		}
	}
	return i
}

// prepareView decodes ahead the picture chosen in the enlarged view and the open ones on
// either side (where a step goes next), and frees those further away: three are held at
// most, however many are looked through.
func (s *galleryScene) prepareView() {
	near := []int{s.sel, s.nextOpen(s.sel, 1), s.nextOpen(s.sel, -1)}
	var cgs, portraits []*character.ImageEntry
	for _, i := range near {
		if it := s.list[i]; it.cg {
			cgs = append(cgs, it.e)
		} else {
			portraits = append(portraits, it.e)
		}
	}
	s.hold(cgs)
	keep := s.loaded[:0]
	for _, e := range s.loaded {
		if slices.Contains(portraits, e) {
			keep = append(keep, e)
		} else {
			s.forgetShown(e)
			e.ReleaseImg()
		}
	}
	s.loaded = keep
	for _, e := range portraits {
		if e.Image == nil && !slices.Contains(s.loaded, e) && !slices.Contains(selectEntries(), e) {
			s.loaded = append(s.loaded, e) // loaded for the view: freed with it (the title keeps the select cards)
		}
	}
	character.PrefetchImgs(portraits)
}

// hold keeps the illustrations es decoded at full size (decoding those not yet) and frees
// the others held before.
func (s *galleryScene) hold(es []*character.ImageEntry) {
	for _, e := range s.held {
		if !slices.Contains(es, e) {
			s.forgetShown(e)
			e.ReleaseFull()
		}
	}
	s.held = append(s.held[:0], es...)
	for _, e := range es {
		e.PrefetchFull()
	}
}

// forgetShown stops showing the picture of e, which is being freed.
func (s *galleryScene) forgetShown(e *character.ImageEntry) {
	if s.shownOf.e == e {
		s.shown, s.shownOf = nil, galleryItem{}
	}
}

// closeView closes the enlarged view and frees what it held.
func (s *galleryScene) closeView() {
	s.viewing = false
	s.hold(nil)
	for _, e := range s.loaded {
		e.ReleaseImg()
	}
	s.loaded = nil
	s.shown, s.shownOf = nil, galleryItem{}
}

// release frees what the gallery holds when it is left (Game.SetScene) and gives up the
// tile pictures not decoded yet.
func (s *galleryScene) release() {
	s.closeView()
	if s.quit != nil {
		close(s.quit)
		s.quit = nil
	}
}

func (s *galleryScene) Draw(screen *ebiten.Image) {
	screen.DrawImage(galleryBackground(), nil)
	items := s.items()
	if s.viewing {
		dimScreen(screen, 0xf0)
		s.drawView(screen, items[s.sel])
		return
	}

	s.finishTiles()
	x0 := (ScreenW - (tileW*galleryCols + tileGap*(galleryCols-1))) / 2
	for i, it := range items {
		col, row := i%galleryCols, i/galleryCols
		x := x0 + float64(col)*(tileW+tileGap)
		y := gridTop + float64(row)*(tileH+tileGap) - s.scrollView
		if y+tileH < gridTop-tileH || y > ScreenH {
			continue
		}
		// Each tile (shadow, panel, picture clipped to the rounded corners and the border) is
		// made once and kept, so a frame only copies finished tiles.
		var tile *ebiten.Image
		if s.open[i] {
			tile = galleryTiles[it.e]
			if tile == nil {
				s.requestPicture(it)
			}
		} else {
			tile = lockedTile(i)
		}
		if tile == nil {
			tile = emptyTile()
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(x, y)
		screen.DrawImage(tile, op)
		if i == s.sel {
			gfx.StrokeRoundRect(screen, float32(x)-4, float32(y)-4, tileW+8, tileH+8, 18, 4, gfx.CandyPink)
		}
	}

	// Top strip: character cards, with the selected one pulled out slightly. The strip
	// shows the candy background of the character frame, with a soft line at its bottom edge.
	// The header shows the same packed gummies as the background, a little brighter.
	if sub, ok := galleryBackground().SubImage(image.Rect(0, 0, ScreenW, int(gridTop-10))).(*ebiten.Image); ok {
		screen.DrawImage(sub, nil)
	}
	vector.FillRect(screen, 0, 0, ScreenW, gridTop-10, color.NRGBA{0xff, 0xff, 0xff, 0x30}, false)
	vector.FillRect(screen, 0, gridTop-14, ScreenW, 4, color.NRGBA{0xd9, 0x7a, 0x9c, 0x60}, false)
	s.pull = drawFaceStrip(screen, s.charIdx, s.pull, 12, 1)
}

// drawView draws the entry it enlarged. While its picture is still being decoded, the
// one shown before stays (on the first, only the dimmed screen shows for those frames).
func (s *galleryScene) drawView(screen *ebiten.Image, it galleryItem) {
	var img *ebiten.Image
	if it.cg {
		img = it.e.FullReady()
	} else {
		img = it.e.ImgReady()
	}
	if img != nil {
		s.shown, s.shownOf = img, it
	} else {
		img, it = s.shown, s.shownOf
	}
	switch {
	case img == nil:
	case it.cg:
		gfx.DrawImageFit(screen, img, 0, 0, ScreenW, ScreenH, 1) // whole, with bands at the sides
	default:
		if bgImg := assets.UI("frame_" + family(it.e.State)); bgImg != nil {
			gfx.DrawImageCover(screen, bgImg, 0, 0, ScreenW, ScreenH, 0.9)
		}
		// the same size and place as in the play screen's frame: every standing pose
		// equally tall, a crouching one not tiny, nothing cut off
		drawPortrait(screen, img, ScreenW, ScreenH, 1, 1, 0, 0, 1, 0, 0)
	}
}

// requestPicture starts decoding e's picture on a worker goroutine, once.
func (s *galleryScene) requestPicture(it galleryItem) {
	e := it.e
	if s.loading[e] {
		return
	}
	if s.loading == nil {
		s.loading = map[*character.ImageEntry]bool{}
		s.decoded = make(chan decodedPicture, 64)
		s.quit = make(chan struct{})
	}
	s.loading[e] = true
	// A portrait already loaded for play (with its figure measured) makes its tile at once,
	// without decoding the file again.
	if img := e.Image; !it.cg && img != nil {
		if f, ok := character.KnownFigure(img); ok {
			galleryTiles[e] = makeTile(func(l *ebiten.Image) {
				drawTileBackdrop(l, it)
				drawPortraitFigure(l, img, f, float64(l.Bounds().Dx()), float64(l.Bounds().Dy()), 1, 1, 0, 0, 1, 0, 0)
			})
			return
		}
	}
	go func(out chan<- decodedPicture, quit <-chan struct{}) {
		// Twice the tile height, so the tile is drawn from a sharper picture.
		d := decodedPicture{it: it, img: e.DecodeScaled(2*tileInnerH, quit)}
		if d.img != nil && !it.cg {
			d.fig = character.MeasureFigure(d.img) // here, not on the main goroutine
		}
		select {
		case out <- d:
		case <-quit: // the gallery was left: nobody makes the tile
		}
	}(s.decoded, s.quit)
}

// finishTiles makes the tiles of pictures that finished decoding. Uploading to the GPU is
// quick, but it is capped per frame so a frame never stalls.
func (s *galleryScene) finishTiles() {
	for range tilesPerFrame {
		select {
		case d := <-s.decoded:
			if e := d.it.e; galleryTiles[e] == nil {
				galleryTiles[e] = makeTile(func(l *ebiten.Image) { drawTilePicture(l, d.it, d.img, d.fig) })
			}
		default:
			return
		}
	}
}

// tilesPerFrame caps the tiles made in a frame: a dozen in view at once each took a few
// ms to make and upload, and the frame took them all.
const tilesPerFrame = 3

// galleryTiles are the finished tiles of the pictures, made once and kept (the entries
// never move).
var galleryTiles = map[*character.ImageEntry]*ebiten.Image{}

// tileInnerW and tileInnerH are the size of a tile's picture inside its border.
var tileInnerW, tileInnerH = int(tileW - 8), int(math.Floor(tileH - 8))

// makeTile returns a finished gallery tile: the panel with its shadow, the picture drawn by
// paint clipped to rounded corners, and the border on top (so no square corner pokes out).
func makeTile(paint func(l *ebiten.Image)) *ebiten.Image {
	t := ebiten.NewImage(int(tileW)+8, int(math.Ceil(tileH))+10)
	drawPanel(t, 0, 0, tileW, tileH)
	if paint != nil {
		l := ebiten.NewImage(tileInnerW, tileInnerH)
		paint(l)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(4, 4)
		t.DrawImage(gfx.RoundedMask(l, 14), op)
		l.Deallocate()
	}
	gfx.StrokeRoundRect(t, 3, 3, tileW-6, tileH-6, 15, 3, gfx.PanelFill)
	return t
}

// drawTilePicture paints an unlocked entry's picture into the tile layer l: an
// illustration as it is, a portrait over the backdrop of its situation.
func drawTilePicture(l *ebiten.Image, it galleryItem, pic image.Image, fig character.Figure) {
	e := it.e
	lw, lh := float64(l.Bounds().Dx()), float64(l.Bounds().Dy())
	drawTileBackdrop(l, it)
	if pic == nil || it.cg {
		var img *ebiten.Image
		if pic != nil {
			img = ebiten.NewImageFromImage(pic)
		} else {
			img = character.Placeholder(e.ID) // made only when it is needed: it is a large image
		}
		// an illustration fills its tile (fitted, it left bands at the top and bottom)
		gfx.DrawImageCoverTop(l, img, 0, 0, lw, lh, 1)
		img.Deallocate()
		return
	}
	// a portrait is drawn as in the play screen's frame (see drawPortrait)
	img := ebiten.NewImageFromImage(pic)
	drawPortraitFigure(l, img, fig, lw, lh, 1, 1, 0, 0, 1, 0, 0)
	img.Deallocate()
}

// drawTileBackdrop paints the backdrop of a portrait's situation into the tile layer l
// (an illustration has none).
func drawTileBackdrop(l *ebiten.Image, it galleryItem) {
	if it.cg {
		return
	}
	if bgImg := assets.UI("frame_" + family(it.e.State)); bgImg != nil {
		gfx.DrawImageCover(l, bgImg, 0, 0, float64(l.Bounds().Dx()), float64(l.Bounds().Dy()), 1)
	}
}

// lockedTiles are the tiles of locked entries: gummies packed edge to edge under a soft
// white veil. A few layouts take turns so neighboring tiles differ.
var (
	lockedTiles [road.Courses]*ebiten.Image
	blankTile   *ebiten.Image
)

// lockedTile returns the finished tile for the locked entry number i.
func lockedTile(i int) *ebiten.Image {
	k := i % len(lockedTiles)
	if lockedTiles[k] == nil {
		lockedTiles[k] = makeTile(func(l *ebiten.Image) {
			// A stretch of a course: the road of the game, walled with the gummy blocks of
			// one of the course colors (the tiles take turns through the colors).
			iw, ih := tileInnerW, tileInnerH
			cellSize := float64(iw) / road.W
			rows := int(math.Ceil(float64(ih)/cellSize)) + 1
			g := road.New(uint64(k) + 11) //nolint:gosec // G115: k is a small index
			for range 40 {                // let the road wander a little first
				g.Safe = 1
				g.Step()
			}
			wall := Kind(road.CourseColors[k%road.Courses])
			oy := (float64(ih) - float64(rows)*cellSize) / 2
			drawGummyGrid(l, road.W, rows, func(x, y int) gummyCell {
				if y >= road.Rows || g.Rows[y][x].Wall == 0 {
					return gummyCell{}
				}
				return gummyCell{kind: wall, id: int32(y*road.W + x + 1), alpha: 1} //nolint:gosec // G115: a few hundred cells
			}, 0, oy, cellSize)
			vector.FillRect(l, 0, 0, float32(iw), float32(ih), color.NRGBA{0xff, 0xff, 0xff, 0x90}, false)
		})
	}
	return lockedTiles[k]
}

// emptyTile is the plain tile shown while an unlocked entry's tile is still being made.
func emptyTile() *ebiten.Image {
	if blankTile == nil {
		blankTile = makeTile(nil)
	}
	return blankTile
}

// drawSilhouette draws img centered in the box as a soft silhouette (only its outline
// shows): covering the box, or (cover false) at the largest size that fits it.
func drawSilhouette(dst, img *ebiten.Image, x, y, w, h float64, cover bool) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Min(w/iw, h/ih)
	if cover {
		s = math.Max(w/iw, h/ih)
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x+(w-iw*s)/2, y+(h-ih*s)/2)
	silhouette(dst, img, op)
}

// drawFaceStrip draws every character's face in a row of cards across the top of the
// screen (at y, scaled by size), the chosen one (sel) raised and ringed in pink, a locked
// character as a black silhouette. pull eases the raise; it is returned for the next frame.
func drawFaceStrip(screen *ebiten.Image, sel int, pull []float64, top, size float64) []float64 {
	n := len(characters)
	if len(pull) != n {
		pull = make([]float64, n)
	}
	w, h := headCardW*size, headCardH*size
	for i, c := range characters {
		t := 0.0
		if i == sel {
			t = 1
		}
		pull[i] += (t - pull[i]) * 0.2
		x := ScreenW/2 + (float64(i)-float64(n-1)/2)*(w+24*size) - w/2
		y := top + 8*size*(1-pull[i])
		gfx.FillRoundRect(screen, float32(x)+4, float32(y)+6, float32(w), float32(h), 12, shadowColor())
		gfx.FillRoundRect(screen, float32(x), float32(y), float32(w), float32(h), 12, gfx.PanelFill)
		alpha := float32(0.55 + 0.45*pull[i])
		face := character.FaceOf(c.SelectEntry())
		switch {
		case locked(c):
			// A locked character shows only the black silhouette of her face.
			gfx.FillRoundRect(screen, float32(x)+4, float32(y)+4, float32(w)-8, float32(h)-8, 10, lockedCardFill)
			if face != nil {
				drawSilhouette(screen, face, x+4, y+4, w-8, h-8, true)
			}
		case face != nil:
			gfx.DrawImageCover(screen, face, x+4, y+4, w-8, h-8, alpha)
		default:
			gfx.DrawImageFit(screen, c.SelectImage(), x+4, y+4, w-8, h-8, alpha)
		}
		if i == sel {
			gfx.StrokeRoundRect(screen, float32(x)-3, float32(y)-3, float32(w)+6, float32(h)+6, 14, 4, gfx.CandyPink)
		}
	}
	return pull
}

// silhouetteColor is the color a locked character is shown in: a soft lilac grey, see-through.
var silhouetteColor = color.NRGBA{0x8c, 0x80, 0xa0, 0xb0}

// silhouette draws img with op as a flat silhouette in silhouetteColor.
func silhouette(dst, img *ebiten.Image, op *ebiten.DrawImageOptions) {
	var cm colorm.ColorM
	cm.Scale(0, 0, 0, float64(silhouetteColor.A)/0xff)
	cm.Translate(float64(silhouetteColor.R)/0xff, float64(silhouetteColor.G)/0xff, float64(silhouetteColor.B)/0xff, 0)
	colorm.DrawImage(dst, img, cm, &colorm.DrawImageOptions{GeoM: op.GeoM, Filter: op.Filter})
}

// galleryArtwork is the artwork of the gallery: its background and the frames behind the
// portraits (one a family of expressions).
var galleryArtwork = func() []string {
	names := make([]string, 0, 1+len(moodBackground))
	names = append(names, "gallery")
	for f := range moodBackground {
		names = append(names, "frame_"+f)
	}
	slices.Sort(names)
	return names
}()

var galleryBG *ebiten.Image

// galleryBackground is the gallery background, built once: the gallery artwork (little
// sweets on cream) covering the screen under a soft white veil, so the tiles stand out.
func galleryBackground() *ebiten.Image {
	if galleryBG != nil {
		return galleryBG
	}
	galleryBG = ebiten.NewImage(ScreenW, ScreenH)
	if art := assets.UI("gallery"); art != nil {
		gfx.DrawImageCover(galleryBG, art, 0, 0, ScreenW, ScreenH, 1)
	}
	vector.FillRect(galleryBG, 0, 0, ScreenW, ScreenH, color.NRGBA{0xff, 0xff, 0xff, 0x60}, false)
	return galleryBG
}
