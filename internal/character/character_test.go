package character

import (
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/gfx"
)

// TestMain reads the game's assets directory from disk, as the game reads the embedded one.
func TestMain(m *testing.M) {
	assets.Use(os.DirFS("../../assets"))
	os.Exit(m.Run())
}

// useFonts loads the game's fonts (the placeholder of a missing picture writes its name).
func useFonts(t *testing.T) {
	t.Helper()
	regular, err := assets.ReadFile("fonts/mplus-1p-regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if err := gfx.LoadFonts(regular, nil); err != nil {
		t.Fatal(err)
	}
}

// allStates lists every expression state the play scene can ask a character for.
var allStates = []string{
	ExprNormal, ExprRelaxed, ExprHappy, ExprGreat, ExprExcited, ExprTreat, ExprCombo,
	ExprPerfect, ExprWorried, ExprNervous, ExprPanic, ExprCrying, ExprGameOver,
	ExprOops, ExprBlocked, ExprReady, ExprWaiting, ExprRelief, ExprLevelUp, ExprDrought, ExprComeback,
}

func TestCharacterManifests(t *testing.T) {
	t.Parallel()
	chars, err := Read(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) == 0 {
		t.Fatal("no characters embedded")
	}
	ids := map[string]bool{}
	for i, c := range chars {
		if i > 0 && chars[i-1].Order > c.Order {
			t.Errorf("characters not sorted by order: %d before %d", chars[i-1].Order, c.Order)
		}
		if c.ID == "" || ids[c.ID] {
			t.Errorf("character id %q is empty or duplicated", c.ID)
		}
		ids[c.ID] = true
		if len(c.Expressions) == 0 || c.Expressions[0].ID != ExprNormal {
			t.Errorf("%s: the first expression must be %q (it is the fallback)", c.ID, ExprNormal)
		}
		if c.Select == nil || c.Select.ID == "" {
			t.Errorf("%s: no character select image", c.ID)
		}
		states := map[string]bool{}
		exprIDs := map[string]bool{}
		for _, e := range c.Expressions {
			st := e.State
			if st == "" {
				st = e.ID
			}
			states[st] = true
			if exprIDs[e.ID] {
				t.Errorf("%s: duplicate expression id %q", c.ID, e.ID)
			}
			exprIDs[e.ID] = true
			if e.base == "" {
				t.Errorf("%s: expression %q has no base dir", c.ID, e.ID)
			}
		}
		for _, st := range allStates {
			if !states[st] {
				t.Errorf("%s: no expression for state %q", c.ID, st)
			}
		}
		for st := range states {
			if !slices.Contains(allStates, st) {
				t.Errorf("%s: expression for unknown state %q", c.ID, st)
			}
		}
		if len(c.CGs) == 0 {
			t.Errorf("%s: no illustrations", c.ID)
		}
		cgIDs := map[string]bool{}
		for j, cg := range c.CGs {
			if j > 0 && c.CGs[j-1].Order >= cg.Order {
				t.Errorf("%s: CG orders not strictly ascending at %d (%d then %d)", c.ID, j, c.CGs[j-1].Order, cg.Order)
			}
			if cg.Order <= 0 {
				t.Errorf("%s: CG %q has order %d", c.ID, cg.ID, cg.Order)
			}
			if cgIDs[cg.ID] || exprIDs[cg.ID] {
				t.Errorf("%s: CG id %q is duplicated", c.ID, cg.ID)
			}
			cgIDs[cg.ID] = true
		}
	}
}

