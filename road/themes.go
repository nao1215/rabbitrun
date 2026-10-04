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
	ThemeSplit                  // the road is squeezed against a side, then opens up and a wall parts it in two lanes
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
	// The designed themes: each is one idea that can be told in a sentence and read on the
	// screen well before she gets there, with its sweets and items laid as part of it (see
	// designSweets). They keep their shape: no block of a long straight run comes into them
	// (trap), and the random sweets stay off them.
	ThemeFork       // a wall down the middle parts the road: a hard lane with a prize at its end, an easy one lined with macarons
	ThemeTrail      // one long S across the screen with a trail of macarons along its middle
	ThemeRooms      // wide rooms joined by short corridors at either side in turn, a line of macarons across each room
	ThemeJar        // the road closes in to a neck at one side, then opens into a field of macarons
	ThemeHammerHall // a hammer lies on the road, and a hall of walls with a wandering doorway follows
	ThemeAlcoves    // a narrow road with dents in its walls on either side in turn, a macaron in each
	ThemeDoors      // walls with two doors at the sides, then one in the middle, a macaron behind one side door
	ThemeWave       // walls whose gaps follow a slow wave across the road, lit by a line of sweets
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
	case ThemeRain:
		return W / 2, 7
	case ThemeSplit:
		// the road is squeezed against one side of the screen (the other side next time),
		// then opens up wide and a wall down the middle parts it in two lanes: a wall in
		// the middle of a wide road alone was a dull straight line
		if p := r % splitPeriod; p < splitSqueeze {
			if (r/splitPeriod)%2 == 0 {
				return 1, 3
			}
			return W - 2, 3
		}
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
	case ThemeFork, ThemeRooms, ThemeJar, ThemeDoors, ThemeWave:
		return W / 2, 7 // the walls inside the road make the shape (themeBlocks)
	case ThemeTrail:
		return wave(trailPeriod, 2.5), 4 - hard
	case ThemeHammerHall:
		return W / 2, 5
	case ThemeAlcoves:
		return wave(32, 1), 3
	default:
		return g.center, g.width
	}
}

// settleRows is how many themed rows come without the theme's blocks after rows that are
// not themed.
const settleRows = 6

