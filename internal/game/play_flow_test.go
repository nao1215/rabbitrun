package game

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/road"
)

// The screens over the road, played through their menus: the miss (RETRY and GIVE UP),
// the game over, the pause menu and the ending. Each run starts on the road with READY
// over, and the screen is drawn on the way.

// startRun puts g on a run of the main character with READY over (no hammer show).
func startRun(t *testing.T, g *Game) *playScene {
	t.Helper()
	s := newPlayScene(characters[defaultCharIndex()])
	s.ready = 0
	g.SetScene(s)
	return s
}

// wallAcross puts a wall across the whole road just ahead of her: she cannot avoid it.
func wallAcross(s *playScene) {
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0]
	}
}

// missOn runs s into a wall across the road and on until the miss menu takes input.
func missOn(t *testing.T, g *Game, s *playScene, screen *ebiten.Image) {
	t.Helper()
	wallAcross(s)
	playUntil(t, g, screen, 5*60, func() bool { return s.eng.G.Missed })
	playDrawn(t, g, screen, wait(missFrames))
}

//nolint:paralleltest // shares the save data and the characters
func TestMissMenu(t *testing.T) {
	t.Run("RETRY uses a life and starts the course over", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		lives := s.eng.G.Lives
		missOn(t, g, s, screen)
		// down to GIVE UP and back up: the menu has two buttons
		playDrawn(t, g, screen, press(input.Down))
		if s.missSel != 1 {
			t.Fatalf("on button %d after down, want GIVE UP", s.missSel)
		}
		playDrawn(t, g, screen, press(input.Up, input.Confirm))
		if s.countdown == 0 {
			t.Fatal("RETRY did not start the countdown")
		}
		// a press of pause during the countdown does not pause the miss screen
		playDrawn(t, g, screen, press(input.Pause))
		if s.paused {
			t.Fatal("paused during the countdown")
		}
		playUntil(t, g, screen, 2*countdownFrames, func() bool { return !s.eng.G.Missed })
		if s.eng.G.Lives != lives-1 {
			t.Errorf("%d lives after RETRY, want %d", s.eng.G.Lives, lives-1)
		}
		if s.ready == 0 || s.countdown != 0 {
			t.Errorf("READY %d, countdown %d: the retry did not start over", s.ready, s.countdown)
		}
		if !hasPopup(s, "FROM ") {
			t.Errorf("no popup of where the retry starts: %+v", s.popups)
		}
		playDrawn(t, g, screen, wait(readyFr+10))
		if s.ready != 0 {
			t.Error("READY did not end after the retry")
		}
	})

	t.Run("GIVE UP ends the game", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		missOn(t, g, s, screen)
		playDrawn(t, g, screen, press(input.Down, input.Confirm))
		if !s.eng.Over() || s.allClear {
			t.Fatalf("over %v, all clear %v after GIVE UP", s.eng.Over(), s.allClear)
		}
		if s.expr != character.ExprGameOver {
			t.Errorf("expression %s after GIVE UP, want %s", s.expr, character.ExprGameOver)
		}
		if p := reloadSave(t).Characters[heroID]; p == nil || p.BestStage < 1 {
			t.Errorf("the run was not recorded: %+v", p)
		}
	})

	t.Run("the demo always retries", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		s.auto = &engine.AutoPlayer{}
		lives := s.eng.G.Lives
		wallAcross(s)
		playUntil(t, g, screen, 5*60, func() bool { return s.eng.G.Missed })
		playUntil(t, g, screen, missFrames+2*countdownFrames, func() bool { return !s.eng.G.Missed })
		if s.eng.G.Lives != lives-1 {
			t.Errorf("%d lives after the demo's retry, want %d", s.eng.G.Lives, lives-1)
		}
	})
}

// hasPopup reports whether a popup of s starts with prefix.
func hasPopup(s *playScene, prefix string) bool {
	for _, p := range s.popups {
		if strings.HasPrefix(p.text, prefix) {
			return true
		}
	}
	return false
}

