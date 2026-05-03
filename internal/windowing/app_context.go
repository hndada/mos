package windowing

import (
	"sync"

	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

// windowContext implements app.Context on behalf of a single window.
// It is created by the windowing server when an app is launched and remains
// alive for the full lifetime of that window.
//
// All command methods (Finish, Launch, ShowKeyboard, HideKeyboard, PostNotice)
// run on the app goroutine and forward the request to the server via
// proc.cmdCh. State queries (ScreenSize, SafeArea, Bus, Screenshots) read
// from goroutine-safe sources.
type windowContext struct {
	ws      *WindowingServer
	proc    *windowProc
	screenW float64
	screenH float64

	safeAreaMu sync.RWMutex
	safeArea   mosapp.SafeArea
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

func (c *windowContext) Bus() *event.Bus { return c.ws.Bus }

func (c *windowContext) Screenshots() []draws.Image { return c.ws.Screenshots() }
