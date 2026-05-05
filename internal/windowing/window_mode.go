package windowing

import "github.com/hndada/mos/internal/draws"

// WindowMode describes how a window participates in the compositor layout.
type WindowMode int

const (
	// WindowModeFullscreen fills the entire screen. Only one fullscreen
	// window is active at a time; all others are hidden or animating.
	WindowModeFullscreen WindowMode = iota

	// WindowModeSplit assigns the window to a fraction of the screen.
	// The multiWindowManager owns the layout geometry.
	WindowModeSplit

	// WindowModePip (picture-in-picture) is a small always-on-top overlay.
	// The multiWindowManager owns its corner position and drag behaviour.
	WindowModePip

	// WindowModeFloat lets the window occupy an arbitrary draggable rect.
	// The multiWindowManager provides the title-bar drag handle.
	WindowModeFloat
)

// WindowPlacement is the desired display rect for a window in screen space.
// Center is the screen-space midpoint; Size is the display dimensions.
//
// The window canvas is always the full screen size — the compositor scales
// the canvas to fit this rect when compositing, exactly as the existing
// open/close animation does via WindowAnim. This means apps always render
// at full resolution; only the visible portion changes. Input events are
// reverse-transformed back to canvas space before being sent to the app.
type WindowPlacement struct {
	Center draws.XY
	Size   draws.XY
}

func fullscreenPlacement(screenW, screenH float64) WindowPlacement {
	return WindowPlacement{
		Center: draws.XY{X: screenW / 2, Y: screenH / 2},
		Size:   draws.XY{X: screenW, Y: screenH},
	}
}
