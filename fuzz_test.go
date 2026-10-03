package main

import (
	"encoding/json"
	"reflect"
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
	// While playing, the player never stands inside a wall unless in the grace after a crash.
	if !g.Over && g.Safe == 0 && g.Rows[road.PlayerRow][g.Col()].Wall != 0 {
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

// FuzzSaveDecode feeds arbitrary bytes to the save-data decoder. Whatever the
// file holds, the game must end up with usable progress for every character,
// and valid data must survive a write / read round trip unchanged.
func FuzzSaveDecode(f *testing.F) {
	f.Add([]byte(`{"characters":{"gyal":{"high_score":10,"total_score":30,"unlocked_cg":{"cg_peace":true},"seen_expressions":{"normal":true}}}}`))
	f.Add([]byte(`{"characters":{"gyal":null}}`))
	f.Add([]byte(`{"characters":null}`))
	f.Add([]byte(`{"characters":{"a":{"unlocked_cg":null,"seen_expressions":{}}}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"characters":{"ÿ":{"high_score":-1}}}`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var s SaveData
		err := decodeSave(&s, raw)
		if s.Characters == nil {
			t.Fatal("Characters is nil after decoding")
		}
		for id := range s.Characters {
			p := s.progress(id)
			if p == nil || p.UnlockedCG == nil || p.SeenExpr == nil {
				t.Fatalf("progress(%q) = %+v is not usable", id, p)
			}
		}
		if p := s.progress("never-saved"); !p.SeenExpr[ExprNormal] {
			t.Fatal("new progress lacks the default portrait")
		}
		if err != nil {
			return
		}
		out, err := json.Marshal(&s)
		if err != nil {
			t.Fatalf("cannot re-encode decoded save: %v", err)
		}
		var back SaveData
		if err := decodeSave(&back, out); err != nil {
			t.Fatalf("cannot decode re-encoded save %s: %v", out, err)
		}
		if !reflect.DeepEqual(&s, &back) {
			t.Fatalf("round trip changed the save:\n %+v\n %+v", s.Characters, back.Characters)
		}
	})
}
