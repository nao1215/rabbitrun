package road

import "testing"

// longestOpenColumn builds a course of theme t and returns the most rows one column stays
// open (past the course's opening rows, outside vaults and feasts).
func longestOpenColumn(t Theme, hard bool, seed uint64) int {
	g := New(seed)
	g.Themes = make([]Theme, 16)
	for i := range g.Themes {
		g.Themes[i] = t
	}
	g.Hard = hard
	worst := 0
	run := [W]int{}
	for i, row := range buildCourse(g, 5, CourseRows) {
		if i < 40 || g.section != sectionNone {
			run = [W]int{}
			continue
		}
		for x := range W {
			if row[x].Wall == 0 {
				run[x]++
				worst = max(worst, run[x])
			} else {
				run[x] = 0
			}
		}
	}
	return worst
}

func TestPatternedRoadsLeaveNoSafeLane(t *testing.T) {
	t.Parallel()
	// a pattern that never blocks a column (the sides of the checkers, the middle of a
	// slalom, the sides beside a diamond) is a lane to run down without moving
	for _, th := range []Theme{ThemeCheckers, ThemeLooseCheckers, ThemeSlalom, ThemeDiamonds} {
		for _, hard := range []bool{false, true} {
			for seed := uint64(1); seed <= 6; seed++ {
				if n := longestOpenColumn(th, hard, seed); n > 32 {
					t.Errorf("theme %d (hard %v, seed %d): a column stays open for %d rows", th, hard, seed, n)
				}
			}
		}
	}
}
