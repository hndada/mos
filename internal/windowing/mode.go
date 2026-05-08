package windowing

import "github.com/hndada/mos/internal/draws"

// Mode describes how a window participates in the compositor layout.
type Mode int

const (
	// ModeFullscreen fills the entire screen. Only one fullscreen
	// window is active at a time; all others are hidden or animating.
	ModeFullscreen Mode = iota

	// ModeSplit assigns the window to a fraction of the screen.
	// The multiWindowManager owns the layout geometry.
	ModeSplit

	// ModePip (picture-in-picture) is a small always-on-top overlay.
	// The multiWindowManager owns its corner position and drag behaviour.
	ModePip

	// ModeFloat lets the window occupy an arbitrary draggable rect.
	// The multiWindowManager provides the title-bar drag handle.
	ModeFloat
)

// Placement is the desired display rect for a window in screen space.
// Center is the screen-space midpoint; Size is the display dimensions.
//
// The window canvas is always the full screen size — the compositor scales
// the canvas to fit this rect when compositing, exactly as the existing
// open/close animation does via anim. This means apps always render
// at full resolution; only the visible portion changes. Input events are
// reverse-transformed back to canvas space before being sent to the app.
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