func TestReadCharactersFromFS(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"characters/b/game.json": {Data: []byte(`{"id":"b","order":2,"expressions":[{"id":"normal"}],
			"cgs":[{"id":"late","score":900},{"id":"early","score":100}]}`)},
		"characters/a/game.json":         {Data: []byte(`{"id":"a","order":1,"expressions":[{"id":"normal"}]}`)},
		"characters/a/images/normal.png": {Data: []byte("png")},
		"characters/b/images/normal.jpg": {Data: []byte("jpg")},
		"characters/wip/game.json":       {Data: []byte(`{"id":"wip","order":3,"expressions":[{"id":"normal"}]}`)},
		"characters/nojson/x.txt":        {Data: []byte("ignored")},
		"characters/stray.json":          {Data: []byte("not a dir")},
	}
	chars, err := Read(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 2 || chars[0].ID != "a" || chars[1].ID != "b" {
		t.Fatalf("characters = %+v, want a then b (wip has no picture yet)", chars)
	}
	if cgs := chars[1].CGs; cgs[0].ID != "early" || cgs[1].ID != "late" || cgs[0].base != "characters/b" || cgs[0].fsys == nil {
		t.Fatalf("CGs = %+v, want sorted by score with base and fsys set", cgs)
	}

	fsys["characters/c/game.json"] = &fstest.MapFile{Data: []byte(`{"id":`)}
	if _, err := Read(fsys); err == nil || !strings.Contains(err.Error(), "characters/c") {
		t.Fatalf("broken manifest error = %v, want one naming the dir", err)
	}
	if _, err := Read(fstest.MapFS{}); err == nil {
		t.Fatal("missing characters dir must be an error")
	}
}

func TestCharacterLookupFallbacks(t *testing.T) {
	t.Parallel()
	// The base dir does not exist in the assets, so no entry has an image.
	none := assets.FS()
	c := &Character{Expressions: []ImageEntry{
		{ID: ExprNormal, fsys: none, base: "characters/__none__"},
		{ID: "happy2", State: ExprHappy, fsys: none, base: "characters/__none__"},
	}}
	if got := c.Expression("happy2"); got != &c.Expressions[1] {
		t.Fatalf("Expression(happy2) = %+v", got)
	}
	if got := c.Expression("missing"); got != &c.Expressions[0] {
		t.Fatalf("Expression(missing) = %+v, want the first expression", got)
	}
	vs := c.Variants(ExprHappy)
	if len(vs) != 1 || vs[0] != &c.Expressions[0] {
		t.Fatalf("Variants without images = %+v, want the default portrait", vs)
	}
	if c.Expressions[1].HasImage() {
		t.Fatal("HasImage reported an image that does not exist")
	}
}

func TestVariantsFallBackToRelatedState(t *testing.T) {
	t.Parallel()
	c := &Character{Expressions: []ImageEntry{
		{ID: "normal", State: ExprNormal},
		{ID: ExprWorried, State: ExprWorried},
		{ID: "oops", State: ExprOops},
	}}
	has := func(e *ImageEntry) bool { return e.ID != "oops" } // the oops portrait is not drawn yet
	got := c.variantsWith(ExprOops, has)
	if len(got) != 1 || got[0].ID != ExprWorried {
		t.Errorf("a reaction without portraits should borrow the related state, got %v", got)
	}
	if got := c.variantsWith(ExprDrought, has); len(got) != 1 || got[0].ID != ExprWorried {
		t.Errorf("drought should fall back to worried, got %v", got)
	}
}

// TestMissingPortraitGetsAPlaceholder loads a portrait whose picture is not there: it
// shows the stand-in, which says its label (or its ID without one).
func TestMissingPortraitGetsAPlaceholder(t *testing.T) { //nolint:paralleltest // loads the fonts
	useFonts(t)
	e := &ImageEntry{ID: "nothing", fsys: assets.FS(), base: "characters/nobody"}
	if e.HasImage() {
		t.Fatal("a missing picture is reported as there")
	}
	if e.name() != "nothing" {
		t.Errorf("name %q without a label, want the ID", e.name())
	}
	e.Label = "Nobody"
	if e.name() != "Nobody" {
		t.Errorf("name %q with a label, want the label", e.name())
	}
	img := e.Img()
	if img == nil || img.Bounds().Dx() != 896 || img.Bounds().Dy() != 1120 {
		t.Fatalf("placeholder %v, want the 896x1120 stand-in", img)
	}
	if e.Img() != img {
		t.Error("the placeholder was made again")
	}
	e.ReleaseImg()
	if e.Image != nil {
		t.Error("ReleaseImg kept the placeholder")
	}
	if loadCharImage(assets.FS(), "characters/nobody", "nothing", "x") == nil {
		t.Error("loadCharImage gave no stand-in for a missing picture")
	}
}
