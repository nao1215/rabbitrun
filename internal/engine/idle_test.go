package engine

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// idleStretch is the longest stretch of a course on which the lazy line stood still: where
// it ends and how long it is.
type idleStretch struct {
	id    string
	level int // the course
	row   int // the row of the course it ends on
	x     int // the column it stood in
	rows  int
	theme road.Theme
}

func (s idleStretch) String() string {
	return fmt.Sprintf("%s regular course %d-%d (theme %d) row %d column %d: %d rows",
		s.id, (s.level-1)/road.Courses+1, (s.level-1)%road.Courses+1, s.theme, s.row, s.x, s.rows)
}

// lazyIdle follows the lazy line through rows (themes is the theme of the course of each
// row): where she is if she only moves when she has to (road.LazyStep), from the middle
// column where she stands when the run begins. It returns, for each row, the line's column
// and the rows it has stood there. The rows before from (the run's opening) are not
// counted, and a row of a feast or of a lane she chose (road.IdleReset) starts the count
// again.
func lazyIdle(rows []road.Row, themes []road.Theme, from int) (cols, idle []int) {
	cols, idle = make([]int, len(rows)), make([]int, len(rows))
	x, run := road.W/2, 0
	for i, row := range rows {
		last := road.Row{}
		if i > 0 {
			last = rows[i-1]
		}
		switch nx := road.LazyStep(x, row, last); {
		case nx < 0: // a pocket with no way on: she had to leave it before
			x, run = road.NearestOpen(row, x), 1
		case nx != x:
			x, run = nx, 1
		default:
			run++
		}
		if i < from || road.IdleReset(themes[i], row, last, x) {
			run = 0
		}
		cols[i], idle[i] = x, run
	}
	return cols, idle
}

// longestIdle measures a character's regular run and returns the longest stretch standing
// still of each course the idle rule covers, past the run's opening rows.
func longestIdle(e *Engine, id string) []idleStretch {
	themes := e.G.Themes
	byLevel := runRows(e)
	var all []road.Row
	var themeOf []road.Theme
	type at struct{ level, row int }
	var where []at
	for lv := 1; lv <= idleUntil; lv++ {
		for i, r := range byLevel[lv] {
			all = append(all, r)
			themeOf = append(themeOf, themes[lv-1])
			where = append(where, at{lv, i})
		}
	}
	// the rows measured start with the second row of the run (the first is on the screen
	// when it starts): the opening ends OpeningRows-1 rows in
	cols, idle := lazyIdle(all, themeOf, road.OpeningRows-1)
	out := make([]idleStretch, idleUntil)
	for lv := range out {
		out[lv] = idleStretch{id: id, level: lv + 1, theme: themes[lv]}
	}
	for i := road.OpeningRows; i < len(all); i++ {
		s := &out[where[i].level-1]
		if idle[i] > s.rows {
			s.rows, s.row, s.x = idle[i], where[i].row, cols[i]
		}
	}
	return out
}

// idleTable lays out the longest stretches, character by character and course by course.
func idleTable(runs [][]idleStretch) string {
	lines := make([]string, 0, len(runs))
	for _, courses := range runs {
		cells := []string{fmt.Sprintf("%-6s", courses[0].id)}
		for _, c := range courses {
			cells = append(cells, fmt.Sprintf("%2d", c.rows))
		}
		lines = append(lines, strings.Join(cells, " "))
	}
	return strings.Join(lines, "\n")
}

// TestNoLongIdleStretch checks that on the first stage of the regular side she never
// gets to stand still for more than half the screen (road.IdleRows) past the run's opening:
// the lazy line (lazyIdle), where she is if she only moves when she has to, moves within
// IdleRows rows on every course of every character, and the table of the longest stretches
// is logged.
func TestNoLongIdleStretch(t *testing.T) {
	t.Parallel()
	ids := selectOrder(t)
	runs := make([][]idleStretch, 0, len(ids))
	var worst []idleStretch
	for _, id := range ids {
		courses := longestIdle(NewRun(id, false), id)
		runs = append(runs, courses)
		for _, c := range courses {
			if c.rows > road.IdleRows {
				worst = append(worst, c)
			}
		}
	}
	t.Logf("the longest stretch standing still, regular courses 1-1 to 1-%d:\n%s", idleUntil, idleTable(runs))
	if len(worst) > 0 {
		sort.Slice(worst, func(i, j int) bool { return worst[i].rows > worst[j].rows })
		var b strings.Builder
		for _, w := range worst {
			b.WriteString("\n  " + w.String())
		}
		t.Errorf("%d courses let her stand still for more than %d rows (half the screen):%s", len(worst), road.IdleRows, b.String())
	}
}
