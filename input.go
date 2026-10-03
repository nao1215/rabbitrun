package main

import (
	"github.com/hajimehoshi/ebiten/v2"
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
}

func (in *Input) Update() {
	in.prev = in.held
	in.pads = ebiten.AppendGamepadIDs(in.pads[:0])
	for a := Action(0); a < actionCount; a++ {
		in.held[a] = in.poll(a)
		if in.held[a] {
			in.holdFrames[a]++
		} else {
			in.holdFrames[a] = 0
		}
	}
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

func (in *Input) Held(a Action) bool    { return in.held[a] }
func (in *Input) Pressed(a Action) bool { return in.held[a] && !in.prev[a] }

// Repeat implements key repeat for menus.
func (in *Input) Repeat(a Action) bool {
	f := in.holdFrames[a]
	return f == 1 || (f > 20 && f%5 == 0)
}
