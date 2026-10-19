//go:build pixels

package gfx

import (
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// The pixel tests of gfx read back what is drawn, which ebiten allows only once the game
// loop runs: with the pixels build tag the tests run inside it, in a window (xvfb-run on
// Linux without a desktop; make test-pixels).

type loopGame struct {
	m    *testing.M
	code int
}

func (g *loopGame) Update() error {
	g.code = g.m.Run()
	return ebiten.Termination
}

func (*loopGame) Draw(*ebiten.Image) {}

func (*loopGame) Layout(int, int) (int, int) { return 64, 64 }

func TestMain(m *testing.M) {
	g := &loopGame{m: m, code: 1}
	ebiten.SetWindowSize(64, 64)
	ebiten.SetWindowTitle("rabbitrun gfx pixel tests")
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{InitUnfocused: true}); err != nil {
		panic(err)
	}
	os.Exit(g.code)
}

// TestPixelsOutlinedTextIsKeptAsDrawn: the outlined text drawn from its kept image is the
// text drawn letter by letter, to the pixel: over a busy background, at whole and
// fractional places, in the sizes and colors the screens use. Composing the layers in an
// image of their own first rounds the edges of the letters in 8 bits once more: a
// channel may move by two steps of 255 (a thousandth of the channels did), invisibly; a
// letter cut off at the edge of the kept image would differ by far more.
func TestPixelsOutlinedTextIsKeptAsDrawn(t *testing.T) { //nolint:paralleltest // uses the package-level fonts and caches
	useFonts(t)
	const w, h = 720, 900
	bg := ebiten.NewImage(w, h)
	pix := make([]byte, 4*w*h)
	for i := range w * h {
		x, y := i%w, i/w
		pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = byte(x*7+y), byte(y*3), byte(x^y), 0xff //nolint:gosec // G115: a pattern that wraps on purpose
	}
	bg.WritePixels(pix)
	direct, kept := ebiten.NewImage(w, h), ebiten.NewImage(w, h)
	for _, dst := range []*ebiten.Image{direct, kept} {
		dst.DrawImage(bg, nil)
	}
	texts := []struct {
		s             string
		x, y, size    float64
		fill, outline color.Color
	}{
		{"STAGE 1-1", 200, 14, 34, CandyPink, color.White},
		{"0/100", 410.5, 14.25, 34, CandyPink, color.White},
		{"x3", 680.3, 18.7, 30, TextMain, color.White},
		{"CONTINUE", 360, 330, 44 * 1.15, CandyPink, color.White},
		{"RESET", 360, 404.8, 44, TextMain, color.White},
		{"RETRY", 360, 460, 40, color.NRGBA{0xee, 0xe6, 0xf2, 0xff}, DarkOutline},
		{"A NEW CHARACTER HAS COME!", 360, 600.5, 36, CandyPink, DarkOutline},
		{"ゲーム", 120.75, 750.5, 40, TextMain, color.White},
	}
	for _, tx := range texts {
		drawTextOutline(direct, tx.s, tx.x, tx.y, tx.size, tx.fill, tx.outline, 1)
		DrawTextOutlineColor(kept, tx.s, tx.x, tx.y, tx.size, tx.fill, tx.outline, 1) // made and kept
	}
	kept.DrawImage(bg, nil) // drawn again, from the kept images
	for _, tx := range texts {
		DrawTextOutlineColor(kept, tx.s, tx.x, tx.y, tx.size, tx.fill, tx.outline, 1)
	}
	a, b := make([]byte, 4*w*h), make([]byte, 4*w*h)
	direct.ReadPixels(a)
	kept.ReadPixels(b)
	differ, worst := 0, 0
	for i := range a {
		d := int(a[i]) - int(b[i])
		d = max(d, -d)
		if d > 0 {
			differ++
		}
		worst = max(worst, d)
	}
	t.Logf("channels that differ: %d of %d, by at most %d", differ, len(a), worst)
	if worst > 2 {
		t.Fatalf("the kept outlined text differs from the drawn one by %d steps in a channel", worst)
	}
}
