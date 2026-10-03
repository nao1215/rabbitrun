package main

import (
	"image"
	"image/color"
	"math"
	"runtime"

	"github.com/nao1215/rabbitrun/road"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// GalleryScene is the screen for browsing portraits and illustrations, reached from the title.
// Small character cards along the top switch characters (LB/RB, Q/E); the grid below lists portraits then illustrations.
// Only portraits seen during play and illustrations earned in play can be viewed.
type GalleryScene struct {
	charIdx int
	sel     int
	viewing bool
	frame   int
	// pan is how far down an illustration is scrolled while viewed (0: its top, 1: its
	// bottom): it fills the width of the window, so up and down move along it.
	pan float64

	scroll, scrollView float64 // vertical grid scroll (target and displayed value)
	pull               []float64

	// list and open are the current character's entries and whether each can be viewed.
	// They are worked out once per character (listFor), not every frame.
	list     []galleryItem
	open     []bool
	listChar int

	// Pictures for tiles are decoded on worker goroutines (decoding a PNG is the slow part)
	// and arrive on decoded; the tile itself is made on the main goroutine.
	loading map[*ImageEntry]bool
	decoded chan decodedPicture
}

// decodedPicture is a tile picture decoded off the main goroutine.
type decodedPicture struct {
	it  galleryItem
	img image.Image
}

// tileDecoders limits how many pictures are decoded at the same time. A few are enough:
// the tiles are uploaded only a few a frame anyway, and every decoder holds a portrait
// of several megabytes while it works (one per CPU spiked memory to hundreds of MB).
var tileDecoders = make(chan struct{}, min(3, max(1, runtime.NumCPU()-1)))

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
	e  *ImageEntry
	cg bool
}

// newGalleryScene opens on the first character (the silver-haired one, leftmost).
func newGalleryScene() *GalleryScene { return &GalleryScene{charIdx: 0} }

func (s *GalleryScene) char() *Character { return characters[s.charIdx] }

// items returns the portraits followed by the illustrations.
func (s *GalleryScene) items() []galleryItem {
	s.listFor()
	return s.list
}

