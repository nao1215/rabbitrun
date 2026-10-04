package main

import (
	"os"
	"testing"

	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/save"
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// The scenario tests play the whole game the way a player does: a script stands in for
// the keyboard (input.Input.SetScript), and Game.Update runs frame by frame from the title.

// script is an input.Script that plays its frames in order, and then nothing.
type script struct {
	frames []scriptFrame
}

// scriptFrame is one frame of a script: the actions held and the letters typed.
type scriptFrame struct {
	held  []input.Action
	typed string
}

func (s *script) Frame() (held [input.NumActions]bool, typed []rune) {
	if len(s.frames) == 0 {
		return held, nil
	}
	f := s.frames[0]
	s.frames = s.frames[1:]
	for _, a := range f.held {
		held[a] = true
	}
	return held, []rune(f.typed)
}

// pressNow presses a on in for one frame, as the player does just now.
func pressNow(in *input.Input, a input.Action) {
	in.SetScript(&script{frames: []scriptFrame{{held: []input.Action{a}}}})
	in.Update()
}

// press presses each action in turn, letting go for a frame after each, so every one is a
// fresh press (and a step of a menu).
func press(actions ...input.Action) []scriptFrame {
	out := make([]scriptFrame, 0, 2*len(actions))
	for _, a := range actions {
		out = append(out, scriptFrame{held: []input.Action{a}}, scriptFrame{})
	}
	return out
}

// typeText types text, one letter a frame.
func typeText(text string) []scriptFrame {
	out := make([]scriptFrame, 0, len(text))
	for _, r := range text {
		out = append(out, scriptFrame{typed: string(r)})
	}
	return out
}

// intro waits out the hammer show that opens a run, to READY (the pause menu takes no
// press before it).
func intro() []scriptFrame {
	return wait(showHold + cutinFrames + (road.Rows+1)*crumbleStep + crumbleFly + 5)
}

// wait does nothing for n frames.
func wait(n int) []scriptFrame { return make([]scriptFrame, n) }

// steps joins parts of a script.
func steps(parts ...[]scriptFrame) []scriptFrame {
	var out []scriptFrame
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// newScenario starts the game on the title, on a save of its own (prepare fills it in
// first, as the progress of earlier games).
func newScenario(t *testing.T, prepare func()) *Game {
	t.Helper()
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	return newScenarioWith(t, chars, prepare)
}

// newScenarioWith is newScenario on the characters chars.
func newScenarioWith(t *testing.T, chars []*Character, prepare func()) *Game {
	t.Helper()
	useTempConfig(t)
	sound.SetMuted(true)
	old := characters
	characters = chars
	t.Cleanup(func() { characters = old })
	if prepare != nil {
		prepare()
		store.Write()
	}
	bg = newBackground()
	g := &Game{bg: bg, scene: newTitleScene()}
	g.in.SetScript(&script{})
	return g
}

// play runs the frames of the script on g.
func play(t *testing.T, g *Game, frames []scriptFrame) {
	t.Helper()
	g.in.SetScript(&script{frames: frames})
	for range frames {
		if err := g.Update(); err != nil {
			t.Fatalf("the game ended: %v", err)
		}
	}
}

// reloadSave reads the save file back, as the next launch does.
func reloadSave(t *testing.T) *save.Data {
	t.Helper()
	raw, err := os.ReadFile(save.Path())
	if err != nil {
		t.Fatal(err)
	}
	sd := &save.Data{}
	if err := save.Decode(sd, raw); err != nil {
		t.Fatal(err)
	}
	return sd
}

// charIndex is the index of the character id, or of the secret character for "".
func charIndex(t *testing.T, id string) int {
	t.Helper()
	for i, c := range characters {
		if c.ID == id || (id == "" && c.Secret) {
			return i
		}
	}
	t.Fatalf("no character %q", id)
	return -1
}

// toChar moves the select screen from the main character to the character id.
func toChar(t *testing.T, id string) []scriptFrame {
	t.Helper()
	n := (charIndex(t, id) - defaultCharIndex() + len(characters)) % len(characters)
	right := make([]input.Action, n)
	for i := range right {
		right[i] = input.Right
	}
	return press(right...)
}

// otherID is a regular character other than the main one (the last).
func otherID() string {
	id := ""
	for _, c := range characters {
		if !c.Secret && c.ID != heroID {
			id = c.ID
		}
	}
	return id
}

// clearRegulars marks every regular character as having cleared the regular stages.
func clearRegulars() {
	for _, c := range characters {
		if !c.Secret {
			progress(c.ID).Cleared = true
		}
	}
}

func titleOf(t *testing.T, g *Game) *TitleScene {
	t.Helper()
	s, ok := g.scene.(*TitleScene)
	if !ok {
		t.Fatalf("not on the title: %T", g.scene)
	}
	return s
}

func playOf(t *testing.T, g *Game) *PlayScene {
	t.Helper()
	s, ok := g.scene.(*PlayScene)
	if !ok {
		t.Fatalf("not in play: %T", g.scene)
	}
	return s
}

// TestScenarioMenus walks the menus from the title: to play and back through the pause
// menu, to the gallery, and back from the select screen.
//
//nolint:paralleltest // shares the save data and the characters
func TestScenarioMenus(t *testing.T) {
	cases := []struct {
		name  string
		input func(t *testing.T) []scriptFrame
		check func(t *testing.T, g *Game)
	}{
		{
			name:  "play opens the select screen on the main character",
			input: func(*testing.T) []scriptFrame { return press(input.Confirm) },
			check: func(t *testing.T, g *Game) {
				t.Helper()
				s, ok := g.scene.(*CharSelectScene)
				if !ok || s.mode != modePlay || characters[s.sel].ID != heroID {
					t.Fatalf("got %T %+v, want the play select screen on %s", g.scene, s, heroID)
				}
			},
		},
		{
			name:  "cancel on the select screen goes back to the title",
			input: func(*testing.T) []scriptFrame { return press(input.Confirm, input.Cancel) },
			check: func(t *testing.T, g *Game) {
				t.Helper()
				titleOf(t, g)
			},
		},
		{
			name: "a run starts on the chosen character",
			input: func(t *testing.T) []scriptFrame {
				t.Helper()
				return steps(press(input.Confirm), toChar(t, otherID()), press(input.Confirm))
			},
			check: func(t *testing.T, g *Game) {
				t.Helper()
				if s := playOf(t, g); s.char.ID != otherID() || s.paused {
					t.Fatalf("playing %s (paused %v), want %s", s.char.ID, s.paused, otherID())
				}
			},
		},
		{
			name: "pause holds the run",
			input: func(*testing.T) []scriptFrame {
				return steps(press(input.Confirm, input.Confirm), intro(), press(input.Pause))
			},
			check: func(t *testing.T, g *Game) {
				t.Helper()
				s := playOf(t, g)
				before := s.ready
				play(t, g, wait(20))
				if !s.paused || s.showing || s.ready == 0 || s.ready != before {
					t.Fatalf("paused %v, READY went from %d to %d", s.paused, before, s.ready)
				}
			},
		},
		{
			name: "continue goes on with the run",
			input: func(*testing.T) []scriptFrame {
				return steps(press(input.Confirm, input.Confirm), intro(), press(input.Pause, input.Confirm))
			},
			check: func(t *testing.T, g *Game) {
				t.Helper()
				if s := playOf(t, g); s.paused {
					t.Fatal("still paused after CONTINUE")
				}
			},
		},
		{
			name: "TITLE on the pause menu goes back to the title",
			input: func(*testing.T) []scriptFrame {
				return steps(press(input.Confirm, input.Confirm), intro(), press(input.Pause, input.Down, input.Down, input.Confirm))
			},
			check: func(t *testing.T, g *Game) {
				t.Helper()
				if s := titleOf(t, g); s.sel != 0 {
					t.Fatalf("the title opened on item %d", s.sel)
				}
				if p := reloadSave(t).Characters[heroID]; p == nil || p.BestStage != 1 {
					t.Fatalf("the run was not recorded: %+v", p)
				}
			},
		},
		{
			name:  "the menu wraps around from the top",
			input: func(*testing.T) []scriptFrame { return press(input.Up) },
			check: func(t *testing.T, g *Game) {
				t.Helper()
				if s := titleOf(t, g); titleItems[s.sel] != "EXIT" {
					t.Fatalf("on %s, want EXIT", titleItems[s.sel])
				}
			},
		},
		{
			name:  "the gallery and back",
			input: func(*testing.T) []scriptFrame { return press(input.Down, input.Confirm) },
			check: func(t *testing.T, g *Game) {
				t.Helper()
				if _, ok := g.scene.(*GalleryScene); !ok {
					t.Fatalf("not in the gallery: %T", g.scene)
				}
				play(t, g, press(input.Cancel))
				titleOf(t, g)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newScenario(t, nil)
			play(t, g, tc.input(t))
			tc.check(t, g)
		})
	}
}

// TestScenarioSecretWord types the secret word on the title: in capitals it switches to
// the extra stages and back, and the gallery remembers it was found (saved), while the
// switch itself is not saved (every launch starts on the regular side). The code does
// not wait for the word to be told: it works on the title menu from the first launch.
//
//nolint:paralleltest // shares the save data and the characters
func TestScenarioSecretWord(t *testing.T) {
	cases := []struct {
		name      string
		prepare   func()
		input     []scriptFrame
		wantFound bool
		wantExtra bool
	}{
		{name: "in capitals", input: typeText(secretWord), wantFound: true, wantExtra: true},
		{name: "all at once", input: []scriptFrame{{typed: secretWord}}, wantFound: true, wantExtra: true},
		{name: "twice switches back", input: typeText(secretWord + secretWord), wantFound: true},
		{name: "after other letters", input: typeText("HELLO" + secretWord), wantFound: true, wantExtra: true},
		{name: "in small letters", input: typeText("rabbitrun")},
		{name: "mixed case", input: typeText("RabbitRun")},
		{name: "with a typo", input: typeText("RABBITRUM")},
		{name: "cut short", input: typeText(secretWord[:len(secretWord)-1])},
		{
			name: "while the word is being told",
			prepare: func() {
				clearRegulars()
				progress(secretID()).Cleared = true
				store.Data.Announced = map[string]bool{secretID(): true}
			},
			input: typeText(secretWord),
		},
		{
			name: "once the word was told",
			prepare: func() {
				clearRegulars()
				progress(secretID()).Cleared = true
				store.Data.Announced = map[string]bool{secretID(): true}
			},
			input:     steps(wait(wordWait+1), press(input.Confirm), typeText(secretWord)),
			wantFound: true, wantExtra: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newScenario(t, tc.prepare)
			play(t, g, tc.input)
			s := titleOf(t, g)
			if store.Data.ExtraFound != tc.wantFound || extraMode() != tc.wantExtra || s.group != tc.wantExtra {
				t.Fatalf("found %v, extra %v (title %v), want found %v, extra %v",
					store.Data.ExtraFound, extraMode(), s.group, tc.wantFound, tc.wantExtra)
			}
			if tc.wantFound {
				if s.sel != 0 {
					t.Errorf("the title is on item %d after the word, want PLAY", s.sel)
				}
				if sd := reloadSave(t); !sd.ExtraFound || sd.ExtraMode {
					t.Errorf("saved found %v, extra %v: want found, regular side", sd.ExtraFound, sd.ExtraMode)
				}
			}
			if tc.wantExtra {
				// the run started now is on the extra stages
				play(t, g, press(input.Confirm, input.Confirm))
				if !playOf(t, g).eng.G.Hard {
					t.Error("the run is not on the extra stages")
				}
			}
		})
	}
}

// secretID is the ID of the secret character.
func secretID() string {
	for _, c := range characters {
		if c.Secret {
			return c.ID
		}
	}
	return ""
}

// TestScenarioSecretCharacter tries to start a run on the secret character: she is locked
// until every regular character has cleared the regular stages; then the title brings
// her in once (saved), and she can be played.
//
//nolint:paralleltest // shares the save data and the characters
func TestScenarioSecretCharacter(t *testing.T) {
	cases := []struct {
		name     string
		prepare  func()
		input    func(t *testing.T) []scriptFrame
		playable bool
	}{
		{
			name: "locked on a new save",
			input: func(t *testing.T) []scriptFrame {
				t.Helper()
				return steps(press(input.Confirm), toChar(t, ""), press(input.Confirm))
			},
		},
		{
			name: "locked with one regular character left",
			prepare: func() {
				clearRegulars()
				progress(heroID).Cleared = false
			},
			input: func(t *testing.T) []scriptFrame {
				t.Helper()
				return steps(press(input.Confirm), toChar(t, ""), press(input.Confirm))
			},
		},
		{
			name:    "brought in on the title once everyone cleared",
			prepare: clearRegulars,
			input: func(t *testing.T) []scriptFrame {
				t.Helper()
				return steps(wait(revealWordsAt+21), press(input.Confirm, input.Confirm), toChar(t, ""), press(input.Confirm))
			},
			playable: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newScenario(t, tc.prepare)
			if secretID() == "" {
				t.Skip("no secret character")
			}
			play(t, g, tc.input(t))
			if !tc.playable {
				if s, ok := g.scene.(*CharSelectScene); !ok || characters[s.sel].ID != secretID() {
					t.Fatalf("got %T, want the select screen still on the locked character", g.scene)
				}
				return
			}
			if s := playOf(t, g); s.char.ID != secretID() {
				t.Fatalf("playing %s, want %s", s.char.ID, secretID())
			}
			if !reloadSave(t).Announced[secretID()] {
				t.Error("her arrival was not saved: the title would bring her in again")
			}
		})
	}
}

