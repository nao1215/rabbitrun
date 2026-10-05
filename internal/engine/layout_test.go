package engine

import (
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// laidOutCourses lists the courses of every run on one of the themes laid out from where
// they begin (second helpings and the lesson), which need the course to have room for them.
func laidOutCourses(th road.Theme) []checkersCourse {
	var out []checkersCourse
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			for i, t := range themesFor(id, extra) {
				if t == th {
					out = append(out, checkersCourse{id, extra, i + 1})
				}
			}
		}
	}
	return out
}

// TestSecondHelpingsHaveRoomForTheirPrize checks every second-helpings course of every run:
// the course as the run builds it (after a vault, a feast or a course that left the road
// narrow) is long enough for a whole trip down the side lane, so its prize, the extra life
// in the last gap beside the middle wall, is there.
func TestSecondHelpingsHaveRoomForTheirPrize(t *testing.T) {
	t.Parallel()
	courses := laidOutCourses(road.ThemeSeconds)
	if len(courses) < 2 {
		t.Fatalf("second helpings on %d courses", len(courses))
	}
	for _, c := range courses {
		found := false
		for _, row := range courseRoad(c.id, c.extra, c.level) {
			for _, x := range []int{road.W/2 - 1, road.W/2 + 1} {
				found = found || row[x].Sweet == road.SweetOneUp && row[road.W/2].Wall != 0
			}
		}
		if !found {
			t.Errorf("%s (extra %v) course %d: no extra life at the end of a side lane", c.id, c.extra, c.level)
		}
	}
}

// TestLessonsHaveRoomForBothPhrases checks every lesson course of every run: the course as
// the run builds it has room for the moves and for the phrase both ways, so the road
// narrows and bends to the left (the bend shown first and the first phrase) and to the
// right (the mirrored phrase).
func TestLessonsHaveRoomForBothPhrases(t *testing.T) {
	t.Parallel()
	courses := laidOutCourses(road.ThemeLesson)
	if len(courses) < 2 {
		t.Fatalf("the lesson on %d courses", len(courses))
	}
	// bends counts the stretches of two rows or more on which the road is three cells wide
	// against the side wall from (1, the left; 5, the right)
	bends := func(rows []road.Row, from int) int {
		n, run := 0, 0
		for _, row := range rows {
			open := 0
			for x := 1; x < road.W-1; x++ {
				if row[x].Wall == 0 {
					open |= 1 << x
				}
			}
			if open == 7<<from {
				if run++; run == 2 {
					n++
				}
			} else {
				run = 0
			}
		}
		return n
	}
	for _, c := range courses {
		rows := courseRoad(c.id, c.extra, c.level)
		if l, r := bends(rows, 1), bends(rows, 5); l < 2 || r < 1 {
			t.Errorf("%s (extra %v) course %d: %d bends to the left and %d to the right, want 2 and 1 at least", c.id, c.extra, c.level, l, r)
		}
	}
}

// TestEveryHammerHallHasItsHammer checks the hammer hall courses of every run: a hammer lies
// on the road before the first hall of walls. On a course where something put the theme off
// past the row of the hammer (a feast on the bonus course, a block at the end of a long
// straight run in the rows that turn the road to the theme), the hall came with no hammer.
func TestEveryHammerHallHasItsHammer(t *testing.T) {
	t.Parallel()
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			themes := themesFor(id, extra)
			rows := runRows(NewRun(id, extra))
			for i, th := range themes {
				if th != road.ThemeHammerHall {
					continue
				}
				lv := i + 1
				hammer, hall := -1, -1
				for r, row := range rows[lv] {
					walls := 0
					for _, c := range row {
						if c.Sweet == road.SweetBomb && hammer < 0 {
							hammer = r
						}
						if c.Wall != 0 {
							walls++
						}
					}
					if walls >= road.W-2 && hall < 0 { // a wall across the road but for its doorway
						hall = r
					}
				}
				if hammer < 0 || hall >= 0 && hammer > hall {
					t.Errorf("%s/%s, course %d (the hammer hall): the hammer on row %d, the first hall on row %d", id, side(extra), lv, hammer, hall)
				}
			}
		}
	}
}
