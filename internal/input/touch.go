package input

import (
	"image"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

// Touch controls, for phones (the browser build) and, the same way, the mouse's left
// button. The screen has no buttons of its own; the gestures stand in for the keys:
//
//   - A drag is a stick that floats where the finger went down: past touchDead the
//     direction it points to (the longer of across and along) is held, as long as the
//     finger stays there. A short swipe is one press, in a menu one step. On the road a
//     drag left or right steers, and a drag up speeds up. The point the finger went down
//     follows it when it goes further than touchReach, so turning back needs only a
//     short way.
//   - A tap (down and up again without moving, quickly) is Confirm: it chooses in a menu.
//     Two taps close together in time and place are a double tap (DoubleTap), which
//     swings the hammer on the road; a single tap there does nothing, as a stray touch
//     while steering threw hammers away. While one finger steers, another double taps.
//   - The corner button the game shows (SetButton) is Escape: Cancel and Pause, so it
//     pauses the road, goes back from a menu and resumes from the pause menu.
//
// The positions are in the game's screen pixels (its Layout), as ebiten gives them. The
// distances below are in device-independent pixels (CSS pixels in a browser), the unit
// phones' own gestures are measured in, and are turned into screen pixels by the scale
// the screen is shown at (SetPointScale): on a phone a screen pixel is about half a
// device-independent pixel, and the first distances, set in screen pixels, wanted a drag
// twice as long as a phone's before the bunny moved.

const (
	// touchSlop is how far a finger may move and still tap (Android's touch slop is 8).
	touchSlop = 10.0
	// touchDead is how far a drag must go before a direction is held.
	touchDead = 12.0
	// touchReach is how far the point a drag started from stays behind the finger.
	touchReach = 48.0
	// tapFrames is how long a tap may last (frames, at 60 a second).
	tapFrames = 24
	// doubleTapFrames is how soon the second tap of a double tap must go down after the
	// first came up (Android's double tap timeout is 300ms).
	doubleTapFrames = 18
	// doubleTapSlop is how far apart the two taps of a double tap may be (Android's is
	// 100).
	doubleTapSlop = 100.0
	// mouseID stands for the mouse's left button among the touches.
	mouseID = math.MinInt32
	// mouseQuiet is how long the mouse is not read after a finger was on the screen
	// (frames): a browser also reports a touch as the mouse, a little after it, and that
	// second finger steered once more where the swipe had started.
	mouseQuiet = 60
)

// pointer is a finger on the screen, or the mouse's left button.
type pointer struct {
	id       int
	x0, y0   float64 // where the stick is anchored (where it went down, then following)
	sx, sy   float64 // where it went down
	x, y     float64 // where it is now
	frames   int     // frames it has been down
	down     int     // the frame it went down (touches.frame)
	moved    bool    // it went further than touchSlop from where it went down
	onButton bool    // it went down on the corner button
}

// touchSample is where a finger (or the mouse) is this frame.
type touchSample struct {
	id   int
	x, y float64
}

// touches follows the fingers from frame to frame and turns them into actions.
type touches struct {
	ptrs []pointer
	// button is the corner button's area; empty when no button is shown.
	button image.Rectangle
	// used is set once the screen has been touched (or clicked): the game shows its
	// corner button only then, so a keyboard or pad player never sees it.
	used bool
	// held are the actions the fingers hold this frame; tapX, tapY is where a tap ended
	// this frame (tapped).
	held       [NumActions]bool
	tapX, tapY float64
	tapped     bool
	// doubled is set on the frame a tap ends that makes a double tap with the one before
	// (lastTap, lastX, lastY: the frame the last single tap ended, and where).
	doubled      bool
	lastTap      int
	lastX, lastY float64
	// frame counts the frames update has seen.
	frame int
	// scale is how many screen pixels a device-independent pixel is (SetPointScale); 0
	// is taken as 1.
	scale float64
}

// px is d device-independent pixels in screen pixels.
func (t *touches) px(d float64) float64 {
	if t.scale > 0 {
		return d * t.scale
	}
	return d
}

// tap records a tap that went down on the frame down and ended at (x, y) now: a press
// of Confirm, and a double tap when it follows the last one soon and near enough.
func (t *touches) tap(x, y float64, down int) {
	t.held[Confirm] = true
	t.tapX, t.tapY, t.tapped = x, y, true
	if t.lastTap > 0 && down-t.lastTap <= doubleTapFrames && math.Hypot(x-t.lastX, y-t.lastY) <= t.px(doubleTapSlop) {
		t.doubled, t.lastTap = true, 0 // a third tap starts over
		return
	}
	t.lastTap, t.lastX, t.lastY = t.frame, x, y
}

// update takes this frame's fingers and works out the actions they hold.
func (t *touches) update(now []touchSample) {
	t.held = [NumActions]bool{}
	t.tapped, t.doubled = false, false
	t.frame++
	// fingers lifted since the last frame: a tap, or a press of the button
	t.ptrs = slices.DeleteFunc(t.ptrs, func(p pointer) bool {
		if slices.ContainsFunc(now, func(s touchSample) bool { return s.id == p.id }) {
			return false
		}
		switch {
		case p.onButton:
			if !p.moved || image.Pt(int(p.x), int(p.y)).In(t.button) {
				t.held[Cancel], t.held[Pause] = true, true
			}
		case !p.moved && p.frames <= tapFrames:
			t.tap(p.x, p.y, p.down)
		}
		return true
	})
	for _, s := range now {
		i := slices.IndexFunc(t.ptrs, func(p pointer) bool { return p.id == s.id })
		if i < 0 {
			t.used = true
			on := image.Pt(int(s.x), int(s.y)).In(t.button)
			t.ptrs = append(t.ptrs, pointer{id: s.id, x0: s.x, y0: s.y, sx: s.x, sy: s.y, down: t.frame, onButton: on})
			i = len(t.ptrs) - 1
		}
		p := &t.ptrs[i]
		p.x, p.y = s.x, s.y
		p.frames++
		if math.Hypot(p.x-p.sx, p.y-p.sy) > t.px(touchSlop) {
			p.moved = true
		}
	}
	// the stick: the first finger down that is not on the button
	for i := range t.ptrs {
		p := &t.ptrs[i]
		if p.onButton {
			continue
		}
		reach, dead := t.px(touchReach), t.px(touchDead)
		p.x0 = follow(p.x0, p.x, reach)
		p.y0 = follow(p.y0, p.y, reach)
		dx, dy := p.x-p.x0, p.y-p.y0
		switch {
		case math.Abs(dx) >= math.Abs(dy) && dx <= -dead:
			t.held[Left] = true
		case math.Abs(dx) >= math.Abs(dy) && dx >= dead:
			t.held[Right] = true
		case math.Abs(dy) > math.Abs(dx) && dy <= -dead:
			t.held[Up] = true
		case math.Abs(dy) > math.Abs(dx) && dy >= dead:
			t.held[Down] = true
		}
		break
	}
}

// follow moves the anchor a of a drag toward the finger at x, so it stays at most reach
// behind.
func follow(a, x, reach float64) float64 {
	switch {
	case x-a > reach:
		return x - reach
	case a-x > reach:
		return x + reach
	}
	return a
}

// readTouches gives the fingers on the screen this frame, and the mouse's left button
// unless a finger was on the screen lately (mouseQuiet).
func (in *Input) readTouches() []touchSample {
	in.touchIDs = ebiten.AppendTouchIDs(in.touchIDs[:0])
	now := make([]touchSample, 0, len(in.touchIDs)+1)
	for _, id := range in.touchIDs {
		x, y := ebiten.TouchPosition(id)
		now = append(now, touchSample{id: int(id), x: float64(x), y: float64(y)})
	}
	if len(now) > 0 {
		in.mouseWait = mouseQuiet
	} else if in.mouseWait > 0 {
		in.mouseWait--
	}
	if in.mouseWait == 0 && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		now = append(now, touchSample{id: mouseID, x: float64(x), y: float64(y)})
	}
	return now
}

// SetButton places the corner button the game shows at r (in screen pixels); an empty r
// takes it away. A finger that goes down on it and comes up there presses Escape (Cancel
// and Pause) instead of tapping.
func (in *Input) SetButton(r image.Rectangle) { in.touch.button = r }

// SetPointScale tells how many screen pixels a device-independent pixel is: the screen
// is shown at 1/scale of its size (Game.Layout knows the size of the window).
func (in *Input) SetPointScale(scale float64) { in.touch.scale = scale }

// DoubleTap reports whether a double tap ended this frame (its second tap is also a Tap
// and a press of Confirm).
func (in *Input) DoubleTap() bool { return in.touch.doubled }

// Touched reports whether the screen has been touched or clicked since the game started:
// the corner button is shown only then.
func (in *Input) Touched() bool { return in.touch.used }

// Tap is where a tap ended this frame (in screen pixels), and whether one did. The tap is
// also a press of Confirm; a screen that answers taps by where they are can take the
// place from here, and let go of Confirm (Release) when the tap was beside everything.
func (in *Input) Tap() (x, y float64, ok bool) {
	return in.touch.tapX, in.touch.tapY, in.touch.tapped
}