// The split theme repeats every splitPeriod rows: the first splitSqueeze of them squeeze
// the road against a side of the screen, the rest open it up and part it in two lanes.
const (
	splitPeriod  = 32
	splitSqueeze = 12
)

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
		// turn where the road meets the side: its middle stops half a road short of the
		// edge (turning only at the last column left a three-wide road stuck on one side,
		// a long straight wall down the screen)
		lo, hi := g.width/2, W-1-g.width/2
		if (g.dir < 0 && g.center <= lo) || (g.dir > 0 && g.center >= hi) {
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
	left, right := g.keepOnScreen()
	row := g.walled(left, right)
	walls := row // the road alone, without the theme's blocks
	if g.settle > 0 {
		// just after a vault or a feast (or rows not themed) the road comes back to the theme
		// first: a gate right behind a vault asked her to cross the whole road in a row
		g.settle--
		return row, left, right
	}
	g.themeRow = true
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
		// reaching one cell past the middle, so she has to swing from side to side (blocks of
		// three left a straight lane down the middle of a wide road)
		every := 4
		if r > 0 && r%every == 0 && right-left >= 4 {
			n := (right-left+1)/2 + 1
			from := left
			if (r/every)%2 == 1 {
				from = right - n + 1
			}
			for x := from; x < from+n; x++ {
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
		// once the road has opened up after the squeeze, a wall down the middle parts two
		// lanes for a while
		if p := r % splitPeriod; p >= splitSqueeze+6 && p < splitPeriod-4 && right-left >= 4 {
			block((left + right) / 2)
		}
	case ThemeCheckers:
		// every third row (fourth, on the regular side) a row of blocks on every other cell,
		// the pattern shifting by one each time (every second row, it asked for a weave one
		// cell a row: too much). The shifted rows take the cells along the walls too: rows
		// that never blocked the sides left a safe lane down each side of the road. Every
		// fourth of them leaves out an inner block (checkerHole), a spot to stand still in,
		// and the course lays a hammer and an extra life on the way (Game.checkerHelp).
		every := 3
		if !hard {
			every = 4
		}
		if r > 0 && r%every == 0 {
			for x := left + 1 - (r/every)%2; x <= right; x += 2 {
				if x != checkerHole(r/every, left, right) {
					block(x)
				}
			}
		}
	case ThemeComb:
		// a tooth of three blocks (four, when hard) reaching in every third row, from the
		// left wall for a while and then from the right. Where the teeth change sides one
		// tooth is left out: from beside the last tooth of one wall to the far side of the
		// first tooth of the other was two cells or more in two rows, too quick on a fast road
		n := 3
		if hard {
			n = 4
		}
		if r > 0 && r%3 == 0 && r%9 != 0 {
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
			// and the sides, so she passes between the diamond and the wall instead of
			// running down the side untouched
			if right-left >= 6 {
				block(left)
				block(right)
			}
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
			// nor beside a block of the two rows before: a block a step to the side of one just
			// passed asked for a step back at once, too quick on a fast road
			near := false
			for dx := -1; dx <= 1; dx++ {
				if nx := x + dx; nx >= 0 && nx < W && (row[nx].Wall != 0 && nx >= left && nx <= right || g.lastRow[nx].Wall != 0 && nx >= left && nx <= right ||
					dx != 0 && nx >= left && nx <= right && g.openRun[nx] < 2) {
					near = true
				}
			}
			if !near {
				block(x)
			}
		}
	case ThemeFork, ThemeRooms, ThemeJar, ThemeHammerHall, ThemeDoors, ThemeWave:
		g.designBlocks(block, t, r, left, right)
	default: // the road alone: its shape is the theme
	}
}

// checkerHole is the cell of the k-th checker row (between left and right) that the
// checkerboard leaves open, or -1: every fourth checker row leaves out one of its inner
// blocks, the one left of the middle and the one right of it in turn. That cell is open in
// the checker rows before and after it as well, so she can stand there past three of them
// instead of stepping aside for every one (the checkerboard of a whole fast course asked
// for a step on every one of them, with no rest). The blocks along the walls always stay:
// a hole there was a safe lane down the side.
func checkerHole(k, left, right int) int {
	if k%4 != 3 {
		return -1
	}
	var inner []int
	for x := left + 1 - k%2; x <= right; x += 2 {
		if x > left && x < right {
			inner = append(inner, x)
		}
	}
	if len(inner) == 0 {
		return -1
	}
	mid := (left + right) / 2
	if (k/4)%2 == 0 { // the inner block nearest the middle on its left, then on its right
		for i := len(inner) - 1; i >= 0; i-- {
			if inner[i] <= mid {
				return inner[i]
			}
		}
		return inner[0]
	}
	for _, x := range inner {
		if x >= mid {
			return x
		}
	}
	return inner[len(inner)-1]
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

// designed reports whether t is one of the designed themes (ThemeFork and the ones after
// it), which lay their own sweets and keep their shape.
func (t Theme) designed() bool { return t >= ThemeFork && t < themeCount }

// ThemeFork, the fork: every forkPeriod rows a wall down the middle of the wide road, from
// row forkFrom to forkTo of the period, parts it in two lanes three cells wide. One lane
// (the left one, then the right one) has a block in two of its three cells every few rows,
// the open cell stepping from its outer side to its inner side and back: it looks hard,
// but it is a plain weave of one cell at a time. At its very end lies a prize: an extra
// life at the first fork of a course, a hammer at the later ones. The other lane runs
// straight and has a macaron on every other row.
//
// Memorable: one choice, told in a sentence (the hard lane pays). Fair: the wall starts in
// sight, both lanes get through, and once in a lane she never has to cross the wall.
const (
	forkPeriod = 40
	forkFrom   = 8
	forkTo     = 30
)

// ThemeTrail, the macaron trail: a road four cells wide (three when hard) sweeps across the
// screen and back in one long S every trailPeriod rows, with a sweet on every other row
// down its middle and a macaron at each far end of the S.
//
// Memorable: follow the macarons. Fair: the S is so slow that the road never shifts on two
// rows running, and the trail itself shows where the road goes next.
const trailPeriod = 48

// ThemeRooms, rooms and corridors: a room (the whole width of the road, open) and then a
// corridor two cells wide against one side of the road, the left and the right side in
// turn, so each room is crossed from corner to corner. A line of macarons crosses each
// room from the door she came in by to the next one. rooms is the length of a room and of
// a corridor; rooms are shorter when hard.
//
// Memorable: left door, right door, like walking through a house. Fair: the next door is in
// sight from the one before, a whole room away, and the way is drawn on the floor.
func (g *Game) rooms() (room, corridor int) {
	if g.hard() {
		return 7, 4
	}
	return 9, 5
}

// ThemeJar, the candy jar: a field of open road, then the road closes in one cell a row from
// both sides toward a neck (jarClose rows), and the neck runs on (jarNeck rows). The neck is
// three cells wide (two when hard) and stands at the left, the right and the middle of the
// road in turn; the field after it holds a patch of macarons, the jar's sweets. jar is the
// length of a field and the width of the neck; a field is shorter when hard.
//
// Memorable: squeeze through the neck of the jar, then help yourself. Fair: the walls close
// in one cell a row, as fast as she can slide, and the neck shows from the field before.
func (g *Game) jar() (field, neck int) {
	if g.hard() {
		return 8, 2
	}
	return 10, 3
}

const (
	jarClose = 4
	jarNeck  = 6
)

// jarNeckAt is the first cell of the neck of the k-th jar (of width neck) on a road from
// left to right: at the left, the right and the middle in turn.
func jarNeckAt(k, neck, left, right int) int {
	switch k % 3 {
	case 0:
		return left + 1
	case 1:
		return right - neck
	default:
		return (left+right+1)/2 - neck/2
	}
}

// ThemeHammerHall, the hammer hall: every hallPeriod rows a hall of walls across the road,
// one every other row from row hallFrom to hallTo of the period, each with a doorway two
// cells wide that moves one cell along from wall to wall, across the road and back
// (hallDoors). A hammer lies in the middle of the road hallHammer rows into the first
// period, just before the first hall; the next hall has none before it.
//
// Memorable: a hammer, then a wall of walls: smash it, or weave through and keep the hammer
// for the next hall. Fair: each doorway overlaps the one before, so the weave is one cell
// at a time, and a sweet in every doorway shows the way.
const (
	hallPeriod = 40
	hallFrom   = 14
	hallTo     = 32
	hallHammer = 8
)

// hallDoors is where the doorway of each wall of a hall is, from the left of the road.
var hallDoors = [...]int{0, 1, 2, 3, 2, 1}

// ThemeAlcoves, the alcoves: a narrow road with a dent in one of its walls every
// alcoveEvery rows (one fewer when hard), at the left and the right in turn, two rows deep
// and a macaron in each.
//
// Memorable: left, right, left, a macaron in every window. Fair: the road itself is plain;
// darting in and out of the dents is the risk she takes for the macarons, and passing them
// by costs nothing.
const alcoveEvery = 6

// ThemeDoors, two doors and one: every doorEvery rows (one fewer when hard) a wall across
// the wide road: one with a door two cells wide at each side, then one with a door three
// cells wide in the middle, and so on. A macaron waits in one of the two side doors, the
// left one and then the right one.
//
// Memorable: the rhythm side, middle, side, middle, with the macaron's side swapping.
// Fair: every wall has a wide door, the next one is two cells away at most, and no column
// stays open past two walls, so she reads each wall rather than running down a lane.
const doorEvery = 6

// ThemeWave, the wave: every waveEvery rows (one fewer when hard) a wall across the wide
// road with a gap three cells wide (two when hard). The gap's middle follows a slow wave
// across the road, a full swing in waveGates walls. A sweet sits in every gap, a macaron at
// the top of every swing.
//
// Memorable: the gaps make one smooth line, the shape of a wave. Fair: the line can be read
// half a screen ahead, and a gap is never more than two cells from the one before.
const (
	waveEvery = 4
	waveGates = 8
)

// waveGap is the first cell and the width of the gap of the k-th wall of the wave.
func (g *Game) waveGap(k, left int) (from, n int) {
	c := left + 3 + int(math.Round(2*math.Sin(2*math.Pi*float64(k)/waveGates)))
	if g.hard() {
		return c - 1, 2
	}
	return c - 1, 3
}

// designBlocks puts the blocks of a designed theme in row r of the road (between left and
// right, by block).
func (g *Game) designBlocks(block func(int), t Theme, r, left, right int) {
	wall := func(from, n int) { // a wall across the road but for the cells from..from+n-1
		for x := left; x <= right; x++ {
			if x < from || x >= from+n {
				block(x)
			}
		}
	}
	wide := right-left == 6
	switch t {
	case ThemeFork:
		p := r % forkPeriod
		if p < forkFrom || p >= forkTo || !wide {
			return
		}
		mid := (left + right) / 2
		block(mid)
		outer, inner := left, mid-1 // the hard lane
		if (r/forkPeriod)%2 == 1 {
			outer, inner = right, mid+1
		}
		every := 4 - g.hardN()
		if q := p - forkFrom; q%every == 2 && p < forkTo-2 {
			// the open cell steps one cell a time: outer, middle, inner, middle
			gap := outer + []int{0, 1, 2, 1}[(q/every)%4]*(inner-outer)/2
			for x := min(outer, inner); x <= max(outer, inner); x++ {
				if x != gap {
					block(x)
				}
			}
		}
	case ThemeRooms:
		room, corridor := g.rooms()
		if p := r % (room + corridor); p >= room && right-left >= 4 {
			door := left
			if (r/(room+corridor))%2 == 1 {
				door = right - 1
			}
			wall(door, 2)
		}
	case ThemeJar:
		field, neck := g.jar()
		period := field + jarClose + jarNeck
		p := r % period
		if p < field || !wide {
			return
		}
		from := jarNeckAt(r/period, neck, left, right)
		reach := max(0, jarClose-1-(p-field)) // how far from the neck the road is still open
		wall(from-reach, neck+2*reach)
	case ThemeHammerHall:
		if p := r % hallPeriod; p >= hallFrom && p < hallTo && (p-hallFrom)%2 == 0 && right-left >= 4 {
			wall(left+hallDoors[((p-hallFrom)/2)%len(hallDoors)], 2)
		}
	case ThemeDoors:
		every := doorEvery - g.hardN()
		if r > 0 && r%every == 0 && wide {
			if (r/every)%2 == 0 {
				for x := left + 2; x <= right-2; x++ {
					block(x)
				}
			} else {
				wall(left+2, 3)
			}
		}
	case ThemeWave:
		every := waveEvery - g.hardN()
		if r > 0 && r%every == 0 && wide {
			wall(g.waveGap(r/every, left))
		}
	default:
	}
}

// designSweets lays the sweets and items of a designed theme on row r of the road (between
// left and right), on open cells that have nothing on them yet.
func (g *Game) designSweets(row *Row, t Theme, r, left, right int) {
	put := func(x int, s int8) {
		if x >= 0 && x < W && row[x].Wall == 0 && row[x].Sweet == SweetNone {
			row[x].Sweet = s
		}
	}
	wide := right-left == 6
	switch t {
	case ThemeFork:
		p := r % forkPeriod
		if p < forkFrom || p >= forkTo || !wide {
			return
		}
		hard, easy := left+1, right-1 // the middles of the lanes
		if (r/forkPeriod)%2 == 1 {
			hard, easy = easy, hard
		}
		if (p-forkFrom)%2 == 0 {
			put(easy, SweetMacaron)
		}
		if p == forkTo-1 {
			prize := SweetBomb
			if r/forkPeriod == 0 {
				prize = SweetOneUp
			}
			put(hard, prize)
		}
	case ThemeTrail:
		mid := (left + right + 1) / 2
		switch {
		case r%(trailPeriod/2) == trailPeriod/4: // the far ends of the S
			put(mid, SweetMacaron)
		case r%2 == 0:
			put(mid, SweetCandy)
		}
	case ThemeRooms:
		room, corridor := g.rooms()
		p, k := r%(room+corridor), r/(room+corridor)
		if p == 0 || p >= room-1 {
			return
		}
		// from the door she came in by (the other side) to the next one
		from, to := right-1, left+1
		if k%2 == 1 {
			from, to = to, from
		}
		x := from + int(math.Round(float64((to-from)*(p-1))/float64(room-3)))
		switch {
		case p == room/2:
			put(x, SweetMacaron)
		case p%2 == 1:
			put(x, SweetCandy)
		}
	case ThemeJar:
		field, neck := g.jar()
		period := field + jarClose + jarNeck
		p := r % period
		switch {
		case !wide:
		case p >= field+jarClose: // a sweet down the neck
			put(jarNeckAt(r/period, neck, left, right)+neck/2, SweetCandy)
		case p >= 2 && p <= 6 && p%2 == 0: // the patch of macarons in the field
			for x := left + (p/2)%2; x <= right; x += 2 {
				s := SweetCandy
				if p == 4 && x == (left+right)/2 {
					s = SweetMacaron
				}
				put(x, s)
			}
		}
	case ThemeHammerHall:
		p := r % hallPeriod
		switch {
		case p == hallHammer && r < hallPeriod:
			put((left+right)/2, SweetBomb)
		case p >= hallFrom && p < hallTo && (p-hallFrom)%2 == 0:
			put(left+hallDoors[((p-hallFrom)/2)%len(hallDoors)], SweetCandy)
		}
	case ThemeAlcoves:
		every := alcoveEvery - g.hardN()
		if p := r % every; p <= 1 && r >= every {
			x := left - 1
			if (r/every)%2 == 1 {
				x = right + 1
			}
			if p == 0 {
				g.alcove, g.alcoveFor = x, 1
			} else if g.alcoveFor > 0 {
				x, g.alcoveFor = g.alcove, 0 // the second row of the dent, where the first one was
			}
			if x >= 0 && x < W {
				row[x].Wall = 0 // the dent, two rows deep
				if p == 0 {
					put(x, SweetMacaron)
				}
			}
		}
	case ThemeDoors:
		every := doorEvery - g.hardN()
		if r == 0 || r%every != 0 || !wide {
			return
		}
		switch k := r / every; {
		case k%2 == 1:
			put((left+right)/2, SweetCandy)
		case (k/2)%2 == 0:
			put(left, SweetMacaron)
		default:
			put(right, SweetMacaron)
		}
	case ThemeWave:
		every := waveEvery - g.hardN()
		if r == 0 || r%every != 0 || !wide {
			return
		}
		k := r / every
		from, n := g.waveGap(k, left)
		s := SweetCandy
		if k%(waveGates/2) == waveGates/4 {
			s = SweetMacaron
		}
		put(from+n/2, s)
	default:
	}
}
