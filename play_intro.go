package main

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The intro of a run: before READY, the character and the bunny she runs with get ready
// together over the whole window, so it is plain that the bunny is hers. It is her start
// picture (images/start, the shape of the window) when she has one, and otherwise her
// cut-in picture with the bunny hopping beside her. A press skips it.

const (
	introFrames = 120 // how long the intro lasts
	introFade   = 14  // its fade in and out
	introSkip   = 20  // a press skips it from this frame on (not the press that started the run)
)

// startIntro shows the intro before the run's READY.
func (s *PlayScene) startIntro() {
	s.intro = introFrames
	if e := s.startPicture(); e != nil {
		prefetchImgs([]*ImageEntry{e})
	}
	playSE(seStreak)
}

// startPicture is the character's start picture for the stages being played, or nil
// when she has none.
func (s *PlayScene) startPicture() *ImageEntry {
	e := s.char.Start
	if extraMode() {
		e = s.char.StartExtra
	}
	if e == nil || !e.HasImage() {
		return nil
	}
	return e
}

// updateIntro counts the intro down; it reports whether the intro is still on.
func (s *PlayScene) updateIntro(g *Game) bool {
	if s.intro == 0 {
		return false
	}
	if introFrames-s.intro >= introSkip && g.in.Pressed(ActConfirm) {
		s.intro = min(s.intro, introFade) // fade out quickly
	}
	s.intro--
	return true
}

// drawIntro draws the intro over the whole window.
func (s *PlayScene) drawIntro(screen *ebiten.Image) {
	if s.intro == 0 {
		return
	}
	t := introFrames - s.intro // frames since it began
	a := float32(math.Min(1, math.Min(float64(t)/introFade, float64(s.intro)/introFade)))
	if e := s.startPicture(); e != nil {
		if img := e.Full(); img != nil {
			drawImageCover(screen, img, 0, 0, ScreenW, ScreenH, a)
			s.drawIntroWords(screen, t, a)
			return
		}
	}
	// the cut-in: a pink window with speed lines streaming out of the middle
	vector.FillRect(screen, 0, 0, ScreenW, ScreenH, color.NRGBA{0xf7, 0x8f, 0xb3, uint8(0xf0 * a)}, false)
	cx, cy := ScreenW*0.5, ScreenH*0.45
	for i := range 28 {
		ang := float64(i)*2*math.Pi/28 + float64(t)*0.01
		r0 := 120 + math.Mod(float64(t*14+i*53), 260)
		r1 := r0 + 80 + float64(i*37%90)
		vector.StrokeLine(screen, float32(cx+math.Cos(ang)*r0), float32(cy+math.Sin(ang)*r0),
			float32(cx+math.Cos(ang)*r1), float32(cy+math.Sin(ang)*r1), 5, color.NRGBA{0xff, 0xff, 0xff, uint8(0x90 * a)}, false)
	}
	// she comes in from the left, the bunny hops in from the right
	in := 1 - math.Pow(1-math.Min(1, float64(t)/18), 3)
	if img := cutinImage(s.char); img != nil {
		iw, ih := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
		sc := math.Min(ScreenH*0.86/ih, ScreenW*0.9/iw)
		x := ScreenW*0.42 - iw*sc/2 - (1-in)*ScreenW*0.6
		drawImageScaled(screen, img, x, ScreenH-ih*sc, sc, a)
	}
	if pet := uiImage("pet"); pet != nil {
		pw, ph := float64(pet.Bounds().Dx()), float64(pet.Bounds().Dy())
		sc := ScreenH * 0.36 / ph
		hop := math.Abs(math.Sin(float64(t)*0.18)) * 36
		x := ScreenW*0.78 - pw*sc/2 + (1-in)*ScreenW*0.5
		drawImageScaled(screen, pet, x, ScreenH-ph*sc-24-hop, sc, a)
	}
	s.drawIntroWords(screen, t, a)
}

// drawIntroWords draws LET'S GO! across the top of the intro, popping in.
func (s *PlayScene) drawIntroWords(screen *ebiten.Image, t int, a float32) {
	pop := 1 + 0.25*math.Max(0, 1-float64(t)/12)
	outline := color.NRGBA{0x40, 0x30, 0x48, 0xff}
	drawTextOutlineColor(screen, "LET'S GO!", ScreenW/2, 70, 76*pop, color.White, outline, a)
}
