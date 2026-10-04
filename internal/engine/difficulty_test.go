package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nao1215/rabbitrun/road"
)

// The difficulty and the fun of the courses, measured on the roads themselves.
//
// Every run is built row by row as the game builds it (the roads are the same every game),
// and the rows of each course are measured. A course gets a difficulty score and a fun
// score, each the weighted sum of the components below; a stage and a run score the mean
// of their courses. The components stay visible in the tables the tests log (go test -v).
//
// Difficulty (diffWeights):
//
//   - speed: rows a second (the level's speed times the character's road), the base of
//     everything else
//   - narrow: the share of rows with three open cells or fewer
//   - width: the mean number of open cells a row (counts against the score: a wide road is
//     easy)
//   - lateral: cells a second she must slide sideways on the easiest line through the road
//     (the line that slides least, sliding at most one cell a row)
//   - reflex: moves a second on that line that turn back within reflexRows rows of the move
//     before (a quick left-right that leaves little time to read)
//   - noRest: the longest stretch, in seconds, without a column that has been open for
//     restRows rows (somewhere to stand still)
//   - tight: the share of the line's cells with a wall right beside them (left or right)
//   - pinch: the share of the line's cells that are a gap one cell wide (walls on both
//     sides): a spot to hit exactly, as in a checkerboard
//   - obstacles: rows a second with a block that stands one row alone (a pillar, a gate, a
//     tooth): something to aim past
//   - help: hammers and extra lives lying on the course (counts against the score)
//
// Fun (funWeights):
//
//   - macarons: sweets a second (every sweet on the road is drawn as a macaron; the
//     bigger ones count the same)
//   - perCourse: sweets on the course
//   - items: hammers and extra lives lying on the course
//   - detours: macarons and items off the easiest line (a reward for a harder way)
//   - feast: rows with four sweets or more (a stretch to help herself)
//   - memo: how much the road repeats itself (the share of rows equal to the row a period
//     before, for the best period): a pattern that can be read and remembered
//
// And for a whole run: the themes it has (variety) and the most courses in a row of one
// theme.

// difficulty weights: score = Σ weight × component.
var diffWeights = map[string]float64{
	"speed": 1, "narrow": 3, "width": -0.4, "lateral": 2, "reflex": 2,
	"noRest": 0.25, "tight": 2, "pinch": 8, "obstacles": 1.5, "help": -0.4,
}

// fun weights: score = Σ weight × component.
var funWeights = map[string]float64{
	"macarons": 1, "perCourse": 0.02, "items": 1, "detours": 0.2, "feast": 0.1, "memo": 2,
}

// The component names in the order of the tables.
var (
	diffKeys = []string{"speed", "narrow", "width", "lateral", "reflex", "noRest", "tight", "pinch", "obstacles", "help"}
	funKeys  = []string{"macarons", "perCourse", "items", "detours", "feast", "memo"}
)

const (
	reflexRows = 3 // a turn back within this many rows of the move before is a reflex
	scoreRest  = 7 // rows a column stays open to be a rest (as in the checkerboard test)
)

// courseScore is what was measured on one course of a run.
type courseScore struct {
	level int
	theme road.Theme
	rows  int
	diff  map[string]float64
	fun   map[string]float64
}

// weighted is the score of the components c with the weights w.
func weighted(c, w map[string]float64) float64 {
	s := 0.0
	for k, v := range c {
		s += w[k] * v
	}
	return s
}

func (c courseScore) difficulty() float64 { return weighted(c.diff, diffWeights) }
func (c courseScore) funScore() float64   { return weighted(c.fun, funWeights) }

// runScore is a character's run on one side, course by course.
type runScore struct {
	id      string
	extra   bool
	courses []courseScore
	themes  []road.Theme
}

// mean is the mean of f over the courses from..to-1.
func (r runScore) mean(from, to int, f func(courseScore) float64) float64 {
	s := 0.0
	for _, c := range r.courses[from:to] {
		s += f(c)
	}
	return s / float64(to-from)
}

