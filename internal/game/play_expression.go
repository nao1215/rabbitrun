package game

import (
	"image/color"
	"math/rand/v2"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/road"
)

// The character's expression in play: the reactions to what happens, the moods of the
// road ahead, and the pose picked for each.

// comebackDelay is how long the retry keeps the collapsed game over pose before the
// character springs up into her "let's go!" pose (in frames, during READY).
const comebackDelay = 40

// updateComeback runs the retry performance during READY: the game over pose stays for
// comebackDelay frames, then the character springs up into her comeback pose.
func (s *playScene) updateComeback() {
	switch {
	case s.comeback > 1:
		s.comeback--
	case s.comeback == 1:
		s.comeback = 0
		s.react(character.ExprComeback, 150, rankBig) // rankBig also makes her hop
		s.updateExpression()
	case s.reactExpr == character.ExprComeback && s.reactTimer > 0:
		s.updateExpression()
	}
}

// getReady shows her ready pose while READY counts down at the start of a run (the road
// waits). After a retry she gets back up instead (updateComeback).
func (s *playScene) getReady() {
	if s.comeback > 0 || s.reactExpr == character.ExprComeback && s.reactTimer > 0 {
		return
	}
	s.reactTimer = 0 // the hammer show's cheer is over: READY wins
	s.react(character.ExprReady, readyFr, rankCombo)
}

// Reaction strengths: a weaker reaction never interrupts a stronger one that is still showing.
const (
	rankHint    = iota // a passing look: level up, a wall brushing past, the rare sweet ahead
	rankSmall          // a sweet, a long way without one
	rankGood           // a macaron, a lost rare sweet, an extra life
	rankCombo          // five sweets in a row, out of danger
	rankBig            // the rare sweet, a hammer, getting back up
	rankPerfect        // a stage cleared, a crash
)

func (s *playScene) react(expr string, frames, rank int) {
	if s.reactTimer > 0 && rank < s.reactRank {
		return
	}
	s.reactExpr = expr
	s.reactTimer = frames
	s.reactRank = rank
	s.repick = true
	s.popNext = true
	s.montage = nil
	if rank >= rankBig {
		s.montage = s.montageFor(expr) // the big moments flash through their poses
		s.montageTimer = 0
	}
	if rank >= rankCombo {
		s.hop = 1
		s.windup = windupFrames // crouch for a moment before springing into the reaction
	}
}

// readRoad reacts to the road ahead: relief when a narrow stretch is behind, a look
// of anticipation when the rare sweet comes into sight, and drought when no sweet has
// been picked up for a long way.
func (s *playScene) readRoad() {
	e := s.eng
	d := e.Danger()
	if s.danger >= 3 && d <= 1 {
		s.react(character.ExprRelief, 120, rankCombo) // through the narrow stretch
	}
	s.danger = d
	if e.G.SweetAhead(10, road.SweetOneUp) {
		if !s.oneUpSeen {
			s.oneUpSeen = true
			s.react(character.ExprWaiting, 80, rankHint) // an extra life is coming
		}
	} else {
		s.oneUpSeen = false
	}
	if s.eng.Steps != s.lastSteps {
		s.sinceSweet += s.eng.Steps - s.lastSteps
		s.lastSteps = s.eng.Steps
	}
	if !s.droughtSeen && s.sinceSweet >= 60 {
		s.droughtSeen = true
		s.react(character.ExprDrought, 100, rankSmall)
	}
}

