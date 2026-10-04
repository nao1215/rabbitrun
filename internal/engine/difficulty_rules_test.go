package engine

import (
	"fmt"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// The rules the course design keeps, checked on the scores of difficulty_test.go. Each
// failure prints the numbers (and the table of the runs concerned); go test -v -run
// TestCourseScores prints every course of every run.

// byID groups the scores of allScores by character: the regular side and the extra side.
func byID(runs []runScore) map[string][2]runScore {
	out := map[string][2]runScore{}
	for _, r := range runs {
		s := out[r.id]
		if r.extra {
			s[1] = r
		} else {
			s[0] = r
		}
		out[r.id] = s
	}
	return out
}

// TestDifficultyRisesAcrossTheCharacterSelect checks that, on each side, a character's run
// is harder overall (the mean difficulty of its courses) than the run of the character to
// her left on the character select screen: the leftmost is the easiest, the secret one on
// the right the hardest.
func TestDifficultyRisesAcrossTheCharacterSelect(t *testing.T) {
	t.Parallel()
	ids := selectOrder(t)
	runsOf := byID(allScores(t))
	for s := range 2 {
		for i := 1; i < len(ids); i++ {
			a, b := runsOf[ids[i-1]][s], runsOf[ids[i]][s]
			if b.overall() <= a.overall() {
				t.Errorf("%s side: %s scores %.2f, not above %s on her left (%.2f)%s",
					side(s == 1), b.id, b.overall(), a.id, a.overall(), table([]runScore{a, b}))
			}
		}
	}
}

// TestExtraStagesAreHarder checks that the extra side of every character is harder than
// her regular side: overall, stage by stage and course by course (the same course of the
// run on both sides).
func TestExtraStagesAreHarder(t *testing.T) {
	t.Parallel()
	for id, s := range byID(allScores(t)) {
		reg, ext := s[0], s[1]
		if ext.overall() <= reg.overall() {
			t.Errorf("%s: the extra side scores %.2f, not above the regular side (%.2f)", id, ext.overall(), reg.overall())
		}
		for st := 1; st <= 4; st++ {
			if ext.stage(st) <= reg.stage(st) {
				t.Errorf("%s stage %d: the extra side scores %.2f, not above the regular side (%.2f)", id, st, ext.stage(st), reg.stage(st))
			}
		}
		for i := range reg.courses {
			if e, r := ext.courses[i].difficulty(), reg.courses[i].difficulty(); e <= r {
				t.Errorf("%s course %d-%d: the extra side (theme %d) scores %.2f, not above the regular side (theme %d, %.2f)",
					id, i/road.Courses+1, i%road.Courses+1, ext.courses[i].theme, e, reg.courses[i].theme, r)
			}
		}
	}
}

// TestDifficultyRisesStageByStage checks that every run gets harder stage by stage (the
// mean of the courses of a stage; a course may dip below the one before it within a stage,
// as a bonus course does).
func TestDifficultyRisesStageByStage(t *testing.T) {
	t.Parallel()
	for _, r := range allScores(t) {
		for st := 2; st <= 4; st++ {
			if r.stage(st) <= r.stage(st-1) {
				t.Errorf("%s %s: stage %d scores %.2f, not above stage %d (%.2f)%s",
					r.id, side(r.extra), st, r.stage(st), st-1, r.stage(st-1), table([]runScore{r}))
			}
		}
	}
}

// minStageFun is the least fun a stage of any run may score: sweets to pick up, help on
// the way and a road that can be remembered.
const minStageFun = 4

// TestEveryStageIsFun checks the fun score of every stage of every run.
func TestEveryStageIsFun(t *testing.T) {
	t.Parallel()
	for _, r := range allScores(t) {
		for st := 1; st <= 4; st++ {
			if f := r.stageFun(st); f < minStageFun {
				t.Errorf("%s %s: stage %d scores %.2f for fun, want %v at least%s", r.id, side(r.extra), st, f, minStageFun, table([]runScore{r}))
			}
		}
	}
}

// TestThemesDoNotRepeatBackToBack checks that no run has the same theme on two courses in a
// row, and that every run has nine themes at least.
func TestThemesDoNotRepeatBackToBack(t *testing.T) {
	t.Parallel()
	for _, r := range allScores(t) {
		if n := r.longestRepeat(); n > 1 {
			t.Errorf("%s %s: %d courses in a row of one theme", r.id, side(r.extra), n)
		}
		if n := r.variety(); n < 9 {
			t.Errorf("%s %s: %d themes, want 9 at least", r.id, side(r.extra), n)
		}
	}
}

// TestEveryRunHasDesignedThemes checks that the designed themes (ThemeFork and the ones
// after it) are spread over the characters: two of them at least on every side of every
// run, three at least in each character's runs, and every one of them in the runs of two
// characters at least.
func TestEveryRunHasDesignedThemes(t *testing.T) {
	t.Parallel()
	users := map[road.Theme]map[string]bool{}
	for id, sides := range courseThemes {
		mine := map[road.Theme]bool{}
		for s, run := range sides {
			kinds := map[road.Theme]bool{}
			for _, th := range run {
				if th >= road.ThemeFork {
					kinds[th], mine[th] = true, true
					if users[th] == nil {
						users[th] = map[string]bool{}
					}
					users[th][id] = true
				}
			}
			if len(kinds) < 2 {
				t.Errorf("%s %s: %d designed themes, want 2 at least", id, side(s == 1), len(kinds))
			}
		}
		if len(mine) < 3 {
			t.Errorf("%s: %d designed themes in her runs, want 3 at least", id, len(mine))
		}
	}
	for th := road.ThemeFork; th <= road.ThemeLesson; th++ {
		if len(users[th]) < 2 {
			t.Errorf("theme %d is in the runs of %d characters, want 2 at least", th, len(users[th]))
		}
	}
}

// TestExtraIsNotPurePunishment checks the rewards of the extra side against the regular
// side of each character. The rule: the extra side is harder, but not only harder; its
// fun score is 90% of the regular side's at least, and it has as many sweets a second at
// least (the road is faster, so the same sweets a row come quicker).
func TestExtraIsNotPurePunishment(t *testing.T) {
	t.Parallel()
	perSecond := func(r runScore) float64 {
		return r.mean(0, len(r.courses), func(c courseScore) float64 { return c.fun["macarons"] })
	}
	for id, s := range byID(allScores(t)) {
		reg, ext := s[0], s[1]
		if ext.overallFun() < 0.9*reg.overallFun() {
			t.Errorf("%s: the extra side scores %.2f for fun, under 90%% of the regular side (%.2f)", id, ext.overallFun(), reg.overallFun())
		}
		if perSecond(ext) < perSecond(reg) {
			t.Errorf("%s: the extra side has %.2f sweets a second, fewer than the regular side (%.2f)", id, perSecond(ext), perSecond(reg))
		}
	}
}

// TestCheckerboardsStayWhereTheyWere pins the checkerboard courses: they stage the
// difficulty but are no fun, so there are no more of them than there were (the cool
// girl's on stage 4, the gyaru's one a side and the bunny girl's three and two).
func TestCheckerboardsStayWhereTheyWere(t *testing.T) {
	t.Parallel()
	want := map[string][2][]int{
		coolID:  {{13}, nil},
		"gyal":  {{4}, {3}},
		"bunny": {{4, 9, 15}, {2, 12}},
	}
	for id, sides := range courseThemes {
		for s, run := range sides {
			var got []int
			for i, th := range run {
				if th == road.ThemeCheckers {
					got = append(got, i+1)
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(want[id][s]) {
				t.Errorf("%s %s: checkerboards on courses %v, want %v", id, side(s == 1), got, want[id][s])
			}
		}
	}
}

// TestCarefulPlayerClearsAtFullSpeed drives the careful auto player at the road's top
// speed (the player's whole speed-up held from the start) through every run, on both sides.
func TestCarefulPlayerClearsAtFullSpeed(t *testing.T) {
	skipWholeRuns(t)
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			t.Run(id+"/"+side(extra), func(t *testing.T) {
				t.Parallel()
				carefulRun(t, id, extra, boostMax)
			})
		}
	}
}

// themeRun is a run of a character's road with every course but the first and the last
// on the theme t.
func themeRun(id string, extra bool, t road.Theme) *Engine {
	e := NewRun(id, extra)
	themes := make([]road.Theme, GameCourses)
	for i := range themes {
		themes[i] = t
	}
	themes[0], themes[GameCourses-1] = road.ThemeWarmUp, road.ThemeMixed
	e.G.Themes = themes
	return e
}

// TestDesignedThemesCanBeReadAhead checks that the designed themes ask for no reflexes: on
// every course of a run of each of them (regular and tight, on the fastest road), the
// easiest line never turns back within reflexRows rows of its last move, and the course
// lays sweets of its own.
func TestDesignedThemesCanBeReadAhead(t *testing.T) {
	t.Parallel()
	for th := road.ThemeFork; th <= road.ThemeLesson; th++ {
		for _, extra := range []bool{false, true} {
			r := scoreEngine(themeRun("bunny", extra, th), "bunny", extra)
			for _, c := range r.courses[1 : GameCourses-1] {
				if c.diff["reflex"] > 0 {
					t.Errorf("theme %d (extra %v) course %d: %.2f reflexes a second on the easiest line", th, extra, c.level, c.diff["reflex"])
				}
				if c.fun["perCourse"] == 0 {
					t.Errorf("theme %d (extra %v) course %d: no sweets", th, extra, c.level)
				}
			}
		}
	}
}
