package windowing

import "github.com/hndada/mos/internal/draws"

// Mode describes how a window participates in the compositor layout.
type Mode int

const (
	// ModeFullscreen fills the entire screen. Only one fullscreen
	// window is active at a time; all others are hidden or animating.
	ModeFullscreen Mode = iota

	// ModeSplit clips the full-size app surface into a fraction of the screen.
	// The multiWindowState owns the pane geometry.
	ModeSplit

	// ModePip (picture-in-picture) is a small always-on-top overlay.
	// The multiWindowState owns its corner position and drag behaviour.
	ModePip

	// ModeFloat lets the window occupy an arbitrary draggable rect.
	// The multiWindowState provides the title-bar drag handle.
	ModeFloat
)

func (m Mode) String() string {
	switch m {
	case ModeFullscreen:
		return "fullscreen"
	case ModeSplit:
		return "split"
	case ModePip:
		return "pip"
	case ModeFloat:
		return "float"
	default:
		return "unknown"
	}
}

// Placement is the desired display rect for a window in screen space.
// Center is the screen-space midpoint; Size is the display dimensions.
//
// The window canvas is always the full screen size. Split panes clip the
// native-size canvas; PiP/freeform scale the canvas to fit this rect, exactly
// as the existing open/close animation does via anim. Apps always render at
// full resolution, and input events are transformed back to canvas space
// before being sent to the app.
type Placement struct {
	Center draws.XY
	Size   draws.XY
}

func fullscreenPlacement(screenW, screenH float64) Placement {
	return Placement{
		Center: draws.XY{X: screenW / 2, Y: screenH / 2},
		Size:   draws.XY{X: screenW, Y: screenH},
	}
}
