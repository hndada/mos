package windowing

import (
	"sync"
	"sync/atomic"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

// windowContext implements app.Context on behalf of a single window.
// It is created by the windowing server when an app is launched and remains
// alive for the full lifetime of that window.
//
// All command methods (Finish, Launch, ShowKeyboard, HideKeyboard, PostNotice,
// RequestFocus, ReleaseFocus) run on the app goroutine and forward the request
// to the server via proc.cmdCh. State queries (ScreenSize, SafeArea, Bus,
// Screenshots, HasFocus) read from goroutine-safe sources.
//
// invalidated is an atomic escape hatch: the app sets it true from any goroutine
// and the windowing server reads-and-clears it once per frame to decide whether
// to deliver a tick even when no input events arrived.
//
// hasFocus is written by the windowing server (main goroutine) under grantFocus
// / releaseFocus and is read by app code (app goroutine) via HasFocus(). The
// atomic ensures the read is safe without a mutex.
type windowContext struct {
	ws      *WindowingServer
	proc    *windowProc
	screenW float64
	screenH float64

	safeAreaMu sync.RWMutex
	safeArea   mosapp.SafeArea

	// invalidated is set true by Invalidate() (any goroutine) and consumed by
	// the windowing server each frame via CompareAndSwap.
	invalidated atomic.Bool

	// hasFocus is true while this window holds keyboard/IME focus. Written by
	// the windowing server on the main goroutine; read by app code via HasFocus().
	hasFocus atomic.Bool
}

func newWindowContext(ws *WindowingServer, proc *windowProc) *windowContext {
	return &windowContext{
		ws:      ws,
		proc:    proc,
		screenW: ws.ScreenW,
		screenH: ws.ScreenH,
	}
}

// setSafeArea is called by the windowing server each frame on the main
// goroutine; the app goroutine reads it via SafeArea() under RLock.
func (c *windowContext) setSafeArea(sa mosapp.SafeArea) {
	c.safeAreaMu.Lock()
	c.safeArea = sa
	c.safeAreaMu.Unlock()
}

func (c *windowContext) ScreenSize() draws.XY {
	return draws.XY{X: c.screenW, Y: c.screenH}
}

func (c *windowContext) SafeArea() mosapp.SafeArea {
	c.safeAreaMu.RLock()
	defer c.safeAreaMu.RUnlock()
	return c.safeArea
}

func (c *windowContext) Finish()             { c.proc.cmdCh <- CmdFinish{} }
func (c *windowContext) Launch(appID string) { c.proc.cmdCh <- CmdLaunch{AppID: appID} }
func (c *windowContext) ShowKeyboard()       { c.proc.cmdCh <- CmdShowKeyboard{} }
func (c *windowContext) HideKeyboard()       { c.proc.cmdCh <- CmdHideKeyboard{} }

func (c *windowContext) PostNotice(n mosapp.Notice) {
	c.proc.cmdCh <- CmdPostNotice{Notice: n}
}

// Invalidate signals the windowing server that this window needs a redraw
// on the next frame, even if no input events have arrived. Safe to call from
// any goroutine. The server consumes the flag at most once per frame.
func (c *windowContext) Invalidate() { c.invalidated.Store(true) }

// RequestFocus / ReleaseFocus post commands to the windowing server. The
// server processes them on the main goroutine after the current tick acks.
func (c *windowContext) RequestFocus() { c.proc.cmdCh <- CmdRequestFocus{} }
func (c *windowContext) ReleaseFocus() { c.proc.cmdCh <- CmdReleaseFocus{} }

// HasFocus reports whether the windowing server has granted keyboard focus to
// this window. The value is updated by the server on the main goroutine and
// read here without a lock via an atomic store/load pair.
func (c *windowContext) HasFocus() bool { return c.hasFocus.Load() }

func (c *windowContext) Bus() *event.Bus { return c.ws.Bus }

func (c *windowContext) Screenshots() []draws.Image { return c.ws.Screenshots() }
