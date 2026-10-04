package road

import "testing"

// idleRoad runs a game of 16 courses of the themes th from its start to the all clear (the
// walls passing through her) with the idle rule on the courses up to until, and returns the
// rows as they come in at the top, course by course.
func idleRoad(seed uint64, th []Theme, until int) map[int][]Row {
	g := New(seed)
	g.TotalCourses = 16
	g.Themes = th
	g.CourseRowsFor = func(int) int { return 50 }
	g.IdleUntil = until
	out := map[int][]Row{}
	for i := 0; i < 2000 && !g.AllClear; i++ {
		built := g.Level
		finishing := g.finishing
		g.Step()
		if !finishing && built <= 16 {
			out[built] = append(out[built], g.Ahead)
		}
		g.Safe = 1
		g.Events = g.Events[:0]
	}
	return out
}

// idleThemes are runs of themes that put the idle rule through its cases: the designed and
// the laid-out themes, the rain and the mixed road, vaults (courses 3 and 7) and a feast
// (course 5). (The jar and the rooms are left out: on the idle rule's courses their layout
// is shorter, see Game.jarNeckRows and Game.rooms, so the road is not the same without it.)
var idleThemes = [][]Theme{
	{ThemeWarmUp, ThemeLanes, ThemeSeconds, ThemeFork, ThemeLesson, ThemeRain, ThemeTrail, ThemeComb,
		ThemeAlcoves, ThemeHammerHall, ThemeDoors, ThemeWave, ThemeCheckers, ThemeGates, ThemeStepGates, ThemeMixed},
	{ThemeWarmUp, ThemeSwing, ThemeMixed, ThemeSnake, ThemeRain, ThemeFunnel, ThemeSplit, ThemeHourglass,
		ThemeEdgeRun, ThemeCorridor, ThemeStairs, ThemeChicane, ThemeWobble, ThemePillars, ThemeDiamonds, ThemeMixed},
}

// TestIdleBlocksStandAloneWithAWayAround checks the idle rule's blocks against the same road
// without the rule: the rows differ only on the courses it covers, by one block a row at
// most, put on an open cell (whatever lay there steps aside), and beside it an open cell
// that was open in the row before too.
func TestIdleBlocksStandAloneWithAWayAround(t *testing.T) {
	t.Parallel()
	for i, th := range idleThemes {
		for seed := uint64(1); seed <= 3; seed++ {
			with, without := idleRoad(seed, th, 16), idleRoad(seed, th, 0)
			blocks := 0
			var prev Row // the row before, across the courses
			for lv := 1; lv <= 16; lv++ {
				a, b := with[lv], without[lv]
				if len(a) != len(b) {
					t.Fatalf("themes %d seed %d course %d: %d rows with the rule, %d without", i, seed, lv, len(a), len(b))
				}
				for r := range a {
					last := prev
					prev = a[r]
					var diff []int
					for x := range W {
						if (a[r][x].Wall != 0) != (b[r][x].Wall != 0) {
							diff = append(diff, x)
						}
					}
					if len(diff) == 0 {
						continue
					}
					blocks++
					x := diff[0]
					if len(diff) > 1 || b[r][x].Wall != 0 {
						t.Fatalf("themes %d seed %d course %d row %d: %v differ, want one block on an open cell", i, seed, lv, r, diff)
					}
					way := false
					for _, d := range []int{x - 1, x + 1} {
						way = way || d >= 0 && d < W && a[r][d].Wall == 0 && last[d].Wall == 0
					}
					if !way {
						t.Errorf("themes %d seed %d course %d row %d: no way around the block at %d", i, seed, lv, r, x)
					}
				}
			}
			if blocks == 0 {
				t.Errorf("themes %d seed %d: the idle rule laid no block", i, seed)
			}
		}
	}
}

// TestTheIdleRuleKeepsToItsCourses checks that the rule leaves the courses past the ones it
// covers (Game.IdleUntil) as they are, row for row and sweet for sweet.
func TestTheIdleRuleKeepsToItsCourses(t *testing.T) {
	t.Parallel()
	for i, th := range idleThemes {
		with, without := idleRoad(4, th, 4), idleRoad(4, th, 0)
		for lv := 5; lv <= 16; lv++ {
			if len(with[lv]) != len(without[lv]) {
				t.Fatalf("themes %d course %d: %d rows with the rule, %d without", i, lv, len(with[lv]), len(without[lv]))
			}
			for r := range with[lv] {
				if with[lv][r] != without[lv][r] {
					t.Fatalf("themes %d course %d row %d: %v with the rule, %v without", i, lv, r, with[lv][r], without[lv][r])
				}
			}
		}
	}
}

// TestTheIdleRuleRetriesTheSameRoad checks that a retry on a course the idle rule covers
// shows the same road again, its blocks too.
func TestTheIdleRuleRetriesTheSameRoad(t *testing.T) {
	t.Parallel()
	for i, th := range idleThemes {
		g := New(6)
		g.TotalCourses = 16
		g.Themes = th
		g.CourseRowsFor = func(int) int { return 50 }
		g.IdleUntil = 16
		g.Safe = 1 << 30
		type key struct{ level, row int }
		seen := map[key]Row{}
		walls := func(r Row) Row {
			for x := range r {
				r[x].Sweet = 0
			}
			return r
		}
		for range 260 {
			g.Step()
			seen[key{g.aheadID.level, g.aheadID.row}] = walls(g.Ahead)
		}
		g.Safe = 0
		g.crash()
		if !g.Restart() {
			t.Fatalf("themes %d: no retry", i)
		}
		for y := range Rows {
			id := g.ids[y]
			if want, ok := seen[key(id)]; ok && walls(g.Rows[y]) != want {
				t.Fatalf("themes %d: the retry shows another row %v at course %d row %d", i, g.Rows[y], id.level, id.row)
			}
		}
	}
}

func TestLazyStepAndSweetWorth(t *testing.T) {
	t.Parallel()
	var open, row Row
	row[4].Wall, row[2].Wall = 1, 1
	if got := LazyStep(4, row, open); got != 3 && got != 5 {
		t.Errorf("LazyStep around a block at 4: %d, want a cell beside it", got)
	}
	if got := LazyStep(-1, row, open); got != -1 {
		t.Errorf("LazyStep from off the road: %d, want -1", got)
	}
	var walled Row
	for x := range walled {
		walled[x].Wall = 1
	}
	if got := NearestOpen(walled, 3); got != 3 {
		t.Errorf("NearestOpen of a walled row: %d, want 3", got)
	}
	if got, ok := lazyNext(4, walled, open); got != 4 || !ok {
		t.Errorf("lazyNext into a walled row: %d %v, want 4 true", got, ok)
	}
	for _, s := range []int8{SweetNone, SweetCandy, SweetMacaron, SweetBomb} {
		_ = sweetWorth(s)
	}
	if sweetWorth(SweetBomb) <= sweetWorth(SweetMacaron) || sweetWorth(SweetMacaron) <= sweetWorth(SweetCandy) || sweetWorth(SweetCandy) <= sweetWorth(SweetNone) {
		t.Error("an item is worth more than a macaron, a macaron more than a candy, a candy more than nothing")
	}
}
