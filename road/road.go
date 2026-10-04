// Package road is the rule engine of the candy road: a road walled with gummies
// scrolls down the screen, the player (at the bottom) only moves left and right to
// stay on it, and picks up the sweets lying on the road.
//
// Time is counted in steps: one step scrolls the road by one row. The game decides
// how many steps a second there are (the speed).
//
// The hammer item, which clears every wall on the screen, is called a bomb in the names
// of this package (SweetBomb, Bombs, UseBomb and so on), from an earlier design.
package road

import (
	"math"
	"math/rand/v2"
)

// Size of the visible road and the row the player is on (0 is the top row).
const (
	W         = 9
	Rows      = 16
	PlayerRow = Rows - 3
)

// Cell is one square of the road. Wall is the color of the gummy block that walls the
// road (one of the Course colors, 0 for none); Sweet is the kind of sweet lying
// there (0 for none).
type Cell struct {
	Wall  int8
	Sweet int8
}

// Row is one row of the road.
type Row [W]Cell

// Sweet kinds.
const (
	SweetNone int8 = iota
	SweetCandy
	SweetMacaron
	SweetOneUp // an extra life (very rare)
	SweetBomb  // a hammer for the stock (rare)
)

// Event is something that happened in a step.
type Event struct {
	Kind   EventKind
	Sweet  int8 // the sweet picked up (EventPick)
	Streak int  // sweets picked up in a row without letting one pass (EventPick)
}

// EventKind tells what happened.
type EventKind int

const (
	EventPick       EventKind = iota // picked up a sweet
	EventMiss                        // a sweet scrolled past the player
	EventCrash                       // ran into a wall: one life lost
	EventNearMiss                    // a wall passed right next to the player
	EventCourse                      // a new course (wall color) began: the next level, faster
	EventStageClear                  // all the courses of a stage are done
	EventAllClear                    // the last course of the game is done
	EventOneUp                       // picked up an extra life
	EventRestart                     // a life was used: the stage starts over
	EventBombGain                    // a hammer was added to the stock
	EventBomb                        // a hammer was swung: every wall on the screen is gone
	EventOver
)

// Game is the road and the player.
type Game struct {
	Rows [Rows]Row // Rows[0] is the top
	X    float64   // middle of the player, in cells from the left edge (0.5 is the middle of column 0)

	Bombs    int // hammers in stock: a hammer clears every wall on the screen
	Sweets   int // sweets picked up toward the next extra life (SweetsPerLife)
	Lives    int
	Level    int // courses reached so far (1 for the first course): sets the speed
	Stage    int // 1 for the first stage
	Course   int // 0-6 within the stage
	Ahead    Row // the row that comes in at the top next (drawn above the visible road)
	Distance int // rows travelled
	Over     bool
	Safe     int // steps left in which walls pass through the player (for tests and demos)
	Streak   int // sweets picked up in a row
	Events   []Event

	rng     *rand.Rand
	seed    uint64  // the road of each course is built from this and the course number
	vaultAt int     // the row of the course where its vault starts (-1: none)
	feastAt int     // the row of the course where its feast starts (-1: none)
	reach   [W]bool // the cells of the last row built that she can be on (themed courses keep to it)
	// Themes gives each course (by Level-1) its theme; without it every course is
	// ThemeMixed. Hard makes the themes tighter (the extra stages).
	Themes []Theme
	Hard   bool
	// HardFrom is the course (Level) from which the themes are tight on the regular side
	// too (0: never): the second half of a regular run is as tight as the extra stages.
	HardFrom int
	shifted  bool   // the road's middle moved on the last row
	still    int    // rows since the road's middle last moved
	openRun  [W]int // rows each column has been open, up to the row last built (see trap)
	// sinceTrap is the rows built since the last trap: no gate or pillar comes right after
	// one (the way around it one side and the gap of the gate the other was a dead end)
	sinceTrap int
	hold      int // rows still to come in the road's shape, after a trap
	settle    int // themed rows still to come without the theme's blocks (after a vault, a feast or rows not themed)
	// starts is the shape of the road where each course (by Level) began: a course runs on
	// from the one before, so a retry that builds a course again starts it from here
	starts map[int]roadShape
	// SideRowBehind: the row that just came in is still above her body (see sideRow).
	SideRowBehind bool
	center        int // middle column of the road being built at the top
	width         int // width of the road being built
	courseRow     int // rows built in the current course
	stageRow      int // rows built in the current stage
	// ids are which row of which course each row on the screen (and Ahead) is, and taken
	// the sweets she has picked up by row and column: a retry builds the road again, and
	// what she has taken stays taken (what she left lies there still)
	ids       [Rows]rowID
	aheadID   rowID
	taken     map[takenKey]bool
	alcove    int // column of a dent being carved into the wall (-1: none)
	alcoveFor int // rows the dent still runs
	Profile   Profile
	// TotalCourses is how many courses the game has (0: it goes on without end). Stages
	// have four courses; a short last stage has what is left (the hardest colors).
	TotalCourses int
	// CourseRowsFor gives the length of a course in rows at a level, so the game can keep
	// every course about as long in time as the scroll speeds up (CourseRows when nil).
	CourseRowsFor func(level int) int
	AllClear      bool // the last course is done and she has run out onto the open road after it
	// finishing: the last course is built and open road follows it; finishLeft is the
	// rows still to come before the all clear (until the last walls are behind her).
	finishing   bool
	finishLeft  int
	Missed      bool // she ran into a wall: the road waits for Restart (or the game is over)
	dir         int  // the way a snake road is shifting (-1 or +1)
	sinceObs    int  // rows since the last pillar or gate (they need room between them)
	targetWidth int  // the width the road is heading for
	sweetLives  int  // extra lives earned with sweets so far
	pathLeft    int  // sweets left to lay in the line of sweets being laid (see pathLine)
	pathX       int  // where the line of sweets is
	lastRow     Row  // the row built before (a gate's gap must be reachable from it)
	// A section is a stretch built to a pattern (a narrow zigzag, a slalom, a narrow
	// tunnel): its kind, the rows it still runs once the road has narrowed to it, and a
	// row counter for its rhythm.
	section     int
	sectionLeft int
	sectionRow  int
	// helpLaid has a bit (1 << the sweet) for each help of checkerHelp laid on the course
	// being built
	helpLaid int
	// ExtraHammerRow, when not 0, lays one more hammer on the first stage, on this row of
	// the stage (a character whose roads are harder gets it on her way).
	ExtraHammerRow int
	// themeRow: the row being built has the theme's blocks (it is past the rows that settle
	// the road into the theme)
	themeRow bool
	// designFrom is the row of the course being built where its theme began on the wide road
	// (-1: not yet): the themes laid out from there (ThemeSeconds, ThemeLesson) count their
	// rows from it, so a vault or a feast first only pushes them along
	designFrom int
}

