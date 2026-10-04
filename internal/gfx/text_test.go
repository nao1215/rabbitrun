package gfx

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// readFont reads a font of the game from the assets directory.
func readFont(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "assets", "fonts", name)) //nolint:gosec // G304: a font of the game, named by the tests
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// useFonts loads the game's fonts, as the game does at launch.
func useFonts(t *testing.T) {
	t.Helper()
	if err := LoadFonts(readFont(t, "mplus-1p-regular.ttf"), readFont(t, "LilitaOne-Regular.ttf")); err != nil {
		t.Fatal(err)
	}
}

func TestIsASCII(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{"": true, "RABBIT RUN 1,234": true, "~": true, "\x7f": false, "スコア": false, "café": false}
	for in, want := range cases {
		if got := isASCII(in); got != want {
			t.Errorf("isASCII(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestLoadFonts: the regular font is needed, the pop font is not.
func TestLoadFonts(t *testing.T) { //nolint:paralleltest // sets the package-level fonts
	if err := LoadFonts([]byte("not a font"), nil); err == nil {
		t.Fatal("a broken regular font loaded")
	}
	regular := readFont(t, "mplus-1p-regular.ttf")
	popSource = nil
	if err := LoadFonts(regular, []byte("not a font")); err != nil || popSource != nil {
		t.Fatalf("a broken pop font: err %v, pop font %v (want it left out)", err, popSource)
	}
	if err := LoadFonts(regular, nil); err != nil || regularSource == nil {
		t.Fatalf("no pop font: err %v", err)
	}
	useFonts(t)
	if popSource == nil {
		t.Fatal("the pop font did not load")
	}
}

// TestFaceTakesThePopFontForLatinText: ASCII-only text is set in the pop font, any other
// in the regular one, and each face is made once per size.
func TestFaceTakesThePopFontForLatinText(t *testing.T) { //nolint:paralleltest // uses the package-level fonts
	useFonts(t)
	clear(faceCache)
	pop, jp := Face(20, "RABBIT"), Face(20, "うさぎ")
	if pop.Source != popSource || jp.Source != regularSource {
		t.Fatal("the faces are not in the fonts they should be")
	}
	if Face(20, "RUN") != pop || Face(30, "RUN") == pop {
		t.Fatal("faces are not kept per size")
	}
}

// TestDrawingHelpers draws with every helper into an offscreen image. The pixels cannot be
// read back before the game loop starts, so it checks that each goes through and that the
// rounded mask is made once per size.
func TestDrawingHelpers(t *testing.T) { //nolint:paralleltest // uses the package-level fonts and caches
	useFonts(t)
	dst := ebiten.NewImage(240, 300)
	pic := ebiten.NewImage(60, 120)
	FillRoundRect(dst, 10, 10, 100, 80, 12, PanelFill)
	StrokeRoundRect(dst, 10, 10, 100, 80, 12, 3, CandyPink)
	DrawImageFit(dst, pic, 0, 0, 100, 100, 1)
	DrawImageCover(dst, pic, 0, 0, 100, 100, 0.5)
	DrawImageCoverTop(dst, pic, 0, 0, 100, 40, 1)
	DrawText(dst, "RABBIT RUN", 120, 20, 24, TextMain)
	DrawTextOutline(dst, "うさぎ", 120, 60, 24, CandyPink)
	DrawTextOutlineColor(dst, "GO", 120, 100, 24, color.White, DarkOutline, 0.5)
	DrawMenuAt(dst, []string{"PLAY", "GALLERY", "EXIT"}, 1, 120, 140, 20)
	DrawMenuOn(dst, []string{"RETRY", "GIVE UP"}, 0, 120, 220, 20, color.White, DarkOutline)
	clear(maskCache)
	a, b := ebiten.NewImage(50, 50), ebiten.NewImage(50, 50)
	if RoundedMask(a, 8) != a || RoundedMask(b, 8) != b {
		t.Fatal("RoundedMask does not round the picture in place")
	}
	if len(maskCache) != 1 {
		t.Fatalf("%d masks for one size", len(maskCache))
	}
	if dst.Bounds().Dx() != 240 || dst.Bounds().Dy() != 300 {
		t.Fatalf("the image changed size: %v", dst.Bounds())
	}
}
