package windowing

import (
	mosapp "github.com/hndada/mos/internal/app"
)

// windowProc is the command queue between app code and the windowing server.
// App lifecycle and Update calls run synchronously on the main goroutine, while
// Context methods enqueue commands that the server drains after each Update.
//
// This keeps the simulator simple and fast: there is no per-frame channel
// handshake and no goroutine that immediately blocks on an ack. The command
// queue still models the direction of OS requests (app to system), and it
// remains safe for app-owned background goroutines to call Context methods.
type windowProc struct {
	cmdCh chan AppCommand

	// Server-side state populated by drain() from cmdCh. Read by the
	// windowing server on the main goroutine each frame.
	shouldClose   bool
	pendingLaunch string
}

func newWindowProc() *windowProc {
	return &windowProc{
		cmdCh: make(chan AppCommand, 64),
	}
}

func (p *windowProc) create(content mosapp.Content, ctx *windowContext) {
	if content == nil {
		return
	}
	if lc, ok := content.(mosapp.Lifecycle); ok {
		lc.OnCreate(ctx)
	}
}

func (p *windowProc) update(content mosapp.Content, frame mosapp.Frame) {
	if content == nil {
		return
	}
	content.Update(frame)

	// Honour the legacy ShouldClose() opt-in without letting the server read
	// app-owned fields directly.
	if sc, ok := content.(interface{ ShouldClose() bool }); ok && sc.ShouldClose() {
		select {
		case p.cmdCh <- CmdFinish{}:
		default:
		}
	}
}

func (p *windowProc) resume(content mosapp.Content) {
	if lc, ok := content.(mosapp.Lifecycle); ok {
		lc.OnResume()
	}
}

func (p *windowProc) pause(content mosapp.Content) {
	if lc, ok := content.(mosapp.Lifecycle); ok {
		lc.OnPause()
	}
}

func (p *windowProc) destroy(content mosapp.Content) {
	if lc, ok := content.(mosapp.Lifecycle); ok {
		lc.OnDestroy()
	}
}

// drain consumes all queued commands from the app and dispatches them.
// w is the Window that owns this proc; it is needed to route focus commands
// back to the correct window. Side-effects on the windowing server
// (ShowKeyboard, focus, log) happen inline; commands that need windowing
// decisions (Finish, Launch) update fields the server reads after this call.
func (p *windowProc) drain(ws *Server, w *Window) {
	for {
		select {
		case cmd := <-p.cmdCh:
			switch c := cmd.(type) {
			case CmdFinish:
				p.shouldClose = true
			case CmdLaunch:
				p.pendingLaunch = c.AppID
			case CmdSetTitle:
				ws.SetTitle(w.AppID(), c.Title)
			case CmdSetAccentColor:
				w.clr = c.Color
				w.app.Color = c.Color
				w.canvasDirty = true
			case CmdSetKeepScreenOn:
				ws.SetKeepScreenOn(w.AppID(), c.Enabled)
			case CmdSetPreferredOrientation:
				ws.SetPreferredOrientation(w.AppID(), c.Orientation)
			case CmdOpenURL:
				ws.OpenURL(c.URL)
			case CmdShareText:
				ws.ShareText(c.Text)
			case CmdShowKeyboard:
				ws.ShowKeyboard()
			case CmdHideKeyboard:
				ws.HideKeyboard()
			case CmdPostNotice:
				ws.PostNotice(c.Notice)
			case CmdShowToast:
				ws.ShowToast(c.Text)
			case CmdScheduleNotice:
				ws.ScheduleNotice(w.AppID(), c.ID, c.Notice, c.At)
			case CmdCancelNotice:
				ws.CancelNotice(w.AppID(), c.ID)
			case CmdSetBadge:
				ws.SetBadge(w.AppID(), c.Count)
			case CmdClearBadge:
				ws.SetBadge(w.AppID(), 0)
			case CmdSetBarStyle:
				ws.SetBarStyle(w.AppID(), c.Slot, c.Style)
			case CmdSetSystemBarsHidden:
				ws.SetSystemBarsHidden(w.AppID(), c.Hidden)
			case CmdSetDarkMode:
				ws.SetDarkMode(c.Enabled)
			case CmdVibrate:
				ws.Vibrate(c.Duration)
			case CmdRequestFocus:
				ws.grantFocus(w)
			case CmdReleaseFocus:
				ws.releaseFocus(w)
			default:
				_ = c
			}
		default:
			return
		}
	}
}

// drainLaunch returns and clears any queued app-id launch.
func (p *windowProc) drainLaunch() string {
	id := p.pendingLaunch
	p.pendingLaunch = ""
	return id
}
