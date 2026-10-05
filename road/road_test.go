package road

import (
	"strings"
	"testing"
)

// open reports the open (not walled) columns of a row.
func open(r Row) []int {
	var out []int
	for x, c := range r {
		if c.Wall == 0 {
			out = append(out, x)
		}
	}
	return out
}

func TestRoadStaysPassable(t *testing.T) {
	t.Parallel()
	profiles := []Profile{Standard,
		{Speed: 1, MaxWidth: 7, Narrowing: 1, Wander: 0.16, Mixed: true, Pillars: 0.1, SweetsRate: 0.2, OneUpRate: 0.05},
		{Speed: 1, MaxWidth: 6, Narrowing: 2, Wander: 0.14, Mixed: true, Pillars: 0.08, SweetsRate: 0.12, OneUpRate: 0.02}}
	for seed := range uint64(30) {
		g := NewWith(seed, profiles[seed%3])
		for range 3000 {
			g.Safe = 1 // keep the test player alive; only the road is checked here
			g.Step()
			top, next := open(g.Rows[0]), open(g.Rows[1])
			if len(top) < minRoadWidth-1 { // a pillar may stand in the narrowest road
				t.Fatalf("seed %d: road %v narrower than %d", seed, top, minRoadWidth)
			}
			// consecutive rows overlap, so the road can always be followed
			overlap := false
			for _, a := range top {
				for _, b := range next {
					overlap = overlap || a == b
				}
			}
			if !overlap {
				t.Fatalf("seed %d: rows %v and %v do not touch", seed, top, next)
			}
			for x, c := range g.Rows[0] {
				if c.Sweet != 0 && c.Wall != 0 {
					t.Fatalf("seed %d: sweet inside the wall at %d", seed, x)
				}
			}
		}
	}
}

func TestMissStopsAndRestartGoesBack(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Stage, g.Course, g.Level = 2, 3, Courses+4
	for x := range W {
		g.Rows[PlayerRow-1][x].Wall = 1 // a full wall about to reach the player
	}
	g.Step()
	g.ReachHalfway()
	if !g.Missed || g.Over || g.Lives != StartLives {
		t.Fatalf("missed %v over %v lives %d", g.Missed, g.Over, g.Lives)
	}
	before := g.Distance
	g.Step()
	if g.Distance != before || g.Move(1) {
		t.Fatal("the road moved while waiting after the miss")
	}
	if !g.Restart() || g.Missed || g.Lives != StartLives-1 {
		t.Fatalf("restart: missed %v lives %d", g.Missed, g.Lives)
	}
	if g.Stage != 2 || g.Course != 2 || g.Level != Courses+3 { // the course had only begun: back into the one before
		t.Fatalf("restarted at stage %d course %d level %d", g.Stage, g.Course, g.Level)
	}
	walls := 0
	for _, r := range g.Rows {
		for _, c := range r {
			if c.Wall != 0 {
				walls++
			}
		}
	}
	if walls < W*Rows/4 {
		t.Fatalf("restarted on a nearly empty screen (%d wall cells)", walls)
	}
	if g.blocked(g.X) || g.blockedIn(g.X, PlayerRow+1) {
		t.Fatal("restarted inside a wall")
	}
}

func TestAWallHitsHalfwayDown(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Safe = 0
	col := g.Col()
	g.Rows[PlayerRow-1][col].Wall = 1 // a single block coming down on her
	g.Step()
	if g.Missed {
		t.Fatal("a wall only touching the tips of her ears was a miss")
	}
	g.ReachHalfway()
	if !g.Missed {
		t.Fatal("a wall halfway down over her was not a miss")
	}

	g = New(1)
	g.Safe = 0
	col = g.Col()
	g.Rows[PlayerRow-1][col].Wall = 1
	g.Step()
	g.Move(1) // she slides out from under it in time
	g.ReachHalfway()
	if g.Missed {
		t.Fatal("she slid out from under the wall in time but it was a miss")
	}
}

func TestMissWithoutLivesEndsTheGame(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Lives = 0
	for x := range W {
		g.Rows[PlayerRow-1][x].Wall = 1
	}
	g.Step()
	g.ReachHalfway()
	if !g.Over || g.Restart() {
		t.Fatal("the game did not end")
	}
}

func TestPickingUpSweetsBuildsAStreak(t *testing.T) {
	t.Parallel()
	g := New(1)
	for i := range 3 {
		g.Rows[PlayerRow-1] = Row{}
		g.Rows[PlayerRow-1][g.Col()].Sweet = SweetCandy
		g.Step()
		if g.Streak != i+1 {
			t.Fatalf("streak %d after %d sweets", g.Streak, i+1)
		}
	}
	// a sweet beside the player scrolls past (by her body, halfway down the next row): the
	// streak ends
	g.Rows[PlayerRow] = Row{}
	g.Rows[PlayerRow][(g.Col()+2)%W].Sweet = SweetCandy
	g.Step()
	g.ReachHalfway()
	if g.Streak != 0 {
		t.Fatalf("streak %d after a miss", g.Streak)
	}
}