// Tuning.
const (
	StartLives    = 3
	CourseRows    = 110 // rows of road in a course
	Courses       = 4   // courses in a stage
	minRoadWidth  = 3   // narrowest road at high levels
	maxRoadWidth  = 6
	sweetChance   = 0.16
	macaronChance = 0.35
	oneUpChance   = 0.03 // of the sweets
	MaxLives      = 9
	StartBombs    = 1
	MaxBombs      = 3
	SweetsPerLife = 100   // sweets for an extra life, the same every time (20 and then 150 was hard to follow; 50 gave four or five a run)
	trailRows     = 20    // the first course opens with a trail of sweets down the middle of the road, one a row
	pathChance    = 0.012 // of the rows with no line of sweets: one starts there
	pathMin       = 5     // a line of sweets is pathMin to pathMin+pathSpread-1 long
	pathSpread    = 5
	alcoveChance  = 0.3 // of the things placed: in a dent in the wall, to be fetched in a hurry
)

// Profile is the character of a road: how it wanders, how wide and fast it is, and
// what lies on it. Each playable character has her own profile.
type Profile struct {
	Speed      float64 // scroll speed relative to the standard road
	MaxWidth   int     // widest road
	Narrowing  int     // how much narrower the road gets with the difficulty (1: standard)
	Wander     float64 // how often the road shifts sideways (0.12 standard)
	Mixed      bool    // the courses take turns: a zigzag, then a wandering road
	Gates      float64 // chance per row of a gate: a row walled across but for a gap of two
	Pillars    float64 // chance per row of a block standing in the road, to weave around
	SweetsRate float64 // chance per row of a sweet
	OneUpRate  float64 // share of sweets that are extra lives
}

// Standard is the profile of the standard road.
var Standard = Profile{Speed: 1, MaxWidth: maxRoadWidth, Narrowing: 1, Wander: 0.12, SweetsRate: sweetChance, OneUpRate: oneUpChance}

// New starts a game on the standard road.
func New(seed uint64) *Game { return NewWith(seed, Standard) }

// NewWith starts a game: an open road with the player in the middle.
func NewWith(seed uint64, p Profile) *Game {
	g := &Game{
		Profile: p,
		seed:    seed,
		Lives:   StartLives,
		Bombs:   StartBombs,
		Level:   1,
		Stage:   1,
		X:       W/2 + 0.5,
		center:  W / 2,
		width:   p.MaxWidth,
		dir:     1,
	}
	for x := range g.reach {
		g.reach[x] = true
	}
	g.startCourse()
	g.aheadID = rowID{1, 0}
	g.Ahead = g.buildRow(true) // the visible rows start open
	return g
}

// CourseColors are the wall colors of the four courses of a stage, from the easiest
// to the most dangerous, from the candy colors of the blocks (values of Kind in the game:
// 1 soda, 2 lemon, 3 grape, 4 melon, 5 strawberry, 6 blueberry, 7 orange). Green is the
// calmest; red is the danger color.
var CourseColors = [Courses]int8{4, 1, 3, 5}

// Sections of the road.
const (
	sectionNone = iota
	sectionZigzag
	sectionSlalom
	sectionTunnel
	sectionVault     // a cage of blocks with a prize inside: only a hammer opens it
	sectionFeast     // a wide stretch with a sweet on every open cell: help yourself
	sectionKinds = 3 // the kinds that come at random (vaults and feasts are placed: Vaults, Feasts)
)

// Vaults are the courses (by Level) that have a cage of blocks with two prizes inside,
// which only a hammer can get at: always worth more than the hammer it takes (an extra life
// and the hammer back, so a vault never adds to the hammers). The road is the same every game, so
// they are always in the same places.
var Vaults = map[int][2]int8{
	3:  {SweetOneUp, SweetBomb},
	7:  {SweetOneUp, SweetBomb},
	11: {SweetOneUp, SweetBomb},
	14: {SweetOneUp, SweetBomb},
}

// Feasts are the courses (by Level) with a stretch of road full of sweets, one in each
// stage after the first: the bonus courses (Bonus).
var Feasts = map[int]bool{5: true, 12: true, 15: true}

// feastRows is how many rows of sweets a feast has.
const feastRows = 10

// vaultRows is how long the vault section runs once the road is wide enough for it: a
// row of room, the three rows of the cage, and a row of room.
const vaultRows = 5

// sectionChance is the chance per row that a section starts: none on the first course,
// then more and more (most of the road is sections in the later stages).
func sectionChance(d int) float64 {
	if d == 0 {
		return 0
	}
	return min(0.09, 0.025*float64(d))
}

// BombChance is the chance that a hammer lies somewhere on the road of a stage: an even
// chance at first, rarer every stage (a hammer is a rare help, not one a stage).
func BombChance(stage int) float64 { return max(0.1, 0.5-0.15*float64(stage-1)) }

// hammerRowOf is the row of a stage (counted from its start) where a hammer lies, or -1 for
// none. It has its own random numbers (from the seed and the stage), so it is the same
// every time, whichever course of the stage is being built, and does not change the road.
func (g *Game) hammerRowOf(stage int) int {
	r := rand.New(rand.NewPCG(g.seed^0x5bd1e995, uint64(stage))) //nolint:gosec // G404: game randomness, not security sensitive
	if r.Float64() >= BombChance(stage) {
		return -1
	}
	rows := 0
	for lv := (stage-1)*Courses + 1; lv < (stage-1)*Courses+1+g.coursesIn(stage); lv++ {
		rows += g.lenOf(lv)
	}
	return r.IntN(max(1, rows))
}

// rowID is a row of the road: the course (Level) and its row in the course.
type rowID struct{ level, row int }

// takenKey is a sweet picked up: its row and its column.
type takenKey struct {
	id rowID
	x  int
}

