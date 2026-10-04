package road

import "testing"

// themedGame is a game of 16 courses, every one of them on the theme t.
func themedGame(seed uint64, t Theme, hard bool) *Game {
	g := New(seed)
	g.TotalCourses = 16
	g.Hard = hard
	g.Themes = make([]Theme, 16)
	for i := range g.Themes {
		g.Themes[i] = t
	}
	return g
}

// TestForkPaysTheHardLane checks the fork: a wall down the middle while the road is wide,
// the hard lane's prize at its end (an extra life at the first fork, a hammer at the next,
// on the other side), and macarons down the easy lane.
func TestForkPaysTheHardLane(t *testing.T) {
	t.Parallel()
	for _, hard := range []bool{false, true} {
		rows := buildCourse(themedGame(1, ThemeFork, hard), 2, 2*forkPeriod)
		for k, want := range []struct {
			prize      int8
			hard, easy int // the middles of the lanes
		}{{SweetOneUp, 2, 6}, {SweetBomb, 6, 2}} {
			from := k * forkPeriod
			macarons := 0
			for p := forkFrom; p < forkTo; p++ {
				row := rows[from+p]
				if row[W/2].Wall == 0 {
					t.Fatalf("hard %v fork %d row %d: no wall down the middle: %v", hard, k, p, row)
				}
				if row[want.easy].Sweet == SweetMacaron {
					macarons++
				}
				for x := want.easy - 1; x <= want.easy+1; x++ {
					if row[x].Wall != 0 {
						t.Fatalf("hard %v fork %d row %d: a block in the easy lane", hard, k, p)
					}
				}
			}
			if got := rows[from+forkTo-1][want.hard].Sweet; got != want.prize {
				t.Errorf("hard %v fork %d: %d at the end of the hard lane, want %d", hard, k, got, want.prize)
			}
			if macarons < (forkTo-forkFrom)/2 {
				t.Errorf("hard %v fork %d: %d macarons down the easy lane", hard, k, macarons)
			}
			// the hard lane can be followed by itself, a cell a row at most
			can := [W]bool{}
			for x := want.hard - 1; x <= want.hard+1; x++ {
				can[x] = true
			}
			for p := forkFrom; p < forkTo; p++ {
				row, last := rows[from+p], rows[from+p-1]
				var next [W]bool
				ok := false
				for x := want.hard - 1; x <= want.hard+1; x++ {
					for d := -1; d <= 1; d++ {
						if px := x + d; can[px] && row[x].Wall == 0 && last[x].Wall == 0 {
							next[x], ok = true, true
						}
					}
				}
				if !ok {
					t.Fatalf("hard %v fork %d: the hard lane is shut at row %d", hard, k, p)
				}
				can = next
			}
		}
	}
}

// TestHammerHallLaysItsHammerBeforeTheFirstHall checks the hammer hall: one hammer, in the
// road before the walls of the first hall begin, and none before the second.
func TestHammerHallLaysItsHammerBeforeTheFirstHall(t *testing.T) {
	t.Parallel()
	rows := buildCourse(themedGame(2, ThemeHammerHall, false), 2, 2*hallPeriod)
	hammers, firstWall := []int{}, -1
	for i, row := range rows {
		for x := range W {
			if row[x].Sweet == SweetBomb {
				hammers = append(hammers, i)
			}
		}
		if i >= hallFrom && firstWall < 0 && row[1].Wall != 0 && row[W-2].Wall != 0 {
			firstWall = i
		}
	}
	// (the stage's own hammer may lie somewhere too: the hall's is the one on its row)
	found := false
	for _, i := range hammers {
		found = found || i == hallHammer
		if i >= hallPeriod && i%hallPeriod == hallHammer {
			t.Errorf("a hammer before the second hall too (row %d)", i)
		}
	}
	if !found || firstWall != hallFrom {
		t.Fatalf("hammers on rows %v, the first wall on row %d: want a hammer on row %d before the hall from row %d", hammers, firstWall, hallHammer, hallFrom)
	}
}

// TestAlcovesHoldMacarons checks that the dents of the alcoves are open, two rows deep,
// with a macaron in the first row of each.
func TestAlcovesHoldMacarons(t *testing.T) {
	t.Parallel()
	rows := buildCourse(themedGame(3, ThemeAlcoves, false), 2, 60)
	dents := 0
	for r := 2 * alcoveEvery; r+1 < len(rows); r += alcoveEvery {
		// the dent is the open cell outside the three cells of the road
		found := false
		for x := range W {
			if rows[r][x].Sweet == SweetMacaron && rows[r+1][x].Wall == 0 &&
				(x == 0 || rows[r][x-1].Wall != 0 || x == W-1 || rows[r][x+1].Wall != 0) {
				found = true
			}
		}
		if !found {
			t.Errorf("row %d: no dent with a macaron: %v", r, rows[r])
		}
		dents++
	}
	if dents == 0 {
		t.Fatal("no dents")
	}
}

// TestDesignedThemesKeepTheirShape builds long stretches of every designed theme and
// checks that no block of a long straight run (trap) comes into their rows, and that they
// lay their sweets themselves (none at random: every row with a sweet is one the theme
// lays it on, apart from the stage's hammer).
func TestDesignedThemesKeepTheirShape(t *testing.T) {
	t.Parallel()
	for th := ThemeFork; th < themeCount; th++ {
		for _, hard := range []bool{false, true} {
			g := themedGame(5, th, hard)
			sweets := 0
			for range 1500 {
				row := g.buildRow(true)
				g.courseRow++
				if g.themeRow && g.sinceTrap == 0 {
					t.Fatalf("theme %d hard %v: a trap block in the theme's row %d", th, hard, g.courseRow)
				}
				for x := range W {
					if row[x].Sweet != SweetNone {
						sweets++
					}
				}
			}
			if sweets < 50 {
				t.Errorf("theme %d hard %v: %d sweets in 1500 rows", th, hard, sweets)
			}
		}
	}
}
