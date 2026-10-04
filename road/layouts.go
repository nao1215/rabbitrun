package road

// The themes laid out from the row where they begin (Game.designFrom) rather than from the
// start of the course: ThemeSeconds and ThemeLesson. Each row of them is worked out by a
// function of its row alone (secondsAt, lessonAt), which gives both its blocks and its
// sweets, so the road and what lies on it always agree.

// laidOut reports whether t is laid out from the row where it begins (Game.designFrom).
func (t Theme) laidOut() bool { return t == ThemeSeconds || t == ThemeLesson }

// laidOutAt is row r of the laid-out theme t on the wide road from left to right.
func (g *Game) laidOutAt(t Theme, r, left, right int) designRow {
	if t == ThemeLesson {
		return g.lessonAt(r, left, right)
	}
	return g.secondsAt(r, left, right)
}

// designRow is one row of a laid-out theme: the blocks standing in the road and the sweets
// lying on it.
type designRow struct {
	blocks []int
	sweets []designSweet
}

// designSweet is a sweet (or an item) of a designRow and its column.
type designSweet struct {
	x    int
	kind int8
}

func (d *designRow) block(xs ...int)        { d.blocks = append(d.blocks, xs...) }
func (d *designRow) sweet(x int, kind int8) { d.sweets = append(d.sweets, designSweet{x, kind}) }
func (d *designRow) restCandy(p, mid int) { // a candy down the middle every fourth row
	if p%4 == 2 {
		d.sweet(mid, SweetCandy)
	}
}

// ThemeSeconds, second helpings: every secPeriod rows a wall down the middle of the wide road
// parts a safe lane (plain, with a candy now and then) from a side lane, the left one and
// then the right one. The side lane has three stretches, each a step harder than the one
// before and with more on it (secStretches): an open lane with candies, then a block that
// steps across the lane, one cell along every third row, with a macaron beside each, then
// walls across the lane that leave one cell open, stepping from its outer side to the middle
// wall, with a macaron in each and the prize in the last one (an extra life on the first
// trip of a course, a hammer on the later ones). Between the stretches the middle wall opens
// for secExit rows: a way back to the safe lane, or into the side lane from it. A tight
// course has the same trips on its faster road: a wall every other row, or a way out of two
// rows, left a single row to make each step in, too quick to be fair.
//
// Memorable: how far will she go for more? Fair: each way out and the stretch after it are
// on the screen before she has to choose (a stretch and a way out together are shorter than
// the road she sees ahead), staying in the safe lane always gets through, and the side
// lane, once in it, never asks her to cross the middle wall, with a candy before every block
// showing the way past it. A trip that the end of the course would cut short is left out:
// the road stays open there.
const (
	secPeriod = 40
	secFrom   = 4 // the open rows before the side lane begins (after the rows that settle the road)
	secSeg    = 8 // rows of a stretch of the side lane
	secExit   = 3 // rows a way out stays open
	secEnd    = secFrom + 3*secSeg + 2*secExit
)

// secStretch is what stands in a stretch of the side lane: on each of its rows, a block in
// one cell of the lane (from the outer cell, 0, to the one beside the middle wall, 2) or a
// wall across the lane but for that cell, and the cell her way past it goes through.
type secStretch struct {
	rows, cells, ways []int
	wall              bool
}

// secStretches are the three stretches of the side lane: an open lane, a block stepping
// across it, walls across it.
var secStretches = [...]secStretch{
	{},
	{rows: []int{1, 4, 7}, cells: []int{0, 1, 2}, ways: []int{1, 2, 1}},
	{rows: []int{1, 4, 7}, cells: []int{0, 1, 2}, ways: []int{0, 1, 2}, wall: true},
}

// tripPart is where a row lies on the trips of ThemeSeconds.
type tripPart struct {
	k    int  // the trip, from 0 (-1: none here, the road is open)
	p    int  // the row within the period of the trip
	seg  int  // the stretch of the side lane (1-3; 0 before it begins or after it ends)
	exit bool // the way out after stretch seg
	j    int  // the row within the stretch or the way out
}