// pushRow moves the rows down by one and brings in a new row at the top (Ahead), built
// as row courseRow of the course at Level.
func (g *Game) pushRow() {
	copy(g.Rows[1:], g.Rows[:Rows-1])
	copy(g.ids[1:], g.ids[:Rows-1])
	g.Rows[0], g.ids[0] = g.Ahead, g.aheadID
	g.aheadID = rowID{g.Level, g.courseRow}
	g.Ahead = g.buildRow(true)
}

// StageCourses is how many courses the current stage has: four, or what is left of the
// game in the last stage.
func (g *Game) StageCourses() int { return g.coursesIn(g.Stage) }

// coursesIn is how many courses the stage has (see StageCourses).
func (g *Game) coursesIn(stage int) int {
	if g.TotalCourses <= 0 {
		return Courses
	}
	return max(1, min(Courses, g.TotalCourses-(stage-1)*Courses))
}

// WallColor is the color of the walls of the current course. A short last stage takes the
// last (hardest) colors.
func (g *Game) WallColor() int8 { return CourseColors[Courses-g.StageCourses()+g.Course] }

// difficulty is how hard the course being built is: 0 for the first course, rising by
// one every two courses.
func (g *Game) difficulty() int { return max(0, g.Level-1) / 2 }

// buildRow makes the next row at the top of the road. withThings adds sweets.
func (g *Game) buildRow(withThings bool) Row {
	reach, last := g.reach, g.lastRow
	g.sinceTrap++
	g.themeRow = false
	row := g.buildRoad(withThings)
	g.hold = max(0, g.hold-1)
	if withThings && !g.finishing && !g.AllClear {
		row = g.trap(row, reach, last)
	}
	for x := range row { // how long each column has been open, for the next trap
		if row[x].Wall == 0 {
			g.openRun[x]++
		} else {
			g.openRun[x] = 0
		}
	}
	if g.Bonus() { // a bonus course is walled in every candy color
		for x := range row {
			if row[x].Wall != 0 {
				row[x].Wall = colorfulWall(g.Level, g.courseRow, x)
			}
		}
	}
	return row
}

// trapRun is how many rows a column may stay open before a block is put in it: a road
// that lets her run straight for that long gets one block in her way at the end.
const trapRun = 10

// trap puts one block in the column that has been open longest, when it has been open for
// trapRun rows: a long straight run ends with a block to dodge. reach and last are the
// cells she could be on and the row before this one; the block goes in only with open
// cells on both sides of it and if the road can still be followed.
func (g *Game) trap(row Row, reach [W]bool, last Row) Row {
	if g.section != sectionNone || g.themeRow && g.theme().designed() || g.theme().laidOut() {
		// not in a vault or a feast, nor in the shape of a designed theme, nor anywhere on a
		// course laid out from where it begins (a block on the way in put it off)
		return row
	}
	open, best := 0, -1
	for x := range W {
		if row[x].Wall != 0 {
			continue
		}
		open++
		if row[x].Sweet == 0 && g.openRun[x] >= trapRun && (best < 0 || g.openRun[x] > g.openRun[best]) {
			best = x
		}
	}
	if best < 0 || open < 3 || !g.canTrap(row, last, best) {
		return row
	}
	built, builtReach := row, g.reach
	row[min(best, W-1)].Wall = g.WallColor()
	g.reach, g.lastRow = reach, last
	if !g.passable(row) {
		g.reach, g.lastRow = builtReach, built
		return built
	}
	g.updateReach(row)
	g.lastRow = row
	g.sinceTrap, g.hold = 0, trapHold
	g.settle = max(g.settle, 6) // and no theme blocks close after it either
	g.openRun = [W]int{}        // the next trap only after another long straight run
	return row
}

// canTrap reports whether column x of row can take a trap block: she can step aside to an
// open neighbour that has been open for a while (in this row and the row before), so there
// is a way around it, and it does not come right after another block. A column along the
// wall has one such side, and that is enough: the side of the road was a safe lane before.
func (g *Game) canTrap(row, last Row, x int) bool {
	sides := 0
	for _, nx := range []int{x - 1, x + 1} {
		if nx < 0 || nx >= W || row[nx].Wall != 0 {
			continue
		}
		if last[nx].Wall != 0 || g.openRun[nx] < trapSideRun {
			return false // an open side that was walled just now: dodging into a block
		}
		sides++
	}
	// along the wall it waits a little longer: a theme that closes the sides now and then
	// (gates with the gap at either side) already keeps her from running down them
	if sides == 2 {
		return true
	}
	if sides == 0 || g.openRun[x] < trapEdgeRun || !alongSide(row, x) {
		return false
	}
	// with one way around it, that way has to have been open as long (a side that only
	// just opened left no time to step over on the fast last courses)
	for _, nx := range []int{x - 1, x + 1} {
		if nx >= 0 && nx < W && row[nx].Wall == 0 && g.openRun[nx] < trapEdgeRun {
			return false
		}
	}
	return true
}

// alongSide reports whether column x of row is next to the side of the road: the cells
// from x to one edge of the screen are all walls but x itself. A column beside a block in
// the middle of the road is not (a trap there made a wall two wide to go around).
func alongSide(row Row, x int) bool {
	left, right := true, true
	for i := range x {
		left = left && row[i].Wall != 0
	}
	for i := x + 1; i < W; i++ {
		right = right && row[i].Wall != 0
	}
	return x > 0 && left || x < W-1 && right
}

// trapEdgeRun is how many rows a column along the wall may stay open before a trap.
const trapEdgeRun = 14

// trapSideRun is how many rows a side of a trap must have been open.
const trapSideRun = 6

// trapHold is how many rows after a trap the road keeps its shape, so a dodge to either
// side of the block leads on.
const trapHold = 3

// bonusSweets is how many times the sweets there are on a bonus course.
func bonusSweets(bonus bool) float64 {
	if bonus {
		return 2
	}
	return 1
}

// Bonus reports whether the current course is a bonus course: one with a feast (Feasts),
// a wider road and twice the sweets, walled in every candy color so it stands out.
func (g *Game) Bonus() bool { return g.TotalCourses > 0 && Feasts[g.Level] }

// colorfulWall is the color of the wall block at column x of row r of the course at level:
// any of the candy colors, the same every time the row is built.
func colorfulWall(level, r, x int) int8 {
	h := uint32(level)*0x9e3779b1 ^ uint32(r)*0x85ebca6b ^ uint32(x)*0xc2b2ae35 //nolint:gosec // G115: small non-negative numbers
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 13
	return int8(1 + h%7)
}

