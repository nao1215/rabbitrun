package game

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/assets"
)

const blockTex = 64 // block texture resolution; scaled down when drawn

// The candy colors of the gummy blocks.
var kindColors = map[Kind]color.RGBA{
	KindSoda:       {0x4c, 0xc4, 0xf4, 0xff}, // soda
	KindLemon:      {0xff, 0xd2, 0x3c, 0xff}, // lemon
	KindGrape:      {0xb0, 0x70, 0xf0, 0xff}, // grape
	KindMelon:      {0x5c, 0xd8, 0x70, 0xff}, // melon
	KindStrawberry: {0xff, 0x55, 0x78, 0xff}, // strawberry
	KindBlueberry:  {0x55, 0x78, 0xf0, 0xff}, // blueberry
	KindOrange:     {0xff, 0x98, 0x40, 0xff}, // orange
}

var blockImages [kindCount + 1]*ebiten.Image

// kindNames are the file names of the blocks (assets/blocks/<name>.png).
var kindNames = map[Kind]string{KindSoda: "soda", KindLemon: "lemon", KindGrape: "grape", KindMelon: "melon",
	KindStrawberry: "strawberry", KindBlueberry: "blueberry", KindOrange: "orange"}

// initBlocks loads the candy blocks from assets/blocks/<name>.png.
// Missing ones fall back to procedurally drawn glossy blocks.
func initBlocks() {
	for k := KindSoda; k <= KindOrange; k++ {
		img, err := assets.DecodeFile(assets.FS(), "blocks/"+kindNames[k]+".png")
		if err == nil {
			blockImages[k] = ebiten.NewImageFromImage(img)
			continue
		}
		assets.LogBroken(kindNames[k], err)
		blockImages[k] = ebiten.NewImageFromImage(renderGlossyBlock(kindColors[k], blockTex))
	}
}

func smoothstep(a, b, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-a)/(b-a)))
	return t * t * (3 - 2*t)
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// renderGlossyBlock draws one glossy block that looks like candy (a jelly bean).
// It layers rounded corners, a bevel, a gloss band on the top half, a top-left specular and a bottom glow.
func renderGlossyBlock(c color.RGBA, size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	br, bg, bb := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	const radius = 0.16
	const bevel = 0.10
	for py := range size {
		for px := range size {
			u := (float64(px) + 0.5) / float64(size)
			v := (float64(py) + 0.5) / float64(size)

			// signed distance to the rounded rectangle (negative inside)
			qx := math.Abs(u-0.5) - (0.5 - radius)
			qy := math.Abs(v-0.5) - (0.5 - radius)
			outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - radius
			alpha := clamp01(-outside*float64(size) + 0.5)
			if alpha <= 0 {
				continue
			}
			edge := -outside // distance from the edge

			// base color with a vertical gradient
			shade := 1.18 - 0.45*v
			r, g, b := br*shade, bg*shade, bb*shade

			// bevel: lighter top-left, darker bottom-right
			if edge < bevel {
				t := 1 - edge/bevel
				light := (0.5 - u) + (0.5 - v) // positive toward the top-left
				k := 1 + 0.55*t*light
				r, g, b = r*k, g*k, b*k
			}

			// bottom glow (slightly brightens the color)
			glow := smoothstep(0.55, 0.95, v) * smoothstep(0.02, 0.12, edge) * 0.35
			r += (1 - r) * glow * 0.6
			g += (1 - g) * glow * 0.6
			b += (1 - b) * glow * 0.6

			// gloss band on the top half with a gently curved lower edge
			boundary := 0.50 - 0.10*math.Pow(2*math.Abs(u-0.5), 2)
			inset := smoothstep(0.05, 0.10, edge)
			if v < boundary {
				t := v / boundary
				gloss := (0.70 - 0.50*t) * inset * smoothstep(boundary, boundary-0.03, v)
				r += (1 - r) * gloss
				g += (1 - g) * gloss
				b += (1 - b) * gloss
			}

			// top-left specular
			d := math.Hypot((u-0.27)/1.3, v-0.20)
			spec := 0.7 * smoothstep(0.08, 0.02, d)
			r += (1 - r) * spec
			g += (1 - g) * spec
			b += (1 - b) * spec

			// dark outline
			if edge < 1.2/float64(size)*2 {
				r, g, b = r*0.55, g*0.55, b*0.55
			}

			a := alpha
			img.SetRGBA(px, py, color.RGBA{
				uint8(clamp01(r) * 255 * a), uint8(clamp01(g) * 255 * a), uint8(clamp01(b) * 255 * a), uint8(a * 255),
			})
		}
	}
	return img
}

// ---- Connected gummy drawing (joins neighboring cells of the same id into one gummy) ----

