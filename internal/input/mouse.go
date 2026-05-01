package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var cursorOffsetX, cursorOffsetY float64

// SetCursorOffset shifts the coordinate space returned by MouseCursorPosition.
// The simulator calls this with the viewport origin of the primary screen so
// that sysapps always work in canvas-relative coordinates.
func SetCursorOffset(x, y float64) { cursorOffsetX, cursorOffsetY = x, y }

func MouseCursorPosition() (float64, float64) {
	x, y := ebiten.CursorPosition()
	return float64(x) - cursorOffsetX, float64(y) - cursorOffsetY
}

// functions
var IsMouseButtonPressed = ebiten.IsMouseButtonPressed
var IsMouseButtonJustPressed = inpututil.IsMouseButtonJustPressed
var IsMouseButtonJustReleased = inpututil.IsMouseButtonJustReleased
var MouseWheelPosition = ebiten.Wheel

type MouseButton = ebiten.MouseButton

const (
	MouseButtonLeft   MouseButton = ebiten.MouseButtonLeft
	MouseButtonMiddle MouseButton = ebiten.MouseButtonMiddle
	MouseButtonRight  MouseButton = ebiten.MouseButtonRight
)
