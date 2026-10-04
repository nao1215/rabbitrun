package engine

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// courseRoad is the road of the course at level in a character's run, row by row from the
// first (the rows as they come in at the top of the screen; the roads are the same every
// game).
func courseRoad(id string, extra bool, level int) []road.Row {
	e := NewRun(id, extra)
	var out []road.Row
	for e.G.Level <= level && !e.G.AllClear {
		built := e.G.Level // the row that comes in next at the top is built on this course
		e.G.Step()
		if built == level {
			out = append(out, e.G.Ahead)
		}
		e.G.Safe = 1 // only the road is wanted, not where she stands
	}
	return out
}

// longestWithoutRest is the most rows in a row of the road on which she has nowhere to stay
// put: no column has been open for the last restRows rows (a column open that long lets her
// stand still past a pattern instead of stepping aside for every row of it).
func longestWithoutRest(rows []road.Row, restRows int) int {
	var open [road.W]int
	longest, run := 0, 0
	for _, row := range rows {
		rest := false
		for x := range road.W {
			if row[x].Wall != 0 {
				open[x] = 0
				continue
			}
			open[x]++
			rest = rest || open[x] >= restRows
		}
		if rest {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	return longest
}

// TestCoolCheckerboardHasRests measures the cool girl's checkerboard (the thirteenth course
// of her regular run, stage 4): a row of blocks every third row on every other cell, the
// pattern shifting each time, asked for a step aside on every one of them all course long.
// A block left out now and then lets her stand still past three of them: the course had 60
// rows in a row with nowhere to stay put, and has 7 at most with 6 of its 218 blocks left
// out.
func TestCoolCheckerboardHasRests(t *testing.T) {
	t.Parallel()
	const level = 13
	if th := themesFor("cool", false)[level-1]; th != road.ThemeLooseCheckers {
		t.Fatalf("course %d of the cool girl's run is theme %d, not the loose checkerboard", level, th)
	}
	rows := courseRoad("cool", false, level)
	if len(rows) < 40 {
		t.Fatalf("the course has %d rows", len(rows))
	}
	blocks := 0
	for _, row := range rows {
		for x := range row {
			if row[x].Wall != 0 {
				blocks++
			}
		}
	}
	// two checker rows and the rows between: she stands in one column past both of them
	const restRows = 7
	got := longestWithoutRest(rows, restRows)
	t.Logf("%d rows, %d blocks, at most %d rows in a row without a rest", len(rows), blocks, got)
	if got > 12 {
		t.Errorf("%d rows in a row without a column to rest in, want 12 at most", got)
	}
	// still a checkerboard: only a few blocks are left out
	if blocks < 200 {
		t.Errorf("%d blocks: too many left out of the checkerboard", blocks)
	}
}
