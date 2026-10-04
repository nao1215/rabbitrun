package game

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/input"
)

// The reward for clearing everything: a picture of the title's own, never in the gallery.

// withTitleArt serves a picture as ui/title_complete over the game's assets for the test.
func withTitleArt(t *testing.T) {
	t.Helper()
	old := assets.FS()
	assets.Use(overlayFS{old, fstest.MapFS{"ui/" + titleCompleteArt + ".png": {Data: tinyPNG(t)}}})
	assets.ReleaseUI(titleCompleteArt)
	t.Cleanup(func() {
		assets.Use(old)
		assets.ReleaseUI(titleCompleteArt)
	})
}

// overlayFS serves the files of top, and the others from base.
type overlayFS struct{ base, top fs.FS }

func (o overlayFS) Open(name string) (fs.File, error) {
	if f, err := o.top.Open(name); err == nil {
		return f, nil
	}
	return o.base.Open(name)
}

// TestTitleRewardNeedsBothStagesOfEveryone: the title shows its own picture once every
// character, the secret one too, has cleared both the regular and the extra stages; it
// shows after the next launch, in the regular stages and in the extra ones, and the
// gallery never lists it.
//
//nolint:paralleltest // shares the save data and the characters
func TestTitleRewardNeedsBothStagesOfEveryone(t *testing.T) {
	clearAll := func(regular, extra bool, except string) func() {
		return func() {
			for _, c := range characters {
				if c.ID == except {
					continue
				}
				p := progress(c.ID)
				p.Cleared, p.ClearedExtra = regular, extra
			}
		}
	}
	cases := []struct {
		name    string
		prepare func()
		want    bool
	}{
		{"nothing cleared", nil, false},
		{"one character left", clearAll(true, true, heroID), false},
		{"the secret character left", func() { clearAll(true, true, secretID())() }, false},
		{"everyone cleared only the extra stages", clearAll(false, true, ""), false},
		{"everyone cleared only the regular stages", clearAll(true, false, ""), false},
		{"everyone cleared both", clearAll(true, true, ""), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTitleArt(t)
			newScenario(t, func() {
				// what the title tells once is told: the secret character's arrival, the word
				store.Data.Announced = map[string]bool{secretID(): true}
				store.Data.WordTold = true
				if tc.prepare != nil {
					tc.prepare()
				}
			})
			if secretID() == "" {
				t.Skip("no secret character")
			}
			for _, extra := range []bool{false, true} {
				g := relaunch(t) // the save as the next launch reads it
				store.Data.ExtraMode = extra && store.Data.ExtraFound
				play(t, g, wait(2))
				if got := titleComplete(); got != tc.want {
					t.Fatalf("extra %v: title picture %v, want %v", extra, got, tc.want)
				}
				if got := bg.image == titleCompleteArt; got != tc.want {
					t.Errorf("extra %v: the title background is %q", extra, bg.image)
				}
				gs := galleryOf(t, g, heroID)
				for _, it := range gs.items() {
					if it.e.ID == titleCompleteArt {
						t.Error("the gallery lists the title picture")
					}
				}
			}
		})
	}
}

// TestTitleRewardShowsAfterTheLastClear: the last character's clear of the extra stages
// brings the picture to the title as soon as the run goes back to it, and the next launch
// still has it.
func TestTitleRewardShowsAfterTheLastClear(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	withTitleArt(t)
	newScenario(t, func() {
		for _, c := range characters {
			p := progress(c.ID)
			p.Cleared, p.ClearedExtra = true, c.ID != heroID
		}
		store.Data.ExtraFound = true
		store.Data.Announced = map[string]bool{secretID(): true}
		store.Data.WordTold = true
	})
	g := relaunch(t)
	if titleComplete() {
		t.Fatal("the title has its picture before the last clear")
	}
	store.Data.ExtraMode = true
	s := newPlayScene(characters[defaultCharIndex()])
	s.ready = 0
	g.SetScene(s)
	s.allClearNow()
	for f := 0; ; f++ {
		if _, ok := g.scene.(*titleScene); ok {
			break
		}
		if f > 60*30 {
			t.Fatalf("the ending does not go back to the title (on %T)", g.scene)
		}
		play(t, g, press(input.Confirm))
	}
	play(t, g, wait(2))
	if !titleComplete() || bg.image != titleCompleteArt {
		t.Fatal("the title has no picture after the last clear")
	}
	g = relaunch(t) // the regular stages, as every launch starts
	play(t, g, wait(2))
	if store.Data.ExtraMode || !titleComplete() || bg.image != titleCompleteArt {
		t.Fatal("the title picture is gone after the next launch")
	}
}
