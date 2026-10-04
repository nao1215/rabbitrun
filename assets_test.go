package main

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/save"
)

// allStates lists every expression state the play scene can ask a character for.
var allStates = []string{
	ExprNormal, ExprRelaxed, ExprHappy, ExprGreat, ExprExcited, ExprTreat, ExprCombo,
	ExprPerfect, ExprWorried, ExprNervous, ExprPanic, ExprCrying, ExprGameOver,
	ExprOops, ExprBlocked, ExprReady, ExprWaiting, ExprRelief, ExprLevelUp, ExprDrought, ExprComeback,
}

// useTempConfig points the user config dir at a fresh temp dir and swaps in an
// empty save, restoring the previous one when the test ends.
func useTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux / BSD
	t.Setenv("HOME", dir)            // macOS and the XDG fallback
	t.Setenv("AppData", dir)         // Windows
	old := store
	store = save.NewStore()
	t.Cleanup(func() { store = old })
}

func TestCharacterManifests(t *testing.T) {
	t.Parallel()
	chars, err := readCharacters(assetFS)
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
		"assets/characters/b/game.json": {Data: []byte(`{"id":"b","order":2,"expressions":[{"id":"normal"}],
			"cgs":[{"id":"late","score":900},{"id":"early","score":100}]}`)},
		"assets/characters/a/game.json":         {Data: []byte(`{"id":"a","order":1,"expressions":[{"id":"normal"}]}`)},
		"assets/characters/a/images/normal.png": {Data: []byte("png")},
		"assets/characters/b/images/normal.jpg": {Data: []byte("jpg")},
		"assets/characters/wip/game.json":       {Data: []byte(`{"id":"wip","order":3,"expressions":[{"id":"normal"}]}`)},
		"assets/characters/nojson/x.txt":        {Data: []byte("ignored")},
		"assets/characters/stray.json":          {Data: []byte("not a dir")},
	}
	chars, err := readCharacters(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 2 || chars[0].ID != "a" || chars[1].ID != "b" {
		t.Fatalf("characters = %+v, want a then b (wip has no picture yet)", chars)
	}
	if cgs := chars[1].CGs; cgs[0].ID != "early" || cgs[1].ID != "late" || cgs[0].base != "assets/characters/b" {
		t.Fatalf("CGs = %+v, want sorted by score with base set", cgs)
	}

	fsys["assets/characters/c/game.json"] = &fstest.MapFile{Data: []byte(`{"id":`)}
	if _, err := readCharacters(fsys); err == nil || !strings.Contains(err.Error(), "assets/characters/c") {
		t.Fatalf("broken manifest error = %v, want one naming the dir", err)
	}
	if _, err := readCharacters(fstest.MapFS{}); err == nil {
		t.Fatal("missing characters dir must be an error")
	}
}

func TestCharacterLookupFallbacks(t *testing.T) {
	t.Parallel()
	// The base dir does not exist in the embedded FS, so no entry has an image.
	c := &Character{Expressions: []ImageEntry{
		{ID: ExprNormal, base: "assets/characters/__none__"},
		{ID: "happy2", State: ExprHappy, base: "assets/characters/__none__"},
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

func TestDecodeImageTriesEachExtension(t *testing.T) {
	t.Parallel()
	if img, err := decodeImage("assets/ui/hammer"); err != nil || img == nil {
		t.Fatalf("the hammer (a .png) did not decode: %v", err)
	}
	if img, err := decodeImage("assets/ui/title"); err != nil || img == nil {
		t.Fatalf("the title art (a .jpg) did not decode: %v", err)
	}
	if _, err := decodeImage("assets/ui/no_such_picture"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a missing picture gave %v, want fs.ErrNotExist", err)
	}
}

// TestExpressionFamilies pins the family (background color and frame) of every
// expression, and checks that a reaction's family is that of the situation whose
// portraits stand in for it (exprFallback), so the two tables cannot drift apart.
func TestExpressionFamilies(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		ExprNormal: ExprNormal, ExprRelaxed: ExprNormal, ExprRelief: ExprNormal,
		ExprHappy: ExprHappy, ExprGreat: ExprHappy, ExprReady: ExprHappy, ExprLevelUp: ExprHappy,
		ExprExcited: ExprExcited, ExprTreat: ExprExcited, ExprCombo: ExprExcited, ExprPerfect: ExprExcited,
		ExprWaiting: ExprExcited, ExprComeback: ExprExcited,
		ExprWorried: ExprWorried, ExprNervous: ExprWorried, ExprOops: ExprWorried, ExprBlocked: ExprWorried,
		ExprDrought: ExprWorried,
		ExprPanic:   ExprPanic, ExprCrying: ExprPanic,
		ExprGameOver: ExprGameOver,
	}
	for _, st := range allStates {
		if got := family(st); got != want[st] {
			t.Errorf("family(%s) = %s, want %s", st, got, want[st])
		}
		if _, ok := moodBackground[family(st)]; !ok {
			t.Errorf("%s: no background color for its family %s", st, family(st))
		}
	}
	for reaction, stand := range exprFallback {
		if family(reaction) != family(stand) {
			t.Errorf("%s is in family %s, but its stand-in %s is in %s", reaction, family(reaction), stand, family(stand))
		}
	}
}

// TestFirstPortraitIsTheNormalExpression: the save data marks the portrait every character
// starts with as seen; it must be the normal expression the game shows first.
func TestFirstPortraitIsTheNormalExpression(t *testing.T) {
	t.Parallel()
	if save.FirstPortrait != ExprNormal {
		t.Fatalf("save.FirstPortrait = %q, want %q", save.FirstPortrait, ExprNormal)
	}
}

// TestEveryCharacterHasHerRun checks that every character in the game data has her course
// themes and her speed: a character missing from the tables (a renamed ID) would run the
// plain mixed road at the usual speed without a word.
func TestEveryCharacterHasHerRun(t *testing.T) {
	t.Parallel()
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chars {
		if !engine.HasOwnRun(c.ID) {
			t.Errorf("%s has no course themes or road speed of her own", c.ID)
		}
	}
}

// TestCoursesMatchTheIllustrations: a regular game unlocks one illustration a course for
// every course but the last, which leads to the ending.
func TestCoursesMatchTheIllustrations(t *testing.T) {
	t.Parallel()
	if engine.GameCourses != MainCGCount+1 {
		t.Fatalf("%d courses for %d illustrations, want one more course than illustrations", engine.GameCourses, MainCGCount)
	}
}
