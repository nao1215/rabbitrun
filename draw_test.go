package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// The drawing tests run every screen into an offscreen image. Without RunGame the pixels
// cannot be read back (ReadPixels panics before the game loop starts), so they check the
// state the screens were driven into and that drawing each of them goes through.

// drawEvery is how often (in frames) the drawing tests draw the screen. Drawing is the
// slow part under the race detector; every few frames still passes through each stage
// of the animations.
const drawEvery = 8

var (
	loadFontsOnce sync.Once
	// drawChars are the characters of the drawing tests, loaded once and shared like the
	// game shares them between its screens: the pictures decoded for one test (the slow
	// part, cached on the entries) serve the next.
	drawChars []*Character
)

// newDrawScenario is newScenario with what drawing needs: the fonts and the blocks
// (loadAssets and initBlocks, as main does), and an offscreen screen.
func newDrawScenario(t *testing.T, prepare func()) (*Game, *ebiten.Image) {
	t.Helper()
	loadFontsOnce.Do(func() {
		old := characters
		loadAssets()
		drawChars, characters = characters, old
		initBlocks()
	})
	g := newScenarioWith(t, drawChars, prepare)
	return g, ebiten.NewImage(ScreenW, ScreenH)
}

// playDrawn is play that also draws the screen every drawEvery frames and on the last.
func playDrawn(t *testing.T, g *Game, screen *ebiten.Image, frames []scriptFrame) {
	t.Helper()
	g.in.script = &script{frames: frames}
	for f := range frames {
		if err := g.Update(); err != nil {
			t.Fatalf("the game ended: %v", err)
		}
		if f%drawEvery == 0 || f == len(frames)-1 {
			g.Draw(screen)
		}
	}
}

// playUntil runs the game, drawing it, until done reports true; it fails after limit frames.
func playUntil(t *testing.T, g *Game, screen *ebiten.Image, limit int, done func() bool) {
	t.Helper()
	g.in.script = &script{}
	for f := 0; !done(); f++ {
		if f >= limit {
			t.Fatalf("not done after %d frames (on %T)", limit, g.scene)
		}
		if err := g.Update(); err != nil {
			t.Fatalf("the game ended: %v", err)
		}
		if f%drawEvery == 0 {
			g.Draw(screen)
		}
	}
	g.Draw(screen)
}

// unlockEverything opens every portrait and illustration of every character, as after
// long play, so the gallery has pictures to show.
func unlockEverything() {
	for _, c := range characters {
		p := progress(c.ID)
		for _, e := range c.Expressions {
			p.SeenExpr[e.ID] = true
		}
		for _, e := range c.CGs {
			p.UnlockedCG[e.ID] = true
		}
	}
}

// TestDrawEveryCaptureStep sets up each screen the capture takes (captureSteps lists every
// state of every screen), runs it for as long as the capture waits and draws it on the
// way. Each must be on the screen it names when its picture would be taken.
//
//nolint:paralleltest // shares the save data and the characters
func TestDrawEveryCaptureStep(t *testing.T) {
	g, screen := newDrawScenario(t, unlockEverything)
	for _, st := range captureSteps {
		t.Run(st.name, func(t *testing.T) {
			st.setup(g)
			playDrawn(t, g, screen, wait(st.wait))
			switch {
			case strings.HasPrefix(st.name, "title"):
				titleOf(t, g)
			case st.name == "select":
				if _, ok := g.scene.(*CharSelectScene); !ok {
					t.Fatalf("got %T, want the select screen", g.scene)
				}
			case strings.HasPrefix(st.name, "gallery"):
				s, ok := g.scene.(*GalleryScene)
				if !ok {
					t.Fatalf("got %T, want the gallery", g.scene)
				}
				if want := st.name == "gallery_cg"; s.viewing != want {
					t.Errorf("viewing %v, want %v", s.viewing, want)
				}
			case st.name == "allclear":
				if s := playOf(t, g); !s.allClear || s.overFrame < endWordsAt {
					t.Errorf("all clear %v at frame %d: the ending is not up", s.allClear, s.overFrame)
				}
			case strings.HasPrefix(st.name, "gameover"):
				if s := playOf(t, g); !s.eng.Over() || s.allClear {
					t.Errorf("over %v, all clear %v: want the game over", s.eng.Over(), s.allClear)
				}
			case st.name == "play_miss":
				if s := playOf(t, g); !s.eng.G.Missed {
					t.Error("no miss on the wall across the road")
				}
			default:
				playOf(t, g)
			}
		})
	}
}
