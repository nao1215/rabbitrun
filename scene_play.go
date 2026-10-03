package main

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/road"
)

// Screen layout: the road on the left; on the right, a tall frame for the full-body
// character (as wide as possible, so she shows large). The lives, stars and sweets sit
// in a row above the frame.
const (
	cell    = 46.0
	boardX  = 14.0
	boardY  = 70.0 // y of the top row of the road
	readyFr = 90   // duration of READY / GO

	rightX, rightW = boardX + cell*BoardW + 20, ScreenW - (boardX + cell*BoardW + 20) - 10

	frameBorder    = 8.0
	frameX, frameY = rightX, boardY - 8 // the character frame takes the whole right side
	frameW, frameH = rightW, boardY + cell*VisibleRows + 8 - frameY
)

type popup struct {
	text  string
	timer int
}

type PlayScene struct {
	prevX, lean float64 // the bunny's last x and how far she leans (smoothed)
	holdDir     int     // the direction held (-1, 0, 1)
	holdFrames  int     // frames it has been held
	char        *Character
	eng         *Engine
	prog        *CharProgress
	auto        *autoPlayer // when set, the game plays itself (demo)

	frame    int
	ready    int
	paused   bool
	pauseSel int

	// Character expression. expr / prevExpr are situations; exprID / prevID are the poses (portrait IDs) chosen within them
	expr, prevExpr string
	exprID, prevID string
	variantTimer   int
	repick         bool
	exprFade       float64
	reactTimer     int
	reactExpr      string
	reactRank      int
	hop            float64
	popFrame       int      // frames since the current pose popped in (-1: it cross-faded in)
	montage        []string // poses still to flash by in a montage
	montageTimer   int
	comboStep      int           // combo reactions so far in this chain (picks the next combo pose)
	popNext        bool          // the next pose change pops in (a reaction) instead of cross-fading
	windup         int           // frames left of the crouch before a strong reaction springs out
	slideDir       float64       // side the next popping pose slides in from (alternates)
	landing        float64       // squash when a sweet is picked up (1 just now, fades out)
	intensity      int           // music intensity stage (for the beat bounce)
	portraits      []*ImageEntry // the character's pictures, decoded in the background
	charScale      float64       // the character's fixed size in the frame (portraitScale)

	// Reading the road for the reactions: the danger level last frame, and whether the
	// "a big sweet is coming" look was shown for the sweet in sight.
	danger      int
	cakeSeen    bool
	sinceSweet  int // rows since the last sweet picked up
	lastSteps   int
	droughtSeen bool

	popups []popup
	// comeback counts down after a retry; at zero the character switches to her comeback pose.
	comeback int

	overFrame int
	overSel   int

	fadeLayer, frameLayer *ebiten.Image

	// Illustration shown behind the board (the newest unlocked one); switches when a new one unlocks
	// The stage background: from the second stage on, the illustration unlocked by the
	// stage just cleared (it stays for the whole stage). stageFade fades it in.
	stageCG   *ImageEntry
	stageFade float64
	// prevCG is the illustration before stageCG, kept under it while the new one fades in
	// (without it the plain board showed for a moment at every change)
	prevCG *ImageEntry
	// After a miss: missFrame counts the frames since it, missSel is the chosen button
	// (RETRY or GIVE UP), and countdown runs while the lives count ticks down on RETRY.
	missFrame int
	missSel   int
	countdown int
	// The bomb: frames left of its cut-in, the walls it blew away (drawn breaking), and the
	// frames since they began to break (0: not breaking).
	crumble   int
	cutin     int
	bombWalls [road.Rows + 1]road.Row
	// allClear: the last locked illustration was unlocked; the run ends on it with a
	// congratulation.
	allClear  bool
	endLayer  *ebiten.Image // the ending's portrait, when there is no illustration
	committed bool          // whether this run's score was added to the total
}

// comebackDelay is how long the retry keeps the collapsed game over pose before the
// character springs up into her "let's go!" pose (in frames, during READY).
const comebackDelay = 40

// updateComeback runs the retry performance during READY: the game over pose stays for
// comebackDelay frames, then the character springs up into her comeback pose.
func (s *PlayScene) updateComeback() {
	switch {
	case s.comeback > 1:
		s.comeback--
	case s.comeback == 1:
		s.comeback = 0
		s.react(ExprComeback, 150, rankBig) // rankBig also makes her hop
		s.updateExpression()
	case s.reactExpr == ExprComeback && s.reactTimer > 0:
		s.updateExpression()
	}
}

// newRetryScene starts a new run after a game over: the character is still down in her
// game over pose and gets back up with a fist pump while READY is shown.
func newRetryScene(c *Character) *PlayScene {
	s := newPlayScene(c)
	s.expr, s.exprID = ExprGameOver, s.pickVariant(ExprGameOver)
	s.prevExpr, s.prevID = s.expr, s.exprID
	s.comeback = comebackDelay
	return s
}

func newPlayScene(c *Character) *PlayScene {
	s := &PlayScene{
		char:  c,
		eng:   newGameEngine(c.ID),
		prog:  progress(c.ID),
		ready: readyFr,
		expr:  ExprNormal, prevExpr: ExprNormal, exprID: ExprNormal, prevID: ExprNormal, exprFade: 1,
		popFrame: -1,
	}
	releasePortraitsExcept(c)
	s.portraits = portraitEntries(c)
	prefetchImgs(s.portraits)
	// Each run starts on the sweets background; illustrations appear as the run earns them.
	return s
}

func (s *PlayScene) Update(g *Game) {
	s.frame++
	uploadPrefetched(s.portraits, 3)
	s.updateEffects()
	s.updateMusic()
	bg.set(moodBackground[family(s.expr)])
	bg.setImage("play")

	if s.allClear {
		s.updateGameOver(g)
		return
	}
	if s.eng.Over() {
		if s.expr != ExprGameOver {
			s.updateExpression() // the game can end without passing through onGameOver
		}
		s.updateGameOver(g)
		return
	}
	if s.paused {
		s.updatePause(g)
		return
	}
	// no pause on the miss screen: it is a menu of its own (the pause menu came up behind it)
	if g.in.Pressed(ActPause) && !s.eng.G.Missed && s.countdown == 0 {
		s.paused = true
		s.pauseSel = 0
		playSE(sePause)
		pauseBGM(true)
		return
	}
	if s.ready > 0 {
		switch s.ready {
		case readyFr:
			playSE(seReady)
		case 35:
			playSE(seGo)
		}
		s.updateComeback()
		s.ready--
		if s.ready == 0 {
			startBGM(gameSong)
		}
		return
	}

	e := s.eng
	// After a miss the road waits: RETRY uses a life (the count ticks down) and starts
	// the stage over; GIVE UP ends the game.
	if e.G.Missed || s.countdown > 0 {
		s.updateMiss(g)
		return
	}
	// The cut-in of a bomb holds the road for a moment; then the walls break, row by row
	// from the bottom up, each row with a crack.
	if s.cutin > 0 {
		s.cutin--
		if s.cutin == 0 {
			s.crumble = 1
		}
		s.updateExpression()
		return
	}
	if s.crumble > 0 {
		if k := (s.crumble - 1) / crumbleStep; (s.crumble-1)%crumbleStep == 0 && k <= road.Rows {
			if rowHasWall(s.bombWalls[road.Rows-k]) { // the row breaking now (rowBroken)
				playSE(seBreak)
			}
		}
		if s.crumble++; s.crumble > (road.Rows+1)*crumbleStep+crumbleFly {
			s.crumble = 0
		}
		s.updateExpression()
		return
	}
	accel := false
	if s.auto != nil {
		s.auto.step(e)
	} else {
		// Sideways moves are smooth: she slides while a direction is held, slowly at
		// first and faster the longer it is held (SlideSpeed).
		dir := boolInt(g.in.Held(ActRight)) - boolInt(g.in.Held(ActLeft))
		if dir != s.holdDir {
			s.holdDir, s.holdFrames = dir, 0
		}
		if dir != 0 {
			s.holdFrames++
			e.Move(float64(dir) * e.SlideSpeed(s.holdFrames) / 60)
		}
		if g.in.Pressed(ActConfirm) && s.cutin == 0 { // Space or Enter, the A button on a pad
			s.bomb()
		}
		accel = g.in.Held(ActUp) // speeds the road up for good
	}
	e.Tick(accel)
	s.handleEvents()
	s.readRoad()
	if e.Over() {
		s.onGameOver()
	}
	s.updateExpression()
}

