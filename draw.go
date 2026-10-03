package main

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Drawing helpers shared by the screens: rounded panels, pictures fitted or cropped to a
// box, and rounded corners.

// roundPath is the path roundRectPath fills in. One is enough, as the path is copied when
// it is filled or stroked, and a frame draws dozens of rounded rectangles.
var roundPath vector.Path

// roundRectPath returns the path of a rounded rectangle, valid until the next call.
func roundRectPath(x, y, w, h, r float32) *vector.Path {
	p := &roundPath
	p.Reset()
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.ArcTo(x+w, y, x+w, y+r, r)
	p.LineTo(x+w, y+h-r)
	p.ArcTo(x+w, y+h, x+w-r, y+h, r)
	p.LineTo(x+r, y+h)
	p.ArcTo(x, y+h, x, y+h-r, r)
	p.LineTo(x, y+r)
	p.ArcTo(x, y, x+r, y, r)
	p.Close()
	return p
}

func fillRoundRect(dst *ebiten.Image, x, y, w, h, r float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.FillPath(dst, roundRectPath(x, y, w, h, r), nil, op)
}

func strokeRoundRect(dst *ebiten.Image, x, y, w, h, r, width float32, clr color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(clr)
	vector.StrokePath(dst, roundRectPath(x, y, w, h, r), &vector.StrokeOptions{Width: width}, op)
}

var (
	panelFill = color.NRGBA{0xff, 0xfe, 0xfb, 0xff}
	textMain  = color.NRGBA{0x3c, 0x2a, 0x2e, 0xff}
	candyPink = color.NRGBA{0xf0, 0x5a, 0x8c, 0xff}
)

// shadowColor returns a flat shadow color derived by darkening the background.
func shadowColor() color.NRGBA {
	c := bg.color()
	return color.NRGBA{uint8(float64(c.R) * 0.78), uint8(float64(c.G) * 0.72), uint8(float64(c.B) * 0.76), 0xff}
}

// drawPanel draws a white rounded panel with a flat shadow offset to the lower right.
func drawPanel(dst *ebiten.Image, x, y, w, h float32) {
	fillRoundRect(dst, x+6, y+8, w, h, 18, shadowColor())
	fillRoundRect(dst, x, y, w, h, 18, panelFill)
}

// drawImageFit draws img centered at the largest size that fits the box.
func drawImageFit(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Min(w/iw, h/ih)
	drawImageScaled(dst, img, x+(w-iw*s)/2, y+(h-ih*s)/2, s, alpha)
}

func drawImageScaled(dst, img *ebiten.Image, x, y, s float64, alpha float32) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleAlpha(alpha)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(img, op)
}

func dimScreen(dst *ebiten.Image, a uint8) {
	c := bg.color()
	vector.FillRect(dst, 0, 0, ScreenW, ScreenH, color.NRGBA{c.R, c.G, c.B, a}, false)
}

// drawImageCover scales img to fill the box and crops the overflow evenly on every side.
func drawImageCover(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	drawImageCropped(dst, img, x, y, w, h, alpha, false)
}

// drawImageCoverTop scales img to fill the box. Overflow is cropped evenly left and right,
// and from the bottom vertically (top-aligned so faces stay visible).
func drawImageCoverTop(dst, img *ebiten.Image, x, y, w, h float64, alpha float32) {
	drawImageCropped(dst, img, x, y, w, h, alpha, true)
}

// drawImageCropped scales img to fill the box, cropping the overflow evenly left and
// right, and vertically either evenly or (top) from the bottom only.
func drawImageCropped(dst, img *ebiten.Image, x, y, w, h float64, alpha float32, top bool) {
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Max(w/iw, h/ih)
	sw, sh := w/s, h/s
	sx, sy := (iw-sw)/2, (ih-sh)/2
	if top {
		sy = 0
	}
	sub, ok := img.SubImage(image.Rect(int(sx), int(sy), int(sx+sw), int(sy+sh))).(*ebiten.Image)
	if !ok {
		return
	}
	drawImageScaled(dst, sub, x, y, s, alpha)
}

var maskCache = map[[3]int]*ebiten.Image{}

// roundedMask rounds the corners of img by radius r, in place, and returns it. The mask
// (a white rounded rectangle) is made once per size and applied with a blend: filling the
// rounded path anew every frame was most of the cost of the character's frame.
func roundedMask(img *ebiten.Image, r float32) *ebiten.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	key := [3]int{w, h, int(r)}
	mask, ok := maskCache[key]
	if !ok {
		mask = ebiten.NewImage(w, h)
		fillRoundRect(mask, 0, 0, float32(w), float32(h), r, color.White)
		maskCache[key] = mask
	}
	img.DrawImage(mask, &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn})
	return img
}
