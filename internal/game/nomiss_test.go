package game

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/road"
)

// The no-miss clear: a run that clears all the courses of a side without running into a
// wall earns the character's no-miss picture of that side. The ending shows it in place of
// the usual picture, and the gallery lists it with her illustrations.

// noMissFiles serves the pictures data as the no-miss pictures of the character id (both
// sides) over the game's assets, under the .jpg name of the real ones (image.Decode reads
// the data by its content).
func noMissFiles(id string, data []byte) fstest.MapFS {
	return fstest.MapFS{
		"characters/" + id + "/images/" + character.NoMissID + ".jpg":      {Data: data},
		"characters/" + id + "/images/" + character.NoMissExtraID + ".jpg": {Data: data},
	}
}

// hideNoMissFS serves base without any no-miss picture: the characters as they are before
// the pictures are drawn.
type hideNoMissFS struct{ base fs.FS }

func (h hideNoMissFS) Open(name string) (fs.File, error) {
	if strings.Contains(name, "/images/"+character.NoMissID) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return h.base.Open(name)
}

// filesOverFS serves the files of top over base. Unlike overlayFS it takes only files
// from top: the directories (the list of characters) are base's.
type filesOverFS struct{ base, top fs.FS }

func (o filesOverFS) Open(name string) (fs.File, error) {
	if f, err := o.top.Open(name); err == nil {
		if st, err := f.Stat(); err == nil && !st.IsDir() {
			return f, nil
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}
	return o.base.Open(name)
}

// useAssets serves fsys as the assets directory for the test, and makes the characters
// read from it the game's.
func useAssets(t *testing.T, fsys fs.FS) {
	t.Helper()
	old, oldChars := assets.FS(), characters
	assets.Use(fsys)
	chars, err := character.Read(fsys)
	if err != nil {
		t.Fatal(err)
	}
	characters = chars
	t.Cleanup(func() {
		assets.Use(old)
		characters = oldChars
	})
}

// withNoMissArt serves a no-miss picture of both sides for the main character.
func withNoMissArt(t *testing.T) {
	t.Helper()
	useAssets(t, filesOverFS{assets.FS(), noMissFiles(heroID, tinyPNG(t))})
}

// withoutNoMissArt serves the assets with no no-miss picture for anyone.
func withoutNoMissArt(t *testing.T) {
	t.Helper()
	useAssets(t, hideNoMissFS{assets.FS()})
}

// noMissScenario starts the game on the title with the characters of the assets in use
// (useAssets), drawing included; prepare fills the save in first.
func noMissScenario(t *testing.T, prepare func()) (*Game, *ebiten.Image) {
	t.Helper()
	chars := characters
	g, screen := newDrawScenario(t, prepare) // loads the fonts (once) and the blocks
	characters = chars                       // in place of the shared ones (put back after the test)
	g.scene = newTitleScene()
	return g, screen
}

// startOnRoad puts a run of the main character on g, past the hammer show and READY.
func startOnRoad(g *Game) *playScene {
	s := newPlayScene(characters[defaultCharIndex()])
	s.ready = 0
	g.SetScene(s)
	return s
}

// runCourses runs the road of s (the walls pass through her) until n more courses are
// cleared or the run ends, handling what happens as play does.
func runCourses(t *testing.T, s *playScene, n int) {
	t.Helper()
	want := s.courses + n
	for f := 0; !s.allClear && s.courses < want; f++ {
		if f > 60*60*30 {
			t.Fatalf("the run is still on course %d after half an hour", s.eng.G.Level)
		}
		s.eng.G.Safe = 1 << 30
		s.eng.Tick(true)
		s.handleEvents()
	}
}

// clearRun runs the road of s to the ending.
func clearRun(t *testing.T, s *playScene) {
	t.Helper()
	runCourses(t, s, 1<<30)
	if !s.allClear {
		t.Fatal("the run did not reach the ending")
	}
}

// missAndRetry runs her into a wall across the road and picks RETRY on the miss screen,
// as the player does.
func missAndRetry(t *testing.T, g *Game, s *playScene) {
	t.Helper()
	s.eng.G.Safe = 0
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = 1
	}
	for f := 0; !s.eng.G.Missed; f++ {
		if f > 600 {
			t.Fatal("no miss")
		}
		play(t, g, wait(1))
	}
	play(t, g, wait(missFrames))
	play(t, g, press(input.Confirm)) // RETRY is the first button
	for f := 0; s.eng.G.Missed || s.countdown > 0; f++ {
		if f > countdownFrames+10 {
			t.Fatal("RETRY did not start over")
		}
		play(t, g, wait(1))
	}
	if s.eng.Over() {
		t.Fatal("RETRY ended the game")
	}
}

