package main

import (
	"image/color"
	"math"
	"sort"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/road"
)

// Screen layout: the road on the left; on the right, a tall frame for the full-body
// character (as wide as possible, so she shows large). The lives, hammers and sweets sit
// in a row above the frame.
const (
	cell     = 46.0
	boardX   = 14.0
	boardY   = 70.0 // y of the top row of the road
	readyFr  = 90   // duration of READY / GO
	goFrames = 35   // the last frames of READY, which show GO

	// boardMidX is the middle of the road across.
	boardMidX = boardX + cell*BoardW/2
	// boardTextX is where READY / GO and the popups are centered: half a cell right of the
	// road's middle (it was the middle of an older, wider road). Moving them to boardMidX
	// would shift them on the screen, so they stay.
	boardTextX = boardX + cell*5

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
	shownPic       *ebiten.Image // the picture drawn last as the current pose (stands in while the next decodes)
	charScale      float64       // the character's fixed size in the frame (portraitScale)

	// Reading the road for the reactions: the danger level last frame, and whether the
	// "a big sweet is coming" look was shown for the sweet in sight.
	danger      int
	oneUpSeen   bool
	sinceSweet  int // rows since the last sweet picked up
	lastSteps   int
	droughtSeen bool

	popups []popup
	// comeback counts down after a retry; at zero the character switches to her comeback pose.
	comeback int

	overFrame int
	overSel   int

	fadeLayer, frameLayer *ebiten.Image

	// stageCG is the illustration behind the road: from the second course on, the newest
	// one earned (it stays until the next is earned). stageFade fades it in.
	stageCG   *ImageEntry
	stageFade float64
	// prevCG is the illustration before stageCG, kept under it while the new one fades in
	// (without it the plain road showed for a moment at every change)
	prevCG *ImageEntry
	// After a miss: missFrame counts the frames since it, missSel is the chosen button
	// (RETRY or GIVE UP), and countdown runs while the lives count ticks down on RETRY.
	missFrame int
	missSel   int
	countdown int
	// The hammer: the frames since the walls it blew away began to break (0: not breaking),
	// the frames left of its cut-in, and those walls (drawn breaking).
	crumble     int
	showHold    int  // frames the hammer show's blocks stand before the hammer (play_intro.go)
	showing     bool // the hammer show is on
	cutin       int
	hammerWalls [road.Rows + 1]road.Row
	// allClear: the last course is done; the run ends on the ending's picture with a
	// congratulation.
	allClear  bool
	endLayer  *ebiten.Image // the ending's portrait, when there is no illustration
	committed bool          // whether this run was recorded in the progress (commitRun)
	// artFor is the course (negative: the course a retry goes back to) whose next
	// illustration prefetchArt has started decoding; prefetched are the illustrations it
	// started, freed with the scene.
	artFor     int
	prefetched []*ImageEntry
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
	prefetchUI(playArtwork...)
	// Each run starts on the sweets background; illustrations appear as the run earns them.
	return s
}

// playArtwork is the artwork the play screen shows, decoded in the background when a run
// starts (prefetchUI): the frame of every family of expressions (moodBackground) and the
// pieces of the road.
var playArtwork = func() []string {
	names := make([]string, 0, 4+len(macaronColors)+len(moodBackground))
	names = append(names, "play", "board", "pet", "hammer")
	for _, c := range macaronColors {
		names = append(names, "macaron_"+c)
	}
	for f := range moodBackground {
		names = append(names, "frame_"+f)
	}
	sort.Strings(names)
	return names
}()

