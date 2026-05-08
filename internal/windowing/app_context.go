package windowing

import (
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

// windowContext implements app.Context on behalf of a single window.
// It is created by the windowing server when an app is launched and remains
// alive for the full lifetime of that window.
//
// All command methods (Finish, Launch, ShowKeyboard, HideKeyboard, PostNotice,
// RequestFocus, ReleaseFocus) forward the request to the server via proc.cmdCh.
// State queries (ScreenSize, SafeArea, Bus, Screenshots, HasFocus) read from
// goroutine-safe sources.
//
// invalidated is an atomic escape hatch: the app sets it true from any goroutine
// and the windowing server reads-and-clears it once per frame to decide whether
// to deliver a tick even when no input events arrived.
//
// hasFocus is written by the windowing server (main goroutine) under grantFocus
// / releaseFocus and may be read by app code from any goroutine via HasFocus().
// The atomic ensures the read is safe without a mutex.
type windowContext struct {
	ws      *Server
	proc    *windowProc
	appID   string
	screenW float64
	screenH float64

	safeAreaMu sync.RWMutex
	safeArea   mosapp.SafeArea

	wakeMu sync.Mutex
	wakeAt time.Time

	backMu      sync.RWMutex
	backHandler func() bool

	// invalidated is set true by Invalidate() (any goroutine) and consumed by
	// the windowing server each frame via CompareAndSwap.
	invalidated atomic.Bool

	// hasFocus is true while this window holds keyboard/IME focus. Written by
	// the windowing server on the main goroutine; read by app code via HasFocus().
	hasFocus atomic.Bool
}

func newWindowContext(ws *Server, proc *windowProc) *windowContext {
	return &windowContext{
		ws:      ws,
		proc:    proc,
		screenW: ws.ScreenW,
		screenH: ws.ScreenH,
	}
}

// setSafeArea is called by the windowing server each frame on the main
// goroutine; app code reads it via SafeArea() under RLock.
func (c *windowContext) setSafeArea(sa mosapp.SafeArea) {
	c.safeAreaMu.Lock()
	c.safeArea = sa
	c.safeAreaMu.Unlock()
}

func (c *windowContext) ScreenSize() draws.XY {
	return draws.XY{X: c.screenW, Y: c.screenH}
}

func (c *windowContext) AppID() string { return c.appID }

func (c *windowContext) Orientation() mosapp.Orientation {
	if c.screenW >= c.screenH {
		return mosapp.OrientationLandscape
	}
	return mosapp.OrientationPortrait
}

func (c *windowContext) DisplayScale() float64    { return 1 }
func (c *windowContext) Locale() string           { return c.ws.Locale() }
func (c *windowContext) TimeZone() *time.Location { return c.ws.TimeZone() }
func (c *windowContext) Now() time.Time           { return c.ws.Now() }
func (c *windowContext) FontScale() float64       { return c.ws.FontScale() }
func (c *windowContext) ReduceMotion() bool       { return c.ws.ReduceMotion() }
func (c *windowContext) BatteryLevel() int        { return c.ws.BatteryLevel() }
func (c *windowContext) NetworkStatus() mosapp.NetworkStatus {
	return c.ws.NetworkStatus()
}

func (c *windowContext) SafeArea() mosapp.SafeArea {
	c.safeAreaMu.RLock()
	defer c.safeAreaMu.RUnlock()
	return c.safeArea
}

func (c *windowContext) Finish() { c.proc.cmdCh <- CmdFinish{} }
func (c *windowContext) SetBackHandler(fn func() bool) {
	c.backMu.Lock()
	c.backHandler = fn
	c.backMu.Unlock()
}
func (c *windowContext) ClearBackHandler() {
	c.SetBackHandler(nil)
}
func (c *windowContext) handleBack() bool {
	c.backMu.RLock()
	fn := c.backHandler
	c.backMu.RUnlock()
	return fn != nil && fn()
}
func (c *windowContext) SetTitle(title string) {
	c.proc.cmdCh <- CmdSetTitle{Title: title}
}
func (c *windowContext) SetAccentColor(clr color.RGBA) {
	c.proc.cmdCh <- CmdSetAccentColor{Color: clr}
}
func (c *windowContext) Launch(appID string) { c.proc.cmdCh <- CmdLaunch{AppID: appID} }
func (c *windowContext) CanLaunch(appID string) bool {
	return c.ws.CanLaunch(appID)
}
func (c *windowContext) SetKeepScreenOn(enabled bool) {
	c.proc.cmdCh <- CmdSetKeepScreenOn{Enabled: enabled}
}
func (c *windowContext) SetPreferredOrientation(o mosapp.Orientation) {
	c.proc.cmdCh <- CmdSetPreferredOrientation{Orientation: o}
}
func (c *windowContext) OpenURL(rawURL string) {
	c.proc.cmdCh <- CmdOpenURL{URL: rawURL}
}
func (c *windowContext) ShareText(text string) {
	c.proc.cmdCh <- CmdShareText{Text: text}
}
func (c *windowContext) ShowKeyboard()         { c.proc.cmdCh <- CmdShowKeyboard{} }
func (c *windowContext) HideKeyboard()         { c.proc.cmdCh <- CmdHideKeyboard{} }
func (c *windowContext) KeyboardVisible() bool { return c.ws.KeyboardVisible() }

func (c *windowContext) PostNotice(n mosapp.Notice) {
	c.proc.cmdCh <- CmdPostNotice{Notice: n}
}
func (c *windowContext) ShowToast(text string) {
	c.proc.cmdCh <- CmdShowToast{Text: text}
}
func (c *windowContext) ScheduleNotice(id string, n mosapp.Notice, at time.Time) {
	c.proc.cmdCh <- CmdScheduleNotice{ID: id, Notice: n, At: at}
}
func (c *windowContext) CancelNotice(id string) {
	c.proc.cmdCh <- CmdCancelNotice{ID: id}
}
func (c *windowContext) SetBadge(count int) {
	c.proc.cmdCh <- CmdSetBadge{Count: count}
}
func (c *windowContext) ClearBadge() {
	c.proc.cmdCh <- CmdClearBadge{}
}
func (c *windowContext) SetStatusBarStyle(style mosapp.BarStyle) {
	c.proc.cmdCh <- CmdSetBarStyle{Slot: "status", Style: style}
}
func (c *windowContext) SetNavigationBarStyle(style mosapp.BarStyle) {
	c.proc.cmdCh <- CmdSetBarStyle{Slot: "navigation", Style: style}
}
func (c *windowContext) SetSystemBarsHidden(hidden bool) {
	c.proc.cmdCh <- CmdSetSystemBarsHidden{Hidden: hidden}
}

func (c *windowContext) IsDarkMode() bool { return c.ws.IsDarkMode() }
func (c *windowContext) SetDarkMode(enabled bool) {
	c.proc.cmdCh <- CmdSetDarkMode{Enabled: enabled}
}

// Invalidate signals the windowing server that this window needs a redraw
// on the next frame, even if no input events have arrived. Safe to call from
// any goroutine. The server consumes the flag at most once per frame.
func (c *windowContext) Invalidate() { c.invalidated.Store(true) }

func (c *windowContext) WakeAt(t time.Time) {
	c.wakeMu.Lock()
	c.wakeAt = t
	c.wakeMu.Unlock()
}

func (c *windowContext) consumeWakeDue(now time.Time) bool {
	c.wakeMu.Lock()
	defer c.wakeMu.Unlock()
	if c.wakeAt.IsZero() || now.Before(c.wakeAt) {
		return false
	}
	c.wakeAt = time.Time{}
	return true
}

// RequestFocus / ReleaseFocus post commands to the windowing server. The
// server processes them on the main goroutine after the current Update.
func (c *windowContext) RequestFocus() { c.proc.cmdCh <- CmdRequestFocus{} }
func (c *windowContext) ReleaseFocus() { c.proc.cmdCh <- CmdReleaseFocus{} }

// HasFocus reports whether the windowing server has granted keyboard focus to
// this window. The value is updated by the server on the main goroutine and
// read here without a lock via an atomic store/load pair.
func (c *windowContext) HasFocus() bool { return c.hasFocus.Load() }

func (c *windowContext) Bus() *event.Bus { return c.ws.Bus }

func (c *windowContext) ClipboardText() string { return c.ws.ClipboardText() }
func (c *windowContext) CopyText(text string)  { c.ws.CopyText(text) }

func (c *windowContext) DocumentsDir() string { return c.ws.DocumentsDir(c.appID) }
func (c *windowContext) CacheDir() string     { return c.ws.CacheDir(c.appID) }
func (c *windowContext) ReadFile(path string) ([]byte, error) {
	return c.ws.ReadFile(c.appID, path)
}
func (c *windowContext) WriteFile(path string, data []byte) error {
	return c.ws.WriteFile(c.appID, path, data)
}
func (c *windowContext) DeleteFile(path string) error {
	return c.ws.DeleteFile(c.appID, path)
}
func (c *windowContext) SaveFile(name string, data []byte) (string, error) {
	return c.ws.SaveFile(c.appID, name, data)
}
func (c *windowContext) PickFile() (string, []byte, bool) {
	return c.ws.PickFile()
}
func (c *windowContext) PickPhoto() (draws.Image, bool) {
	return c.ws.PickPhoto()
}

func (c *windowContext) Preference(key string) (string, bool) {
	return c.ws.Preference(c.appID, key)
}
func (c *windowContext) SetPreference(key, value string) {
	c.ws.SetPreference(c.appID, key, value)
}
func (c *windowContext) RemovePreference(key string) {
	c.ws.RemovePreference(c.appID, key)
}

func (c *windowContext) PermissionStatus(p mosapp.Permission) mosapp.PermissionStatus {
	return c.ws.PermissionStatus(p)
}
func (c *windowContext) RequestPermission(p mosapp.Permission) mosapp.PermissionStatus {
	return c.ws.RequestPermission(p)
}

func (c *windowContext) Vibrate(duration time.Duration) {
	c.proc.cmdCh <- CmdVibrate{Duration: duration}
}

func (c *windowContext) RequestAudioFocus() bool { return c.ws.RequestAudioFocus(c.appID) }
func (c *windowContext) ReleaseAudioFocus()      { c.ws.ReleaseAudioFocus(c.appID) }
func (c *windowContext) HasAudioFocus() bool     { return c.ws.HasAudioFocus(c.appID) }

func (c *windowContext) BeginBackgroundTask(name string) string {
	return c.ws.BeginBackgroundTask(c.appID, name)
}
func (c *windowContext) EndBackgroundTask(token string) {
	c.ws.EndBackgroundTask(token)
}

func (c *windowContext) Announce(text string) { c.ws.Announce(text) }

func (c *windowContext) Screenshots() []draws.Image { return c.ws.Screenshots() }
