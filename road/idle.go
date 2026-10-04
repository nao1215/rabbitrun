package road

import "math/rand/v2"

// The idle rule: on the courses up to Game.IdleUntil (the first stage of the regular side),
// a road that would let her stand still for more than half a screen gets one block
// in her way. Where she stands is the lazy line: where she is if she only moves when she
// has to. When the road ahead would leave it standing in one column for more than IdleRows
// rows, a block comes into that column, on the last row before that where a block has a
// way around it of one step.
//
// The block is laid on the row as it is shown and nothing else: the road is built as before
// (the reach, the walls of the row before, the columns' open runs and the rest of the state
// the next rows are built from never see it). So the rule can look ahead at the very rows
// that come next (it builds them on a copy of the game), and every row past its courses is
// the row it always was.

// IdleRows is the most rows in a row she may stand in one column on a course the idle rule
// covers: half the screen.
const IdleRows = Rows / 2

// OpeningRows is how many rows the run opens with: the road settling and the trail of
// sweets down the middle that leads to the first extra life. The idle rule leaves them as
// they are, and her standing still there does not count.
const OpeningRows = settleRows + trailRows

// idleState is the lazy line and the idle rule's last block, carried from row to row (and
// course to course: see roadShape).
type idleState struct {
	x, run int     // the lazy line's column and the rows it has stood there
	shown  Row     // the row last built as it is shown (with the idle rule's block)
	shown2 Row     // and the one before it
	since  int     // rows since the idle rule's last block
	blockX int     // its column
	sides  [2]bool // its sides (left, right) that were open: she may have stepped there
}

// clearRows is how many rows after an idle block the cell she steps to around it stays
// open (and no other idle block comes beside the block): a wall right after the step was two
// moves too close together. So does the other side, if open (she may step there instead).
const clearRows = 2

// lookRows is how many rows the idle rule looks ahead: enough to see where the lazy line
// has to move, and where it has to move again after a block.
const lookRows = 2*IdleRows + clearRows + 1

// opening reports whether the row being built is one of the run's OpeningRows.
func (g *Game) opening() bool { return g.Stage == 1 && g.Course == 0 && g.stageRow <= OpeningRows }

// LazyStep is where she is on row if she was in column x on the row before (last) and only
// moves when she has to: she stays in her column while it is open, and when a wall comes
// into it she slides along the row before to the nearest open cell of row. When the nearest
// cells either way are as near, she goes to the side with more open road, and then toward
// the middle of the screen. It is -1 when she cannot get onto row from x.
func LazyStep(x int, row, last Row) int {
	if x < 0 || x >= W {
		return -1
	}
	if row[x].Wall == 0 {
		return x
	}
	best, bestDist, bestRoom := -1, 0, 0
	for _, d := range []int{1, -1} { // toward the middle first, so a tie keeps it
		if x >= W/2 {
			d = -d
		}
		for nx := x + d; nx >= 0 && nx < W && last[nx].Wall == 0; nx += d {
			if row[nx].Wall != 0 {
				continue
			}
			room := 0
			for rx := nx; rx >= 0 && rx < W && row[rx].Wall == 0; rx += d {
				room++
			}
			if dist := abs(nx - x); best < 0 || dist < bestDist || dist == bestDist && room > bestRoom {
				best, bestDist, bestRoom = nx, dist, room
			}
			break
		}
	}
	return best
}

// NearestOpen is the open cell of row nearest column x (x itself when there is none), where
// the lazy line goes when it is in a pocket with no way on (she had to leave it before).
func NearestOpen(row Row, x int) int {
	best := -1
	for nx := range W {
		if row[nx].Wall == 0 && (best < 0 || abs(nx-x) < abs(best-x)) {
			best = nx
		}
	}
	if best < 0 {
		return x
	}
	return best
}

// lazyNext is where the lazy line in column x on last is on row (see LazyStep), and whether
// it moved.
func lazyNext(x int, row, last Row) (int, bool) {
	switch nx := LazyStep(x, row, last); {
	case nx < 0:
		return NearestOpen(row, x), true
	case nx != x:
		return nx, true
	default:
		return x, false
	}
}

// InChosenLane reports whether column x of row (after last) is in one of the two lanes a
// wall down the middle of the wide road parts it into on a course of theme t: the fork's and
// the second helpings'. Which lane she runs down is her choice, the easy one beside a harder
// one with a prize in it (and their layouts are exact), so standing still there is the rest
// she chose and not idle. The wall stands in the row before too (a single block in the
// middle is not a wall down it).
func InChosenLane(t Theme, row, last Row, x int) bool {
	return (t == ThemeFork || t == ThemeSeconds) && x != W/2 && row[W/2].Wall != 0 && last[W/2].Wall != 0 &&
		row[W/2-1].Wall == 0 && row[W/2+1].Wall == 0
}

