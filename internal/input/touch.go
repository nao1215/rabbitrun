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
//   - A tap (down and up again without moving, quickly) is Confirm: it chooses in a menu
//     and swings the hammer on the road. While one finger steers, a tap with another
//     swings the hammer.
//   - The corner button the game shows (SetButton) is Escape: Cancel and Pause, so it
//     pauses the road, goes back from a menu and resumes from the pause menu.
//
// The positions are in the game's screen pixels (its Layout), as ebiten gives them.

const (
	// touchSlop is how far a finger may move and still tap.
	touchSlop = 24.0
	// touchDead is how far a drag must go before a direction is held.
	touchDead = 36.0
	// touchReach is how far the point a drag started from stays behind the finger.
	touchReach = 90.0
	// tapFrames is how long a tap may last (frames, at 60 a second).
	tapFrames = 24
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
}

// update takes this frame's fingers and works out the actions they hold.
func (t *touches) update(now []touchSample) {
	t.held = [NumActions]bool{}
	t.tapped = false
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
			t.held[Confirm] = true
			t.tapX, t.tapY, t.tapped = p.x, p.y, true
		}
		return true
	})
	for _, s := range now {
		i := slices.IndexFunc(t.ptrs, func(p pointer) bool { return p.id == s.id })
		if i < 0 {
			t.used = true
			on := image.Pt(int(s.x), int(s.y)).In(t.button)
			t.ptrs = append(t.ptrs, pointer{id: s.id, x0: s.x, y0: s.y, sx: s.x, sy: s.y, onButton: on})
			i = len(t.ptrs) - 1
		}
		p := &t.ptrs[i]
		p.x, p.y = s.x, s.y
		p.frames++
		if math.Hypot(p.x-p.sx, p.y-p.sy) > touchSlop {
			p.moved = true
		}
	}
	// the stick: the first finger down that is not on the button
	for i := range t.ptrs {
		p := &t.ptrs[i]
		if p.onButton {
			continue
		}
		p.x0 = follow(p.x0, p.x)
		p.y0 = follow(p.y0, p.y)
		dx, dy := p.x-p.x0, p.y-p.y0
		switch {
		case math.Abs(dx) >= math.Abs(dy) && dx <= -touchDead:
			t.held[Left] = true
		case math.Abs(dx) >= math.Abs(dy) && dx >= touchDead:
			t.held[Right] = true
		case math.Abs(dy) > math.Abs(dx) && dy <= -touchDead:
			t.held[Up] = true
		case math.Abs(dy) > math.Abs(dx) && dy >= touchDead:
			t.held[Down] = true
		}
		break
	}
}

// follow moves the anchor a of a drag toward the finger at x, so it stays at most
// touchReach behind.
func follow(a, x float64) float64 {
	switch {
	case x-a > touchReach:
		return x - touchReach
	case a-x > touchReach:
		return x + touchReach
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

// Touched reports whether the screen has been touched or clicked since the game started:
// the corner button is shown only then.
func (in *Input) Touched() bool { return in.touch.used }

// Tap is where a tap ended this frame (in screen pixels), and whether one did. The tap is
// also a press of Confirm; a screen that answers taps by where they are can take the
// place from here, and let go of Confirm (Release) when the tap was beside everything.
func (in *Input) Tap() (x, y float64, ok bool) {
	return in.touch.tapX, in.touch.tapY, in.touch.tapped
}