// roadSpan is the first and last open columns of a road width cells wide around the
// column center, kept on the screen.
func roadSpan(center, width int) (left, right int) {
	left = center - width/2
	right = left + width - 1
	if left < 0 {
		left, right = 0, width-1
	}
	if right > W-1 {
		right, left = W-1, W-width
	}
	return left, right
}

// keepOnScreen works out where the road being built lies (roadSpan) and moves its middle
// there: the next row (of this course or the next, which may build its rows another way)
// goes on from where this one is.
func (g *Game) keepOnScreen() (left, right int) {
	left, right = roadSpan(g.center, g.width)
	g.center = left + g.width/2
	return left, right
}

// walled is a row of road open from left to right and walled in the course's color
// (WallColor) on both sides.
func (g *Game) walled(left, right int) Row {
	var row Row
	for x := range W {
		if x < left || x > right {
			row[x].Wall = g.WallColor()
		}
	}
	return row
}

// buildRoad builds the next row with the walls in the course's own color (WallColor).
func (g *Game) buildRoad(withThings bool) Row {
	if g.finishing || g.AllClear {
		// the open road after the last course (the last illustration shows through it)
		g.lastRow = Row{}
		for x := range g.reach {
			g.reach[x] = true
		}
		return Row{}
	}
	// The road wanders: the middle moves by one now and then, and the width breathes
	// between the narrowest width for the course and maxRoadWidth. Harder courses are
	// narrower and wander more.
	p := g.Profile
	d := g.difficulty()
	narrow := max(minRoadWidth, p.MaxWidth-d*p.Narrowing/2)
	wander := min(0.4, p.Wander+0.02*float64(d))
	centerBefore := g.center
	canLeft, canRight := g.center > 1+g.width/2, g.center < W-2-(g.width-1)/2
	slalom := -1   // the side a slalom block juts from this row (-1: none)
	cage := -1     // the row of the vault's cage this row is (0-2; -1: none)
	feast := false // a row of a feast: a sweet on every open cell
	// a vault or a feast is due: nothing new starts until it has room (six rows past the
	// last obstacle)
	vaultDue := (g.vaultAt >= 0 && g.courseRow >= g.vaultAt) || (g.feastAt >= 0 && g.courseRow >= g.feastAt)
	if g.section == sectionNone && withThings && vaultDue && g.sinceObs >= 6 {
		if g.feastAt >= 0 && g.courseRow >= g.feastAt {
			g.section, g.sectionLeft, g.sectionRow, g.feastAt = sectionFeast, feastRows, 0, -1
		} else {
			g.section, g.sectionLeft, g.sectionRow, g.vaultAt = sectionVault, vaultRows, 0, -1
		}
		vaultDue = false
	}
	if withThings && !g.themed(vaultDue) {
		g.settle = settleRows
	}
	if withThings && g.themed(vaultDue) {
		row, left, right := g.buildThemedRow()
		g.updateReach(row)
		g.lastRow = row
		g.sinceObs = 0
		return g.addThings(row, left, right)
	}
	if !vaultDue && g.section == sectionNone && withThings && g.Themes == nil && g.rng.Float64() < sectionChance(d) {
		g.section = 1 + g.rng.IntN(sectionKinds)
		g.sectionLeft, g.sectionRow = 14+g.rng.IntN(12), 0
	}
	switch {
	case g.section != sectionNone:
		want := 2
		switch g.section {
		case sectionZigzag:
			want = 3 // two wide was too hard to follow on the fast last course
		case sectionSlalom:
			want = 5
		case sectionVault:
			want = 7 // the cage takes four, and three are left to pass by
		case sectionFeast:
			want = 7
		}
		switch {
		case g.width > want: // narrow down first, one cell a row
			g.width--
		case g.width < want:
			g.width++
		default:
			g.sectionRow++
			switch g.section {
			case sectionZigzag:
				// three cells wide, shifting one cell every other row, turning at the sides
				if g.sectionRow%2 == 0 {
					if (g.dir < 0 && !canLeft) || (g.dir > 0 && !canRight) {
						g.dir = -g.dir
					}
					if g.dir < 0 && canLeft {
						g.center--
					} else if g.dir > 0 && canRight {
						g.center++
					}
				}
			case sectionSlalom:
				// a block of three juts in every fourth row, from the left and the right in turn
				if g.sectionRow%4 == 0 {
					slalom = (g.sectionRow / 4) % 2
				}
			case sectionVault:
				cage = g.sectionRow - 2 // the cage's three rows come after a row of room
			case sectionFeast:
				feast = true
			}
			if g.sectionLeft--; g.sectionLeft <= 0 {
				g.section = sectionNone
				g.targetWidth = min(g.width, p.MaxWidth) // narrow back to the usual road
			}
		}
	case g.width < narrow:
		// out of a narrow section: widen back first (one cell a row), and only then wander
		g.width++
		g.targetWidth = max(g.targetWidth, g.width)
	default:
		if p.Mixed && g.Course%2 == 0 {
			// a zigzag: keep shifting one way, turn at the sides (and now and then on the way)
			if (g.dir < 0 && !canLeft) || (g.dir > 0 && !canRight) || g.rng.Float64() < 0.04 {
				g.dir = -g.dir
			}
			if g.rng.Float64() < 2*wander {
				if g.dir < 0 && canLeft {
					g.center--
				} else if g.dir > 0 && canRight {
					g.center++
				}
			}
		} else {
			switch r := g.rng.Float64(); {
			case r < wander && canLeft:
				g.center--
			case r < 2*wander && canRight:
				g.center++
			}
		}
		// a road four cells wide or less shifts at most every other row (as on a themed course),
		// and one two wide runs straight: a diagonal that narrow is too much on the fast
		// last courses
		// nor right after a gate or a pillar: through a gap at one side of the road and
		// then off the other way at once was a dash on a single row
		if g.width <= 2 || g.width <= 4 && g.shifted || g.sinceObs < 2 || g.hold > 0 {
			g.center = centerBefore
		}
		// The width moves one cell at a time toward a target, and not on a row where the
		// middle moved, so an edge of the road never jumps by more than one cell a row.
		if g.rng.Float64() < 0.12 {
			g.targetWidth = narrow + g.rng.IntN(max(1, p.MaxWidth-narrow+1))
		}
		g.targetWidth = max(g.targetWidth, narrow)
		if g.center == centerBefore && g.hold == 0 {
			switch {
			case g.width < g.targetWidth:
				g.width++
			case g.width > g.targetWidth:
				g.width--
			}
		}
	}
	g.shifted = g.center != centerBefore
	if g.still++; g.shifted {
		g.still = 0
	}
	left, right := g.keepOnScreen()
	row := g.walled(left, right)
	if slalom >= 0 {
		from := left
		if slalom == 1 {
			from = right - 2
		}
		for x := from; x < from+3; x++ {
			row[x].Wall = g.WallColor()
		}
		g.sinceObs = 0
	}
	if feast {
		for x := left; x <= right; x++ {
			row[x].Sweet = SweetCandy
		}
		g.sinceObs = 0
		g.updateReach(row)
		g.lastRow = row
		return row
	}
	if cage >= 0 && cage <= 2 {
		// the cage stands against one side of the road (the side changes course by course):
		// a row of four blocks, then a block on each side of the two prizes, then four again
		c := left
		if g.Level%2 == 0 {
			c = right - 3
		}
		row[c].Wall, row[c+3].Wall = g.WallColor(), g.WallColor()
		if cage == 1 {
			row[c+1].Sweet, row[c+2].Sweet = Vaults[g.Level][0], Vaults[g.Level][1]
		} else {
			row[c+1].Wall, row[c+2].Wall = g.WallColor(), g.WallColor()
		}
		g.sinceObs = 0
		g.updateReach(row)
		g.lastRow = row
		return row
	}
	// Obstacles inside the road, with room between them: a pillar (one block away from
	// both walls, to slip past on either side) or a gate (the road walled across but for
	// a gap of two cells, to aim for).
	g.sinceObs++
	if withThings && g.sinceObs >= 6 && g.sinceTrap >= 6 && g.section == sectionNone && !vaultDue && g.width <= p.MaxWidth &&
		(g.Themes == nil || g.theme() == ThemeMixed) {
		switch r := g.rng.Float64(); {
		case g.width >= 4 && r < p.Gates*(1+0.1*float64(d)):
			// the gap opens over cells that are open in the row before, so it can be reached
			var gaps []int
			for x := left; x < right; x++ {
				if g.lastRow[x].Wall == 0 && g.lastRow[x+1].Wall == 0 {
					gaps = append(gaps, x)
				}
			}
			if len(gaps) > 0 {
				gap := gaps[g.rng.IntN(len(gaps))]
				for x := left; x <= right; x++ {
					if x != gap && x != gap+1 {
						row[x].Wall = g.WallColor()
					}
				}
				g.sinceObs = 0
			}
		case g.width >= 5 && r < (p.Gates+p.Pillars)*(1+0.1*float64(d)):
			row[left+2+g.rng.IntN(g.width-4)].Wall = g.WallColor()
			g.sinceObs = 0
		}
	}
	// A dent in the wall carries on for its second row.
	if g.alcoveFor > 0 {
		g.alcoveFor--
		if g.alcove >= 0 && g.alcove < W && (g.alcove == left-1 || g.alcove == right+1) {
			row[g.alcove].Wall = 0
		}
	}
	g.updateReach(row)
	g.lastRow = row
	if !withThings {
		return row
	}
	return g.addThings(row, left, right)
}

