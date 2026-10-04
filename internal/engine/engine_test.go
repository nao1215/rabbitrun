package engine

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

func TestRoadScrollsAtTheLevelSpeed(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	e.G.Safe = 1 << 30 // nobody steers; walls pass through
	for range 600 {
		e.Tick(false)
	}
	want := int(e.RowsPerSec() * 10)
	if d := e.G.Distance; d < want-1 || d > want+1 {
		t.Fatalf("scrolled %d rows in 10 seconds, want about %d", d, want)
	}
}

func TestRowsPerSecondRisesAndCaps(t *testing.T) {
	t.Parallel()
	prev := 0.0
	for lv := 1; lv <= 2*GameCourses; lv++ {
		v := RowsPerSecond(lv)
		if v < prev || v > 16 {
			t.Fatalf("level %d: %v rows/s after %v", lv, v, prev)
		}
		prev = v
	}
}

func TestDangerReadsTheRoadAndLives(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	for y := range road.PlayerRow {
		e.G.Rows[y] = road.Row{}
	}
	if d := e.Danger(); d != 0 {
		t.Fatalf("open road: danger %d", d)
	}
	for y := range road.PlayerRow {
		for x := range road.W {
			if x < 3 || x > 5 {
				e.G.Rows[y][x].Wall = 1
			}
		}
	}
	if d := e.Danger(); d != 3 {
		t.Fatalf("narrow road: danger %d", d)
	}
	e.G.Lives = 0
	if d := e.Danger(); d != 4 {
		t.Fatalf("narrow road on the last life: danger %d", d)
	}
}

func TestProgressReadsStageAndCourse(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	e.G.Stage, e.G.Course = 2, 4
	if got := e.Progress(); got != "2-5" {
		t.Fatalf("progress %q", got)
	}
}

func TestSlideSpeedsUpWhileHeld(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	if v := e.SlideSpeed(1); v != slideStart {
		t.Fatalf("first frame: %v cells/s", v)
	}
	prev := 0.0
	for h := 1; h <= slideRampFrames+5; h++ {
		v := e.SlideSpeed(h)
		if v < prev || v > e.PlayerSpeed() {
			t.Fatalf("held %d frames: %v cells/s after %v", h, v, prev)
		}
		prev = v
	}
	if prev != e.PlayerSpeed() {
		t.Fatalf("held long: %v cells/s, want %v", prev, e.PlayerSpeed())
	}
}

func TestRestartProgressIsTenRowsBack(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	e.G.Safe = 1 << 30
	for e.G.Level < 3 { // into course 3: just started, ten rows back is course 2
		e.G.Step()
	}
	if got := e.RestartProgress(); got != "1-2" {
		t.Fatalf("just into course 1-3: %q", got)
	}
	for range road.RewindRows + 5 {
		e.G.Step()
	}
	if got := e.RestartProgress(); got != "1-3" {
		t.Fatalf("well into course 1-3: %q", got)
	}
}

func TestHoldingUpSpeedsTheStageUp(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	e.G.Safe = 1 << 30
	base := e.RowsPerSec()
	for range 60 { // one second held
		e.Tick(true)
	}
	fast := e.RowsPerSec()
	if fast < base*1.25 {
		t.Fatalf("held up for a second: %v rows/s from %v", fast, base)
	}
	for range 120 { // let go: it stays fast
		e.Tick(false)
	}
	if e.RowsPerSec() != fast {
		t.Fatalf("let go: %v rows/s, want it to stay %v", e.RowsPerSec(), fast)
	}
	for range 600 {
		e.Tick(true)
	}
	if e.Boost > boostMax {
		t.Fatalf("boost %v past its cap %v", e.Boost, boostMax)
	}
	if slide := NewRun("gyal", false).PlayerSpeed(); e.PlayerSpeed() != slide {
		t.Fatalf("sideways speed %v with the speed-up, want it unchanged at %v", e.PlayerSpeed(), slide)
	}
	held := e.Boost
	e.G.Events = append(e.G.Events, road.Event{Kind: road.EventStageClear})
	e.collect()
	if e.Boost != held {
		t.Fatalf("a new stage took the speed-up from %v to %v, want it kept", held, e.Boost)
	}
	e.Boost = 1.5
	e.G.Events = append(e.G.Events, road.Event{Kind: road.EventRestart})
	e.collect()
	if e.Boost != 1 {
		t.Fatalf("a retry starts at boost %v, want 1", e.Boost)
	}
}

