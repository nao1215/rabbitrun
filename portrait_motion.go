package main

import (
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/sound"
)

// The character in the frame moves like a little animation instead of only swapping pictures:
//   - A reaction pops in: the new pose springs up from slightly small with a soft overshoot.
//     Calm pose changes still cross-fade.
//   - A big moment (the precious sweet, a stage cleared, the comeback) plays a quick montage:
//     the situation's poses flash by one after another, then the last one stays.
//   - Combos step through the combo poses in order, so a long combo looks like a sequence.
//   - In danger she changes pose more and more often, so the frame looks restless.
//   - A strong reaction starts with a short crouch, and each popping pose slides in a
//     little from alternating sides, like cuts in an animation.
//   - She squishes a little each time a sweet is picked up.
//   - She breathes slowly (quick, deeper breaths in danger), and in the cheerful moods she
//     bounces softly on the music's beat.
//   - After a game over her color drains slowly.

const (
	popFrames    = 16 // length of the pop-in of a reaction pose
	montageStep  = 20 // frames each pose of a montage is shown
	windupFrames = 5  // the crouch before a strong reaction
)

// popSlide is how far (pixels) a popping pose still is from its place at frame f: it
// comes in from the side like a cut in an animation.
func popSlide(f int) float64 {
	if f < 0 || f >= popFrames {
		return 0
	}
	t := float64(f) / popFrames
	return 18 * (1 - t) * (1 - t) * (1 - t)
}

// portraitMotion is the motion shared by the poses in the frame this frame.
type portraitMotion struct {
	sx, sy float64 // scale around the feet
	lift   float64 // pixels up (the hop)
	gray   float64 // 0 in color, 1 fully gray
}

// motion works out the breath, the beat bounce, the landing squish, the crouch before a
// strong reaction, the hop and the fading color after a game over.
func (s *PlayScene) motion() portraitMotion {
	// Calm breathing; quick, deeper breaths in danger.
	period, depth := 200.0, 0.006
	switch s.expr {
	case character.ExprPanic, character.ExprCrying:
		period, depth = 50, 0.01
	case character.ExprNervous:
		period, depth = 90, 0.008
	}
	breath := depth * math.Sin(float64(s.frame)*2*math.Pi/period)
	squash := s.bounce() + 0.025*s.landing
	if s.windup > 0 {
		squash += 0.05 * float64(windupFrames-s.windup+1) / windupFrames
	}
	m := portraitMotion{
		sx:   1 + squash*0.6,
		sy:   1 + breath - squash,
		lift: math.Abs(math.Sin(float64(s.frame)*0.25)) * 8 * s.hop,
	}
	if s.eng != nil && s.eng.Over() {
		m.gray = 0.55 * math.Min(1, float64(s.overFrame)/90) // the color drains slowly
	}
	return m
}

// popScale is the size of a popping pose at frame f: from 0.9 up past full size and back.
func popScale(f int) float64 {
	if f < 0 || f >= popFrames {
		return 1
	}
	t := float64(f) / popFrames
	// ease-out-back: overshoots a little before settling
	const c1 = 1.9
	u := t - 1
	e := 1 + (c1+1)*u*u*u + c1*u*u
	return 0.9 + 0.1*e
}

// poseInterval is how long one pose of a situation lasts before another is picked:
// the closer to the top, the more restless she gets.
func poseInterval(expr string) int {
	switch expr {
	case character.ExprGameOver:
		return 1 << 30 // she stays down
	case character.ExprCrying:
		return 120
	case character.ExprPanic:
		return 150
	case character.ExprNervous:
		return 200
	}
	return 300
}

// montageFor returns the poses of state in a random order, ending away from current.
func (s *PlayScene) montageFor(state string) []string {
	if s.char == nil {
		return nil
	}
	switch state {
	case character.ExprTreat, character.ExprCombo, character.ExprPerfect, character.ExprExcited, character.ExprComeback, character.ExprGreat:
	default:
		return nil // only the happy moments flash through their poses
	}
	vs := s.char.Variants(state)
	if len(vs) < 2 {
		return nil
	}
	ids := make([]string, len(vs))
	for i, v := range vs {
		ids[i] = v.ID
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] }) //nolint:gosec // G404: game randomness
	if ids[0] == s.exprID {
		ids[0], ids[len(ids)-1] = ids[len(ids)-1], ids[0]
	}
	return ids
}

// comboPose returns the combo pose for the n-th combo step, going through them in order.
func (s *PlayScene) comboPose(n int) string {
	vs := s.char.Variants(character.ExprCombo)
	return vs[max(0, n)%len(vs)].ID
}

// bounce returns the soft squash of the beat bounce (0 none, up to about 0.015), only in
// the cheerful moods while the music is lively.
func (s *PlayScene) bounce() float64 {
	if s.intensity == 0 {
		return 0
	}
	switch family(s.expr) {
	case character.ExprHappy, character.ExprExcited:
	default:
		return 0
	}
	ph, ok := beatPhase()
	if !ok {
		return 0
	}
	return 0.008 * float64(s.intensity) * math.Exp(-ph*7)
}

// beatPhase is where the music is within its beat (sound.BeatPhase); the tests stand in
// for the music with it.
var beatPhase = sound.BeatPhase

// drawPortrait draws img fitted into the w x h box of dst, scaled by sx and sy around the
// middle of its feet (so she squashes and springs from the floor), moved by dx and lifted
// by lift. gray drains the color (0 none, 1 fully gray).
func drawPortrait(dst, img *ebiten.Image, w, h, sx, sy, dx, lift float64, alpha float32, gray float64, scale float64) {
	drawPortraitFigure(dst, img, character.FigureOf(img), w, h, sx, sy, dx, lift, alpha, gray, scale)
}

