package road

import "testing"

// laidOutCourse builds a course of n rows on the theme th (the course after the first, so it
// runs on from a course of its own theme) and returns its rows and the row where the theme
// began on the wide road.
func laidOutCourse(t *testing.T, th Theme, hard bool, n int) (rows []Row, from int) {
	t.Helper()
	g := themedGame(1, th, hard)
	g.CourseRowsFor = func(int) int { return n }
	rows = buildCourse(g, 2, n)
	if g.designFrom < 0 {
		t.Fatalf("theme %d hard %v: the theme never began on the wide road", th, hard)
	}
	return rows, g.designFrom
}

// mostSweets is the most sweets (and items) a player can pick up on rows from..to-1, moving
// as the road lets her (a cell a row at most, onto a cell open in the row before too, see
// Game.updateReach) and only over the cells allowed, starting on any allowed open cell of
// the first row. It is -1 when no way gets through.
func mostSweets(rows []Row, from, to int, allowed func(x int) bool) int {
	const none = -1
	var best [W]int
	for x := range W {
		best[x] = none
		if allowed(x) && rows[from][x].Wall == 0 {
			best[x] = sweetAt(rows[from][x])
		}
	}
	for r := from + 1; r < to; r++ {
		var next [W]int
		for x := range W {
			next[x] = none
			if !allowed(x) || rows[r][x].Wall != 0 || rows[r-1][x].Wall != 0 {
				continue
			}
			for px := x - 1; px <= x+1; px++ {
				if px >= 0 && px < W && best[px] != none && rows[r-1][px].Wall == 0 {
					next[x] = max(next[x], best[px]+sweetAt(rows[r][x]))
				}
			}
		}
		best = next
	}
	most := none
	for _, b := range best {
		most = max(most, b)
	}
	return most
}

func sweetAt(c Cell) int {
	if c.Sweet != SweetNone {
		return 1
	}
	return 0
}

// sweetsIn counts the sweets (and items) on rows from..to-1 in the cells allowed.
func sweetsIn(rows []Row, from, to int, allowed func(x int) bool) int {
	n := 0
	for r := from; r < to; r++ {
		for x := range W {
			if allowed(x) {
				n += sweetAt(rows[r][x])
			}
		}
	}
	return n
}

// blocksIn counts the blocks on rows from..to-1 in the cells allowed.
func blocksIn(rows []Row, from, to int, allowed func(x int) bool) int {
	n := 0
	for r := from; r < to; r++ {
		for x := range W {
			if allowed(x) && rows[r][x].Wall != 0 {
				n++
			}
		}
	}
	return n
}

// The wide road of a laid-out theme: columns 1 to 7, the middle at 4.
const (
	wideLeft  = 1
	wideRight = 7
	wideMid   = 4
)