// gummyCell describes one cell to draw.
type gummyCell struct {
	kind  Kind
	id    int32 // adjacent cells with the same ID are joined into one gummy
	alpha float32
}

// drawGummyGrid draws a w x h grid from (ox, oy) with size spacing, skipping cells where at returns kind=Empty.
// Each cell is split 3x3; sides joined to a same-ID neighbor use the image center instead of its edge.
// This removes the borders and rounding between neighbors so joined cells look like one gummy.
func drawGummyGrid(dst *ebiten.Image, w, h int, at func(x, y int) gummyCell, ox, oy, size float64) {
	get := func(x, y int) gummyCell {
		if x < 0 || y < 0 || x >= w || y >= h {
			return gummyCell{}
		}
		return at(x, y)
	}
	same := func(a, b gummyCell) bool { return a.kind != Empty && a.kind == b.kind && a.id == b.id }
	for y := range h {
		for x := range w {
			c := get(x, y)
			if c.kind == Empty {
				continue
			}
			// whether each direction is joined
			l, r := same(c, get(x-1, y)), same(c, get(x+1, y))
			u, d := same(c, get(x, y-1)), same(c, get(x, y+1))
			px, py := ox+float64(x)*size, oy+float64(y)*size
			// An opaque lone gummy on whole pixels (every wall block of the road) is drawn at
			// once from its finished picture, the same pixels as its nine parts. (See-through,
			// the parts drawn one by one show faint seams where they meet; the fading walls of
			// a game over keep them.)
			if !l && !r && !u && !d && c.alpha == 1 && whole(px) && whole(py) && whole(size) {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(px, py)
				dst.DrawImage(loneGummy(c.kind, int(size)), op)
				continue
			}
			drawGummyParts(dst, c, l, r, u, d, px, py, size)
		}
	}
}

// whole reports whether v is a whole number.
func whole(v float64) bool { return v == math.Trunc(v) }

type loneGummyKey struct {
	kind Kind
	size int
}

// loneGummies are the finished pictures of a gummy joined on no side, per kind and size.
var loneGummies = map[loneGummyKey]*ebiten.Image{}

// loneGummy returns the picture of a gummy of kind joined on no side, size pixels square,
// made once from its nine parts: drawing every wall block as nine stretched parts was nine
// times the draws a frame.
func loneGummy(kind Kind, size int) *ebiten.Image {
	k := loneGummyKey{kind, size}
	img, ok := loneGummies[k]
	if !ok {
		img = ebiten.NewImage(size, size)
		drawGummyParts(img, gummyCell{kind: kind, alpha: 1}, false, false, false, false, 0, 0, float64(size))
		loneGummies[k] = img
	}
	return img
}

// drawGummyParts draws the cell c at (px, py), size square, as its 3x3 parts; l, r, u and
// d tell which sides are joined to a neighbor.
func drawGummyParts(dst *ebiten.Image, c gummyCell, l, r, u, d bool, px, py, size float64) {
	edges := [4]float64{0, 0.3, 0.7, 1} // 3x3 split on the destination
	// split on the source image; the middle uses only the flat center so stretching does not cause banding
	src := [4]float64{0, 0.3, 0.7, 1}
	srcMid := [2]float64{0.46, 0.54}
	for j := range 3 {
		srcRow := j
		if (j == 0 && u) || (j == 2 && d) {
			srcRow = 1
		}
		for i := range 3 {
			srcCol := i
			if (i == 0 && l) || (i == 2 && r) {
				srcCol = 1
			}
			su0, su1 := src[srcCol], src[srcCol+1]
			if srcCol == 1 {
				su0, su1 = srcMid[0], srcMid[1]
			}
			sv0, sv1 := src[srcRow], src[srcRow+1]
			if srcRow == 1 {
				sv0, sv1 = srcMid[0], srcMid[1]
			}
			drawGummyPart(dst, c, su0, sv0, su1, sv1,
				px+edges[i]*size, py+edges[j]*size, (edges[i+1]-edges[i])*size, (edges[j+1]-edges[j])*size)
		}
	}
}

// drawGummyPart stretches the (u0,v0)-(u1,v1) region (0 to 1) of the gummy image onto (x, y, w, h).
func drawGummyPart(dst *ebiten.Image, c gummyCell, u0, v0, u1, v1, x, y, w, h float64) {
	img := blockImages[c.kind]
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	src, ok := img.SubImage(image.Rect(int(u0*iw), int(v0*ih), int(u1*iw), int(v1*ih))).(*ebiten.Image)
	if !ok {
		return
	}
	sw, sh := (u1-u0)*iw, (v1-v0)*ih
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(w/sw, h/sh)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(c.alpha)
	dst.DrawImage(src, op)
}
