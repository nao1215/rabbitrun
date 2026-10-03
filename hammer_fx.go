package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/road"
)

// The hammer: swinging one, its cut-in, and the walls breaking after it.

// cutinFrames is how long the cut-in of a hammer lasts (the road waits meanwhile).
const cutinFrames = 70

// useHammer swings a hammer: the character cuts in big, and every wall on the screen bursts.
func (s *PlayScene) useHammer() {
	g := s.eng.G
	s.hammerWalls[0] = g.Ahead
	copy(s.hammerWalls[1:], g.Rows[:])
	if !s.eng.UseHammer() {
		playSE(seDenied)
		return
	}
	playSE(seHammer)
	s.cutin = cutinFrames
	s.react(ExprExcited, 120, rankBig)
}

// drawCutin draws the cut-in of a hammer: a pastel band with speed lines sweeps across
// the screen, and the character bursts out of it (her head rises above the band), holds,
// and sweeps out to the left.
func (s *PlayScene) drawCutin(screen *ebiten.Image) {
	if s.cutin == 0 {
		return
	}
	t := 1 - float64(s.cutin)/cutinFrames // 0 -> 1
	var x float64
	switch {
	case t < 0.2:
		u := 1 - t/0.2
		x = u * u * ScreenW // in, easing out
	case t > 0.85:
		u := (t - 0.85) / 0.15
		x = -u * u * ScreenW // out to the left
	}
	dim := uint8(0x60 * math.Sin(math.Min(1, t*1.2)*math.Pi))
	dimScreen(screen, dim)
	const bandH = 360.0
	bandY := ScreenH*0.56 - bandH/2
	// the band: pink, white edges, and white speed lines streaming left
	vector.FillRect(screen, float32(x), float32(bandY-8), ScreenW, bandH+16, color.White, false)
	vector.FillRect(screen, float32(x), float32(bandY), ScreenW, bandH, color.NRGBA{0xf7, 0x8f, 0xb3, 0xf0}, false)
	for i := range 14 {
		ly := bandY + 14 + float64(i*37%int(bandH-28))
		length := 80 + float64(i*53%160)
		lx := math.Mod(float64(i*97)-float64(s.cutin)*28, ScreenW+length) // streaming left
		if lx < 0 {
			lx += ScreenW + length
		}
		vector.FillRect(screen, float32(x+lx-length), float32(ly), float32(length), 4, color.NRGBA{0xff, 0xff, 0xff, 0xb0}, false)
	}
	// her upper body stands on the bottom of the band and rises out of its top
	if img := cutinImage(s.char); img != nil {
		iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
		h := bandH * 1.55
		sc := math.Min(h/ih, ScreenW*0.8/iw)
		drawImageScaled(screen, img, x+ScreenW*0.55-iw*sc/2, bandY+bandH-ih*sc, sc, 1)
	}
	drawTextOutline(screen, "SMASH!", x+ScreenW*0.2, bandY+bandH-90, 64, candyPink)
}

// hammerImage is the picture of the hammer item: a pop squeaky toy hammer that smashes the walls
// (assets/ui/hammer.png), shown on the road and in the stock.
func hammerImage() *ebiten.Image { return uiImage("hammer") }

// cutinImage is the big picture of the hammer's cut-in: images/cutin.png (no background, a
// "here I go!" pose), or the character select picture until it exists. It is decoded in
// the background with her portraits (portraitEntries) and is not waited for: nil until it
// is ready (the band sweeps in from the right meanwhile).
func cutinImage(c *Character) *ebiten.Image {
	if c.Cutin != nil && c.Cutin.HasImage() {
		return c.Cutin.ImgReady()
	}
	return c.SelectImage()
}

// The walls break after the hammer's cut-in: a row every crumbleStep frames from the
// bottom up, its blocks flying apart as shards for crumbleFly frames.
const (
	crumbleStep = 3
	crumbleFly  = 18
)

// breaking reports whether the walls are breaking after the hammer.
func (s *PlayScene) breaking() bool { return s.crumble > 0 }

// updateCrumble moves the breaking on a frame: a row breaks every crumbleStep frames,
// with a crack when it has walls.
func (s *PlayScene) updateCrumble() {
	if k := (s.crumble - 1) / crumbleStep; (s.crumble-1)%crumbleStep == 0 && k <= road.Rows {
		if rowHasWall(s.hammerWalls[road.Rows-k]) { // the row breaking now (rowBroken)
			playSE(seBreak)
		}
	}
	if s.crumble++; s.crumble > (road.Rows+1)*crumbleStep+crumbleFly {
		s.crumble = 0
	}
}

// rowBroken reports whether row y of the hammer's walls (0 is the row coming in at the
// top) has broken yet.
func (s *PlayScene) rowBroken(y int) bool { return s.crumble >= (road.Rows-y)*crumbleStep }

// rowHasWall reports whether a row has any wall in it.
func rowHasWall(r road.Row) bool {
	for _, c := range r {
		if c.Wall != 0 {
			return true
		}
	}
	return false
}

// drawShards draws the shards of the rows breaking: each block bursts into four pieces
// that fly apart, fall and fade.
func (s *PlayScene) drawShards(dst *ebiten.Image, rowY func(int) float64) {
	for y := 0; y <= road.Rows; y++ {
		t := float64(s.crumble-(road.Rows-y)*crumbleStep) / crumbleFly // 0 -> 1 after the row breaks
		if t < 0 || t > 1 {
			continue
		}
		for x, c := range s.hammerWalls[y] {
			if c.Wall == 0 {
				continue
			}
			col := kindColors[Kind(c.Wall)]
			cx, cy := (float64(x)+0.5)*cell, rowY(y)+0.5*cell
			for k := range 4 {
				dx, dy := float64(k%2)*2-1, float64(k/2)*2-1
				px := cx + dx*(cell*0.25+t*cell*0.9)
				py := cy + dy*cell*0.25 - t*cell*0.6 + t*t*cell*1.6 // up a little, then falling
				size := float32(cell * 0.42 * (1 - 0.5*t))
				a := uint8(255 * (1 - t))
				vector.FillRect(dst, float32(px)-size/2, float32(py)-size/2, size, size, color.NRGBA{col.R, col.G, col.B, a}, false)
			}
		}
	}
}
