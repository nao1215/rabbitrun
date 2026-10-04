package engine

import "github.com/nao1215/rabbitrun/road"

// Short names for the course themes, to keep the tables below readable.
const (
	tWarm  = road.ThemeWarmUp
	tSnake = road.ThemeSnake
	tSwing = road.ThemeSwing
	tSlal  = road.ThemeSlalom
	tPill  = road.ThemePillars
	tGate  = road.ThemeGates
	tStair = road.ThemeStairs
	tFunn  = road.ThemeFunnel
	tSplit = road.ThemeSplit
	tRain  = road.ThemeRain
	tChic  = road.ThemeChicane
	tCheck = road.ThemeCheckers
	tCorr  = road.ThemeCorridor
	tHour  = road.ThemeHourglass
	tComb  = road.ThemeComb
	tStep  = road.ThemeStepGates
	tDiam  = road.ThemeDiamonds
	tEdge  = road.ThemeEdgeRun
	tLane  = road.ThemeLanes
	tWobb  = road.ThemeWobble
	tMix   = road.ThemeMixed
	tFork  = road.ThemeFork
	tTrail = road.ThemeTrail
	tRoom  = road.ThemeRooms
	tJar   = road.ThemeJar
	tHall  = road.ThemeHammerHall
	tAlc   = road.ThemeAlcoves
	tDoor  = road.ThemeDoors
	tWave  = road.ThemeWave
	tSec   = road.ThemeSeconds
	tLess  = road.ThemeLesson
)

// courseThemes is the theme of each of a character's 16 courses, on the regular side and
// on the extra stages. Each character's roads have a character of their own: most of her
// courses come from a few themes that suit her, with a few others between them.
//
// The runs get harder from left to right in the order of the character select screen
// (the cool girl's are the easiest, the bunny girl's the hardest), the extra side of each
// is harder than her regular side course by course, and each run gets harder stage by
// stage. Every character's road runs at the same speed (a road of her own that ran faster
// or slower made the controls feel different from one character to the next), so that
// order comes from these tables alone. The tests in difficulty_test.go score every course
// on its road and hold the tables to that; change a table and they tell you what moved.
//
//   - cool: clear, readable roads (lanes, the fork, the trail, rooms, doors, diamonds,
//     gates and pillars), the lesson on the extra side, the checkerboard only on stage 4 of
//     the regular side
//   - cute: soft curves (the trail, swings, wobbles, funnels, snakes) with the jar and the
//     alcoves for sweets
//   - gyal: busy, restless roads (scattered blocks, chicanes, pillars, slaloms, stairs), the
//     jar, the alcoves, second helpings and the hammer hall; one checkers course a side
//   - street: regular patterns, like a kimono's (combs, lanes, splits, hourglasses, stairs,
//     stepping gates), with doors, the wave, rooms and the lesson (a pattern learned, then
//     mirrored)
//   - bunny: the hardest of every kind (corridors, stepping gates, chicanes, snakes, stairs),
//     with the fork, second helpings, the wave and the hammer hall, and the most checkers
//
// No theme comes twice in a row. A run opens with the warm-up and ends on the mixed road;
// the courses get tight from road.Game.HardFrom on (every course on the extra stages, see
// NewRun).
var courseThemes = map[string][2][GameCourses]road.Theme{
	coolID: {
		{tWarm, tFork, tLane, tTrail, tSwing, tTrail, tPill, tDiam, tGate, tRoom, tStep, tLane, tCheck, tFork, tSwing, tMix},
		{tWarm, tFork, tGate, tRoom, tDiam, tGate, tPill, tWave, tSwing, tLess, tStep, tDoor, tTrail, tStep, tDiam, tMix},
	},
	"cute": {
		{tWarm, tAlc, tDiam, tTrail, tFunn, tSnake, tJar, tSwing, tHour, tSwing, tHour, tTrail, tWobb, tAlc, tJar, tMix},
		{tWarm, tWobb, tAlc, tTrail, tJar, tSnake, tFunn, tSwing, tWobb, tSnake, tSwing, tJar, tTrail, tHour, tDiam, tMix},
	},
	"gyal": {
		{tWarm, tJar, tRain, tCheck, tChic, tSec, tSwing, tRain, tAlc, tChic, tPill, tStair, tAlc, tPill, tHall, tMix},
		{tWarm, tAlc, tCheck, tHall, tPill, tSwing, tSlal, tRain, tHall, tChic, tStair, tJar, tAlc, tSlal, tPill, tMix},
	},
	"street": {
		{tWarm, tRoom, tLane, tHour, tLane, tSplit, tDoor, tComb, tDoor, tHour, tWave, tComb, tLess, tCorr, tWave, tMix},
		{tWarm, tCorr, tStep, tStair, tRoom, tDoor, tStair, tStep, tLess, tWave, tCorr, tComb, tStair, tComb, tWave, tMix},
	},
	"bunny": {
		{tWarm, tHall, tSnake, tCheck, tCorr, tStep, tSnake, tSec, tCheck, tCorr, tFork, tChic, tStair, tWave, tCheck, tMix},
		{tWarm, tCheck, tChic, tHall, tWave, tStep, tStair, tFork, tSnake, tCorr, tSnake, tCheck, tGate, tEdge, tGate, tMix},
	},
}

