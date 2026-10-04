package gfx

import (
	"bytes"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// Text: the fonts by size, and centered text, outlined or not, and menus.

var (
	regularSource *text.GoTextFaceSource // M+ 1p, which also covers Japanese
	popSource     *text.GoTextFaceSource // Lilita One, a pop typeface for Latin text (OFL, assets/fonts/)
)

// LoadFonts parses the fonts the text is drawn in: regular (M+ 1p, which also covers
// Japanese) for any text, and pop (Lilita One) for ASCII-only text. Without a pop font
// (nil, or one that does not parse, which is logged) every text uses the regular one.
func LoadFonts(regular, pop []byte) error {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(regular))
	if err != nil {
		return err
	}
	regularSource = src
	if pop == nil {
		return nil
	}
	if src, err := text.NewGoTextFaceSource(bytes.NewReader(pop)); err == nil {
		popSource = src
	} else {
		log.Printf("cannot load the pop font, using the default font: %v", err)
	}
	return nil
}

// faceKey names a cached font face: its size and whether it is the pop typeface.
type faceKey struct {
	size float64
	pop  bool
}

var faceCache = map[faceKey]*text.GoTextFace{}

// Face returns a font face for the size. ASCII-only strings use the pop typeface (Lilita One).
func Face(size float64, s string) *text.GoTextFace {
	pop := popSource != nil && isASCII(s)
	k := faceKey{size, pop}
	f, ok := faceCache[k]
	if !ok {
		src := regularSource
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

// DrawText draws s horizontally centered on x.
func DrawText(dst *ebiten.Image, s string, x, y, size float64, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(clr)
	op.LineSpacing = size * 1.4
	op.PrimaryAlign = text.AlignCenter
	text.Draw(dst, s, Face(size, s), op)
}

// DrawTextOutline draws centered text with a white outline so it stays readable over images.
func DrawTextOutline(dst *ebiten.Image, str string, x, y, size float64, clr color.Color) {
	DrawTextOutlineColor(dst, str, x, y, size, clr, color.White, 1)
}

// DrawTextOutlineColor draws centered text in fill with an outline of the given color, both
// faded by alpha (dark outlines read on a dark screen).
func DrawTextOutlineColor(dst *ebiten.Image, str string, x, y, size float64, fill, outline color.Color, alpha float32) {
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
		DrawText(dst, str, x+math.Cos(t)*w, y+math.Sin(t)*w, size, fade(outline))
	}
	DrawText(dst, str, x, y, size, fade(fill))
}

// DarkOutline is the outline of light text on a dark screen (the game over, the reveal of
// a new character and the secret word).
var DarkOutline = color.NRGBA{0x40, 0x30, 0x48, 0xff}

// DrawMenuAt draws a vertical menu centered at cx.
func DrawMenuAt(dst *ebiten.Image, items []string, sel int, cx, y, size float64) {
	DrawMenuOn(dst, items, sel, cx, y, size, TextMain, color.White)
}

// DrawMenuOn draws a vertical menu centered at cx, the items not selected in normal, all
// outlined in outline (light items with a dark outline read on a dark screen).
func DrawMenuOn(dst *ebiten.Image, items []string, sel int, cx, y, size float64, normal, outline color.Color) {
	for i, s := range items {
		yy := y + float64(i)*size*1.7
		clr, sz := normal, size
		if i == sel {
			clr, sz = CandyPink, size*1.15
			yy -= (sz - size) / 2
		}
		DrawTextOutlineColor(dst, s, cx, yy, sz, clr, outline, 1)
	}
}
