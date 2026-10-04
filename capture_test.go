package main

import (
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// TestCaptureScenesAreOnRealCourses sets up the screenshots of play: each must be on a
// course the game can be on (its stage and course in step with its level), and the vault
// and the feast must be in view.
func TestCaptureScenesAreOnRealCourses(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := readCharacters(assetFS)
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
		s, ok := g.scene.(*PlayScene)
		if !ok {
			t.Fatalf("%s: not a play scene", st.name)
		}
		e := s.eng.G
		lv := min(e.Level, GameCourses) // one past the last course on the open road after it
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
