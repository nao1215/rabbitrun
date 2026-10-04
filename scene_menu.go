package main

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/internal/sound"
)

// ---- Title ----

type TitleScene struct {
	sel     int
	frame   int
	command commandBuffer
	group   bool // the extra stages are on (the secret command): the group picture is the background
	// reveal is a secret character just unlocked and not yet shown (-1: none): the title
	// first brings her in, from grey to her colors, before the usual screen
	reveal      int
	revealFrame int
	revealLayer *ebiten.Image
	// word: the secret word is to be told (after the secret character's clear), once
	word      bool
	wordFrame int
}

var titleItems = []string{"PLAY", "GALLERY", "EXIT"}

func newTitleScene() *TitleScene {
	s := &TitleScene{group: extraMode(), reveal: newSecret(), word: wordDue()}
	prefetchImgs(selectEntries())
	return s
}

// newSecret is the index of the secret character unlocked and not yet announced (the
// rightmost when there are several), or -1.
func newSecret() int {
	n := -1
	for i, c := range characters {
		if c.Secret && !c.locked() && !store.Data.Announced[c.ID] {
			n = i
		}
	}
	return n
}

// markAnnounced records that the unlock of the character id has been announced.
func markAnnounced(id string) {
	if store.Data.Announced == nil {
		store.Data.Announced = map[string]bool{}
	}
	store.Data.Announced[id] = true
	store.Mark()
}

// The reveal of a new character on the title: she comes in grey, takes her colors over
// revealColorFrames from revealColorStart, and then the words come up and a press goes on.
const (
	revealColorStart  = 30
	revealColorFrames = 90
	revealWordsAt     = revealColorStart + revealColorFrames
	revealPortraitY   = 120
	revealPortraitH   = 800
)

// selectEntries are the pictures of the character select cards. The title decodes them in
// the background (decoding the five took the select screen's first frame 160ms).
func selectEntries() []*ImageEntry {
	out := make([]*ImageEntry, len(characters))
	for i, c := range characters {
		out[i] = c.selectEntry()
	}
	return out
}

func (s *TitleScene) Update(g *Game) {
	s.frame++
	uploadPrefetched(selectEntries(), 2)
	if sound.CurrentSong() != sound.TitleSong {
		sound.StartBGM(sound.TitleSong)
	}
	sound.SetBGMState(0)
	bg.set(popPink)
	bg.setImage("title")
	if titleComplete() {
		bg.setImage("title_complete") // every character cleared the extra stages
	}
	if s.reveal >= 0 {
		s.revealFrame++
		if s.revealFrame == revealWordsAt {
			sound.Play(sound.Unlock)
		}
		if s.revealFrame > revealWordsAt+20 && g.in.Pressed(ActConfirm) {
			sound.Play(sound.Confirm)
			markAnnounced(characters[s.reveal].ID)
			s.reveal = -1
		}
		return
	}
	if s.word {
		s.wordFrame++
		if s.wordFrame == 1 {
			sound.Play(sound.Unlock)
		}
		if s.wordFrame > wordWait && g.in.Pressed(ActConfirm) {
			sound.Play(sound.Confirm)
			store.Data.WordTold = true
			store.Mark()
			s.word = false
		}
		return
	}
	if s.command.feed(g.in.Chars()) {
		s.group = toggleExtra()
		s.sel = 0 // the letters typed also moved the menu
		sound.Play(sound.Unlock)
	}
	s.sel = g.in.menuNav(s.sel, len(titleItems), ActUp, ActDown)
	if g.in.Pressed(ActConfirm) {
		sound.Play(sound.Confirm)
		switch s.sel {
		case 0:
			g.SetScene(newCharSelectScene(modePlay))
		case 1:
			g.SetScene(newGalleryScene()) // the gallery skips character selection
		case 2:
			quitRequested = true
		}
	}
}

