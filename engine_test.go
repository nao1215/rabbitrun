package main

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

func TestRoadScrollsAtTheLevelSpeed(t *testing.T) {
	t.Parallel()
	e := newRun(heroID, false)
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
	e := newRun(heroID, false)
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
	e := newRun(heroID, false)
	e.G.Stage, e.G.Course = 2, 4
	if got := e.Progress(); got != "2-5" {
		t.Fatalf("progress %q", got)
	}
}

func TestSlideSpeedsUpWhileHeld(t *testing.T) {
	t.Parallel()
	e := newRun(heroID, false)
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

func TestMusicSpeedsUpWithTheRoad(t *testing.T) {
	t.Parallel()
	first, last := playBPM(1, roadProfile.Speed), playBPM(GameCourses, roadProfile.Speed)
	if first != 136 || last < 170 || last > 190 {
		t.Fatalf("tempo %v on the first course, %v on the last", first, last)
	}
	prev := 0.0
	for lv := 1; lv <= GameCourses; lv++ {
		if b := playBPM(lv, roadProfile.Speed); b < prev {
			t.Fatalf("tempo falls at level %d: %v after %v", lv, b, prev)
		} else {
			prev = b
		}
	}
}

func TestRestartProgressIsTenRowsBack(t *testing.T) {
	t.Parallel()
	e := newRun(heroID, false)
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
	e := newRun(heroID, false)
	e.G.Safe = 1 << 30
	base := e.RowsPerSec()
	for range 60 { // one second held
		e.Tick(true)
	}
	fast := e.RowsPerSec()
	if fast < base*1.25 {
		t.Fatalf("held up for a second: %v rows/s from %v", fast, base)
	}
	for range 120 { // let go: it stays fast for the rest of the stage
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
	if slide := newRun(heroID, false).PlayerSpeed(); e.PlayerSpeed() != slide {
		t.Fatalf("sideways speed %v with the speed-up, want it unchanged at %v", e.PlayerSpeed(), slide)
	}
	e.G.Events = append(e.G.Events, road.Event{Kind: road.EventStageClear})
	e.collect()
	if e.Boost != 1 {
		t.Fatalf("a new stage starts at boost %v, want 1", e.Boost)
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
		if _, ok := courseThemes[c.ID]; !ok {
			t.Errorf("%s has no course themes", c.ID)
		}
		if _, ok := charSpeed[c.ID]; !ok {
			t.Errorf("%s has no road speed", c.ID)
		}
	}
}
