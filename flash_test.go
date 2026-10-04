package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/save"
)

// The stage tests below share the package-level save data (courseClear changes it), so
// they do not run in parallel.

// tinyPNG is a one-pixel picture.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stageScene is a play scene on a test character with n illustrations that all have a picture.
func stageScene(t *testing.T, n int) *PlayScene {
	t.Helper()
	pic := tinyPNG(t)
	fsys := fstest.MapFS{"characters/t/images/normal.png": {Data: pic}}
	cgs := make([]string, 0, n)
	for i := range n {
		id := "cg" + string(rune('a'+i))
		cgs = append(cgs, fmt.Sprintf(`{"id":%q,"score":%d}`, id, i+1))
		fsys["characters/t/images/"+id+".png"] = &fstest.MapFile{Data: pic}
	}
	fsys["characters/t/game.json"] = &fstest.MapFile{Data: []byte(`{"id":"t","expressions":[{"id":"normal","state":"normal"}],"cgs":[` +
		strings.Join(cgs, ",") + `]}`)}
	chars, err := character.Read(fsys)
	if err != nil || len(chars) != 1 {
		t.Fatalf("the test character did not load: %v", err)
	}
	return &PlayScene{char: chars[0], eng: engine.NewRun(heroID, false),
		prog: &save.CharProgress{SeenExpr: map[string]bool{}, UnlockedCG: map[string]bool{}}}
}

func TestCourseClearUnlocksItsIllustration(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	s := stageScene(t, engine.GameCourses-1) // one illustration a course
	s.courseClear(1)
	if s.stageCG != &s.char.CGs[0] || !s.prog.UnlockedCG["cga"] {
		t.Fatalf("after course 1: background %v, unlocked %v", s.stageCG, s.prog.UnlockedCG)
	}
	s.courseClear(2)
	if s.stageCG != &s.char.CGs[1] || !s.prog.UnlockedCG["cgb"] || s.prog.UnlockedCG["cgc"] {
		t.Fatalf("after course 2: background %v, unlocked %v", s.stageCG, s.prog.UnlockedCG)
	}
}

func TestAllClearEndsTheRun(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	s := stageScene(t, 2)
	s.allClearNow()
	if !s.allClear || !s.committed {
		t.Fatalf("all clear %v, committed %v", s.allClear, s.committed)
	}
}

func TestIllustrationsAreSharedOutOverTheCourses(t *testing.T) {
	t.Parallel()
	for _, n := range []int{15, 30, 7} {
		prev := 0
		for c := 1; c < engine.GameCourses; c++ {
			u := unlockedAfter(c, n)
			if u < prev || u > n {
				t.Fatalf("%d illustrations: %d after course %d (was %d)", n, u, c, prev)
			}
			prev = u
		}
		if prev != n {
			t.Fatalf("%d illustrations: only %d by the next-to-last course", n, prev)
		}
	}
}
