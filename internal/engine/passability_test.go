package engine

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// runFrames runs a character's run at the speed boost (the player's speed-up, held from
// the start and kept) to the all clear, with the walls passing through her, and notes what
// each frame of the road does (see frameWalls). The road is built the same way wherever she
// is, so these are the frames of every game of the run at that speed.
func runFrames(id string, extra bool, boost float64) []frameWalls {
	e := NewRun(id, extra)
	e.Boost = boost
	e.G.Safe = math.MaxInt32
	var out []frameWalls
	for f := 0; f < 60*60*10 && !e.G.AllClear; f++ {
		g := e.G
		side := g.Rows[road.PlayerRow]
		if g.SideRowBehind && road.PlayerRow+1 < road.Rows {
			side = g.Rows[road.PlayerRow+1]
		}
		fw := frameWalls{side: wallBits(side), top: e.PlayerSpeed()}
		before, steps := e.stepAcc, e.Steps
		e.Tick(false)
		if e.Steps != steps {
			before = 0
		}
		if before < 0.5 && e.stepAcc >= 0.5 {
			fw.halfway, fw.half = true, wallBits(g.Rows[road.PlayerRow])
		}
		out = append(out, fw)
		e.Events = e.Events[:0]
	}
	return out
}

// playInputs plays the directions held frame by frame on the real engine, as the play scene
// does with the keys, and reports whether the run was cleared without a miss and the course
// it got to.
func playInputs(id string, extra bool, boost float64, inputs []int) (bool, string) {
	e := NewRun(id, extra)
	e.Boost = boost
	holdDir, holdFrames := 0, 0
	for _, in := range inputs {
		if dir := in; dir != holdDir {
			holdDir, holdFrames = dir, 0
		}
		if holdDir != 0 {
			holdFrames++
			e.Move(float64(holdDir) * e.SlideSpeed(holdFrames) / 60)
		}
		e.Tick(false)
		e.Events = e.Events[:0]
		if e.G.Missed || e.G.Over {
			return false, e.Progress()
		}
	}
	return e.G.AllClear, e.Progress()
}

// progressAt is the course a run is on after frames frames at the speed boost.
func progressAt(id string, extra bool, boost float64, frames int) string {
	e := NewRun(id, extra)
	e.Boost = boost
	e.G.Safe = math.MaxInt32
	for range frames {
		e.Tick(false)
	}
	return e.Progress()
}

// passBins is how finely the passability search tells places apart (per cell).
const passBins = 20

// TestEveryRunCanBePassed searches every run, on both sides, for a way through without a
// hammer and without a miss: at the road's own speed, at the player's whole speed-up (held
// from the start) and at speeds between. The search (findWay) knows the whole run, as a
// player who has learned the road does (it is the same every game); the careful auto player
// passes every run reading only the screen (TestCarefulPlayerClearsAtFullSpeed). The way
// the search finds is played again on the real engine, the keys held frame by frame as the
// play scene reads them, so a pass here is a way a player can take, not the search's word
// for it.
func TestEveryRunCanBePassed(t *testing.T) {
	skipWholeRuns(t)
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			for _, boost := range []float64{1, 1.1, 1.2, boostMax} {
				t.Run(fmt.Sprintf("%s/%s/x%.1f", id, side(extra), boost), func(t *testing.T) {
					t.Parallel()
					frames := runFrames(id, extra, boost)
					inputs, got := findWay(frames, wayStart{x: road.W/2 + 0.5}, passBins)
					if got < len(frames) {
						t.Fatalf("no way through: every way ends on frame %d of %d, on course %s", got, len(frames), progressAt(id, extra, boost, got))
					}
					if ok, at := playInputs(id, extra, boost, inputs); !ok {
						t.Fatalf("the way found misses on the real engine on course %s", at)
					}
				})
			}
		}
	}
}

