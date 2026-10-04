// Package assets reads the game data under the assets directory: the artwork of the
// screens (ui/), the gummy blocks (blocks/), the fonts (fonts/) and the characters
// (characters/). The game embeds the directory and hands it over with Use; the tests read
// it from disk.
package assets

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // the illustrations and most artwork
	_ "image/png"  // the portraits and the pieces with transparency
	"io/fs"
	"log"
	"path"

	"golang.org/x/image/draw"
)

// root is the assets directory the game reads (Use).
var root fs.FS

// Use makes fsys the assets directory: its root holds ui/, blocks/, fonts/ and characters/.
func Use(fsys fs.FS) { root = fsys }

// FS is the assets directory given to Use.
func FS() fs.FS { return root }

// ReadFile reads the file name from the assets directory.
func ReadFile(name string) ([]byte, error) { return fs.ReadFile(root, name) }

// imageExts lists the image file extensions tried in order when looking up artwork.
var imageExts = []string{".jpg", ".png"}

// HasImage reports whether the picture stem.jpg or stem.png is in fsys (false without one).
func HasImage(fsys fs.FS, stem string) bool {
	if fsys == nil {
		return false
	}
	for _, ext := range imageExts {
		if _, err := fs.Stat(fsys, stem+ext); err == nil {
			return true
		}
	}
	return false
}

// ToRGBA returns img as premultiplied RGBA, the form ebiten.NewImageFromImage uploads as
// it is. Any other form (a JPEG's YCbCr, a PNG's NRGBA) it converts first, the same way
// (draw.Src into RGBA), on the goroutine that calls it: done on the main goroutine, that
// took 2 to 6 ms the frame a picture first showed. The decoders call it, off the main
// goroutine, so uploading is only a copy.
func ToRGBA(img image.Image) image.Image {
	if p, ok := img.(*image.RGBA); ok && p.Rect.Min == (image.Point{}) && p.Stride == 4*p.Rect.Dx() {
		return p
	}
	b := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// LogBroken logs err when a picture is there but does not decode. A missing picture is not
// logged: the game expects some to be missing and shows a stand-in.
func LogBroken(name string, err error) {
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("cannot decode %s: %v", name, err)
	}
}

// Decode decodes the picture stem.jpg or stem.png in fsys, trying them in that order. A
// file that does not decode is passed over for the next one too; the error wraps
// fs.ErrNotExist when there is no file at all.
func Decode(fsys fs.FS, stem string) (image.Image, error) {
	var errs []error
	for _, ext := range imageExts {
		img, err := DecodeFile(fsys, stem+ext)
		if err == nil {
			return img, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("%s: %w", path.Base(stem+ext), err))
		}
	}
	if len(errs) == 0 {
		return nil, fs.ErrNotExist
	}
	return nil, errors.Join(errs...)
}

// DecodeFile decodes the picture name in fsys. It reads the file as it goes: reading it
// whole first would copy an embedded file. A missing file (or no fsys) gives an error
// wrapping fs.ErrNotExist.
func DecodeFile(fsys fs.FS, name string) (image.Image, error) {
	if fsys == nil {
		return nil, fs.ErrNotExist
	}
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	return img, errors.Join(err, f.Close())
}