func (s *PlayScene) handleEvents() {
	e := s.eng
	picked := 0
	for _, ev := range e.Events {
		switch ev.Kind {
		case road.EventPick:
			picked++
			s.sinceSweet = 0
			s.droughtSeen = false
			s.landing = 1 // a little squish for each sweet
			switch {
			case ev.Sweet == road.SweetCake:
				playSE(seTreat)
				s.react(ExprTreat, 150, rankBig) // the rare sweet: the big moment
			case ev.Streak >= 5 && ev.Streak%5 == 0:
				playSE(seStreak)
				s.comboStep = ev.Streak/5 - 1 // each five in a row shows the next combo pose
				s.react(ExprCombo, 120, rankCombo)
			case ev.Sweet == road.SweetMacaron:
				playSE(sePick)
				s.react(ExprGreat, 100, rankGood)
			default:
				playSE(sePick)
				s.react(ExprHappy, 80, rankSmall)
			}
		case road.EventOneUp:
			playSE(seLevelUp)
			s.popups = append(s.popups, popup{text: "1UP", timer: 60})
			s.react(ExprExcited, 100, rankGood)
		case road.EventMiss:
			if ev.Sweet == road.SweetCake {
				s.react(ExprOops, 90, rankGood) // the rare one got away
			}
		case road.EventNearMiss:
			s.react(ExprNervous, 50, rankHint)
		case road.EventCrash:
			playSE(seGameOver)
			if !e.Over() {
				s.react(ExprCrying, missFrames-5, rankPerfect) // a miss: she cries until the restart
			}
		case road.EventCourse:
			s.courseClear(e.G.Level - 1)
			if e.G.Bonus() {
				s.popups = append(s.popups, popup{text: "BONUS!", timer: 120})
			}
		case road.EventStageClear:
			s.popups = append(s.popups, popup{text: "STAGE " + strconv.Itoa(e.G.Stage), timer: 90})
			s.react(ExprPerfect, 180, rankPerfect)
		case road.EventAllClear:
			s.allClearNow()
		case road.EventBombGain:
			playSE(seConfirm)
		case road.EventRestart:
			s.react(ExprComeback, 150, rankBig) // back on her feet
		case road.EventLevelUp, road.EventBomb, road.EventOver:
			// level ups are counted by the Engine (one per course); the wall color shows the
			// course; the game over is handled by onGameOver
		}
	}
	e.Events = e.Events[:0]
	if e.LevelUps > 0 {
		e.LevelUps = 0
		playSE(seLevelUp)
		s.react(ExprLevelUp, 60, rankHint)
	}
}

// cutinFrames is how long the cut-in of a bomb lasts (the road waits meanwhile).
const cutinFrames = 70

// bomb sets off a bomb: the character cuts in big, and every wall on the screen bursts.
func (s *PlayScene) bomb() {
	g := s.eng.G
	s.bombWalls[0] = g.Ahead
	copy(s.bombWalls[1:], g.Rows[:])
	if !s.eng.UseBomb() {
		playSE(seDenied)
		return
	}
	playSE(seBomb)
	s.cutin = cutinFrames
	s.react(ExprExcited, 120, rankBig)
}

// drawCutin draws the cut-in of a bomb: a pastel band with speed lines sweeps across
// the screen, and the character bursts out of it (her head rises above the band), holds,
// and sweeps out to the left.
func (s *PlayScene) drawCutin(screen *ebiten.Image) {
	if s.cutin == 0 {
		return
	}
	t := 1 - float64(s.cutin)/cutinFrames // 0 -> 1
	var x float64
	switch {
	case t < 0.2:
		u := 1 - t/0.2
		x = u * u * ScreenW // in, easing out
	case t > 0.85:
		u := (t - 0.85) / 0.15
		x = -u * u * ScreenW // out to the left
	}
	dim := uint8(0x60 * math.Sin(math.Min(1, t*1.2)*math.Pi))
	dimScreen(screen, dim)
	const bandH = 360.0
	bandY := ScreenH*0.56 - bandH/2
	// the band: pink, white edges, and white speed lines streaming left
	vector.FillRect(screen, float32(x), float32(bandY-8), ScreenW, bandH+16, color.White, false)
	vector.FillRect(screen, float32(x), float32(bandY), ScreenW, bandH, color.NRGBA{0xf7, 0x8f, 0xb3, 0xf0}, false)
	for i := range 14 {
		ly := bandY + 14 + float64(i*37%int(bandH-28))
		length := 80 + float64(i*53%160)
		lx := math.Mod(float64(i*97)-float64(s.cutin)*28, ScreenW+length) // streaming left
		if lx < 0 {
			lx += ScreenW + length
		}
		vector.FillRect(screen, float32(x+lx-length), float32(ly), float32(length), 4, color.NRGBA{0xff, 0xff, 0xff, 0xb0}, false)
	}
	img := cutinImage(s.char)
	if img == nil {
		return
	}
	// her upper body stands on the bottom of the band and rises out of its top
	iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	h := bandH * 1.55
	sc := math.Min(h/ih, ScreenW*0.8/iw)
	drawImageScaled(screen, img, x+ScreenW*0.55-iw*sc/2, bandY+bandH-ih*sc, sc, 1)
	drawTextOutline(screen, "SMASH!", x+ScreenW*0.2, bandY+bandH-90, 64, candyPink)
}