func (s *TitleScene) Draw(screen *ebiten.Image) {
	if s.reveal >= 0 {
		s.drawReveal(screen)
		return
	}
	if s.word {
		s.drawWord(screen)
		return
	}
	if s.group {
		// The group picture: a small logo at the top and the menu at the bottom, so the
		// text never covers the characters' faces.
		if titleComplete() {
			drawImageCover(screen, uiImage("title_complete"), 0, 0, ScreenW, ScreenH, 1)
		} else {
			screen.DrawImage(buildGroupPicture(groupMembers()), nil)
		}
		drawLogoLine(screen, "RABBIT RUN EXTRA", ScreenW/2, 14, 50)
		// the menu in the middle, on a soft band so it reads over the characters
		const menuY, menuSize = 380.0, 42.0
		vector.FillRect(screen, 0, menuY-26, ScreenW, menuSize*1.7*float32(len(titleItems))+30, color.NRGBA{0xff, 0xff, 0xff, 0x8c}, false)
		drawMenu(screen, titleItems, s.sel, menuY, menuSize)
		return
	}
	// Normally no characters here: only the logo and the menu over the background art.
	// The RUN line of the logo sits at the screen's vertical center.
	drawTitleLogo(screen, ScreenW/2, ScreenH/2, 1)
	drawMenu(screen, titleItems, s.sel, 600, 44)
}

// drawReveal brings the new character in on the title: her picture from grey to her
// colors, then the words, and a prompt to go on.
func (s *TitleScene) drawReveal(screen *ebiten.Image) {
	vector.FillRect(screen, 0, 0, ScreenW, ScreenH, color.NRGBA{0xff, 0xff, 0xff, 0x70}, false)
	c := characters[s.reveal]
	if img := c.SelectImage(); img != nil {
		if s.revealLayer == nil {
			s.revealLayer = ebiten.NewImage(ScreenW, revealPortraitH)
		}
		s.revealLayer.Clear()
		t := math.Min(1, math.Max(0, float64(s.revealFrame-revealColorStart)/revealColorFrames))
		a := float32(math.Min(1, float64(s.revealFrame)/20))
		drawPortrait(s.revealLayer, img, ScreenW, revealPortraitH, 1, 1, 0, 0, a, 1-t, 0)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, revealPortraitY)
		screen.DrawImage(s.revealLayer, op)
	}
	if s.revealFrame < revealWordsAt {
		return
	}
	a := float32(math.Min(1, float64(s.revealFrame-revealWordsAt)/20))
	outline := darkOutline
	drawTextOutlineColor(screen, "A NEW CHARACTER", ScreenW/2, 40, 50, candyPink, outline, a)
	drawTextOutlineColor(screen, "HAS COME!", ScreenW/2, 100, 50, candyPink, outline, a)
	if s.revealFrame > revealWordsAt+20 && (s.revealFrame/30)%2 == 0 {
		// under the words: at the bottom it fell on her feet once she was drawn larger
		drawTextOutlineColor(screen, "PRESS ENTER", ScreenW/2, 158, 30, color.White, outline, 1)
	}
}

// wordWait is how long the secret word stays before a press goes on (so a press meant for
// something before it does not skip it).
const wordWait = 40

// wordDue reports whether the title has the secret word to tell: the secret character has
// cleared the regular stages, and it has not been told yet.
func wordDue() bool {
	if store.Data.WordTold {
		return false
	}
	for _, c := range characters {
		if p := store.Data.Characters[c.ID]; c.Secret && p != nil && p.Cleared {
			return true
		}
	}
	return false
}

// drawWord tells the secret word on the title, in quotes, as it is to be typed.
func (s *TitleScene) drawWord(screen *ebiten.Image) {
	a := float32(math.Min(1, float64(s.wordFrame)/20))
	vector.FillRect(screen, 0, 0, ScreenW, ScreenH, color.NRGBA{0x20, 0x16, 0x2a, uint8(0xb0 * a)}, false)
	outline := darkOutline
	drawTextOutlineColor(screen, "THE SECRET WORD", ScreenW/2, 250, 46, color.White, outline, a)
	drawTextOutlineColor(screen, `"`+secretWord+`"`, ScreenW/2, 350, 64, candyPink, outline, a)
	drawTextOutlineColor(screen, "TYPE IT ON THE TITLE SCREEN", ScreenW/2, 470, 32, color.White, outline, a)
	drawTextOutlineColor(screen, "(IN CAPITALS)", ScreenW/2, 520, 28, color.White, outline, a)
	if s.wordFrame > wordWait && (s.wordFrame/30)%2 == 0 {
		drawTextOutlineColor(screen, "PRESS ENTER", ScreenW/2, 820, 34, color.White, outline, 1)
	}
}

