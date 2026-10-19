package input

import (
	"image"
	"testing"
)

// fingers plays frames of touches: each frame is the fingers down then (id, x, y).
func fingers(t *testing.T, tc *touches, frames ...[]touchSample) {
	t.Helper()
	for _, f := range frames {
		tc.update(f)
	}
}

// still is n frames of one finger id resting at (x, y).
func still(id int, x, y float64, n int) [][]touchSample {
	out := make([][]touchSample, n)
	for i := range out {
		out[i] = []touchSample{{id, x, y}}
	}
	return out
}

func TestTapConfirmsWhereItEnds(t *testing.T) {
	t.Parallel()
	var tc touches
	fingers(t, &tc, still(1, 200, 300, 5)...)
	if tc.held[Confirm] || !tc.used {
		t.Fatal("a finger still down is not a tap yet, but the screen counts as touched")
	}
	tc.update(nil)
	if !tc.held[Confirm] {
		t.Fatal("a quick touch without moving must be a press of Confirm")
	}
	if x, y, ok := (&Input{touch: tc}).Tap(); !ok || x != 200 || y != 300 {
		t.Fatalf("Tap() = %v, %v, %v; want 200, 300, true", x, y, ok)
	}
	tc.update(nil)
	if tc.held[Confirm] || tc.tapped {
		t.Fatal("a tap is one frame's press")
	}
}

func TestLongPressIsNoTap(t *testing.T) {
	t.Parallel()
	var tc touches
	fingers(t, &tc, still(1, 200, 300, tapFrames+1)...)
	tc.update(nil)
	if tc.held[Confirm] || tc.tapped {
		t.Fatal("a finger held still for long must not confirm")
	}
}

func TestDragHoldsADirection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		dx, dy float64
		want   Action
	}{
		{"left", -60, 10, Left},
		{"right", 60, -20, Right},
		{"up", 15, -60, Up},
		{"down", 0, 60, Down},
	}
	for _, c := range cases {
		var tc touches
		fingers(t, &tc, []touchSample{{1, 300, 400}}, []touchSample{{1, 300 + c.dx/2, 400 + c.dy/2}}, []touchSample{{1, 300 + c.dx, 400 + c.dy}})
		for a := Action(0); a < NumActions; a++ {
			if tc.held[a] != (a == c.want) {
				t.Errorf("%s: action %d held = %v", c.name, a, tc.held[a])
			}
		}
		tc.update([]touchSample{{1, 300 + c.dx, 400 + c.dy}})
		if !tc.held[c.want] {
			t.Errorf("%s: the direction must stay held while the finger stays", c.name)
		}
		tc.update(nil)
		if tc.held[c.want] || tc.held[Confirm] {
			t.Errorf("%s: a drag let go of holds nothing and is no tap", c.name)
		}
	}
}

func TestSmallMovesHoldNothing(t *testing.T) {
	t.Parallel()
	var tc touches
	fingers(t, &tc, []touchSample{{1, 300, 400}}, []touchSample{{1, 300 + touchDead - 1, 400}})
	if tc.held[Right] {
		t.Fatal("a move inside the dead zone must not steer")
	}
}

// TestTurningBackIsShort: after a long drag one way, the way back to the other direction
// is no longer than touchReach plus touchDead.
func TestTurningBackIsShort(t *testing.T) {
	t.Parallel()
	var tc touches
	fingers(t, &tc, []touchSample{{1, 100, 400}}, []touchSample{{1, 300, 400}}, []touchSample{{1, 500, 400}})
	if !tc.held[Right] {
		t.Fatal("dragged right, not held right")
	}
	tc.update([]touchSample{{1, 500 - touchReach - touchDead, 400}})
	if !tc.held[Left] {
		t.Fatal("turning back by touchReach+touchDead must steer the other way")
	}
}

// TestTapWithAnotherFingerWhileSteering: one finger steers and another taps (the hammer).
func TestTapWithAnotherFingerWhileSteering(t *testing.T) {
	t.Parallel()
	var tc touches
	fingers(t, &tc, []touchSample{{1, 300, 600}}, []touchSample{{1, 380, 600}})
	fingers(t, &tc, []touchSample{{1, 380, 600}, {2, 500, 300}}, []touchSample{{1, 380, 600}, {2, 500, 300}})
	tc.update([]touchSample{{1, 380, 600}})
	if !tc.held[Right] || !tc.held[Confirm] {
		t.Fatalf("steering right and a tap: held %v", tc.held)
	}
	// the second finger is not the stick, even when it moves
	fingers(t, &tc, []touchSample{{1, 380, 600}, {3, 500, 300}}, []touchSample{{1, 380, 600}, {3, 300, 300}})
	if tc.held[Left] || !tc.held[Right] {
		t.Fatalf("the first finger steers: held %v", tc.held)
	}
}

func TestCornerButtonIsEscape(t *testing.T) {
	t.Parallel()
	tc := touches{button: image.Rect(0, 0, 80, 80)}
	fingers(t, &tc, still(1, 30, 30, 3)...)
	tc.update(nil)
	if !tc.held[Cancel] || !tc.held[Pause] || tc.held[Confirm] || tc.tapped {
		t.Fatalf("the button must press Cancel and Pause and not tap: held %v", tc.held)
	}
	// a finger that went down on the button and slid off presses nothing, and steers nothing
	fingers(t, &tc, []touchSample{{2, 30, 30}}, []touchSample{{2, 200, 30}})
	if tc.held[Right] {
		t.Fatal("a finger from the button must not steer")
	}
	tc.update(nil)
	if tc.held[Cancel] || tc.held[Pause] {
		t.Fatal("a finger slid off the button must not press it")
	}
	// without a button the corner is the screen like anywhere else
	tc.button = image.Rectangle{}
	fingers(t, &tc, still(3, 30, 30, 2)...)
	tc.update(nil)
	if !tc.held[Confirm] {
		t.Fatal("with the button taken away, a tap in the corner is a tap")
	}
}

// tapScript taps at (x, y) on its first frame.
type tapScript struct {
	x, y float64
	done bool
}

func (s *tapScript) Frame() (held [NumActions]bool, typed []rune) { return held, nil }

func (s *tapScript) Tap() (float64, float64, bool) {
	if s.done {
		return 0, 0, false
	}
	s.done = true
	return s.x, s.y, true
}

func TestScriptCanTap(t *testing.T) {
	t.Parallel()
	var in Input
	in.SetScript(&tapScript{x: 10, y: 20})
	in.Update()
	if x, y, ok := in.Tap(); !ok || x != 10 || y != 20 || !in.Pressed(Confirm) || !in.Touched() {
		t.Fatalf("a scripted tap: Tap() = %v, %v, %v, Confirm pressed %v", x, y, ok, in.Pressed(Confirm))
	}
	in.Update()
	if _, _, ok := in.Tap(); ok || in.Held(Confirm) {
		t.Fatal("the tap is over on the next frame")
	}
}
