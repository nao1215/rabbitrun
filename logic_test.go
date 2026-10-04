package main

import (
	"slices"
	"testing"

	"github.com/nao1215/rabbitrun/internal/save"
)

func TestStickHit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    Action
		x, y float64
		want bool
	}{
		{"left pushed", ActLeft, -0.9, 0, true},
		{"left in dead zone", ActLeft, -0.5, 0, false},
		{"right pushed", ActRight, 0.51, 0, true},
		{"right when pushed left", ActRight, -1, 0, false},
		{"up pushed", ActUp, 0, -1, true},
		{"down pushed", ActDown, 0, 1, true},
		{"down in dead zone", ActDown, 0, 0.4, false},
		{"confirm never from the stick", ActConfirm, 1, 1, false},
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
		in.holdFrames[ActDown] = f
		if in.Repeat(ActDown) {
			fired = append(fired, f)
		}
	}
	if want := []int{1, 25, 30, 35, 40}; !slices.Equal(fired, want) {
		t.Fatalf("menu repeat fired on %v, want %v", fired, want)
	}

	in.held[ActConfirm] = true
	if !in.Pressed(ActConfirm) || !in.Held(ActConfirm) {
		t.Fatal("a new press must be both pressed and held")
	}
	in.prev[ActConfirm] = true
	if in.Pressed(ActConfirm) || !in.Held(ActConfirm) {
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
		in.held[ActLeft], in.held[ActRight] = left, right
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
	for a := range actionCount {
		if len(keyMap[a]) == 0 {
			t.Errorf("action %d has no keyboard key", a)
		}
		if len(padMap[a]) == 0 {
			t.Errorf("action %d has no standard gamepad button", a)
		}
	}
}

func TestMoodFamily(t *testing.T) {
	t.Parallel()
	families := []string{ExprNormal, ExprHappy, ExprExcited, ExprWorried, ExprPanic, ExprGameOver}
	for _, st := range allStates {
		if f := family(st); !slices.Contains(families, f) {
			t.Errorf("family(%q) = %q, not one of the six families", st, f)
		}
	}
	for _, f := range families {
		if family(f) != f {
			t.Errorf("family head %q maps to %q", f, family(f))
		}
	}
}

func TestReactPriority(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		current   string
		timer     int
		next      string
		wantExpr  string
		wantTimer int
		wantHop   bool
	}{
		{"first reaction", "", 0, ExprHappy, ExprHappy, 90, false},
		{"weaker is ignored while active", ExprExcited, 50, ExprHappy, ExprExcited, 50, false},
		{"weaker replaces an expired one", ExprExcited, 0, ExprHappy, ExprHappy, 90, false},
		{"stronger overrides", ExprHappy, 50, ExprPerfect, ExprPerfect, 90, true},
		{"equal rank overrides", ExprExcited, 50, ExprTreat, ExprTreat, 90, true},
		{"combo hops", "", 0, ExprCombo, ExprCombo, 90, true},
	}
	rankOf := map[string]int{
		"": rankHint, ExprHappy: rankSmall, ExprCombo: rankCombo, ExprExcited: rankBig, ExprTreat: rankBig, ExprPerfect: rankPerfect,
	}
	for _, tc := range cases {
		s := &PlayScene{reactExpr: tc.current, reactTimer: tc.timer, reactRank: rankOf[tc.current]}
		s.react(tc.next, 90, rankOf[tc.next])
		if s.reactExpr != tc.wantExpr || s.reactTimer != tc.wantTimer || (s.hop == 1) != tc.wantHop {
			t.Errorf("%s: expr %q timer %d hop %v", tc.name, s.reactExpr, s.reactTimer, s.hop)
		}
		if tc.wantExpr == tc.next && !s.repick {
			t.Errorf("%s: an accepted reaction must re-pick the pose", tc.name)
		}
	}
}

func TestRetryShowsComebackAfterGameOverPose(t *testing.T) {
	t.Parallel()
	c := &Character{ID: "t", Expressions: []ImageEntry{
		{ID: ExprNormal, State: ExprNormal}, {ID: ExprGameOver, State: ExprGameOver}, {ID: ExprComeback, State: ExprComeback},
	}}
	s := &PlayScene{char: c, eng: newRun(heroID, false), prog: &save.CharProgress{SeenExpr: map[string]bool{}}}
	s.expr, s.exprID, s.comeback = ExprGameOver, ExprGameOver, comebackDelay
	for range comebackDelay - 1 {
		s.updateComeback()
	}
	if s.expr != ExprGameOver {
		t.Fatalf("the game over pose stays first, got %q", s.expr)
	}
	s.updateComeback()
	for range windupFrames { // she crouches for a moment before springing up
		s.updateComeback()
	}
	if s.reactExpr != ExprComeback || s.expr != ExprComeback {
		t.Errorf("then the comeback pose shows, got reaction %q expression %q", s.reactExpr, s.expr)
	}
	if s.hop != 1 {
		t.Error("the comeback pose starts with a hop")
	}
}

func TestSelectMusicSpeedsUpOverTime(t *testing.T) {
	t.Parallel()
	cases := []struct{ frames, want int }{
		{0, 0},
		{selectGrooveSeconds*60 - 1, 0},
		{selectGrooveSeconds * 60, 1},
		{selectFullSeconds * 60, 2},
		{selectFullSeconds * 600, 2},
	}
	for _, tc := range cases {
		if got := selectIntensity(tc.frames); got != tc.want {
			t.Errorf("selectIntensity(%d) = %d, want %d", tc.frames, got, tc.want)
		}
	}
}

func TestMenuNavWrapsAround(t *testing.T) {
	t.Parallel()
	var in Input
	in.holdFrames[ActUp] = 1
	if got := in.menuNav(0, 3, ActUp, ActDown); got != 2 {
		t.Fatalf("up from the top went to %d, want the bottom (2)", got)
	}
	in.holdFrames[ActUp], in.holdFrames[ActDown] = 0, 1
	if got := in.menuNav(2, 3, ActUp, ActDown); got != 0 {
		t.Fatalf("down from the bottom went to %d, want the top (0)", got)
	}
	in.holdFrames[ActDown] = 2 // held, not repeating yet
	if got := in.menuNav(1, 3, ActUp, ActDown); got != 1 {
		t.Fatalf("a held key moved the menu to %d before repeating", got)
	}
}