// drawLives shows, at the top right (where the score used to be), the lives left as
// her SD figure times the number, the star candies in stock (Space sets one off), and
// the sweets gathered toward the next life.
func (s *PlayScene) drawLives(screen *ebiten.Image) {
	const size = 46.0
	right := rightX + rightW - 4
	item := func(img *ebiten.Image, label string) {
		tw := 26.0 * float64(len([]rune(label)))
		if img != nil {
			drawImageFit(screen, img, right-tw-size, 4, size, size, 1)
		}
		drawTextOutline(screen, label, right-tw/2, 8, 34, candyPink)
		right -= tw + size + 14
	}
	g := s.eng.G
	item(charFace(s.char), "x"+strconv.Itoa(g.Lives))
	item(bombImage(), "x"+strconv.Itoa(g.Bombs))
	item(macaronImage(0), strconv.Itoa(g.Sweets)+"/"+strconv.Itoa(g.SweetsForLife())) // sweets toward the next life
}

// bombImage is the picture of the bomb: a pop squeaky toy hammer that smashes the walls
// (assets/ui/hammer.png), shown on the road and in the stock.
func bombImage() *ebiten.Image { return uiImage("hammer") }

// cutinImage is the big picture of the bomb cut-in: images/cutin.png (no background, a
// "here I go!" pose), or the character select picture until it exists.
func cutinImage(c *Character) *ebiten.Image {
	if c.Cutin != nil && c.Cutin.HasImage() {
		return c.Cutin.Img()
	}
	return c.SelectImage()
}

// missFrames is how long MISS shows before the buttons take input.
const missFrames = 40

// countdownFrames is how long the lives count takes to tick down after RETRY.
const countdownFrames = 50

// missItems are the buttons after a miss.
var missItems = []string{"RETRY", "GIVE UP"}

// updateMiss runs the screen after a miss.
func (s *PlayScene) updateMiss(g *Game) {
	s.missFrame++
	s.updateExpression()
	if s.countdown > 0 {
		s.countdown--
		if s.countdown == countdownFrames/2 {
			playSE(seCancel) // the life goes
		}
		if s.countdown == 0 && s.eng.Restart() {
			s.missFrame = 0
			s.ready = readyFr
			s.popups = append(s.popups, popup{text: "FROM " + s.eng.Progress(), timer: 120})
			s.restartBackground()
			s.handleEvents()
		}
		return
	}
	if s.missFrame < missFrames {
		return
	}
	if s.auto != nil { // the demo always goes on
		s.countdown = countdownFrames
		return
	}
	if g.in.Repeat(ActUp) || g.in.Repeat(ActDown) {
		s.missSel = 1 - s.missSel
		playSE(seMove)
	}
	if g.in.Pressed(ActConfirm) {
		playSE(seConfirm)
		if s.missSel == 0 {
			s.countdown = countdownFrames
			return
		}
		s.eng.GiveUp()
		s.handleEvents()
		s.onGameOver()
	}
}

// restartBackground puts back the background the course she restarts on began with: the
// illustration of the course before it (the plain background on the first course).
func (s *PlayScene) restartBackground() {
	cgs := s.char.PlayCGs()
	n := unlockedAfter(s.eng.G.Level-1, len(cgs)) // earned before this course
	for i := n - 1; i >= 0; i-- {                 // the newest drawn yet (as courseClear)
		if cgs[i].HasImage() {
			s.setStageCG(&cgs[i])
			s.stageFade = 1 // (re)starting on a course: its picture is there at once, no fade from the plain board
			return
		}
	}
	if n == 0 && s.stageCG != nil {
		s.stageCG.ReleaseFull()
		s.stageCG = nil
		if s.prevCG != nil {
			s.prevCG.ReleaseFull()
			s.prevCG = nil
		}
	}
}

// roadSeed builds the roads: the same seed every game, so every course is the same road
// each time (it can be learned, and a retry runs the same road again).
const roadSeed = 20261002

// newGameEngine starts a game for the character id: her roads, her course themes, and
// the extra stages' tighter ones when they are being played.
func newGameEngine(id string) *Engine { return newRun(id, extraMode()) }

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
// minutes; longer games dragged). The
// illustrations are shared out over the first GameCourses-1 courses, one a course; the
// last course leads to the ending.
const GameCourses = MainCGCount + 1

// unlockedAfter is how many illustrations have been earned once n courses are cleared:
// all of them after the next-to-last course.
func unlockedAfter(n, illustrations int) int {
	if n <= 0 {
		return 0
	}
	return min(illustrations, (n*illustrations+GameCourses-2)/(GameCourses-1))
}

// courseClear runs when course n (counting from 1 over the whole game) is done: the
// illustrations earned by then are unlocked, and the newest becomes the background of
// the next course.
func (s *PlayScene) courseClear(n int) {
	cgs := s.char.PlayCGs()
	upto := unlockedAfter(n, len(cgs))
	if upto == 0 {
		return
	}
	for i := range upto {
		cg := &cgs[i]
		if !s.prog.UnlockedCG[cg.ID] {
			s.prog.UnlockedCG[cg.ID] = true
			writeSave()
			if cg.HasImage() {
				playSE(seUnlock)
			}
		}
	}
	// the newest illustration shows behind the road (the newest that is drawn yet: some
	// are still being made)
	for i := upto - 1; i >= 0; i-- {
		if cg := &cgs[i]; cg.HasImage() {
			s.setStageCG(cg)
			break
		}
	}
}

// allClearNow ends the run on the last illustration with a congratulation.
func (s *PlayScene) allClearNow() {
	s.allClear = true
	s.overFrame = 0
	s.commitScore()
	stopBGM()
	playSE(seUnlock)
	// the rewards (see secret.go) show on the title screen: a new character comes in, the
	// secret word is told, or the title gets its own picture
	if extraMode() {
		s.prog.ClearedExtra = true
	} else {
		s.prog.Cleared = true
	}
	writeSave()
}

func (s *PlayScene) setStageCG(cg *ImageEntry) {
	if cg == s.stageCG {
		return // the same picture: no fade again
	}
	if s.prevCG != nil && s.prevCG != cg {
		s.prevCG.ReleaseFull()
	}
	s.prevCG = s.stageCG
	s.stageCG, s.stageFade = cg, 0
}

// Reaction strengths: a weaker reaction never interrupts a stronger one that is still showing.
const (
	rankHint    = iota // a passing look: level up, a wall brushing past
	rankSmall          // a sweet
	rankGood           // a macaron, a lost rare sweet
	rankCombo          // five sweets in a row, out of danger
	rankBig            // the rare sweet, a crash
	rankPerfect        // (unused: kept for the order of the ranks)
)

func (s *PlayScene) react(expr string, frames, rank int) {
	if s.reactTimer > 0 && rank < s.reactRank {
		return
	}
	s.reactExpr = expr
	s.reactTimer = frames
	s.reactRank = rank
	s.repick = true
	s.popNext = true
	s.montage = nil
	if rank >= rankBig {
		s.montage = s.montageFor(expr) // the big moments flash through their poses
		s.montageTimer = 0
	}
	if rank >= rankCombo {
		s.hop = 1
		s.windup = windupFrames // crouch for a moment before springing into the reaction
	}
}