func TestTouchingAWallFromTheSideIsAMiss(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Rows[PlayerRow] = Row{}
	g.Rows[PlayerRow][g.Col()+1].Wall = 2
	if !g.Move(-0.2) || g.Missed {
		t.Fatal("could not move into the open")
	}
	g.Move(2)
	if !g.Missed {
		t.Fatal("touching the wall from the side was not a miss")
	}
	if g.Move(-1) {
		t.Fatal("moved after the miss")
	}
}

func TestCoursesChangeColorAndMakeAStage(t *testing.T) {
	t.Parallel()
	g := New(3)
	g.Safe = 1 << 30
	var courses, stages int
	for range CourseRows*Courses + 5 {
		g.Step()
		for _, ev := range g.Events {
			if ev.Kind == EventCourse {
				courses++
			}
			if ev.Kind == EventStageClear {
				stages++
			}
		}
		g.Events = g.Events[:0]
	}
	if courses != Courses || stages != 1 || g.Stage != 2 || g.Course != 0 || g.Level != Courses+1 {
		t.Fatalf("courses %d stages %d stage %d course %d level %d", courses, stages, g.Stage, g.Course, g.Level)
	}
	// the walls built now are the first color again
	for _, c := range g.Ahead {
		if c.Wall != 0 && c.Wall != CourseColors[0] {
			t.Fatalf("wall color %d at the start of stage 2", c.Wall)
		}
	}
}

func TestLastCourseIsNarrower(t *testing.T) {
	t.Parallel()
	widest := func(level int) int {
		g := New(5)
		g.Level = level
		g.Stage, g.Course = (level-1)/Courses+1, (level-1)%Courses
		w := 0
		for range 2000 {
			r := g.buildRow(false)
			open := 0
			for _, c := range r {
				if c.Wall == 0 {
					open++
				}
			}
			w += open
		}
		return w
	}
	if widest(4*Courses) >= widest(1) {
		t.Fatal("the last course is not narrower than the first one")
	}
}

func TestBombClearsTheWallsAndKeepsTheSweets(t *testing.T) {
	t.Parallel()
	g := New(2)
	g.Rows[3][4].Sweet = SweetCandy
	g.Rows[3][4].Wall = 0
	if !g.UseBomb() || g.Bombs != StartBombs-1 {
		t.Fatalf("bomb not used: %d left", g.Bombs)
	}
	for y, r := range g.Rows {
		for x, c := range r {
			if c.Wall != 0 {
				t.Fatalf("wall left at %d,%d", x, y)
			}
		}
	}
	if g.Rows[3][4].Sweet != SweetCandy {
		t.Fatal("the bomb took a sweet")
	}
	if g.UseBomb() {
		t.Fatal("used a bomb with none in stock")
	}
}

// TestPlayerLevelFollowsHerRow runs a game through its first course: the course of her row
// goes up when the first row of the next course comes down to it: PlayerRow+2 steps after
// Level went up, as the last row of the course came in above the screen (Ahead).
func TestPlayerLevelFollowsHerRow(t *testing.T) {
	t.Parallel()
	g := New(3)
	g.TotalCourses = 4
	g.Safe = 1 << 30
	at := -1
	for s := 1; s < 1000 && g.PlayerLevel() == 1; s++ {
		g.Step()
		if at < 0 && g.Level == 2 {
			at = s
		}
		if g.PlayerLevel() == 2 && s-at != PlayerRow+2 {
			t.Fatalf("her row is on course 2 %d steps after Level went up, want %d", s-at, PlayerRow+2)
		}
	}
	if at < 0 || g.PlayerLevel() != 2 {
		t.Fatalf("course 2 began on step %d, her row on course %d", at, g.PlayerLevel())
	}
}

// TestNoHammerAfterAMiss swings a hammer on the stopped road of a miss: it is refused, and
// the stock and the walls stay for the retry.
func TestNoHammerAfterAMiss(t *testing.T) {
	t.Parallel()
	g := New(2)
	g.Rows[PlayerRow][0].Wall = 1
	g.crash()
	if !g.Missed {
		t.Fatal("no miss")
	}
	if g.UseBomb() || g.Bombs != StartBombs || g.Rows[PlayerRow][0].Wall == 0 {
		t.Fatalf("a hammer swung after the miss: %d left, wall %d", g.Bombs, g.Rows[PlayerRow][0].Wall)
	}
}

