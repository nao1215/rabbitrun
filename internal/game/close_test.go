package game

import (
	"errors"
	"os"
	"testing"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/save"
	"github.com/nao1215/rabbitrun/road"
)

// runOnStage2 starts a run of the main character from the title, past the hammer show, and
// puts it on the second stage ten seconds into play.
func runOnStage2(t *testing.T, g *Game) *playScene {
	t.Helper()
	play(t, g, steps(press(input.Confirm, input.Confirm), intro()))
	s := playOf(t, g)
	s.eng.G.StartAt(road.Courses + 1)
	s.eng.PlayFrames = 10 * 60
	return s
}

// TestCloseRecordsTheRunInPlay: closing the window in the middle of a run records how far
// it got and how long it lasted, as quitting through the pause menu does. Close only wrote
// what was already marked, so a run ended by closing the window was lost.
//
//nolint:paralleltest // shares the save data and the characters
func TestCloseRecordsTheRunInPlay(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, g *Game, s *playScene)
	}{
		{name: "playing", setup: func(*testing.T, *Game, *playScene) {}},
		{name: "paused", setup: func(t *testing.T, g *Game, s *playScene) {
			t.Helper()
			play(t, g, press(input.Pause))
			if !s.paused {
				t.Fatal("not paused")
			}
		}},
		{name: "after a miss", setup: func(t *testing.T, g *Game, s *playScene) {
			t.Helper()
			for x := range road.W {
				s.eng.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0] // a wall she cannot avoid
			}
			frames := s.eng.PlayFrames
			for i := 0; i < 600 && !s.eng.G.Missed; i++ {
				play(t, g, wait(1))
			}
			if !s.eng.G.Missed {
				t.Fatal("no miss")
			}
			s.eng.PlayFrames = frames // keep the ten seconds the case checks
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newScenario(t, nil)
			s := runOnStage2(t, g)
			tc.setup(t, g, s)
			g.Close()
			p := reloadSave(t).Characters[heroID]
			if p == nil || p.BestStage != 2 || p.PlaySeconds != 10 {
				t.Fatalf("saved %+v, want stage 2 and 10 seconds", p)
			}
			// closing again (main calls Close once, but nothing must count twice)
			g.Close()
			if p := reloadSave(t).Characters[heroID]; p.PlaySeconds != 10 {
				t.Fatalf("a second Close counted the run again: %d seconds", p.PlaySeconds)
			}
		})
	}
}

// TestCloseAfterGameOverCountsTheRunOnce: a run already recorded at its game over (or at its
// ending) is not counted again when the window is closed on its menu.
//
//nolint:paralleltest // shares the save data and the characters
func TestCloseAfterGameOverCountsTheRunOnce(t *testing.T) {
	g := newScenario(t, nil)
	s := runOnStage2(t, g)
	s.eng.G.Lives = 0
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0]
	}
	for i := 0; i < 600 && !s.eng.Over(); i++ {
		play(t, g, wait(1))
	}
	if !s.eng.Over() || !s.committed {
		t.Fatalf("over %v, committed %v", s.eng.Over(), s.committed)
	}
	want := reloadSave(t).Characters[heroID].PlaySeconds
	g.Close()
	if p := reloadSave(t).Characters[heroID]; p.PlaySeconds != want || p.BestStage != 2 {
		t.Fatalf("saved %+v, want %d seconds once and stage 2", p, want)
	}
}

// TestCloseDuringARecordingWritesNothing: a scripted run (--record-demo, --capture,
// --debug) closed in play still leaves the player's save alone.
//
//nolint:paralleltest // shares the save data and the characters
func TestCloseDuringARecordingWritesNothing(t *testing.T) {
	g := newScenario(t, nil)
	store.ReadOnly = true
	runOnStage2(t, g)
	g.Close()
	if _, err := os.Stat(save.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a read-only run wrote the save: %v", err)
	}
}