// readRoad reacts to the road ahead: relief when a narrow stretch is behind, a look
// of anticipation when the rare sweet comes into sight, and drought when no sweet has
// been picked up for a long way.
func (s *PlayScene) readRoad() {
	e := s.eng
	d := e.Danger()
	if s.danger >= 3 && d <= 1 {
		s.react(ExprRelief, 120, rankCombo) // through the narrow stretch
	}
	s.danger = d
	if e.G.SweetAhead(10, road.SweetCake) {
		if !s.cakeSeen {
			s.cakeSeen = true
			s.react(ExprWaiting, 80, rankHint) // the rare sweet is coming
		}
	} else {
		s.cakeSeen = false
	}
	if s.eng.Steps != s.lastSteps {
		s.sinceSweet += s.eng.Steps - s.lastSteps
		s.lastSteps = s.eng.Steps
	}
	if !s.droughtSeen && s.sinceSweet >= 60 {
		s.droughtSeen = true
		s.react(ExprDrought, 100, rankSmall)
	}
}

// updateExpression picks the expression from how tight the road is and recent events.
func (s *PlayScene) updateExpression() {
	// After a miss, while the player decides, she keeps one crying pose.
	if s.eng.G.Missed && s.expr == ExprCrying {
		if s.popFrame >= 0 {
			s.popFrame++
		}
		return
	}
	want := ExprNormal
	d := s.eng.Danger()
	switch {
	case s.eng.Over():
		want = ExprGameOver
	case s.reactTimer > 0 && (d < 3 || s.reactRank >= rankCombo):
		// Reactions win, except that small ones do not hide panic.
		want = s.reactExpr
	case d >= 4:
		want = ExprPanic // the narrowest road on the last life
	case d >= 3:
		want = ExprNervous
	case d >= 2:
		want = ExprWorried
	case d == 0 && s.eng.G.Lives == road.StartLives:
		want = ExprRelaxed
	}
	if s.reactTimer > 0 {
		s.reactTimer--
	}
	s.variantTimer++
	if s.popFrame >= 0 {
		s.popFrame++
	}
	// The crouch before a strong reaction: the current pose squashes, then the new one springs out.
	if s.windup > 0 && !s.eng.Over() {
		s.windup--
		return
	}
	s.windup = 0
	// A montage flashes its poses by, one every montageStep frames, while its reaction shows.
	if len(s.montage) > 0 {
		if want != s.reactExpr {
			s.montage = nil
		} else {
			if s.montageTimer%montageStep == 0 {
				id := s.montage[0]
				s.montage = s.montage[1:]
				s.setPose(want, id, true)
			}
			s.montageTimer++
			s.repick = false
			return
		}
	}
	// Re-pick a pose when the situation changes, on a new reaction, or after the same
	// situation lasts a while (shorter the tighter the road is)
	if want != s.expr || s.repick || s.variantTimer > poseInterval(want) {
		pop := s.popNext || (want != s.expr && reactionExpr(want))
		s.repick, s.popNext = false, false
		id := s.pickVariant(want)
		if want == ExprCombo {
			id = s.comboPose(s.comboStep)
		}
		s.setPose(want, id, pop)
	}
}

// reactionExpr reports whether expr is a reaction (as opposed to a mood of the road ahead).
func reactionExpr(expr string) bool {
	switch expr {
	case ExprNormal, ExprRelaxed, ExprWorried, ExprNervous, ExprPanic, ExprCrying:
		return false
	}
	return true
}

// setPose shows the pose id of the situation expr: popping in (pop) or cross-fading from
// the previous pose.
func (s *PlayScene) setPose(expr, id string, pop bool) {
	s.variantTimer = 0
	if id == s.exprID {
		s.expr = expr
		return // only one pose
	}
	s.prevExpr, s.prevID = s.expr, s.exprID
	s.expr, s.exprID = expr, id
	s.exprFade = 0
	s.popFrame = -1
	if pop {
		s.popFrame = 0
		s.slideDir = -s.slideDir
		if s.slideDir == 0 {
			s.slideDir = 1
		}
	}
	if !s.prog.SeenExpr[s.exprID] {
		s.prog.SeenExpr[s.exprID] = true
		writeSave()
	}
}

// updateMusic matches the music intensity to the situation, ramping up at high speed and in danger.
func (s *PlayScene) updateMusic() {
	lv, d := s.eng.Level(), s.eng.Danger()
	intensity := 0
	switch {
	case lv >= 8 || d >= 4:
		intensity = 2
	case lv >= 4 || d >= 3:
		intensity = 1
	}
	s.intensity = intensity
	setBGMTempo(intensity, playBPM(lv, s.eng.G.Profile.Speed*math.Max(1, s.eng.Boost)))
}

// playBPM is the tempo of the music on a course: it follows the speed of the road (speed
// is the road's profile times the player's boost), from 136 on the first course to about
// 180 when the road runs twice as fast (the last course).
func playBPM(level int, speed float64) float64 {
	r := RowsPerSecond(level) * speed / (RowsPerSecond(1) * roadProfile.Speed)
	return math.Max(136, math.Min(210, 136+44*(r-1)))
}

// pickVariant picks a random pose for state, avoiding the current pose when possible.
func (s *PlayScene) pickVariant(state string) string {
	vs := s.char.Variants(state)
	for range 4 {
		id := vs[rand.IntN(len(vs))].ID //nolint:gosec // G404: game randomness, not security sensitive
		if id != s.exprID {
			return id
		}
	}
	return vs[0].ID
}

func (s *PlayScene) onGameOver() {
	s.overFrame = 0
	stopBGM()
	playSE(seGameOver)
	s.commitScore()
	s.updateExpression()
}

// commitScore records how far this run got and how long it lasted (also called on quit).
func (s *PlayScene) commitScore() {
	if s.committed {
		return
	}
	s.committed = true
	s.prog.BestStage = max(s.prog.BestStage, s.eng.G.Stage)
	s.prog.PlaySeconds += s.eng.PlayFrames / 60
	writeSave()
}

func (s *PlayScene) updateEffects() {
	if s.exprFade < 1 {
		step := 0.06 // a calm cross-fade
		if s.popFrame >= 0 {
			step = 0.25 // a pop shows almost at once
		}
		s.exprFade = math.Min(1, s.exprFade+step)
	}
	if s.stageCG != nil {
		s.stageFade = math.Min(1, s.stageFade+1.0/60)
		if s.stageFade == 1 && s.prevCG != nil {
			s.prevCG.ReleaseFull()
			s.prevCG = nil
		}
	}

	s.hop *= 0.95
	s.landing *= 0.8
	alive := s.popups[:0]
	for _, p := range s.popups {
		p.timer--
		if p.timer > 0 {
			alive = append(alive, p)
		}
	}
	s.popups = alive
}

var pauseItems = []string{"CONTINUE", "RESET", itemTitle}
var overItems = []string{"RETRY", "SELECT", itemTitle}

// itemTitle is the button back to the title screen.
const itemTitle = "TITLE"

