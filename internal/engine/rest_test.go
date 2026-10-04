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

// checkersCourse is a checkerboard course of a character's run.
type checkersCourse struct {
	id    string
	extra bool
	level int
}

// checkersCourses lists every checkerboard course of every run.
func checkersCourses() []checkersCourse {
	var out []checkersCourse
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			for i, th := range themesFor(id, extra) {
				if th == road.ThemeCheckers {
					out = append(out, checkersCourse{id, extra, i + 1})
				}
			}
		}
	}
	return out
}

// TestCheckerboardsHaveRestsAndHelp measures every checkerboard course (the cool girl's on
// stage 4, the gyaru's and the bunny girl's): a row of blocks on every other cell every
// third or fourth row, the pattern shifting each time, asked for a step aside on every one
// of them all course long. A block left out of every fourth checker row lets her stand
// still past three of them, and each course lays a hammer as its checker rows begin and an
// extra life halfway through.
func TestCheckerboardsHaveRestsAndHelp(t *testing.T) {
	t.Parallel()
	courses := checkersCourses()
	if len(courses) < 8 {
		t.Fatalf("%d checkerboard courses, want the cool girl's, the gyaru's two and the bunny girl's five", len(courses))
	}
	for _, c := range courses {
		rows := courseRoad(c.id, c.extra, c.level)
		if len(rows) < 30 {
			t.Fatalf("%v: the course has %d rows", c, len(rows))
		}
		blocks, hammers, lives := 0, 0, 0
		for _, row := range rows {
			for x := range row {
				if row[x].Wall != 0 {
					blocks++
				}
				switch row[x].Sweet {
				case road.SweetBomb:
					hammers++
				case road.SweetOneUp:
					lives++
				}
			}
		}
		// two checker rows and the rows between: she stands in one column past both of them
		const restRows = 7
		got := longestWithoutRest(rows, restRows)
		t.Logf("%s extra %v course %d-%d: %d rows, %d blocks, at most %d rows without a rest, %d hammers, %d extra lives",
			c.id, c.extra, (c.level-1)/road.Courses+1, (c.level-1)%road.Courses+1, len(rows), blocks, got, hammers, lives)
		if got > 14 {
			t.Errorf("%v: %d rows in a row without a column to rest in, want 14 at most", c, got)
		}
		if hammers == 0 || lives == 0 {
			t.Errorf("%v: %d hammers and %d extra lives on the course, want one of each at least", c, hammers, lives)
		}
	}
}
