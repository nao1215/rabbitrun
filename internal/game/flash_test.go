package game

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"slices"
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
func stageScene(t *testing.T, n int) *playScene {
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
	return &playScene{char: chars[0], eng: engine.NewRun(heroID, false),
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

// TestIllustrationWantedNextStaysLoaded: an illustration fading out behind the road that
// is shown again next (a retry goes back to it) is not freed at the end of its fade, so
// the frame that shows it again does not decode it on the main goroutine; once it is no
// longer wanted next it is freed.
func TestIllustrationWantedNextStaysLoaded(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	s := stageScene(t, 3)
	a, b := &s.char.CGs[0], &s.char.CGs[1]
	t.Cleanup(func() { s.release() })
	s.setStageCG(a)
	if a.Full() == nil {
		t.Fatal("the first illustration did not load")
	}
	s.setStageCG(b) // a fades out under b
	s.next = []*character.ImageEntry{a}
	for s.prevCG != nil {
		s.updateEffects()
	}
	if a.FullReady() == nil {
		t.Fatal("the illustration wanted next was freed as its fade ended")
	}
	s.eng.G.Level = engine.GameCourses - 1 // a later course: its illustration is the last one
	s.artFor = -1                          // the course changes: a is not wanted any more
	s.prefetchArt()
	if slices.Contains(s.next, a) {
		t.Fatal("still wanted")
	}
	if a.FullReady() != nil {
		t.Fatal("the illustration no longer wanted next is still loaded")
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
