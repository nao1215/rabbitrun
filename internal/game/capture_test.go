package game

import (
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// TestCaptureScenesAreOnRealCourses sets up the screenshots of play: each must be on a
// course the game can be on (its stage and course in step with its level), and the vault
// and the feast must be in view.
func TestCaptureScenesAreOnRealCourses(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := character.Read(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	old := characters
	characters = chars
	t.Cleanup(func() { characters = old })
	bg = newBackground()
	for _, st := range captureSteps {
		if !strings.HasPrefix(st.name, "play_") && !strings.HasPrefix(st.name, "allclear") {
			continue
		}
		g := &Game{bg: bg}
		st.setup(g)
		s, ok := g.scene.(*playScene)
		if !ok {
			t.Fatalf("%s: not a play scene", st.name)
		}
		e := s.eng.G
		lv := min(e.Level, engine.GameCourses) // one past the last course on the open road after it
		if e.Stage != (lv-1)/road.Courses+1 || e.Course != (lv-1)%road.Courses {
			t.Errorf("%s: level %d on stage %d, course %d", st.name, e.Level, e.Stage, e.Course)
		}
		switch st.name {
		case "play_vault":
			if !vaultOnScreen(e) {
				t.Errorf("%s: no vault in view", st.name)
			}
		case "play_feast":
			if !feastOnScreen(e) {
				t.Errorf("%s: no feast in view", st.name)
			}
		}
	}
}

// TestCaptureEnlargesAnIllustration runs a whole --capture on a fresh save, as it is run
// for the README, up to its gallery_cg screenshot: that screen shows an illustration
// enlarged. The scripted clears earn the main character's illustrations, while the gallery
// opened on the first character, who had none, so the screenshot showed the grid.
//
//nolint:paralleltest // shares the save data and the characters
func TestCaptureEnlargesAnIllustration(t *testing.T) {
	g, screen := newDrawScenario(t, nil)
	store.ReadOnly = true
	// the steps one after another as captureState runs them (without reading the screen
	// back, which only a running game can do)
	for _, st := range captureSteps {
		st.setup(g)
		for range st.wait {
			if err := g.Update(); err != nil {
				t.Fatalf("%s: the game ended: %v", st.name, err)
			}
			g.Draw(screen)
		}
		if st.name == "gallery_cg" {
			break
		}
	}
	s, ok := g.scene.(*galleryScene)
	if !ok {
		t.Fatalf("gallery_cg is on %T, want the gallery", g.scene)
	}
	if !s.viewing || !s.list[s.sel].cg {
		t.Errorf("gallery_cg shows the grid of %s, want an illustration enlarged", s.char().ID)
	}
}

// TestCaptureTitlesShowTheTitle captures on a save with a secret character's arrival and
// the secret word still to be told: the title and title_group screenshots show the title
// (and the group picture), not the arrival or the word. The title scene told what was due
// first, so on the player's own save those screenshots showed the arrival.
//
//nolint:paralleltest // shares the save data and the characters
func TestCaptureTitlesShowTheTitle(t *testing.T) {
	g, screen := newDrawScenario(t, func() {
		for _, c := range characters {
			progress(c.ID).Cleared = true // the secret one too: the word is due
		}
	})
	if s := newTitleScene(); s.reveal < 0 || !s.word {
		t.Fatalf("the save has nothing due on the title (reveal %d, word %v)", s.reveal, s.word)
	}
	store.ReadOnly = true
	for _, st := range captureSteps {
		if st.name != "title" && st.name != "title_group" {
			continue
		}
		st.setup(g)
		for range st.wait {
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
			g.Draw(screen)
		}
		s := titleOf(t, g)
		if s.reveal >= 0 || s.word {
			t.Errorf("%s shows what is due on the title (reveal %d, word %v), not the title", st.name, s.reveal, s.word)
		}
		if s.group != (st.name == "title_group") {
			t.Errorf("%s: group picture %v", st.name, s.group)
		}
	}
}