// IdleReset reports whether standing still on row (after last, on a course of theme t) is
// not idle, so the lazy line's count starts again: a row of a feast (a sweet on every open
// cell, picked up standing still) or a lane she chose (InChosenLane).
func IdleReset(t Theme, row, last Row, x int) bool {
	open, sweets := 0, 0
	for _, c := range row {
		if c.Wall == 0 {
			open++
			if c.Sweet != SweetNone {
				sweets++
			}
		}
	}
	return open >= 5 && sweets == open || InChosenLane(t, row, last, x)
}

// lookRow is a row of the road as the game builds it (without the idle rule's block), with
// what the idle rule needs to know about it.
type lookRow struct {
	row     Row
	takes   bool             // the idle rule may lay a block on it at all (see idleTakes)
	covered bool             // it is on a course the idle rule covers (Game.IdleUntil)
	figure  bool             // it has blocks of a designed theme's figure
	mixed   bool             // it is a row of the mixed road
	vault   bool             // it is a row of a vault at its full width
	cage    [2]int           // the first and last columns of the vault's cage
	designR func(x int) bool // a designed theme lays a sweet in column x
}

// idleBlock lays the idle rule's block on row (as it is shown) and moves the lazy line on.
func (g *Game) idleBlock(row Row) Row {
	last, last2 := g.idle.shown, g.idle.shown2
	g.idle.since++
	if !g.lookingAhead && g.idleTakes() && g.idle.run >= 1 {
		look := append([]lookRow{g.lookHere(row)}, g.lookAhead()...)
		if x := g.idle.x; row[x].Wall == 0 && g.blockNow(look, last, last2, x, g.idle.run+1) {
			row[x].Wall = g.WallColor()
			if s := row[x].Sweet; s != SweetNone {
				// the sweet steps aside with her, onto the cell she goes around the block by
				// (or the other side), unless one worth as much lies there already
				row[x].Sweet = SweetNone
				d := LazyStep(x, row, last)
				for _, c := range []int{d, 2*x - d} {
					if c >= 0 && c < W && row[c].Wall == 0 && sweetWorth(row[c].Sweet) < sweetWorth(s) {
						row[c].Sweet = s
						break
					}
				}
			}
			g.idle.since, g.idle.blockX = 0, x
			g.idle.sides = [2]bool{x > 0 && row[x-1].Wall == 0, x < W-1 && row[x+1].Wall == 0}
		}
	}
	x, moved := lazyNext(g.idle.x, row, last)
	if g.idle.x, g.idle.run = x, g.idle.run+1; moved {
		g.idle.run = 1
	}
	if g.opening() || IdleReset(g.theme(), row, last, g.idle.x) {
		g.idle.run = 0
	}
	g.idle.shown2, g.idle.shown = g.idle.shown, row
	return row
}

// idleTakes reports whether the idle rule covers the row being built and may lay a block on
// it: not in the opening, a feast, nor the layout of a laid-out theme (in a vault, only in
// the lane beside the cage).
func (g *Game) idleTakes() bool {
	if g.Level > g.IdleUntil || g.finishing || g.AllClear || g.opening() || g.feastRow {
		return false
	}
	switch g.section {
	case sectionNone, sectionVault:
	case sectionFeast:
		if g.sectionRow > 0 {
			return false
		}
	default:
		return false
	}
	t := g.theme()
	return !t.laidOut() || !g.themeRow || g.layoutRest(t, g.courseRow)
}

// lookHere is the row being built as the idle rule sees it.
func (g *Game) lookHere(row Row) lookRow {
	left, right := roadSpan(g.center, g.width)
	t, r := g.theme(), g.courseRow
	designed := g.themeRow && t.designed() && t != ThemeAlcoves // (the alcoves' sweets lie in dents)
	c := left
	if g.Level%2 == 0 {
		c = right - 3
	}
	return lookRow{row: row, takes: g.idleTakes(), covered: g.Level <= g.IdleUntil && !g.finishing && !g.AllClear, figure: g.themeRow && t.designed() && g.figure,
		mixed: t == ThemeMixed, vault: g.section == sectionVault && g.width == vaultWidth, cage: [2]int{c, c + 3},
		designR: func(x int) bool {
			if !designed {
				return false
			}
			d := g.walled(left, right)
			g.designSweets(&d, t, r, left, right)
			return d[x].Sweet != SweetNone
		}}
}

// lookAhead builds the next lookRows rows of the road on a copy of the game, as the game
// will build them (the idle rule's blocks do not change them).
func (g *Game) lookAhead() []lookRow {
	b, err := g.src.MarshalBinary()
	if err != nil {
		return nil
	}
	c := *g
	c.src = &rand.PCG{}
	if c.src.UnmarshalBinary(b) != nil {
		return nil
	}
	c.rng = rand.New(c.src) //nolint:gosec // G404: game randomness, not security sensitive
	c.Events, c.taken, c.lookingAhead = nil, nil, true
	c.starts = make(map[int]roadShape, len(g.starts))
	for k, v := range g.starts {
		c.starts[k] = v
	}
	out := make([]lookRow, 0, lookRows)
	for range lookRows {
		c.nextCourse()
		if c.AllClear || c.finishing {
			break
		}
		out = append(out, c.lookHere(c.buildRow(true)))
	}
	return out
}