// endingToTitle waits for the ending's words, drawing it, and goes back to the title.
func endingToTitle(t *testing.T, g *Game, screen *ebiten.Image) {
	t.Helper()
	playDrawn(t, g, screen, wait(endWordsAt+endBandFrames))
	playDrawn(t, g, screen, press(input.Confirm))
	titleOf(t, g)
}

// listedAt is where the gallery s lists the entry id (-1: not listed).
func listedAt(s *galleryScene, id string) int {
	for i, it := range s.items() {
		if it.cg && it.e.ID == id {
			return i
		}
	}
	return -1
}

// TestNoMissClearUnlocksItsPicture runs the regular side to the end without a miss: the
// ending shows her no-miss picture with NO MISS!, the clear is saved under the regular
// key only, and after the next launch the gallery lists the picture open, right after the
// regular illustrations (the extra one is not listed before the hidden command).
func TestNoMissClearUnlocksItsPicture(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	withNoMissArt(t)
	g, screen := noMissScenario(t, nil)
	hero := characters[defaultCharIndex()]
	if hero.NoMiss == nil || hero.NoMissExtra == nil {
		t.Fatal("the no-miss pictures served are not read")
	}
	s := startOnRoad(g)
	clearRun(t, s)
	if s.misses != 0 || s.courses < engine.GameCourses || !s.noMissRun() {
		t.Fatalf("misses %d, courses %d: not a no-miss clear", s.misses, s.courses)
	}
	if s.ending() != hero.NoMiss || !s.noMissShown() {
		t.Errorf("the ending shows %v, not her no-miss picture", s.ending())
	}
	if !s.prog.UnlockedCG[character.NoMissID] || s.prog.UnlockedCG[character.NoMissExtraID] {
		t.Errorf("unlocked %v", s.prog.UnlockedCG)
	}
	endingToTitle(t, g, screen)
	saved := reloadSave(t).Characters[heroID]
	if saved == nil || !saved.Cleared || !saved.UnlockedCG[character.NoMissID] || saved.UnlockedCG[character.NoMissExtraID] {
		t.Fatalf("saved %+v", saved)
	}

	g = relaunch(t)
	gs := galleryOf(t, g, heroID)
	i := listedAt(gs, character.NoMissID)
	if i < 0 || !gs.open[i] {
		t.Fatalf("the gallery lists the no-miss picture at %d (open %v)", i, i >= 0 && gs.open[i])
	}
	if listedAt(gs, character.NoMissExtraID) >= 0 {
		t.Error("the gallery lists the extra no-miss picture before the hidden command")
	}
	main := map[*character.ImageEntry]bool{}
	for j := range hero.MainCGs() {
		main[&hero.MainCGs()[j]] = true
	}
	for j, it := range gs.items() {
		if it.cg && it.e != hero.NoMiss && main[it.e] != (j < i) {
			t.Errorf("%s is listed at %d, on the wrong side of the no-miss picture at %d", it.e.ID, j, i)
		}
	}
}