func TestSweetsAddUpToAnExtraLife(t *testing.T) {
	t.Parallel()
	g := New(2)
	lives := g.Lives
	pick := func(n int) {
		for range n {
			g.Rows[PlayerRow-1] = Row{}
			g.Rows[PlayerRow-1][g.Col()].Sweet = SweetCandy
			g.Step()
		}
	}
	pick(SweetsPerLife) // every life takes the same number
	if g.Lives != lives+1 || g.Sweets != 0 || g.SweetsForLife() != SweetsPerLife {
		t.Fatalf("lives %d (was %d), sweets %d after %d sweets", g.Lives, lives, g.Sweets, SweetsPerLife)
	}
	pick(SweetsPerLife)
	if g.Lives != lives+2 {
		t.Fatalf("lives %d after %d more sweets", g.Lives, SweetsPerLife)
	}
}

func TestAtMostOneBombAStage(t *testing.T) {
	t.Parallel()
	for seed := range uint64(40) {
		g := New(seed)
		g.Safe = 1 << 30
		bombs := 0
		for range CourseRows*Courses - 20 {
			g.Step()
			for _, c := range g.Ahead {
				if c.Sweet == SweetBomb {
					bombs++
				}
			}
		}
		if bombs > 1 {
			t.Fatalf("seed %d: %d bombs in one stage", seed, bombs)
		}
	}
}

func TestBombChanceFallsWithTheStage(t *testing.T) {
	t.Parallel()
	prev := 1.0
	for stage := 1; stage <= 12; stage++ {
		c := BombChance(stage)
		if c > prev || c < 0.1 {
			t.Fatalf("stage %d: chance %v after %v", stage, c, prev)
		}
		prev = c
	}
}

func TestShortLastStageAndAllClear(t *testing.T) {
	t.Parallel()
	g := New(4)
	g.TotalCourses = Courses + 3 // a full stage, then a short stage of three
	g.CourseRowsFor = func(int) int { return 5 }
	g.Safe = 1 << 30
	var stage2Colors []int8
	allClear := 0
	for range 5 * 12 {
		g.Step()
		if g.Stage == 2 && (len(stage2Colors) == 0 || stage2Colors[len(stage2Colors)-1] != g.WallColor()) {
			stage2Colors = append(stage2Colors, g.WallColor())
		}
		for _, ev := range g.Events {
			if ev.Kind == EventAllClear {
				allClear++
				for y := 0; y <= PlayerRow+1; y++ {
					for x := range W {
						if g.Rows[y][x].Wall != 0 {
							t.Fatalf("all clear with a wall at row %d (she has not run out onto the open road)", y)
						}
					}
				}
			}
		}
		g.Events = g.Events[:0]
	}
	if allClear != 1 || !g.AllClear {
		t.Fatalf("all clear events %d, AllClear %v", allClear, g.AllClear)
	}
	want := CourseColors[Courses-3:]
	if len(stage2Colors) != 3 || stage2Colors[0] != want[0] || stage2Colors[2] != want[2] {
		t.Fatalf("last stage colors %v, want %v", stage2Colors, want)
	}
}

// TestRoadCanBeFollowedAllTheWay builds long roads (with every kind of section) and
// checks with a search over the whole road that a player who slides at most one
// cell while one row passes (only over open cells of the row she is on) can always
// get through.
func TestRoadCanBeFollowedAllTheWay(t *testing.T) {
	t.Parallel()
	regular := Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true, Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16, OneUpRate: 0.03}
	extra := Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.2, Mixed: true, Pillars: 0.1, Gates: 0.08, SweetsRate: 0.16, OneUpRate: 0.03}
	for seed := range uint64(24) {
		p := regular
		if seed%2 == 1 {
			p = extra
		}
		g := NewWith(seed, p)
		g.Safe = 1 << 30
		g.Level, g.Course, g.Stage = 15, 2, 4 // a late, hard course
		rows := make([]Row, 0, 3000)
		for range 3000 {
			g.Step()
			rows = append(rows, g.Rows[0])
			g.Events = g.Events[:0]
		}
		// rows[i] comes after rows[i+1]... the newest row is last; walk from the oldest
		reach := 1
		var can [W]bool
		for x, c := range rows[0] {
			can[x] = c.Wall == 0
		}
		for i := 1; i < len(rows); i++ {
			cur, next := rows[i-1], rows[i]
			var nxt [W]bool
			any := false
			for x := range W {
				if !can[x] {
					continue
				}
				// slide along cur (open cells only) up to reach cells, to a cell open in next
				for dir := -1; dir <= 1; dir += 2 {
					for d := 0; d <= reach; d++ {
						nx := x + dir*d
						if nx < 0 || nx >= W || cur[nx].Wall != 0 {
							break
						}
						if next[nx].Wall == 0 {
							nxt[nx] = true
							any = true
						}
					}
				}
			}
			if !any {
				for j := max(0, i-8); j <= i; j++ {
					var line strings.Builder
					for _, c := range rows[j] {
						if c.Wall != 0 {
							line.WriteByte('#')
						} else {
							line.WriteByte('.')
						}
					}
					t.Logf("row %d %s", j, line.String())
				}
				t.Fatalf("seed %d: no way from row %d to row %d", seed, i-1, i)
			}
			can = nxt
		}
	}
}