// TestGameOverMenu gives up after a miss with no life left and picks each button of the
// game over menu once the curtain is down.
//
//nolint:paralleltest // shares the save data and the characters
func TestGameOverMenu(t *testing.T) {
	cases := []struct {
		name  string
		input []scriptFrame
		check func(t *testing.T, g *Game, over *playScene)
	}{
		{
			name:  "RETRY starts a new run with her getting back up",
			input: press(input.Confirm),
			check: func(t *testing.T, g *Game, over *playScene) {
				t.Helper()
				s := playOf(t, g)
				if s == over || s.char != over.char {
					t.Fatalf("not a new run on %s", over.char.ID)
				}
				// a frame of the new run has gone by
				if s.comeback == 0 || s.comeback > comebackDelay || s.expr != character.ExprGameOver {
					t.Errorf("comeback %d in %s, want her down in %s and getting up", s.comeback, s.expr, character.ExprGameOver)
				}
			},
		},
		{
			name:  "SELECT goes to the select screen",
			input: press(input.Down, input.Confirm),
			check: func(t *testing.T, g *Game, _ *playScene) {
				t.Helper()
				if s, ok := g.scene.(*charSelectScene); !ok || s.mode != modePlay {
					t.Fatalf("got %T, want the play select screen", g.scene)
				}
			},
		},
		{
			name:  "TITLE goes back to the title",
			input: press(input.Up, input.Confirm),
			check: func(t *testing.T, g *Game, _ *playScene) {
				t.Helper()
				titleOf(t, g)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, screen := newDrawScenario(t, nil)
			s := startRun(t, g)
			s.eng.G.Lives = 0
			wallAcross(s)
			playUntil(t, g, screen, 5*60, s.eng.Over)
			// a press before the curtain is down does nothing
			playDrawn(t, g, screen, press(input.Confirm))
			if g.scene != s {
				t.Fatalf("the game over menu took a press before the curtain was down (%T)", g.scene)
			}
			playDrawn(t, g, screen, wait(curtainStart+curtainFrames+10))
			playDrawn(t, g, screen, tc.input)
			tc.check(t, g, s)
			// the next screen draws too
			playDrawn(t, g, screen, wait(drawEvery+1))
		})
	}
}

// TestRetryGetsBackUp lets the run after a game over's RETRY count down its READY: she
// gets up from her game over pose with her comeback pose.
func TestRetryGetsBackUp(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil)
	s := newRetryScene(characters[defaultCharIndex()])
	g.SetScene(s)
	playUntil(t, g, screen, readyFr+10, func() bool { return s.ready == 0 })
	if s.comeback != 0 {
		t.Errorf("comeback still %d after READY", s.comeback)
	}
	if s.expr == character.ExprGameOver {
		t.Error("she is still down after READY")
	}
}

// TestPauseMenu opens the pause menu on the road and picks each of its buttons.
//
//nolint:paralleltest // shares the save data and the characters
func TestPauseMenu(t *testing.T) {
	t.Run("cancel and pause both resume", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		for _, a := range []input.Action{input.Cancel, input.Pause} {
			playDrawn(t, g, screen, press(input.Pause))
			if !s.paused {
				t.Fatal("pause did not pause")
			}
			steps := s.eng.Steps
			playDrawn(t, g, screen, wait(30))
			if s.eng.Steps != steps {
				t.Fatal("the road moved while paused")
			}
			playDrawn(t, g, screen, press(a))
			if s.paused {
				t.Fatalf("action %d did not resume", a)
			}
		}
	})

	t.Run("RESET starts the run over with the hammer show", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		playDrawn(t, g, screen, wait(60))
		playDrawn(t, g, screen, press(input.Pause, input.Down, input.Confirm))
		p := playOf(t, g)
		if p == s || !p.showing || p.char != s.char {
			t.Fatalf("new run %v, show %v: RESET did not start over", p != s, p.showing)
		}
		if !s.committed {
			t.Error("the run that was reset was not recorded")
		}
		playDrawn(t, g, screen, intro())
		if p.showing {
			t.Error("the hammer show did not end")
		}
	})
}

