package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Text: the fonts by size, and centered text, outlined or not, and menus.

// faceKey names a cached font face: its size and whether it is the pop typeface.
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

// drawTextOutline draws centered text with a white outline so it stays readable over images.
func drawTextOutline(dst *ebiten.Image, str string, x, y, size float64, clr color.Color) {
	drawTextOutlineColor(dst, str, x, y, size, clr, color.White, 1)
}

// drawTextOutlineColor draws centered text in fill with an outline of the given color, both
// faded by alpha (dark outlines read on a dark screen).
func drawTextOutlineColor(dst *ebiten.Image, str string, x, y, size float64, fill, outline color.Color, alpha float32) {
	fade := func(c color.Color) color.Color {
		n, ok := color.NRGBAModel.Convert(c).(color.NRGBA)
		if !ok {
			return c // NRGBAModel always returns NRGBA; drawn unfaded if that ever changes
		}
		n.A = uint8(float32(n.A) * alpha)
		return n
	}
	w := math.Max(2, size/12)
	for i := range 12 {
		t := float64(i) / 12 * 2 * math.Pi
		drawText(dst, str, x+math.Cos(t)*w, y+math.Sin(t)*w, size, fade(outline))
	}
	drawText(dst, str, x, y, size, fade(fill))
}

// darkOutline is the outline of light text on a dark screen (the game over, the reveal of
// a new character and the secret word).
var darkOutline = color.NRGBA{0x40, 0x30, 0x48, 0xff}

// drawMenu draws a vertical menu. The selected item is pink and a little larger.
func drawMenu(dst *ebiten.Image, items []string, sel int, y, size float64) {
	drawMenuAt(dst, items, sel, ScreenW/2, y, size)
}

// drawMenuAt draws a vertical menu centered at cx.
func drawMenuAt(dst *ebiten.Image, items []string, sel int, cx, y, size float64) {
	drawMenuOn(dst, items, sel, cx, y, size, textMain, color.White)
}

// drawMenuOn draws a vertical menu centered at cx, the items not selected in normal, all
// outlined in outline (light items with a dark outline read on a dark screen).
func drawMenuOn(dst *ebiten.Image, items []string, sel int, cx, y, size float64, normal, outline color.Color) {
	for i, s := range items {
		yy := y + float64(i)*size*1.7
		clr, sz := normal, size
		if i == sel {
			clr, sz = candyPink, size*1.15
			yy -= (sz - size) / 2
		}
		drawTextOutlineColor(dst, s, cx, yy, sz, clr, outline, 1)
	}
}
