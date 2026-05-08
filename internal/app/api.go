// Package app defines the public API surface that MOS exposes to app authors.
// Every app implements Content; the optional Lifecycle and Context interfaces
// give richer integration with the OS.
package app

import (
	"image/color"
	"time"

	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
	"github.com/hndada/mos/internal/input"
)

// Frame is the per-tick input bundle handed to Content.Update. Cursor is
// the latest pointer position (canvas-relative); Events is the ordered
// list of input events that occurred since the last tick.
//
// Apps that only need a cursor position can read frame.Cursor and ignore
// Events; widgets in the ui package consume the full Frame.
type Frame struct {
	Cursor draws.XY
	Events []input.Event
}

// Content is the interface every MOS app must implement.
// Update is called each frame while the window is fully open (Shown);
// Draw is called every frame the window is visible, including open/close animations.
type Content interface {
	Update(frame Frame)
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

// BarStyle controls status/navigation bar contrast.
type BarStyle int

const (
	BarStyleAuto BarStyle = iota
	BarStyleLight
	BarStyleDark
)

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

// Orientation is the current shape of the app canvas.
type Orientation int

const (
	OrientationUnspecified Orientation = iota
	OrientationPortrait
	OrientationLandscape
)

// NetworkStatus describes coarse connectivity.
type NetworkStatus int

const (
	NetworkOffline NetworkStatus = iota
	NetworkOnline
)

// Permission names a simulated protected capability.
type Permission string

const (
	PermissionCamera        Permission = "camera"
	PermissionMicrophone    Permission = "microphone"
	PermissionPhotos        Permission = "photos"
	PermissionNotifications Permission = "notifications"
	PermissionLocation      Permission = "location"
)

// PermissionStatus mirrors the common mobile granted/denied/not-determined flow.
type PermissionStatus int

const (
	PermissionNotDetermined PermissionStatus = iota
	PermissionGranted
	PermissionDenied
)

// Context is the OS handle every app receives at creation.
// It provides access to system services and lets the app drive OS actions.
//
// Implementation note: state queries (ScreenSize, SafeArea, Bus, Screenshots)
// read from goroutine-safe sources. Command methods (Finish, Launch,
// ShowKeyboard, HideKeyboard, PostNotice) are forwarded to the windowing
// server over a channel and dispatched on the main goroutine.
type Context interface {
	// AppID returns the registry id of this app, similar to Android's package
	// name or iOS's bundle identifier.
	AppID() string

	// ScreenSize returns the pixel dimensions of the window's display canvas.
	ScreenSize() draws.XY

	// Orientation reports whether the canvas is portrait or landscape.
	Orientation() Orientation

	// DisplayScale returns the simulated device pixel scale. MOS currently
	// renders in logical pixels, so the value is 1.
	DisplayScale() float64

	// Locale, FontScale, and ReduceMotion expose common accessibility and
	// configuration values that apps often use while laying out UI.
	Locale() string
	TimeZone() *time.Location
	Now() time.Time
	FontScale() float64
	ReduceMotion() bool
	BatteryLevel() int
	NetworkStatus() NetworkStatus

	// SafeArea returns the per-edge offsets reserved by system UI for the
	// current frame. The value can change between frames (e.g. when the
	// soft keyboard is shown).
	SafeArea() SafeArea

	// Finish asks the windowing server to close this app's window.
	Finish()
	SetBackHandler(fn func() bool)
	ClearBackHandler()

	// SetTitle / SetAccentColor update OS-visible app metadata used by window
	// chrome, logs, and future recents/search surfaces.
	SetTitle(title string)
	SetAccentColor(c color.RGBA)
	SetKeepScreenOn(enabled bool)
	SetPreferredOrientation(o Orientation)

	// Launch opens another app by ID. The new window animates in from screen centre.
	Launch(appID string)
	CanLaunch(appID string) bool

	// OpenURL asks the system to handle a URL. The simulator records/logs the
	// request and posts a lightweight notice instead of leaving MOS.
	OpenURL(rawURL string)

	// ShareText asks the system share sheet to share text. The simulator stores
	// the latest shared value and posts a notice.
	ShareText(text string)

	// Bus returns the OS-wide event bus.
	// Apps subscribe here to receive navigation, lifecycle, system, and custom events.
	Bus() *event.Bus

	// ShowKeyboard / HideKeyboard toggle the system soft keyboard.
	ShowKeyboard()
	HideKeyboard()

	// KeyboardVisible reports whether the soft keyboard is currently visible.
	KeyboardVisible() bool

	// PostNotice sends a notice to the curtain / notification centre.
	PostNotice(n Notice)
	ShowToast(text string)
	ScheduleNotice(id string, n Notice, at time.Time)
	CancelNotice(id string)
	SetBadge(count int)
	ClearBadge()

	// SetStatusBarStyle and SetNavigationBarStyle record requested system bar
	// contrast. Current MOS chrome is minimal, so these are simulated metadata.
	SetStatusBarStyle(style BarStyle)
	SetNavigationBarStyle(style BarStyle)
	SetSystemBarsHidden(hidden bool)

	// IsDarkMode / SetDarkMode expose the OS appearance setting.
	IsDarkMode() bool
	SetDarkMode(enabled bool)

	// Invalidate schedules a content redraw for the next frame. Call this when
	// the app changes visual state outside of an Update tick — for example from
	// a background goroutine, a timer callback, or an event-bus handler.
	//
	// Apps whose animated widgets (toggles, sliders, etc.) continue animating
	// beyond the input event that triggered them should also call Invalidate
	// from Update while the animation is still in-flight, so the windowing
	// server keeps delivering ticks and the canvas stays live.
	//
	// It is safe to call from any goroutine. Calling Invalidate on a window
	// that is not currently Shown is a no-op.
	Invalidate()

	// WakeAt asks the server to deliver an Update no earlier than t, even if no
	// input arrives. Use it for clocks, timers, and animations with a known next
	// frame. Passing a zero time clears any pending wake.
	WakeAt(t time.Time)

	// RequestFocus asks the windowing server to give keyboard and IME focus to
	// this app's window. Call this when a text field or other keyboard-driven
	// widget receives a tap. The server serialises focus changes and notifies
	// the previous holder via HasFocus() becoming false.
	//
	// In single-window (fullscreen) mode the active window always holds focus;
	// an explicit RequestFocus is only meaningful in split or freeform layouts
	// where multiple windows compete for keyboard input.
	RequestFocus()

	// ReleaseFocus voluntarily gives up keyboard and IME focus. The windowing
	// server clears the focus slot; future keyboard events are unrouted until
	// another window calls RequestFocus.
	ReleaseFocus()

	// HasFocus reports whether this window currently holds keyboard focus.
	// Read this before routing physical keyboard events to text fields so that
	// a background window does not "steal" characters from the focused one.
	//
	// Thread-safe: may be called from any goroutine.
	HasFocus() bool

	// ClipboardText / CopyText expose the simulated system clipboard.
	ClipboardText() string
	CopyText(text string)

	// DocumentsDir and CacheDir return per-app sandbox roots. File helpers
	// below are constrained to DocumentsDir.
	DocumentsDir() string
	CacheDir() string
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	DeleteFile(path string) error
	SaveFile(name string, data []byte) (string, error)
	PickFile() (path string, data []byte, ok bool)
	PickPhoto() (draws.Image, bool)

	// Preference reads and writes small per-app key/value settings. Keys are
	// namespaced by AppID so apps cannot collide with each other.
	Preference(key string) (string, bool)
	SetPreference(key, value string)
	RemovePreference(key string)

	// Permission APIs model common Android/iOS runtime permission flow. MOS
	// grants not-determined permissions by default so demos can proceed.
	PermissionStatus(p Permission) PermissionStatus
	RequestPermission(p Permission) PermissionStatus

	// Vibrate triggers simulated haptic feedback. The desktop simulator logs it.
	Vibrate(duration time.Duration)

	// Audio focus mirrors Android/iOS audio-session ownership at a coarse level.
	RequestAudioFocus() bool
	ReleaseAudioFocus()
	HasAudioFocus() bool

	// Background tasks model short-lived work that may finish after the app
	// leaves the foreground. The returned token must be ended by the app.
	BeginBackgroundTask(name string) string
	EndBackgroundTask(token string)

	// Announce sends an accessibility announcement. The simulator logs it and
	// posts a lightweight toast-style notice.
	Announce(text string)

	// Screenshots returns the list of in-memory screenshots captured by the user,
	// newest last. Primarily used by the Gallery app.
	Screenshots() []draws.Image
}
