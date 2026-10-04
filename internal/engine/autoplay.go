package engine

import (
	"math"

	"github.com/nao1215/rabbitrun/road"
)

// AutoPlayer plays the game by itself, sliding the way a player would, so the rules can
// be seen in motion: for the demo recording, and later for an attract mode on the title.
type AutoPlayer struct {
	// Careful ignores the sweets and only stays safe (to test that the road can be passed):
	// it plans the directions to hold frame by frame over the road on the screen (findWay)
	// and follows the plan.
	Careful bool
	// the direction she holds and for how long, so she speeds up the way a player does
	holdDir, holdFrames int
	// the careful player's plan: the directions to hold in the frames ahead, the rows the
	// road had scrolled when it was made, and where she was left after the last frame (a
	// new plan is made when something else has moved her)
	plan      []int
	planSteps int
	planX     float64
}

// autoLook is how many rows ahead the auto player plans (the careful one reads the whole
// screen, see framesAhead).
const autoLook = 8

// Step slides the player toward the middle of the column the plan heads for (the careful
// player holds the direction its plan gives for the frame).
func (a *AutoPlayer) Step(e *Engine) {
	if e.Over() {
		return
	}
	if a.Careful {
		a.stepCareful(e)
		return
	}
	// how far she can still slide before the next row comes down (starting from rest, or
	// going on at the speed she has, which is optimistic about turning around); a wall
	// hits only halfway down its row (road.Game.ReachHalfway), so this leaves her a margin
	frames := int((1 - e.Scroll()) / e.RowsPerSec() * 60)
	left := slideDistance(e, a.holdFrames, frames)
	target := float64(bestColumn(e.G, movesPerRow(e), left)) + 0.5
	d := target - e.G.X
	dir := 0
	if math.Abs(d) > 1e-9 {
		dir = int(math.Copysign(1, d))
	}
	if dir != a.holdDir {
		a.holdDir, a.holdFrames = dir, 0
	}
	if dir == 0 {
		return
	}
	a.holdFrames++
	v := e.SlideSpeed(a.holdFrames) / 60
	if math.Abs(d) < v {
		v = math.Abs(d)
	}
	e.Move(math.Copysign(v, d))
}

// movesPerRow is how many columns the player can surely slide while the road scrolls
// one row: starting from rest, as she speeds up (SlideSpeed).
func movesPerRow(e *Engine) int {
	frames := int(60 / e.RowsPerSec())
	return max(1, int(slideDistance(e, 0, frames)))
}

// slideDistance is how far she slides in the next frames frames with a direction held,
// having held it for held frames already.
func slideDistance(e *Engine, held, frames int) float64 {
	d := 0.0
	for f := 1; f <= frames; f++ {
		d += e.SlideSpeed(held+f) / 60
	}
	return d
}

// bestColumn plans a way through the next rows: a column is worth the sweets that can
// be picked up from it, and a wall ends a way. It returns the column to head for now.
func bestColumn(g *road.Game, reach int, slide float64) int {
	// Plan as far ahead as a way through exists: when no way gets through all the rows in
	// view, plan for fewer rows, so she still heads for the column that lasts longest.
	for look := autoLook; look > 0; look-- {
		if x, ok := planColumn(g, reach, slide, look); ok {
			return x
		}
	}
	return g.Col()
}

