package game

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/nao1215/rabbitrun/internal/gfx"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
)

// Playing by touch (the browser on a phone) or with the mouse. The gestures themselves
// are in internal/input (a drag holds a direction, a tap confirms); here are the parts
// that depend on the screen: the corner button that stands for Escape, and the taps that
// choose what they land on (a menu item, a character's card, a gallery tile).

// cornerButton is what the button in the top left corner does on a screen, by its icon.
type cornerButton int

const (
	noButton     cornerButton = iota
	backButton                // back to the screen before (Cancel)
	pauseButton               // pause the road (Pause)
	resumeButton              // leave the pause menu (Cancel)
)

// buttonArea is where the corner button takes touches: the top left corner, which no
// screen fills with anything to press (the gallery's first card sits under its corner).
var buttonArea = image.Rect(0, 0, 80, 80)

// buttonCenter and buttonRadius place the button drawn in buttonArea.
const (
	buttonCenter = 36.0
	buttonRadius = 26.0
)

// buttonOf is the corner button the scene s shows.
func buttonOf(s Scene) cornerButton {
	if b, ok := s.(interface{ cornerButton() cornerButton }); ok {
		return b.cornerButton()
	}
	return noButton
}

func (s *charSelectScene) cornerButton() cornerButton { return backButton }

func (s *galleryScene) cornerButton() cornerButton { return backButton }

func (s *playScene) cornerButton() cornerButton {
	switch {
	case s.auto != nil, s.allClear, s.eng.Over(), s.showing, s.showHold > 0:
		return noButton
	case s.paused:
		return resumeButton
	case s.eng.G.Missed || s.countdown > 0:
		return noButton // the miss is a menu of its own, without a pause
	}
	return pauseButton
}

// placeButton tells the controls where the corner button is for the screen shown, so a
// touch there presses it.
func (g *Game) placeButton() {
	if g.cap != nil || g.rec != nil || buttonOf(g.scene) == noButton {
		g.in.SetButton(image.Rectangle{})
		return
	}
	g.in.SetButton(buttonArea)
}

// drawButton draws the corner button, once the screen has been touched or clicked: a
// keyboard or pad player never sees it.
func (g *Game) drawButton(screen *ebiten.Image) {
	b := buttonOf(g.scene)
	if b == noButton || !g.in.Touched() || g.cap != nil || g.rec != nil {
		return
	}
	const c, r = buttonCenter, buttonRadius
	vector.FillCircle(screen, c+2, c+3, r, shadowColor(), true)
	vector.FillCircle(screen, c, c, r, gfx.PanelFill, true)
	vector.StrokeCircle(screen, c, c, r, 3, gfx.CandyPink, true)
	ink := gfx.CandyPink
	switch b {
	case pauseButton:
		vector.FillRect(screen, c-9, c-10, 6, 20, ink, true)
		vector.FillRect(screen, c+3, c-10, 6, 20, ink, true)
	case resumeButton:
		var p vector.Path
		p.MoveTo(c-7, c-11)
		p.LineTo(c+11, c)
		p.LineTo(c-7, c+11)
		p.Close()
		vector.FillPath(screen, &p, &vector.FillOptions{}, &vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScale(ink)})
	case backButton:
		var p vector.Path
		p.MoveTo(c+5, c-11)
		p.LineTo(c-6, c)
		p.LineTo(c+5, c+11)
		vector.StrokePath(screen, &p, &vector.StrokeOptions{Width: 5, LineCap: vector.LineCapRound, LineJoin: vector.LineJoinRound},
			&vector.DrawPathOptions{AntiAlias: true, ColorScale: colorScale(ink)})
	case noButton:
	}
}

// colorScale is the color scale that paints a path in clr.
func colorScale(clr color.Color) ebiten.ColorScale {
	var cs ebiten.ColorScale
	cs.ScaleWithColor(clr)
	return cs
}

// shownMenu is the vertical menu drawn on the last frame (noteMenu), so a tap can find the
// item it lands on. n is 0 when no menu was drawn.
var shownMenu struct {
	n           int
	cx, y, size float64
}

