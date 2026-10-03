package road

import "math"

// Theme is what a course is about: the shape of its road and what stands in it. A game
// gives every course a theme (Game.Themes); a course's vault and its
// feast come first, and then the road follows the theme.
type Theme uint8

// The themes of a course.
const (
	ThemeMixed     Theme = iota // the road wanders, with sections and obstacles at random
	ThemeWarmUp                 // a wide road that bends gently: the first course
	ThemeSnake                  // a narrow road winding in long curves
	ThemeSwing                  // the road swings from one side of the screen to the other
	ThemeSlalom                 // blocks jut in from the left and the right in turn
	ThemePillars                // a wide road with pillars standing in a staggered pattern
	ThemeGates                  // walls across the road, each with a gap of two at the other side
	ThemeStairs                 // a narrow road stepping sideways, turning at the sides
	ThemeFunnel                 // the road narrows to two cells and opens up again, over and over
	ThemeSplit                  // a wall down the middle parts the road in two lanes for a while
	ThemeRain                   // single blocks scattered over a wide road
	ThemeChicane                // a narrow road that jumps from side to side in S-bends
	ThemeCheckers               // rows of blocks in a checkered pattern across a wide road
	ThemeCorridor               // a long, nearly straight road two cells wide (three, when hard)
	ThemeHourglass              // the road wide and narrow in quick turns
	ThemeComb                   // teeth of blocks reaching in from one wall
	ThemeStepGates              // walls across the road with gaps that step along diagonally
	ThemeDiamonds               // diamond-shaped islands of blocks parting the road around them
	ThemeEdgeRun                // a narrow road along one side of the screen, then the other
	ThemeLanes                  // two walls part the road into three lanes for a while
	ThemeWobble                 // a road that shakes from side to side
	themeCount
)

// theme is the theme of the course being built (ThemeMixed without Themes).
func (g *Game) theme() Theme {
	if i := g.Level - 1; i >= 0 && i < len(g.Themes) {
		return g.Themes[i]
	}
	return ThemeMixed
}

// themed reports whether the row being built follows the course's theme: a theme other
// than ThemeMixed, with no vault or feast due or running.
func (g *Game) themed(specialDue bool) bool {
	return g.theme() != ThemeMixed && !specialDue &&
		g.section != sectionVault && g.section != sectionFeast
}

// themeTarget is where the theme wants the road on row r of the theme (0 for its first
// row): the middle column and the width. The road moves toward it one cell a row.
func (g *Game) themeTarget(t Theme, r int) (center, width int) {
	hard := g.hardN()
	wave := func(period, amp float64) int {
		return int(math.Round(W/2 + amp*math.Sin(2*math.Pi*float64(r)/period)))
	}
	switch t {
	case ThemeWarmUp:
		return wave(36, 1), 6 - hard
	case ThemeSnake:
		return wave(18, 3), 3 - hard
	case ThemeSwing:
		// a triangle wave from one side to the other
		p := float64(r%24) / 24
		tri := 1 - math.Abs(2*p-1) // 0 -> 1 -> 0
		return int(math.Round(2 + 4*tri)), 4 - hard
	case ThemeSlalom:
		return W / 2, 6 - hard
	case ThemePillars:
		// the road edges over to the right side and narrows as the course goes on, from
		// seven cells to four (three, when hard), so the weave through the pillars tightens
		p := math.Min(1, float64(r)/48)
		return W/2 + int(math.Round(2*p)), 7 - int(math.Round(float64(3+hard)*p))
	case ThemeRain, ThemeSplit:
		return W / 2, 7
	case ThemeGates:
		return W / 2, 6
	case ThemeStairs:
		return g.center, 3 // the steps move the middle themselves (buildThemedRow)
	case ThemeChicane:
		every := 8 - 2*hard
		if (r/every)%2 == 0 {
			return 2, 3 - hard
		}
		return W - 3, 3 - hard
	case ThemeCheckers, ThemeDiamonds, ThemeLanes:
		return W / 2, 7
	case ThemeCorridor:
		if g.hard() {
			// three wide: two wide on a road that bends this much left no time to follow a
			// bend on the fast last courses
			return wave(24, 2), 3
		}
		return wave(40, 1), 2
	case ThemeHourglass:
		if (r/4)%2 == 0 {
			return W / 2, 7
		}
		return W / 2, 3 - hard
	case ThemeComb, ThemeStepGates:
		return W / 2, 6
	case ThemeEdgeRun:
		if (r/16)%2 == 0 {
			return 1, 3 - hard
		}
		return W - 2, 3 - hard
	case ThemeWobble:
		return W/2 + []int{0, 1, 0, -1}[(r/2)%4], 4 - hard
	case ThemeFunnel:
		f := 0.5 + 0.5*math.Cos(2*math.Pi*float64(r)/14)
		return W / 2, 2 + int(math.Round(5*f)) - hard*int(math.Round(f))
	default:
		return g.center, g.width
	}
}