func TestTheGameOpensWithATrailOfSweets(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Safe = 1 << 30
	n := 0
	for range trailRows + 10 {
		g.Step()
		for _, c := range g.Rows[0] {
			if c.Sweet == SweetCandy {
				n++
			}
		}
	}
	if n < trailRows {
		t.Fatalf("%d sweets on the trail, want at least %d", n, trailRows)
	}
}

func TestTheFirstCourseRestartsOnItself(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.courseRow = 5
	g.Missed = true
	if !g.Restart() || g.Level != 1 || g.Stage != 1 || g.Course != 0 {
		t.Fatalf("restarted at stage %d course %d level %d", g.Stage, g.Course, g.Level)
	}
}

// buildCourse builds the road of course level (from its start) and returns its rows.
func buildCourse(g *Game, level, n int) []Row {
	g.Level = level
	g.Stage, g.Course = (level-1)/Courses+1, (level-1)%Courses
	g.startCourse()
	g.courseRow = 0
	rows := make([]Row, 0, n)
	for range n {
		rows = append(rows, g.buildRow(true))
		g.courseRow++ // as nextCourse counts them
	}
	return rows
}

func TestEveryCourseIsTheSameRoadEveryTime(t *testing.T) {
	t.Parallel()
	a := buildCourse(New(7), 5, 80)
	b := buildCourse(New(7), 5, 80)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("row %d differs: %v / %v", i, a[i], b[i])
		}
	}
}

// TestRetryRunsTheSameRoadAgain plays on, misses, and checks that after the retry the
// screen shows the walls it showed when the run was there the first time (the courses run
// on from each other, so the retry must start the course before where it began).
func TestRetryRunsTheSameRoadAgain(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 12; seed++ {
		g := New(seed)
		g.TotalCourses = 16
		g.Themes = make([]Theme, 16)
		for i := range g.Themes {
			g.Themes[i] = Theme(1 + (int(seed)+i)%int(themeCount-1))
		}
		g.CourseRowsFor = func(int) int { return 40 }
		g.Safe = 1 << 30
		type key struct{ level, row int }
		seen := map[key][Rows + 1]Row{}
		walls := func() [Rows + 1]Row {
			var out [Rows + 1]Row
			for y := range Rows {
				for x := range W {
					out[y][x].Wall = g.Rows[y][x].Wall
				}
			}
			for x := range W {
				out[Rows][x].Wall = g.Ahead[x].Wall
			}
			return out
		}
		for range 150 + int(seed)*17 {
			g.Step()
			seen[key{g.Level, g.courseRow}] = walls()
		}
		g.Safe = 0
		g.crash()
		if !g.Restart() {
			t.Fatalf("seed %d: no retry", seed)
		}
		want, ok := seen[key{g.Level, g.courseRow}]
		if !ok {
			t.Fatalf("seed %d: the retry starts where the run never was (level %d row %d)", seed, g.Level, g.courseRow)
		}
		if got := walls(); got != want {
			t.Fatalf("seed %d: the retry at level %d row %d shows another road", seed, g.Level, g.courseRow)
		}
	}
}

func TestVaultsHoldTwoPrizesOnlyABombOpens(t *testing.T) {
	t.Parallel()
	for level, prizes := range Vaults {
		g := New(9)
		g.TotalCourses = 16
		rows := buildCourse(g, level, CourseRows)
		found := false
		for i := 1; i+1 < len(rows); i++ {
			for x := 1; x+2 < W; x++ {
				r := rows[i]
				if r[x].Sweet != prizes[0] || r[x+1].Sweet != prizes[1] {
					continue
				}
				// walled in: a block on each side, and blocks above and below both
				if r[x-1].Wall == 0 || r[x+2].Wall == 0 || rows[i-1][x].Wall == 0 || rows[i-1][x+1].Wall == 0 ||
					rows[i+1][x].Wall == 0 || rows[i+1][x+1].Wall == 0 {
					continue
				}
				found = true
				open := 0
				for _, c := range r {
					if c.Wall == 0 && c.Sweet == SweetNone {
						open++
					}
				}
				if open < 2 {
					t.Fatalf("level %d: no way past the vault (%d open)", level, open)
				}
			}
		}
		if !found {
			t.Fatalf("level %d: no walled-in %v found", level, prizes)
		}
		// worth more than the hammer it takes
		if prizes[0] != SweetOneUp && prizes[0] != SweetBomb || prizes[1] != SweetOneUp && prizes[1] != SweetBomb {
			t.Fatalf("level %d: prizes %v", level, prizes)
		}
	}
}