// TestMissThenRetryIsNoNoMissClear runs into a wall once, picks RETRY and clears the run:
// the clear is saved, the usual ending shows, and the no-miss picture stays locked.
func TestMissThenRetryIsNoNoMissClear(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	withNoMissArt(t)
	g, screen := noMissScenario(t, nil)
	hero := characters[defaultCharIndex()]
	s := startOnRoad(g)
	runCourses(t, s, 3)
	missAndRetry(t, g, s)
	clearRun(t, s)
	if s.misses != 1 || s.noMissRun() {
		t.Fatalf("misses %d, no-miss %v", s.misses, s.noMissRun())
	}
	if s.ending() != hero.Ending {
		t.Errorf("the ending shows %v, not the usual picture", s.ending())
	}
	endingToTitle(t, g, screen)
	saved := reloadSave(t).Characters[heroID]
	if saved == nil || !saved.Cleared || saved.UnlockedCG[character.NoMissID] {
		t.Fatalf("saved %+v", saved)
	}
	gs := galleryOf(t, relaunch(t), heroID)
	if i := listedAt(gs, character.NoMissID); i < 0 || gs.open[i] {
		t.Errorf("the no-miss picture is at %d, open %v: want a locked tile", i, i >= 0 && gs.open[i])
	}
}

// TestNewRunStartsWithoutMisses: a run after a game over (RETRY), after RESET or from the
// select screen counts its misses from zero; the misses of the run before are gone.
func TestNewRunStartsWithoutMisses(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g := newScenario(t, nil)
	s := startOnRoad(g)
	missAndRetry(t, g, s)
	if s.misses != 1 {
		t.Fatalf("misses %d after one miss", s.misses)
	}
	for _, next := range []*playScene{newRetryScene(s.char, ""), newRunScene(s.char), newPlayScene(s.char)} {
		if next.misses != 0 || next.courses != 0 {
			t.Errorf("a new run starts with %d misses and %d courses", next.misses, next.courses)
		}
	}
}

// TestNoMissClearOfTheExtraSide runs the extra side to the end without a miss: only the
// extra no-miss picture is earned and shown, and the gallery (which knows the extras now)
// lists it after the extra illustrations, the regular one still locked.
func TestNoMissClearOfTheExtraSide(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	withNoMissArt(t)
	g, screen := noMissScenario(t, func() { store.Data.ExtraFound = true })
	hero := characters[defaultCharIndex()]
	store.Data.ExtraMode = true
	s := startOnRoad(g)
	clearRun(t, s)
	if s.ending() != hero.NoMissExtra {
		t.Errorf("the ending shows %v, not her extra no-miss picture", s.ending())
	}
	endingToTitle(t, g, screen)
	saved := reloadSave(t).Characters[heroID]
	if saved == nil || !saved.ClearedExtra || saved.Cleared || !saved.UnlockedCG[character.NoMissExtraID] || saved.UnlockedCG[character.NoMissID] {
		t.Fatalf("saved %+v", saved)
	}
	gs := galleryOf(t, relaunch(t), heroID)
	regular, extra := listedAt(gs, character.NoMissID), listedAt(gs, character.NoMissExtraID)
	if regular < 0 || gs.open[regular] {
		t.Errorf("the regular no-miss picture is at %d: want a locked tile", regular)
	}
	if extra < 0 || !gs.open[extra] || extra != len(gs.items())-1 {
		t.Errorf("the extra no-miss picture is at %d of %d, want open and last", extra, len(gs.items()))
	}
	for j, it := range gs.items() {
		if it.cg && j > regular && j < extra && !inExtras(hero, it.e) {
			t.Errorf("%s is listed between the no-miss pictures", it.e.ID)
		}
	}
}

// inExtras reports whether e is one of c's extra illustrations.
func inExtras(c *character.Character, e *character.ImageEntry) bool {
	extra := c.ExtraCGs()
	for i := range extra {
		if &extra[i] == e {
			return true
		}
	}
	return false
}