// settleRows is how many themed rows come without the theme's blocks after rows that are
// not themed.
const settleRows = 6

// hard reports whether the course being built is tight: every course of the extra stages
// (Hard), and the regular courses from HardFrom on.
func (g *Game) hard() bool { return g.Hard || g.HardFrom > 0 && g.Level >= g.HardFrom }

// hardN is 1 on a tight course (hard) and 0 otherwise.
func (g *Game) hardN() int {
	if g.hard() {
		return 1
	}
	return 0
}

// buildThemedRow builds a row of a themed course: the road moves toward the theme's
// target (one cell a row, the middle or the width but not both, so an edge never jumps),
// and the theme's blocks stand in it. Every row is checked against the cells she can be
// on in the row before (reach): if the theme's blocks would leave her no way on, they are
// left out of that row.
func (g *Game) buildThemedRow() (Row, int, int) {
	t := g.theme()
	r := g.courseRow
	wantC, wantW := g.themeTarget(t, r)
	if g.Bonus() {
		wantW = min(W-2, wantW+2) // a bonus course is easy going: two cells wider
	}
	if every := 3 - g.hardN(); t == ThemeStairs && r%every == 0 {
		// three cells wide, a step sideways every third row (every other, when hard),
		// turning at the sides: a step every other row on a road two wide was too hard
		// to follow
		if (g.dir < 0 && g.center <= 1) || (g.dir > 0 && g.center >= W-1) {
			g.dir = -g.dir
		}
		wantC = g.center + g.dir
	}
	// a road four cells wide or less shifts at most every other row, and one two wide every
	// third: shifting every row asks for a dash on every row, and a diagonal two wide
	// leaves no time to follow it (too much on a fast or sped-up road)
	canShift := g.width > 4 || g.width >= 3 && g.still >= 1 || g.still >= 2
	g.shifted = false
	switch {
	case g.hold > 0: // just after a trap: the road keeps its shape (see trap)
	case g.width > wantW:
		g.width--
	case g.width < wantW:
		g.width++
	case g.center < wantC && canShift:
		g.center++
		g.shifted = true
	case g.center > wantC && canShift:
		g.center--
		g.shifted = true
	}
	if g.still++; g.shifted {
		g.still = 0
	}
	left := g.center - g.width/2
	right := left + g.width - 1
	if left < 0 {
		left, right = 0, g.width-1
	}
	if right > W-1 {
		right, left = W-1, W-g.width
	}
	// the middle follows the road kept on the screen: the next row (of this course or the
	// next, which may build its rows another way) goes on from where this one is
	g.center = left + g.width/2
	var row Row
	for x := range W {
		if x < left || x > right {
			row[x].Wall = g.WallColor()
		}
	}
	walls := row // the road alone, without the theme's blocks
	if g.settle > 0 {
		// just after a vault or a feast (or rows not themed) the road comes back to the theme
		// first: a gate right behind a vault asked her to cross the whole road in a row
		g.settle--
		return row, left, right
	}
	g.themeBlocks(&row, t, r, left, right)
	if !g.passable(row) {
		row = walls
	}
	return row, left, right
}