func TestRoadsWithVaultsCanBeFollowed(t *testing.T) {
	t.Parallel()
	p := Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true, Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16, OneUpRate: 0.03}
	levels := make([]int, 0, len(Vaults)+len(Feasts))
	for level := range Vaults {
		levels = append(levels, level)
	}
	for level := range Feasts {
		levels = append(levels, level)
	}
	for _, level := range levels {
		g := NewWith(4, p)
		g.TotalCourses = 21
		rows := buildCourse(g, level, CourseRows)
		var can [W]bool
		for x, c := range rows[0] {
			can[x] = c.Wall == 0
		}
		for i := 1; i < len(rows); i++ {
			var nxt [W]bool
			ok := false
			for x := range W {
				for d := -1; d <= 1 && can[x]; d++ {
					if nx := x + d; nx >= 0 && nx < W && rows[i-1][nx].Wall == 0 && rows[i][nx].Wall == 0 {
						nxt[nx], ok = true, true
					}
				}
			}
			if !ok {
				t.Fatalf("level %d: stuck at row %d", level, i)
			}
			can = nxt
		}
	}
}

func TestRestartGoesBackTenRowsOnTheSameRoad(t *testing.T) {
	t.Parallel()
	for _, crashAt := range []int{30, 3} { // in the middle of a course, and just after one started
		g := New(5)
		g.Safe = 1 << 30
		var before [Rows]Row
		target := -1
		for g.Level < 6 || g.courseRow < crashAt {
			if g.Level == 6 && g.courseRow == crashAt-RewindRows || (crashAt < RewindRows && g.Level == 5 && g.courseRow == g.lenOf(5)+crashAt-RewindRows) {
				before, target = g.Rows, g.Distance
			}
			g.Step()
		}
		if target < 0 {
			t.Fatalf("crash at %d: never saw the place to go back to", crashAt)
		}
		g.Safe = 0
		g.Missed = true
		wantLevel := 6
		if crashAt < RewindRows {
			wantLevel = 5
		}
		if !g.Restart() || g.Level != wantLevel {
			t.Fatalf("crash at %d: restarted at level %d, want %d", crashAt, g.Level, wantLevel)
		}
		// the road ahead of her is what it was ten rows back (behind her, sweets were picked up)
		for y := range PlayerRow {
			for x := range W {
				if g.Rows[y][x].Wall != before[y][x].Wall {
					t.Fatalf("crash at %d: row %d differs from the road ten rows back", crashAt, y)
				}
			}
		}
		if g.blocked(g.X) || g.Rows[PlayerRow-1][g.Col()].Wall != 0 {
			t.Fatalf("crash at %d: restarted on or right under a wall", crashAt)
		}
	}
}

func TestFeastsFillTheRoadWithSweets(t *testing.T) {
	t.Parallel()
	for level := range Feasts {
		g := New(2)
		g.TotalCourses = 16
		full := 0
		for _, r := range buildCourse(g, level, CourseRows) {
			n := 0
			for _, c := range r {
				if c.Sweet == SweetCandy {
					n++
				}
			}
			if n >= 6 {
				full++
			}
		}
		if full != feastRows {
			t.Fatalf("level %d: %d rows full of sweets, want %d", level, full, feastRows)
		}
	}
}

// TestVaultsAndFeastsFitInShortCourses builds the special courses at the length they
// have in the game (about 40 rows): the cage and the sweets are all there.
func TestVaultsAndFeastsFitInShortCourses(t *testing.T) {
	t.Parallel()
	for level, prizes := range Vaults {
		g := New(9)
		g.TotalCourses = 16
		n := 0
		for _, r := range buildCourse(g, level, 40) {
			for x := 0; x+1 < W; x++ {
				if r[x].Sweet == prizes[0] && r[x+1].Sweet == prizes[1] {
					n++
				}
			}
		}
		if n != 1 {
			t.Fatalf("level %d: %d vault rows in a 40-row course", level, n)
		}
	}
	for level := range Feasts {
		g := New(9)
		g.TotalCourses = 16
		full := 0
		for _, r := range buildCourse(g, level, 40) {
			k := 0
			for _, c := range r {
				if c.Sweet == SweetCandy {
					k++
				}
			}
			if k >= 6 {
				full++
			}
		}
		if full != feastRows {
			t.Fatalf("level %d: %d rows of sweets in a 40-row course", level, full)
		}
	}
}