// planColumn plans a way through the next look rows and returns the column to head for
// now, or false when no way gets through.
func planColumn(g *road.Game, reach int, slide float64, look int) (int, bool) {
	const dead = -1 << 20
	worth := func(c road.Cell) int {
		if c.Wall != 0 {
			return dead
		}
		return 1000 * [...]int{0, 3, 5, 12, 20, 15}[c.Sweet]
	}
	// value[x]: the best a way through column x of row y can collect, from the far rows
	// down to the row that reaches the player next. A move costs a little, and more the
	// later it comes, so of two ways she takes the one that moves early (from rest she
	// needs most of a row to slide one cell).
	var value [road.W]int
	top := max(0, road.PlayerRow-look)
	for y := top; y < road.PlayerRow; y++ {
		var next [road.W]int
		for x := range road.W {
			w := worth(g.Rows[y][x])
			if w == dead {
				next[x] = dead
				continue
			}
			best := dead
			if y == top {
				best = 0
			}
			// slide along this row (only over open cells: touching a wall from the side is
			// a miss) to a column that is open in the farther row
			for d := -reach; d <= reach && y > top; d++ {
				if nx := x + d; nx >= 0 && nx < road.W && value[nx] > dead && clearBetween(g, y, x, nx) {
					best = max(best, value[nx]-abs(d)*2*(road.PlayerRow-y))
				}
			}
			if best == dead {
				next[x] = dead
			} else {
				next[x] = w + best
			}
		}
		value = next
	}
	// Head for the best column of the next row that can be reached along the player's
	// row without a wall in between (nearer columns win ties).
	pick, best := g.Col(), dead-1
	row := g.Rows[road.PlayerRow]
	// while the row is still above her body, a slide is judged against the row behind too
	behind := g.SideRowBehind && road.PlayerRow+1 < road.Rows
	open := func(x int) bool {
		return g.Safe > 0 || (row[x].Wall == 0 && (!behind || x == g.Col() || g.Rows[road.PlayerRow+1][x].Wall == 0))
	}
	for _, dir := range []int{-1, 1} {
		for x := g.Col(); x >= 0 && x < road.W && open(x); x += dir {
			if value[x] == dead {
				continue
			}
			// only a column she can reach before the next row arrives, or one she can go on
			// sliding to while the next row passes (the way is open in it too)
			if d := math.Abs(float64(x)+0.5-g.X) - (0.5 - road.Half); d > slide && x != g.Col() &&
				!clearBetween(g, road.PlayerRow-1, g.Col(), x) {
				continue
			}
			if v := value[x] - abs(x-g.Col()); v > best {
				pick, best = x, v
			}
		}
	}
	return pick, best > dead-1
}

// clearBetween reports whether every cell from column a to column b is open in row y (the
// row she slides along before the next one comes down).
func clearBetween(g *road.Game, y, a, b int) bool {
	if a > b {
		a, b = b, a
	}
	for x := a; x <= b; x++ {
		if g.Rows[y][x].Wall != 0 {
			return false
		}
	}
	return true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Plans of the careful player.
const (
	// planBins is how finely the careful player's plans tell places apart (per cell).
	planBins = 10
	// replanRows is how many rows the road may scroll before the careful player plans again
	// with the rows that have come into view.
	replanRows = 3
)

// stepCareful holds the direction the plan gives for this frame, planning again first when
// the plan is used up, old, or no longer where she is.
func (a *AutoPlayer) stepCareful(e *Engine) {
	if len(a.plan) == 0 || e.Steps-a.planSteps >= replanRows || e.G.X != a.planX {
		a.plan, _ = findWay(framesAhead(e), wayStart{x: e.G.X, dir: a.holdDir, held: a.holdFrames}, planBins)
		a.planSteps = e.Steps
	}
	dir := 0
	if len(a.plan) > 0 {
		dir = a.plan[0]
		a.plan = a.plan[1:]
	}
	if dir != a.holdDir {
		a.holdDir, a.holdFrames = dir, 0
	}
	if dir != 0 {
		a.holdFrames++
		e.Move(float64(dir) * e.SlideSpeed(a.holdFrames) / 60)
	}
	a.planX = e.G.X
}

// framesAhead is what the frames ahead will do to her (see frameWalls), as far as the road
// on the screen (and the row about to come in at the top) tells: up to the frame in which a
// row not yet in view would reach halfway down onto her. The road speeds up on the step
// that begins the next course, as Tick will have it (the player's speed-up stays as it is).
func framesAhead(e *Engine) []frameWalls {
	g := e.G
	level, toNext := g.Level, g.StepsToNextCourse()
	rate, top := e.rowsPerSecAt(level)/60, e.playerSpeedAt(level)
	acc, behind, steps := e.stepAcc, g.SideRowBehind, 0
	// the row that will be at screen row y once the road has scrolled steps more rows
	rowAt := func(y int) (road.Row, bool) {
		switch y -= steps; {
		case y >= 0:
			return g.Rows[y], true
		case y == -1:
			return g.Ahead, true
		}
		return road.Row{}, false
	}
	var out []frameWalls
	for len(out) < 60*60 {
		sideY := road.PlayerRow
		if behind && road.PlayerRow+1 < road.Rows {
			sideY = road.PlayerRow + 1
		}
		side, _ := rowAt(sideY)
		fw := frameWalls{side: wallBits(side), top: top}
		before := acc
		acc += rate
		for acc >= 1 {
			acc--
			before = 0
			steps++
			behind = true
			if steps == toNext {
				level++
				top = e.playerSpeedAt(level) // the next frame's slide: the scene slides before it ticks
				rate = e.rowsPerSecAt(level) / 60
			}
		}
		if before < 0.5 && acc >= 0.5 {
			behind = false
			r, ok := rowAt(road.PlayerRow)
			if !ok {
				break
			}
			fw.halfway, fw.half = true, wallBits(r)
		}
		out = append(out, fw)
	}
	return out
}
