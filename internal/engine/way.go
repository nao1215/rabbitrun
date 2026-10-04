package engine

import (
	"math"

	"github.com/nao1215/rabbitrun/road"
)

// A search for a way through the road, frame by frame, with the real rules of moving and
// of touching walls. It is the careful auto player's planner, and the tests run it over
// every run to show that each can be passed.
//
// In a frame the play scene first slides her (road.Game.Move, judged against the side row,
// road.Game.sideRow) and then ticks the road (Engine.Tick), which may step it a row and may
// bring the row that came in halfway down onto her (road.Game.ReachHalfway: a miss if she
// stands in one of its walls). Stepping into a row with a wall is not a miss by itself. So
// a frame of the road is told by two rows of walls: the one her slide that frame is judged
// against, and the one that reaches halfway at the end of the frame, if any. The road is
// built the same way wherever she is, so the frames of a run at a speed are known in
// advance.

// frameWalls is what one frame of the road does to her.
type frameWalls struct {
	side    uint16  // walls (bit x for column x) of the row a slide in the frame is judged against
	halfway bool    // a row reaches halfway down onto her at the end of the frame
	half    uint16  // the walls of that row
	top     float64 // her top sliding speed in the frame (Engine.PlayerSpeed)
}

// wallBits is the walls of a row, bit x for column x.
func wallBits(r road.Row) uint16 {
	var m uint16
	for x, c := range r {
		if c.Wall != 0 {
			m |= 1 << x
		}
	}
	return m
}

// touches mirrors road.Game.blockedIn: whether a player at x touches a wall of walls.
func touches(walls uint16, x float64) bool {
	lo, hi := max(0, int(x-road.Half)), min(road.W-1, int(x+road.Half-1e-9))
	for c := lo; c <= hi; c++ {
		if walls&(1<<c) != 0 {
			return true
		}
	}
	return false
}

// slideOver is road.Game.Move on a row of walls: where a slide of dx from x ends (she
// stops at the edges of the screen), and false when she touches a wall on the way. Move
// slides in small steps and stops at the first that touches a wall; a wall with her on
// either side of it is wider (1+2*road.Half) than a frame's slide, so a slide that ends
// clear of the walls went past none, and one that ends in a wall touched it. (Move's small
// steps add up to the same place but for the last bits of the float.)
func slideOver(walls uint16, x, dx float64) (float64, bool) {
	next := x + dx
	if next < road.Half {
		next = road.Half
	} else if next > road.W-road.Half {
		next = road.W - road.Half
	}
	return next, !touches(walls, next)
}

// heldMax is the frames of holding a direction after which the slide is at its top speed:
// holding longer changes nothing, so the search counts no further.
const heldMax = slideRampFrames + 1

// wayStart is where a search starts: her place and the direction held (-1, 0 or 1) and for
// how many frames, as the play scene counts them.
type wayStart struct {
	x         float64
	dir, held int
}

// wayNode is a state the search reached in a frame: her place, the direction held and for
// how long (up to heldMax), and the state of the frame before it came from.
type wayNode struct {
	x                 float64
	dir, held, parent int
}

// wayLink is what the search keeps of a state of a frame it has gone past: the state it
// came from and the direction held to get there (enough to give the inputs back).
type wayLink struct{ parent, dir int }

// findWay searches for the directions to hold, frame by frame, that take her from start
// through the frames without touching a wall. It reports the inputs of the way that got
// furthest and how many frames it got through (len(frames) for a way through them all).
//
// Each frame it keeps, for each place (to 1/bins of a cell) and direction, the state that
// has held the direction longest. Every state it keeps is really reached by the inputs
// that lead to it, so a way it reports is a real way; it can only miss one by dropping a
// state that shared a place with another.
func findWay(frames []frameWalls, start wayStart, bins int) ([]int, int) {
	cur := []wayNode{{x: start.x, dir: start.dir, held: min(start.held, heldMax), parent: -1}}
	links := make([][]wayLink, 0, len(frames))
	slots := make([]int, (road.W*bins+1)*3)
	stamp := make([]int, len(slots))
	var speed [heldMax + 1]float64
	lastTop := math.NaN()
	next := make([]wayNode, 0, 256)
	for f, fw := range frames {
		if fw.top != lastTop {
			lastTop = fw.top
			for h := 1; h <= heldMax; h++ {
				speed[h] = slideSpeedAt(fw.top, h)
			}
		}
		next = next[:0]
		for pi, s := range cur {
			for nd := -1; nd <= 1; nd++ {
				nh := 0
				x, ok := s.x, true
				if nd != 0 {
					nh = 1
					if nd == s.dir {
						nh = min(heldMax, s.held+1)
					}
					x, ok = slideOver(fw.side, s.x, float64(nd)*speed[nh]/60)
				}
				if !ok || (fw.halfway && touches(fw.half, x)) {
					continue
				}
				key := int(x*float64(bins))*3 + nd + 1
				n := wayNode{x, nd, nh, pi}
				if stamp[key] == f+1 {
					if j := slots[key]; next[j].held < nh {
						next[j] = n
					}
					continue
				}
				stamp[key], slots[key] = f+1, len(next)
				next = append(next, n)
			}
		}
		if len(next) == 0 {
			return wayBack(links, cur, 0), f
		}
		keep := make([]wayLink, len(cur))
		for i, s := range cur {
			keep[i] = wayLink{s.parent, s.dir}
		}
		links = append(links, keep)
		cur, next = next, cur
	}
	// of the ways through them all, the one that ends with the most room around her
	best := 0
	if n := len(frames); n > 0 {
		for i, s := range cur {
			if room(frames[n-1].side, s.x) > room(frames[n-1].side, cur[best].x) {
				best = i
			}
		}
	}
	return wayBack(links, cur, best), len(frames)
}

// room is how far a player at x can slide either way along a row of walls before she
// touches one (or the edge of the road): the less of the two.
func room(walls uint16, x float64) float64 {
	c := min(road.W-1, max(0, int(x)))
	left, right := road.Half, road.W-road.Half
	for i := c; i >= 0; i-- {
		if walls&(1<<i) != 0 {
			left = float64(i+1) + road.Half
			break
		}
	}
	for i := c; i < road.W; i++ {
		if walls&(1<<i) != 0 {
			right = float64(i) - road.Half
			break
		}
	}
	return min(x-left, right-x)
}

// wayBack gives back the inputs that lead to the state i of last, the states of the frame
// after the ones links keeps.
func wayBack(links [][]wayLink, last []wayNode, i int) []int {
	inputs := make([]int, len(links))
	if len(links) == 0 {
		return inputs
	}
	j := last[i].parent
	inputs[len(links)-1] = last[i].dir
	for f := len(links) - 1; f > 0; f-- {
		inputs[f-1] = links[f][j].dir
		j = links[f][j].parent
	}
	return inputs
}
