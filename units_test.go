package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flag "github.com/spf13/pflag"
)

// TestSoundEffectsAreSynthesized synthesizes every sound effect: each is stereo float32
// audio of some length, within -1 and 1, not silent, and ends faded out (no click).
func TestSoundEffectsAreSynthesized(t *testing.T) {
	t.Parallel()
	var d [seCount][]byte
	synthEffects(&d)
	for id, b := range d {
		if len(b) == 0 || len(b)%8 != 0 {
			t.Errorf("effect %d: %d bytes, want whole stereo frames", id, len(b))
			continue
		}
		peak := 0.0
		for i := 0; i+8 <= len(b); i += 8 {
			l := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i:])))
			r := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i+4:])))
			if l != r {
				t.Fatalf("effect %d: left %v and right %v differ at frame %d", id, l, r, i/8)
			}
			if math.IsNaN(l) || l < -1 || l > 1 {
				t.Fatalf("effect %d: sample %v out of range at frame %d", id, l, i/8)
			}
			peak = math.Max(peak, math.Abs(l))
		}
		if peak < 0.01 {
			t.Errorf("effect %d is silent (peak %v)", id, peak)
		}
		last := math.Float32frombits(binary.LittleEndian.Uint32(b[len(b)-8:]))
		if math.Abs(float64(last)) > 0.01 {
			t.Errorf("effect %d ends on %v, not faded out", id, last)
		}
	}
}

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

// TestUsageListsTheOptions writes the help: it names the game, every option and where to
// report an issue.
func TestUsageListsTheOptions(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	writeUsage(&b)
	out := b.String()
	want := []string{"Rabbit Run", "Usage:", "https://github.com/nao1215/rabbitrun/issues"}
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		_, usage := flag.UnquoteUsage(f) // the name in backquotes is shown without them
		want = append(want, "--"+f.Name, usage)
	})
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("the help does not mention %q:\n%s", w, out)
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

// TestKeyboardPollingWithNothingPressed reads the real keyboard and pads (no script)
// with no key down: no action is held and nothing is typed.
func TestKeyboardPollingWithNothingPressed(t *testing.T) { //nolint:paralleltest // reads ebiten's input state
	var in Input
	in.Update()
	for a := Action(0); a < actionCount; a++ {
		if in.Held(a) || in.Pressed(a) {
			t.Errorf("action %d is held with no key down", a)
		}
	}
	if c := in.Chars(); len(c) != 0 {
		t.Errorf("typed %q with no key down", string(c))
	}
	if in.Side() != 0 {
		t.Errorf("side %d with no key down", in.Side())
	}
}

// TestBGMControlsWithoutMusic changes the music when none plays: nothing happens, and
// nothing fails.
func TestBGMControlsWithoutMusic(t *testing.T) { //nolint:paralleltest // uses the package-level music
	oldPlayer, oldBGM, oldMuted := bgmPlayer, bgm, audioMuted
	t.Cleanup(func() { bgmPlayer, bgm, audioMuted = oldPlayer, oldBGM, oldMuted })
	bgmPlayer, bgm, audioMuted = nil, nil, true
	bgmSong = titleSong
	startBGM(gameSong)
	if bgmSong != "" || bgm != nil {
		t.Errorf("muted, startBGM set song %q, stream %v", bgmSong, bgm)
	}
	pauseBGM(true)
	pauseBGM(false)
	setBGMState(2)
	setBGMTempo(1, 160)
	playSE(seMove)

	bgm = newMusicStream(songs[gameSong])
	setBGMState(2)
	if got := bgm.curBPMShared(); got != intensityBPM[2] {
		t.Errorf("tempo %v after setBGMState(2), want %v", got, intensityBPM[2])
	}
	setBGMTempo(1, 160)
	if got := bgm.curBPMShared(); got != 160 {
		t.Errorf("tempo %v after setBGMTempo, want 160", got)
	}
	if got := bgm.target.Load(); got != 1 {
		t.Errorf("intensity %d after setBGMTempo(1), want 1", got)
	}
}

// TestBeatBounce squashes her to the beat only in a cheerful mood while lively music plays.
func TestBeatBounce(t *testing.T) { //nolint:paralleltest // uses the package-level music
	old := bgm
	t.Cleanup(func() { bgm = old })
	s := &PlayScene{expr: ExprHappy, intensity: 2}
	bgm = nil
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v with no music", b)
	}
	bgm = newMusicStream(songs[gameSong])
	if _, ok := bgm.beatPhase(); ok {
		t.Error("a beat phase before the music played")
	}
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v before the music played", b)
	}
	if _, err := bgm.Read(make([]byte, 8*256)); err != nil {
		t.Fatal(err)
	}
	ph, ok := bgm.beatPhase()
	if !ok || ph < 0 || ph >= 1 {
		t.Fatalf("beat phase %v, %v after the music played", ph, ok)
	}
	if b := s.bounce(); b <= 0 || b > 0.016 {
		t.Errorf("bounce %v in a happy mood with lively music", b)
	}
	s.expr = ExprNervous
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v in a nervous mood", b)
	}
	s.expr, s.intensity = ExprHappy, 0
	if b := s.bounce(); b != 0 {
		t.Errorf("bounce %v with calm music", b)
	}
}

func TestPoseInterval(t *testing.T) {
	t.Parallel()
	if poseInterval(ExprGameOver) < 60*60 {
		t.Error("she does not stay down on the game over")
	}
	restless := []string{ExprCrying, ExprPanic, ExprNervous, ExprNormal}
	for i := 1; i < len(restless); i++ {
		if poseInterval(restless[i-1]) >= poseInterval(restless[i]) {
			t.Errorf("%s changes poses no faster than %s", restless[i-1], restless[i])
		}
	}
}

// TestMissingPortraitGetsAPlaceholder loads a portrait whose picture is not there: it
// shows the stand-in, which says its label (or its ID without one).
func TestMissingPortraitGetsAPlaceholder(t *testing.T) { //nolint:paralleltest // needs the fonts (newDrawScenario)
	newDrawScenario(t, nil)
	e := &ImageEntry{ID: "nothing", base: "assets/characters/nobody"}
	if e.HasImage() {
		t.Fatal("a missing picture is reported as there")
	}
	if e.name() != "nothing" {
		t.Errorf("name %q without a label, want the ID", e.name())
	}
	e.Label = "Nobody"
	if e.name() != "Nobody" {
		t.Errorf("name %q with a label, want the label", e.name())
	}
	img := e.Img()
	if img == nil || img.Bounds().Dx() != 896 || img.Bounds().Dy() != 1120 {
		t.Fatalf("placeholder %v, want the 896x1120 stand-in", img)
	}
	if e.Img() != img {
		t.Error("the placeholder was made again")
	}
	e.ReleaseImg()
	if e.Image != nil {
		t.Error("ReleaseImg kept the placeholder")
	}
	if loadCharImage("assets/characters/nobody", "nothing", "x") == nil {
		t.Error("loadCharImage gave no stand-in for a missing picture")
	}
}
