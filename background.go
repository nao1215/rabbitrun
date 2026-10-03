package main

import (
	"image"
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

// uiImage reads assets/ui/<name>.jpg (or .png), returning nil if missing. If prefetchUI
// started decoding it, it only waits for that and uploads it.
func uiImage(name string) *ebiten.Image {
	if name == "" {
		return nil
	}
	if img, ok := uiCache[name]; ok {
		return img
	}
	var d decodedUI
	if ch, ok := uiPending[name]; ok {
		d = <-ch
		delete(uiPending, name)
	} else {
		d = decodeUI(name)
	}
	var img *ebiten.Image
	if d.err == nil {
		img = ebiten.NewImageFromImage(d.img)
	} else {
		logBrokenImage(name, d.err)
	}
	uiCache[name] = img
	return img
}

// decodedUI is a piece of artwork decoded off the main goroutine.
type decodedUI struct {
	img image.Image
	err error
}

// decodeUI decodes assets/ui/<name> as uiImage uploads it. It may run on any goroutine.
func decodeUI(name string) decodedUI {
	img, err := decodeImage("assets/ui/" + name)
	if err != nil {
		return decodedUI{err: err}
	}
	return decodedUI{img: toRGBA(img)}
}

// uiPending holds the artwork prefetchUI is decoding in the background. Only the main
// goroutine uses the map; each decoder only sends on its channel.
var uiPending = map[string]chan decodedUI{}

// prefetchUI starts decoding the artwork names in the background, so the frame that first
// shows one only uploads it: a frame of the play screen decoded the frame of a new
// expression (a JPEG) on the main goroutine, 5 to 6 ms, the first time it showed.
func prefetchUI(names ...string) {
	for _, name := range names {
		if _, ok := uiCache[name]; ok {
			continue
		}
		if _, ok := uiPending[name]; ok {
			continue
		}
		ch := make(chan decodedUI, 1)
		uiPending[name] = ch
		go func() { ch <- decodeUI(name) }()
	}
}
