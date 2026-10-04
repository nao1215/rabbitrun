//go:build pixels

package game

import (
	"image"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/character"
)

// The pixel tests read back what the screens draw, which ebiten allows only once the game
// loop runs: with the pixels build tag the tests run inside it, in a window (an X server
// such as xvfb on Linux), as ebiten tests itself:
//
//	xvfb-run go test -tags pixels -run Pixels ./internal/game/   (or make test-pixels)
//
// They check the structure of a frame, not its art, so a new picture does not break
// them: the screen is drawn again with a part of it left out, and that part must change
// the frame. A screen that drew nothing over the background, or a character hidden under
// a layer drawn after her (or not drawn at all), leaves the frame as it was.

// loopGame runs the tests in the first frame of the game loop.
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

func runTests(m *testing.M) int {
	g := &loopGame{m: m, code: 1}
	ebiten.SetWindowSize(64, 64)
	ebiten.SetWindowTitle("rabbitrun pixel tests")
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{InitUnfocused: true}); err != nil {
		panic(err)
	}
	return g.code
}

// grab draws the frame with draw onto screen and reads it back.
func grab(screen *ebiten.Image, draw func(*ebiten.Image)) []byte {
	screen.Clear()
	draw(screen)
	pix := make([]byte, 4*ScreenW*ScreenH)
	screen.ReadPixels(pix)
	return pix
}

// settled grabs the frame once drawing it again gives the same picture: a pose still
// being decoded in the background shows the one before it until it is ready.
func settled(t *testing.T, screen *ebiten.Image, draw func(*ebiten.Image)) []byte {
	t.Helper()
	frame := grab(screen, draw)
	for range 100 {
		next := grab(screen, draw)
		if changed(frame, next) == 0 {
			return frame
		}
		frame = next
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the frame keeps changing although the game does not move on")
	return nil
}

// changed is the share (0-1) of the pixels that differ between two frames by more than
// a slight rounding in some channel.
func changed(a, b []byte) float64 { return changedIn(a, b, image.Rect(0, 0, ScreenW, ScreenH)) }

// changedIn is changed within r.
func changedIn(a, b []byte, r image.Rectangle) float64 {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := 4 * (y*ScreenW + x)
			for c := range 3 {
				if d := int(a[i+c]) - int(b[i+c]); d > 8 || d < -8 {
					n++
					break
				}
			}
		}
	}
	return float64(n) / float64(r.Dx()*r.Dy())
}

// pictureKind is a kind of character picture a check leaves out of the frame.
type pictureKind int

const (
	portraits     pictureKind = iota // her poses on the play screen (and the select image)
	illustrations                    // the illustrations behind the road and the ending's picture (or the no-miss one)
)

// entries are the pictures of kind of every character.
func (k pictureKind) entries() []*character.ImageEntry {
	var out []*character.ImageEntry
	for _, c := range characters {
		var list []character.ImageEntry
		var single []*character.ImageEntry
		switch k {
		case portraits:
			list, single = c.Expressions, []*character.ImageEntry{c.Select}
		case illustrations:
			list, single = c.CGs, []*character.ImageEntry{c.Ending, c.EndingExtra, c.NoMiss, c.NoMissExtra}
		}
		for i := range list {
			out = append(out, &list[i])
		}
		for _, e := range single {
			if e != nil {
				out = append(out, e)
			}
		}
	}
	return out
}

// withBlank draws with the pictures of kind swapped for a transparent one, as if they
// were not drawn at all.
func withBlank(g *Game, kind pictureKind, draw func()) {
	entries := kind.entries()
	blank := ebiten.NewImage(1, 1) // Img and Full both give Image when it is set
	saved := make([]*ebiten.Image, len(entries))
	for i, e := range entries {
		saved[i], e.Image = e.Image, blank
	}
	s, inPlay := g.scene.(*playScene)
	var shown *ebiten.Image
	if inPlay {
		shown, s.shownPic = s.shownPic, nil
	}
	draw()
	if inPlay {
		s.shownPic = shown
	}
	for i, e := range entries {
		e.Image = saved[i]
	}
}

// TestPixelsScreensShowWhatTheyDraw takes the screens the capture takes of the title, play,
// a miss, the ending and the game over, and checks that each draws over the background
// and that the character's pictures it shows (her portrait, the ending's picture) are
// seen in the frame.
//
//nolint:paralleltest // shares the save data and the characters
func TestPixelsScreensShowWhatTheyDraw(t *testing.T) {
	g, screen := newDrawScenario(t, unlockEverything)
	steps := map[string]captureStep{}
	for _, st := range captureSteps {
		steps[st.name] = st
	}
	frameRect := image.Rect(int(frameX), int(frameY), int(frameX+frameW), int(frameY+frameH))
	whole := image.Rect(0, 0, ScreenW, ScreenH)
	type seen struct {
		kind  pictureKind
		where image.Rectangle // the part of the screen they are on
		min   float64         // the share of it they must change
	}
	for _, tc := range []struct {
		step  string
		shows []seen // the pictures that must be seen on this screen
	}{
		{step: "title"},
		{step: "play", shows: []seen{{portraits, frameRect, 0.1}}},
		{step: "play_miss", shows: []seen{{portraits, frameRect, 0.1}}},
		{step: "allclear", shows: []seen{{illustrations, whole, 0.5}}},
		{step: "gameover", shows: []seen{{portraits, whole, 0.03}}},
	} {
		t.Run(tc.step, func(t *testing.T) {
			st := steps[tc.step]
			st.setup(g)
			playDrawn(t, g, screen, wait(st.wait))
			frame := settled(t, screen, g.Draw)
			scene := changed(frame, grab(screen, g.bg.draw))
			t.Logf("the screen changes %.1f%% of the background", 100*scene)
			if scene < 0.05 {
				t.Errorf("the screen changed only %.2f%% of the background", 100*scene)
			}
			for _, want := range tc.shows {
				var blank []byte
				withBlank(g, want.kind, func() { blank = grab(screen, g.Draw) })
				got := changedIn(frame, blank, want.where)
				t.Logf("pictures %d change %.1f%% of %v", want.kind, 100*got, want.where)
				if got < want.min {
					t.Errorf("pictures %d change only %.2f%% of %v, want %.0f%% or more", want.kind, 100*got, want.where, 100*want.min)
				}
			}
		})
	}
}
