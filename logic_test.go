package main

import (
	"slices"
	"testing"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/save"
)

func TestMoodFamily(t *testing.T) {
	t.Parallel()
	families := []string{character.ExprNormal, character.ExprHappy, character.ExprExcited, character.ExprWorried, character.ExprPanic, character.ExprGameOver}
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
		{"first reaction", "", 0, character.ExprHappy, character.ExprHappy, 90, false},
		{"weaker is ignored while active", character.ExprExcited, 50, character.ExprHappy, character.ExprExcited, 50, false},
		{"weaker replaces an expired one", character.ExprExcited, 0, character.ExprHappy, character.ExprHappy, 90, false},
		{"stronger overrides", character.ExprHappy, 50, character.ExprPerfect, character.ExprPerfect, 90, true},
		{"equal rank overrides", character.ExprExcited, 50, character.ExprTreat, character.ExprTreat, 90, true},
		{"combo hops", "", 0, character.ExprCombo, character.ExprCombo, 90, true},
	}
	rankOf := map[string]int{
		"": rankHint, character.ExprHappy: rankSmall, character.ExprCombo: rankCombo, character.ExprExcited: rankBig, character.ExprTreat: rankBig, character.ExprPerfect: rankPerfect,
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
	c := &character.Character{ID: "t", Expressions: []character.ImageEntry{
		{ID: character.ExprNormal, State: character.ExprNormal}, {ID: character.ExprGameOver, State: character.ExprGameOver}, {ID: character.ExprComeback, State: character.ExprComeback},
	}}
	s := &PlayScene{char: c, eng: engine.NewRun(heroID, false), prog: &save.CharProgress{SeenExpr: map[string]bool{}}}
	s.expr, s.exprID, s.comeback = character.ExprGameOver, character.ExprGameOver, comebackDelay
	for range comebackDelay - 1 {
		s.updateComeback()
	}
	if s.expr != character.ExprGameOver {
		t.Fatalf("the game over pose stays first, got %q", s.expr)
	}
	s.updateComeback()
	for range windupFrames { // she crouches for a moment before springing up
		s.updateComeback()
	}
	if s.reactExpr != character.ExprComeback || s.expr != character.ExprComeback {
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
	var in input.Input
	// up pressed, then down pressed, then down still held
	in.SetScript(&script{frames: []scriptFrame{{held: []input.Action{input.Up}}, {held: []input.Action{input.Down}}, {held: []input.Action{input.Down}}}})
	in.Update()
	if got := menuNav(&in, 0, 3, input.Up, input.Down); got != 2 {
		t.Fatalf("up from the top went to %d, want the bottom (2)", got)
	}
	in.Update()
	if got := menuNav(&in, 2, 3, input.Up, input.Down); got != 0 {
		t.Fatalf("down from the bottom went to %d, want the top (0)", got)
	}
	in.Update() // held, not repeating yet
	if got := menuNav(&in, 1, 3, input.Up, input.Down); got != 1 {
		t.Fatalf("a held key moved the menu to %d before repeating", got)
	}
}

func TestMusicSpeedsUpWithTheRoad(t *testing.T) {
	t.Parallel()
	first, last := playBPM(1, engine.RegularSpeed), playBPM(engine.GameCourses, engine.RegularSpeed)
	if first != 136 || last < 170 || last > 190 {
		t.Fatalf("tempo %v on the first course, %v on the last", first, last)
	}
	prev := 0.0
	for lv := 1; lv <= engine.GameCourses; lv++ {
		if b := playBPM(lv, engine.RegularSpeed); b < prev {
			t.Fatalf("tempo falls at level %d: %v after %v", lv, b, prev)
		} else {
			prev = b
		}
	}
}
