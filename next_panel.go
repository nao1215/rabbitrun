package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"path"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// cropKind selects which part of a standing portrait to cut out.
type cropKind int

const (
	cropFace cropKind = iota // a square around the face
	cropWide                 // head and shoulders, in a wide box
)

// portraitBox finds a box of the given kind in a standing portrait. The head is
// the topmost opaque part, so each box starts just under the top of the figure,
// centered on the opaque pixels of the head rows. Only the alpha of the RGBA
// pixels is read. It returns false when the image is empty.
func portraitBox(pix []byte, w, h int, kind cropKind) (image.Rectangle, bool) {
	opaque := func(x, y int) bool { return pix[(y*w+x)*4+3] > 128 }
	top := -1
	for y := 0; y < h && top < 0; y++ {
		for x := range w {
			if opaque(x, y) {
				top = y
				break
			}
		}
	}
	if top < 0 {
		return image.Rectangle{}, false
	}
	// Something thin sticking up a long way above the head (a bunny's ears) is not the
	// head: the head starts at the first row about as wide as a head.
	for y := top; y < min(h, top+h/6); y++ {
		n := 0
		for x := range w {
			if opaque(x, y) {
				n++
			}
		}
		if n >= w*13/100 {
			if y-top > h*3/100 {
				top = y - h/100
			}
			break
		}
	}
	side := h * 13 / 100 // a head is about an eighth of a full-body figure
	sum, n := 0, 0
	for y := top; y < min(h, top+side*6/10); y++ {
		for x := range w {
			if opaque(x, y) {
				sum += x
				n++
			}
		}
	}
	cx := sum / n
	if kind == cropWide {
		// Head and shoulders in a wide box.
		// The box stays centered on the head even past the image edges; that part is transparent.
		// Tighter than the character frame shows it, so the face reads larger here.
		bh := h * 13 / 100
		bw := bh * 23 / 10
		y0 := max(0, top-h/200)
		return image.Rect(cx-bw/2, y0, cx-bw/2+bw, min(h, y0+bh)), true
	}
	x0 := min(max(0, cx-side/2), max(0, w-side))
	y0 := max(0, top+side/12)
	return image.Rect(x0, y0, min(w, x0+side), min(h, y0+side)), true
}

// faceCrops holds the face close-ups cut from the full-resolution portraits.
// Decoding a portrait takes a few frames, so it runs in the background; until a
// crop is ready the panel keeps showing the previous one.
var faceCrops = struct {
	sync.Mutex
	ready   map[string]*image.RGBA   // decoded crops waiting to be uploaded to the GPU
	images  map[string]*ebiten.Image // uploaded crops (nil when the portrait has no image)
	loading map[string]bool
}{ready: map[string]*image.RGBA{}, images: map[string]*ebiten.Image{}, loading: map[string]bool{}}

// cropPortrait decodes the portrait at full resolution and cuts out the part of the given kind.
func cropPortrait(e *ImageEntry, kind cropKind) *image.RGBA {
	for _, ext := range imageExts {
		raw, err := assetFS.ReadFile(path.Join(e.base, "images", e.ID+ext))
		if err != nil {
			continue
		}
		src, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil
		}
		b := src.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
		r, ok := portraitBox(rgba.Pix, b.Dx(), b.Dy(), kind)
		if !ok {
			return nil
		}
		// Parts of r outside the portrait stay transparent.
		face := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
		draw.Draw(face, face.Bounds(), rgba, r.Min, draw.Src)
		return face
	}
	return nil
}

// faceOf returns the face close-up of a portrait, or nil while it is still being
// prepared (or when the portrait has no image).
func faceOf(e *ImageEntry) *ebiten.Image { return portraitCrop(e, cropFace) }

func portraitCrop(e *ImageEntry, kind cropKind) *ebiten.Image {
	if e == nil {
		return nil
	}
	id := fmt.Sprintf("%s/%s#%d", e.base, e.ID, kind)
	faceCrops.Lock()
	defer faceCrops.Unlock()
	if img, ok := faceCrops.images[id]; ok {
		return img
	}
	if rgba, ok := faceCrops.ready[id]; ok {
		delete(faceCrops.ready, id)
		var img *ebiten.Image
		if rgba != nil {
			img = ebiten.NewImageFromImage(rgba)
		}
		faceCrops.images[id] = img
		return img
	}
	if !faceCrops.loading[id] {
		faceCrops.loading[id] = true
		if !e.HasImage() {
			faceCrops.images[id] = nil
			return nil
		}
		go func() {
			face := cropPortrait(e, kind)
			faceCrops.Lock()
			faceCrops.ready[id] = face
			faceCrops.Unlock()
		}()
	}
	return nil
}