func TestEveryCharacterRunsHerOwnRoads(t *testing.T) {
	t.Parallel()
	seen := map[uint64]string{}
	for id := range courseThemes {
		s := roadSeedFor(id)
		if other, ok := seen[s]; ok {
			t.Fatalf("%s and %s share a road seed", id, other)
		}
		seen[s] = id
		if s != roadSeedFor(id) {
			t.Fatalf("%s: the seed changes between games", id)
		}
	}
}

func TestCourseThemesAreTheirOwn(t *testing.T) {
	t.Parallel()
	seen := map[[GameCourses]road.Theme]string{}
	for id, sides := range courseThemes {
		for side, run := range sides {
			if run[0] != road.ThemeWarmUp {
				t.Errorf("%s side %d: the first course is %d, not the warm-up", id, side, run[0])
			}
			if other, ok := seen[run]; ok {
				t.Errorf("%s side %d runs the same themes as %s", id, side, other)
			}
			seen[run] = id
		}
	}
	for c := range courseThemes {
		if len(themesFor(c, false)) != GameCourses || len(themesFor(c, true)) != GameCourses {
			t.Errorf("%s has no run of themes", c)
		}
	}
}

func TestExtraRoadIsFaster(t *testing.T) {
	t.Parallel()
	if NewRun("gyal", true).G.Profile.Speed <= NewRun("gyal", false).G.Profile.Speed {
		t.Fatal("the extra road is not faster")
	}
}

func TestGameHasThreeStages(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
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

func TestHasOwnRun(t *testing.T) {
	t.Parallel()
	for id := range courseThemes {
		if !HasOwnRun(id) {
			t.Errorf("%s has her themes but HasOwnRun says not", id)
		}
	}
	if HasOwnRun("nobody") {
		t.Error("an unknown character has a run of her own")
	}
}

// missOnTheWall runs e into a wall across the road just ahead of her.
func missOnTheWall(t *testing.T, e *Engine) {
	t.Helper()
	for x := range road.W {
		e.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0]
	}
	for f := 0; f < 600 && !e.G.Missed && !e.Over(); f++ {
		e.Tick(false)
	}
	if !e.G.Missed {
		t.Fatal("no miss on a wall across the road")
	}
}

func TestRetryAndGiveUpAfterAMiss(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	if e.Level() != 1 {
		t.Fatalf("a run starts on level %d", e.Level())
	}
	e.Boost = 1.2
	missOnTheWall(t, e)
	if !e.Restart() || e.G.Missed || e.Boost != 1 || e.Scroll() != 0 {
		t.Fatalf("retry: missed %v, boost %v, scroll %v", e.G.Missed, e.Boost, e.Scroll())
	}

	e = NewRun("gyal", false)
	missOnTheWall(t, e)
	e.GiveUp()
	if !e.Over() {
		t.Fatal("giving up did not end the game")
	}
	frames, dist := e.PlayFrames, e.G.Distance
	e.Tick(true)
	(&AutoPlayer{}).Step(e)
	if e.PlayFrames != frames || e.G.Distance != dist {
		t.Fatal("the game went on after it was over")
	}
}

func TestHammerNeedsOneInStock(t *testing.T) {
	t.Parallel()
	e := NewRun("gyal", false)
	e.G.Bombs = 0
	if e.UseHammer() {
		t.Fatal("a hammer swung with none in stock")
	}
	e.G.Bombs = 1
	if !e.UseHammer() || e.G.Bombs != 0 {
		t.Fatalf("swinging the one hammer: %d left", e.G.Bombs)
	}
}

func TestDangerRisesAsTheRoadNarrows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ width, want int }{{4, 2}, {5, 1}} {
		e := NewRun("gyal", false)
		for y := range road.PlayerRow {
			for x := range road.W {
				e.G.Rows[y][x].Wall = 0
				if x < 2 || x >= 2+tc.width {
					e.G.Rows[y][x].Wall = 1
				}
			}
		}
		if d := e.Danger(); d != tc.want {
			t.Errorf("a road %d wide: danger %d, want %d", tc.width, d, tc.want)
		}
	}
}

func TestUnknownCharacterRunsTheMixedRoad(t *testing.T) {
	t.Parallel()
	if themesFor("nobody", false) != nil {
		t.Fatal("a character without a table has themes")
	}
}
