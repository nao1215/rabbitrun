//go:build pixels

package game

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/assets"
)

// TestPixelsTitleShowsItsReward draws the title once every character has cleared both
// stages, with a plain blue picture served as its reward: the picture covers the screen
// behind the logo and the menu, in the regular stages and in the extra ones.
//
//nolint:paralleltest // shares the save data and the characters
func TestPixelsTitleShowsItsReward(t *testing.T) {
	var buf bytes.Buffer
	blue := image.NewNRGBA(image.Rect(0, 0, 720, 1280)) // the size of the title art
	for i := 0; i < len(blue.Pix); i += 4 {
		copy(blue.Pix[i:], []byte{0, 0, 0xff, 0xff})
	}
	if err := png.Encode(&buf, blue); err != nil {
		t.Fatal(err)
	}
	old := assets.FS()
	// under the .jpg name, which is looked up before .png and is the name of the real picture
	// (image.Decode reads the PNG data by its content, not by the name)
	assets.Use(overlayFS{old, fstest.MapFS{"ui/" + titleCompleteArt + ".jpg": {Data: buf.Bytes()}}})
	assets.ReleaseUI(titleCompleteArt)
	t.Cleanup(func() {
		assets.Use(old)
		assets.ReleaseUI(titleCompleteArt)
	})
	g, screen := newDrawScenario(t, func() {
		store.Data.Announced = map[string]bool{secretID(): true}
		store.Data.WordTold = true
		store.Data.ExtraFound = true
		for _, c := range characters {
			p := progress(c.ID)
			p.Cleared, p.ClearedExtra = true, true
		}
	})
	isBlue := func(c color.RGBA) bool { return c.B > 0xe0 && c.R < 0x20 && c.G < 0x20 }
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*ScreenW + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	for _, extra := range []bool{false, true} {
		store.Data.ExtraMode = extra
		g.SetScene(newTitleScene())
		playDrawn(t, g, screen, wait(10))
		frame := settled(t, screen, g.Draw)
		for _, p := range []image.Point{{4, 4}, {ScreenW - 5, ScreenH - 5}} {
			if c := at(frame, p.X, p.Y); !isBlue(c) {
				t.Errorf("extra %v: the title shows %v at %v, not its picture", extra, c, p)
			}
		}
	}
}
