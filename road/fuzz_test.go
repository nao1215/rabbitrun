package road

import (
	"strings"
	"testing"
)

// TestTwoRetriesInARowKeepWhatWasTaken misses again right after a retry: the second retry
// goes back another RewindRows, past the rows the first one put on the screen, and the
// rows built again as the road comes on brought back the sweets she had picked up there
// (FuzzRetriesKeepWhatWasTaken found it). Only the rows on the screen at a retry were
// cleared of them.
func TestTwoRetriesInARowKeepWhatWasTaken(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 20; seed++ {
		g := New(seed)
		g.Safe = 1 << 30 // run through the walls, picking up the sweets in her way
		for range 200 {
			g.Step()
			g.X = float64(g.Distance%(W-1)) + 1 // across the road, to take more
			g.pick()
		}
		g.Safe = 0
		for range 2 {
			g.crash()
			if !g.Restart() {
				t.Fatalf("seed %d: no retry", seed)
			}
		}
		g.Safe = 1 << 30
		for i := range 3 * RewindRows {
			g.Step()
			rows := append(g.Rows[:], g.Ahead)
			ids := append(g.ids[:], g.aheadID)
			for y, r := range rows {
				for x, c := range r {
					if c.Sweet != SweetNone && g.taken[takenKey{ids[y], x}] {
						t.Fatalf("seed %d, %d rows after the second retry: the sweet taken at %+v, column %d is back", seed, i+1, ids[y], x)
					}
				}
			}
		}
	}
}

// Opcodes FuzzRetriesKeepWhatWasTaken reads from its input, one byte per action.
const (
	rOpLeft = iota
	rOpRight
	rOpStep // a row comes down and reaches halfway, as a frame of play does
	rOpRun  // ten rows with nobody steering
	rOpHammer
	rOpRestart
	rOpGiveUp
	rOpCount
)

// FuzzRetriesKeepWhatWasTaken plays a road (its themes, side and seed from the first
// bytes) through the actions of the input, retrying after misses, and checks that a sweet
// she picked up never comes back on a retry. What she picked up is told from the screen
// (a sweet gone from a row still there after a step or a slide), not from the game's own
// record, so the check does not lean on what it checks.
func FuzzRetriesKeepWhatWasTaken(f *testing.F) {
	f.Add([]byte{0, 0, 0})
	for th := range themeCount {
		f.Add(append([]byte{byte(th), byte(th) * 7, byte(th)}, strings.Repeat(string([]byte{rOpRun, rOpLeft, rOpRun, rOpRestart, rOpRight, rOpStep, rOpRun, rOpRestart}), 40)...))
	}
	f.Add(append([]byte{1, 2, 3}, strings.Repeat(string([]byte{rOpRun, rOpHammer, rOpRun, rOpGiveUp}), 20)...))
	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) < 3 {
			return
		}
		setup, ops := in[:3], in[3:]
		if len(ops) > 2048 {
			ops = ops[:2048]
		}
		g := NewWith(uint64(setup[1])+1, Standard)
		g.TotalCourses = 16
		g.Hard = setup[2]&1 == 1
		var all []Theme
		for th := range themeCount {
			all = append(all, th)
		}
		g.Themes = make([]Theme, g.TotalCourses)
		for i := range g.Themes {
			g.Themes[i] = all[(int(setup[0])+i*int(setup[2]|1))%len(all)]
		}
		picked := map[takenKey]bool{}
		for i, b := range ops {
			op := b % rOpCount
			seen := sweetsOnScreen(g)
			switch op {
			case rOpLeft:
				g.Move(-0.4)
			case rOpRight:
				g.Move(0.4)
			case rOpStep:
				g.Step()
				g.ReachHalfway()
			case rOpRun:
				for range 10 {
					g.Step()
					g.ReachHalfway()
				}
			case rOpHammer:
				g.UseBomb() // takes the walls, leaves the sweets
			case rOpRestart:
				missed, lives := g.Missed, g.Lives
				if g.Restart() != (missed && lives > 0) {
					t.Fatalf("step %d: RETRY after a miss %v with %d lives", i, missed, lives)
				}
			case rOpGiveUp:
				g.GiveUp()
			}
			now := sweetsOnScreen(g)
			for k := range now {
				if picked[k] {
					t.Fatalf("step %d (op %d): the sweet picked up at row %+v, column %d is back", i, op, k.id, k.x)
				}
			}
			if op == rOpRestart {
				continue // the road was built again: nothing was picked up
			}
			for k := range seen {
				if rowOnScreen(g, k.id) && !now[k] {
					picked[k] = true // a sweet gone from a row still on the screen: she took it
				}
			}
		}
	})
}

// sweetsOnScreen is the sweets on the road and in the row about to come in, by row and column.
func sweetsOnScreen(g *Game) map[takenKey]bool {
	out := map[takenKey]bool{}
	for y, r := range g.Rows {
		for x, c := range r {
			if c.Sweet != SweetNone {
				out[takenKey{g.ids[y], x}] = true
			}
		}
	}
	for x, c := range g.Ahead {
		if c.Sweet != SweetNone {
			out[takenKey{g.aheadID, x}] = true
		}
	}
	return out
}

// rowOnScreen reports whether the row id is on the screen (on the road or about to come in).
func rowOnScreen(g *Game, id rowID) bool {
	for y := range g.Rows {
		if g.ids[y] == id {
			return true
		}
	}
	return g.aheadID == id
}
