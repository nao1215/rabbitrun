package game

import (
	"testing"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
)

// The collection: every picture the gallery lists can be earned in play, and what play
// earns is saved, read back by the next launch and open in the gallery.

// galleryOf opens the gallery from the title on the character id and returns it.
func galleryOf(t *testing.T, g *Game, id string) *galleryScene {
	t.Helper()
	titleOf(t, g)
	play(t, g, press(input.Down, input.Confirm))
	s, ok := g.scene.(*galleryScene)
	if !ok {
		t.Fatalf("not in the gallery: %T", g.scene)
	}
	for range characters {
		if s.char().ID == id {
			return s
		}
		play(t, g, press(input.TabNext))
	}
	t.Fatalf("the gallery does not show %s", id)
	return nil
}

// relaunch reads the save file back into a new store, as the next launch of the game does,
// and starts on the title.
func relaunch(t *testing.T) *Game {
	t.Helper()
	return launch(t, Options{})
}

// runToRoad starts a run of the character c from the select screen and plays it through
// the hammer show and READY, until the road moves.
func runToRoad(t *testing.T, g *Game, c *character.Character) *playScene {
	t.Helper()
	g.SetScene(newRunScene(c))
	s := playOf(t, g)
	play(t, g, intro())
	for f := 0; s.ready > 0; f++ {
		if f > readyFr+10 {
			t.Fatal("READY does not end")
		}
		play(t, g, wait(1))
	}
	return s
}

// TestReadyPosesAreEarnedInPlay: the portraits of the countdown (ready) are listed in the
// gallery, so play shows them: she takes a ready pose while READY counts down at the start
// of a run, a new one each run until she has shown them all. They are saved, read back by
// the next launch and open in the gallery.
func TestReadyPosesAreEarnedInPlay(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	hero := characters[defaultCharIndex()]
	var ready []*character.ImageEntry
	for i := range hero.Expressions {
		if e := &hero.Expressions[i]; e.State == character.ExprReady && e.HasImage() {
			ready = append(ready, e)
		}
	}
	if len(ready) == 0 {
		t.Skip("no ready portrait is drawn yet")
	}
	g.SetScene(newRunScene(hero))
	s := playOf(t, g)
	play(t, g, intro())
	if s.showing || s.ready == 0 {
		t.Fatalf("the hammer show is still on (%v) or READY is over (%d)", s.showing, s.ready)
	}
	play(t, g, wait(readyFr/2))
	if s.expr != character.ExprReady || hero.Expression(s.exprID).State != character.ExprReady {
		t.Fatalf("during READY she shows %s (%s), want a ready pose", s.expr, s.exprID)
	}
	for range len(ready) - 1 {
		runToRoad(t, g, hero)
	}
	for _, e := range ready {
		if !progress(hero.ID).SeenExpr[e.ID] {
			t.Errorf("%s not shown after %d runs", e.ID, len(ready))
		}
	}
	// the hammer show's poses (blocked) are shown once a run too: each run a new one
	if blocked := hero.Variants(character.ExprBlocked); len(blocked) <= len(ready) {
		for _, e := range blocked {
			if !progress(hero.ID).SeenExpr[e.ID] {
				t.Errorf("%s not shown after %d runs", e.ID, len(ready))
			}
		}
	}
	play(t, g, press(input.Pause, input.Down, input.Down, input.Confirm)) // back to the title
	g = relaunch(t)
	saved := store.Data.Progress(hero.ID)
	gs := galleryOf(t, g, hero.ID)
	for _, e := range ready {
		if !saved.SeenExpr[e.ID] {
			t.Errorf("%s was not saved", e.ID)
		}
		found := false
		for i, it := range gs.items() {
			if it.e.ID == e.ID && !it.cg {
				found = true
				if !gs.open[i] {
					t.Errorf("%s is locked in the gallery after the next launch", e.ID)
				}
			}
		}
		if !found {
			t.Errorf("the gallery does not list %s", e.ID)
		}
	}
}

// TestRetryStillGetsBackUpDuringREADY: the ready pose is for the start of a run; after a
// retry she springs back up (comeback) as before.
func TestRetryStillGetsBackUpDuringREADY(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	g.SetScene(newRetryScene(characters[defaultCharIndex()], ""))
	s := playOf(t, g)
	play(t, g, wait(comebackDelay+20))
	if s.expr != character.ExprComeback {
		t.Fatalf("after a retry she shows %s during READY, want comeback", s.expr)
	}
}

// TestEveryIllustrationIsEarnedThroughPlay clears every course of the regular and of the
// extra stages with the main character the way play does (courseClear), then launches
// again: every illustration is saved and open in the gallery, which lists all of them once
// the extra stages have been found.
func TestEveryIllustrationIsEarnedThroughPlay(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	newScenario(t, nil)
	hero := characters[defaultCharIndex()]
	for _, extra := range []bool{false, true} {
		store.Data.ExtraFound, store.Data.ExtraMode = extra, extra
		s := newPlayScene(hero)
		for n := 1; n < engine.GameCourses; n++ {
			s.courseClear(n)
		}
		s.courses = engine.GameCourses // every course of the run, without a miss
		s.allClearNow()
		s.release()
	}
	store.Data.ExtraMode = false
	store.Flush()

	g := relaunch(t)
	p := store.Data.Progress(hero.ID)
	if !p.Cleared || !p.ClearedExtra {
		t.Fatalf("cleared %v, cleared the extra stages %v after the next launch", p.Cleared, p.ClearedExtra)
	}
	for _, e := range hero.CGs {
		if !p.UnlockedCG[e.ID] {
			t.Errorf("%s was not earned", e.ID)
		}
	}
	gs := galleryOf(t, g, hero.ID)
	listed := 0
	for i, it := range gs.items() {
		if !it.cg {
			continue
		}
		listed++
		if !gs.open[i] {
			t.Errorf("%s is locked in the gallery", it.e.ID)
		}
	}
	drawn := drawnIllustrations(hero)
	if listed != drawn {
		t.Errorf("the gallery lists %d illustrations, want the %d drawn", listed, drawn)
	}
}

// drawnIllustrations counts the illustrations of c that are drawn: the ones a course
// unlocks and her no-miss pictures.
func drawnIllustrations(c *character.Character) int {
	n := 0
	for i := range c.CGs {
		if c.CGs[i].HasImage() {
			n++
		}
	}
	for _, e := range []*character.ImageEntry{c.NoMiss, c.NoMissExtra} {
		if e != nil && e.HasImage() {
			n++
		}
	}
	return n
}

// TestGalleryDebugListsTheExtras: --debug opens everything for its run, so the gallery
// lists the extra illustrations too, also before the hidden command has been found.
func TestGalleryDebugListsTheExtras(t *testing.T) { //nolint:paralleltest // changes the debug flag
	newScenario(t, nil)
	c := thirtyCGs()
	g := launch(t, Options{Debug: true})
	if n := len(galleryCGs(c)); n != len(c.CGs) {
		t.Fatalf("--debug lists %d illustrations of %d", n, len(c.CGs))
	}
	hero := characters[defaultCharIndex()]
	drawn := drawnIllustrations(hero)
	gs := galleryOf(t, g, hero.ID)
	listed := 0
	for i, it := range gs.items() {
		if it.cg {
			listed++
			if !gs.open[i] {
				t.Errorf("%s is locked under --debug", it.e.ID)
			}
		}
	}
	if listed != drawn {
		t.Errorf("--debug lists %d illustrations of %s, want the %d drawn", listed, hero.ID, drawn)
	}
	if store.Data.ExtraFound {
		t.Error("--debug marked the extra stages as found")
	}
}
