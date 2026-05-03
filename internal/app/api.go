// Package app defines the public API surface that MOS exposes to app authors.
// Every app implements Content; the optional Lifecycle and Context interfaces
// give richer integration with the OS.
package app

import (
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
)

// Content is the interface every MOS app must implement.
// Update is called each frame while the window is fully open (Shown);
// Draw is called every frame the window is visible, including open/close animations.
type Content interface {
	Update(cursor draws.XY)
	Draw(dst draws.Image)
}

// Lifecycle is an optional extension of Content.
// The windowing server calls each method at the appropriate phase.
// Apps that hold subscriptions or timers should use OnCreate / OnDestroy.
type Lifecycle interface {
	// OnCreate is called once, immediately after the app is instantiated.
	// ctx is the same Context the factory received; store it for later use.
	OnCreate(ctx Context)
	// OnResume is called when the opening animation finishes (window is fully visible).
	OnResume()
	// OnPause is called when the closing animation begins.
	OnPause()
	// OnDestroy is called just before the window is purged from memory.
	OnDestroy()
}

// Notice is a message posted to the OS notification center (curtain panel).
type Notice struct {
	Title string
	Body  string
}

// SafeArea describes the screen edges reserved by system UI: status bar,
// soft keyboard, navigation chrome. Apps should keep critical UI inside the
// rectangle that ScreenSize() shrinks to after each edge is subtracted.
//
// Equivalent terminology elsewhere:
//   - Android:        WindowInsets / safeDrawingPadding
//   - iOS / SwiftUI:  safeAreaInsets
//   - CSS:            env(safe-area-inset-*)
//   - Flutter:        MediaQuery.padding / viewInsets
//
// We use the Go-idiomatic name SafeArea (concrete noun, no platform jargon)
// rather than "Insets".
type SafeArea struct {
	Top, Right, Bottom, Left float64
}

// Context is the OS handle every app receives at creation.
// It provides access to system services and lets the app drive OS actions.
//
// Implementation note: state queries (ScreenSize, SafeArea, Bus, Screenshots)
// read from goroutine-safe sources. Command methods (Finish, Launch,
// ShowKeyboard, HideKeyboard, PostNotice) are forwarded to the windowing
// server over a channel and dispatched on the main goroutine.
type Context interface {
	// ScreenSize returns the pixel dimensions of the window's display canvas.
	ScreenSize() draws.XY

	// SafeArea returns the per-edge offsets reserved by system UI for the
	// current frame. The value can change between frames (e.g. when the
	// soft keyboard is shown).
	SafeArea() SafeArea

	// PollInput returns the next pending input event for this tick, with
	// ok==true. When no more events are queued for this tick, ok==false.
	// Apps that only need a cursor can ignore this; one is supplied to
	// Update directly.
	PollInput() (input.Event, bool)

	// Finish asks the windowing server to close this app's window.
	Finish()

	// Launch opens another app by ID. The new window animates in from screen centre.
	Launch(appID string)

	// Bus returns the OS-wide event bus.
	// Apps subscribe here to receive navigation, lifecycle, system, and custom events.
	Bus() *event.Bus

	// ShowKeyboard / HideKeyboard toggle the system soft keyboard.
	ShowKeyboard()
	HideKeyboard()

	// PostNotice sends a notice to the curtain / notification centre.
	PostNotice(n Notice)

	// Screenshots returns the list of in-memory screenshots captured by the user,
	// newest last. Primarily used by the Gallery app.
	Screenshots() []draws.Image
}