func (s *PlayScene) updatePause(g *Game) {
	if g.in.Repeat(ActUp) {
		s.pauseSel = (s.pauseSel + len(pauseItems) - 1) % len(pauseItems)
		playSE(seMove)
	}
	if g.in.Repeat(ActDown) {
		s.pauseSel = (s.pauseSel + 1) % len(pauseItems)
		playSE(seMove)
	}
	resume := g.in.Pressed(ActCancel) || g.in.Pressed(ActPause)
	if g.in.Pressed(ActConfirm) {
		playSE(seConfirm)
		switch s.pauseSel {
		case 0:
			resume = true
		case 1:
			stopBGM()
			s.commitScore()
			g.SetScene(newPlayScene(s.char))
			return
		case 2:
			stopBGM()
			s.commitScore()
			g.SetScene(newTitleScene())
			return
		}
	}
	if resume {
		s.paused = false
		pauseBGM(false)
	}
}

func (s *PlayScene) updateGameOver(g *Game) {
	s.overFrame++
	wait := 60
	if !s.allClear {
		wait = curtainStart + curtainFrames + 10 // the menu takes input once the curtain is down
	}
	if s.overFrame < wait {
		return
	}
	if s.allClear {
		// the ending has one way on: back to the title (where what the clear opened shows)
		if g.in.Pressed(ActConfirm) {
			playSE(seConfirm)
			g.SetScene(newTitleScene())
		}
		return
	}
	if g.in.Repeat(ActUp) {
		s.overSel = (s.overSel + len(overItems) - 1) % len(overItems)
		playSE(seMove)
	}
	if g.in.Repeat(ActDown) {
		s.overSel = (s.overSel + 1) % len(overItems)
		playSE(seMove)
	}
	if g.in.Pressed(ActConfirm) {
		playSE(seConfirm)
		switch s.overSel {
		case 0:
			g.SetScene(newRetryScene(s.char))
		case 1:
			g.SetScene(newCharSelectScene(modePlay))
		case 2:
			g.SetScene(newTitleScene())
		}
	}
}

// ---- Drawing ----

func (s *PlayScene) Draw(screen *ebiten.Image) {
	s.drawBoard(screen)
	s.drawCharacter(screen)
	s.drawLives(screen)
	s.drawPopups(screen)
	s.drawCutin(screen)

	if s.ready > 0 {
		msg := "READY"
		if s.ready < 35 {
			msg = "GO"
		}
		drawTextOutline(screen, msg, boardX+cell*5, boardY+cell*8, 48, candyPink)
	}
	if s.paused {
		dimScreen(screen, 0xb0)
		drawMenu(screen, pauseItems, s.pauseSel, 330, 44)
	}
	if s.eng.G.Missed {
		s.drawMiss(screen)
	}
	if s.allClear {
		s.drawAllClear(screen)
	} else if s.eng.Over() {
		s.drawGameOver(screen)
	}
}

