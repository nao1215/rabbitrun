package main

import (
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// The start of a run: before READY, the open road fills with blocks, she swings a hammer,
// and they all break, so the player sees what a hammer does before the first one comes up.
// It does not use up her hammer, and the blocks are only shown (the road itself stays
// open). (A LET'S GO! picture before it made the start too long.)

const showHold = 40 // frames the blocks stand before the hammer comes

// startHammerShow begins a new run with the show: it fills the screen above the bunny with
// blocks to break.
func (s *PlayScene) startHammerShow() {
	s.keepWalls()
	for y := range road.PlayerRow { // the rows above her row; hers and those below stay open
		for x := range road.W {
			if c := &s.hammerWalls[y][x]; c.Wall == 0 && c.Sweet == road.SweetNone {
				c.Wall = road.CourseColors[(x+y)%len(road.CourseColors)]
			}
		}
	}
	s.showHold, s.showing = showHold, true
	sound.Play(sound.Denied) // the way is shut
	s.react(ExprBlocked, showHold, rankBig)
}

// updateHammerShow runs the show: the blocks stand, the cut-in, then the breaking (the
// same as a hammer swung in play). It reports whether the show is still on (the road and
// READY wait meanwhile).
func (s *PlayScene) updateHammerShow() bool {
	switch {
	case s.showHold > 0:
		if s.showHold--; s.showHold == 0 {
			s.swingHammer()
		}
	case s.cutin > 0 && s.crumble == 0 && s.showing:
		if s.cutin--; s.cutin == 0 {
			s.crumble = 1
		}
	case s.crumble > 0 && s.showing:
		s.updateCrumble()
		if s.crumble == 0 {
			s.showing = false
		}
	default:
		return false
	}
	s.updateExpression()
	return true
}

// showingWalls reports whether the board shows the show's blocks (before they break).
func (s *PlayScene) showingWalls() bool { return s.showing && s.crumble == 0 }