// TestSecondsSideLaneGetsHarderTowardItsPrize checks the second helpings course by course:
// the safe lane is never blocked, the side lane's three stretches get harder and hold more,
// the prize lies at its end (an extra life first, a hammer next, on the other side), every
// sweet of the side lane can be picked up on one way through it, each way out leads to the
// safe lane and back, and each way out shows the stretch after it before she gets there.
func TestSecondsSideLaneGetsHarderTowardItsPrize(t *testing.T) {
	t.Parallel()
	for _, hard := range []bool{false, true} {
		rows, from := laidOutCourse(t, ThemeSeconds, hard, 100)
		n := secSeg
		trips := 0
		for k := 0; from+k*secPeriod+secEnd <= len(rows); k++ {
			trips++
			start := from + k*secPeriod + secFrom
			end := start + 3*n + 2*secExit
			side := func(x int) bool { return x >= wideLeft && x < wideMid }
			safe := func(x int) bool { return x > wideMid && x <= wideRight }
			if k%2 == 1 {
				side, safe = safe, side
			}
			if b := blocksIn(rows, start, end, safe); b != 0 {
				t.Errorf("hard %v trip %d: %d blocks in the safe lane", hard, k, b)
			}
			var blocks, macarons [4]int
			for s := 1; s <= 3; s++ {
				a := start + (s-1)*(n+secExit)
				for r := a; r < a+n; r++ {
					if rows[r][wideMid].Wall == 0 {
						t.Fatalf("hard %v trip %d stretch %d row %d: no middle wall", hard, k, s, r-start)
					}
					for x := range W {
						if side(x) && rows[r][x].Sweet == SweetMacaron {
							macarons[s]++
						}
					}
				}
				blocks[s] = blocksIn(rows, a, a+n, side)
				if s == 3 {
					break
				}
				// the way out after the stretch: the middle open, leading both ways, and the
				// next stretch (and its way out, or the prize) in sight from its first row
				e := a + n
				for r := e; r < e+secExit; r++ {
					if rows[r][wideMid].Wall != 0 {
						t.Fatalf("hard %v trip %d: the way out after stretch %d is shut at row %d", hard, k, s, r-start)
					}
				}
				if !crosses(rows, e-1, e+secExit, side, safe) || !crosses(rows, e-1, e+secExit, safe, side) {
					t.Errorf("hard %v trip %d: the way out after stretch %d does not lead across", hard, k, s)
				}
				if next := e + secExit + n; next-e >= PlayerRow {
					t.Errorf("hard %v trip %d: the way out after stretch %d is %d rows before the next one, out of sight", hard, k, s, next-e)
				}
			}
			if blocks[1] != 0 || blocks[2] <= blocks[1] || blocks[3] <= blocks[2] {
				t.Errorf("hard %v trip %d: blocks in the side lane by stretch %v, want more each time", hard, k, blocks[1:])
			}
			if macarons[1] != 0 || macarons[2] != 3 || macarons[3] < 2 {
				t.Errorf("hard %v trip %d: macarons in the side lane by stretch %v", hard, k, macarons[1:])
			}
			prize := SweetOneUp
			if k > 0 {
				prize = SweetBomb
			}
			found := false
			for x := range W {
				found = found || side(x) && rows[end-1][x].Sweet == prize
			}
			if !found {
				t.Errorf("hard %v trip %d: no prize %d at the end of the side lane: %v", hard, k, prize, rows[end-1])
			}
			// the whole side lane in one go, without crossing the middle wall
			if got, want := mostSweets(rows, start, end, side), sweetsIn(rows, start, end, side); got != want {
				t.Errorf("hard %v trip %d: %d of the %d sweets of the side lane on the best way through it", hard, k, got, want)
			}
			if mostSweets(rows, start, end, safe) < 0 {
				t.Errorf("hard %v trip %d: the safe lane cannot be followed", hard, k)
			}
		}
		if trips != 2 {
			t.Errorf("hard %v: %d trips on a course of 100 rows, want 2", hard, trips)
		}
	}
}

// crosses reports whether a player in the cells from on row a can be in the cells to on
// row b, moving as the road lets her.
func crosses(rows []Row, a, b int, from, to func(int) bool) bool {
	var can [W]bool
	for x := range W {
		can[x] = from(x) && rows[a][x].Wall == 0
	}
	for r := a + 1; r <= b; r++ {
		var next [W]bool
		for x := range W {
			if rows[r][x].Wall != 0 || rows[r-1][x].Wall != 0 {
				continue
			}
			for px := x - 1; px <= x+1; px++ {
				next[x] = next[x] || px >= 0 && px < W && can[px] && rows[r-1][px].Wall == 0
			}
		}
		can = next
	}
	for x := range W {
		if can[x] && to(x) {
			return true
		}
	}
	return false
}

// TestSecondsLeavesOutATripCutShort checks that a trip the end of the course would cut short
// is not begun: the road stays open there.
func TestSecondsLeavesOutATripCutShort(t *testing.T) {
	t.Parallel()
	rows, from := laidOutCourse(t, ThemeSeconds, false, 70)
	if from+secPeriod+secEnd <= len(rows) {
		t.Fatalf("a second trip fits a course of %d rows from row %d", len(rows), from)
	}
	inside := func(x int) bool { return x >= wideLeft && x <= wideRight }
	if b := blocksIn(rows, from+secPeriod, len(rows), inside); b != 0 {
		t.Errorf("%d blocks in the road after the only trip", b)
	}
}

// openCells is the open cells of row between the walls of the wide road, as a bit mask.
func openCells(row Row) int {
	m := 0
	for x := wideLeft; x <= wideRight; x++ {
		if row[x].Wall == 0 {
			m |= 1 << x
		}
	}
	return m
}