// addThings puts the sweets (and the stage's hammer) on a row of road between left and right.
func (g *Game) addThings(row Row, left, right int) Row {
	p := g.Profile
	g.stageRow++
	var thing int8
	// The very start of the game: a trail of sweets down the middle of the road, one a
	// row, which leads straight to the first extra life.
	if g.Stage == 1 && g.Course == 0 && g.stageRow > 6 && g.stageRow <= 6+trailRows {
		for _, x := range []int{(left + right) / 2, left, right} { // the middle, unless a pillar stands there
			if row[x].Wall == 0 {
				row[x].Sweet = SweetCandy
				break
			}
		}
		return row
	}
	switch {
	case g.checkerHelp() != SweetNone:
		thing = g.checkerHelp()
		g.helpLaid |= 1 << thing
	case g.stageRow == g.hammerRowOf(g.Stage):
		thing = SweetBomb
	case g.Stage == 1 && g.ExtraHammerRow > 0 && g.stageRow == g.ExtraHammerRow:
		thing = SweetBomb
	case g.themeRow && g.theme().designed():
		// a designed theme lays its own sweets (below), and none at random
	case g.rng.Float64() < p.SweetsRate*bonusSweets(g.Bonus()):
		switch r := g.rng.Float64(); {
		case r < p.OneUpRate && g.Lives < MaxLives:
			thing = SweetOneUp
		case r < p.OneUpRate+macaronChance:
			thing = SweetMacaron
		default:
			thing = SweetCandy
		}
	}
	if thing != SweetNone {
		g.place(&row, left, right, thing)
	}
	if g.themeRow && g.theme().designed() {
		g.pathLeft = 0
		g.designSweets(&row, g.theme(), g.courseRow, left, right)
		return row
	}
	g.pathLine(&row, left, right)
	return row
}

// checkerHelp is the help due on the row being built of a checkerboard course (or
// SweetNone): a hammer as its checker rows begin, from row checkerHammerRow, and an extra
// life halfway through, each on the first row of sweets from there (not in a feast). The
// checkerboard asks for a step aside on most of its rows, and a dense stretch of it was
// hard to get through without one of them.
func (g *Game) checkerHelp() int8 {
	if g.TotalCourses == 0 || g.theme() != ThemeCheckers {
		return SweetNone
	}
	switch {
	case g.helpLaid&(1<<SweetBomb) == 0 && g.courseRow >= checkerHammerRow:
		return SweetBomb
	case g.helpLaid&(1<<SweetOneUp) == 0 && g.courseRow >= g.lenOf(g.Level)/2:
		return SweetOneUp
	}
	return SweetNone
}

// checkerHammerRow is the row of a checkerboard course from which its hammer is laid: just
// past the rows that turn the road to the theme (settleRows), as the first checker rows come.
const checkerHammerRow = settleRows + 2

