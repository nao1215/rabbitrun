package main

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/gfx"
	"github.com/nao1215/rabbitrun/road"
)

// The road on the screen: its frame and illustration, the gummy walls, the sweets and the
// bunny the player steers.

func (s *PlayScene) drawBoard(screen *ebiten.Image) {
	e := s.eng
	bx, by := boardX, boardY
	w, h := float32(cell*BoardW), float32(cell*VisibleRows)
	// The road's frame: a white border with a flat shadow
	gfx.FillRoundRect(screen, float32(bx), float32(by)+2, w+16, h+16, 18, shadowColor())
	gfx.FillRoundRect(screen, float32(bx)-8, float32(by)-8, w+16, h+16, 18, gfx.PanelFill)
	// The base picture under the road (assets/ui/board), covering the frame
	if img := assets.UI("board"); img != nil {
		gfx.DrawImageCover(screen, img, bx, by, float64(w), float64(h), 1)
	}
	// After the first course, the latest illustration fills the frame behind the road.
	a := float32(0)
	if s.prevCG != nil {
		a = 1
		gfx.DrawImageCoverTop(screen, s.prevCG.Full(), bx, by, float64(w), float64(h), 1)
	}
	if s.stageCG != nil {
		a = max(a, float32(s.stageFade))
		gfx.DrawImageCoverTop(screen, s.stageCG.Full(), bx, by, float64(w), float64(h), float32(s.stageFade))
	}
	// Wash with white so the walls stay readable: the plain board a little more, an
	// illustration lightly (cgVeil), all the time, so the picture still shows clearly
	wash := float32(0x70)*(1-a) + float32(cgVeil)*a
	vector.FillRect(screen, float32(bx), float32(by), w, h, color.NRGBA{0xff, 0xff, 0xff, uint8(wash)}, false)

	// The road: gummy walls and sweets, moving down smoothly between steps. It is drawn
	// on a layer the size of the road's frame, so nothing pokes out of it.
	layer := offscreen(&s.fadeLayer)
	// the road stays where it stopped, also after the game is over (drawing it at a whole
	// row there made the walls and the sweets jump up to a row at the miss)
	scroll := e.Scroll()
	// whole pixels only: at a fraction of a pixel the seams of the blocks shimmer as lines
	rowY := func(y int) float64 { return math.Round((float64(y) + scroll - 1) * cell) }
	g := e.G
	fade := float32(1)
	if e.Over() {
		fade = float32(math.Max(0.35, 1-float64(s.overFrame)/90))
	}
	// The walls are single gummy blocks, one per cell, like the old brick games.
	wallAt := func(x, y int) int8 {
		if x < 0 || x >= BoardW || y < 0 || y > VisibleRows {
			return 1 // beyond the screen: as good as a wall
		}
		w := g.Ahead[x].Wall // the row coming in at the top
		if y > 0 {
			w = g.Rows[y-1][x].Wall
		}
		if s.showingWalls() || s.breaking() && !s.rowBroken(y) {
			w = s.hammerWalls[y][x].Wall // the hammer's walls, not broken yet
		}
		return w
	}
	buried := func(x, y int) bool {
		if s.showing {
			return false // the hammer show's wall of blocks is drawn whole
		}
		return wallAt(x-1, y) != 0 && wallAt(x+1, y) != 0 && wallAt(x, y-1) != 0 && wallAt(x, y+1) != 0
	}
	// the cells of the walls left out get a soft white, so the illustration shows there
	// but it does not look like road
	for y := range VisibleRows + 1 {
		for x := range BoardW {
			if wallAt(x, y) != 0 && buried(x, y) {
				vector.FillRect(layer, float32(float64(x)*cell), float32(rowY(y)), cell, cell, color.NRGBA{0xff, 0xff, 0xff, 0x80}, false)
			}
		}
	}
	drawGummyGrid(layer, BoardW, VisibleRows+1, func(x, y int) gummyCell {
		if y > VisibleRows {
			return gummyCell{}
		}
		w := wallAt(x, y)
		if w == 0 {
			return gummyCell{}
		}
		// only the blocks that face the road are drawn: a wall inside walls on every side
		// is left out, so a thick wall shows as one row of blocks with the illustration
		// behind it (it is a wall all the same)
		if buried(x, y) {
			return gummyCell{}
		}
		return gummyCell{kind: Kind(w), id: int32(y*BoardW + x + 1), alpha: fade} //nolint:gosec // G115: at most 17*9 cells; each cell its own block
	}, 0, rowY(0), cell)
	if s.breaking() {
		s.drawShards(layer, rowY)
	}
	for y := -1; y < VisibleRows; y++ {
		r := g.Ahead
		if y >= 0 {
			r = g.Rows[y]
		}
		for x := range BoardW {
			if k := r[x].Sweet; k != road.SweetNone {
				drawSweet(layer, k, nearestWall(r, x, g.WallColor()), charFace(s.char), (float64(x)+0.5)*cell, rowY(y+1)+0.5*cell, cell, s.frame)
			}
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(bx, by)
	screen.DrawImage(layer, op)
	if s.bunnyHidden() {
		return
	}
	// The player: the bunny hopping up the road on her row. After a crash she is
	// see-through while walls pass through her.
	alpha := float32(1)
	if g.Safe > 0 {
		alpha = 0.45
	}
	// She stands just below her row: a row that hits her comes in with its bottom edge
	// on the tips of her ears (a wall running into her stops the road as it touches her)
	// and slides down over her while it is hers.
	drawBunny(screen, bx+g.X*cell, by+(float64(road.PlayerRow)+0.75)*cell+bunnyHeight,
		float64(g.Distance)+s.eng.Scroll(), s.lean, alpha)
}

// bunnyHidden reports whether the bunny is left out this frame: she blinks during READY
// (but not while she gets back up after a retry).
func (s *PlayScene) bunnyHidden() bool { return s.ready > 0 && s.comeback == 0 && s.frame%2 == 0 }

// updateLean follows which way and how fast the bunny slides, smoothed, for her tilt. It
// runs at the end of every Update, on the frames she is drawn, so Draw only reads it.
func (s *PlayScene) updateLean() {
	if s.bunnyHidden() {
		return
	}
	x := s.eng.G.X
	lean := math.Max(-1, math.Min(1, (x-s.prevX)*12))
	s.prevX = x
	s.lean += (lean - s.lean) * 0.2
}

// bunnyHeight is how tall the bunny is drawn (a little over a row).
const bunnyHeight = cell * 1.25

// The bunny the player steers (assets/ui/pet.png, seen from behind). It is drawn in
// two parts, the ears and the body, so the ears can flop while she hops.
const (
	bunnyEarsBottom = 90 // the ears are the rows above this (they overlap the head a little)
	bunnyBodyTop    = 72 // the body is the rows from this down (drawn over the ear roots)
)

// drawBunny draws the bunny with her feet at (cx, footY). dist is how far the road
// has run (in rows): she hops once every two rows, squashing as she lands. lean (-1..1)
// tilts her toward the way she slides, and her ears trail behind.
func drawBunny(dst *ebiten.Image, cx, footY, dist, lean float64, alpha float32) {
	img := assets.UI("pet")
	if img == nil {
		return
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	sc := bunnyHeight / h
	p := math.Mod(dist/2, 1)           // 0 -> 1 over one hop
	up := math.Sin(math.Pi * p)        // 0 on the ground, 1 at the top
	squash := math.Pow(1-up, 6) * 0.14 // flattened as she lands
	stretch := up * 0.05               // a little taller in the air
	sx, sy := sc*(1+squash-stretch), sc*(1-squash+stretch)
	lift := up * cell * 0.22
	tilt := lean * 0.18
	// the shadow stays on the ground, smaller while she is up
	sw := w * sc * (0.75 - 0.25*up)
	drawEllipse(dst, cx, footY-2, sw/2, sw/7, color.NRGBA{0x60, 0x40, 0x50, 0x40}, alpha)
	// the body pivots at her feet; the ears at their roots
	body, ok1 := img.SubImage(image.Rect(0, bunnyBodyTop, int(w), int(h))).(*ebiten.Image)
	ears, ok2 := img.SubImage(image.Rect(0, 0, int(w), bunnyEarsBottom)).(*ebiten.Image)
	if !ok1 || !ok2 {
		return
	}
	// each part gets a soft rose outline first (a white bunny on a pale road would
	// melt into it), then the part itself
	place := func(part *ebiten.Image, top, pivotY, ang, px, py float64) {
		for i := range 9 {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(-w/2, top-pivotY)
			op.GeoM.Scale(sx, sy)
			op.GeoM.Rotate(ang)
			op.GeoM.Translate(px, py)
			op.Filter = ebiten.FilterLinear
			if i < 8 {
				a := float64(i) * math.Pi / 4
				op.GeoM.Translate(1.4*math.Cos(a), 1.4*math.Sin(a))
				op.ColorScale.Scale(0.95, 0.68, 0.8, 0.8)
			}
			op.ColorScale.ScaleAlpha(alpha)
			dst.DrawImage(part, op)
		}
	}
	feetX, feetY := cx, footY-lift
	// where the ear roots are once the body is tilted and scaled
	rootDY := (float64(bunnyEarsBottom) - 10 - h) * sy
	rootX := feetX - math.Sin(tilt)*rootDY
	rootY := feetY + math.Cos(tilt)*rootDY
	// ears: trail the slide and flop with the hop (back when rising, forward on landing)
	earAng := tilt - lean*0.25 + math.Cos(math.Pi*p)*0.10
	place(ears, 0, float64(bunnyEarsBottom)-10, earAng, rootX, rootY)
	place(body, float64(bunnyBodyTop), h, tilt, feetX, feetY)
}

// drawEllipse fills an axis-aligned ellipse (a soft shadow on the ground).
func drawEllipse(dst *ebiten.Image, cx, cy, rx, ry float64, clr color.NRGBA, alpha float32) {
	if ellipseImg == nil {
		ellipseImg = ebiten.NewImage(64, 64)
		vector.FillCircle(ellipseImg, 32, 32, 32, color.White, true)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-32, -32)
	op.GeoM.Scale(rx/32, ry/32)
	op.GeoM.Translate(cx, cy)
	op.ColorScale.ScaleWithColor(clr)
	op.ColorScale.ScaleAlpha(alpha)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(ellipseImg, op)
}

var ellipseImg *ebiten.Image

// offscreen prepares and clears a scratch image the size of the road's frame.
func offscreen(img **ebiten.Image) *ebiten.Image {
	if *img == nil {
		*img = ebiten.NewImage(int(cell*BoardW), int(cell*VisibleRows))
	}
	(*img).Clear()
	return *img
}

// drawSweet draws a sweet lying on the road, bobbing gently, centered at cx, cy. The
// extra life is the character's SD figure (sd) marked 1UP.
func drawSweet(dst *ebiten.Image, kind, wall int8, sd *ebiten.Image, cx, cy, cell float64, frame int) {
	bob := math.Sin(float64(frame)*0.12+cx) * cell * 0.04
	if kind == road.SweetOneUp {
		// her face with "1UP" on it, as in the old games; kept inside its cell, so it does
		// not seem to run into the walls beside it
		if sd != nil {
			size := cell * 0.78
			gfx.DrawImageFit(dst, sd, cx-size/2, cy-size/2+bob-cell*0.1, size, size, 1)
		}
		gfx.DrawTextOutline(dst, "1UP", cx, cy+cell*0.14+bob, cell*0.36, gfx.CandyPink)
		return
	}
	if kind == road.SweetBomb {
		if img := hammerImage(); img != nil {
			gfx.DrawImageFit(dst, img, cx-cell*0.55, cy-cell*0.55+bob, cell*1.1, cell*1.1, 1)
		}
		return
	}
	// Every sweet is a macaron of the same size in one of three colors.
	img := macaronFor(int(cx/cell), wall)
	size := cell * 0.8
	gfx.DrawImageFit(dst, img, cx-size/2, cy-size/2+bob, size, size, 1)
}

// The colors of the macarons on the road (assets/ui/macaron_<color>.png).
const (
	macPink  = "pink"
	macMint  = "mint"
	macLemon = "lemon"
)

// macaronColors are the macaron colors, in the order they take turns.
var macaronColors = []string{macPink, macMint, macLemon}

// macaronClash is the macaron color that looks too much like a wall color (the candy
// colors of Kind: 1 soda, 2 lemon, 4 melon, 5 strawberry, 7 orange), so a sweet is never
// lost against the walls around it.
var macaronClash = map[int8]string{1: macMint, 2: macLemon, 4: macMint, 5: macPink, 7: macLemon}

// macaronImage is the macaron of the i-th color (the colors take turns).
func macaronImage(i int) *ebiten.Image {
	return assets.UI(macaronNames(-1)[i%len(macaronColors)])
}

// macaronFor is the macaron for column i among walls of the color wall: the colors take
// turns across the road, leaving out the one that looks like the walls.
func macaronFor(i int, wall int8) *ebiten.Image {
	names := macaronNames(wall)
	return assets.UI(names[i%len(names)])
}

// macaronSets are the artwork names of the macaron colors that go with each wall color
// (all of them under -1), worked out once: every sweet on the road asks every frame.
var macaronSets = map[int8][]string{}

// macaronNames returns the artwork names of the macaron colors, in turn, leaving out the
// one that looks like walls of the color wall (-1 leaves none out).
func macaronNames(wall int8) []string {
	if names, ok := macaronSets[wall]; ok {
		return names
	}
	names := make([]string, 0, len(macaronColors))
	for _, c := range macaronColors {
		if wall < 0 || c != macaronClash[wall] {
			names = append(names, "macaron_"+c)
		}
	}
	macaronSets[wall] = names
	return names
}

// cgVeil is how strongly the illustration behind the road is washed with white: light,
// so the picture shows, but enough for the walls and the sweets to read over it.
const cgVeil = 0x88

// nearestWall is the color of the wall nearest to column x in row r (the walls a sweet there
// is seen against; on a colorful course they differ block by block), or def without walls.
func nearestWall(r road.Row, x int, def int8) int8 {
	for d := 1; d < road.W; d++ {
		for _, nx := range []int{x - d, x + d} {
			if nx >= 0 && nx < road.W && r[nx].Wall != 0 {
				return r[nx].Wall
			}
		}
	}
	return def
}