// themeBlocks puts the theme's blocks in the road of row r (between left and right).
func (g *Game) themeBlocks(row *Row, t Theme, r, left, right int) {
	hard := g.hard()
	block := func(x int) {
		if x >= left && x <= right {
			row[x].Wall = g.WallColor()
		}
	}
	switch t {
	case ThemeSlalom:
		// a block of three juts in every fourth row, from each side in turn
		every := 4
		if r > 0 && r%every == 0 && right-left >= 4 {
			from := left
			if (r/every)%2 == 1 {
				from = right - 2
			}
			for x := from; x < from+3; x++ {
				block(x)
			}
		}
	case ThemePillars:
		// a pillar every third row that steps one cell along each time, turning at the
		// sides of the road, so she weaves past them on a slant (pillars that stood in one
		// column all the way made a dull straight line); a second one beside it, three
		// cells away, while the road is wide enough, when hard
		if r > 0 && r%3 == 0 && right-left >= 2 {
			// on a wide road the pillars keep off the walls; on a narrow one they stand at
			// the sides too, or they would line up down the middle again
			first, last := left+1, right-1
			if right-left < 4 {
				first, last = left, right
			}
			span := last - first + 1
			k := r / 3
			pos := k % (2 * span)
			if pos >= span {
				pos = 2*span - pos - 1
			}
			block(first + pos)
			if hard && right-left >= 6 {
				block(first + (pos+3)%span)
			}
		}
	case ThemeGates:
		// a wall across every sixth row with a gap of two, at the left and the right in turn
		// (closer, there was no time to cross the road between them on a fast road)
		every := 6
		if r > 0 && r%every == 0 && right-left >= 3 {
			gap := left
			if (r/every)%2 == 1 {
				gap = right - 1
			}
			for x := left; x <= right; x++ {
				if x != gap && x != gap+1 {
					block(x)
				}
			}
		}
	case ThemeSplit:
		// a wall down the middle for twelve rows in every twenty, parting two lanes
		if p := r % 20; p >= 4 && p < 16 && right-left >= 4 {
			block((left + right) / 2)
		}
	case ThemeCheckers:
		// every third row a row of blocks on every other cell, the pattern shifting by one
		// each time (every second row, it asked for a weave one cell a row: too much)
		every := 3
		if r > 0 && r%every == 0 {
			for x := left + 1 + (r/every)%2; x < right; x += 2 {
				block(x)
			}
		}
	case ThemeComb:
		// a tooth of three blocks (four, when hard) reaching in every third row, from the
		// left wall for a while and then from the right
		n := 3
		if hard {
			n = 4
		}
		if r > 0 && r%3 == 0 {
			for i := range n {
				if (r/9)%2 == 0 {
					block(left + i)
				} else {
					block(right - i)
				}
			}
		}
	case ThemeStepGates:
		// a wall across every fifth row with a gap of two that moves two cells along each
		// time, bouncing between the sides
		if r > 0 && r%5 == 0 && right-left >= 3 {
			k := r / 5
			span := right - left - 1 // gap positions 0..span
			pos := (2 * k) % (2 * span)
			if pos > span {
				pos = 2*span - pos
			}
			gap := left + pos
			for x := left; x <= right; x++ {
				if x != gap && x != gap+1 {
					block(x)
				}
			}
		}
	case ThemeDiamonds:
		// every eighth row a diamond of blocks in the middle: one, three, one
		mid := (left + right) / 2
		switch r % 8 {
		case 3, 5:
			block(mid)
		case 4:
			block(mid - 1)
			block(mid)
			block(mid + 1)
		}
	case ThemeLanes:
		// two walls part three lanes for ten rows in every sixteen
		if p := r % 16; p >= 3 && p < 13 && right-left >= 6 {
			block(left + 2)
			block(right - 2)
		}
	case ThemeRain:
		// single blocks dropped here and there, never next to another (so there is always
		// a way around one)
		n := 1
		if hard {
			n = 2
		}
		for i := range n {
			// the second block of a tight course comes less often: two a row on a fast
			// road left no time to weave through them
			chance := 0.55
			if i > 0 {
				chance = 0.3
			}
			if g.rng.Float64() > chance {
				continue
			}
			x := left + g.rng.IntN(right-left+1)
			near := false
			for dx := -1; dx <= 1; dx++ {
				if nx := x + dx; nx >= 0 && nx < W && (row[nx].Wall != 0 && nx >= left && nx <= right || g.lastRow[nx].Wall != 0 && nx >= left && nx <= right) {
					near = true
				}
			}
			if !near {
				block(x)
			}
		}
	default: // the road alone: its shape is the theme
	}
}

// passable reports whether she can get onto row from a cell she can be on in the row
// before (g.reach): straight on, or sliding one cell along the row before.
func (g *Game) passable(row Row) bool {
	for x := range W {
		if row[x].Wall != 0 {
			continue
		}
		for px := x - 1; px <= x+1; px++ {
			if px >= 0 && px < W && g.reach[px] && g.lastRow[x].Wall == 0 {
				return true
			}
		}
	}
	return false
}

// updateReach works out the cells of row she can be on, from those of the row before.
func (g *Game) updateReach(row Row) {
	var next [W]bool
	any := false
	for x := range W {
		if row[x].Wall != 0 {
			continue
		}
		for px := x - 1; px <= x+1; px++ {
			if px >= 0 && px < W && g.reach[px] && g.lastRow[x].Wall == 0 && g.lastRow[px].Wall == 0 {
				next[x], any = true, true
			}
		}
	}
	if !any { // (cannot happen on a passable road) start over from every open cell
		for x := range W {
			next[x] = row[x].Wall == 0
		}
	}
	g.reach = next
}
