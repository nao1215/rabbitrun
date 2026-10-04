// Package input reads the player's controls: the keyboard (arrows, WASD and the vi keys)
// and gamepads, as the actions the game knows. A Script can stand in for the player.
package input

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// Action is something the player can do, from any key or button mapped to it.
type Action int

// The actions.
const (
	Left Action = iota
	Right
	Up
	Down
	Confirm
	Cancel
	Pause
	TabPrev
	TabNext
	// NumActions is the number of actions (not an action itself).
	NumActions
)

// stickDead is how far the stick must be pushed to count as a direction.
const stickDead = 0.5

// keyMap maps actions to keys: the arrows, WASD and the vi keys (HJKL), so a keyboard
// without arrows works too.
var keyMap = map[Action][]ebiten.Key{
	Left:    {ebiten.KeyArrowLeft, ebiten.KeyA, ebiten.KeyH},
	Right:   {ebiten.KeyArrowRight, ebiten.KeyD, ebiten.KeyL},
	Up:      {ebiten.KeyArrowUp, ebiten.KeyW, ebiten.KeyK},
	Down:    {ebiten.KeyArrowDown, ebiten.KeyS, ebiten.KeyJ},
	Confirm: {ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyX},
	Cancel:  {ebiten.KeyEscape, ebiten.KeyBackspace, ebiten.KeyZ},
	Pause:   {ebiten.KeyEscape, ebiten.KeyF1, ebiten.KeyP},
	TabPrev: {ebiten.KeyQ, ebiten.KeyPageUp},
	TabNext: {ebiten.KeyE, ebiten.KeyPageDown},
}

// padMap maps actions to buttons on standard-layout gamepads (Xbox / PlayStation / Switch Pro, etc.).
var padMap = map[Action][]ebiten.StandardGamepadButton{
	Left:    {ebiten.StandardGamepadButtonLeftLeft},
	Right:   {ebiten.StandardGamepadButtonLeftRight},
	Up:      {ebiten.StandardGamepadButtonLeftTop},
	Down:    {ebiten.StandardGamepadButtonLeftBottom},
	Confirm: {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonCenterRight},
	Cancel:  {ebiten.StandardGamepadButtonRightRight, ebiten.StandardGamepadButtonCenterLeft},
	Pause:   {ebiten.StandardGamepadButtonCenterRight},
	TabPrev: {ebiten.StandardGamepadButtonFrontTopLeft},
	TabNext: {ebiten.StandardGamepadButtonFrontTopRight},
}

// rawPadMap maps actions to raw button numbers for pads not recognized as standard layout (many USB pads order them A,B,X,Y,L,R,Select,Start).
var rawPadMap = map[Action][]ebiten.GamepadButton{
	Confirm: {0, 7},
	Cancel:  {1, 6},
	Pause:   {7},
	TabPrev: {4},
	TabNext: {5},
}

// Input is the state of the controls, read once a frame by Update.
type Input struct {
	held, prev [NumActions]bool
	holdFrames [NumActions]int
	pads       []ebiten.GamepadID
	// lastSide is the side (-1 left, 1 right) pressed last: it wins while both are held.
	lastSide int
	// script, when set, stands in for the keyboard and the pads (the scenario tests play
	// the game with it); typed are the letters it typed this frame.
	script Script
	typed  []rune
}

// Script plays the game in place of a player: each frame it gives the actions held and
// the letters typed.
type Script interface {
	Frame() (held [NumActions]bool, typed []rune)
}

// SetScript makes s stand in for the keyboard and the pads from the next Update on (nil
// goes back to them).
func (in *Input) SetScript(s Script) { in.script = s }

// Update reads the controls for this frame.
func (in *Input) Update() {
	in.prev = in.held
	if in.script != nil {
		in.held, in.typed = in.script.Frame()
	} else {
		in.pads = ebiten.AppendGamepadIDs(in.pads[:0])
		for a := Action(0); a < NumActions; a++ {
			in.held[a] = in.poll(a)
		}
	}
	for a := Action(0); a < NumActions; a++ {
		if in.held[a] {
			in.holdFrames[a]++
		} else {
			in.holdFrames[a] = 0
		}
	}
	in.trackSide()
}

// trackSide notes which of left and right was pressed last.
func (in *Input) trackSide() {
	l, r := in.Pressed(Left), in.Pressed(Right)
	switch {
	case l && !r:
		in.lastSide = -1
	case r && !l:
		in.lastSide = 1
	}
}

// Side is the way the player steers: -1 left, 1 right, 0 neither. While both are held
// the one pressed last wins, so rolling from one key to the other turns her at once;
// before, both held stopped her for the frames the keys overlapped, and she started
// again from the slow start of a slide (SlideSpeed) when the first key came up.
func (in *Input) Side() int {
	l, r := in.held[Left], in.held[Right]
	switch {
	case l && r:
		return in.lastSide
	case l:
		return -1
	case r:
		return 1
	}
	return 0
}

func (in *Input) poll(a Action) bool {
	for _, k := range keyMap[a] {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	for _, id := range in.pads {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			for _, b := range padMap[a] {
				if ebiten.IsStandardGamepadButtonPressed(id, b) {
					return true
				}
			}
			x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
			y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
			if stickHit(a, x, y) {
				return true
			}
			continue
		}
		for _, b := range rawPadMap[a] {
			if ebiten.IsGamepadButtonPressed(id, b) {
				return true
			}
		}
		if ebiten.GamepadAxisCount(id) >= 2 &&
			stickHit(a, ebiten.GamepadAxisValue(id, 0), ebiten.GamepadAxisValue(id, 1)) {
			return true
		}
	}
	return false
}

// stickHit treats the left stick as a D-pad (for the directions only).
func stickHit(a Action, x, y float64) bool {
	switch a {
	case Left:
		return x < -stickDead
	case Right:
		return x > stickDead
	case Up:
		return y < -stickDead
	case Down:
		return y > stickDead
	default:
		return false
	}
}

// Chars returns the letters typed this frame.
func (in *Input) Chars() []rune {
	if in.script != nil {
		return in.typed
	}
	return ebiten.AppendInputChars(nil)
}

// Held reports whether a is held down this frame.
func (in *Input) Held(a Action) bool { return in.held[a] }

// Pressed reports whether a went down this frame.
func (in *Input) Pressed(a Action) bool { return in.held[a] && !in.prev[a] }

// Repeat implements key repeat for menus.
func (in *Input) Repeat(a Action) bool {
	f := in.holdFrames[a]
	return f == 1 || (f > 20 && f%5 == 0)
}