// TestScenarioGallerySkipsLockedCharacters switches characters in the gallery: a locked
// secret character is skipped.
func TestScenarioGallerySkipsLockedCharacters(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	if secretID() == "" {
		t.Skip("no secret character")
	}
	play(t, g, press(input.Down, input.Confirm))
	for range 2 * len(characters) {
		play(t, g, press(input.TabNext))
		if s, ok := g.scene.(*GalleryScene); !ok || s.char().Secret {
			t.Fatalf("got %T on a locked character", g.scene)
		}
	}
}

// TestScenarioAutoplayClearsACourseAndUnlocksItsIllustration starts a run from the title
// and lets the careful auto player run the first course: no miss, and the course's
// illustration is earned, saved, and open in the gallery (locked there before).
func TestScenarioAutoplayClearsACourseAndUnlocksItsIllustration(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	hero := characters[defaultCharIndex()]
	cgs := hero.MainCGs()
	if len(cgs) == 0 || !cgs[0].HasImage() {
		t.Skip("the first illustration is not drawn yet")
	}
	first := cgs[0].ID
	if open := galleryOpen(t, g, first); open {
		t.Fatal("the illustration is open in the gallery before it was earned")
	}

	play(t, g, press(input.Confirm, input.Confirm))
	s := playOf(t, g)
	s.auto = &autoPlayer{careful: true}
	g.in.SetScript(&script{})
	for f := 0; s.eng.Level() < 2; f++ {
		if f > 60*30 {
			t.Fatalf("the first course is not done after 30 seconds (on %s)", s.eng.Progress())
		}
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		if s.eng.G.Missed || s.eng.Over() {
			t.Fatalf("a miss on %s", s.eng.Progress())
		}
	}
	if !progress(hero.ID).UnlockedCG[first] {
		t.Fatal("clearing the course did not unlock its illustration")
	}
	if s.stageCG == nil || s.stageCG.ID != first {
		t.Error("the illustration is not behind the road on the next course")
	}
	if !reloadSave(t).Characters[hero.ID].UnlockedCG[first] {
		t.Error("the illustration was not saved")
	}

	s.auto = nil
	play(t, g, press(input.Pause, input.Down, input.Down, input.Confirm))
	if open := galleryOpen(t, g, first); !open {
		t.Error("the illustration is still locked in the gallery")
	}
}

// galleryOpen opens the gallery from the title on the main character and reports whether
// the illustration id can be viewed there; it goes back to the title.
func galleryOpen(t *testing.T, g *Game, id string) bool {
	t.Helper()
	titleOf(t, g)
	play(t, g, press(input.Down, input.Confirm))
	s, ok := g.scene.(*GalleryScene)
	if !ok {
		t.Fatalf("not in the gallery: %T", g.scene)
	}
	for range characters {
		if s.char().ID == heroID {
			break
		}
		play(t, g, press(input.TabNext))
	}
	found, open := false, false
	for i, it := range s.items() {
		if it.cg && it.e.ID == id {
			found, open = true, s.open[i]
		}
	}
	if !found {
		t.Fatalf("the gallery of %s does not list %s", s.char().ID, id)
	}
	play(t, g, press(input.Cancel))
	titleOf(t, g)
	return open
}
