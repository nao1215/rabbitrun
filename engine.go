package main

import (
	"math"
	"strconv"

	"github.com/nao1215/rabbitrun/road"
)

// The road. The rules (the wandering road, sweets, crashes, lives) live in package
// road; the Engine adds the timing: how fast the road scrolls.
const (
	BoardW      = road.W
	VisibleRows = road.Rows
	MaxLevel    = road.MaxLevel
)

type Engine struct {
	G *road.Game

	// Events of the frames since the scene last read them.
	Events []road.Event
	// PlayFrames counts the frames played (Tick calls while not paused).
	PlayFrames int
	stepAcc    float64
	Steps      int // rows scrolled (for the scroll animation)
	// Boost is how much faster the player has made the road (1: not at all). Holding up
	// raises it; it does not come down within the stage, and goes back to 1 when the next
	// stage starts or the player retries.
	Boost float64
}

// The player's own speed-up: holding up raises Boost by boostRate a second, to boostMax
// (30% faster at most).
const (
	boostRate = 0.5
	boostMax  = 1.3
)

// RowsPerSec is how fast the road runs now, in rows a second: the speed of the course
// times the road's profile and the player's Boost.
func (e *Engine) RowsPerSec() float64 {
	return RowsPerSecond(e.G.Level) * e.G.Profile.Speed * math.Max(1, e.Boost)
}

// newEngineWith starts a game of courses courses on the road p (newRun builds a
// character's run on it).
func newEngineWith(seed uint64, courses int, p road.Profile) *Engine {
	g := road.NewWith(seed, p)
	g.TotalCourses = courses
	g.CourseRowsFor = func(level int) int {
		return int(math.Round(RowsPerSecond(level) * p.Speed * CourseSeconds))
	}
	return &Engine{G: g}
}

// CourseSeconds is how long a course lasts: the game (16 courses) is over in about three
// minutes.
const CourseSeconds = 11

// roadProfile is the road of every character: narrow, winding (zigzags and wandering
// stretches take turns course by course), with pillars to slip past and gates to aim
// through. The characters all play the same road.
var roadProfile = road.Profile{Speed: 1.15, MaxWidth: 5, Narrowing: 2, Wander: 0.16, Mixed: true,
	Pillars: 0.07, Gates: 0.05, SweetsRate: 0.16} // no extra lives lying on the road: they come from sweets and vaults

// extraProfile is the road of the extra stages (the hidden command): the same road, faster
// and with more pillars and gates.
var extraProfile = road.Profile{Speed: 1.32, MaxWidth: 5, Narrowing: 2, Wander: 0.2, Mixed: true,
	Pillars: 0.1, Gates: 0.08, SweetsRate: 0.16}

func (e *Engine) Level() int { return e.G.Level }
func (e *Engine) Over() bool { return e.G.Over }

// GiveUp ends the game after a miss.
func (e *Engine) GiveUp() {
	e.G.GiveUp()
	e.collect()
}

// Restart uses a life after a miss and starts the stage over.
func (e *Engine) Restart() bool {
	ok := e.G.Restart()
	if ok {
		e.stepAcc = 0 // the road starts again with its last row just come in
	}
	e.collect()
	return ok
}

// RowsPerSecond is the scroll speed at a level (one level per course): calm at first,
// and faster by 4.7% a course (twice as fast on the last of 16 courses).
func RowsPerSecond(level int) float64 {
	return math.Min(12, 3.2*math.Pow(1.047, float64(level-1)))
}

// PlayerSpeed is how fast the player slides sideways, in cells per second: 8, and quicker
// on a fast course so she can still follow it. The player's own speed-up (Boost) leaves it
// as it is: sliding stays easy to control.
func (e *Engine) PlayerSpeed() float64 {
	return math.Max(7, 1.15*RowsPerSecond(e.G.Level)*e.G.Profile.Speed) // the speed-up (Boost) does not change it
}

// Sideways slides start slow, so a short tap nudges her a little, and speed up while the
// key is held: from slideStart cells a second to PlayerSpeed over slideRampFrames.
const (
	slideStart      = 3.5
	slideRampFrames = 12
)

// SlideSpeed is how fast she slides (cells a second) after a direction has been held for
// held frames (1 on the first frame).
func (e *Engine) SlideSpeed(held int) float64 {
	top := e.PlayerSpeed()
	t := math.Min(1, float64(held-1)/slideRampFrames)
	return slideStart + (top-slideStart)*t
}

// Move slides the player by dx cells.
func (e *Engine) Move(dx float64) {
	e.G.Move(dx)
	e.collect()
}

// UseBomb sets off a bomb from the stock (every wall on the screen is gone).
func (e *Engine) UseBomb() bool {
	ok := e.G.UseBomb()
	e.collect()
	return ok
}

// Progress is how far the run got, as "stage-course" (for example 2-5).
func (e *Engine) Progress() string {
	return strconv.Itoa(e.G.Stage) + "-" + strconv.Itoa(e.G.Course+1)
}

// RestartProgress is the course a retry would start on, as "stage-course": the place
// road.RewindRows back, in this course or the one before.
func (e *Engine) RestartProgress() string {
	lv := e.G.RewindLevel()
	return strconv.Itoa((lv-1)/road.Courses+1) + "-" + strconv.Itoa((lv-1)%road.Courses+1)
}

// Tick advances one frame: the road scrolls by RowsPerSec/60 rows. Holding up (accel)
// speeds the road up for good (Boost).
func (e *Engine) Tick(accel bool) {
	if e.G.Over {
		return
	}
	e.PlayFrames++
	if accel {
		e.Boost = math.Min(boostMax, math.Max(1, e.Boost)+boostRate/60)
	}
	before := e.stepAcc
	e.stepAcc += e.RowsPerSec() / 60
	for e.stepAcc >= 1 && !e.G.Over {
		e.stepAcc--
		before = 0
		e.G.Step()
		e.Steps++
	}
	if before < 0.5 && e.stepAcc >= 0.5 {
		e.G.ReachHalfway() // the row is now beside her body (see road.Game.sideRow)
	}
	e.collect()
}

// Scroll is how far (0-1 of a row) the road has moved toward the next row, for drawing
// the road moving smoothly.
func (e *Engine) Scroll() float64 { return e.stepAcc }

// collect moves the events of the rule engine to the Engine.
func (e *Engine) collect() {
	for _, ev := range e.G.Events {
		switch ev.Kind {
		case road.EventStageClear, road.EventRestart:
			e.Boost = 1 // the speed-up holds for one stage; a new stage or a retry starts at its own speed
		default:
		}
		e.Events = append(e.Events, ev)
	}
	e.G.Events = e.G.Events[:0]
}

// Danger reads how tight things are: 0 when the road ahead is wide, up to 3 when it
// is at its narrowest, plus one when no life is left (a miss now ends the game).
func (e *Engine) Danger() int {
	d := 0
	switch w := e.G.RoadWidthAhead(6); {
	case w <= 3:
		d = 3
	case w <= 4:
		d = 2
	case w <= 5:
		d = 1
	}
	if e.G.Lives == 0 {
		d++
	}
	return d
}
