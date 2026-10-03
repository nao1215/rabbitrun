package main

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// TestAutoPlayerSurvivesTheEarlyRoad drives the auto player on the first courses of the
// road: it must be able to follow it, or the road is not passable at that speed.
func TestAutoPlayerSurvivesTheEarlyRoad(t *testing.T) {
	t.Parallel()
	for seed := range uint64(10) {
		e := NewEngineFor(seed, GameCourses, false)
		a := &autoPlayer{}
		crashes := 0
		for range 60 * 60 { // one minute
			a.step(e)
			e.Tick(false)
			for _, ev := range e.Events {
				if ev.Kind == road.EventCrash {
					crashes++
					t.Logf("seed %d: crash at row %d level %d player %d", seed, e.G.Distance, e.G.Level, e.G.Col())
				}
			}
			e.Events = e.Events[:0]
		}
		if crashes > 0 {
			t.Errorf("seed %d: %d crashes in the first minute", seed, crashes)
		}
	}
}
