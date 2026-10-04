package engine

import (
	"sort"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// runIDs is every character with a run of course themes, in a fixed order.
func runIDs() []string {
	ids := make([]string, 0, len(courseThemes))
	for id := range courseThemes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// TestAutoPlayerSurvivesTheEarlyRoad drives the auto player on the first courses of every
// character's road: it must be able to follow it, or the road is not passable at that speed.
func TestAutoPlayerSurvivesTheEarlyRoad(t *testing.T) {
	t.Parallel()
	for _, id := range runIDs() {
		e := NewRun(id, false)
		a := &AutoPlayer{}
		crashes := 0
		for range 60 * 60 { // one minute
			a.Step(e)
			e.Tick(false)
			for _, ev := range e.Events {
				if ev.Kind == road.EventCrash {
					crashes++
					t.Logf("%s: crash at row %d level %d player %d", id, e.G.Distance, e.G.Level, e.G.Col())
				}
			}
			e.Events = e.Events[:0]
		}
		if crashes > 0 {
			t.Errorf("%s: %d crashes in the first minute", id, crashes)
		}
	}
}

// TestCarefulPlayerClearsEveryRun drives the careful auto player through every character's
// whole run, on the regular side and on the extra stages, at the road's own speed: the
// roads are the same every game, so each must be cleared without a single miss.
func TestCarefulPlayerClearsEveryRun(t *testing.T) {
	skipWholeRuns(t)
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			t.Run(id+"/"+side(extra), func(t *testing.T) {
				t.Parallel()
				carefulRun(t, id, extra, 1)
			})
		}
	}
}

// skipWholeRuns skips a test that plays or searches whole runs: in -short, and when the
// coverage is measured (the coverage job runs the tests under the race detector, whose
// atomic coverage counters make the frame-by-frame search tens of times slower). The plain
// test job runs them; TestCarefulPlayerReadsTheScreen and the tests of the search cover the
// same code in a few seconds.
func skipWholeRuns(t *testing.T) {
	t.Helper()
	if testing.Short() || testing.CoverMode() != "" {
		t.Skip("plays whole runs (skipped in -short and when measuring the coverage)")
	}
}

// TestCarefulPlayerReadsTheScreen drives the careful auto player over the first stretch of
// the hardest run at the player's whole speed-up: it plans over the screen, plans again as
// the road comes in and slides as it planned, without a miss.
func TestCarefulPlayerReadsTheScreen(t *testing.T) {
	t.Parallel()
	e := NewRun("bunny", true)
	e.Boost = boostMax
	a := &AutoPlayer{Careful: true}
	for range 20 * 60 {
		a.Step(e)
		e.Tick(false)
		e.Events = e.Events[:0]
		if e.G.Missed {
			t.Fatalf("a miss on course %s", e.Progress())
		}
	}
	if e.Steps < 100 {
		t.Fatalf("the road scrolled %d rows in 20 seconds", e.Steps)
	}
	// when something else has moved her (a retry puts her elsewhere), she plans again
	a.planX, a.planSteps = -1, e.Steps
	left := len(a.plan)
	a.Step(e)
	if a.planX != e.G.X || len(a.plan) <= left {
		t.Errorf("no new plan after she was moved (%d frames of plan, %d before)", len(a.plan), left)
	}
}

// carefulRun drives the careful auto player through a character's whole run at the speed
// boost (the player's speed-up, held from the start) and fails the test on a miss.
func carefulRun(t *testing.T, id string, extra bool, boost float64) {
	t.Helper()
	e := NewRun(id, extra)
	e.Boost = boost
	a := &AutoPlayer{Careful: true}
	for f := 0; f < 60*60*10 && !e.G.AllClear && !e.Over() && !e.G.Missed; f++ { // ten minutes at most
		a.Step(e)
		e.Tick(false)
		e.Events = e.Events[:0]
	}
	if !e.G.AllClear {
		t.Errorf("not cleared: stopped on course %s (a miss: %v)", e.Progress(), e.G.Missed || e.Over())
	}
}
