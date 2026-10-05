package game

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
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

func TestDemoSwingsTheHammerOnce(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil)
	s := startRun(t, g)
	s.auto = &engine.AutoPlayer{}
	bombs := s.eng.G.Bombs
	playUntil(t, g, screen, 30*60, func() bool { return s.demoSwung })
	if s.cutin == 0 {
		t.Error("the demo swung its hammer without the cut-in")
	}
	if s.eng.G.Bombs != bombs-1 {
		t.Errorf("%d hammers after the demo's swing, want %d", s.eng.G.Bombs, bombs-1)
	}
	for range cutinFrames + demoHammerFrame {
		if err := g.Update(); err != nil {
			t.Fatalf("the game ended: %v", err)
		}
	}
	if s.eng.G.Bombs != bombs-1 {
		t.Errorf("the demo swung again: %d hammers, want %d", s.eng.G.Bombs, bombs-1)
	}
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
	s := newRetryScene(characters[defaultCharIndex()], "")
	g.SetScene(s)
	playUntil(t, g, screen, readyFr+10, func() bool { return s.ready == 0 })
	if s.comeback != 0 {
		t.Errorf("comeback still %d after READY", s.comeback)
	}
	if s.expr == character.ExprGameOver {
		t.Error("she is still down after READY")
	}
}