// tripAt works out where row r of the course lies on the trips.
func (g *Game) tripAt(r int) tripPart {
	if g.designFrom < 0 || r < g.designFrom {
		return tripPart{k: -1, p: r}
	}
	q := r - g.designFrom
	k, p := q/secPeriod, q%secPeriod
	if g.designFrom+k*secPeriod+secEnd > g.lenOf(g.Level) {
		return tripPart{k: -1, p: p} // the course ends before this trip would
	}
	if p < secFrom || p >= secEnd {
		return tripPart{k: k, p: p}
	}
	j := p - secFrom
	for s := 1; ; s++ {
		if j < secSeg {
			return tripPart{k: k, p: p, seg: s, j: j}
		}
		if j -= secSeg; j < secExit {
			return tripPart{k: k, p: p, seg: s, exit: true, j: j}
		}
		j -= secExit
	}
}

// secondsAt is row r of ThemeSeconds on the wide road from left to right.
func (g *Game) secondsAt(r, left, right int) designRow {
	var d designRow
	t := g.tripAt(r)
	mid := (left + right) / 2
	if t.k < 0 {
		d.restCandy(t.p, mid)
		return d
	}
	outer, step := left, 1 // the side lane, from its outer cell to the one beside the middle wall
	if t.k%2 == 1 {
		outer, step = right, -1
	}
	lane := func(i int) int { return outer + i*step }
	safe := 2*mid - lane(1) // the middle of the safe lane
	if t.seg == 0 {
		if t.p == secFrom-3 || t.p == secFrom-1 { // two candies lead to the side lane
			d.sweet(lane(1), SweetCandy)
		}
		return d
	}
	if (t.p-secFrom)%4 == 1 {
		d.sweet(safe, SweetCandy)
	}
	if t.exit {
		return d
	}
	d.block(mid)
	st := secStretches[t.seg-1]
	if len(st.rows) == 0 {
		if t.j%2 == 0 {
			d.sweet(lane(1), SweetCandy)
		}
		return d
	}
	for i, j := range st.rows {
		switch t.j {
		case j - 1: // a candy just before each block shows the way past it
			d.sweet(lane(st.ways[i]), SweetCandy)
		case j:
			for c := range 3 {
				if (c == st.cells[i]) != st.wall {
					d.block(lane(c))
				}
			}
			prize := SweetMacaron // and the prize at the very end
			if t.seg == 3 && i == len(st.rows)-1 {
				prize = SweetBomb
				if t.k == 0 {
					prize = SweetOneUp
				}
			}
			d.sweet(lane(st.ways[i]), prize)
		}
	}
	return d
}

// ThemeLesson, the lesson: three moves, each shown alone with room around it, then put
// together into one phrase, which comes again the other way round.
//
// The moves: a bend (the road narrows to three cells and swings two cells to the left and
// back, a step every other row), a gate (a wall across the road with a gap three cells wide
// at the left, then one at the right) and a rock (three blocks in the middle of the road, to
// pass on either side, a macaron on the right). Then a phrase of the three: a bend to one
// side, two rows of open road, a gate a little to the other side, three rows, the rock in the
// middle again (a macaron past it by the wall on the gate's side), two rows; and after three
// more rows the same phrase mirrored, the bend to the other side. The gate of the phrase is
// two cells wide when hard. Then the phrase again, as long as the course has room for a
// whole one. A line of sweets runs through each phrase.
//
// Memorable: learn the moves, then dance them. Fair: every move of the phrase is one she has
// just been shown, she has two rows at least to get from one move to the next, nothing asks
// for more than a cell a row, no sweet lures her into a spot she cannot get out of in time,
// and a phrase the end of the course would cut short is left out.
//
// The rows of the lesson, counted from where it begins: the bend from row 0, a gate at the
// left, one at the right five rows on (from the left side of the road to the right is a long
// way on a fast road), a rock of two rows, and room before the phrases. The rows of a phrase:
// the bend from row 0, the gate, a rock of two rows and two rows of room, and three more
// before a phrase that follows it.
const (
	lessonGateA  = 12
	lessonGateB  = 18
	lessonRock   = 21
	lessonLearn  = 25
	phraseGate   = 7
	phraseRock   = 11
	phraseLen    = 15
	lessonPhrase = phraseLen + 3
)