// TestEveryThemeCanBeFollowed builds courses of every theme (both the regular and the hard
// ones) and checks with a search over the whole road that a player who slides at most one
// cell while a row passes can always get through.
func TestEveryThemeCanBeFollowed(t *testing.T) {
	t.Parallel()
	p := Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true, Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16, OneUpRate: 0.03}
	for th := range themeCount {
		for _, hard := range []bool{false, true} {
			for seed := range uint64(4) {
				g := NewWith(seed, p)
				g.TotalCourses = 16
				g.Hard = hard
				g.Themes = make([]Theme, 16)
				for i := range g.Themes {
					g.Themes[i] = th
				}
				var rows []Row
				for level := 1; level <= 16; level++ {
					rows = append(rows, buildCourse(g, level, 45)...)
				}
				var can [W]bool
				for x, c := range rows[0] {
					can[x] = c.Wall == 0
				}
				for i := 1; i < len(rows); i++ {
					var nxt [W]bool
					ok := false
					for x := range W {
						for d := -1; d <= 1 && can[x]; d++ {
							if nx := x + d; nx >= 0 && nx < W && rows[i-1][nx].Wall == 0 && rows[i][nx].Wall == 0 {
								nxt[nx], ok = true, true
							}
						}
					}
					if !ok {
						t.Fatalf("theme %d hard %v seed %d: stuck at row %d", th, hard, seed, i)
					}
					can = nxt
				}
			}
		}
	}
}

func TestRetriesNeverStartInAWall(t *testing.T) {
	t.Parallel()
	for seed := uint64(1); seed <= 40; seed++ {
		g := New(seed)
		g.Hard = seed%2 == 0
		for i := range 60 + int(seed)*37 {
			_ = i
			g.Safe = 1 // run through the walls to somewhere along the road
			g.Step()
		}
		g.Safe = 0
		g.crash()
		if !g.Restart() {
			t.Fatalf("seed %d: no retry", seed)
		}
		if g.blocked(g.X) || g.blockedIn(g.X, PlayerRow+1) {
			t.Fatalf("seed %d: retried inside a wall at level %d", seed, g.Level)
		}
	}
}

func TestBonusCoursesAreColorfulAndOnlyThem(t *testing.T) {
	t.Parallel()
	for level := 1; level <= 4*Courses; level++ {
		g := New(1)
		g.TotalCourses = 4 * Courses
		g.Level = level
		g.Stage, g.Course = (level-1)/Courses+1, (level-1)%Courses
		g.startCourse()
		colors := map[int8]bool{}
		for range 200 {
			for _, c := range g.buildRow(true) {
				if c.Wall != 0 {
					colors[c.Wall] = true
				}
			}
		}
		if g.Bonus() != Feasts[level] || (len(colors) > 1) != Feasts[level] {
			t.Fatalf("level %d: bonus %v, %d wall colors", level, g.Bonus(), len(colors))
		}
	}
}

func TestRetryDoesNotBringBackTheSweetsOfTheRowsRun(t *testing.T) {
	t.Parallel()
	kept := 0
	for seed := uint64(1); seed <= 10; seed++ {
		g := New(seed)
		g.Safe = 1 << 30
		for range 300 {
			g.Step()
		}
		g.Safe = 0
		g.crash()
		if !g.Restart() {
			t.Fatal("no retry")
		}
		rows := append(g.Rows[:], g.Ahead)
		ids := append(g.ids[:], g.aheadID)
		for y, r := range rows {
			for x, c := range r {
				if c.Sweet == 0 {
					continue
				}
				if g.taken[takenKey{ids[y], x}] {
					t.Fatalf("seed %d: a sweet taken before came back at row %d, column %d", seed, y, x)
				}
				if y > PlayerRow-RewindRows && y <= PlayerRow {
					kept++ // one she ran past without taking: still there to take
				}
			}
		}
	}
	if kept == 0 {
		t.Fatal("the retry took away the sweets she had not taken (a hammer ahead of her vanished)")
	}
}

// TestLongStraightRunsEndInABlock runs a wide road with no blocks of its own (the warm-up
// theme): a column she could run straight up for long gets one block in it, with room to
// step aside on both sides.
func TestLongStraightRunsEndInABlock(t *testing.T) {
	t.Parallel()
	g := New(5)
	g.TotalCourses = 16
	g.Themes = make([]Theme, 16)
	for i := range g.Themes {
		g.Themes[i] = ThemeWarmUp
	}
	traps := 0
	var prev Row
	for i := range 2000 {
		row := g.buildRow(true)
		g.courseRow++
		for x := 1; x < W-1; x++ {
			inner := row[x].Wall != 0 && row[x-1].Wall == 0 && row[x+1].Wall == 0
			if !inner {
				continue
			}
			traps++
			if i > 0 && (prev[x-1].Wall != 0 || prev[x+1].Wall != 0) {
				t.Fatalf("row %d: the block at %d has no room beside it in the row before", i, x)
			}
		}
		prev = row
	}
	if traps == 0 {
		t.Fatal("a long straight run never ended in a block")
	}
}

