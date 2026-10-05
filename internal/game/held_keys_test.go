package game

import (
	"math"
	"testing"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/road"
)

// Keys held from the road into a menu over it: holding up speeds the road up, so a player
// often still holds it when she runs into a wall or pauses.

// holding holds the actions a for n frames.
func holding(n int, a ...input.Action) []scriptFrame {
	out := make([]scriptFrame, n)
	for i := range out {
		out[i].held = a
	}
	return out
}

// TestUpHeldIntoAMenuDoesNotMoveIt holds up (the speed-up) from the road into the menu
// that comes up over it: the miss, the game over and the pause menu. The key repeat of a
// menu counted the frames it had been held on the road, so the choice ran up and down by
// itself every few frames, and letting go left it anywhere: on GIVE UP after a miss, a
// press of Enter ended the run.
//
//nolint:paralleltest // shares the save data and the characters
func TestUpHeldIntoAMenuDoesNotMoveIt(t *testing.T) {
	crash := func(t *testing.T, lives int) (*Game, *playScene) {
		t.Helper()
		g := newScenario(t, nil)
		s := startRun(t, g)
		s.eng.G.Lives = lives
		wallAcross(s)
		for f := 0; !s.eng.G.Missed && !s.eng.Over(); f++ {
			if f > 5*60 {
				t.Fatal("no miss on the wall across the road")
			}
			play(t, g, holding(1, input.Up))
		}
		return g, s
	}
	t.Run("miss", func(t *testing.T) {
		g, s := crash(t, 2)
		for f := range missFrames + 90 {
			play(t, g, holding(1, input.Up))
			if s.missSel != 0 {
				t.Fatalf("the miss menu went to %q %d frames in with up held from the road, want it on %q", missItems[s.missSel], f+1, missItems[0])
			}
		}
		play(t, g, steps(wait(1), press(input.Up)))
		if s.missSel != 1 {
			t.Errorf("a fresh press of up left the miss menu on %q, want %q", missItems[s.missSel], missItems[1])
		}
	})
	t.Run("game over", func(t *testing.T) {
		g, s := crash(t, 0)
		for f := range curtainStart + curtainFrames + 120 {
			play(t, g, holding(1, input.Up))
			if s.overSel != 0 {
				t.Fatalf("the game over menu went to %q %d frames in with up held from the road, want it on %q", overItems[s.overSel], f+1, overItems[0])
			}
		}
	})
	t.Run("pause", func(t *testing.T) {
		g := newScenario(t, nil)
		s := startRun(t, g)
		for y := range road.Rows {
			s.eng.G.Rows[y] = road.Row{}
		}
		play(t, g, holding(30, input.Up))
		play(t, g, holding(1, input.Up, input.Pause))
		if !s.paused {
			t.Fatal("not paused")
		}
		for f := range 90 {
			play(t, g, holding(1, input.Up))
			if s.pauseSel != 0 {
				t.Fatalf("the pause menu went to %q %d frames in with up held from the road, want it on %q", pauseItems[s.pauseSel], f+1, pauseItems[0])
			}
		}
	})
}

// TestSlideStartsSlowAfterAPause lets go of a direction while the game is paused and
// presses it again as it goes on: the slide starts slow, as every new press does
// (Engine.SlideSpeed). The frames held were counted on from before the pause, so the new
// press slid her at full speed at once and a little tap went too far.
//
//nolint:paralleltest // shares the save data and the characters
func TestSlideStartsSlowAfterAPause(t *testing.T) {
	g := newScenario(t, nil)
	s := startRun(t, g)
	e := s.eng
	for y := range road.Rows { // an open road: nothing stops her slide
		e.G.Rows[y] = road.Row{}
	}
	e.G.Ahead = road.Row{}
	e.G.X = 1.5
	play(t, g, holding(20, input.Right)) // up to full speed
	play(t, g, holding(1, input.Right, input.Pause))
	if !s.paused {
		t.Fatal("not paused")
	}
	play(t, g, wait(30))                             // let go while paused
	play(t, g, holding(1, input.Right, input.Pause)) // pressed again as the game goes on
	if s.paused {
		t.Fatal("still paused")
	}
	x := e.G.X
	play(t, g, holding(1, input.Right))
	want := e.SlideSpeed(1) / 60
	if got := e.G.X - x; math.Abs(got-want) > 1e-9 {
		t.Errorf("the first frame of a new press slid her %.4f cells, want %.4f (a new press starts slow)", got, want)
	}
}