// lessonBend is the bend shown first, and phraseBend the bend of a phrase: how far from the
// middle the narrow road is, row by row.
var (
	lessonBend = [...]int{0, 0, 1, 1, 2, 2, 1, 1, 0, 0}
	phraseBend = [...]int{0, 1, 1, 2, 2}
)

// lessonPhrases is how many phrases the lesson of a course of n rows has, when it begins on
// row from of the course (designFrom).
func lessonPhrases(n, from int) int {
	if room := n - from - lessonLearn - phraseLen; room >= 0 {
		return room/lessonPhrase + 1
	}
	return 0
}

// lessonAt is row r of ThemeLesson on the wide road from left to right.
func (g *Game) lessonAt(r, left, right int) designRow {
	var d designRow
	if g.designFrom < 0 || r < g.designFrom {
		return d
	}
	q, mid := r-g.designFrom, (left+right)/2
	narrow := func(c int) { // the road narrowed to the three cells around c
		for x := left; x <= right; x++ {
			if x < c-1 || x > c+1 {
				d.block(x)
			}
		}
	}
	gate := func(lo, hi int) { // a wall across the road but for the cells lo..hi
		for x := left; x <= right; x++ {
			if x < lo || x > hi {
				d.block(x)
			}
		}
		d.sweet((lo+hi)/2, SweetMacaron)
	}
	rock := func(c int) { d.block(c-1, c, c+1) }
	if q < lessonLearn {
		switch {
		case q < len(lessonBend):
			c := mid - lessonBend[q]
			narrow(c)
			switch {
			case q == 4:
				d.sweet(c, SweetMacaron) // the far end of the bend
			case q%2 == 0:
				d.sweet(c, SweetCandy)
			}
		case q == lessonGateA:
			gate(left, left+2)
		case q == lessonGateB:
			gate(right-2, right)
		case q == lessonRock, q == lessonRock+1:
			rock(mid)
			if q == lessonRock {
				d.sweet(right-1, SweetMacaron)
			}
		case q == lessonGateA-2: // candies on the way from one move to the next
			d.sweet(mid-1, SweetCandy)
		case q == lessonGateB-3:
			d.sweet(mid, SweetCandy)
		case q == lessonRock-2:
			d.sweet(right-1, SweetCandy)
		case q == lessonLearn-2:
			d.sweet(mid+1, SweetCandy)
		}
		return d
	}
	m, j := (q-lessonLearn)/lessonPhrase, (q-lessonLearn)%lessonPhrase
	if m >= lessonPhrases(g.lenOf(g.Level), g.designFrom) {
		d.restCandy(j, mid)
		return d
	}
	dir := -1 // the way the bend goes: to the left, and to the right every other phrase
	if m%2 == 1 {
		dir = 1
	}
	gap := 3
	if g.hard() {
		gap = 2
	}
	switch {
	case j < len(phraseBend):
		c := mid + dir*phraseBend[j]
		narrow(c)
		if j%2 == 0 {
			d.sweet(c, SweetCandy)
		}
	case j == phraseGate: // the gate on the other side: its gap from the middle on
		a, b := mid, mid-dir*(gap-1)
		gate(min(a, b), max(a, b))
	case j == phraseRock, j == phraseRock+1: // the rock, a macaron past it by the wall on the gate's side
		rock(mid)
		if j == phraseRock {
			d.sweet(mid-3*dir, SweetMacaron)
		}
	case j == phraseGate-1: // candies on the way from one move to the next
		d.sweet(mid, SweetCandy)
	case j == phraseGate+2:
		d.sweet(mid-2*dir, SweetCandy)
	case j == phraseLen-1:
		d.sweet(mid-2*dir, SweetCandy)
	}
	return d
}