// TestSlideOverIsMove checks the search's slide against road.Game.Move on random rows of
// walls, places and slides up to the fastest a frame can slide: the same place (but for the
// last bits of the float) and the same miss.
func TestSlideOverIsMove(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // G404: test data
	fastest := 0.0
	for lv := 1; lv <= GameCourses+1; lv++ {
		fastest = max(fastest, NewRun(coolID, true).playerSpeedAt(lv)/60)
	}
	for i := range 20000 {
		var walls uint16
		for c := range road.W {
			if r.IntN(2) == 0 {
				walls |= 1 << c
			}
		}
		x := road.Half + r.Float64()*(road.W-2*road.Half)
		if touches(walls, x) {
			continue
		}
		dx := (2*r.Float64() - 1) * fastest
		g := road.New(1)
		for c := range road.W {
			g.Rows[road.PlayerRow][c].Wall = int8(walls >> c & 1)
		}
		g.SideRowBehind, g.X = false, x
		g.Move(dx)
		got, ok := slideOver(walls, x, dx)
		if ok == g.Missed || (ok && math.Abs(got-g.X) > 1e-9) {
			t.Fatalf("case %d: walls %09b, %.4f by %.4f: search %.6f (clear %v), Move %.6f (miss %v)", i, walls, x, dx, got, ok, g.X, g.Missed)
		}
	}
}

// TestFramesAheadAreTheFramesToCome checks the careful player's reading of the road on the
// screen against the frames that then come, all through a run on both sides and at both
// ends of the speed-up (over the starts of the courses, where the road speeds up).
func TestFramesAheadAreTheFramesToCome(t *testing.T) {
	t.Parallel()
	for _, extra := range []bool{false, true} {
		for _, boost := range []float64{1, boostMax} {
			all := runFrames("bunny", extra, boost)
			e := NewRun("bunny", extra)
			e.Boost = boost
			e.G.Safe = math.MaxInt32
			for f := range all {
				if f%7 == 0 {
					ahead := framesAhead(e)
					if len(ahead) == 0 {
						t.Fatalf("extra %v x%.1f frame %d: nothing read ahead", extra, boost, f)
					}
					for i, fw := range ahead {
						if f+i < len(all) && fw != all[f+i] {
							t.Fatalf("extra %v x%.1f frame %d: frame %d ahead read as %+v, came as %+v", extra, boost, f, i, fw, all[f+i])
						}
					}
				}
				e.Tick(false)
				e.Events = e.Events[:0]
			}
		}
	}
}

// TestFindWayTellsAWayFromNone checks the search on frames made up for it: a row with one
// gap at the far side, reached halfway after a few frames, can be got to with time enough to
// slide there and not without; and the way it gives gets there.
func TestFindWayTellsAWayFromNone(t *testing.T) {
	t.Parallel()
	const gap = uint16(1<<road.W-1) &^ 1 // only column 0 open
	frames := func(n int) []frameWalls {
		out := make([]frameWalls, n+1)
		for i := range out {
			out[i] = frameWalls{top: 8}
		}
		out[n].halfway, out[n].half = true, gap
		return out
	}
	start := wayStart{x: road.W/2 + 0.5}
	if _, got := findWay(frames(10), start, passBins); got != 10 {
		t.Errorf("4 cells in 11 frames: the search got through %d frames, want none past the gate (10)", got)
	}
	fs := frames(60)
	inputs, got := findWay(fs, start, passBins)
	if got != len(fs) {
		t.Fatalf("4 cells in 61 frames: the search got through %d of %d frames", got, len(fs))
	}
	x, held, dir := start.x, 0, 0
	for i, in := range inputs {
		if in != dir {
			dir, held = in, 0
		}
		if dir != 0 {
			held++
			x, _ = slideOver(fs[i].side, x, float64(dir)*slideSpeedAt(fs[i].top, held)/60)
		}
	}
	if touches(gap, x) {
		t.Errorf("the way found ends at %.2f, in the wall", x)
	}
	if inputs, got := findWay(nil, start, passBins); got != 0 || len(inputs) != 0 {
		t.Errorf("no frames: %d inputs, %d frames", len(inputs), got)
	}
}