// TestNoMissClearWithoutPicture: a character whose no-miss picture is not drawn yet still
// earns the clear (saved, so the picture opens once it is drawn), the usual ending shows
// without a crash, and the gallery lists nothing for it.
func TestNoMissClearWithoutPicture(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	withoutNoMissArt(t)
	g, screen := noMissScenario(t, nil)
	hero := characters[defaultCharIndex()]
	if hero.NoMiss != nil || hero.NoMissExtra != nil {
		t.Fatal("a no-miss picture is read although none is served")
	}
	s := startOnRoad(g)
	clearRun(t, s)
	if !s.noMissRun() || s.noMissShown() || s.ending() != hero.Ending {
		t.Fatalf("no-miss %v, shown %v, ending %v", s.noMissRun(), s.noMissShown(), s.ending())
	}
	endingToTitle(t, g, screen)
	if saved := reloadSave(t).Characters[heroID]; saved == nil || !saved.UnlockedCG[character.NoMissID] {
		t.Fatalf("saved %+v", saved)
	}
	gs := galleryOf(t, relaunch(t), heroID)
	if listedAt(gs, character.NoMissID) >= 0 || listedAt(gs, character.NoMissExtraID) >= 0 {
		t.Error("the gallery lists a no-miss picture that is not drawn")
	}
}

// TestGalleryNoMissBeforeTheClear: before any no-miss clear the gallery lists her drawn
// no-miss picture as a locked tile (the extra one only once the hidden command is found),
// and --debug opens both for its run.
//
//nolint:paralleltest // shares the save data and the characters
func TestGalleryNoMissBeforeTheClear(t *testing.T) {
	withNoMissArt(t)
	newScenario(t, nil) // reads the characters from the assets served
	cases := []struct {
		name          string
		opts          Options
		found         bool
		regular, extr int // -1 not listed, 0 locked, 1 open
	}{
		{"new save", Options{}, false, 0, -1},
		{"hidden command found", Options{}, true, 0, 0},
		{"debug", Options{Debug: true}, false, 1, 1},
	}
	state := func(gs *galleryScene, id string) int {
		i := listedAt(gs, id)
		if i < 0 {
			return -1
		}
		if gs.open[i] {
			return 1
		}
		return 0
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := launch(t, tc.opts)
			store.Data.ExtraFound = tc.found
			gs := galleryOf(t, g, heroID)
			if got := state(gs, character.NoMissID); got != tc.regular {
				t.Errorf("regular: %d, want %d", got, tc.regular)
			}
			if got := state(gs, character.NoMissExtraID); got != tc.extr {
				t.Errorf("extra: %d, want %d", got, tc.extr)
			}
		})
	}
}

// TestNoMissIsNotSavedByDebugOrCapture: a no-miss clear under --debug or --capture shows
// on the ending but is never written to the save; the scripted ending of the capture,
// which starts on the last course, is no no-miss clear at all.
//
//nolint:paralleltest // shares the save data and the characters
func TestNoMissIsNotSavedByDebugOrCapture(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"debug", Options{Debug: true}},
		{"capture", Options{CaptureDir: "unused"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withNoMissArt(t)
			newScenario(t, func() { progress(heroID).BestStage = 1 }) // a save file to read back
			g := launch(t, tc.opts)
			hero := characters[defaultCharIndex()]
			s := startOnRoad(g)
			clearRun(t, s)
			if s.ending() != hero.NoMiss {
				t.Errorf("the ending shows %v, not her no-miss picture", s.ending())
			}
			store.Flush()
			if saved := reloadSave(t).Characters[heroID]; saved.Cleared || saved.UnlockedCG[character.NoMissID] {
				t.Errorf("saved %+v", saved)
			}
		})
	}
	t.Run("the capture's ending", func(t *testing.T) {
		withNoMissArt(t)
		g := newScenario(t, nil)
		allClearScene(g)
		s := playOf(t, g)
		for f := 0; !s.allClear; f++ {
			if f > 60*60 {
				t.Fatal("no ending")
			}
			play(t, g, wait(1))
		}
		if s.noMissRun() || s.prog.UnlockedCG[character.NoMissID] || s.ending() != characters[defaultCharIndex()].Ending {
			t.Errorf("courses %d: a run from the last course counts as a no-miss clear", s.courses)
		}
	})
}