// coolID is the cool girl's character id.
const coolID = "cool"

// extraHammerRow is the row of the first stage on which a character finds one more
// hammer on her road. The cool girl, on the left of the select screen, has the easiest
// roads and is the one to start with, so a spare hammer lies on her way early, just after
// the opening trail of sweets: a first look at the hammer. It is placed on the road rather
// than given at the start (the user's choice).
var extraHammerRow = map[string]int{coolID: 40}

// idleUntil is the last course of the regular side on which a road that would let her
// stand still for more than half a screen gets a block in her way (road.Game.IdleUntil):
// the first stage, the easy roads where that happened most (the user found them too easy,
// going most of the screen without a key). On the stage after it the regular courses would
// come out harder than the same courses of the extra stages.
const idleUntil = 4

// frontHardFrom is the course from which the regular side is as tight as the extra stages
// (the second half of a run). The extra stages are tight from the first course, and faster.
const frontHardFrom = 7

// HasOwnRun reports whether the character id has course themes of her own; any other
// character runs the mixed road. Every character's road runs at the same speed (the
// regular side's or the extra side's), so the controls feel the same whoever runs.
func HasOwnRun(id string) bool {
	_, ok := courseThemes[id]
	return ok
}

// NewRun starts a character's run: her roads and her themes, on the regular side or on
// the extra stages (the speed is the side's, the same for every character).
func NewRun(id string, extra bool) *Engine {
	p := roadProfile
	if extra {
		p = extraProfile
	}
	e := newEngineWith(roadSeedFor(id), GameCourses, p)
	e.G.ExtraHammerRow = extraHammerRow[id]
	e.G.Themes = themesFor(id, extra)
	e.G.Hard = extra
	if !extra {
		e.G.HardFrom = frontHardFrom
		e.G.IdleUntil = idleUntil
	}
	return e
}

// themesFor is the run of course themes of a character (nil, every course mixed, for one
// without a table).
func themesFor(id string, extra bool) []road.Theme {
	t, ok := courseThemes[id]
	if !ok {
		return nil
	}
	side := 0
	if extra {
		side = 1
	}
	out := t[side]
	return out[:]
}

// roadSeed builds the roads: the same seed every game, so every course is the same road
// each time (it can be learned, and a retry runs the same road again).
const roadSeed = 20261002

// roadSeedFor is the seed of a character's roads: each character runs roads of her own
// (as hard as the others, shaped differently), the same every time.
func roadSeedFor(id string) uint64 {
	h := uint64(14695981039346656037) // FNV-1a
	for _, b := range []byte(id) {
		h = (h ^ uint64(b)) * 1099511628211
	}
	return roadSeed ^ h
}

// GameCourses is how many courses the game has: four stages of four (about three
// minutes; longer games dragged). The illustrations are shared out over the first
// GameCourses-1 courses, one a course; the last course leads to the ending.
const GameCourses = 16