// pathLine lays lines of sweets that trace the way along the road, like the coins of the
// old platform games: a line starts now and then and puts a sweet a row in the middle of
// the road (or the open cell next to it, when a pillar stands there), so following the
// line picks them all up.
func (g *Game) pathLine(row *Row, left, right int) {
	if g.section != sectionNone {
		g.pathLeft = 0
		return
	}
	// its own numbers from the seed and the row, so the lines leave the road and the other
	// sweets exactly as they were (the roads can be learned)
	h := pathHash(g.seed, g.Stage, g.stageRow)
	if g.pathLeft == 0 {
		if float64(h%1000)/1000 >= pathChance {
			return
		}
		g.pathLeft = pathMin + int(h/1000%pathSpread)
		g.pathX = (left + right) / 2
	}
	mid := (left + right) / 2
	best := -1
	for _, x := range []int{g.pathX, g.pathX - 1, g.pathX + 1} {
		if x < left || x > right || x < 0 || x >= W || row[x].Wall != 0 {
			continue
		}
		if best < 0 || abs(x-mid) < abs(best-mid) {
			best = x
		}
	}
	if best < 0 {
		g.pathLeft = 0 // the way is shut here: the line ends
		return
	}
	if row[best].Sweet == SweetNone { // a sweet or an item already there is part of the line
		row[best].Sweet = SweetCandy
	}
	g.pathX = best
	g.pathLeft--
}

// pathHash mixes the seed, the stage and the row into the numbers of pathLine.
func pathHash(seed uint64, stage, row int) uint64 {
	x := seed ^ uint64(stage)*0x9e3779b97f4a7c15 ^ uint64(row)*0xbf58476d1ce4e5b9 //nolint:gosec // G115: small non-negative numbers
	x ^= x >> 31
	x *= 0x94d049bb133111eb
	return x ^ x>>29
}

// place puts a thing on the road where it tempts the player into a risk: often right by
// a wall or a pillar, and sometimes in a dent in the wall that she must dart into and out
// of before the wall comes back. Extra lives and hammers go to the risky places more often.
func (g *Game) place(row *Row, left, right int, thing int8) {
	precious := thing == SweetOneUp || thing == SweetBomb
	dent := alcoveChance
	if precious {
		dent *= 2
	}
	if g.alcoveFor == 0 && g.rng.Float64() < dent {
		x := left - 1
		if g.rng.IntN(2) == 0 {
			x = right + 1
		}
		if x >= 0 && x < W {
			row[x].Wall = 0
			row[x].Sweet = thing
			g.alcove, g.alcoveFor = x, 1 // the dent is two rows deep, so it can be reached
			return
		}
	}
	var spots []int
	for x := left; x <= right; x++ {
		if row[x].Wall != 0 {
			continue
		}
		// right by a wall or a pillar
		if x == left || x == right || (x > 0 && row[x-1].Wall != 0) || (x < W-1 && row[x+1].Wall != 0) {
			spots = append(spots, x, x) // twice as likely
		}
		spots = append(spots, x)
	}
	if len(spots) > 0 {
		row[spots[g.rng.IntN(len(spots))]].Sweet = thing
	}
}

// roadShape is the state of the road being built that carries over from one course to
// the next.
type roadShape struct {
	center, width, targetWidth, still, settle int
	shifted                                   bool
	lastRow                                   Row
	reach                                     [W]bool
	openRun                                   [W]int
	sinceTrap, hold                           int
	pathLeft, pathX                           int
}

func shapeOf(g *Game) roadShape {
	return roadShape{center: g.center, width: g.width, targetWidth: g.targetWidth, still: g.still,
		settle: g.settle, shifted: g.shifted, lastRow: g.lastRow, reach: g.reach, openRun: g.openRun, sinceTrap: g.sinceTrap, hold: g.hold,
		pathLeft: g.pathLeft, pathX: g.pathX}
}

func (s roadShape) restore(g *Game) {
	g.center, g.width, g.targetWidth, g.still, g.settle = s.center, s.width, s.targetWidth, s.still, s.settle
	g.shifted, g.lastRow, g.reach, g.openRun, g.sinceTrap, g.hold = s.shifted, s.lastRow, s.reach, s.openRun, s.sinceTrap, s.hold
	g.pathLeft, g.pathX = s.pathLeft, s.pathX
}

// specialAt is the row of a course where its vault or feast starts: early, so it is all
// built before the course ends (a short course could end in the middle of it, which cut
// the cage open).
const specialAt = 2

// startCourse begins building the road of the course at g.Level: the same road every
// time for the same course (the random numbers start over from the seed and the course
// number). The road runs on from the course before: there is no open stretch between
// courses (it was there to show the new illustration, which now shows all the time).
func (g *Game) startCourse() {
	g.rng = rand.New(rand.NewPCG(g.seed, uint64(g.Level)*0x9e3779b97f4a7c15)) //nolint:gosec // G404: game randomness, not security sensitive
	if s, ok := g.starts[g.Level]; ok {
		s.restore(g) // built again (a retry): from where it began the first time
	} else {
		if g.starts == nil {
			g.starts = map[int]roadShape{}
		}
		g.starts[g.Level] = shapeOf(g)
	}
	g.settle = settleRows // the road turns to the new course's theme before its blocks come
	g.section, g.sinceObs, g.alcove, g.alcoveFor, g.dir = sectionNone, 0, -1, 0, 1
	g.vaultAt, g.feastAt = -1, -1
	g.helpLaid = 0
	g.designFrom = -1
	if _, ok := Vaults[g.Level]; ok && g.TotalCourses > 0 {
		g.vaultAt = specialAt
	}
	if Feasts[g.Level] && g.TotalCourses > 0 {
		g.feastAt = specialAt
	}
}

// nextCourse counts the rows of the course being built and starts the next course
// (and, after the last of a stage, the next stage) when it is long enough.
func (g *Game) nextCourse() {
	if g.AllClear {
		return
	}
	g.courseRow++
	if g.finishing {
		if g.finishLeft--; g.finishLeft <= 0 {
			g.AllClear = true
			g.Events = append(g.Events, Event{Kind: EventAllClear})
		}
		return
	}
	if g.courseRow < g.lenOf(g.Level) {
		return
	}
	g.courseRow = 0
	g.Course++
	g.Level++
	if g.TotalCourses > 0 && g.Level > g.TotalCourses {
		// the last course is built: open road follows, and the all clear comes once the
		// last walls have passed her (she runs out onto the open road, not mid-course)
		g.Course--
		g.finishing, g.finishLeft = true, finishRows
		g.Events = append(g.Events, Event{Kind: EventCourse})
		return
	}
	g.startCourse()
	g.Events = append(g.Events, Event{Kind: EventCourse})
	if g.Course == g.StageCourses() {
		g.Course = 0
		g.Stage++
		g.stageRow = 0
		g.Events = append(g.Events, Event{Kind: EventStageClear})
	}
}

