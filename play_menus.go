package main

import (
	"image/color"
	"math"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The screens over the road: the pause menu, the miss, the game over and the ending.

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

var pauseItems = []string{"CONTINUE", "RESET", itemTitle}
var overItems = []string{"RETRY", "SELECT", itemTitle}

// itemTitle is the button back to the title screen.
const itemTitle = "TITLE"

func (s *PlayScene) updatePause(g *Game) {
	s.pauseSel = g.in.menuNav(s.pauseSel, len(pauseItems), ActUp, ActDown)
	resume := g.in.Pressed(ActCancel) || g.in.Pressed(ActPause)
	if g.in.Pressed(ActConfirm) {
		playSE(seConfirm)
		switch s.pauseSel {
		case 0:
			resume = true
		case 1:
			stopBGM()
			s.commitRun()
			g.SetScene(newPlayScene(s.char))
			return
		case 2:
			stopBGM()
			s.commitRun()
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
	wait := endInputAt
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
	s.overSel = g.in.menuNav(s.overSel, len(overItems), ActUp, ActDown)
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

// drawMiss shows MISS over the stopped road, and the lives left to start over with.
func (s *PlayScene) drawMiss(screen *ebiten.Image) {
	a := uint8(math.Min(1, float64(s.missFrame)/20) * 0x70)
	dimScreen(screen, a)
	if s.missFrame < 10 {
		return
	}
	drawTextOutline(screen, "MISS", boardMidX, boardY+cell*4, 64, candyPink)
	// The lives: on RETRY the number ticks down by one, dropping out and the new count
	// bouncing in.
	if img := charFace(s.char); img != nil {
		const size = 64.0
		cx, cy := boardMidX, boardY+cell*7
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
		drawTextOutline(screen, "RESTART AT "+s.eng.RestartProgress(), boardMidX, boardY+cell*8.9, 26, textMain)
		drawMenuAt(screen, missItems, s.missSel, boardMidX, boardY+cell*10, 40)
	}
}

// The ending's timing: the picture fades in over endFadeFrames, the menu takes input from
// endInputAt, and the words come up from endWordsAt (over endBandFrames).
const (
	endFadeFrames = 60
	endInputAt    = 60
	endWordsAt    = 90
	endBandFrames = 20
)

// The box of the ending's portrait (when there is no illustration): between the words
// at the top and the menu at the bottom.
const (
	endPortraitY = 240
	endPortraitH = 440
)

// drawAllClear is the ending: the last illustration over the whole screen, and a congratulation.
func (s *PlayScene) drawAllClear(screen *ebiten.Image) {
	a := float32(math.Min(1, float64(s.overFrame)/endFadeFrames))
	dimScreen(screen, uint8(0xff*a))
	if ending := s.ending(); ending != nil && ending.HasImage() {
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
	if s.overFrame < endWordsAt {
		return
	}
	// just the congratulation in a thin band at the top and the way back to the title at
	// the bottom, so the picture (her face is near the top) stays in view
	bands := float32(math.Min(1, float64(s.overFrame-endWordsAt)/endBandFrames))
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
