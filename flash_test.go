package main

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// testCGID is an illustration ID used by the tests.
const testCGID = "cg_peace"

// The stage tests below share the package-level save data (courseClear changes it), so
// they do not run in parallel.

// stageScene is a play scene on a test character with n illustrations that all have a picture.
func stageScene(n int) *PlayScene {
	c := &Character{ID: "t", Expressions: []ImageEntry{{ID: ExprNormal, State: ExprNormal}}}
	for i := range n {
		c.CGs = append(c.CGs, ImageEntry{ID: "cg" + string(rune('a'+i)), has: 1})
	}
	return &PlayScene{char: c, eng: newRun(heroID, false),
		prog: &CharProgress{SeenExpr: map[string]bool{}, UnlockedCG: map[string]bool{}}}
}

func TestCourseClearUnlocksItsIllustration(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	s := stageScene(GameCourses - 1) // one illustration a course
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
	s := stageScene(2)
	s.allClearNow()
	if !s.allClear || !s.committed {
		t.Fatalf("all clear %v, committed %v", s.allClear, s.committed)
	}
}

func TestIllustrationsAreSharedOutOverTheCourses(t *testing.T) {
	t.Parallel()
	for _, n := range []int{15, 30, 7} {
		prev := 0
		for c := 1; c < GameCourses; c++ {
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

func TestGameHasThreeStages(t *testing.T) {
	t.Parallel()
	e := newRun(heroID, false)
	if e.G.TotalCourses != 16 || e.G.CourseRowsFor == nil || e.G.StageCourses() != road.Courses {
		t.Fatalf("total %d", e.G.TotalCourses)
	}
	// a course lasts about CourseSeconds at any level
	for _, lv := range []int{1, 10, 30} {
		rows := e.G.CourseRowsFor(lv)
		secs := float64(rows) / (RowsPerSecond(lv) * e.G.Profile.Speed)
		if secs < CourseSeconds-1 || secs > CourseSeconds+1 {
			t.Fatalf("level %d: a course lasts %.1f seconds", lv, secs)
		}
	}
}
