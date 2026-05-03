package windowing

import (
	mosapp "github.com/hndada/mos/internal/app"
)

// windowProc is the IPC fixture between the windowing server (main goroutine)
// and a single app's goroutine. Its purpose is to model the boundary that a
// real multi-process mobile OS enforces between system_server / SpringBoard
// and an app's separate UNIX/Mach process.
//
// Channel map:
//
//	tickCh: server → app. The server sends one tickMsg per frame describing
//	        either a content Update (with the input Frame) or a lifecycle
//	        callback. The goroutine processes it and signals back via ackCh.
//	ackCh:  app → server. Lock-step barrier — the server blocks on ackCh
//	        after every tick send so app state is never read concurrently
//	        with mutation.
//	cmdCh:  app → server. Whenever app code invokes a Context method like
//	        Finish(), Launch(), or PostNotice(), the implementation enqueues
//	        an AppCommand here. The server drains cmdCh after each ack.
//
// Real-OS mapping (this is "just a simulator," but the protocol shape is the
// same one a real mobile OS would use across process boundaries):
//
//	goroutine        ↔  separate UNIX/Mach process running the app binary
//	tickCh + ackCh   ↔  ABI calls into the app surface
//	                   - Android: Choreographer + BufferQueue + onFrameAvailable
//	                   - iOS:     -drawRect: + presentRenderbuffer
//	cmdCh            ↔  IPC up to the system server
//	                   - Android: Binder transactions to system_server
//	                   - iOS:     XPC messages to SpringBoard / running boards
//
// Where we cheat: app.Content.Draw is invoked on the main goroutine because
// Ebiten's image API is not goroutine-safe. A real OS solves the equivalent
// problem with shared GPU memory and a buffer-acquire/release protocol; we
// lock-step Update→ack→Draw so this is race-free without a mutex.
type windowProc struct {
	tickCh chan tickMsg
	ackCh  chan struct{}
	cmdCh  chan AppCommand

	// Server-side state populated by drain() from cmdCh. Read by the
	// windowing server on the main goroutine each frame.
	shouldClose   bool
	pendingLaunch string
}

// tickKind enumerates the actions the goroutine must perform per tick.
type tickKind int

const (
	tickUpdate  tickKind = iota // run content.Update(frame)
	tickResume                  // run OnResume
	tickPause                   // run OnPause
	tickDestroy                 // run OnDestroy and exit
)

type tickMsg struct {
	kind  tickKind
	frame mosapp.Frame
}

func newWindowProc() *windowProc {
	return &windowProc{
		tickCh: make(chan tickMsg),
		ackCh:  make(chan struct{}, 1),
		cmdCh:  make(chan AppCommand, 64),
	}
}

// run is the goroutine entry point. It drives one app's content from
// OnCreate through OnDestroy. content may be nil for placeholder windows
// (no registered factory); in that case the loop drains ticks and acks
// without invoking app code.
func (p *windowProc) run(content mosapp.Content, ctx *windowContext) {
	if content == nil {
		// Placeholder window. Ack the synthetic OnCreate, then drain ticks.
		p.ackCh <- struct{}{}
		for msg := range p.tickCh {
			if msg.kind == tickDestroy {
				p.ackCh <- struct{}{}
				return
			}
			p.ackCh <- struct{}{}
		}
		return
	}

	if lc, ok := content.(mosapp.Lifecycle); ok {
		lc.OnCreate(ctx)
	}
	p.ackCh <- struct{}{}

	for msg := range p.tickCh {
		switch msg.kind {
		case tickUpdate:
			content.Update(msg.frame)

			// Honour the legacy ShouldClose() opt-in: if the app exposes one
			// and reports true, self-issue a Finish so we don't have to read
			// its state from the main goroutine.
			if sc, ok := content.(interface{ ShouldClose() bool }); ok && sc.ShouldClose() {
				select {
				case p.cmdCh <- CmdFinish{}:
				default:
				}
			}

		case tickResume:
			if lc, ok := content.(mosapp.Lifecycle); ok {
				lc.OnResume()
			}

		case tickPause:
			if lc, ok := content.(mosapp.Lifecycle); ok {
				lc.OnPause()
			}

		case tickDestroy:
			if lc, ok := content.(mosapp.Lifecycle); ok {
				lc.OnDestroy()
			}
			p.ackCh <- struct{}{}
			return
		}
		p.ackCh <- struct{}{}
	}
}

// drain consumes all queued commands from the goroutine and dispatches
// them. Side-effects on the windowing server (ShowKeyboard, log) happen
// inline; commands that need windowing decisions (Finish, Launch) update
// fields the server reads after this call.
func (p *windowProc) drain(ws *WindowingServer) {
	for {
		select {
		case cmd := <-p.cmdCh:
			switch c := cmd.(type) {
			case CmdFinish:
				p.shouldClose = true
			case CmdLaunch:
				p.pendingLaunch = c.AppID
			case CmdShowKeyboard:
				ws.ShowKeyboard()
			case CmdHideKeyboard:
				ws.HideKeyboard()
			case CmdPostNotice:
				ws.log("notice: " + c.Notice.Title + " — " + c.Notice.Body)
				// TODO: enqueue into the curtain notice list
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

// sendTick blocks until the goroutine has acked the message. This is the
// lock-step barrier that lets the server safely call content.Draw on the
// main goroutine afterwards.
func (p *windowProc) sendTick(msg tickMsg) {
	p.tickCh <- msg
	<-p.ackCh
}