// drawPortraitFigure is drawPortrait with the figure already measured (f, in img's
// coordinates), for pictures that are not on the GPU yet when they are measured.
func drawPortraitFigure(dst, img *ebiten.Image, f character.Figure, w, h, sx, sy, dx, lift float64, alpha float32, gray float64, scale float64) {
	// Every standing pose is drawn to the same height (portraitFill of the frame), measured
	// on her figure, so she does not grow or shrink as the pose changes. A crouching or
	// sitting pose is drawn at the character's standing size (scale, see portraitScale),
	// and larger if that would leave her small. Her body's middle goes to the middle of the
	// frame, moved aside if her hair or arms would stick out; only a pose too wide for the
	// frame altogether is drawn smaller.
	b := img.Bounds()
	ih := float64(b.Dy())
	figH := float64(f.Box.Dy())
	if f.Box.Empty() || figH == 0 {
		f.Box, f.BodyX, figH = b, b.Min.X+b.Dx()/2, ih
	}
	fit := scaleOr(scale, portraitFill*h/(standingHeight*ih))
	// standing: the figure takes most of the picture's height, or it is tall and narrow
	// (a standing figure drawn small in its picture is still a standing figure)
	if figH >= standingShare*ih || figH >= standingAspect*float64(f.Box.Dx()) {
		fit = portraitFill * h / figH // standing: the same height in every pose
	} else if figH*fit < minCrouch*portraitFill*h {
		fit = minCrouch * portraitFill * h / figH // crouching: never tiny
	}
	if bw := float64(f.Box.Dx()) * fit; bw > w*0.98 {
		fit *= w * 0.98 / bw // too wide even when moved aside
	}
	left := w/2 - float64(f.BodyX-f.Box.Min.X)*fit
	right := left + float64(f.Box.Dx())*fit
	shift := 0.0
	switch {
	case left < w*0.01:
		shift = w*0.01 - left
	case right > w*0.99:
		shift = w*0.99 - right
	}
	floor := (h + portraitFill*h) / 2 // where the feet stand
	var geo ebiten.GeoM
	geo.Translate(-float64(f.BodyX), -float64(f.Box.Max.Y)) // her feet, under the body's middle, at the origin
	geo.Scale(fit*sx, fit*sy)
	geo.Translate(w/2+shift+dx, floor-lift)
	if gray > 0 {
		var cm colorm.ColorM
		cm.ChangeHSV(0, 1-gray, 1)
		cm.Scale(1, 1, 1, float64(alpha))
		colorm.DrawImage(dst, img, cm, &colorm.DrawImageOptions{Filter: ebiten.FilterLinear, GeoM: geo})
		return
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, GeoM: geo}
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(img, op)
}

// portraitFill is the height of a standing figure in the frame, as a share of the frame.
const portraitFill = 0.86

// standingShare: a figure at least this share of its picture's height is standing.
const standingShare = 0.8

// standingAspect: a figure at least this many times as tall as it is wide is standing.
const standingAspect = 2.2

// minCrouch is the smallest height of a crouching figure, as a share of a standing one.
const minCrouch = 0.7

// standingHeight is the share of the picture height a standing figure takes (the
// generator places her at 95%).
const standingHeight = 0.95

func scaleOr(scale, fallback float64) float64 {
	if scale > 0 {
		return scale
	}
	return fallback
}

// portraitScale is the character's standing size in a frame h pixels tall (picture
// pixels to frame pixels) for the crouching poses: a figure of a typical standing height
// (standingHeight of the picture) fills portraitFill of the frame.
func (s *PlayScene) portraitScale(h float64) float64 {
	if s.charScale > 0 {
		return s.charScale
	}
	for i := range s.char.Expressions {
		if e := &s.char.Expressions[i]; e.HasImage() && e.Image != nil {
			s.charScale = portraitFill * h / (standingHeight * float64(e.Image.Bounds().Dy()))
			return s.charScale
		}
	}
	return 0
}

// ---- Loading the portraits ahead ----

// A portrait loads the first time it is shown, which takes a moment (decoding the PNG) and
// would stall a montage. Play decodes all of the character's portraits in the background
// (prefetchImgs) and uploads them a few a frame (uploadPrefetched).

// portraitEntries are the pictures play shows of c, in the order they are wanted: the
// usual pose (shown first), the cut-in of the hammer, the poses of the hammer show at the
// start of a run (blocked, then excited, which flashes through all its poses within a
// second), then every other pose. Decoding them all takes a second or two, so the show's
// poses come first or the show caught up with the decoding.
func portraitEntries(c *character.Character) []*character.ImageEntry {
	first := c.Expression(character.ExprNormal)
	out := []*character.ImageEntry{first, c.Cutin}
	seen := map[*character.ImageEntry]bool{first: true}
	for _, state := range []string{character.ExprBlocked, character.ExprExcited} {
		for _, e := range c.Variants(state) {
			if !seen[e] {
				seen[e] = true
				out = append(out, e)
			}
		}
	}
	for i := range c.Expressions {
		if e := &c.Expressions[i]; !seen[e] {
			out = append(out, e)
		}
	}
	return out
}

// releasePortraitsExcept frees the portraits (and cut-ins) of every character but c on the
// GPU: they load again when that character is played, instead of piling up as one
// character after another is played.
func releasePortraitsExcept(c *character.Character) {
	for _, o := range characters {
		if o == c {
			continue
		}
		keep := o.SelectEntry() // the select screen and the title keep showing it
		for _, e := range portraitEntries(o) {
			if e != keep {
				e.ReleaseImg()
			}
		}
	}
}
