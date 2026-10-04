package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/sound"
)

type Action int

const (
	ActLeft Action = iota
	ActRight
	ActUp
	ActDown
	ActConfirm
	ActCancel
	ActPause
	ActTabPrev
	ActTabNext
	actionCount
)

// stickDead is how far the stick must be pushed to count as a direction.
const stickDead = 0.5

// keyMap maps actions to keys: the arrows, WASD and the vi keys (HJKL), so a keyboard
// without arrows works too.
var keyMap = map[Action][]ebiten.Key{
	ActLeft:    {ebiten.KeyArrowLeft, ebiten.KeyA, ebiten.KeyH},
	ActRight:   {ebiten.KeyArrowRight, ebiten.KeyD, ebiten.KeyL},
	ActUp:      {ebiten.KeyArrowUp, ebiten.KeyW, ebiten.KeyK},
	ActDown:    {ebiten.KeyArrowDown, ebiten.KeyS, ebiten.KeyJ},
	ActConfirm: {ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyX},
	ActCancel:  {ebiten.KeyEscape, ebiten.KeyBackspace, ebiten.KeyZ},
	ActPause:   {ebiten.KeyEscape, ebiten.KeyF1, ebiten.KeyP},
	ActTabPrev: {ebiten.KeyQ, ebiten.KeyPageUp},
	ActTabNext: {ebiten.KeyE, ebiten.KeyPageDown},
}

// padMap maps actions to buttons on standard-layout gamepads (Xbox / PlayStation / Switch Pro, etc.).
var padMap = map[Action][]ebiten.StandardGamepadButton{
	ActLeft:    {ebiten.StandardGamepadButtonLeftLeft},
	ActRight:   {ebiten.StandardGamepadButtonLeftRight},
	ActUp:      {ebiten.StandardGamepadButtonLeftTop},
	ActDown:    {ebiten.StandardGamepadButtonLeftBottom},
	ActConfirm: {ebiten.StandardGamepadButtonRightBottom, ebiten.StandardGamepadButtonCenterRight},
	ActCancel:  {ebiten.StandardGamepadButtonRightRight, ebiten.StandardGamepadButtonCenterLeft},
	ActPause:   {ebiten.StandardGamepadButtonCenterRight},
	ActTabPrev: {ebiten.StandardGamepadButtonFrontTopLeft},
	ActTabNext: {ebiten.StandardGamepadButtonFrontTopRight},
}

// rawPadMap maps actions to raw button numbers for pads not recognized as standard layout (many USB pads order them A,B,X,Y,L,R,Select,Start).
var rawPadMap = map[Action][]ebiten.GamepadButton{
	ActConfirm: {0, 7},
	ActCancel:  {1, 6},
	ActPause:   {7},
	ActTabPrev: {4},
	ActTabNext: {5},
}

type Input struct {
	held, prev [actionCount]bool
	holdFrames [actionCount]int
	pads       []ebiten.GamepadID
	// lastSide is the side (-1 left, 1 right) pressed last: it wins while both are held.
	lastSide int
	// script, when set, stands in for the keyboard and the pads (the scenario tests play
	// the game with it); typed are the letters it typed this frame.
	script inputScript
	typed  []rune
}

// inputScript plays the game in place of a player: each frame it gives the actions held
// and the letters typed.
type inputScript interface {
	frame() (held [actionCount]bool, typed []rune)
}

func (in *Input) Update() {
	in.prev = in.held
	if in.script != nil {
		in.held, in.typed = in.script.frame()
	} else {
		in.pads = ebiten.AppendGamepadIDs(in.pads[:0])
		for a := Action(0); a < actionCount; a++ {
			in.held[a] = in.poll(a)
		}
	}
	for a := Action(0); a < actionCount; a++ {
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
	l, r := in.Pressed(ActLeft), in.Pressed(ActRight)
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
	l, r := in.held[ActLeft], in.held[ActRight]
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
	case ActLeft:
		return x < -stickDead
	case ActRight:
		return x > stickDead
	case ActUp:
		return y < -stickDead
	case ActDown:
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

func (in *Input) Held(a Action) bool    { return in.held[a] }
func (in *Input) Pressed(a Action) bool { return in.held[a] && !in.prev[a] }

// Repeat implements key repeat for menus.
func (in *Input) Repeat(a Action) bool {
	f := in.holdFrames[a]
	return f == 1 || (f > 20 && f%5 == 0)
}

// menuNav moves the selection sel of a menu of n items with key repeat: prev steps it back
// and next forward, wrapping around at the ends, each step with a click.
func (in *Input) menuNav(sel, n int, prev, next Action) int {
	if in.Repeat(prev) {
		sel = (sel + n - 1) % n
		sound.Play(sound.Move)
	}
	if in.Repeat(next) {
		sel = (sel + 1) % n
		sound.Play(sound.Move)
	}
	return sel
}
