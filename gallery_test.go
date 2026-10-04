package main

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// openGallery goes from the title to the gallery, drawing it.
func openGallery(t *testing.T, g *Game, screen *ebiten.Image) *GalleryScene {
	t.Helper()
	playDrawn(t, g, screen, press(ActDown, ActConfirm))
	s, ok := g.scene.(*GalleryScene)
	if !ok {
		t.Fatalf("not in the gallery: %T", g.scene)
	}
	return s
}

// TestGalleryGrid moves the selection about the grid: left and right step one tile and
// stop at the ends, up and down step a row, and a selection below the rows in view
// scrolls the grid by whole rows.
func TestGalleryGrid(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil) // locked tiles: none is decoded
	s := openGallery(t, g, screen)
	n := len(s.items())
	if n < 2*galleryCols {
		t.Skipf("the gallery of %s has only %d entries", s.char().ID, n)
	}
	cases := []struct {
		input []scriptFrame
		want  int
	}{
		{press(ActLeft), 0},
		{press(ActRight), 1},
		{press(ActDown), 1 + galleryCols},
		{press(ActUp), 1},
		{press(ActUp), 1},
		{press(ActLeft, ActLeft), 0},
	}
	for i, tc := range cases {
		playDrawn(t, g, screen, tc.input)
		if s.sel != tc.want {
			t.Fatalf("step %d: on tile %d, want %d", i, s.sel, tc.want)
		}
	}
	down := make([]Action, (n-1)/galleryCols+1)
	for i := range down {
		down[i] = ActDown
	}
	playDrawn(t, g, screen, press(down...))
	if s.sel/galleryCols != (n-1)/galleryCols {
		t.Errorf("on tile %d of %d after going all the way down", s.sel, n)
	}
	if row := (n - 1) / galleryCols; row >= galleryRows && s.scroll == 0 {
		t.Error("the grid did not scroll down to the last row")
	}
	playDrawn(t, g, screen, wait(30))
	if d := s.scroll - s.scrollView; d > 1 || d < -1 {
		t.Errorf("the grid shows %v, still far from its scroll %v", s.scrollView, s.scroll)
	}
}

// TestGalleryTilesAreMade waits for the tiles of an open portrait and an open
// illustration: each is decoded in the background and made into a tile, the
// illustration's once the grid has scrolled to it.
func TestGalleryTilesAreMade(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	var cg string
	g, screen := newDrawScenario(t, func() {
		c := characters[0] // the gallery opens on the first character
		for _, e := range c.GalleryCGs() {
			if e.HasImage() {
				cg = e.ID
				progress(c.ID).UnlockedCG[cg] = true
				break
			}
		}
	})
	if cg == "" {
		t.Skip("no illustration is drawn yet")
	}
	s := openGallery(t, g, screen)
	items := s.items()
	want := []int{0} // her usual pose is open from the start
	for i, it := range items {
		if it.cg && it.e.ID == cg {
			want = append(want, i)
		}
	}
	if len(want) != 2 || !s.open[want[0]] || !s.open[want[1]] {
		t.Fatalf("entries %v are not both open", want)
	}
	for _, i := range want { // made by an earlier test on the shared characters
		items[i].e.tile = nil
	}
	s.loading = nil
	playDrawn(t, g, screen, wait(2)) // the portrait's tile is asked for
	s.sel = want[1]
	g.in.script = &script{}
	deadline := time.Now().Add(30 * time.Second)
	for items[want[0]].e.tile == nil || items[want[1]].e.tile == nil {
		if time.Now().After(deadline) {
			t.Fatalf("the tiles are not made after 30 seconds (portrait %v, illustration %v)",
				items[want[0]].e.tile != nil, items[want[1]].e.tile != nil)
		}
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		g.Draw(screen) // asks for the pictures in view and makes the tiles of those decoded
		time.Sleep(10 * time.Millisecond)
	}
	if s.scroll == 0 && want[1] >= galleryCols*galleryRows {
		t.Error("the grid did not scroll to the illustration")
	}
}

// TestGalleryViewer opens an entry full screen and steps through the open ones: a
// locked entry is skipped, and a locked tile does not open.
func TestGalleryViewer(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, nil)
	s := openGallery(t, g, screen)
	n := len(s.items())
	if n < 3 {
		t.Skipf("the gallery of %s has only %d entries", s.char().ID, n)
	}
	// open all but the second entry, as if it had not been seen yet (on the screen the
	// tiles stay locked: nothing is decoded for them)
	for i := range s.open {
		s.open[i] = i != 1
	}

	playDrawn(t, g, screen, press(ActRight, ActConfirm))
	if s.viewing {
		t.Fatal("a locked tile opened")
	}
	playDrawn(t, g, screen, press(ActLeft, ActConfirm))
	if !s.viewing || s.sel != 0 {
		t.Fatalf("viewing %v on %d, want the first entry open", s.viewing, s.sel)
	}
	playDrawn(t, g, screen, press(ActRight))
	if s.sel != 2 {
		t.Errorf("stepped to %d, want 2 (past the locked entry)", s.sel)
	}
	playDrawn(t, g, screen, press(ActLeft))
	if s.sel != 0 {
		t.Errorf("stepped back to %d, want 0", s.sel)
	}
	playDrawn(t, g, screen, press(ActLeft))
	if s.sel != n-1 {
		t.Errorf("stepped back from the first to %d, want the last (%d)", s.sel, n-1)
	}
	if !s.items()[s.sel].cg {
		t.Errorf("the last entry is not an illustration")
	}
	playDrawn(t, g, screen, press(ActCancel))
	if s.viewing {
		t.Error("cancel did not close the picture")
	}
	if g.scene != s {
		t.Error("cancel on the picture left the gallery")
	}
}

// TestGalleryTabs switches characters along the top strip both ways: each switch starts
// at the first tile of the new character, and going around comes back.
func TestGalleryTabs(t *testing.T) { //nolint:paralleltest // shares the save data and the characters
	g, screen := newDrawScenario(t, func() {
		clearRegulars()
		progress(secretID()).Cleared = true                      // the secret character is open too, already
		store.Data.Announced = map[string]bool{secretID(): true} // brought in on the title
		store.Data.WordTold = true
	})
	s := openGallery(t, g, screen)
	first := s.charIdx
	playDrawn(t, g, screen, press(ActRight, ActTabNext))
	if s.charIdx == first || s.sel != 0 {
		t.Fatalf("on character %d, tile %d after the next tab", s.charIdx, s.sel)
	}
	playDrawn(t, g, screen, press(ActTabPrev))
	if s.charIdx != first {
		t.Fatalf("on character %d after back, want %d", s.charIdx, first)
	}
	for range characters {
		playDrawn(t, g, screen, press(ActTabPrev))
	}
	if s.charIdx != first {
		t.Errorf("going around came back to %d, want %d", s.charIdx, first)
	}
}

// TestGalleryDebugUnlocksEverything opens the gallery with --debug on a new save: every
// entry with a picture can be viewed.
func TestGalleryDebugUnlocksEverything(t *testing.T) { //nolint:paralleltest // changes the debug flag
	g, _ := newDrawScenario(t, nil)
	old := *debugMode
	*debugMode = true
	t.Cleanup(func() { *debugMode = old })
	play(t, g, press(ActDown, ActConfirm)) // not drawn: that would decode every tile
	s, ok := g.scene.(*GalleryScene)
	if !ok {
		t.Fatalf("not in the gallery: %T", g.scene)
	}
	for i, it := range s.items() {
		if !s.open[i] {
			t.Errorf("%s is locked in debug mode", it.e.ID)
		}
	}
}
