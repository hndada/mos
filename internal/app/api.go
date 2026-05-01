// Package app defines the public API surface that MOS exposes to app authors.
// Every app implements Content; the optional Lifecycle and Context interfaces
// give richer integration with the OS.
package app

import (
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
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

// Notification is a message posted to the OS notification center (curtain panel).
type Notification struct {
	Title string
	Body  string
}

// Context is the OS handle every app receives at creation.
// It provides access to system services and lets the app drive OS actions.
type Context interface {
	// ScreenSize returns the pixel dimensions of the window's display canvas.
	ScreenSize() draws.XY

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

	// PostNotification sends a notification to the curtain / notification centre.
	PostNotification(n Notification)

	// Screenshots returns the list of in-memory screenshots captured by the user,
	// newest last. Primarily used by the Gallery app.
	Screenshots() []draws.Image
}
