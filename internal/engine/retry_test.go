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