// ---- Character select ----

const (
	modePlay = iota
	modeGallery
)

type CharSelectScene struct {
	mode     int
	sel      int
	frame    int
	cards    map[string]*ebiten.Image // card image per character (and lock state)
	pull     []float64                // how far each card is pulled out (0-1, animated smoothly)
	announce int                      // index of a newly unlocked secret character, or -1
}

// Seconds on the character select screen after which the music speeds up one stage.
const (
	selectGrooveSeconds = 15
	selectFullSeconds   = 30
)

// selectIntensity returns the music stage for the time spent on the select screen.
func selectIntensity(frames int) int {
	switch {
	case frames >= selectFullSeconds*60:
		return 2
	case frames >= selectGrooveSeconds*60:
		return 1
	}
	return 0
}

// heroID is the main character: selected first on the play select screen.
const heroID = "gyal"

// defaultCharIndex is the index of the main character (or the first one if missing).
func defaultCharIndex() int {
	for i, c := range characters {
		if c.ID == heroID {
			return i
		}
	}
	return 0
}

// newCharSelectScene starts with the gyaru selected, or with the secret character
// when it has just been unlocked (selected with a chime the first time only).
func newCharSelectScene(mode int) *CharSelectScene {
	s := &CharSelectScene{mode: mode, sel: defaultCharIndex(), announce: newSecret()}
	if mode == modePlay {
		// the play screen's artwork, so its first frame (the hammer show) only uploads it:
		// decoding it as play started held that frame for 30 to 50 ms
		prefetchUI(playArtwork...)
	}
	if s.announce >= 0 {
		s.sel = s.announce // the most recently opened secret wins (rightmost)
	}
	return s
}

// leave records that the newly unlocked secret character has been announced.
func (s *CharSelectScene) leave() {
	if s.announce >= 0 {
		markAnnounced(characters[s.announce].ID)
	}
}

func (s *CharSelectScene) Update(g *Game) {
	s.frame++
	if s.frame == 1 && s.announce >= 0 {
		sound.Play(sound.Unlock) // the newly unlocked character is already selected; a chime marks it
	}
	// The select screen has its own song (the road has another).
	if sound.CurrentSong() != sound.SelectSong {
		sound.StartBGM(sound.SelectSong)
	}
	// The longer the player stays on this screen, the faster and busier the music gets.
	sound.SetBGMState(selectIntensity(s.frame))
	bg.set(popYellow)
	bg.setImage("select")
	n := len(characters)
	s.sel = g.in.menuNav(s.sel, n, ActLeft, ActRight)
	if c := characters[s.sel]; s.mode == modePlay && !c.locked() {
		// her usual pose and the cut-in, decoded while she is chosen: play's first frame
		// waited 30 to 40 ms for the pose otherwise (the rest decode as play starts)
		prefetchImgs([]*ImageEntry{c.Expression(ExprNormal), c.Cutin})
	}
	if g.in.Pressed(ActCancel) {
		sound.Play(sound.Cancel)
		sound.StopBGM()
		s.leave()
		g.SetScene(newTitleScene())
		return
	}
	if g.in.Pressed(ActConfirm) {
		c := characters[s.sel]
		if c.locked() {
			sound.Play(sound.Denied)
			return
		}
		sound.Play(sound.Confirm)
		s.leave()
		sound.StopBGM() // play starts music after READY; the gallery plays sound effects only
		if s.mode == modePlay {
			g.SetScene(newRunScene(c))
		} else {
			g.SetScene(newGalleryScene())
		}
	}
}

