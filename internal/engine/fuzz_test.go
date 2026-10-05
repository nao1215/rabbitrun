package engine

import (
	"slices"
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// maxWall is the highest wall color: the seven candy colors of the gummy blocks.
const maxWall = 7

// Opcodes FuzzEngineOps reads from its input, one byte per action.
const (
	opLeft = iota
	opRight
	opTick
	opBoost
	opBomb
	opRestart // RETRY after a miss
	opGiveUp  // GIVE UP after a miss
	opRun     // half a second with nobody steering
	opCount
)

// fuzzRunIDs are the runs FuzzEngineOps can play: every character with a table of course
// themes, and one without (every course mixed).
var fuzzRunIDs = func() []string {
	ids := make([]string, 0, len(courseThemes)+1)
	for id := range courseThemes {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return append(ids, "no-table")
}()

// checkEngine asserts the invariants that must hold after every action. gaveUp is whether
// the player gave up, which ends the game with lives left.
func checkEngine(t *testing.T, e *Engine, prevScore, step int, gaveUp bool) {
	t.Helper()
	g := e.G
	if g.Distance < prevScore {
		t.Fatalf("step %d: distance went down %d -> %d", step, prevScore, g.Distance)
	}
	if g.Bombs < 0 || g.Bombs > road.MaxBombs {
		t.Fatalf("step %d: %d bombs", step, g.Bombs)
	}
	if g.X < road.Half || g.X > road.W-road.Half {
		t.Fatalf("step %d: player at %.2f", step, g.X)
	}
	// A miss on the last life is the game over; a miss with a life left waits for RETRY
	// (which uses it, so play goes on with none left) or GIVE UP (over with lives left).
	if g.Lives < 0 || g.Lives > road.MaxLives || (g.Over && g.Missed) ||
		(g.Missed && g.Lives == 0) || (g.Over && g.Lives > 0 && !gaveUp) {
		t.Fatalf("step %d: %d lives, over %v, missed %v, gave up %v", step, g.Lives, g.Over, g.Missed, gaveUp)
	}
	if g.Level < 1 || g.Level > GameCourses+1 { // one past the last course on the open road after it
		t.Fatalf("step %d: level %d", step, g.Level)
	}
	if want := (g.Level-1)/road.Courses + 1; g.Level <= GameCourses && (g.Stage != want || g.Course != (g.Level-1)%road.Courses) {
		t.Fatalf("step %d: level %d is stage %d course %d", step, g.Level, g.Stage, g.Course)
	}
	if e.Boost > boostMax || (e.Boost != 0 && e.Boost < 1) {
		t.Fatalf("step %d: boost %v", step, e.Boost)
	}
	// While playing, the player never stands inside a wall of the row beside her body
	// unless in the grace after a crash. Until the row that just came in has slid
	// halfway down (SideRowBehind), that is still the row before it: a wall arriving
	// in her own row is only a miss at ReachHalfway, so she may stand under it until then.
	besideRow := road.PlayerRow
	if g.SideRowBehind && road.PlayerRow+1 < road.Rows {
		besideRow++
	}
	if !g.Over && !g.Missed && g.Safe == 0 && g.Rows[besideRow][g.Col()].Wall != 0 {
		t.Fatalf("step %d: player inside a wall", step)
	}
	for _, r := range g.Rows {
		for _, c := range r {
			if c.Wall > maxWall || c.Wall < 0 || (c.Sweet != 0 && c.Wall != 0) {
				t.Fatalf("step %d: bad cell %+v", step, c)
			}
		}
	}
}

// before is what checkStep compares an action's result with.
type before struct {
	distance, lives, level, rewind, frames int
	over, missed                           bool
}

func snapshot(e *Engine) before {
	return before{distance: e.G.Distance, lives: e.G.Lives, level: e.G.Level, rewind: e.G.RewindLevel(),
		frames: e.PlayFrames, over: e.G.Over, missed: e.G.Missed}
}

// checkStep asserts what the action op did, given the state before it and what it returned.
func checkStep(t *testing.T, e *Engine, op byte, b before, ok bool, step int) {
	t.Helper()
	g := e.G
	if b.over && (!g.Over || g.Distance != b.distance || e.PlayFrames != b.frames) {
		t.Fatalf("step %d: engine changed after game over", step)
	}
	if b.missed && (op == opTick || op == opBoost || op == opRun) && (g.Distance != b.distance || e.PlayFrames != b.frames) {
		t.Fatalf("step %d: the road moved on after a miss", step)
	}
	switch op {
	case opRestart:
		if want := b.missed && b.lives > 0; ok != want {
			t.Fatalf("step %d: RETRY did %v after a miss %v with %d lives", step, ok, b.missed, b.lives)
		}
		if !ok {
			if g.Lives != b.lives || g.Missed != b.missed || g.Level != b.level {
				t.Fatalf("step %d: a RETRY that did nothing changed the game", step)
			}
			return
		}
		// one life used, back RewindRows rows (into the course before at most), at its own speed;
		// a sweet on the cell she is put on is hers at once, and it can give a life back
		gained := 0
		for _, ev := range e.Events {
			if ev.Kind == road.EventOneUp {
				gained++
			}
		}
		if g.Lives != min(road.MaxLives, b.lives-1+gained) || g.Missed || g.Over || g.Level != b.rewind || g.Level > b.level || g.Level < b.level-1 {
			t.Fatalf("step %d: RETRY from level %d (rewind %d) with %d lives: level %d, %d lives, missed %v",
				step, b.level, b.rewind, b.lives, g.Level, g.Lives, g.Missed)
		}
		if e.Boost != 1 || e.Scroll() != 0 || !g.SideRowBehind {
			t.Fatalf("step %d: RETRY kept boost %v, scroll %v", step, e.Boost, e.Scroll())
		}
	case opGiveUp:
		if b.missed != (g.Over && !b.over) || g.Lives != b.lives {
			t.Fatalf("step %d: GIVE UP after a miss %v: over %v, %d lives (had %d)", step, b.missed, g.Over, g.Lives, b.lives)
		}
	}
}

// FuzzEngineOps plays a run through the actions encoded in the input and checks the
// engine invariants after each one. The first byte picks the run: the character (by
// fuzzRunIDs) and the side (regular or extra stages); the rest are opcodes. RETRY and GIVE
// UP are among them, so a miss does not end the exploration.
func FuzzEngineOps(f *testing.F) {
	f.Add([]byte{})
	f.Add(append([]byte{0}, strings.Repeat(string([]byte{opTick}), 900)...))
	f.Add(append([]byte{1}, strings.Repeat(string([]byte{opLeft, opTick, opTick, opRight, opBoost}), 200)...))
	// run into walls and retry on every life, on each run
	for i := range 2 * len(fuzzRunIDs) {
		f.Add(append([]byte{byte(i)}, strings.Repeat(string([]byte{opRun, opRun, opRestart, opBomb, opRun, opLeft, opRun, opRight}), 60)...))
	}
	f.Add(append([]byte{3}, strings.Repeat(string([]byte{opRun, opRun, opRun, opGiveUp}), 40)...))
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 4096 {
			ops = ops[:4096]
		}
		setup := byte(0)
		if len(ops) > 0 {
			setup, ops = ops[0], ops[1:]
		}
		id, extra := fuzzRunIDs[int(setup>>1)%len(fuzzRunIDs)], setup&1 == 1
		e := NewRun(id, extra)
		speed := roadProfile.Speed
		if extra {
			speed = extraProfile.Speed
		}
		if e.G.Hard != extra || e.G.Profile.Speed != speed {
			t.Fatalf("run %s extra %v: hard %v, speed %v", id, extra, e.G.Hard, e.G.Profile.Speed)
		}
		gaveUp := false
		checkEngine(t, e, 0, -1, gaveUp)
		for i, b := range ops {
			op := b % opCount
			was := snapshot(e)
			ok := false
			switch op {
			case opLeft:
				e.Move(-0.4)
			case opRight:
				e.Move(0.4)
			case opTick:
				e.Tick(false)
			case opBoost:
				e.Tick(true)
			case opBomb:
				e.UseHammer()
			case opRestart:
				ok = e.Restart()
			case opGiveUp:
				gaveUp = gaveUp || e.G.Missed
				e.GiveUp()
			case opRun:
				for range 30 {
					e.Tick(false)
				}
			}
			checkStep(t, e, op, was, ok, i)
			checkEngine(t, e, was.distance, i, gaveUp)
			e.Events = e.Events[:0]
		}
	})
}