// cellsFrom is the bit mask of the cells lo..hi.
func cellsFrom(lo, hi int) int {
	m := 0
	for x := lo; x <= hi; x++ {
		m |= 1 << x
	}
	return m
}

// mirrored is the mask m mirrored around the middle of the road.
func mirrored(m int) int {
	out := 0
	for x := range W {
		if m&(1<<x) != 0 {
			out |= 1 << (W - 1 - x)
		}
	}
	return out
}

// TestLessonShowsTheMovesThenThePhrase checks the lesson: the bend, the gates and the rock,
// each alone with open road around it; then the phrase made of them, and the same phrase
// mirrored after a rest; the gate of the phrase three cells wide (two when hard); and every
// sweet of a phrase on one way through it.
func TestLessonShowsTheMovesThenThePhrase(t *testing.T) {
	t.Parallel()
	all := cellsFrom(wideLeft, wideRight)
	for _, hard := range []bool{false, true} {
		rows, from := laidOutCourse(t, ThemeLesson, hard, 80)
		want := map[int]int{ // the open cells of the rows of the learning
			lessonGateA:    cellsFrom(wideLeft, wideLeft+2),
			lessonGateB:    cellsFrom(wideRight-2, wideRight),
			lessonRock:     all &^ cellsFrom(wideMid-1, wideMid+1),
			lessonRock + 1: all &^ cellsFrom(wideMid-1, wideMid+1),
		}
		for i, b := range lessonBend {
			want[i] = cellsFrom(wideMid-b-1, wideMid-b+1)
		}
		for q := range lessonLearn {
			w, ok := want[q]
			if !ok {
				w = all // the open road between the moves
			}
			if got := openCells(rows[from+q]); got != w {
				t.Errorf("hard %v learning row %d: open cells %09b, want %09b", hard, q, got, w)
			}
		}
		if n := lessonPhrases(len(rows), from); n < 2 {
			t.Fatalf("hard %v: %d phrases on a course of %d rows", hard, n, len(rows))
		}
		a, b := from+lessonLearn, from+lessonLearn+lessonPhrase
		for j := range phraseLen {
			if got, w := openCells(rows[b+j]), mirrored(openCells(rows[a+j])); got != w {
				t.Errorf("hard %v phrase row %d: the second phrase %09b is not the first one mirrored (%09b)", hard, j, got, w)
			}
		}
		for r := a + phraseLen - 2; r < b; r++ {
			if openCells(rows[r]) != all {
				t.Errorf("hard %v: blocks on row %d, in the rest before the mirrored phrase", hard, r-from)
			}
		}
		gap := 3
		if hard {
			gap = 2
		}
		if got := openCells(rows[a+phraseGate]); got != cellsFrom(wideMid, wideMid+gap-1) {
			t.Errorf("hard %v: the gate of the phrase opens %09b, want %d cells from the middle", hard, got, gap)
		}
		for _, p := range []int{a, b} {
			everywhere := func(x int) bool { return x >= wideLeft && x <= wideRight }
			if got, w := mostSweets(rows, p, p+phraseLen, everywhere), sweetsIn(rows, p, p+phraseLen, everywhere); got != w || w < 6 {
				t.Errorf("hard %v phrase at row %d: %d of its %d sweets on the best way through it", hard, p-from, got, w)
			}
		}
	}
}

// TestLessonLeavesOutAPhraseCutShort checks that a phrase the end of the course would cut
// short is not begun: after the last whole phrase the road stays open.
func TestLessonLeavesOutAPhraseCutShort(t *testing.T) {
	t.Parallel()
	rows, from := laidOutCourse(t, ThemeLesson, false, 50)
	if n := lessonPhrases(len(rows), from); n != 1 {
		t.Fatalf("%d phrases on a course of %d rows from row %d, want 1", n, len(rows), from)
	}
	inside := func(x int) bool { return x >= wideLeft && x <= wideRight }
	if b := blocksIn(rows, from+lessonLearn+phraseLen, len(rows), inside); b != 0 {
		t.Errorf("%d blocks in the road after the only phrase", b)
	}
}