func TestSweetAheadFindsAKindAmongOthers(t *testing.T) {
	t.Parallel()
	g := New(1)
	g.Rows[PlayerRow-3][2].Sweet = SweetOneUp
	g.Rows[PlayerRow-2][5].Sweet = SweetBomb // a hammer nearer must not hide the extra life
	if !g.SweetAhead(10, SweetOneUp) {
		t.Fatal("the extra life ahead was not found because a hammer was also ahead")
	}
	if g.SweetAhead(1, SweetOneUp) {
		t.Fatal("an extra life three rows ahead was found within one row")
	}
}

// TestStartAtIsTheRoadOfPlay checks that a game started at a later course is on the road a
// game played up to there is on, with its stage and course in step with its level.
func TestStartAtIsTheRoadOfPlay(t *testing.T) {
	t.Parallel()
	newGame := func() *Game {
		g := NewWith(7, Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true,
			Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16})
		g.TotalCourses = 14
		return g
	}
	for _, level := range []int{2, Courses + 1, Courses + 2, 14} {
		played, started := newGame(), newGame()
		played.Safe = 1 << 30
		for played.Level < level {
			played.Step()
		}
		started.StartAt(level)
		if started.Level != level || started.Stage != played.Stage || started.Course != played.Course {
			t.Fatalf("level %d: started on level %d, stage %d, course %d; played to stage %d, course %d",
				level, started.Level, started.Stage, started.Course, played.Stage, played.Course)
		}
		if started.Stage != (level-1)/Courses+1 || started.Course != (level-1)%Courses {
			t.Fatalf("level %d: stage %d, course %d", level, started.Stage, started.Course)
		}
		if len(started.Events) != 0 {
			t.Fatalf("level %d: events %v on the way", level, started.Events)
		}
		started.Safe = 1 << 30
		for range 200 {
			played.Step()
			started.Step()
			for x := range W {
				if played.Rows[0][x].Wall != started.Rows[0][x].Wall {
					t.Fatalf("level %d: the road differs from the one played", level)
				}
			}
		}
	}
	g := newGame()
	g.StartAt(99)
	if g.Level != 14 || g.Stage != 4 || g.Course != 1 {
		t.Fatalf("past the end: level %d, stage %d, course %d", g.Level, g.Stage, g.Course)
	}
}

// TestStepsToNextCourseCountsDownToTheNextLevel checks that StepsToNextCourse tells when
// the next course begins: Level goes up on exactly that step, all through a short game,
// and it reports -1 on the open road after the last course.
func TestStepsToNextCourseCountsDownToTheNextLevel(t *testing.T) {
	t.Parallel()
	g := New(7)
	g.TotalCourses = 3
	g.CourseRowsFor = func(level int) int { return 20 + level }
	g.Safe = 1 << 30
	for steps := 0; !g.AllClear; steps++ {
		if steps > 1000 {
			t.Fatal("no all clear after 1000 steps")
		}
		n, lv := g.StepsToNextCourse(), g.Level
		if n < 0 {
			if lv != g.TotalCourses+1 {
				t.Fatalf("-1 on course %d, before the open road after the last", lv)
			}
			g.Step()
			continue
		}
		for i := 1; i < n; i++ {
			if g.Step(); g.Level != lv {
				t.Fatalf("course %d after %d of %d steps from course %d", g.Level, i, n, lv)
			}
		}
		if g.Step(); g.Level != lv+1 {
			t.Fatalf("course %d after the %d steps from course %d, want %d", g.Level, n, lv, lv+1)
		}
	}
}

