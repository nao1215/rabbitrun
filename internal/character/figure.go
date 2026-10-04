package character

import (
	"image"
	"image/draw"

	"github.com/hajimehoshi/ebiten/v2"
)

// Figure is where the figure is in a portrait: the box around it and the middle of her
// body (the median of the opaque pixels across the middle half of her height).
type Figure struct {
	Box   image.Rectangle
	BodyX int
}

// figureCache keeps the figure of each portrait, measured when it was decoded (or read
// from the GPU once).
var figureCache = map[*ebiten.Image]Figure{}

// FigureOf finds the figure in a portrait. Every portrait Img loads was measured on the
// CPU as it was decoded; only another picture (a placeholder) is read back from the GPU.
func FigureOf(img *ebiten.Image) Figure {
	if f, ok := figureCache[img]; ok {
		return f
	}
	b := img.Bounds()
	pix := make([]byte, 4*b.Dx()*b.Dy())
	img.ReadPixels(pix)
	f := measureFigure(pix, b.Dx(), b.Dy())
	f.Box = f.Box.Add(b.Min)
	f.BodyX += b.Min.X
	figureCache[img] = f
	return f
}

// KnownFigure is the figure of a portrait measured as it was decoded, if it was.
func KnownFigure(img *ebiten.Image) (Figure, bool) {
	f, ok := figureCache[img]
	return f, ok
}

// MeasureFigure finds the figure in a decoded picture. It only touches the CPU, so it may
// run on any goroutine.
func MeasureFigure(img image.Image) Figure {
	b := img.Bounds()
	return measureFigure(alphaPixels(img), b.Dx(), b.Dy())
}

// alphaPixels returns the pixels of img as rows of 4*w bytes with the alpha at offset 3,
// as measureFigure and portraitBox read them. A decoded picture (*image.RGBA or
// *image.NRGBA, whose alpha is the same) is used as it is, without a copy.
func alphaPixels(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if b.Min == (image.Point{}) {
		switch p := img.(type) {
		case *image.RGBA:
			if p.Stride == 4*w && len(p.Pix) == 4*w*h {
				return p.Pix
			}
		case *image.NRGBA:
			if p.Stride == 4*w && len(p.Pix) == 4*w*h {
				return p.Pix
			}
		}
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba.Pix
}

// measureFigure finds the figure in RGBA pixels of size w x h.
func measureFigure(pix []byte, w, h int) Figure {
	opaque := func(x, y int) bool { return pix[(y*w+x)*4+3] > 24 }
	minX, minY, maxX, maxY := w, h, -1, -1
	for y := range h {
		for x := range w {
			if opaque(x, y) {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	var f Figure
	if maxX < 0 {
		return f
	}
	f.Box = image.Rect(minX, minY, maxX+1, maxY+1)
	// the middle of the body: half of the opaque pixels of the middle rows lie left of it
	count := make([]int, w)
	n := 0
	for y := minY + (maxY-minY)/4; y <= minY+(maxY-minY)*3/4; y++ {
		for x := range w {
			if opaque(x, y) {
				count[x]++
				n++
			}
		}
	}
	acc := 0
	for x := range w {
		if acc += count[x]; acc*2 >= n {
			f.BodyX = x
			break
		}
	}
	return f
}