// noteMenu records the menu of n items drawn as gfx.DrawMenuOn draws it: centered at cx,
// the first item at y, the items size*1.7 apart.
func noteMenu(n int, cx, y, size float64) {
	shownMenu.n, shownMenu.cx, shownMenu.y, shownMenu.size = n, cx, y, size
}

// menuItemAt is the item of the menu shown at (x, y), or -1.
func menuItemAt(x, y float64) int {
	m := shownMenu
	if m.n == 0 || math.Abs(x-m.cx) > max(240, m.size*5) {
		return -1
	}
	step := m.size * 1.7
	i := int(math.Floor((y - m.y + step*0.15) / step))
	if i < 0 || i >= m.n {
		return -1
	}
	return i
}

// tapMenu takes a tap on the menu of n items shown: a tap on an item chooses it (and its
// press of Confirm goes on to it), and a tap beside the items is let go of, so it does not
// choose the item that was selected. Without a tap, or with another menu shown, sel stays.
func tapMenu(in *input.Input, sel, n int) int {
	x, y, ok := in.Tap()
	if !ok || shownMenu.n != n {
		return sel
	}
	if i := menuItemAt(x, y); i >= 0 {
		return i
	}
	in.Release(input.Confirm)
	return sel
}

// cardX is the middle of the character select's card i of n, across: the cards are spread
// evenly across the screen.
func cardX(i, n int) float64 {
	if n < 2 {
		return ScreenW / 2.0
	}
	return cardMargin + cardW/2 + float64(i)*(ScreenW-2*cardMargin-cardW)/float64(n-1)
}

// tapCard takes a tap on the character select: a tap on the card chosen goes on with it
// (Confirm), and a tap on another card chooses that one instead. The cards are fanned out
// over each other, so the one tapped is the one whose middle is nearest across.
func tapCard(in *input.Input, sel, n int) int {
	x, _, ok := in.Tap()
	if !ok {
		return sel
	}
	near := sel
	for i := range n {
		if math.Abs(x-cardX(i, n)) < math.Abs(x-cardX(near, n)) {
			near = i
		}
	}
	if near != sel {
		in.Release(input.Confirm)
		sound.Play(sound.Move)
	}
	return near
}

// faceCardAt is the gallery's character card at (x, y) in its top strip, or -1.
func faceCardAt(x, y float64) int {
	n := len(characters)
	if y < 0 || y > gridTop-14 {
		return -1
	}
	for i := range n {
		left := faceCardLeft(i, n, 1)
		if x >= left-12 && x < left+headCardW+12 {
			return i
		}
	}
	return -1
}

// tileAt is the gallery's tile at (x, y) among n, or -1.
func (s *galleryScene) tileAt(x, y float64, n int) int {
	x0 := (ScreenW - (tileW*galleryCols + tileGap*(galleryCols-1))) / 2
	if y < gridTop || y > gridBottom || x < x0 {
		return -1
	}
	col := int((x - x0) / (tileW + tileGap))
	gy := y - gridTop + s.scrollView
	row := int(gy / (tileH + tileGap))
	if col >= galleryCols || x-x0-float64(col)*(tileW+tileGap) > tileW || gy-float64(row)*(tileH+tileGap) > tileH {
		return -1
	}
	if i := row*galleryCols + col; i < n {
		return i
	}
	return -1
}

// tap takes a tap on the gallery's grid of n tiles. A tap on a character's card at the
// top gives her index (ok) to show; one on a locked character only says no. A tap on a
// tile chooses it, and its press of Confirm opens it; a tap beside everything is let go of.
func (s *galleryScene) tap(in *input.Input, n int) (int, bool) {
	x, y, ok := in.Tap()
	if !ok {
		return 0, false
	}
	if c := faceCardAt(x, y); c >= 0 {
		in.Release(input.Confirm)
		switch {
		case locked(characters[c]):
			sound.Play(sound.Denied)
		case c != s.charIdx:
			return c, true
		}
		return 0, false
	}
	if i := s.tileAt(x, y, n); i >= 0 {
		s.sel = i
		return 0, false
	}
	in.Release(input.Confirm)
	return 0, false
}