func (s *PlayScene) drawBoard(screen *ebiten.Image) {
	e := s.eng
	bx, by := boardX, boardY
	w, h := float32(cell*BoardW), float32(cell*VisibleRows)
	// White-bordered board with a whitened background image
	fillRoundRect(screen, float32(bx), float32(by)+2, w+16, h+16, 18, shadowColor())
	fillRoundRect(screen, float32(bx)-8, float32(by)-8, w+16, h+16, 18, panelFill)
	// Board background: the unlocked illustration (character on transparent background) scaled to fill the board over the base image
	// (overflow may be cropped so the face does not look too small)
	if img := uiImage("board"); img != nil {
		drawImageCover(screen, img, bx, by, float64(w), float64(h), 1)
	}
	// After the first course, the latest illustration fills the frame behind the road.
	a := float32(0)
	if s.prevCG != nil {
		a = 1
		drawImageCoverTop(screen, s.prevCG.Full(), bx, by, float64(w), float64(h), 1)
	}
	if s.stageCG != nil {
		a = max(a, float32(s.stageFade))
		drawImageCoverTop(screen, s.stageCG.Full(), bx, by, float64(w), float64(h), float32(s.stageFade))
	}
	// Wash with white so the walls stay readable: the plain board a little more, an
	// illustration lightly (cgVeil), all the time, so the picture still shows clearly
	wash := float32(0x70)*(1-a) + float32(cgVeil)*a
	vector.FillRect(screen, float32(bx), float32(by), w, h, color.NRGBA{0xff, 0xff, 0xff, uint8(wash)}, false)

	// The road: gummy walls and sweets, moving down smoothly between steps. It is drawn
	// on a layer the size of the road's frame, so nothing pokes out of it.
	layer := offscreen(&s.fadeLayer)
	scroll := e.Scroll()
	if e.Over() {
		scroll = 0
	}
	// whole pixels only: at a fraction of a pixel the seams of the blocks shimmer as lines
	rowY := func(y int) float64 { return math.Round((float64(y) + scroll - 1) * cell) }
	g := e.G
	fade := float32(1)
	if e.Over() {
		fade = float32(math.Max(0.35, 1-float64(s.overFrame)/90))
	}
	// The walls are single gummy blocks, one per cell, like the old brick games.
	wallAt := func(x, y int) int8 {
		if x < 0 || x >= BoardW || y < 0 || y > VisibleRows {
			return 1 // beyond the screen: as good as a wall
		}
		w := g.Ahead[x].Wall // the row coming in at the top
		if y > 0 {
			w = g.Rows[y-1][x].Wall
		}
		if s.breaking() && !s.rowBroken(y) {
			w = s.bombWalls[y][x].Wall // the hammer's walls, not broken yet
		}
		return w
	}
	buried := func(x, y int) bool {
		return wallAt(x-1, y) != 0 && wallAt(x+1, y) != 0 && wallAt(x, y-1) != 0 && wallAt(x, y+1) != 0
	}
	// the cells of the walls left out get a soft white, so the illustration shows there
	// but it does not look like road
	for y := range VisibleRows + 1 {
		for x := range BoardW {
			if wallAt(x, y) != 0 && buried(x, y) {
				vector.FillRect(layer, float32(float64(x)*cell), float32(rowY(y)), cell, cell, color.NRGBA{0xff, 0xff, 0xff, 0x80}, false)
			}
		}
	}
	drawGummyGrid(layer, BoardW, VisibleRows+1, func(x, y int) gummyCell {
		if y > VisibleRows {
			return gummyCell{}
		}
		w := wallAt(x, y)
		if w == 0 {
			return gummyCell{}
		}
		// only the blocks that face the road are drawn: a wall inside walls on every side
		// is left out, so a thick wall shows as one row of blocks with the illustration
		// behind it (it is a wall all the same)
		if buried(x, y) {
			return gummyCell{}
		}
		return gummyCell{kind: Kind(w), id: int32(y*BoardW + x + 1), alpha: fade} //nolint:gosec // G115: at most 17*9 cells; each cell its own block
	}, 0, rowY(0), cell)
	if s.breaking() {
		s.drawShards(layer, rowY)
	}
	for y := -1; y < VisibleRows; y++ {
		r := g.Ahead
		if y >= 0 {
			r = g.Rows[y]
		}
		for x := range BoardW {
			if k := r[x].Sweet; k != road.SweetNone {
				drawSweet(layer, k, nearestWall(r, x, g.WallColor()), charFace(s.char), (float64(x)+0.5)*cell, rowY(y+1)+0.5*cell, cell, s.frame)
			}
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(bx, by)
	screen.DrawImage(layer, op)
	if s.ready > 0 && s.comeback == 0 && s.frame%2 == 0 {
		return
	}
	// The player: the bunny hopping up the road on her row. After a crash she is
	// see-through while walls pass through her.
	alpha := float32(1)
	if g.Safe > 0 {
		alpha = 0.45
	}
	lean := math.Max(-1, math.Min(1, (g.X-s.prevX)*12)) // which way and how fast she slides
	s.prevX = g.X
	s.lean += (lean - s.lean) * 0.2
	// She stands just below her row: a row that hits her comes in with its bottom edge
	// on the tips of her ears (a wall running into her stops the road as it touches her)
	// and slides down over her while it is hers.
	drawBunny(screen, bx+g.X*cell, by+(float64(road.PlayerRow)+0.75)*cell+bunnyHeight,
		float64(g.Distance)+s.eng.Scroll(), s.lean, alpha)
}

// bunnyHeight is how tall the bunny is drawn (a little over a row).
const bunnyHeight = cell * 1.25

// The bunny the player steers (assets/ui/pet.png, seen from behind). It is drawn in
// two parts, the ears and the body, so the ears can flop while she hops.
const (
	bunnyEarsBottom = 90 // the ears are the rows above this (they overlap the head a little)
	bunnyBodyTop    = 72 // the body is the rows from this down (drawn over the ear roots)
)

// drawBunny draws the bunny with her feet at (cx, footY). dist is how far the road
// has run (in rows): she hops once every two rows, squashing as she lands. lean (-1..1)
// tilts her toward the way she slides, and her ears trail behind.
func drawBunny(dst *ebiten.Image, cx, footY, dist, lean float64, alpha float32) {
	img := uiImage("pet")
	if img == nil {
		return
	}
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	sc := bunnyHeight / h
	p := math.Mod(dist/2, 1)           // 0 -> 1 over one hop
	up := math.Sin(math.Pi * p)        // 0 on the ground, 1 at the top
	squash := math.Pow(1-up, 6) * 0.14 // flattened as she lands
	stretch := up * 0.05               // a little taller in the air
	sx, sy := sc*(1+squash-stretch), sc*(1-squash+stretch)
	lift := up * cell * 0.22
	tilt := lean * 0.18
	// the shadow stays on the ground, smaller while she is up
	sw := w * sc * (0.75 - 0.25*up)
	drawEllipse(dst, cx, footY-2, sw/2, sw/7, color.NRGBA{0x60, 0x40, 0x50, 0x40}, alpha)
	// the body pivots at her feet; the ears at their roots
	body, ok1 := img.SubImage(image.Rect(0, bunnyBodyTop, int(w), int(h))).(*ebiten.Image)
	ears, ok2 := img.SubImage(image.Rect(0, 0, int(w), bunnyEarsBottom)).(*ebiten.Image)
	if !ok1 || !ok2 {
		return
	}
	// each part gets a soft rose outline first (a white bunny on a pale road would
	// melt into it), then the part itself
	place := func(part *ebiten.Image, top, pivotY, ang, px, py float64) {
		for i := range 9 {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(-w/2, top-pivotY)
			op.GeoM.Scale(sx, sy)
			op.GeoM.Rotate(ang)
			op.GeoM.Translate(px, py)
			op.Filter = ebiten.FilterLinear
			if i < 8 {
				a := float64(i) * math.Pi / 4
				op.GeoM.Translate(1.4*math.Cos(a), 1.4*math.Sin(a))
				op.ColorScale.Scale(0.95, 0.68, 0.8, 0.8)
			}
			op.ColorScale.ScaleAlpha(alpha)
			dst.DrawImage(part, op)
		}
	}
	feetX, feetY := cx, footY-lift
	// where the ear roots are once the body is tilted and scaled
	rootDY := (float64(bunnyEarsBottom) - 10 - h) * sy
	rootX := feetX - math.Sin(tilt)*rootDY
	rootY := feetY + math.Cos(tilt)*rootDY
	// ears: trail the slide and flop with the hop (back when rising, forward on landing)
	earAng := tilt - lean*0.25 + math.Cos(math.Pi*p)*0.10
	place(ears, 0, float64(bunnyEarsBottom)-10, earAng, rootX, rootY)
	place(body, float64(bunnyBodyTop), h, tilt, feetX, feetY)
}

// drawEllipse fills an axis-aligned ellipse (a soft shadow on the ground).
func drawEllipse(dst *ebiten.Image, cx, cy, rx, ry float64, clr color.NRGBA, alpha float32) {
	if ellipseImg == nil {
		ellipseImg = ebiten.NewImage(64, 64)
		vector.FillCircle(ellipseImg, 32, 32, 32, color.White, true)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-32, -32)
	op.GeoM.Scale(rx/32, ry/32)
	op.GeoM.Translate(cx, cy)
	op.ColorScale.ScaleWithColor(clr)
	op.ColorScale.ScaleAlpha(alpha)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(ellipseImg, op)
}

var ellipseImg *ebiten.Image

// offscreen prepares and clears a scratch image the size of the road's frame.
func offscreen(img **ebiten.Image) *ebiten.Image {
	if *img == nil {
		*img = ebiten.NewImage(int(cell*BoardW), int(cell*VisibleRows))
	}
	(*img).Clear()
	return *img
}

// drawSweet draws a sweet lying on the road, bobbing gently, centered at cx, cy. The
// extra life is the character's SD figure (sd) marked 1UP.
func drawSweet(dst *ebiten.Image, kind, wall int8, sd *ebiten.Image, cx, cy, cell float64, frame int) {
	bob := math.Sin(float64(frame)*0.12+cx) * cell * 0.04
	if kind == road.SweetOneUp {
		// her face with "1UP" on it, as in the old games; kept inside its cell, so it does
		// not seem to run into the walls beside it
		if sd != nil {
			size := cell * 0.78
			drawImageFit(dst, sd, cx-size/2, cy-size/2+bob-cell*0.1, size, size, 1)
		}
		drawTextOutline(dst, "1UP", cx, cy+cell*0.14+bob, cell*0.36, candyPink)
		return
	}
	if kind == road.SweetBomb {
		if img := bombImage(); img != nil {
			drawImageFit(dst, img, cx-cell*0.55, cy-cell*0.55+bob, cell*1.1, cell*1.1, 1)
		}
		return
	}
	// Every sweet is a macaron of the same size in one of three colors; the precious one
	// (worth three) glows softly and twinkles.
	img := macaronFor(int(cx/cell), wall)
	size := cell * 0.8
	if kind == road.SweetCake {
		pulse := 0.75 + 0.25*math.Sin(float64(frame)*0.15+cx)
		drawEllipse(dst, cx, cy+bob, cell*0.62, cell*0.55, color.NRGBA{0xff, 0xf3, 0xa0, 0xa0}, float32(pulse))
	}
	drawImageFit(dst, img, cx-size/2, cy-size/2+bob, size, size, 1)
}

// The colors of the macarons on the road (assets/ui/macaron_<color>.png).
const (
	macPink  = "pink"
	macMint  = "mint"
	macLemon = "lemon"
)

// macaronColors are the macaron colors, in the order they take turns.
var macaronColors = []string{macPink, macMint, macLemon}

// macaronClash is the macaron color that looks too much like a wall color (the candy
// colors of Kind: 1 soda, 2 lemon, 4 melon, 5 strawberry, 7 orange), so a sweet is never
// lost against the walls around it.
var macaronClash = map[int8]string{1: macMint, 2: macLemon, 4: macMint, 5: macPink, 7: macLemon}

// macaronImage is the macaron of the i-th color (the colors take turns).
func macaronImage(i int) *ebiten.Image {
	return uiImage("macaron_" + macaronColors[i%len(macaronColors)])
}

// macaronFor is the macaron for column i among walls of the color wall: the colors take
// turns across the road, leaving out the one that looks like the walls.
func macaronFor(i int, wall int8) *ebiten.Image {
	colors := make([]string, 0, len(macaronColors))
	for _, c := range macaronColors {
		if c != macaronClash[wall] {
			colors = append(colors, c)
		}
	}
	return uiImage("macaron_" + colors[i%len(colors)])
}

// cgVeil is how strongly the illustration behind the road is washed with white: light,
// so the picture shows, but enough for the walls and the sweets to read over it.
const cgVeil = 0x88

// nearestWall is the color of the wall nearest to column x in row r (the walls a sweet there
// is seen against; on a colorful course they differ block by block), or def without walls.
func nearestWall(r road.Row, x int, def int8) int8 {
	for d := 1; d < road.W; d++ {
		for _, nx := range []int{x - d, x + d} {
			if nx >= 0 && nx < road.W && r[nx].Wall != 0 {
				return r[nx].Wall
			}
		}
	}
	return def
}

// charFace is the character's face, cut from her usual portrait: shown with the lives, on
// the extra life item and on the miss screen (her own face reads better there than a
// small figure).
func charFace(c *Character) *ebiten.Image { return faceOf(c.Expression(ExprNormal)) }

// drawCharacter draws the per-expression background and the character image inside the lower-right frame.
func (s *PlayScene) drawCharacter(screen *ebiten.Image) {
	iw, ih := int(frameW-frameBorder*2), int(frameH-frameBorder*2)
	if s.frameLayer == nil {
		s.frameLayer = ebiten.NewImage(iw, ih)
	}
	l := s.frameLayer
	l.Clear()
	fw, fh := float64(iw), float64(ih)
	m := s.motion()
	// A pose still being decoded in the background is not waited for: the pose before it
	// stays until it is ready (only with nothing to show is it waited for).
	img := s.char.Expression(s.exprID).ImgReady()
	prev := s.char.Expression(s.prevID).ImgReady()
	if img == nil {
		img = prev
	}
	if img == nil {
		img = s.char.Expression(s.exprID).Img()
	}
	drawLayer := func(expr string, pic *ebiten.Image, alpha float32, cur bool) {
		if bgImg := uiImage("frame_" + family(expr)); bgImg != nil {
			drawImageCover(l, bgImg, 0, 0, fw, fh, alpha)
		}
		if pic == nil {
			return
		}
		sx, sy, dx := m.sx, m.sy, 0.0
		if cur {
			p := popScale(s.popFrame)
			sx, sy = sx*p, sy*p
			dx = s.slideDir * popSlide(s.popFrame)
		}
		drawPortrait(l, pic, fw, fh, sx, sy, dx, m.lift, alpha, m.gray, s.portraitScale(fw, fh))
	}
	if s.exprFade < 1 {
		drawLayer(s.prevExpr, prev, 1, false)
	}
	drawLayer(s.expr, img, float32(s.exprFade), true)

	fillRoundRect(screen, frameX+8, frameY+10, frameW, frameH, 22, shadowColor())
	fillRoundRect(screen, frameX, frameY, frameW, frameH, 22, panelFill)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(frameX+frameBorder, frameY+frameBorder)
	screen.DrawImage(roundedMask(l, 14), op)
}

func (s *PlayScene) drawPopups(screen *ebiten.Image) {
	cx := boardX + cell*5
	for i, p := range s.popups {
		a := math.Min(1, float64(p.timer)/25)
		y := boardY + cell*6 + float64(i)*50 - float64(70-p.timer)*0.3
		drawText(screen, p.text, cx+2, y+2, 32, color.NRGBA{0xff, 0xff, 0xff, uint8(230 * a)})
		drawText(screen, p.text, cx, y, 32, color.NRGBA{candyPink.R, candyPink.G, candyPink.B, uint8(255 * a)})
	}
}

// drawMiss shows MISS over the stopped road, and the lives left to start over with.
func (s *PlayScene) drawMiss(screen *ebiten.Image) {
	a := uint8(math.Min(1, float64(s.missFrame)/20) * 0x70)
	dimScreen(screen, a)
	if s.missFrame < 10 {
		return
	}
	drawTextOutline(screen, "MISS", boardX+cell*BoardW/2, boardY+cell*4, 64, candyPink)
	// The lives: on RETRY the number ticks down by one, dropping out and the new count
	// bouncing in.
	if img := charFace(s.char); img != nil {
		const size = 64.0
		cx, cy := boardX+cell*BoardW/2, boardY+cell*7
		drawImageFit(screen, img, cx-size*1.2, cy-size/2, size, size, 1)
		n := s.eng.G.Lives
		if s.countdown > 0 {
			t := 1 - float64(s.countdown)/countdownFrames // 0 -> 1
			if t < 0.5 {
				drawTextOutline(screen, "x"+strconv.Itoa(n), cx+size*0.4, cy-24+t*2*40, 48, candyPink)
			} else {
				u := (t - 0.5) * 2
				drawTextOutline(screen, "x"+strconv.Itoa(n-1), cx+size*0.4, cy-24-math.Sin(u*math.Pi)*16, 48, candyPink)
			}
		} else {
			drawTextOutline(screen, "x"+strconv.Itoa(n), cx+size*0.4, cy-24, 48, candyPink)
		}
	}
	if s.missFrame >= missFrames && s.countdown == 0 {
		drawTextOutline(screen, "RESTART AT "+s.eng.RestartProgress(), boardX+cell*BoardW/2, boardY+cell*8.9, 26, textMain)
		drawMenuAt(screen, missItems, s.missSel, boardX+cell*BoardW/2, boardY+cell*10, 40)
	}
}

// The box of the ending's portrait (when there is no illustration): between the words
// at the top and the menu at the bottom.
const (
	endPortraitY = 240
	endPortraitH = 440
)

// drawAllClear is the ending: the last illustration over the whole screen, and a congratulation.
func (s *PlayScene) drawAllClear(screen *ebiten.Image) {
	a := float32(math.Min(1, float64(s.overFrame)/60))
	dimScreen(screen, uint8(0xff*a))
	ending := s.char.Ending
	if extraMode() {
		ending = s.char.EndingExtra
	}
	if ending != nil && ending.HasImage() {
		// the ending's own picture, made the shape of the window: it fills it
		drawImageCover(screen, ending.Full(), 0, 0, ScreenW, ScreenH, a)
	} else if s.stageCG != nil {
		drawImageFit(screen, s.stageCG.Full(), 0, 0, ScreenW, ScreenH, a)
	} else if img := s.char.SelectImage(); img != nil {
		// no illustration drawn yet: she waves between the words and the menu instead,
		// all of her in view
		if s.endLayer == nil {
			s.endLayer = ebiten.NewImage(ScreenW, endPortraitH)
		}
		s.endLayer.Clear()
		drawPortrait(s.endLayer, img, ScreenW, endPortraitH, 1, 1, 0, 0, 1, 0, 0)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, endPortraitY)
		op.ColorScale.ScaleAlpha(a)
		screen.DrawImage(s.endLayer, op)
	}
	if s.overFrame < 90 {
		return
	}
	// just the congratulation in a thin band at the top and the way back to the title at
	// the bottom, so the picture (her face is near the top) stays in view
	bands := float32(math.Min(1, float64(s.overFrame-90)/20))
	vector.FillRect(screen, 0, 0, ScreenW, 66, color.NRGBA{0xff, 0xff, 0xff, uint8(0x90 * bands)}, false)
	vector.FillRect(screen, 0, ScreenH-74, ScreenW, 74, color.NRGBA{0xff, 0xff, 0xff, uint8(0x90 * bands)}, false)
	drawTextOutline(screen, "CONGRATULATIONS!", ScreenW/2, 10, 42, candyPink)
	drawMenu(screen, endItems, 0, ScreenH-62, 32)
}

// endItems is the ending's one button.
var endItems = []string{itemTitle}

func (s *PlayScene) drawGameOver(screen *ebiten.Image) {
	// The full game over (no life left, or given up): a black curtain comes down from the
	// top of the screen, unlike a miss, and the words come up on the dark.
	p := math.Min(1, math.Max(0, float64(s.overFrame-curtainStart)/curtainFrames))
	edge := float32(p * (ScreenH + curtainSoft))
	vector.FillRect(screen, 0, 0, ScreenW, max(0, edge-curtainSoft), color.NRGBA{0x20, 0x16, 0x2a, curtainAlpha}, false)
	for i := range int(curtainSoft) / 4 { // a soft lower edge
		y := edge - curtainSoft + float32(i*4)
		a := uint8(curtainAlpha * (1 - float64(i*4)/curtainSoft))
		vector.FillRect(screen, 0, y, ScreenW, 4, color.NRGBA{0x20, 0x16, 0x2a, a}, false)
	}
	if p < 1 {
		return
	}
	a := float32(math.Min(1, float64(s.overFrame-curtainStart-curtainFrames)/20))
	drawTextOutlineColor(screen, "GAME OVER", ScreenW/2, 240, 64, color.White, color.NRGBA{0x40, 0x30, 0x48, 0xff}, a)
	drawTextOutlineColor(screen, "STAGE "+s.eng.Progress(), ScreenW/2, 330, 44, candyPink, color.NRGBA{0x40, 0x30, 0x48, 0xff}, a)
	drawMenuOn(screen, overItems, s.overSel, ScreenW/2, 460, 40, color.NRGBA{0xee, 0xe6, 0xf2, 0xff}, color.NRGBA{0x40, 0x30, 0x48, 0xff})
}

// The curtain of the full game over: it starts curtainStart frames in, takes
// curtainFrames to come down, and has a soft lower edge curtainSoft pixels tall. It dims
// the road and the character rather than hiding them (curtainAlpha).
const (
	curtainAlpha  = 0xb0
	curtainStart  = 20
	curtainFrames = 60
	curtainSoft   = 80.0
)

// moodBackground maps each expression to a solid background color that reflects its mood.
var moodBackground = map[string]color.NRGBA{
	ExprNormal:   popMint,
	ExprHappy:    popYellow,
	ExprExcited:  popPink,
	ExprWorried:  popLavender,
	ExprPanic:    popOrange,
	ExprGameOver: popGray,
}

// moodFamily groups expressions into six families that share a background color and frame image.
var moodFamily = map[string]string{
	ExprRelaxed: ExprNormal, ExprGreat: ExprHappy, ExprTreat: ExprExcited, ExprCombo: ExprExcited,
	ExprPerfect: ExprExcited, ExprNervous: ExprWorried, ExprCrying: ExprPanic,
	ExprOops: ExprWorried, ExprBlocked: ExprWorried, ExprReady: ExprHappy, ExprWaiting: ExprExcited,
	ExprRelief: ExprNormal, ExprLevelUp: ExprHappy, ExprDrought: ExprWorried,
	ExprComeback: ExprExcited,
}

func family(expr string) string {
	if f, ok := moodFamily[expr]; ok {
		return f
	}
	return expr
}

// The walls break after the hammer's cut-in: a row every crumbleStep frames from the
// bottom up, its blocks flying apart as shards for crumbleFly frames.
const (
	crumbleStep = 3
	crumbleFly  = 18
)

// breaking reports whether the walls are breaking after the hammer.
func (s *PlayScene) breaking() bool { return s.crumble > 0 }

// rowBroken reports whether row y of the hammer's walls (0 is the row coming in at the
// top) has broken yet.
func (s *PlayScene) rowBroken(y int) bool { return s.crumble >= (road.Rows-y)*crumbleStep }

// rowHasWall reports whether a row has any wall in it.
func rowHasWall(r road.Row) bool {
	for _, c := range r {
		if c.Wall != 0 {
			return true
		}
	}
	return false
}

// drawShards draws the shards of the rows breaking: each block bursts into four pieces
// that fly apart, fall and fade.
func (s *PlayScene) drawShards(dst *ebiten.Image, rowY func(int) float64) {
	for y := 0; y <= road.Rows; y++ {
		t := float64(s.crumble-(road.Rows-y)*crumbleStep) / crumbleFly // 0 -> 1 after the row breaks
		if t < 0 || t > 1 {
			continue
		}
		for x, c := range s.bombWalls[y] {
			if c.Wall == 0 {
				continue
			}
			col := kindColors[Kind(c.Wall)]
			cx, cy := (float64(x)+0.5)*cell, rowY(y)+0.5*cell
			for k := range 4 {
				dx, dy := float64(k%2)*2-1, float64(k/2)*2-1
				px := cx + dx*(cell*0.25+t*cell*0.9)
				py := cy + dy*cell*0.25 - t*cell*0.6 + t*t*cell*1.6 // up a little, then falling
				size := float32(cell * 0.42 * (1 - 0.5*t))
				a := uint8(255 * (1 - t))
				vector.FillRect(dst, float32(px)-size/2, float32(py)-size/2, size, size, color.NRGBA{col.R, col.G, col.B, a}, false)
			}
		}
	}
}
