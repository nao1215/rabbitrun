package main

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
//   - cool: roads that test the aim (gates, stepping gates, corridors, slaloms, edge runs)
//   - cute: soft curves (snakes, swings, wobbles, funnels, diamonds)
//   - gyal: busy, restless roads (scattered blocks, checkers, chicanes, pillars, stairs)
//   - street: regular patterns, like a kimono's (combs, lanes, splits, hourglasses, stairs)
//   - bunny: the hardest of every kind
//
// A run opens with the warm-up and ends on the mixed road; the courses get tight from
// road.Game.HardFrom on (every course on the extra stages, see newRun).
var courseThemes = map[string][2][GameCourses]road.Theme{
	"cool": {
		{tWarm, tGate, tSlal, tPill, tGate, tCorr, tStep, tEdge, tSlal, tSnake, tStep, tCorr, tCheck, tEdge, tStep, tMix},
		{tWarm, tSlal, tStep, tCorr, tGate, tEdge, tDiam, tStep, tCorr, tGate, tEdge, tChic, tStep, tCorr, tEdge, tMix},
	},
	"cute": {
		{tWarm, tSwing, tSnake, tDiam, tWobb, tFunn, tPill, tSwing, tSnake, tDiam, tWobb, tLane, tFunn, tSnake, tSwing, tMix},
		{tWarm, tSnake, tWobb, tFunn, tSwing, tDiam, tHour, tSnake, tWobb, tFunn, tSwing, tChic, tDiam, tSnake, tWobb, tMix},
	},
	heroID: {
		{tWarm, tPill, tRain, tCheck, tSwing, tChic, tRain, tStair, tCheck, tPill, tChic, tSlal, tRain, tStair, tCheck, tMix},
		{tWarm, tRain, tCheck, tPill, tChic, tStair, tEdge, tRain, tCheck, tChic, tStair, tStep, tRain, tCheck, tChic, tMix},
	},
	streetID: {
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
var charSpeed = map[string]float64{"cool": 1.04, "cute": 1, heroID: 1.06, streetID: 1, "bunny": 1.08}

// streetID is the Taisho romance character.
const streetID = "street"

// frontHardFrom is the course from which the regular side is as tight as the extra stages
// (the second half of a run). The extra stages are tight from the first course, and faster.
const frontHardFrom = 7

// newRun starts a character's run: her roads, her themes and her speed, on the regular
// side or on the extra stages.
func newRun(id string, extra bool) *Engine {
	p := roadProfile
	if extra {
		p = extraProfile
	}
	if f, ok := charSpeed[id]; ok {
		p.Speed *= f
	}
	e := newEngineWith(roadSeedFor(id), GameCourses, p)
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
