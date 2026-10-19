//go:build !js

package assets

import (
	"image"
	"io"
)

// decodeImage decodes the picture read from r with Go's decoders.
func decodeImage(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}