// updateExpression picks the expression from how tight the road is and recent events.
func (s *playScene) updateExpression() {
	// After a miss, while the player decides, she keeps one crying pose.
	if s.eng.G.Missed && s.expr == character.ExprCrying {
		if s.popFrame >= 0 {
			s.popFrame++
		}
		return
	}
	want := character.ExprNormal
	d := s.eng.Danger()
	switch {
	case s.eng.Over():
		want = character.ExprGameOver
	case s.reactTimer > 0 && (d < 3 || s.reactRank >= rankCombo):
		// Reactions win, except that small ones do not hide panic.
		want = s.reactExpr
	case d >= 4:
		want = character.ExprPanic // the narrowest road on the last life
	case d >= 3:
		want = character.ExprNervous
	case d >= 2:
		want = character.ExprWorried
	case d == 0 && s.eng.G.Lives == road.StartLives:
		want = character.ExprRelaxed
	}
	if s.reactTimer > 0 {
		s.reactTimer--
	}
	s.variantTimer++
	if s.popFrame >= 0 {
		s.popFrame++
	}
	// The crouch before a strong reaction: the current pose squashes, then the new one springs out.
	if s.windup > 0 && !s.eng.Over() {
		s.windup--
		return
	}
	s.windup = 0
	// A montage flashes its poses by, one every montageStep frames, while its reaction shows.
	if len(s.montage) > 0 {
		if want != s.reactExpr {
			s.montage = nil
		} else {
			if s.montageTimer%montageStep == 0 {
				id := s.montage[0]
				s.montage = s.montage[1:]
				s.setPose(want, id, true)
			}
			s.montageTimer++
			s.repick = false
			return
		}
	}
	// Re-pick a pose when the situation changes, on a new reaction, or after the same
	// situation lasts a while (shorter the tighter the road is)
	if want != s.expr || s.repick || s.variantTimer > poseInterval(want) {
		pop := s.popNext || (want != s.expr && reactionExpr(want))
		s.repick, s.popNext = false, false
		id := s.pickVariant(want)
		if want == character.ExprCombo {
			id = s.comboPose(s.comboStep)
		}
		s.setPose(want, id, pop)
	}
}

// reactionExpr reports whether expr is a reaction (as opposed to a mood of the road ahead).
func reactionExpr(expr string) bool {
	switch expr {
	case character.ExprNormal, character.ExprRelaxed, character.ExprWorried, character.ExprNervous, character.ExprPanic, character.ExprCrying:
		return false
	}
	return true
}

// setPose shows the pose id of the situation expr: popping in (pop) or cross-fading from
// the previous pose.
func (s *playScene) setPose(expr, id string, pop bool) {
	s.variantTimer = 0
	if id == s.exprID {
		s.expr = expr
		return // only one pose
	}
	s.prevExpr, s.prevID = s.expr, s.exprID
	s.expr, s.exprID = expr, id
	s.exprFade = 0
	s.popFrame = -1
	if pop {
		s.popFrame = 0
		s.slideDir = -s.slideDir
		if s.slideDir == 0 {
			s.slideDir = 1
		}
	}
	if !s.prog.SeenExpr[s.exprID] {
		s.prog.SeenExpr[s.exprID] = true
		store.Mark()
	}
}

// oncePerRun are the situations shown only once a run (the hammer show, READY): a pose of
// theirs not shown yet comes before the others, or a few of them would wait many runs to
// be seen (and opened in the gallery).
var oncePerRun = map[string]bool{character.ExprBlocked: true, character.ExprReady: true}

// pickVariant picks a random pose for state, avoiding the current pose when possible.
func (s *playScene) pickVariant(state string) string {
	vs := s.char.Variants(state)
	if oncePerRun[state] {
		for _, v := range vs {
			if !s.prog.SeenExpr[v.ID] && v.ID != s.exprID {
				return v.ID
			}
		}
	}
	for range 4 {
		id := vs[rand.IntN(len(vs))].ID //nolint:gosec // G404: game randomness, not security sensitive
		if id != s.exprID {
			return id
		}
	}
	return vs[0].ID
}

// moodBackground maps each expression to a solid background color that reflects its mood.
var moodBackground = map[string]color.NRGBA{
	character.ExprNormal:   popMint,
	character.ExprHappy:    popYellow,
	character.ExprExcited:  popPink,
	character.ExprWorried:  popLavender,
	character.ExprPanic:    popOrange,
	character.ExprGameOver: popGray,
}

// moodFamily groups the situations into six families that share a background color and
// frame image. A reaction (character.StandIn) is in the family of the situation whose portraits
// stand in for it, so the two tables cannot disagree.
var moodFamily = map[string]string{
	character.ExprRelaxed: character.ExprNormal, character.ExprGreat: character.ExprHappy, character.ExprTreat: character.ExprExcited, character.ExprCombo: character.ExprExcited,
	character.ExprPerfect: character.ExprExcited, character.ExprNervous: character.ExprWorried, character.ExprCrying: character.ExprPanic,
}

// family is the family of the expression expr (see moodFamily).
func family(expr string) string {
	if stand, ok := character.StandIn(expr); ok {
		expr = stand
	}
	if f, ok := moodFamily[expr]; ok {
		return f
	}
	return expr
}