// Cards are fanned out like a hand of cards. The pivot of the fan lies below the screen.
const (
	cardW, cardH  = 196.0, 436.0 // large cards, spread across the screen
	fanPivotY     = ScreenH + 430.0
	fanRadius     = 880.0             // from the pivot to the card center
	fanStep       = 0.08              // tilt per card (radians)
	cardPull      = 90.0              // distance the selected card is pulled up
	cardPullScale = 0.1               // extra scale applied to the selected card
	cardRes       = 1 + cardPullScale // cards are built at the selected size so the enlarged card stays sharp
	cardMargin    = 6.0               // gap kept between a card and the screen edge
	cardBorder    = 10.0
	cardCorner    = 20.0
)

// card builds and caches the character's card image (white border, background art, modest full-body portrait).
func (s *CharSelectScene) card(c *Character) *ebiten.Image {
	if s.cards == nil {
		s.cards = map[string]*ebiten.Image{}
	}
	locked := c.locked()
	key := c.ID
	if locked {
		key += "#locked"
	}
	if img, ok := s.cards[key]; ok {
		return img
	}
	w, h := cardW*cardRes, cardH*cardRes
	img := ebiten.NewImage(int(w), int(h))
	fillRoundRect(img, 0, 0, float32(w), float32(h), cardCorner*cardRes, panelFill)
	iw, ih := w-cardBorder*cardRes*2, h-cardBorder*cardRes*2
	inner := ebiten.NewImage(int(iw), int(ih))
	switch {
	case locked:
		// A pastel card with the character's soft silhouette.
		fillRoundRect(inner, 0, 0, float32(iw), float32(ih), 0, lockedCardFill)
		if c.selectEntry().HasImage() {
			drawSilhouette(inner, c.SelectImage(), 0, 0, iw, ih, false)
		}
	default:
		if bgImg := uiImage("frame_normal"); bgImg != nil {
			drawImageCover(inner, bgImg, 0, 0, iw, ih, 1)
		}
		drawImageFit(inner, c.SelectImage(), 0, 0, iw, ih, 1)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cardBorder*cardRes, cardBorder*cardRes)
	img.DrawImage(roundedMask(inner, (cardCorner-6)*cardRes), op)
	s.cards[key] = img
	return img
}

// lockedCardFill is the pastel background of a locked character's card.
var lockedCardFill = color.NRGBA{0xf3, 0xe4, 0xfb, 0xff}

func (s *CharSelectScene) Draw(screen *ebiten.Image) {
	n := len(characters)
	if len(s.pull) != n {
		s.pull = make([]float64, n)
	}
	for i := range s.pull {
		target := 0.0
		if i == s.sel {
			target = 1
		}
		s.pull[i] += (target - s.pull[i]) * 0.2
	}
	// Draw unselected cards first and the selected card last (frontmost).
	// The cards farthest from the selected one go under, the nearer ones on top of them,
	// and the selected card on top of all.
	order := make([]int, 0, n)
	for i := range n {
		if i != s.sel {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return abs(order[a]-s.sel) > abs(order[b]-s.sel) })
	order = append(order, s.sel)
	for _, i := range order {
		ang := (float64(i) - float64(n-1)/2) * fanStep
		sc := 0.86 + (0.14+cardPullScale)*s.pull[i] // the others a little smaller, so their faces show beside the selected one
		img := s.card(characters[i])
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(-cardW*cardRes/2, -cardH*cardRes/2)
		op.GeoM.Scale(sc/cardRes, sc/cardRes)
		op.GeoM.Rotate(ang)
		// The cards are spread evenly across the screen (tilted like a fan), so every face
		// shows beside the selected one, which is pulled straight up.
		cx := ScreenW / 2.0
		if n > 1 {
			cx = cardMargin + cardW/2 + float64(i)*(ScreenW-2*cardMargin-cardW)/float64(n-1)
		}
		cy := fanPivotY - math.Cos(ang)*fanRadius - cardPull*s.pull[i]
		// shadow
		sh := *op
		sh.GeoM.Translate(cx+10, cy+14)
		sh.ColorScale.Scale(0, 0, 0, 0.25)
		screen.DrawImage(img, &sh)
		op.GeoM.Translate(cx, cy)
		if i != s.sel {
			op.ColorScale.Scale(0.8, 0.8, 0.8, 1)
		}
		screen.DrawImage(img, op)
	}
}