// listFor builds the entry list and their unlocked flags when the character changes.
func (s *GalleryScene) listFor() {
	if s.list != nil && s.listChar == s.charIdx {
		return
	}
	c := s.char()
	cgs := c.GalleryCGs()
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

func (s *GalleryScene) unlocked(it galleryItem) bool {
	if *debugMode {
		return it.e.HasImage()
	}
	p := progress(s.char().ID)
	if it.cg {
		return p.UnlockedCG[it.e.ID] && it.e.HasImage()
	}
	return p.SeenExpr[it.e.ID] && it.e.HasImage()
}

func (s *GalleryScene) Update(g *Game) {
	s.frame++
	if bgmSong != gallerySong {
		startBGM(gallerySong)
	}
	setBGMState(0) // calm: the pictures are looked at slowly
	bg.set(popCream)
	bg.setImage("gallery")
	items := s.items()
	n := len(items)
	if s.viewing {
		// step to the previous or next unlocked entry
		step := 0
		if g.in.Repeat(ActLeft) {
			step = -1
		}
		if g.in.Repeat(ActRight) {
			step = 1
		}
		if items[s.sel].cg {
			const panSpeed = 0.02 // of the picture's spare height a frame
			if g.in.Held(ActUp) {
				s.pan = max(0, s.pan-panSpeed)
			}
			if g.in.Held(ActDown) {
				s.pan = min(1, s.pan+panSpeed)
			}
		}
		if step != 0 {
			for i := 1; i < n; i++ {
				j := (s.sel + step*i + n*n) % n
				if s.open[j] {
					s.releaseViewed(items)
					s.sel = j
					s.pan = 0
					playSE(seMove)
					break
				}
			}
		}
		if g.in.Pressed(ActCancel) || g.in.Pressed(ActConfirm) {
			s.releaseViewed(items)
			s.viewing = false
			playSE(seCancel)
		}
		return
	}
	// switch characters
	if d := boolInt(g.in.Pressed(ActTabNext)) - boolInt(g.in.Pressed(ActTabPrev)); d != 0 {
		// Skip characters that are still locked.
		n := len(characters)
		for range n {
			s.charIdx = (s.charIdx + d + n) % n
			if !characters[s.charIdx].locked() {
				break
			}
		}
		s.sel, s.scroll, s.scrollView = 0, 0, 0
		playSE(seMove)
		return
	}
	old := s.sel
	if g.in.Repeat(ActLeft) && s.sel > 0 {
		s.sel--
	}
	if g.in.Repeat(ActRight) && s.sel < n-1 {
		s.sel++
	}
	if g.in.Repeat(ActUp) && s.sel >= galleryCols {
		s.sel -= galleryCols
	}
	if g.in.Repeat(ActDown) && s.sel+galleryCols < n {
		s.sel += galleryCols
	}
	if s.sel != old {
		playSE(seMove)
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
	if g.in.Pressed(ActConfirm) {
		if s.open[s.sel] {
			s.viewing, s.pan = true, 0
			playSE(seConfirm)
		} else {
			playSE(seDenied)
		}
	}
	if g.in.Pressed(ActCancel) {
		playSE(seCancel)
		g.SetScene(newTitleScene())
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// releaseViewed frees the full-size image of the illustration being viewed (so 100 of them are not held).
func (s *GalleryScene) releaseViewed(items []galleryItem) {
	if it := items[s.sel]; it.cg {
		it.e.ReleaseFull()
	}
}

func (s *GalleryScene) Draw(screen *ebiten.Image) {
	screen.DrawImage(galleryBackground(), nil)
	items := s.items()
	if s.viewing {
		dimScreen(screen, 0xf0)
		it := items[s.sel]
		if it.cg {
			// as wide as the window (no bands at the sides), scrolled up and down by pan
			if img := it.e.Full(); img != nil {
				iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
				sc := ScreenW / iw
				drawImageScaled(screen, img, 0, -max(0, ih*sc-ScreenH)*s.pan, sc, 1)
			}
		} else {
			if bgImg := uiImage("frame_" + family(it.e.State)); bgImg != nil {
				drawImageCover(screen, bgImg, 0, 0, ScreenW, ScreenH, 0.9)
			}
			// the same size and place as in the play screen's frame: every standing pose
			// equally tall, a crouching one not tiny, nothing cut off
			if img := it.e.Img(); img != nil {
				drawPortrait(screen, img, ScreenW, ScreenH, 1, 1, 0, 0, 1, 0, 0)
			}
		}
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
			tile = it.e.tile
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
			strokeRoundRect(screen, float32(x)-4, float32(y)-4, tileW+8, tileH+8, 18, 4, candyPink)
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

// requestPicture starts decoding e's picture on a worker goroutine, once.
func (s *GalleryScene) requestPicture(it galleryItem) {
	e := it.e
	if s.loading[e] {
		return
	}
	if s.loading == nil {
		s.loading = map[*ImageEntry]bool{}
		s.decoded = make(chan decodedPicture, 64)
	}
	s.loading[e] = true
	// A portrait already loaded for play (with its figure measured) makes its tile at once,
	// without decoding the file again.
	if img := e.Image; !it.cg && img != nil {
		if f, ok := figureCache[img]; ok {
			e.tile = makeTile(func(l *ebiten.Image) {
				drawTileBackdrop(l, it)
				drawPortraitFigure(l, img, f, float64(l.Bounds().Dx()), float64(l.Bounds().Dy()), 1, 1, 0, 0, 1, 0, 0)
			})
			return
		}
	}
	go func(out chan<- decodedPicture) {
		tileDecoders <- struct{}{}
		// Twice the tile height, so the tile is drawn from a sharper picture.
		img := decodeScaled(e.base, e.ID, 2*tileInnerH)
		<-tileDecoders
		out <- decodedPicture{it, img}
	}(s.decoded)
}

// finishTiles makes the tiles of pictures that finished decoding. Uploading to the GPU is
// quick, but it is capped per frame so a frame never stalls.
func (s *GalleryScene) finishTiles() {
	for range 8 {
		select {
		case d := <-s.decoded:
			if e := d.it.e; e.tile == nil {
				e.tile = makeTile(func(l *ebiten.Image) { drawTilePicture(l, d.it, d.img) })
			}
		default:
			return
		}
	}
}

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
		t.DrawImage(roundedMask(l, 14), op)
		l.Deallocate()
	}
	strokeRoundRect(t, 3, 3, tileW-6, tileH-6, 15, 3, panelFill)
	return t
}

// drawTilePicture paints an unlocked entry's picture into the tile layer l: an
// illustration as it is, a portrait over the backdrop of its situation.
func drawTilePicture(l *ebiten.Image, it galleryItem, pic image.Image) {
	e := it.e
	lw, lh := float64(l.Bounds().Dx()), float64(l.Bounds().Dy())
	drawTileBackdrop(l, it)
	if pic == nil || it.cg {
		var img *ebiten.Image
		if pic != nil {
			img = ebiten.NewImageFromImage(pic)
		} else {
			img = placeholderImage(e.ID) // made only when it is needed: it is a large image
		}
		drawImageFit(l, img, 0, 0, lw, lh, 1)
		img.Deallocate()
		return
	}
	// a portrait is drawn as in the play screen's frame (see drawPortrait)
	b := pic.Bounds()
	img := ebiten.NewImageFromImage(pic)
	drawPortraitFigure(l, img, measureFigure(alphaPixels(pic), b.Dx(), b.Dy()), lw, lh, 1, 1, 0, 0, 1, 0, 0)
	img.Deallocate()
}

// drawTileBackdrop paints the backdrop of a portrait's situation into the tile layer l
// (an illustration has none).
func drawTileBackdrop(l *ebiten.Image, it galleryItem) {
	if it.cg {
		return
	}
	if bgImg := uiImage("frame_" + family(it.e.State)); bgImg != nil {
		drawImageCover(l, bgImg, 0, 0, float64(l.Bounds().Dx()), float64(l.Bounds().Dy()), 1)
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

// drawSilhouette draws img covering the box as a soft silhouette (only its outline shows).
func drawSilhouette(dst, img *ebiten.Image, x, y, w, h float64) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Max(w/iw, h/ih)
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
		fillRoundRect(screen, float32(x)+4, float32(y)+6, float32(w), float32(h), 12, shadowColor())
		fillRoundRect(screen, float32(x), float32(y), float32(w), float32(h), 12, panelFill)
		alpha := float32(0.55 + 0.45*pull[i])
		face := faceOf(c.selectEntry())
		switch {
		case c.locked():
			// A locked character shows only the black silhouette of her face.
			fillRoundRect(screen, float32(x)+4, float32(y)+4, float32(w)-8, float32(h)-8, 10, lockedCardFill)
			if face != nil {
				drawSilhouette(screen, face, x+4, y+4, w-8, h-8)
			}
		case face != nil:
			drawImageCover(screen, face, x+4, y+4, w-8, h-8, alpha)
		default:
			drawImageFit(screen, c.SelectImage(), x+4, y+4, w-8, h-8, alpha)
		}
		if i == sel {
			strokeRoundRect(screen, float32(x)-3, float32(y)-3, float32(w)+6, float32(h)+6, 14, 4, candyPink)
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

var galleryBG *ebiten.Image

// galleryBackground is the gallery background, built once: the gallery artwork (little
// sweets on cream) covering the screen under a soft white veil, so the tiles stand out.
func galleryBackground() *ebiten.Image {
	if galleryBG != nil {
		return galleryBG
	}
	galleryBG = ebiten.NewImage(ScreenW, ScreenH)
	if art := uiImage("gallery"); art != nil {
		drawImageCover(galleryBG, art, 0, 0, ScreenW, ScreenH, 1)
	}
	vector.FillRect(galleryBG, 0, 0, ScreenW, ScreenH, color.NRGBA{0xff, 0xff, 0xff, 0x60}, false)
	return galleryBG
}