// blockNow reports whether the idle rule's block goes in column x of the row being built
// (look[0], after last and last2), where the lazy line has stood run rows (this row too):
// the road ahead (look) would keep it there past IdleRows rows, and this is the last row
// before that on which a block fits (blockFits), or a row past it, too late, that fits.
func (g *Game) blockNow(look []lookRow, last, last2 Row, x, run int) bool {
	if run-1+standing(look, x) <= IdleRows {
		return false // the road moves her on in time
	}
	latest := -1
	for j := 0; j < len(look) && j <= IdleRows+1-run; j++ {
		if g.blockFits(look, last, last2, x, j, 1) {
			latest = j
		}
	}
	if latest < 0 {
		return g.blockFits(look, last, last2, x, 0, 1) // too late: as soon as it fits
	}
	return latest == 0
}

// standing is how many of the rows look the lazy line in column x stands there before it has
// to move, or before the courses the idle rule covers end (all of them, when it does not).
func standing(look []lookRow, x int) int {
	for i, l := range look {
		if l.row[x].Wall != 0 || !l.covered {
			return i
		}
	}
	return len(look)
}

// blockFits reports whether a block fits in column x of row j of look (look[0] is the row
// being built, after last and last2): the idle rule may lay a block on that row (its course
// and its section; not a row of a designed theme's figure, nor a designed sweet's cell; in a
// vault only in the lane beside the cage), the road keeps two cells open (three on the mixed
// road), and it does not come beside the idle rule's last block within clearRows rows. She
// steps around it to an open neighbour that was open in the row before too (LazyStep), one
// step, and a neighbour open beside it was open in the two rows before. The cells she may step
// to stay open for clearRows rows. And where she
// steps to she does not stand for more than IdleRows rows again with no row for a block in
// time (looking depth blocks ahead).
func (g *Game) blockFits(look []lookRow, last, last2 Row, x, j, depth int) bool {
	l := look[j]
	if !l.takes || l.figure || l.designR(x) || W-walls(l.row) < 2 || l.mixed && W-walls(l.row) < 3 {
		return false
	}
	if g.idle.since+j <= clearRows && (x == g.idle.blockX-1 && g.idle.sides[0] || x == g.idle.blockX+1 && g.idle.sides[1]) {
		return false // she may have stepped there around the last one
	}
	prev, prev2 := last, last2
	if j >= 1 {
		prev, prev2 = look[j-1].row, last
		if j >= 2 {
			prev2 = look[j-2].row
		}
	}
	for _, nx := range []int{x - 1, x + 1} {
		if nx >= 0 && nx < W && l.row[nx].Wall == 0 && (prev[nx].Wall != 0 || prev2[nx].Wall != 0) {
			return false // an open side that was walled just now: no room beside it before
		}
	}
	blocked := l.row
	blocked[x].Wall = 1
	d := LazyStep(x, blocked, prev)
	if d < 0 || abs(d-x) != 1 {
		return false
	}
	if l.vault && (x >= l.cage[0] && x <= l.cage[1] || d >= l.cage[0] && d <= l.cage[1]) {
		return false
	}
	for _, s := range []int{x - 1, x + 1} {
		if s < 0 || s >= W || blocked[s].Wall != 0 {
			continue
		}
		for k := 1; k <= clearRows; k++ {
			if j+k >= len(look) || look[j+k].row[s].Wall != 0 {
				return false
			}
		}
	}
	if depth == 0 {
		return true
	}
	// where she steps to: moved on in time, or a block fits there in time
	if 1+standing(look[j+1:], d) <= IdleRows {
		return true
	}
	for k := j + 1 + clearRows; k < len(look) && k <= j+IdleRows; k++ {
		if g.blockFits(look, last, last2, d, k, depth-1) {
			return true
		}
	}
	return false
}

// layoutRest reports whether row r of a course of the laid-out theme t is outside its
// layout: before it begins, between two trips of second helpings or before the first, and
// past the last trip or phrase the course has room for.
func (g *Game) layoutRest(t Theme, r int) bool {
	if g.designFrom < 0 || r < g.designFrom {
		return true
	}
	if t == ThemeSeconds {
		return g.tripAt(r).seg == 0
	}
	q := r - g.designFrom
	return q >= lessonLearn && (q-lessonLearn)/lessonPhrase >= lessonPhrases(g.lenOf(g.Level), g.designFrom)
}

// vaultWidth is how wide the road is at a vault (see buildRoad).
const vaultWidth = 7

// walls is how many cells of row are walls.
func walls(row Row) int {
	n := 0
	for _, c := range row {
		if c.Wall != 0 {
			n++
		}
	}
	return n
}

// sweetWorth orders the sweets for the one that stays when two meet: an item, a macaron, a
// candy, none.
func sweetWorth(s int8) int {
	switch s {
	case SweetNone:
		return 0
	case SweetCandy:
		return 1
	case SweetMacaron:
		return 2
	default:
		return 3
	}
}
