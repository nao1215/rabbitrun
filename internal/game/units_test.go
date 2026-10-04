package game

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/save"
)

// TestGlossyBlockShape renders the stand-in block drawn when a block picture is missing:
// see-through at the rounded corners, opaque and in its candy color in the middle, and
// lighter at the top than at the bottom.
func TestGlossyBlockShape(t *testing.T) {
	t.Parallel()
	for k, c := range kindColors {
		img := renderGlossyBlock(c, blockTex)
		if b := img.Bounds(); b.Dx() != blockTex || b.Dy() != blockTex {
			t.Fatalf("kind %d: size %v", k, b)
		}
		if a := img.RGBAAt(0, 0).A; a != 0 {
			t.Errorf("kind %d: the corner has alpha %d, want see-through", k, a)
		}
		mid := img.RGBAAt(blockTex/2, blockTex/2)
		if mid.A != 0xff {
			t.Errorf("kind %d: the middle has alpha %d, want opaque", k, mid.A)
		}
		if !dominant(mid, c) {
			t.Errorf("kind %d: the middle %v is not in the color %v", k, mid, c)
		}
		top, bottom := img.RGBAAt(blockTex/2, blockTex/5), img.RGBAAt(blockTex/2, blockTex*4/5)
		if luma(top) <= luma(bottom) {
			t.Errorf("kind %d: the top %v is not lighter than the bottom %v", k, top, bottom)
		}
	}
}

// dominant reports whether the strongest channel of want is also the strongest of got.
func dominant(got, want color.RGBA) bool {
	ch := func(c color.RGBA) int {
		switch {
		case c.R >= c.G && c.R >= c.B:
			return 0
		case c.G >= c.B:
			return 1
		}
		return 2
	}
	return ch(got) == ch(want)
}

func luma(c color.RGBA) int { return 299*int(c.R) + 587*int(c.G) + 114*int(c.B) }

func TestSmoothstepAndClamp(t *testing.T) {
	t.Parallel()
	cases := []struct{ a, b, x, want float64 }{
		{0, 1, -1, 0},
		{0, 1, 0, 0},
		{0, 1, 0.5, 0.5},
		{0, 1, 1, 1},
		{0, 1, 2, 1},
		{2, 4, 3, 0.5},
	}
	for _, tc := range cases {
		if got := smoothstep(tc.a, tc.b, tc.x); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("smoothstep(%v, %v, %v) = %v, want %v", tc.a, tc.b, tc.x, got, tc.want)
		}
	}
	for x, want := range map[float64]float64{-0.5: 0, 0.25: 0.25, 1.5: 1} {
		if got := clamp01(x); got != want {
			t.Errorf("clamp01(%v) = %v, want %v", x, got, want)
		}
	}
}

// TestWritePNG writes a picture and reads it back; a directory that is not there fails.
func TestWritePNG(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{0x10, 0x20, 0x30, 0xff})
	p := filepath.Join(t.TempDir(), "shot.png")
	if err := writePNG(p, img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p) //nolint:gosec // G304: a file under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != img.Bounds() {
		t.Fatalf("read back %v, want %v", got.Bounds(), img.Bounds())
	}
	if r, g, b, _ := got.At(1, 1).RGBA(); r>>8 != 0x10 || g>>8 != 0x20 || b>>8 != 0x30 {
		t.Errorf("pixel (1,1) is %x %x %x", r>>8, g>>8, b>>8)
	}
	if err := writePNG(filepath.Join(t.TempDir(), "missing", "shot.png"), img); err == nil {
		t.Error("writing into a missing directory did not fail")
	}
}

// TestCaptureStateSetsUpEachStepOnce runs the capture's update: a step's screen is set up
// on its first frame only, and nothing happens past the last step.
func TestCaptureStateSetsUpEachStepOnce(t *testing.T) { //nolint:paralleltest // swaps the capture steps
	old := captureSteps
	t.Cleanup(func() { captureSteps = old })
	calls := 0
	captureSteps = []captureStep{{name: "only", setup: func(*Game) { calls++ }, wait: 3}}
	c := &captureState{}
	g := &Game{}
	for range 3 {
		c.update(g)
	}
	if calls != 1 || c.frame != 3 {
		t.Fatalf("set up %d times over %d frames, want once over 3", calls, c.frame)
	}
	c.step = len(captureSteps)
	c.update(g)
	if calls != 1 || c.frame != 3 {
		t.Errorf("past the last step: set up %d times, frame %d", calls, c.frame)
	}
	if !c.afterDraw(nil) {
		t.Error("past the last step the capture is not over")
	}
}

