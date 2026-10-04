//go:build pixels

package game

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
)

// bluePicture is a plain blue PNG the size of the endings (896x1152).
func bluePicture(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 896, 1152))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:], []byte{0, 0, 0xff, 0xff})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPixelsEndingShowsTheNoMissPicture draws the ending of a no-miss clear with a plain
// blue picture served as her no-miss picture: it fills the screen between the top band
// (taller, for NO MISS!) and the bottom one. After a run with a miss the usual ending
// shows instead.
//
//nolint:paralleltest // shares the save data and the characters
func TestPixelsEndingShowsTheNoMissPicture(t *testing.T) {
	useAssets(t, filesOverFS{assets.FS(), noMissFiles(heroID, bluePicture(t))})
	isBlue := func(c color.RGBA) bool { return c.B > 0xe0 && c.R < 0x20 && c.G < 0x20 }
	at := func(pix []byte, x, y int) color.RGBA {
		i := 4 * (y*ScreenW + x)
		return color.RGBA{pix[i], pix[i+1], pix[i+2], pix[i+3]}
	}
	for _, missed := range []bool{false, true} {
		g, screen := noMissScenario(t, nil)
		s := startOnRoad(g)
		clearRun(t, s)
		if missed {
			s.misses = 1 // as if she had run into a wall on the way
		}
		playDrawn(t, g, screen, wait(endWordsAt+endBandFrames+5))
		frame := settled(t, screen, g.Draw)
		mid := at(frame, ScreenW/2, ScreenH/2)
		if isBlue(mid) == missed {
			t.Errorf("missed %v: the middle of the ending is %v", missed, mid)
		}
		// the band of NO MISS! reaches lower than the congratulation's alone
		band := at(frame, 4, (endBandH+endBandNoMissH)/2)
		if !missed && isBlue(band) {
			t.Errorf("no band under the congratulation for NO MISS! (%v)", band)
		}
	}
}