// TestADentIsTwoRowsDeepOnAThemedCourse builds courses of every theme and checks the dents
// that a sweet is put in (place): while the road still runs beside it, the dent is open on
// the row after it too, so she has two rows to dart in for the sweet and out again. On a
// row of a course's theme the second row was never carved, so the sweet (often an extra
// life or a hammer, which go to the dents twice as often) lay in a notch one row deep that
// the next row walled off.
func TestADentIsTwoRowsDeepOnAThemedCourse(t *testing.T) {
	t.Parallel()
	p := Profile{Speed: 1, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true, Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16, OneUpRate: 0.03}
	dents := 0
	for th := range themeCount {
		if th == ThemeAlcoves || th == ThemeMixed {
			continue // its dents are its own (designSweets); a mixed course is not themed
		}
		for _, hard := range []bool{false, true} {
			for seed := range uint64(8) {
				g := NewWith(seed, p)
				g.TotalCourses = 16
				g.Hard = hard
				g.Themes = make([]Theme, 16)
				for i := range g.Themes {
					g.Themes[i] = th
				}
				for level := 1; level <= 16; level++ {
					g.Level = level
					g.Stage, g.Course = (level-1)/Courses+1, (level-1)%Courses
					g.startCourse()
					g.courseRow = 0
					for range 45 {
						before := g.alcoveFor
						row := g.buildRow(true)
						g.courseRow++
						x := g.alcove
						if before != 0 || g.alcoveFor != 1 || x < 0 || x >= W || row[x].Sweet == SweetNone || !g.themed(false) {
							continue
						}
						side := 1 // the road is on this side of the dent
						if l := max(x-1, 0); l < x && row[l].Wall == 0 {
							side = -1
						}
						next := g.buildRow(true)
						g.courseRow++
						if in := x + side; in < 0 || in >= W || next[in].Wall != 0 || g.alcove != x || g.sinceTrap == 0 {
							// the road has moved away from the dent (and a new one may be beside
							// it), or a long straight run ended with a block there (trap)
							continue
						}
						dents++
						if next[x].Wall != 0 {
							t.Errorf("theme %d hard %v seed %d course %d row %d: the dent at column %d with sweet %d is one row deep", th, hard, seed, level, g.courseRow-2, x, row[x].Sweet)
						}
					}
				}
			}
		}
	}
	if dents == 0 {
		t.Fatal("no dent with a sweet on a themed course was built")
	}
}

// TestRetryTakesTheSweetUnderHer starts a retry where a sweet lies on the cell she is put
// on, and she stands still: she takes it, as she takes a sweet that comes into her row
// where she stands. Nothing took it until she moved, so standing still let it go by.
func TestRetryTakesTheSweetUnderHer(t *testing.T) {
	t.Parallel()
	g := NewWith(1, Profile{Speed: 1, MaxWidth: 5})
	g.X = 5.5 // beside the trail of sweets down the middle that opens the game
	for range 32 {
		g.Step()
	}
	g.X, g.Missed = 4.5, true // a miss on the trail
	g.Restart()
	lo, hi := span(g.X)
	for c := lo; c <= hi; c++ {
		if g.Rows[PlayerRow][c].Sweet != SweetNone {
			t.Errorf("a sweet lies under her as the retry starts (column %d), not taken", c)
		}
	}
	g.Step() // she stands still
	if g.Streak == 0 {
		t.Errorf("standing still on the trail of sweets after the retry broke the streak (events %+v)", g.Events)
	}
}

// TestASweetBesideHerBodyCanStillBeTaken slides her into a sweet of the row that has just
// gone by her row, while it is still beside her body (the first half of the next row's
// time, when that row is drawn mostly above her and a wall in the row gone by stops her
// from the side, see sideRow): she takes it. The sweet was told as let go of as soon as
// its row left hers, so sliding into it in the picture took nothing, and the streak of
// sweets was already broken. Once the row is past her body (ReachHalfway) a sweet left in
// it is let go of, as before.
func TestASweetBesideHerBodyCanStillBeTaken(t *testing.T) {
	t.Parallel()
	setup := func() *Game {
		g := New(1)
		g.Rows, g.Ahead, g.Safe = [Rows]Row{}, Row{}, 0
		g.X, g.Streak = 4.84, 3 // covers column 4 only
		g.Rows[PlayerRow][5].Sweet = SweetCandy
		g.Step()
		g.Events = g.Events[:0]
		return g
	}
	g := setup()
	g.Move(0.06) // into column 5, before the halfway point
	if g.Sweets != 1 || g.Streak != 4 || g.Rows[PlayerRow+1][5].Sweet != SweetNone {
		t.Errorf("sliding into the sweet beside her body: %d sweets, streak %d, the sweet still there: %v", g.Sweets, g.Streak, g.Rows[PlayerRow+1][5].Sweet != SweetNone)
	}
	for _, ev := range g.Events {
		if ev.Kind == EventMiss {
			t.Errorf("the sweet she took was told as let go of: %+v", g.Events)
		}
	}

	g = setup()
	g.ReachHalfway() // past her body: let go of
	missed := false
	for _, ev := range g.Events {
		missed = missed || ev.Kind == EventMiss
	}
	if !missed || g.Streak != 0 {
		t.Errorf("the sweet that went by her body was not let go of (events %+v, streak %d)", g.Events, g.Streak)
	}
	g.Move(0.06)
	if g.Sweets != 0 {
		t.Error("a sweet let go of was taken after all")
	}
}
