package main

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
		e := newRun(id, false)
		a := &autoPlayer{}
		crashes := 0
		for range 60 * 60 { // one minute
			a.step(e)
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
	if testing.Short() {
		t.Skip("plays every run to its end")
	}
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			e := newRun(id, extra)
			a := &autoPlayer{careful: true}
			for f := 0; f < 60*60*10 && !e.G.AllClear && !e.Over(); f++ { // ten minutes at most
				a.step(e)
				e.Tick(false)
				for _, ev := range e.Events {
					if ev.Kind == road.EventCrash {
						t.Errorf("%s (extra %v): a miss on course %s", id, extra, e.Progress())
					}
				}
				e.Events = e.Events[:0]
				if e.G.Missed {
					break
				}
			}
			if !e.G.AllClear {
				t.Errorf("%s (extra %v): not cleared (stopped on course %s)", id, extra, e.Progress())
			}
		}
	}
}