func (s *PlayScene) Update(g *Game) {
	s.frame++
	defer s.updateLean() // after everything that moves her this frame
	uploadPrefetched(s.portraits, 3)
	s.prefetchArt()
	s.updateEffects()
	s.updateMusic()
	bg.set(moodBackground[family(s.expr)])
	bg.setImage("play")

	if s.updateHammerShow() {
		return
	}
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
		case goFrames:
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
	// The cut-in of a hammer holds the road for a moment; then the walls break, row by row
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
		s.updateCrumble()
		s.updateExpression()
		return
	}
	accel := false
	if s.auto != nil {
		s.auto.step(e)
	} else {
		// Sideways moves are smooth: she slides while a direction is held, slowly at
		// first and faster the longer it is held (SlideSpeed).
		dir := g.in.Side()
		if dir != s.holdDir {
			s.holdDir, s.holdFrames = dir, 0
		}
		if dir != 0 {
			s.holdFrames++
			e.Move(float64(dir) * e.SlideSpeed(s.holdFrames) / 60)
		}
		if g.in.Pressed(ActConfirm) && s.cutin == 0 { // Space or Enter, the A button on a pad
			s.useHammer()
			if s.cutin > 0 {
				// the road holds from this frame on: a step now moved the sweets a row away
				// from the walls kept for the breaking
				s.handleEvents()
				s.updateExpression()
				return
			}
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
	picked, courses := 0, 0
	for _, ev := range e.Events {
		switch ev.Kind {
		case road.EventPick:
			picked++
			s.sinceSweet = 0
			s.droughtSeen = false
			s.landing = 1 // a little squish for each sweet
			switch {
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
			if ev.Sweet == road.SweetOneUp {
				playSE(seTreat)
				s.react(ExprTreat, 150, rankBig) // an extra life picked up off the road: the big moment
			} else {
				s.react(ExprExcited, 100, rankGood)
			}
		case road.EventMiss:
			if ev.Sweet == road.SweetOneUp {
				s.react(ExprOops, 90, rankGood) // the extra life got away
			}
		case road.EventNearMiss:
			s.react(ExprNervous, 50, rankHint)
		case road.EventCrash:
			playSE(seGameOver)
			if !e.Over() {
				s.react(ExprCrying, missFrames-5, rankPerfect) // a miss: she cries until the restart
			}
		case road.EventCourse:
			courses++
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
		case road.EventBomb, road.EventOver:
			// the game over is handled by onGameOver
		}
	}
	e.Events = e.Events[:0]
	if courses > 0 { // a new course is a level up: the road is faster now
		playSE(seLevelUp)
		s.react(ExprLevelUp, 60, rankHint)
	}
}

// drawLives shows, at the top right, the lives left as her face times the number, the
// hammers in stock (Space swings one), and
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
	item(hammerImage(), "x"+strconv.Itoa(g.Bombs))
	item(macaronImage(0), strconv.Itoa(g.Sweets)+"/"+strconv.Itoa(g.SweetsForLife())) // sweets toward the next life
	// the stage and course, left of the sweets
	label := "STAGE " + s.eng.Progress()
	tw := 21.0 * float64(len(label))
	drawTextOutline(screen, label, right-tw/2, 10, 30, candyPink)
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

func (s *PlayScene) onGameOver() {
	s.overFrame = 0
	stopBGM()
	playSE(seGameOver)
	s.commitRun()
	s.updateExpression()
}

// commitRun records how far this run got and how long it lasted (also called on quit).
func (s *PlayScene) commitRun() {
	if s.committed {
		return
	}
	s.committed = true
	s.prog.BestStage = max(s.prog.BestStage, s.eng.G.Stage)
	s.prog.PlaySeconds += s.eng.PlayFrames / 60
	markSave()
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

// ---- Drawing ----

func (s *PlayScene) Draw(screen *ebiten.Image) {
	s.drawBoard(screen)
	s.drawCharacter(screen)
	s.drawLives(screen)
	s.drawPopups(screen)
	s.drawCutin(screen)

	if s.ready > 0 && !s.showing {
		msg := "READY"
		if s.ready < goFrames {
			msg = "GO"
		}
		drawTextOutline(screen, msg, boardTextX, boardY+cell*8, 48, candyPink)
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
		// the pose before that may not be ready either (a montage flashes a new pose every
		// few frames while the portraits are still decoding): the picture shown last stays
		img = s.shownPic
	}
	if img == nil {
		img = s.char.Expression(s.exprID).Img()
	}
	s.shownPic = img
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
		drawPortrait(l, pic, fw, fh, sx, sy, dx, m.lift, alpha, m.gray, s.portraitScale(fh))
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
	cx := boardTextX
	for i, p := range s.popups {
		a := math.Min(1, float64(p.timer)/25)
		y := boardY + cell*6 + float64(i)*50 - float64(70-p.timer)*0.3
		drawText(screen, p.text, cx+2, y+2, 32, color.NRGBA{0xff, 0xff, 0xff, uint8(230 * a)})
		drawText(screen, p.text, cx, y, 32, color.NRGBA{candyPink.R, candyPink.G, candyPink.B, uint8(255 * a)})
	}
}
