package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// The background: a flat solid color chosen by scene and expression, faded in slowly,
// under the screen's artwork (assets/ui/).

// Pop color palette for the flat backgrounds.
var (
	popPink     = color.NRGBA{0xf6, 0xb3, 0xc6, 0xff}
	popYellow   = color.NRGBA{0xf8, 0xdc, 0x6c, 0xff}
	popOrange   = color.NRGBA{0xf4, 0x95, 0x55, 0xff}
	popMint     = color.NRGBA{0xb8, 0xe6, 0xda, 0xff}
	popLavender = color.NRGBA{0xd4, 0xc4, 0xf0, 0xff}
	popCream    = color.NRGBA{0xfb, 0xf1, 0xd8, 0xff}
	popGray     = color.NRGBA{0xc4, 0xc0, 0xcc, 0xff}
)

type background struct {
	cur, target [3]float64
	image       string // assets/ui/<image>.jpg drawn underneath (color only if empty)
}

func (b *background) setImage(name string) { b.image = name }

func newBackground() *background {
	b := &background{}
	b.set(popPink)
	b.cur = b.target
	return b
}

func (b *background) set(c color.NRGBA) {
	b.target = [3]float64{float64(c.R), float64(c.G), float64(c.B)}
}

func (b *background) update() {
	for i := range b.cur {
		b.cur[i] += (b.target[i] - b.cur[i]) * 0.06
	}
}

func (b *background) color() color.NRGBA {
	return color.NRGBA{uint8(b.cur[0]), uint8(b.cur[1]), uint8(b.cur[2]), 0xff}
}

func (b *background) draw(screen *ebiten.Image) {
	screen.Fill(b.color())
	if img := uiImage(b.image); img != nil {
		drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, 1)
	}
}

// bg is the current background. Scenes set its color.
var bg *background

var uiCache = map[string]*ebiten.Image{}

// imageExts lists the image file extensions tried in order when looking up artwork.
var imageExts = []string{".jpg", ".png"}

// uiImage reads assets/ui/<name>.jpg (or .png), returning nil if missing.
func uiImage(name string) *ebiten.Image {
	if name == "" {
		return nil
	}
	if img, ok := uiCache[name]; ok {
		return img
	}
	var img *ebiten.Image
	if dec, err := decodeImage("assets/ui/" + name); err == nil {
		img = ebiten.NewImageFromImage(dec)
	}
	uiCache[name] = img
	return img
}
