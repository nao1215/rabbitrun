package engine

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// TestRetryShowsTheRoadOfBefore misses at points all along every run and checks that the
// retry puts on the screen just what was there road.RewindRows rows before the miss, wall for
// wall and sweet for sweet. The first course was built again one row short and with other
// sweets (the game's first row is built before the run is set up), and the courses after a
// feast or a vault's cage in the same stage with their lines of sweets and hammer moved (the
// stage's row count was taken for the length of the courses before).
func TestRetryShowsTheRoadOfBefore(t *testing.T) {
	t.Parallel()
	const every = 23 // rows between the misses tried
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			rows := 0
			for _, r := range runRows(NewRun(id, extra)) {
				rows += len(r)
			}
			for missAt := road.RewindRows + 1; missAt < rows; missAt += every {
				g := NewRun(id, extra).G
				g.Safe = 1 << 30
				var before [road.Rows]road.Row
				for s := 1; s <= missAt && !g.AllClear; s++ {
					g.X = 0.5 // in the wall at the edge: she takes (almost) nothing
					g.Step()
					if s == missAt-road.RewindRows {
						before = g.Rows
					}
				}
				if g.AllClear {
					break
				}
				g.Safe, g.Missed = 0, true
				if !g.Restart() {
					t.Fatalf("%s/%s, row %d: no retry", id, side(extra), missAt)
				}
				after := g.Rows
				for y := range after { // what she took at the edge stays taken
					before[y][0].Sweet, after[y][0].Sweet = 0, 0
				}
				for y := range after {
					if after[y] != before[y] {
						t.Errorf("%s/%s, a miss %d rows in (course %d): row %d of the screen was %v, after the retry %v", id, side(extra), missAt, g.Level, y, before[y], after[y])
						break
					}
				}
			}
		}
	}
}

// TestRetryOnTheOpenRoadAfterTheLastCourse misses on every row of the open road after the
// last course (the last walls are still coming down onto her) and checks that the retry
// shows the screen of road.RewindRows rows before, and that the road from there is the one
// the run had: the rest of the last walls, then the open road to the all clear.
func TestRetryOnTheOpenRoadAfterTheLastCourse(t *testing.T) {
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			// the screens of the run, step by step, and the step on which the last course ends
			var screens [][road.Rows]road.Row
			last := -1
			g := NewRun(id, extra).G
			g.Safe = 1 << 30
			for !g.AllClear {
				g.X = 0.5 // in the wall at the edge: she takes (almost) nothing
				g.Step()
				screens = append(screens, g.Rows)
				if last < 0 && g.Level > GameCourses {
					last = len(screens)
				}
			}
			for missAt := last; missAt < len(screens); missAt++ {
				g := NewRun(id, extra).G
				g.Safe = 1 << 30
				for range missAt {
					g.X = 0.5
					g.Step()
				}
				g.Safe, g.Missed = 0, true
				if !g.Restart() {
					t.Fatalf("%s/%s, %d rows into the open road: no retry", id, side(extra), missAt-last)
				}
				g.Safe = 1 << 30
				for s := missAt - road.RewindRows; ; s++ {
					after, before := g.Rows, screens[s-1]
					for y := range after { // what she took at the edge stays taken
						before[y][0].Sweet, after[y][0].Sweet = 0, 0
					}
					if after != before {
						t.Errorf("%s/%s, a miss %d rows into the open road: %d rows after the retry the screen is not the one of the run", id, side(extra), missAt-last, s-missAt+road.RewindRows)
						break
					}
					if s == len(screens) || g.AllClear {
						if s != len(screens) || !g.AllClear {
							t.Errorf("%s/%s, a miss %d rows into the open road: the all clear came %d rows after the retry, in the run %d", id, side(extra), missAt-last, s-missAt+road.RewindRows, len(screens)-missAt+road.RewindRows)
						}
						break
					}
					g.X = 0.5
					g.Step()
				}
			}
		}
	}
}
