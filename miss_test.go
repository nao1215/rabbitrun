package main

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

func TestMissHoldsOnePose(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	audioMuted = true
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	s := newPlayScene(chars[0])
	s.ready = 0
	bg = newBackground()
	g := &Game{scene: s, bg: bg}
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = 1
	}
	for range 60 { // reach the miss
		s.Update(g)
	}
	if !s.eng.G.Missed {
		t.Fatal("no miss")
	}
	id := s.exprID
	for range 600 { // ten seconds of deciding
		s.Update(g)
		if s.exprID != id || s.expr != ExprCrying {
			t.Fatalf("the pose changed to %s (%s) while deciding", s.exprID, s.expr)
		}
	}
}

func TestPauseKeyDoesNothingOnTheMissScreen(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	audioMuted = true
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	s := newPlayScene(chars[0])
	s.ready = 0
	bg = newBackground()
	g := &Game{scene: s, bg: bg}
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = 1
	}
	for range 60 { // reach the miss
		s.Update(g)
	}
	if !s.eng.G.Missed {
		t.Fatal("no miss")
	}
	g.in.held[ActPause] = true // Esc on the miss screen
	s.Update(g)
	if s.paused {
		t.Fatal("the pause menu opened behind the miss screen")
	}
}
