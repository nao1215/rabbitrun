package game

import (
	"testing"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/road"
)

// pressAltEnter stands in for the fullscreen keys for the test: Alt+Enter is pressed on
// the frames altEnter[g.frame] (Enter is a confirm key too, so the script holds Confirm
// on them), and the switches of the window are counted in toggled.
func pressAltEnter(t *testing.T, g *Game, frames map[int]bool) *int {
	t.Helper()
	toggled := 0
	oldKeys, oldToggle := fullscreenKeys, toggleFullscreen
	fullscreenKeys = func() (bool, bool) { return false, frames[g.frame] }
	toggleFullscreen = func() { toggled++ }
	t.Cleanup(func() { fullscreenKeys, toggleFullscreen, quitRequested = oldKeys, oldToggle, false })
	return &toggled
}

// TestAltEnterOnlySwitchesTheWindow presses Alt+Enter on a menu: the window switches to
// fullscreen and nothing on the menu is chosen. Enter is also the confirm key, and the
// press went on to the screen, so Alt+Enter on EXIT closed the game and on RESET threw
// the run away (and in play it swung a hammer).
//
//nolint:paralleltest // shares the save data and the characters
func TestAltEnterOnlySwitchesTheWindow(t *testing.T) {
	altEnter := scriptFrame{held: []input.Action{input.Confirm}}
	t.Run("EXIT on the title", func(t *testing.T) {
		g := newScenario(t, nil)
		play(t, g, press(input.Down, input.Down))
		if s := titleOf(t, g); s.sel != 2 {
			t.Fatalf("on title item %d, want EXIT", s.sel)
		}
		toggled := pressAltEnter(t, g, map[int]bool{g.frame + 1: true})
		play(t, g, []scriptFrame{altEnter, {}})
		if *toggled != 1 {
			t.Errorf("the window switched %d times, want once", *toggled)
		}
		if quitRequested {
			t.Error("Alt+Enter on EXIT closed the game")
		}
	})
	t.Run("RESET on the pause menu", func(t *testing.T) {
		g := newScenario(t, nil)
		s := startRun(t, g)
		for y := range road.Rows {
			s.eng.G.Rows[y] = road.Row{}
		}
		play(t, g, press(input.Pause, input.Down))
		if !s.paused || s.pauseSel != 1 {
			t.Fatalf("paused %v on item %d, want RESET", s.paused, s.pauseSel)
		}
		toggled := pressAltEnter(t, g, map[int]bool{g.frame + 1: true})
		play(t, g, []scriptFrame{altEnter, {}})
		if *toggled != 1 {
			t.Errorf("the window switched %d times, want once", *toggled)
		}
		if g.scene != s || !s.paused {
			t.Error("Alt+Enter on RESET threw the run away")
		}
	})
	t.Run("in play", func(t *testing.T) {
		g := newScenario(t, nil)
		s := startRun(t, g)
		hammers := s.eng.G.Bombs
		pressAltEnter(t, g, map[int]bool{g.frame + 1: true})
		play(t, g, []scriptFrame{altEnter, {}})
		if s.eng.G.Bombs != hammers || s.cutin != 0 {
			t.Errorf("Alt+Enter swung a hammer (%d -> %d in stock)", hammers, s.eng.G.Bombs)
		}
	})
}
