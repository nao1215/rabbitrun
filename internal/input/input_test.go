package input

import (
	"slices"
	"testing"
)

func TestStickHit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    Action
		x, y float64
		want bool
	}{
		{"left pushed", Left, -0.9, 0, true},
		{"left in dead zone", Left, -0.5, 0, false},
		{"right pushed", Right, 0.51, 0, true},
		{"right when pushed left", Right, -1, 0, false},
		{"up pushed", Up, 0, -1, true},
		{"down pushed", Down, 0, 1, true},
		{"down in dead zone", Down, 0, 0.4, false},
		{"confirm never from the stick", Confirm, 1, 1, false},
	}
	for _, tc := range cases {
		if got := stickHit(tc.a, tc.x, tc.y); got != tc.want {
			t.Errorf("%s: stickHit = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestInputRepeatAndPress(t *testing.T) {
	t.Parallel()
	var fired []int
	var in Input
	for f := 1; f <= 40; f++ {
		in.holdFrames[Down] = f
		if in.Repeat(Down) {
			fired = append(fired, f)
		}
	}
	if want := []int{1, 25, 30, 35, 40}; !slices.Equal(fired, want) {
		t.Fatalf("menu repeat fired on %v, want %v", fired, want)
	}

	in.held[Confirm] = true
	if !in.Pressed(Confirm) || !in.Held(Confirm) {
		t.Fatal("a new press must be both pressed and held")
	}
	in.prev[Confirm] = true
	if in.Pressed(Confirm) || !in.Held(Confirm) {
		t.Fatal("a kept press must be held but not pressed again")
	}
}

// TestSideFollowsTheLastPressed rolls from left to right the way fingers do on a
// keyboard (the second key goes down before the first comes up): she must turn at once,
// not stop while both are held.
func TestSideFollowsTheLastPressed(t *testing.T) {
	t.Parallel()
	var in Input
	frame := func(left, right bool) int {
		in.prev = in.held
		in.held[Left], in.held[Right] = left, right
		in.trackSide()
		return in.Side()
	}
	steps := []struct {
		left, right bool
		want        int
	}{
		{false, false, 0},
		{true, false, -1},
		{true, true, 1},   // right goes down over left: right wins
		{false, true, 1},  // left comes up: still right
		{true, true, -1},  // left pressed anew over right: left wins
		{true, false, -1}, // right comes up
		{false, false, 0},
	}
	for i, st := range steps {
		if got := frame(st.left, st.right); got != st.want {
			t.Fatalf("step %d (left %v right %v): side %d, want %d", i, st.left, st.right, got, st.want)
		}
	}
}

func TestEveryActionHasAKey(t *testing.T) {
	t.Parallel()
	for a := range NumActions {
		if len(keyMap[a]) == 0 {
			t.Errorf("action %d has no keyboard key", a)
		}
		if len(padMap[a]) == 0 {
			t.Errorf("action %d has no standard gamepad button", a)
		}
	}
}

// TestKeyboardPollingWithNothingPressed reads the real keyboard and pads (no script)
// with no key down: no action is held and nothing is typed.
func TestKeyboardPollingWithNothingPressed(t *testing.T) { //nolint:paralleltest // reads ebiten's input state
	var in Input
	in.Update()
	for a := Action(0); a < NumActions; a++ {
		if in.Held(a) || in.Pressed(a) {
			t.Errorf("action %d is held with no key down", a)
		}
	}
	if c := in.Chars(); len(c) != 0 {
		t.Errorf("typed %q with no key down", string(c))
	}
	if in.Side() != 0 {
		t.Errorf("side %d with no key down", in.Side())
	}
}

// oneFrame is a Script of a single frame.
type oneFrame struct {
	held  [NumActions]bool
	typed []rune
	done  bool
}

func (s *oneFrame) Frame() ([NumActions]bool, []rune) {
	if s.done {
		return [NumActions]bool{}, nil
	}
	s.done = true
	return s.held, s.typed
}

// TestScriptStandsInForThePlayer plays a frame from a script: its actions are pressed and
// held, its letters typed, and the frame after it lets go.
func TestScriptStandsInForThePlayer(t *testing.T) {
	t.Parallel()
	var in Input
	s := &oneFrame{typed: []rune("R")}
	s.held[Confirm] = true
	in.SetScript(s)
	in.Update()
	if !in.Pressed(Confirm) || !in.Held(Confirm) || in.Held(Cancel) {
		t.Fatal("the scripted press did not come through")
	}
	if got := string(in.Chars()); got != "R" {
		t.Fatalf("typed %q, want R", got)
	}
	in.Update()
	if in.Held(Confirm) || len(in.Chars()) != 0 {
		t.Fatal("the press was still held after the script ran out")
	}
}

// heldScript holds the same actions every frame.
type heldScript struct{ held [NumActions]bool }

func (s *heldScript) Frame() ([NumActions]bool, []rune) { return s.held, nil }

// TestReleaseWaitsForTheKeyToComeUp lets go of a held key: it is not held, pressed or
// repeated while it stays down, and a new press after it comes up counts again.
func TestReleaseWaitsForTheKeyToComeUp(t *testing.T) {
	t.Parallel()
	var in Input
	s := &heldScript{}
	s.held[Up] = true
	in.SetScript(s)
	for range 30 {
		in.Update()
	}
	in.Release(Up, Down) // Down is not held: nothing to let go of
	for f := range 60 {
		if in.Held(Up) || in.Pressed(Up) || in.Repeat(Up) {
			t.Fatalf("up counts %d frames after it was let go of, still held down", f)
		}
		in.Update()
	}
	s.held[Up] = false
	in.Update()
	s.held[Up], s.held[Down] = true, true
	in.Update()
	if !in.Pressed(Up) || !in.Repeat(Up) || !in.Pressed(Down) {
		t.Fatal("a new press after the key came up does not count")
	}
}