// finishRows is how many rows of open road come after the last course before the all
// clear: the walls ahead of her when it is built (the screen above her) get behind her.
const finishRows = PlayerRow + 3

// Half is half the width of the player in cells: she covers 0.3 of a cell, less than the
// bunny drawn (forgiving: brushing a wall with her ears or fur is not a miss, but 0.22 made
// the walls too easy to slip by).
const Half = 0.15

// Col is the column under the middle of the player.
func (g *Game) Col() int { return min(W-1, max(0, int(g.X))) }

// span returns the first and last columns the player covers when her middle is at x.
func span(x float64) (int, int) {
	return max(0, int(x-Half)), min(W-1, int(x+Half-1e-9))
}

// blocked reports whether a wall in the player's row covers part of the span at x.
func (g *Game) blocked(x float64) bool { return g.blockedIn(x, PlayerRow) }

// blockedIn reports whether a player at x touches a wall of row y.
func (g *Game) blockedIn(x float64, y int) bool {
	lo, hi := span(x)
	for c := lo; c <= hi; c++ {
		if g.Rows[y][c].Wall != 0 {
			return true
		}
	}
	return false
}

// SideRowBehind is set while the row that just came in is still mostly above the bunny
// (the first half of its time, as the game draws it): touching from the side is then
// judged against the row before, which is the one beside her body. ReachHalfway
// switches back to her own row.
func (g *Game) sideRow() int {
	if g.SideRowBehind && PlayerRow+1 < Rows {
		return PlayerRow + 1
	}
	return PlayerRow
}

// ReachHalfway is called when the row that came in has slid halfway down onto the
// bunny: from now on she is judged against it from the side too, and if she stands in
// one of its walls, it is a miss (it has come down over her ears and head).
func (g *Game) ReachHalfway() {
	g.SideRowBehind = false
	if !g.Over && !g.Missed && g.Safe == 0 && g.blocked(g.X) {
		g.crash()
	}
}

// Move slides the player by dx cells (any amount; negative is left). She stops at the
// edges of the screen; touching a wall from the side is a miss. It reports whether she moved.
func (g *Game) Move(dx float64) bool {
	if g.Over || g.Missed || dx == 0 {
		return false
	}
	start := g.X
	// slide in small steps so a wall stops her right at its side
	const stepSize = 0.05
	for dx != 0 {
		d := math.Copysign(math.Min(math.Abs(dx), stepSize), dx)
		next := math.Min(W-Half, math.Max(Half, g.X+d))
		if next == g.X {
			break
		}
		if g.blockedIn(next, g.sideRow()) && g.Safe == 0 {
			// touching a wall from the side is a miss too
			g.X = next
			g.crash()
			return true
		}
		g.X = next
		dx -= d
	}
	g.pick()
	return g.X != start
}

// Step scrolls the road down by one row and resolves what reaches the player.
func (g *Game) Step() {
	if g.Over || g.Missed {
		return
	}
	// a sweet that scrolls past the player is missed
	for _, c := range g.Rows[PlayerRow] {
		switch c.Sweet {
		case SweetNone, SweetBomb:
		case SweetOneUp: // an extra life let go by: told, but the streak of sweets holds
			g.Events = append(g.Events, Event{Kind: EventMiss, Sweet: c.Sweet})
		default:
			g.Streak = 0
			g.Events = append(g.Events, Event{Kind: EventMiss, Sweet: c.Sweet})
		}
	}
	g.pushRow()
	g.Distance++
	g.SideRowBehind = true
	g.nextCourse()
	if g.Safe > 0 {
		g.Safe--
		g.pick()
		return
	}
	// A wall that comes into her row is not a miss yet: it is drawn with only its bottom
	// edge on the tips of her ears. It hits her halfway down (ReachHalfway), unless she
	// has slid out from under it by then.
	row := g.Rows[PlayerRow]
	g.pick()
	// walls right beside the player on both sides are a near miss
	lo, hi := span(g.X)
	if lo > 0 && hi < W-1 && row[lo-1].Wall != 0 && row[hi+1].Wall != 0 { //nolint:gosec // G602: hi < W-1 keeps hi+1 inside the row
		g.Events = append(g.Events, Event{Kind: EventNearMiss})
	}
}

// pick collects the sweets the player covers.
func (g *Game) pick() {
	lo, hi := span(g.X)
	for col := lo; col <= hi; col++ {
		g.pickAt(col)
	}
}

func (g *Game) pickAt(col int) {
	c := &g.Rows[PlayerRow][col]
	if c.Sweet == 0 {
		return
	}
	if g.taken == nil {
		g.taken = map[takenKey]bool{}
	}
	g.taken[takenKey{g.ids[PlayerRow], col}] = true
	switch c.Sweet {
	case SweetOneUp:
		g.Lives = min(MaxLives, g.Lives+1)
		g.Events = append(g.Events, Event{Kind: EventOneUp, Sweet: SweetOneUp}) // picked up, not earned with sweets
		c.Sweet = 0
		return
	case SweetBomb:
		g.gainHammer()
		c.Sweet = 0
		return
	}
	g.Streak++
	g.Events = append(g.Events, Event{Kind: EventPick, Sweet: c.Sweet, Streak: g.Streak})
	g.Sweets++
	if need := g.SweetsForLife(); g.Sweets >= need {
		g.Sweets -= need
		g.sweetLives++
		if g.Lives < MaxLives {
			g.Lives++
			g.Events = append(g.Events, Event{Kind: EventOneUp})
		}
	}
	c.Sweet = 0
}

// gainHammer adds a hammer to the stock, up to MaxBombs.
func (g *Game) gainHammer() {
	if g.Bombs < MaxBombs {
		g.Bombs++
		g.Events = append(g.Events, Event{Kind: EventBombGain})
	}
}

// UseBomb swings a hammer from the stock: every wall on the screen (and the row about
// to come in) is gone; the sweets stay. It reports whether there was a hammer.
func (g *Game) UseBomb() bool {
	if g.Over || g.Bombs == 0 {
		return false
	}
	g.Bombs--
	for y := range g.Rows {
		for x := range W {
			g.Rows[y][x].Wall = 0
		}
	}
	for x := range W {
		g.Ahead[x].Wall = 0
	}
	g.Events = append(g.Events, Event{Kind: EventBomb})
	return true
}

