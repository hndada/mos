package windowing

import (
	mosapp "github.com/hndada/mos/internal/app"
	"github.com/hndada/mos/internal/draws"
	"github.com/hndada/mos/internal/event"
)

// windowContext implements app.Context on behalf of a single window.
// It is created by the windowing server when an app is launched and remains
// alive for the full lifetime of that window.
type windowContext struct {
	ws      *WindowingServer
	screenW float64
	screenH float64

	// Flags written by the app and drained by the windowing server each frame.
	shouldClose   bool
	pendingLaunch string
}

func newWindowContext(ws *WindowingServer) *windowContext {
	return &windowContext{ws: ws, screenW: ws.ScreenW, screenH: ws.ScreenH}
}

func (c *windowContext) ScreenSize() draws.XY {
	return draws.XY{X: c.screenW, Y: c.screenH}
}

func (c *windowContext) Finish() { c.shouldClose = true }

func (c *windowContext) Launch(appID string) { c.pendingLaunch = appID }

func (c *windowContext) Bus() *event.Bus { return c.ws.Bus }

func (c *windowContext) ShowKeyboard() { c.ws.ShowKeyboard() }
func (c *windowContext) HideKeyboard() { c.ws.HideKeyboard() }

func (c *windowContext) PostNotification(n mosapp.Notification) {
	c.ws.log("notification: " + n.Title + " — " + n.Body)
	// TODO: enqueue into the curtain notification list
}

func (c *windowContext) Screenshots() []draws.Image {
	return c.ws.screenshots
}

// drainLaunch returns and clears any pending app-ID launch the app requested.
func (c *windowContext) drainLaunch() string {
	id := c.pendingLaunch
	c.pendingLaunch = ""
	return id
}
