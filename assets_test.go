package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// allStates lists every expression state the play scene can ask a character for.
var allStates = []string{
	ExprNormal, ExprRelaxed, ExprHappy, ExprGreat, ExprExcited, ExprTreat, ExprCombo,
	ExprPerfect, ExprWorried, ExprNervous, ExprPanic, ExprCrying, ExprGameOver,
	ExprOops, ExprBlocked, ExprReady, ExprWaiting, ExprRelief, ExprLevelUp, ExprDrought, ExprComeback,
}

func TestProgressDefaults(t *testing.T) {
	t.Parallel()
	s := &SaveData{Characters: map[string]*CharProgress{}}
	p := s.progress("gyal")
	if p == nil || p.UnlockedCG == nil || p.SeenExpr == nil {
		t.Fatalf("progress = %+v, want initialized maps", p)
	}
	if !p.SeenExpr[ExprNormal] || len(p.SeenExpr) != 1 || len(p.UnlockedCG) != 0 {
		t.Fatalf("defaults = %+v, want only the normal portrait seen", p)
	}
	if s.Characters["gyal"] != p || s.progress("gyal") != p {
		t.Fatal("progress must be stored and returned again")
	}

	// Existing progress keeps its data; missing maps are filled in.
	s.Characters["cool"] = &CharProgress{HighScore: 5, SeenExpr: map[string]bool{ExprHappy: true}}
	q := s.progress("cool")
	if q.HighScore != 5 || !q.SeenExpr[ExprHappy] || q.SeenExpr[ExprNormal] || q.UnlockedCG == nil {
		t.Fatalf("existing progress changed: %+v", q)
	}

	// A null entry in the save file behaves like a missing one.
	s.Characters["someone"] = nil
	if r := s.progress("someone"); r == nil || !r.SeenExpr[ExprNormal] {
		t.Fatalf("null entry gave %+v", r)
	}
}

func TestDecodeSave(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    map[string]*CharProgress
	}{
		{
			name: "full",
			raw:  `{"characters":{"gyal":{"high_score":10,"total_score":30,"unlocked_cg":{"cg_peace":true},"seen_expressions":{"happy":true}}}}`,
			want: map[string]*CharProgress{"gyal": {HighScore: 10, TotalScore: 30,
				UnlockedCG: map[string]bool{testCGID: true}, SeenExpr: map[string]bool{"happy": true}}},
		},
		{name: "empty object", raw: `{}`, want: map[string]*CharProgress{}},
		{name: "null characters", raw: `{"characters":null}`, want: map[string]*CharProgress{}},
		{name: "unknown fields are ignored", raw: `{"version":3,"characters":{}}`, want: map[string]*CharProgress{}},
		{name: "broken", raw: `{"characters":`, wantErr: true, want: map[string]*CharProgress{}},
		{name: "wrong type", raw: `{"characters":[]}`, wantErr: true, want: map[string]*CharProgress{}},
		{name: "empty file", raw: ``, wantErr: true, want: map[string]*CharProgress{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var s SaveData
			err := decodeSave(&s, []byte(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if s.Characters == nil {
				t.Fatal("Characters must never be nil after decoding")
			}
			if !reflect.DeepEqual(s.Characters, tc.want) {
				t.Fatalf("characters = %+v, want %+v", s.Characters, tc.want)
			}
		})
	}
}

func TestSaveDataJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := &SaveData{Characters: map[string]*CharProgress{}}
	p := in.progress("gyal")
	p.HighScore, p.TotalScore = 12345, 99999
	p.UnlockedCG["cg03"] = true
	p.SeenExpr[ExprTreat] = true
	in.progress("cool")

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	// Keys are part of the on-disk format and must not change.
	for _, key := range []string{`"characters"`, `"high_score"`, `"total_score"`, `"unlocked_cg"`, `"seen_expressions"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("save JSON %s lacks %s", raw, key)
		}
	}
	var out SaveData
	if err := decodeSave(&out, raw); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("round trip changed the data:\n in  %+v\n out %+v", in.Characters, out.Characters)
	}
}

// useTempConfig points the user config dir at a fresh temp dir and swaps in an
// empty save, restoring the previous one when the test ends.
func useTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux / BSD
	t.Setenv("HOME", dir)            // macOS and the XDG fallback
	t.Setenv("AppData", dir)         // Windows
	old := save
	save = &SaveData{Characters: map[string]*CharProgress{}}
	t.Cleanup(func() { save = old })
}

//nolint:paralleltest // uses t.Setenv and the package-level save data
func TestSaveFiles(t *testing.T) {
	t.Run("write then load", func(t *testing.T) {
		useTempConfig(t)
		p := progress("gyal")
		p.HighScore = 4200
		p.UnlockedCG["cg01"] = true
		writeSave()
		if _, err := os.Stat(savePath()); err != nil {
			t.Fatalf("save file missing: %v", err)
		}
		if _, err := os.Stat(savePath() + ".tmp"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary file left behind: %v", err)
		}
		want := save
		save = &SaveData{}
		loadSave()
		if !reflect.DeepEqual(save, want) {
			t.Fatalf("loaded %+v, want %+v", save.Characters, want.Characters)
		}
	})
	t.Run("missing file keeps defaults", func(t *testing.T) {
		useTempConfig(t)
		loadSave()
		if save.Characters == nil || len(save.Characters) != 0 {
			t.Fatalf("characters = %+v, want empty", save.Characters)
		}
	})
	t.Run("broken file keeps a usable save", func(t *testing.T) {
		useTempConfig(t)
		if err := os.MkdirAll(filepath.Dir(savePath()), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(savePath(), []byte(`{"characters":{"gyal":`), 0o600); err != nil {
			t.Fatal(err)
		}
		save = &SaveData{}
		loadSave()
		if save.Characters == nil {
			t.Fatal("characters map is nil after a broken save file")
		}
		if p := progress("gyal"); p.UnlockedCG == nil {
			t.Fatal("progress unusable after a broken save file")
		}
	})
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

func TestIsASCII(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{"": true, "RABBIT RUN 1,234": true, "~": true, "\x7f": false, "スコア": false, "café": false}
	for in, want := range cases {
		if got := isASCII(in); got != want {
			t.Errorf("isASCII(%q) = %v, want %v", in, got, want)
		}
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

//nolint:paralleltest // uses t.Setenv and the package-level save data
func TestResetSaveStartsOverAndKeepsABackup(t *testing.T) {
	useTempConfig(t)
	progress(heroID).Cleared = true
	save.ExtraFound = true
	writeSave()
	if err := resetSave(); err != nil {
		t.Fatal(err)
	}
	save = &SaveData{Characters: map[string]*CharProgress{}}
	loadSave()
	if save.ExtraFound || progress(heroID).Cleared {
		t.Fatal("the save data is still there after the reset")
	}
	if _, err := os.Stat(savePath() + ".bak"); err != nil {
		t.Fatalf("no backup of the old save: %v", err)
	}
}
