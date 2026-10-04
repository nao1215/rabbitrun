package game

import (
	"math"
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// bunnyOnRoad is where the bunny is drawn against the road (bunnyPose): her middle and her
// feet in cells from the top left of the road as it is drawn (scrolled by Engine.Scroll),
// and the phase of her hop (0 to 1).
func bunnyOnRoad(s *playScene) (x, y, hop float64) {
	cx, footY, dist := s.bunnyPose(0, 0)
	return cx / cell, footY/cell - s.eng.Scroll(), math.Mod(dist/2, 1)
}

// TestBunnyStaysWhereSheHitTheWall runs her into a wall from the side (with the row
// coming in before and past halfway down onto her, when the side is judged against the row
// below hers and then her own, see road.Game.sideRow) and from the front, and checks that
// she is drawn against the road where she touched the wall, on the frame of the miss and
// on the miss screen after it. The frame that slid her into a wall still scrolled the road
// on, so she was drawn lower than the wall she had touched (a whole row lower when the
// road reached its next row).
//
//nolint:paralleltest // shares the save data and the characters
func TestBunnyStaysWhereSheHitTheWall(t *testing.T) {
	const tol = 1e-9
	cases := []struct {
		name string
		// the scroll of the road to run into the wall at (the row in her row came in at 0)
		from, to float64
		side     bool
	}{
		{name: "side, the row coming in still above her body", from: 0.3, to: 0.5, side: true},
		{name: "side, the row coming in halfway down onto her", from: 0.8, to: 1, side: true},
		{name: "front", side: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newScenario(t, nil)
			s := startRun(t, g)
			e := s.eng
			for y := range road.Rows { // an open road, so nothing else is run into
				e.G.Rows[y] = road.Row{}
			}
			e.G.X = 4.5
			if !tc.side {
				e.G.Rows[road.PlayerRow-1][4].Wall = road.CourseColors[0]
				play(t, g, wait(1))
				for f := 0; !e.G.Missed; f++ {
					if f > 120 {
						t.Fatal("no miss on the wall ahead")
					}
					play(t, g, wait(1))
				}
				x, y, hop := bunnyOnRoad(s)
				play(t, g, wait(60))
				if x2, y2, hop2 := bunnyOnRoad(s); x2 != x || math.Abs(y2-y) > tol || math.Abs(hop2-hop) > tol {
					t.Errorf("drawn at (%v, %v) hop %v on the miss, then at (%v, %v) hop %v", x, y, hop, x2, y2, hop2)
				}
				return
			}
			for f := 0; e.G.Distance < 1 || e.Scroll() < tc.from || e.Scroll() >= tc.to; f++ { // after a step: the first row of a run has no row behind
				if f > 240 {
					t.Fatalf("the road never scrolled to %v (at %v)", tc.from, e.Scroll())
				}
				play(t, g, wait(1))
			}
			if e.G.Missed {
				t.Fatal("a miss on the open road")
			}
			// a wall right beside her on the left, in the row her side is judged against:
			// before halfway the row below hers (her own row is left open there), then hers
			sideRow := road.PlayerRow
			if e.G.SideRowBehind {
				sideRow = road.PlayerRow + 1
			}
			if want := tc.from < 0.5; e.G.SideRowBehind != want {
				t.Fatalf("judged against the row behind %v at scroll %v", e.G.SideRowBehind, e.Scroll())
			}
			e.G.Rows[sideRow][3].Wall = road.CourseColors[0]
			e.G.X = 4 + road.Half + 0.01
			_, y, hop := bunnyOnRoad(s)
			play(t, g, []scriptFrame{{held: []input.Action{input.Left}}})
			if !e.G.Missed {
				t.Fatalf("no miss on the wall beside her (x %v)", e.G.X)
			}
			x2, y2, hop2 := bunnyOnRoad(s)
			if math.Abs(y2-y) > tol || math.Abs(hop2-hop) > tol {
				t.Errorf("she touched the wall drawn at row %v hop %v, and the miss drew her at row %v hop %v", y, hop, y2, hop2)
			}
			play(t, g, wait(60))
			if x3, y3, hop3 := bunnyOnRoad(s); x3 != x2 || math.Abs(y3-y) > tol || math.Abs(hop3-hop) > tol {
				t.Errorf("on the miss screen she moved from (%v, %v) hop %v to (%v, %v) hop %v", x2, y, hop, x3, y3, hop3)
			}
		})
	}
}

func TestMissHoldsOnePose(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := character.Read(assets.FS())
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
		if s.exprID != id || s.expr != character.ExprCrying {
			t.Fatalf("the pose changed to %s (%s) while deciding", s.exprID, s.expr)
		}
	}
}

func TestPauseKeyDoesNothingOnTheMissScreen(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := character.Read(assets.FS())
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
	pressNow(&g.in, input.Pause) // Esc on the miss screen
	s.Update(g)
	if s.paused {
		t.Fatal("the pause menu opened behind the miss screen")
	}
}