// TestRecorderTakesEveryOtherUpdate holds the game until the frame before was drawn, so
// the video gets each frame of the game.
func TestRecorderTakesEveryOtherUpdate(t *testing.T) {
	t.Parallel()
	r := &recorder{drawn: true}
	if r.skipUpdate() {
		t.Fatal("the first update after a draw was skipped")
	}
	if !r.skipUpdate() {
		t.Fatal("a second update before the draw was not skipped")
	}
	if r.frame != 1 {
		t.Errorf("frame %d, want 1", r.frame)
	}
}

// TestBeatBounce squashes her to the beat only in a cheerful mood while lively music plays.
func TestBeatBounce(t *testing.T) { //nolint:paralleltest // swaps the music's beat
	old := beatPhase
	t.Cleanup(func() { beatPhase = old })
	s := &playScene{expr: character.ExprHappy, intensity: 2}
	beatPhase = func() (float64, bool) { return 0, false }
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v with no music", b)
	}
	beatPhase = func() (float64, bool) { return 0.1, true }
	if b := s.bounce(); b <= 0 || b > 0.016 {
		t.Errorf("bounce %v in a happy mood with lively music", b)
	}
	s.expr = character.ExprNervous
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v in a nervous mood", b)
	}
	s.expr, s.intensity = character.ExprHappy, 0
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v with calm music", b)
	}
}

func TestPoseInterval(t *testing.T) {
	t.Parallel()
	if poseInterval(character.ExprGameOver) < 60*60 {
		t.Error("she does not stay down on the game over")
	}
	restless := []string{character.ExprCrying, character.ExprPanic, character.ExprNervous, character.ExprNormal}
	for i := 1; i < len(restless); i++ {
		if poseInterval(restless[i-1]) >= poseInterval(restless[i]) {
			t.Errorf("%s changes poses no faster than %s", restless[i-1], restless[i])
		}
	}
}

// TestNewSetsUpACapture starts the game as --capture does: on the title, the save
// read-only (its scripted runs must not change the player's save), the debug switch as
// asked, and the capture writing into its directory.
func TestNewSetsUpACapture(t *testing.T) { //nolint:paralleltest // sets up the package-level game state
	useTempConfig(t)
	oldChars, oldBg := characters, bg
	t.Cleanup(func() { characters, bg = oldChars, oldBg })
	if err := Load(assets.FS()); err != nil {
		t.Fatal(err)
	}
	if ids := CharacterIDs(); len(ids) != len(characters) || ids[defaultCharIndex()] != heroID {
		t.Fatalf("character IDs %v", ids)
	}
	oldDebug := debugMode
	t.Cleanup(func() { debugMode = oldDebug })
	dir := t.TempDir()
	g, err := New(Options{CaptureDir: dir, Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := g.scene.(*titleScene); !ok || g.cap == nil || g.cap.dir != dir || g.rec != nil {
		t.Fatalf("scene %T, capture %+v, recorder %v", g.scene, g.cap, g.rec)
	}
	if !store.ReadOnly || !debugMode {
		t.Fatalf("read-only %v, debug %v", store.ReadOnly, debugMode)
	}
	progress(heroID).Cleared = true
	store.Mark()
	g.Close()
	if _, err := os.Stat(save.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a capture wrote the save: %v", err)
	}
	if w, h := g.Layout(1, 1); w != ScreenW || h != ScreenH {
		t.Fatalf("layout %dx%d", w, h)
	}
}

// TestLoadNeedsTheFontAndCharacters: an assets directory without the font or without any
// character cannot start the game.
func TestLoadNeedsTheFontAndCharacters(t *testing.T) { //nolint:paralleltest // sets the package-level assets
	t.Cleanup(func() { assets.Use(os.DirFS("../../assets")) })
	if err := loadAssets(fstest.MapFS{}); err == nil {
		t.Fatal("no font loaded")
	}
	regular, err := fs.ReadFile(os.DirFS("../../assets"), "fonts/mplus-1p-regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	fonts := fstest.MapFS{"fonts/mplus-1p-regular.ttf": {Data: regular}}
	if err := loadAssets(fonts); err == nil {
		t.Fatal("loaded without a characters directory")
	}
	fonts["characters/x/game.json"] = &fstest.MapFile{Data: []byte(`{"id":"x"}`)}
	if err := loadAssets(fonts); err == nil || !strings.Contains(err.Error(), "no characters") {
		t.Fatalf("loaded without a playable character: %v", err)
	}
	fonts["fonts/mplus-1p-regular.ttf"] = &fstest.MapFile{Data: []byte("not a font")}
	if err := loadAssets(fonts); err == nil {
		t.Fatal("loaded a broken font")
	}
}