// TestEndingGoesBackToTitle runs the last course to the end: the ending takes no press
// until its picture is in, and then goes back to the title, with the clear saved.
func TestEndingGoesBackToTitle(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil)
	allClearScene(g)
	s := playOf(t, g)
	playUntil(t, g, screen, 10*60, func() bool { return s.allClear })
	playDrawn(t, g, screen, press(input.Confirm))
	if g.scene != s {
		t.Fatal("the ending took a press before its picture was in")
	}
	playDrawn(t, g, screen, wait(endWordsAt+endBandFrames))
	playDrawn(t, g, screen, press(input.Confirm))
	titleOf(t, g)
	if p := reloadSave(t).Characters[heroID]; p == nil || !p.Cleared {
		t.Errorf("the clear was not saved: %+v", p)
	}
}

// TestEndingWithoutIllustration draws the ending of a character with no illustration: she
// stands in her portrait instead.
func TestEndingWithoutIllustration(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil)
	c := *characters[defaultCharIndex()]
	c.Ending, c.EndingExtra, c.CGs = nil, nil, nil
	s := newPlayScene(&c)
	s.ready = 0
	s.allClearNow()
	g.SetScene(s)
	playDrawn(t, g, screen, wait(endWordsAt+endBandFrames))
	if s.endLayer == nil {
		t.Error("the ending did not draw her portrait")
	}
	if !s.prog.Cleared {
		t.Error("the all clear was not recorded")
	}
}

// TestSteering holds the directions on the road: right and left slide her, up speeds
// the road up for good, and confirm swings a hammer from the stock.
//
//nolint:paralleltest // shares the save data and the characters
func TestSteering(t *testing.T) {
	hold := func(a input.Action, n int) []scriptFrame {
		out := make([]scriptFrame, n)
		for i := range out {
			out[i].held = []input.Action{a}
		}
		return out
	}
	t.Run("left and right", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		s.eng.G.Safe = 1 << 30 // walls pass through
		x := s.eng.G.X
		playDrawn(t, g, screen, hold(input.Right, 20))
		if s.eng.G.X <= x {
			t.Errorf("x went from %v to %v holding right", x, s.eng.G.X)
		}
		x = s.eng.G.X
		playDrawn(t, g, screen, hold(input.Left, 20))
		if s.eng.G.X >= x {
			t.Errorf("x went from %v to %v holding left", x, s.eng.G.X)
		}
	})
	t.Run("up speeds the road up", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		s.eng.G.Safe = 1 << 30
		playDrawn(t, g, screen, hold(input.Up, 30))
		if s.eng.Boost <= 1 {
			t.Errorf("boost %v after holding up", s.eng.Boost)
		}
	})
	t.Run("confirm swings a hammer", func(t *testing.T) {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		for y := range road.Rows {
			for _, x := range []int{0, 1, 7, 8} {
				s.eng.G.Rows[y][x].Wall = road.CourseColors[0]
			}
		}
		hammers := s.eng.G.Bombs
		playDrawn(t, g, screen, press(input.Confirm))
		if s.eng.G.Bombs != hammers-1 || s.cutin == 0 {
			t.Fatalf("hammers %d -> %d, cut-in %d", hammers, s.eng.G.Bombs, s.cutin)
		}
		playUntil(t, g, screen, cutinFrames+(road.Rows+2)*crumbleStep+crumbleFly, func() bool {
			return s.cutin == 0 && !s.breaking()
		})
		for y := range road.Rows {
			if rowHasWall(s.eng.G.Rows[y]) {
				t.Fatalf("row %d still has a wall after the hammer", y)
			}
		}
		// with the stock empty, another press swings nothing
		s.eng.G.Bombs = 0
		playDrawn(t, g, screen, press(input.Confirm))
		if s.cutin != 0 {
			t.Error("a hammer swung with none in stock")
		}
	})
}