func (r runScore) overall() float64 { return r.mean(0, len(r.courses), courseScore.difficulty) }
func (r runScore) overallFun() float64 {
	return r.mean(0, len(r.courses), courseScore.funScore)
}

// stage is the mean difficulty of stage s (1-4).
func (r runScore) stage(s int) float64 {
	return r.mean((s-1)*road.Courses, s*road.Courses, courseScore.difficulty)
}

// stageFun is the mean fun of stage s (1-4).
func (r runScore) stageFun(s int) float64 {
	return r.mean((s-1)*road.Courses, s*road.Courses, courseScore.funScore)
}

// longestRepeat is the most courses in a row with the same theme.
func (r runScore) longestRepeat() int {
	best, run := 1, 1
	for i := 1; i < len(r.themes); i++ {
		if r.themes[i] == r.themes[i-1] {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
	}
	return best
}

// variety is how many themes the run has.
func (r runScore) variety() int {
	seen := map[road.Theme]bool{}
	for _, t := range r.themes {
		seen[t] = true
	}
	return len(seen)
}

// runRows builds the whole run of e and returns its rows by course (Level).
func runRows(e *Engine) map[int][]road.Row {
	out := map[int][]road.Row{}
	for !e.G.AllClear {
		built := e.G.Level
		finishing := e.G.Level > GameCourses
		e.G.Step()
		if !finishing && built <= GameCourses {
			out[built] = append(out[built], e.G.Ahead)
		}
		e.G.Safe = 1 // only the road is wanted, not where she stands
		if e.G.Distance > 10000 {
			break
		}
	}
	return out
}

// easiestLine is the line through rows that slides least (at most a cell a row, along
// the row before as the road allows), one column a row; of the lines that slide as little,
// the one with the fewest quick turns back (see reflexRows). A line that moves at the last
// moment and turns back at once is no harder to slide than one that moves early, so the
// turns back counted are only those the road leaves no way around.
func easiestLine(rows []road.Row) []int {
	const (
		inf   = math.MaxInt32
		since = reflexRows + 1 // rows since the last move are counted up to this
		dirs  = 3              // the way of the last move: none, left, right
		perX  = dirs * (since + 1)
		n     = road.W * perX
	)
	state := func(x, d, s int) int { return x*perX + d*(since+1) + s }
	rowsN := len(rows)
	cost := make([][n]int, rowsN)
	from := make([][n]int, rowsN)
	for i := range cost {
		for k := range n {
			cost[i][k], from[i][k] = inf, -1
		}
	}
	start := func(i int) {
		for x := range road.W {
			if rows[i][x].Wall == 0 {
				cost[i][state(x, 0, since)] = 0
			}
		}
	}
	start(0)
	for i := 1; i < rowsN; i++ {
		any := false
		for k := range n {
			c := cost[i-1][k]
			if c == inf {
				continue
			}
			x, d, s := k/perX, k%perX/(since+1), k%(since+1)
			for m := -1; m <= 1; m++ {
				nx := x + m
				if nx < 0 || nx >= road.W || rows[i][nx].Wall != 0 || rows[i-1][nx].Wall != 0 {
					continue
				}
				next, add := state(nx, d, min(since, s+1)), 0
				if m != 0 {
					nd := 1
					if m > 0 {
						nd = 2
					}
					add = 1000
					if d != 0 && d != nd && s+1 <= reflexRows {
						add++
					}
					next = state(nx, nd, 0)
				}
				if c+add < cost[i][next] {
					cost[i][next], from[i][next] = c+add, k
					any = true
				}
			}
		}
		if !any { // (a road that cannot be followed: start over from the open cells)
			start(i)
		}
	}
	best := -1
	for k := range n {
		if cost[rowsN-1][k] != inf && (best < 0 || cost[rowsN-1][k] < cost[rowsN-1][best]) {
			best = k
		}
	}
	line := make([]int, rowsN)
	for i := rowsN - 1; i >= 0; i-- {
		line[i] = best / perX
		if i > 0 {
			if p := from[i][best]; p >= 0 {
				best = p
			} else { // a fresh start: any state of the row before
				for k := range n {
					if cost[i-1][k] != inf {
						best = k
						break
					}
				}
			}
		}
	}
	return line
}

// openKey is the open cells of a row as bits.
func openKey(r road.Row) int {
	k := 0
	for x := range road.W {
		if r[x].Wall == 0 {
			k |= 1 << x
		}
	}
	return k
}

// scoreRun builds and measures a character's run on one side.
func scoreRun(id string, extra bool) runScore {
	return scoreEngine(NewRun(id, extra), id, extra)
}

// scoreEngine builds and measures the run of e (a run just started).
func scoreEngine(e *Engine, id string, extra bool) runScore {
	themes := e.G.Themes
	byLevel := runRows(e)
	var all []road.Row
	start := map[int]int{}
	for lv := 1; lv <= GameCourses; lv++ {
		start[lv] = len(all)
		all = append(all, byLevel[lv]...)
	}
	line := easiestLine(all)
	if themes == nil {
		themes = make([]road.Theme, GameCourses)
	}
	rs := runScore{id: id, extra: extra, themes: themes}
	for lv := 1; lv <= GameCourses; lv++ {
		rows := byLevel[lv]
		ln := line[start[lv] : start[lv]+len(rows)]
		v := RowsPerSecond(lv) * e.G.Profile.Speed
		secs := float64(len(rows)) / v
		c := courseScore{level: lv, theme: themes[lv-1], rows: len(rows), diff: map[string]float64{}, fun: map[string]float64{}}
		narrow, width, lateral, reflex, tight, pinch, obst := 0, 0, 0, 0, 0, 0, 0
		sweets, items, detours, feast := 0, 0, 0, 0
		lastMove, lastDir := -100, 0
		for i, row := range rows {
			open, here := 0, 0
			for x := range road.W {
				if row[x].Wall == 0 {
					open++
				}
				switch row[x].Sweet {
				case road.SweetNone:
				case road.SweetCandy, road.SweetMacaron:
					sweets++
					here++
					if row[x].Sweet == road.SweetMacaron && x != ln[i] {
						detours++
					}
				default:
					items++
					if x != ln[i] {
						detours++
					}
				}
				// a block standing one row alone, inside the road
				if i > 0 && i+1 < len(rows) && row[x].Wall != 0 && rows[i-1][x].Wall == 0 && rows[i+1][x].Wall == 0 {
					inside := false
					for lx := x - 1; lx >= 0 && !inside; lx-- {
						inside = row[lx].Wall == 0
					}
					right := false
					for rx := x + 1; rx < road.W && !right; rx++ {
						right = row[rx].Wall == 0
					}
					if inside || right {
						obst++
						break
					}
				}
			}
			if here >= 4 {
				feast++
			}
			width += open
			if open <= 3 {
				narrow++
			}
			x := ln[i]
			wallLeft := x > 0 && row[max(0, x-1)].Wall != 0
			wallRight := x < road.W-1 && row[min(road.W-1, x+1)].Wall != 0
			if wallLeft || wallRight {
				tight++
			}
			if (x == 0 || wallLeft) && (x == road.W-1 || wallRight) {
				pinch++
			}
			if i > 0 {
				if d := ln[i] - ln[i-1]; d != 0 {
					lateral += abs(d)
					if d*lastDir < 0 && i-lastMove <= reflexRows {
						reflex++
					}
					lastMove, lastDir = i, d
				}
			}
		}
		// the share of rows equal to the row a period before, for the best period
		memo := 0.0
		for p := 2; p <= 48 && p < len(rows); p++ {
			same := 0
			for i := p; i < len(rows); i++ {
				if openKey(rows[i]) == openKey(rows[i-p]) {
					same++
				}
			}
			memo = math.Max(memo, float64(same)/float64(len(rows)-p))
		}
		n := float64(len(rows))
		c.diff["speed"] = v
		c.diff["narrow"] = float64(narrow) / n
		c.diff["width"] = float64(width) / n
		c.diff["lateral"] = float64(lateral) / secs
		c.diff["reflex"] = float64(reflex) / secs
		c.diff["noRest"] = float64(longestWithoutRest(rows, scoreRest)) / v
		c.diff["tight"] = float64(tight) / n
		c.diff["pinch"] = float64(pinch) / n
		c.diff["obstacles"] = float64(obst) / secs
		c.diff["help"] = float64(items)
		c.fun["macarons"] = float64(sweets) / secs
		c.fun["perCourse"] = float64(sweets)
		c.fun["items"] = float64(items)
		c.fun["detours"] = float64(detours)
		c.fun["feast"] = float64(feast)
		c.fun["memo"] = memo
		rs.courses = append(rs.courses, c)
	}
	return rs
}

// selectOrder is the characters with a run of their own, in the order of the character
// select screen (the "order" of each game.json), left to right.
func selectOrder(t *testing.T) []string {
	t.Helper()
	type manifest struct {
		Order int `json:"order"`
	}
	order := map[string]int{}
	for id := range courseThemes {
		b, err := os.ReadFile(filepath.Join("..", "..", "assets", "characters", id, "game.json")) //nolint:gosec // G304: a character manifest of the game
		if err != nil {
			t.Fatal(err)
		}
		var m manifest
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		order[id] = m.Order
	}
	ids := runIDs()
	sort.Slice(ids, func(i, j int) bool { return order[ids[i]] < order[ids[j]] })
	return ids
}

// allScores measures every run of every character, both sides, in select order.
func allScores(t *testing.T) []runScore {
	t.Helper()
	ids := selectOrder(t)
	out := make([]runScore, 0, 2*len(ids))
	for _, id := range ids {
		for _, extra := range []bool{false, true} {
			out = append(out, scoreRun(id, extra))
		}
	}
	return out
}

func side(extra bool) string {
	if extra {
		return "extra"
	}
	return "regular"
}

// table lays out the scores of every run: by run and stage, and course by course with
// every component.
func table(runs []runScore) string {
	var parts []string
	printf := func(format string, a ...any) { parts = append(parts, fmt.Sprintf(format, a...)) }
	printf("\n%-7s %-7s | %6s | %6s %6s %6s %6s | %6s | %5s %5s %5s %5s | variety repeat\n",
		"char", "side", "diff", "st1", "st2", "st3", "st4", "fun", "f1", "f2", "f3", "f4")
	for _, r := range runs {
		printf("%-7s %-7s | %6.2f | %6.2f %6.2f %6.2f %6.2f | %6.2f | %5.2f %5.2f %5.2f %5.2f | %7d %6d\n",
			r.id, side(r.extra), r.overall(), r.stage(1), r.stage(2), r.stage(3), r.stage(4),
			r.overallFun(), r.stageFun(1), r.stageFun(2), r.stageFun(3), r.stageFun(4), r.variety(), r.longestRepeat())
	}
	printf("\n%-7s %-7s %-6s %5s | %6s |", "char", "side", "course", "theme", "diff")
	for _, k := range diffKeys {
		printf(" %7s", k)
	}
	printf(" | %5s |", "fun")
	for _, k := range funKeys {
		printf(" %9s", k)
	}
	printf("\n")
	for _, r := range runs {
		for _, c := range r.courses {
			printf("%-7s %-7s %d-%-4d %5d | %6.2f |", r.id, side(r.extra), (c.level-1)/road.Courses+1, (c.level-1)%road.Courses+1, c.theme, c.difficulty())
			for _, k := range diffKeys {
				printf(" %7.2f", c.diff[k])
			}
			printf(" | %5.2f |", c.funScore())
			for _, k := range funKeys {
				printf(" %9.2f", c.fun[k])
			}
			printf("\n")
		}
	}
	return strings.Join(parts, "")
}

// TestCourseScores logs the difficulty and the fun of every course of every run.
func TestCourseScores(t *testing.T) {
	t.Parallel()
	t.Log(table(allScores(t)))
}