// crash is a miss: the road stops. With a life left, Restart starts the current stage
// over from its first course (using up the life); with none, the game is over.
func (g *Game) crash() {
	g.Streak = 0
	g.Events = append(g.Events, Event{Kind: EventCrash})
	if g.Lives <= 0 {
		g.Over = true
		g.Events = append(g.Events, Event{Kind: EventOver})
		return
	}
	g.Missed = true
}

// SweetsForLife is how many sweets the next extra life takes (SweetsPerLife).
func (g *Game) SweetsForLife() int { return SweetsPerLife }

// GiveUp ends the game after a miss instead of using a life.
func (g *Game) GiveUp() {
	if !g.Missed {
		return
	}
	g.Missed = false
	g.Over = true
	g.Events = append(g.Events, Event{Kind: EventOver})
}

// RewindRows is how far back a retry starts: this many rows before the miss.
const RewindRows = 10

// rewindTo is where a retry starts: the course (Level) and the rows of it built by then,
// RewindRows before now (into the course before, when this one is younger than that).
func (g *Game) rewindTo() (level, rows int) {
	if g.finishing { // on the open road after the last course: back into the last course
		level := g.Level - 1
		return level, max(0, min(g.lenOf(level), g.lenOf(level)+g.courseRow-RewindRows))
	}
	level, rows = g.Level, g.courseRow-RewindRows
	for rows < 0 && level > 1 {
		level--
		rows += g.lenOf(level)
	}
	return level, max(0, rows)
}

// RewindLevel is the course (Level) a retry would start on.
func (g *Game) RewindLevel() int {
	level, _ := g.rewindTo()
	return level
}

// lenOf is the length in rows of the course at level.
func (g *Game) lenOf(level int) int {
	if g.CourseRowsFor != nil {
		return max(1, g.CourseRowsFor(level))
	}
	return CourseRows
}

// Restart uses a life after a miss and starts over RewindRows before the place she
// missed at. The roads are the same every time, so the road is built again from the
// start of the course before (the screen shows just what it showed then), and she is
// put on an open cell of her row with open cells ahead of it. It reports whether it did.
func (g *Game) Restart() bool {
	if !g.Missed || g.Lives <= 0 {
		return false
	}
	g.Lives--
	g.Missed = false
	g.Safe, g.Streak = 0, 0
	level, rows := g.rewindTo()
	g.finishing, g.finishLeft = false, 0
	oldX := g.X
	g.Rows, g.Ahead = [Rows]Row{}, Row{}
	for x := range g.reach {
		g.reach[x] = true
	}
	g.center, g.width = W/2, W
	build := func(lv, n int) {
		g.Level = lv
		g.Stage, g.Course = (lv-1)/Courses+1, (lv-1)%Courses
		g.startCourse()
		g.stageRow = 0
		for c := (g.Stage-1)*Courses + 1; c < lv; c++ {
			g.stageRow += g.lenOf(c)
		}
		for g.courseRow = 0; g.courseRow < n; g.courseRow++ {
			g.pushRow()
		}
	}
	if level > 1 {
		build(level-1, g.lenOf(level-1))
	}
	build(level, rows)
	if level == 1 && rows == 0 {
		g.aheadID = rowID{1, 0}
		g.Ahead = g.buildRow(true)
		g.courseRow = 1 // the row just built is the first of the course
	}
	g.X = g.openColumn(oldX)
	// what she has taken stays taken: a retry is not a second helping (lives taken again
	// made a run easy to force through); what she left lies there still
	for y := range g.Rows {
		for x := range W {
			if g.taken[takenKey{g.ids[y], x}] {
				g.Rows[y][x].Sweet = 0
			}
		}
	}
	for x := range W {
		if g.taken[takenKey{g.aheadID, x}] {
			g.Ahead[x].Sweet = 0
		}
	}
	g.SideRowBehind = true // the road starts again with its last row just come in
	g.Events = append(g.Events, Event{Kind: EventRestart})
	return true
}

// StartAt runs the road of a game just begun on to the start of the course at level (the
// last course at most): the rows are built one by one as in play, so Level, Stage and
// Course agree, the screen shows the end of the course before, and the road from there
// on is the one every game has. It is for the demo recording and the screenshots.
func (g *Game) StartAt(level int) {
	if g.TotalCourses > 0 {
		level = min(level, g.TotalCourses)
	}
	events := len(g.Events)
	for g.Level < level {
		g.pushRow()
		g.Distance++
		g.nextCourse()
	}
	g.Events = g.Events[:events] // nothing happened to her on the way
	g.X = g.openColumn(g.X)
}

// openColumn is where she stands after a retry: an open cell of her row with open cells
// straight ahead of it (up to seven rows of room count), the nearest to x among those. The
// row below hers counts too: most of her body is drawn over it.
func (g *Game) openColumn(x float64) float64 {
	best, bestRun, bestDist := W/2, -1, math.MaxFloat64
	for c := range W {
		run := 0
		for y := PlayerRow + 1; y >= 0 && run < 8 && g.Rows[y][c].Wall == 0; y-- {
			run++
		}
		if dist := math.Abs(float64(c) + 0.5 - x); run > bestRun || (run == bestRun && dist < bestDist) {
			best, bestRun, bestDist = c, run, dist
		}
	}
	return float64(best) + 0.5
}

// RoadWidthAhead returns the narrowest road width within the next n rows above the player.
func (g *Game) RoadWidthAhead(n int) int {
	narrow := W
	for y := max(0, PlayerRow-n); y < PlayerRow; y++ {
		w := 0
		for x := range W {
			if g.Rows[y][x].Wall == 0 {
				w++
			}
		}
		narrow = min(narrow, w)
	}
	return narrow
}

// SweetAhead reports whether a sweet of the kind lies within the next n rows above the player
// (by kind, not by the largest kind number: a hammer nearby hid an extra life).
func (g *Game) SweetAhead(n int, kind int8) bool {
	for y := max(0, PlayerRow-n); y < PlayerRow; y++ {
		for x := range W {
			if g.Rows[y][x].Sweet == kind {
				return true
			}
		}
	}
	return false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
