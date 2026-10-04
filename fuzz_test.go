package main

import (
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// Opcodes FuzzEngineOps reads from its input, one byte per action.
const (
	opLeft = iota
	opRight
	opTick
	opBoost
	opBomb
	opCount
)

// checkEngine asserts the invariants that must hold after every action.
func checkEngine(t *testing.T, e *Engine, prevScore, step int) {
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
	if g.Lives < 0 || g.Lives > road.MaxLives || (g.Lives == 0) != g.Over {
		t.Fatalf("step %d: %d lives, over %v", step, g.Lives, g.Over)
	}
	if g.Level < 1 || g.Level > GameCourses+1 { // one past the last course on the open road after it
		t.Fatalf("step %d: level %d", step, g.Level)
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
			if c.Wall > int8(KindOrange) || c.Wall < 0 || (c.Sweet != 0 && c.Wall != 0) {
				t.Fatalf("step %d: bad cell %+v", step, c)
			}
		}
	}
}

// FuzzEngineOps plays a fresh engine with a fixed seed through the actions
// encoded in the input and checks the engine invariants after each one.
func FuzzEngineOps(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte(strings.Repeat(string([]byte{opTick}), 900)))
	f.Add([]byte(strings.Repeat(string([]byte{opLeft, opTick, opTick, opRight, opBoost}), 200)))
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 4096 {
			ops = ops[:4096]
		}
		e := newRun(heroID, false)
		checkEngine(t, e, 0, -1)
		for i, b := range ops {
			prev := e.G.Distance
			overBefore := e.G.Over
			switch b % opCount {
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
			}
			if overBefore && (!e.G.Over || e.G.Distance != prev) {
				t.Fatalf("step %d: engine changed after game over", i)
			}
			checkEngine(t, e, prev, i)
			e.Events = e.Events[:0]
		}
	})
}