// TestRetryKeepsHerGameOverPose picks RETRY on the game over of every character with more
// than one game over pose, a few times each: the new run opens on the pose she was down in.
// It picked one of them at random, without noting it as seen, so a pose shown there could
// stay locked in the gallery.
func TestRetryKeepsHerGameOverPose(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	tried := 0
	for _, c := range characters {
		if len(c.Variants(character.ExprGameOver)) < 2 || locked(c) {
			continue
		}
		tried++
		for range 8 {
			over := newPlayScene(c)
			over.ready = 0
			g.SetScene(over)
			over.eng.G.Lives = 0
			wallAcross(over)
			for f := 0; !over.eng.Over(); f++ {
				if f > 5*60 {
					t.Fatal("no game over")
				}
				play(t, g, wait(1))
			}
			play(t, g, wait(curtainStart+curtainFrames+10))
			down := over.exprID
			play(t, g, press(input.Confirm))
			s := playOf(t, g)
			if s == over || s.exprID != down || !s.prog.SeenExpr[s.exprID] {
				t.Fatalf("%s: down in %s, the retry opens in %s (seen %v)", c.ID, down, s.exprID, s.prog.SeenExpr[s.exprID])
			}
		}
	}
	if tried == 0 {
		t.Skip("no character has more than one game over pose")
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

	// The Start button of a pad is both the pause button and a confirm button (input.padMap):
	// pressed again on the pause menu it resumes, whatever is chosen. On RESET or TITLE it
	// threw the run away.
	t.Run("the pause button of a pad resumes on any item", func(t *testing.T) {
		start := []scriptFrame{{held: []input.Action{input.Confirm, input.Pause}}, {}}
		for down := 1; down < len(pauseItems); down++ {
			g, screen := newDrawScenario(t, nil)
			s := startRun(t, g)
			playDrawn(t, g, screen, start)
			if !s.paused {
				t.Fatal("Start did not pause")
			}
			for range down {
				playDrawn(t, g, screen, press(input.Down))
			}
			playDrawn(t, g, screen, start)
			if g.scene != s || s.paused || s.committed {
				t.Errorf("Start on %s: the same run %v, paused %v, recorded %v", pauseItems[down], g.scene == s, s.paused, s.committed)
			}
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

// TestRetryAfterAMissGetsHerBackUp spends a life on RETRY after a miss: she gets back up
// in her comeback pose ("let's go!") as the road starts again, not in the READY pose. The
// crying of the miss was still counted as showing (its time stands still while the player
// chooses), so the weaker comeback could not take its place, and READY came instead.
//
//nolint:paralleltest // shares the save data and the characters
func TestRetryAfterAMissGetsHerBackUp(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	s := startRun(t, g)
	missOn(t, g, s, screen)
	playDrawn(t, g, screen, press(input.Confirm)) // RETRY
	playUntil(t, g, screen, 2*countdownFrames, func() bool { return !s.eng.G.Missed })
	comeback := false
	for range readyFr {
		play(t, g, wait(1))
		comeback = comeback || s.expr == character.ExprComeback
	}
	if !comeback {
		t.Errorf("she never got back up in her comeback pose after RETRY (showing %q)", s.expr)
	}
}

// TestHammeredWallsStandThroughTheCutIn swings a hammer at walls on the screen: they stand
// while the cut-in plays and then break row by row from the bottom up. The rule engine
// takes them off the road as the hammer is swung, and the board drew the road's walls
// during the cut-in, so they all vanished at once and came back whole when the breaking
// began.
//
//nolint:paralleltest // shares the save data and the characters
func TestHammeredWallsStandThroughTheCutIn(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	s := startRun(t, g)
	for y := range road.Rows {
		for _, x := range []int{0, 1, 7, 8} {
			s.eng.G.Rows[y][x].Wall = road.CourseColors[0]
		}
	}
	playDrawn(t, g, screen, press(input.Confirm))
	if s.cutin == 0 {
		t.Fatal("no hammer swung")
	}
	for s.cutin > 0 {
		for y := 1; y <= road.Rows; y++ {
			if s.wallAt(0, y) == 0 || s.wallAt(8, y) == 0 {
				t.Fatalf("the walls of row %d are gone %d frames into the cut-in, before they break", y, cutinFrames-s.cutin)
			}
		}
		playDrawn(t, g, screen, wait(1))
	}
	playUntil(t, g, screen, (road.Rows+2)*crumbleStep+crumbleFly, func() bool { return !s.breaking() })
	for y := range road.Rows + 1 {
		if s.wallAt(0, y) != 0 {
			t.Fatalf("row %d still has a wall after the breaking", y)
		}
	}
}

// TestCrashPlaysItsJingleOnce runs into a wall with a life left and with none: either way
// the game over jingle plays once. On the last life the crash played it and the game over
// played it again on the same frame, two at once, twice as loud.
//
//nolint:paralleltest // shares the save data and the characters
func TestCrashPlaysItsJingleOnce(t *testing.T) {
	for _, lives := range []int{2, 0} {
		g, screen := newDrawScenario(t, nil)
		s := startRun(t, g)
		s.eng.G.Lives = lives
		jingles := 0
		sound.Listen(func(e sound.Effect) {
			if e == sound.GameOver {
				jingles++
			}
		})
		t.Cleanup(func() { sound.Listen(nil) })
		wallAcross(s)
		playUntil(t, g, screen, 5*60, func() bool { return s.eng.G.Missed || s.eng.Over() })
		playDrawn(t, g, screen, wait(30))
		if jingles != 1 {
			t.Errorf("with %d lives left the crash played the game over jingle %d times, want once", lives, jingles)
		}
	}
}

// TestPauseHoldsThePopups pauses during the READY of a retry, for longer than the popup of
// where it starts (FROM x-y) lasts, and goes on: the popup is still there, as the road and
// READY are. Its time ran on behind the pause menu, so it was gone when the game went on.
//
//nolint:paralleltest // shares the save data and the characters
func TestPauseHoldsThePopups(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	s := startRun(t, g)
	missOn(t, g, s, screen)
	playDrawn(t, g, screen, press(input.Confirm)) // RETRY
	playUntil(t, g, screen, 2*countdownFrames, func() bool { return !s.eng.G.Missed })
	if !hasPopup(s, "FROM ") {
		t.Fatalf("no popup of where the retry starts: %+v", s.popups)
	}
	playDrawn(t, g, screen, press(input.Pause))
	if !s.paused {
		t.Fatal("not paused")
	}
	playDrawn(t, g, screen, steps(wait(150), press(input.Pause)))
	if s.paused {
		t.Fatal("still paused")
	}
	if !hasPopup(s, "FROM ") {
		t.Errorf("the popup of where the retry starts ran out behind the pause menu: %+v", s.popups)
	}
}

// TestMusicStopsAtAMissAndStartsAfterTheRetry runs into a wall with a life left, retries,
// and follows the music: it stops at the miss (the jingle plays on its own, as at a game
// over), stays off through the miss screen and the READY of the retry, as at the start of
// a run, and starts once READY is over. It played on under the jingle, the miss screen and
// READY, and was cut off and started over from its beginning as READY ended.
//
//nolint:paralleltest // shares the save data and the characters
func TestMusicStopsAtAMissAndStartsAfterTheRetry(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	playing := ""
	starts := 0
	sound.ListenMusic(func(song string) {
		playing = song
		if song != "" {
			starts++
		}
	})
	t.Cleanup(func() { sound.ListenMusic(nil) })
	s := newPlayScene(characters[defaultCharIndex()]) // with READY, without the hammer show
	g.SetScene(s)
	playDrawn(t, g, screen, wait(readyFr+1))
	if playing != sound.GameSong {
		t.Fatalf("the music after READY is %q, want the game song", playing)
	}
	for y := range road.Rows {
		s.eng.G.Rows[y] = road.Row{}
	}
	missOn(t, g, s, screen)
	if playing != "" {
		t.Errorf("the music plays on through the miss (%q)", playing)
	}
	starts = 0
	playDrawn(t, g, screen, press(input.Confirm)) // RETRY
	playUntil(t, g, screen, 2*countdownFrames, func() bool { return !s.eng.G.Missed })
	for s.ready > 1 {
		playDrawn(t, g, screen, wait(1))
		if playing != "" {
			t.Fatalf("the music plays during the READY of the retry (%q)", playing)
		}
	}
	playDrawn(t, g, screen, wait(2))
	if playing != sound.GameSong || starts != 1 {
		t.Errorf("after the READY of the retry the music is %q, started %d times, want the game song once", playing, starts)
	}
}
