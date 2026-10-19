package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/save"
)

// tapAt taps the screen at (x, y), then lets go for a frame.
func tapAt(x, y float64) []scriptFrame {
	return []scriptFrame{{tap: &[2]float64{x, y}}, {}}
}

// playEveryFrame is play that draws the screen after every frame: a tap finds the menu
// drawn on the frame before.
func playEveryFrame(t *testing.T, g *Game, screen *ebiten.Image, frames []scriptFrame) {
	t.Helper()
	g.in.SetScript(&script{frames: frames})
	for range frames {
		if err := g.Update(); err != nil {
			t.Fatalf("the game ended: %v", err)
		}
		g.Draw(screen)
	}
}

func TestMenuItemAt(t *testing.T) { //nolint:paralleltest // sets the menu noted for taps, which drawing sets too
	noteMenu(3, ScreenW/2, 600, 44)
	defer noteMenu(0, 0, 0, 0)
	cases := []struct {
		x, y float64
		want int
	}{
		{360, 610, 0},
		{360, 600 + 44*1.7 + 20, 1},
		{360, 600 + 2*44*1.7 + 40, 2},
		{360, 600 + 3*44*1.7 + 20, -1}, // under the last item
		{360, 560, -1},                 // over the first
		{40, 610, -1},                  // far beside
	}
	for _, c := range cases {
		if got := menuItemAt(c.x, c.y); got != c.want {
			t.Errorf("menuItemAt(%v, %v) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
}

// TestTapChoosesATitleItem: a tap on GALLERY opens the gallery; a tap beside the menu does
// nothing (it does not choose PLAY, which is selected).
//
//nolint:paralleltest // the scenario uses the package-level game state
func TestTapChoosesATitleItem(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	playEveryFrame(t, g, screen, steps(wait(2), tapAt(ScreenW/2, 300), wait(2)))
	if _, ok := g.scene.(*titleScene); !ok {
		t.Fatalf("a tap beside the menu left the title for %T", g.scene)
	}
	playEveryFrame(t, g, screen, steps(tapAt(ScreenW/2, 600+44*1.7+20), wait(2)))
	if _, ok := g.scene.(*galleryScene); !ok {
		t.Fatalf("a tap on GALLERY went to %T", g.scene)
	}
}

// TestTapChoosesACard: on the character select a tap on another card chooses it, and a tap
// on the card chosen goes on to play with her.
//
//nolint:paralleltest // the scenario uses the package-level game state
func TestTapChoosesACard(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	playEveryFrame(t, g, screen, steps(press(input.Confirm), wait(2)))
	s, ok := g.scene.(*charSelectScene)
	if !ok {
		t.Fatalf("PLAY went to %T", g.scene)
	}
	from := s.sel
	to := (from + 1) % len(characters)
	playEveryFrame(t, g, screen, steps(tapAt(cardX(to, len(characters)), 400), wait(2)))
	if g.scene != s || s.sel != to {
		t.Fatalf("a tap on card %d: on %T with card %d chosen", to, g.scene, s.sel)
	}
	if locked(characters[to]) {
		t.Skip("the next card is locked")
	}
	playEveryFrame(t, g, screen, steps(tapAt(cardX(to, len(characters)), 400), wait(2)))
	if _, ok := g.scene.(*playScene); !ok {
		t.Fatalf("a tap on the chosen card went to %T, want play", g.scene)
	}
}

// TestTapInTheGallery: a tap on a character's card at the top shows her pictures, and one
// on a tile opens it when it is unlocked.
//
//nolint:paralleltest // the scenario uses the package-level game state
func TestTapInTheGallery(t *testing.T) {
	g, screen := newDrawScenario(t, func() {
		for _, c := range characters {
			if !c.Secret {
				store.Data.Progress(c.ID).SeenExpr[save.FirstPortrait] = true
			}
		}
	})
	playEveryFrame(t, g, screen, steps(press(input.Down, input.Confirm), wait(2)))
	s, ok := g.scene.(*galleryScene)
	if !ok {
		t.Fatalf("GALLERY went to %T", g.scene)
	}
	n := len(characters)
	to := (s.charIdx + 1) % n
	if locked(characters[to]) {
		t.Skip("the next character is locked")
	}
	playEveryFrame(t, g, screen, steps(tapAt(faceCardLeft(to, n, 1)+headCardW/2, 70), wait(2)))
	if s.charIdx != to || s.viewing {
		t.Fatalf("a tap on card %d: showing %d (viewing %v)", to, s.charIdx, s.viewing)
	}
	x0 := (ScreenW - (tileW*galleryCols + tileGap*(galleryCols-1))) / 2
	playEveryFrame(t, g, screen, steps(tapAt(x0+tileW/2, gridTop+tileH/2), wait(2)))
	if s.sel != 0 || !s.viewing {
		t.Fatalf("a tap on the first tile (her first portrait): sel %d, viewing %v", s.sel, s.viewing)
	}
	playEveryFrame(t, g, screen, steps(tapAt(ScreenW/2, ScreenH/2), wait(2)))
	if s.viewing {
		t.Fatal("a tap on the enlarged picture must close it")
	}
}

// TestCornerButtonFollowsTheScreen: the corner button is shown where Escape does something.
//
//nolint:paralleltest // the scenario uses the package-level game state
func TestCornerButtonFollowsTheScreen(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	if b := buttonOf(g.scene); b != noButton {
		t.Fatalf("the title shows button %d, want none", b)
	}
	playEveryFrame(t, g, screen, steps(press(input.Confirm), wait(2)))
	if b := buttonOf(g.scene); b != backButton {
		t.Fatalf("the character select shows button %d, want back", b)
	}
	playEveryFrame(t, g, screen, steps(press(input.Confirm), intro()))
	s, ok := g.scene.(*playScene)
	if !ok {
		t.Fatalf("on %T, want play", g.scene)
	}
	if b := buttonOf(s); b != pauseButton {
		t.Fatalf("the road shows button %d, want pause", b)
	}
	playEveryFrame(t, g, screen, press(input.Pause))
	if b := buttonOf(s); !s.paused || b != resumeButton {
		t.Fatalf("paused %v with button %d, want resume", s.paused, b)
	}
	g.placeButton()
	g.drawButton(screen) // drawn once the screen was touched; it goes through either way
}
