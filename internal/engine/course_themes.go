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
)

// courseThemes is the theme of each of a character's 16 courses, on the regular side and
// on the extra stages. Each character's roads have a character of their own: most of her
// courses come from a few themes that suit her, with a few others between them.
//
//   - cool: roads that test the aim (gates, stepping gates, slaloms, pillars), with swings
//     and scattered blocks between them; the corridor only on the extra stages: six
//     courses of gates a side and a corridor made her roads plainly the hardest
//   - cute: soft curves (snakes, swings, wobbles, funnels, diamonds)
//   - gyal: busy, restless roads (scattered blocks, chicanes, pillars, stairs, slaloms; one
//     checkers course a side: three of them were too much)
//   - street: regular patterns, like a kimono's (combs, lanes, splits, hourglasses, stairs)
//   - bunny: the hardest of every kind
//
// A run opens with the warm-up and ends on the mixed road; the courses get tight from
// road.Game.HardFrom on (every course on the extra stages, see NewRun).
var courseThemes = map[string][2][GameCourses]road.Theme{
	coolID: {
		{tWarm, tGate, tSlal, tPill, tSwing, tHour, tStep, tSwing, tSlal, tSnake, tPill, tRain, tCheck, tEdge, tGate, tMix},
		{tWarm, tSlal, tStep, tCorr, tGate, tEdge, tDiam, tSwing, tPill, tGate, tSwing, tChic, tStep, tRain, tPill, tMix},
	},
	"cute": {
		{tWarm, tSwing, tSnake, tDiam, tWobb, tFunn, tPill, tSwing, tSnake, tDiam, tWobb, tLane, tFunn, tSnake, tSwing, tMix},
		{tWarm, tSnake, tWobb, tFunn, tSwing, tDiam, tHour, tSnake, tWobb, tFunn, tSwing, tChic, tDiam, tSnake, tWobb, tMix},
	},
	"gyal": {
		{tWarm, tPill, tRain, tCheck, tSwing, tChic, tRain, tStair, tSlal, tPill, tChic, tSlal, tRain, tStair, tPill, tMix},
		{tWarm, tRain, tCheck, tPill, tChic, tStair, tEdge, tRain, tSlal, tChic, tStair, tStep, tRain, tPill, tChic, tMix},
	},
	"street": {
		{tWarm, tLane, tSplit, tComb, tHour, tStair, tGate, tLane, tComb, tSplit, tHour, tFunn, tComb, tStair, tHour, tMix},
		{tWarm, tSplit, tComb, tLane, tHour, tStair, tCorr, tComb, tSplit, tHour, tLane, tStep, tComb, tStair, tHour, tMix},
	},
	"bunny": {
		{tWarm, tHour, tChic, tCheck, tCorr, tStep, tEdge, tChic, tCheck, tHour, tCorr, tStep, tEdge, tChic, tCheck, tMix},
		{tWarm, tCheck, tCorr, tEdge, tStep, tChic, tHour, tCorr, tEdge, tStep, tChic, tCheck, tHour, tEdge, tCorr, tMix},
	},
}

// charSpeed is how much faster than the others each character's road runs: the restless
// gyaru and the secret bunny a little faster, the soft cute road at the usual speed.
var charSpeed = map[string]float64{coolID: 1.02, "cute": 1, "gyal": 1.06, "street": 1, "bunny": 1.08}

// coolID is the cool girl's character id.
const coolID = "cool"

// charHammers is how many hammers a character starts her run with when it is not
// road.StartBombs. The cool girl's roads (gates, stepping gates, slaloms and a checkerboard,
// all testing the aim) are the hardest of the four regular characters, so she starts with
// one hammer more, on both sides: a spare for the first stages instead of a pickup placed
// on her road, which would have changed the roads every game has learned.
var charHammers = map[string]int{coolID: road.StartBombs + 1}

// frontHardFrom is the course from which the regular side is as tight as the extra stages
// (the second half of a run). The extra stages are tight from the first course, and faster.
const frontHardFrom = 7

// HasOwnRun reports whether the character id has course themes and a road speed of her
// own; any other character runs the mixed road at the usual speed.
func HasOwnRun(id string) bool {
	_, themes := courseThemes[id]
	_, speed := charSpeed[id]
	return themes && speed
}

// NewRun starts a character's run: her roads, her themes and her speed, on the regular
// side or on the extra stages.
func NewRun(id string, extra bool) *Engine {
	p := roadProfile
	if extra {
		p = extraProfile
	}
	if f, ok := charSpeed[id]; ok {
		p.Speed *= f
	}
	e := newEngineWith(roadSeedFor(id), GameCourses, p)
	if n, ok := charHammers[id]; ok {
		e.G.Bombs = min(n, road.MaxBombs)
	}
	e.G.Themes = themesFor(id, extra)
	e.G.Hard = extra
	if !extra {
		e.G.HardFrom = frontHardFrom
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
